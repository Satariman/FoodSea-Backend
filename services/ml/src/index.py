"""KNN index implementation for analog search."""

from __future__ import annotations

import pickle
from pathlib import Path

import numpy as np
from sklearn.neighbors import NearestNeighbors


class AnalogIndex:
    """In-memory index with optional persistence."""

    def __init__(self) -> None:
        self.knn: NearestNeighbors | None = None
        self.product_ids: list[str] = []
        self.product_names: dict[str, str] = {}
        self.vectors: np.ndarray | None = None
        self.id_to_idx: dict[str, int] = {}
        self.product_offers: dict[str, dict[str, int]] = {}
        self.min_prices: dict[str, int] = {}

    def build(
        self,
        product_ids: list[str],
        names: dict[str, str],
        vectors: np.ndarray,
        offers: dict[str, dict[str, int]],
    ) -> None:
        self.product_ids = product_ids
        self.product_names = names
        self.vectors = np.asarray(vectors, dtype=np.float32)
        self.id_to_idx = {pid: idx for idx, pid in enumerate(product_ids)}
        self.product_offers = offers
        self.min_prices = {
            pid: min(store_prices.values()) if store_prices else 0
            for pid, store_prices in offers.items()
        }

        self.knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self.knn.fit(self.vectors)

    def query(
        self,
        product_id: str,
        top_k: int = 5,
        price_aware: bool = False,
        filter_store_ids: set[str] | None = None,
        price_penalty: float = 0.3,
    ) -> list[tuple[str, str, float, int]]:
        """Returns tuples: (product_id, product_name, score, min_price_kopecks)."""

        if top_k <= 0:
            top_k = 5
        if self.knn is None or self.vectors is None or len(self.product_ids) < 2:
            return []
        vectors = self.vectors

        idx = self.id_to_idx.get(product_id)
        if idx is None:
            return []

        fetch_k = min(max(top_k * 5, top_k + 1), len(self.product_ids))
        query_vector = vectors[idx].reshape(1, -1)
        distances, indices = self.knn.kneighbors(query_vector, n_neighbors=fetch_k)

        original_price = self.min_prices.get(product_id, 0)
        results: list[tuple[str, str, float, int]] = []

        for distance, neighbor_idx in zip(distances[0], indices[0]):
            neighbor_id = self.product_ids[int(neighbor_idx)]
            if neighbor_id == product_id:
                continue

            similarity = 1.0 - float(distance)
            neighbor_offers = self.product_offers.get(neighbor_id, {})

            if filter_store_ids:
                available_stores = set(neighbor_offers.keys()) & filter_store_ids
                if not available_stores:
                    continue
                min_price = min(neighbor_offers[store_id] for store_id in available_stores)
            else:
                min_price = self.min_prices.get(neighbor_id, 0)

            score = similarity
            if price_aware and original_price > 0 and min_price > 0:
                if min_price <= original_price:
                    savings_ratio = (original_price - min_price) / original_price
                    score = similarity * (1.0 + price_penalty * savings_ratio)
                else:
                    overprice_ratio = (min_price - original_price) / original_price
                    score = similarity * (1.0 - price_penalty * min(overprice_ratio, 1.0))
                    if score < 0:
                        score = 0.0

            results.append((neighbor_id, self.product_names.get(neighbor_id, ""), score, min_price))

        results.sort(key=lambda item: item[2], reverse=True)
        return results[:top_k]

    def save(self, path: str) -> None:
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "product_ids": self.product_ids,
            "product_names": self.product_names,
            "vectors": self.vectors,
            "product_offers": self.product_offers,
            "min_prices": self.min_prices,
        }
        target.write_bytes(pickle.dumps(payload))

    def load(self, path: str) -> bool:
        source = Path(path)
        if not source.exists():
            return False

        data = pickle.loads(source.read_bytes())

        self.product_ids = data["product_ids"]
        self.product_names = data["product_names"]
        self.vectors = data["vectors"]
        self.product_offers = data["product_offers"]
        self.min_prices = data["min_prices"]
        self.id_to_idx = {pid: idx for idx, pid in enumerate(self.product_ids)}

        if self.vectors is None or len(self.product_ids) == 0:
            self.knn = None
            return True

        self.knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self.knn.fit(self.vectors)
        return True
