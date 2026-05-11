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
    image_url: str


@dataclass(frozen=True)
class PhotoSearchResult:
    product_id: str
    image_url: str
    score: float


class PhotoProductIndex:
    def __init__(self, provider: str, model: str) -> None:
        self.provider = provider
        self.model = model
        self._product_metas: list[PhotoProductMeta] = []
        self._vectors: np.ndarray | None = None
        self._knn: NearestNeighbors | None = None
        self._dim: int | None = None

    @property
    def product_metas(self) -> list[PhotoProductMeta]:
        return list(self._product_metas)

    def build(self, metas: list[PhotoProductMeta], vectors: np.ndarray) -> None:
        normalized = normalize_rows(vectors)
        if len(metas) != normalized.shape[0]:
            raise ValueError("metas length must match vectors rows")
        if normalized.shape[0] == 0:
            raise ValueError("index cannot be built from empty vectors")

        self._product_metas = list(metas)
        self._vectors = normalized
        self._dim = int(normalized.shape[1])
        self._knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self._knn.fit(self._vectors)

    def query(self, query_vector: np.ndarray, top_k: int = 5) -> list[PhotoSearchResult]:
        if self._knn is None or self._vectors is None or self._dim is None:
            return []

        q = np.asarray(query_vector, dtype=np.float32)
        if q.ndim != 1:
            raise ValueError("query_vector must be a 1D array")
        if q.shape[0] != self._dim:
            raise ValueError("query_vector dimensions must match index dimensions")
        if top_k <= 0:
            top_k = 5

        q2 = normalize_rows(q.reshape(1, -1))
        n_neighbors = min(top_k, len(self._product_metas))
        distances, indices = self._knn.kneighbors(q2, n_neighbors=n_neighbors)

        results: list[PhotoSearchResult] = []
        for distance, idx in zip(distances[0], indices[0]):
            meta = self._product_metas[int(idx)]
            score = 1.0 - float(distance)
            results.append(
                PhotoSearchResult(
                    product_id=meta.product_id,
                    image_url=meta.image_url,
                    score=score,
                )
            )

        results.sort(key=lambda item: item.score, reverse=True)
        return results

    def save(self, path: str) -> None:
        if self._vectors is None or self._dim is None:
            raise ValueError("cannot save an empty index")
        payload = {
            "provider": self.provider,
            "model": self.model,
            "dimensions": self._dim,
            "product_metas": self._product_metas,
            "vectors": self._vectors,
        }
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(pickle.dumps(payload))

    def load(self, path: str) -> None:
        source = Path(path)
        data = pickle.loads(source.read_bytes())

        file_provider = data["provider"]
        file_model = data["model"]
        file_dim = int(data["dimensions"])

        if file_provider != self.provider:
            raise ValueError("incompatible provider in persisted index")
        if file_model != self.model:
            raise ValueError("incompatible model in persisted index")

        vectors = np.asarray(data["vectors"], dtype=np.float32)
        if vectors.ndim != 2:
            raise ValueError("persisted vectors must be a 2D array")
        if vectors.shape[1] != file_dim:
            raise ValueError("incompatible dimensions in persisted index")

        metas = list(data["product_metas"])
        if len(metas) != vectors.shape[0]:
            raise ValueError("persisted metas length must match vectors rows")

        self._product_metas = metas
        self._vectors = normalize_rows(vectors)
        self._dim = file_dim
        self._knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self._knn.fit(self._vectors)
