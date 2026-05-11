from __future__ import annotations

from dataclasses import dataclass
import numpy as np

from src.config import Config
from src.photo_search.embeddings import EmbeddingProvider
from src.photo_search.fusion import weighted_fuse
from src.photo_search.index import PhotoProductIndex
from src.photo_search.parser import OCRProductTextParser, ProductTextMeta


class PhotoSearchIndexNotReady(RuntimeError):
    pass


@dataclass(frozen=True)
class PhotoSearchCandidateDTO:
    product_id: str
    score: float


@dataclass(frozen=True)
class PhotoSearchResultDTO:
    matched_name: str
    matched_brand: str
    candidates: list[PhotoSearchCandidateDTO]


class PhotoSearchEngine:
    def __init__(
        self,
        index: PhotoProductIndex | None,
        provider: EmbeddingProvider,
        config: Config,
    ) -> None:
        self._index = index
        self._provider = provider
        self._config = config

    def _query_weights(self) -> dict[str, float]:
        return {
            "image": float(self._config.PHOTO_SEARCH_QUERY_WEIGHT_IMAGE),
            "ocr_raw": float(self._config.PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW),
            "ocr_name": float(self._config.PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME),
            "ocr_brand": float(self._config.PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND),
            "ocr_percentages": float(self._config.PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES),
            "ocr_volume": float(self._config.PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME),
        }

    def search(
        self,
        image: bytes,
        mime_type: str,
        ocr_text: str,
        top_k: int,
    ) -> PhotoSearchResultDTO:
        if self._index is None or self._index.knn is None:
            raise PhotoSearchIndexNotReady("photo search index is not ready")

        parser = OCRProductTextParser(
            products=[
                ProductTextMeta(
                    product_id=meta.product_id,
                    name=meta.name,
                    brand_name=meta.brand_name,
                )
                for meta in self._index.product_metas()
            ]
        )
        parsed = parser.parse(ocr_text)

        query_mode = getattr(self._config, "PHOTO_SEARCH_INDEX_MODE", "legacy_image_only")
        if query_mode == "legacy_image_only":
            query_payload = {
                "text": parsed.normalized_ocr,
                "image_bytes": image,
                "mime_type": mime_type,
            }
            embeddings = self._provider.embed_multimodal([query_payload])
            if embeddings.shape[0] == 0:
                return PhotoSearchResultDTO(
                    matched_name=parsed.matched_name or "",
                    matched_brand=parsed.matched_brand or "",
                    candidates=[],
                )
            query_vector = embeddings[0]
        else:
            channel_vectors: dict[str, np.ndarray] = {}
            weights = self._query_weights()

            text_channels = {
                "ocr_raw": parsed.normalized_ocr,
                "ocr_name": parsed.extracted_product_name or parsed.matched_name or "",
                "ocr_brand": parsed.matched_brand or "",
                "ocr_percentages": " ".join(f"{value:g}%" for value in parsed.extracted_percentages),
                "ocr_volume": parsed.extracted_volume or "",
            }
            for channel, text_value in text_channels.items():
                if weights.get(channel, 0.0) <= 0.0:
                    continue
                text = text_value.strip()
                if not text:
                    continue
                text_embeddings = self._provider.embed_texts([text])
                if text_embeddings.shape[0] != 1:
                    continue
                channel_vectors[channel] = text_embeddings[0]

            if weights.get("image", 0.0) > 0.0 and image:
                query_payload = {
                    "text": parsed.normalized_ocr,
                    "image_bytes": image,
                    "mime_type": mime_type,
                }
                try:
                    image_embeddings = self._provider.embed_multimodal([query_payload])
                    if image_embeddings.shape[0] == 1:
                        channel_vectors["image"] = image_embeddings[0]
                except Exception:
                    pass

            try:
                query_vector = weighted_fuse(channel_vectors, weights)
            except ValueError:
                return PhotoSearchResultDTO(
                    matched_name=parsed.matched_name or "",
                    matched_brand=parsed.matched_brand or "",
                    candidates=[],
                )

        base_results = self._index.query(
            query_vector=query_vector,
            top_k=top_k,
            min_score=self._config.PHOTO_SEARCH_MIN_SCORE,
        )

        candidates: list[PhotoSearchCandidateDTO] = []
        for item in base_results:
            score = float(item.score)
            if parsed.matched_brand and item.meta.brand_name == parsed.matched_brand:
                score += 0.08
            if parsed.matched_name and item.meta.name == parsed.matched_name:
                score += 0.12
            score = max(0.0, min(1.0, score))
            candidates.append(PhotoSearchCandidateDTO(product_id=item.product_id, score=score))

        candidates.sort(key=lambda candidate: candidate.score, reverse=True)
        return PhotoSearchResultDTO(
            matched_name=parsed.matched_name or "",
            matched_brand=parsed.matched_brand or "",
            candidates=candidates,
        )
