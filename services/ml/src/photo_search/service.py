from __future__ import annotations

from dataclasses import dataclass

from src.config import Config
from src.photo_search.embeddings import EmbeddingProvider
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

        base_results = self._index.query(
            query_vector=embeddings[0],
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
            candidates.append(PhotoSearchCandidateDTO(product_id=item.product_id, score=score))

        candidates.sort(key=lambda candidate: candidate.score, reverse=True)
        return PhotoSearchResultDTO(
            matched_name=parsed.matched_name or "",
            matched_brand=parsed.matched_brand or "",
            candidates=candidates,
        )

