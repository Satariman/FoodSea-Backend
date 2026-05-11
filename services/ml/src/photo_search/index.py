from __future__ import annotations

import pickle
from dataclasses import dataclass
from pathlib import Path

import numpy as np
from sklearn.neighbors import NearestNeighbors


def normalize_rows(vectors: np.ndarray) -> np.ndarray:
    arr = np.asarray(vectors, dtype=np.float32)
    if arr.ndim != 2:
        raise ValueError("vectors must be a 2D array")
    norms = np.linalg.norm(arr, axis=1, keepdims=True)
    safe_norms = np.where(norms == 0.0, 1.0, norms)
    return arr / safe_norms


@dataclass(frozen=True)
class PhotoProductMeta:
    product_id: str
    name: str
    brand_name: str
    category_name: str
    subcategory_name: str
    image_url: str


@dataclass(frozen=True)
class PhotoSearchResult:
    product_id: str
    score: float
    meta: PhotoProductMeta


class PhotoProductIndex:
    def __init__(self) -> None:
        self.knn: NearestNeighbors | None = None
        self.product_ids: list[str] = []
        self.metas: list[PhotoProductMeta] = []
        self.vectors: np.ndarray | None = None
        self.provider: str = ""
        self.model: str = ""
        self.dimensions: int = 0

    def product_metas(self) -> list[PhotoProductMeta]:
        by_id = {meta.product_id: meta for meta in self.metas}
        return [by_id[product_id] for product_id in self.product_ids if product_id in by_id]

    def build(
        self,
        metas: list[PhotoProductMeta],
        vectors: np.ndarray,
        provider: str,
        model: str,
        dimensions: int,
    ) -> None:
        normalized = normalize_rows(vectors)
        if len(metas) != normalized.shape[0]:
            raise ValueError("metas length must match vectors rows")
        if normalized.shape[0] == 0:
            raise ValueError("index cannot be built from empty vectors")
        if int(normalized.shape[1]) != int(dimensions):
            raise ValueError("dimensions must match vectors columns")

        self.product_ids = [meta.product_id for meta in metas]
        self.metas = list(metas)
        self.vectors = normalized
        self.provider = provider
        self.model = model
        self.dimensions = int(dimensions)
        self.knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self.knn.fit(self.vectors)

    def query(
        self, query_vector: np.ndarray, top_k: int = 5, min_score: float = 0.0
    ) -> list[PhotoSearchResult]:
        if self.knn is None or self.vectors is None or self.dimensions == 0:
            return []

        q = np.asarray(query_vector, dtype=np.float32)
        if q.ndim != 1:
            raise ValueError("query_vector must be a 1D array")
        if q.shape[0] != self.dimensions:
            raise ValueError("query_vector dimensions must match index dimensions")
        if top_k <= 0:
            top_k = 5

        q2 = normalize_rows(q.reshape(1, -1))
        n_neighbors = min(top_k, len(self.product_ids))
        distances, indices = self.knn.kneighbors(q2, n_neighbors=n_neighbors)

        results: list[PhotoSearchResult] = []
        for distance, idx in zip(distances[0], indices[0]):
            meta = self.metas[int(idx)]
            score = 1.0 - float(distance)
            if score < min_score:
                continue
            results.append(
                PhotoSearchResult(
                    product_id=meta.product_id,
                    score=score,
                    meta=meta,
                )
            )

        results.sort(key=lambda item: item.score, reverse=True)
        return results

    def save(self, path: str) -> None:
        if self.vectors is None or self.dimensions == 0:
            raise ValueError("cannot save an empty index")
        payload = {
            "provider": self.provider,
            "model": self.model,
            "dimensions": self.dimensions,
            "product_ids": self.product_ids,
            "metas": self.metas,
            "vectors": self.vectors,
        }
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(pickle.dumps(payload))

    def load(self, path: str, provider: str, model: str, dimensions: int) -> bool:
        source = Path(path)
        if not source.exists():
            return False
        try:
            data = pickle.loads(source.read_bytes())
        except Exception:
            return False
        if not isinstance(data, dict):
            return False

        file_provider = data.get("provider")
        file_model = data.get("model")
        file_dim_raw = data.get("dimensions", 0)
        if not isinstance(file_provider, str) or not isinstance(file_model, str):
            return False
        try:
            file_dim = int(file_dim_raw)
        except (TypeError, ValueError):
            return False
        if file_provider != provider:
            return False
        if file_model != model:
            return False
        if file_dim != int(dimensions):
            return False

        try:
            vectors = np.asarray(data.get("vectors"), dtype=np.float32)
        except (TypeError, ValueError):
            return False
        if vectors.ndim != 2:
            return False
        if vectors.shape[0] == 0:
            return False
        if vectors.shape[1] != file_dim:
            return False

        metas_raw = data.get("metas")
        product_ids_raw = data.get("product_ids")
        if not isinstance(metas_raw, list) or not isinstance(product_ids_raw, list):
            return False
        if not all(isinstance(meta, PhotoProductMeta) for meta in metas_raw):
            return False
        if not all(isinstance(product_id, str) for product_id in product_ids_raw):
            return False

        metas = list(metas_raw)
        product_ids = list(product_ids_raw)
        if len(metas) != vectors.shape[0]:
            return False
        if len(product_ids) != len(metas):
            return False
        if product_ids != [meta.product_id for meta in metas]:
            return False

        self.product_ids = product_ids
        self.metas = metas
        self.vectors = normalize_rows(vectors)
        self.provider = provider
        self.model = model
        self.dimensions = file_dim
        self.knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self.knn.fit(self.vectors)
        return True
