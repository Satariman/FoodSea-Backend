from __future__ import annotations

from dataclasses import dataclass
import re
from typing import Iterable


_TOKEN_RE = re.compile(r"[a-zа-я0-9]+", flags=re.IGNORECASE)
_PERCENT_RE = re.compile(r"(\d+(?:\.\d+)?)\s*%")
_VOLUME_RE = re.compile(r"\b(\d+(?:\.\d+)?)\s*(мл|л|г|гр|кг)\b")


@dataclass(frozen=True)
class ProductTextMeta:
    product_id: str
    name: str
    brand_name: str


@dataclass(frozen=True)
class ParsedOCR:
    matched_name: str | None
    matched_brand: str | None
    normalized_ocr: str
    name_confidence: float
    extracted_product_name: str | None
    extracted_percentages: tuple[float, ...]
    extracted_volume: str | None


@dataclass(frozen=True)
class _IndexedProduct:
    meta: ProductTextMeta
    normalized_brand: str
    normalized_name: str
    brand_tokens: tuple[str, ...]
    name_tokens: tuple[str, ...]


class OCRProductTextParser:
    _LOW_CONFIDENCE_THRESHOLD = 0.55
    _MIN_BRAND_OVERLAP_THRESHOLD = 0.5

    def __init__(self, products: Iterable[ProductTextMeta]) -> None:
        self._products = [self._index_product(product) for product in products]

    def parse(self, text: str) -> ParsedOCR:
        canonical_text = self._canonicalize_text(text)
        normalized_ocr = self._normalize_text(canonical_text)
        ocr_tokens = self._tokenize(normalized_ocr)

        matched_brand = self._match_brand(ocr_tokens)
        matched_product, name_confidence = self._match_product(ocr_tokens, matched_brand)
        matched_name = matched_product.meta.name if matched_product else None
        extracted_percentages = self._extract_percentages(canonical_text)
        extracted_volume = self._extract_volume(canonical_text)
        extracted_product_name = self._extract_product_name(
            normalized_ocr,
            matched_brand,
            extracted_percentages,
            extracted_volume,
        )

        if name_confidence < self._LOW_CONFIDENCE_THRESHOLD:
            matched_name = self._fallback_name_snippet(normalized_ocr)

        return ParsedOCR(
            matched_name=matched_name,
            matched_brand=matched_brand,
            normalized_ocr=normalized_ocr,
            name_confidence=name_confidence,
            extracted_product_name=extracted_product_name,
            extracted_percentages=extracted_percentages,
            extracted_volume=extracted_volume,
        )

    def _match_brand(self, ocr_tokens: tuple[str, ...]) -> str | None:
        best_brand: str | None = None
        best_score = 0.0

        seen: dict[str, tuple[str, ...]] = {}
        for product in self._products:
            seen.setdefault(product.meta.brand_name, product.brand_tokens)

        for brand, brand_tokens in seen.items():
            score = self._overlap_score(ocr_tokens, brand_tokens)
            if score > best_score or (
                score == best_score and best_brand is not None and brand < best_brand
            ):
                best_score = score
                best_brand = brand

        if best_brand is None:
            return None
        if best_score < self._MIN_BRAND_OVERLAP_THRESHOLD:
            return None
        return best_brand

    def _match_product(
        self, ocr_tokens: tuple[str, ...], brand: str | None
    ) -> tuple[_IndexedProduct | None, float]:
        candidates = self._products
        if brand is not None:
            candidates = [product for product in self._products if product.meta.brand_name == brand]

        best: _IndexedProduct | None = None
        best_score = 0.0

        for product in candidates:
            score = self._name_confidence(ocr_tokens, product.name_tokens)
            if score > best_score or (
                score == best_score
                and best is not None
                and self._product_tie_key(product) < self._product_tie_key(best)
            ):
                best_score = score
                best = product

        if best_score <= 0:
            return None, 0.0
        return best, best_score

    @staticmethod
    def _index_product(product: ProductTextMeta) -> _IndexedProduct:
        normalized_brand = OCRProductTextParser._normalize_text(product.brand_name)
        normalized_name = OCRProductTextParser._normalize_text(product.name)
        return _IndexedProduct(
            meta=product,
            normalized_brand=normalized_brand,
            normalized_name=normalized_name,
            brand_tokens=OCRProductTextParser._tokenize(normalized_brand),
            name_tokens=OCRProductTextParser._tokenize(normalized_name),
        )

    @staticmethod
    def _normalize_text(text: str) -> str:
        return " ".join(_TOKEN_RE.findall(OCRProductTextParser._canonicalize_text(text)))

    @staticmethod
    def _canonicalize_text(text: str) -> str:
        lowered = text.lower().replace("ё", "е").replace("Ё", "Е")
        lowered = lowered.replace(",", ".")
        lowered = re.sub(r"\s+", " ", lowered)
        return lowered.strip()

    @staticmethod
    def _tokenize(text: str) -> tuple[str, ...]:
        return tuple(token for token in text.split(" ") if token)

    @staticmethod
    def _overlap_score(ocr_tokens: tuple[str, ...], candidate_tokens: tuple[str, ...]) -> float:
        if not ocr_tokens or not candidate_tokens:
            return 0.0
        ocr_set = set(ocr_tokens)
        overlap = sum(1 for token in candidate_tokens if token in ocr_set)
        if overlap == 0:
            return 0.0
        return overlap / len(candidate_tokens)

    @staticmethod
    def _important_token_overlap(
        ocr_tokens: tuple[str, ...], candidate_tokens: tuple[str, ...]
    ) -> float:
        if not ocr_tokens or not candidate_tokens:
            return 0.0
        important = tuple(token for token in candidate_tokens if len(token) >= 4)
        if not important:
            return OCRProductTextParser._overlap_score(ocr_tokens, candidate_tokens)
        return OCRProductTextParser._overlap_score(ocr_tokens, important)

    @staticmethod
    def _name_confidence(ocr_tokens: tuple[str, ...], candidate_tokens: tuple[str, ...]) -> float:
        base_overlap = OCRProductTextParser._overlap_score(ocr_tokens, candidate_tokens)
        important_overlap = OCRProductTextParser._important_token_overlap(
            ocr_tokens, candidate_tokens
        )
        return round((base_overlap * 0.4) + (important_overlap * 0.6), 3)

    @staticmethod
    def _product_tie_key(product: _IndexedProduct) -> tuple[str, str, str]:
        return (
            product.normalized_name,
            product.normalized_brand,
            product.meta.product_id,
        )

    @staticmethod
    def _fallback_name_snippet(normalized_ocr: str, max_tokens: int = 6) -> str | None:
        tokens = OCRProductTextParser._tokenize(normalized_ocr)
        if not tokens:
            return None
        return " ".join(tokens[:max_tokens])

    @staticmethod
    def _extract_percentages(canonical_text: str) -> tuple[float, ...]:
        seen: set[float] = set()
        values: list[float] = []
        for raw in _PERCENT_RE.findall(canonical_text):
            value = float(raw)
            if value in seen:
                continue
            seen.add(value)
            values.append(value)
        return tuple(values)

    @staticmethod
    def _extract_volume(canonical_text: str) -> str | None:
        match = _VOLUME_RE.search(canonical_text)
        if match is None:
            return None
        value = OCRProductTextParser._normalize_number(float(match.group(1)))
        unit = match.group(2)
        if unit == "гр":
            unit = "г"
        return f"{value} {unit}"

    @staticmethod
    def _extract_product_name(
        normalized_ocr: str,
        matched_brand: str | None,
        percentages: tuple[float, ...],
        volume: str | None,
    ) -> str | None:
        tokens = OCRProductTextParser._tokenize(normalized_ocr)
        if not tokens:
            return None

        drop_tokens = {
            "акция",
            "скидка",
            "новинка",
            "которым",
            "которые",
            "мы",
            "гордимся",
            "процент",
            "процентов",
            "мл",
            "л",
            "г",
            "гр",
            "кг",
        }
        if matched_brand:
            brand_normalized = OCRProductTextParser._normalize_text(
                OCRProductTextParser._canonicalize_text(matched_brand)
            )
            drop_tokens.update(
                OCRProductTextParser._tokenize(
                    brand_normalized
                )
            )
            drop_tokens.update(
                OCRProductTextParser._tokenize(
                    OCRProductTextParser._normalize_text(
                        OCRProductTextParser._transliterate_ru_to_lat(brand_normalized)
                    )
                )
            )
        if volume:
            drop_tokens.update(
                OCRProductTextParser._tokenize(
                    OCRProductTextParser._normalize_text(
                        OCRProductTextParser._canonicalize_text(volume)
                    )
                )
            )
            drop_tokens.update(
                OCRProductTextParser._tokenize(
                    OCRProductTextParser._normalize_text(
                        OCRProductTextParser._canonicalize_text(volume.replace(" ", ""))
                    )
                )
            )
        for percent in percentages:
            percent_token_source = OCRProductTextParser._normalize_number(percent)
            drop_tokens.update(
                OCRProductTextParser._tokenize(
                    OCRProductTextParser._normalize_text(percent_token_source)
                )
            )

        filtered_with_index = [
            (index, token)
            for index, token in enumerate(tokens)
            if token not in drop_tokens
        ]
        filtered = [token for _, token in filtered_with_index]
        if not filtered:
            return None

        percent_tokens = set()
        for percent in percentages:
            percent_tokens.update(
                OCRProductTextParser._tokenize(
                    OCRProductTextParser._normalize_text(
                        OCRProductTextParser._normalize_number(percent)
                    )
                )
            )
        percent_positions = [
            index for index, token in enumerate(tokens) if token in percent_tokens
        ]
        if percent_positions:
            last_percent_position = max(percent_positions)
            tail = [
                token for index, token in filtered_with_index if index > last_percent_position
            ]
            if tail:
                return " ".join(tail[:8])

        return " ".join(filtered[:8])

    @staticmethod
    def _normalize_number(value: float) -> str:
        if value.is_integer():
            return str(int(value))
        return f"{value:.6f}".rstrip("0").rstrip(".")

    @staticmethod
    def _transliterate_ru_to_lat(text: str) -> str:
        table = {
            "а": "a",
            "б": "b",
            "в": "v",
            "г": "g",
            "д": "d",
            "е": "e",
            "ж": "zh",
            "з": "z",
            "и": "i",
            "й": "i",
            "к": "k",
            "л": "l",
            "м": "m",
            "н": "n",
            "о": "o",
            "п": "p",
            "р": "r",
            "с": "s",
            "т": "t",
            "у": "u",
            "ф": "f",
            "х": "h",
            "ц": "ts",
            "ч": "ch",
            "ш": "sh",
            "щ": "sch",
            "ы": "y",
            "э": "e",
            "ю": "yu",
            "я": "ya",
            "ь": "",
            "ъ": "",
            " ": " ",
        }
        return "".join(table.get(char, char) for char in text)
