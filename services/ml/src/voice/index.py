import pickle
from dataclasses import dataclass
from pathlib import Path

import numpy as np
from sklearn.neighbors import NearestNeighbors


@dataclass(frozen=True)
class Match:
    product_id: str
    product_name: str
    score: float


class VoiceIndex:
    def __init__(self) -> None:
        self.product_ids: list[str] = []
        self.product_names: list[str] = []
        self.vectors: np.ndarray | None = None
        self.source_provider: str = ""
        self.source_model: str = ""
        self.source_dimensions: int = 0
        self._knn: NearestNeighbors | None = None

    def fit(
        self,
        ids: list[str],
        names: list[str],
        vectors: list[np.ndarray],
        *,
        source_provider: str = "",
        source_model: str = "",
        source_dimensions: int | None = None,
    ) -> None:
        if not (len(ids) == len(names) == len(vectors)):
            raise ValueError("ids, names, and vectors must have equal length")
        self.product_ids = list(ids)
        self.product_names = list(names)
        self.vectors = np.vstack(vectors).astype(np.float32)
        dimensions = int(source_dimensions) if source_dimensions is not None else int(self.vectors.shape[1])
        if dimensions <= 0:
            raise ValueError("source_dimensions must be positive")
        self.source_provider = source_provider
        self.source_model = source_model
        self.source_dimensions = dimensions
        self._knn = NearestNeighbors(metric="cosine", algorithm="brute").fit(self.vectors)

    def query(self, vec: np.ndarray, top_k: int = 5) -> list[Match]:
        if self._knn is None or not self.product_ids:
            return []
        k = min(top_k, len(self.product_ids))
        distances, indices = self._knn.kneighbors([vec], n_neighbors=k)
        return [
            Match(product_id=self.product_ids[i], product_name=self.product_names[i], score=float(1.0 - d))
            for d, i in zip(distances[0], indices[0])
        ]

    def save(self, path: Path | str) -> None:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        with open(path, "wb") as f:
            pickle.dump(self, f)

    @classmethod
    def load(cls, path: Path | str) -> "VoiceIndex":
        with open(path, "rb") as f:
            return pickle.load(f)
