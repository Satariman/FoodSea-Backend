from __future__ import annotations

from dataclasses import dataclass
import re
from typing import Iterable


_TOKEN_RE = re.compile(r"[a-zа-я0-9]+", flags=re.IGNORECASE)


@dataclass(frozen=True)
class ProductTextMeta:
    product_id: str
    brand: str
    name: str


@dataclass(frozen=True)
class ParsedOCR:
    raw_text: str
    normalized_text: str
    brand: str | None
    product_name: str | None
    product_id: str | None


@dataclass(frozen=True)
class _IndexedProduct:
    meta: ProductTextMeta
    normalized_brand: str
    normalized_name: str
    brand_tokens: tuple[str, ...]
    name_tokens: tuple[str, ...]


class OCRProductTextParser:
    def __init__(self, products: Iterable[ProductTextMeta]) -> None:
        self._products = [self._index_product(product) for product in products]

    def parse(self, text: str) -> ParsedOCR:
        normalized_text = self._normalize_text(text)
        ocr_tokens = self._tokenize(normalized_text)

        brand = self._match_brand(ocr_tokens)
        matched_product = self._match_product(ocr_tokens, brand)

        return ParsedOCR(
            raw_text=text,
            normalized_text=normalized_text,
            brand=brand,
            product_name=matched_product.meta.name if matched_product else None,
            product_id=matched_product.meta.product_id if matched_product else None,
        )

    def _match_brand(self, ocr_tokens: tuple[str, ...]) -> str | None:
        best_brand: str | None = None
        best_score = 0.0

        seen: dict[str, tuple[str, ...]] = {}
        for product in self._products:
            seen.setdefault(product.meta.brand, product.brand_tokens)

        for brand, brand_tokens in seen.items():
            score = self._overlap_score(ocr_tokens, brand_tokens)
            if score > best_score:
                best_score = score
                best_brand = brand

        return best_brand if best_score > 0 else None

    def _match_product(
        self, ocr_tokens: tuple[str, ...], brand: str | None
    ) -> _IndexedProduct | None:
        candidates = self._products
        if brand is not None:
            candidates = [product for product in self._products if product.meta.brand == brand]

        best: _IndexedProduct | None = None
        best_score = 0.0

        for product in candidates:
            score = self._overlap_score(ocr_tokens, product.name_tokens)
            if score > best_score:
                best_score = score
                best = product

        return best if best_score > 0 else None

    @staticmethod
    def _index_product(product: ProductTextMeta) -> _IndexedProduct:
        normalized_brand = OCRProductTextParser._normalize_text(product.brand)
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
        lowered = text.lower().replace("ё", "е").replace("Ё", "Е")
        lowered = lowered.replace(",", ".")
        return " ".join(_TOKEN_RE.findall(lowered))

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
