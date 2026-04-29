"""Feature vector builder for product analog search."""

from __future__ import annotations

import re
from typing import Protocol

import numpy as np

from src.data_loader import ProductData


class TextEncoder(Protocol):
    def encode(self, texts: list[str], **kwargs: object) -> np.ndarray: ...


class FeatureBuilder:
    """Builds combined feature vectors from text, nutrition, category, size and price."""

    _WEIGHT_RE = re.compile(r"(?P<num>\d+(?:[.,]\d+)?)\s*(?P<unit>кг|г|гр|мл|л)", re.IGNORECASE)

    def __init__(
        self,
        text_model_name: str,
        text_weight: float,
        category_weight: float,
        nutrition_weight: float,
        price_weight: float,
        encoder: TextEncoder | None = None,
    ) -> None:
        if encoder is None:
            from sentence_transformers import SentenceTransformer

            encoder = SentenceTransformer(text_model_name)
        self.model = encoder
        self.text_weight = text_weight
        self.category_weight = category_weight
        self.nutrition_weight = nutrition_weight
        self.price_weight = price_weight
        self.category_to_idx: dict[str, int] = {}

    def build(self, products: list[ProductData]) -> np.ndarray:
        """Builds feature matrix of shape (N, D) for all products."""

        if not products:
            return np.zeros((0, 0), dtype=np.float32)

        categories = sorted({p.category_id for p in products if p.category_id})
        self.category_to_idx = {cat: idx for idx, cat in enumerate(categories)}
        n_cats = len(categories)

        texts = [self._product_text(product) for product in products]
        text_embeddings = self.model.encode(
            texts,
            show_progress_bar=False,
            normalize_embeddings=True,
            convert_to_numpy=True,
        )
        text_embeddings = np.asarray(text_embeddings, dtype=np.float32) * self.text_weight

        nutrition = np.array(
            [[p.calories, p.protein, p.fat, p.carbohydrates] for p in products],
            dtype=np.float32,
        )
        nmin = nutrition.min(axis=0)
        nmax = nutrition.max(axis=0)
        denom = nmax - nmin
        denom[denom == 0] = 1.0
        nutrition_norm = (nutrition - nmin) / denom * self.nutrition_weight

        cat_onehot = np.zeros((len(products), n_cats), dtype=np.float32)
        for i, product in enumerate(products):
            cat_idx = self.category_to_idx.get(product.category_id)
            if cat_idx is not None:
                cat_onehot[i, cat_idx] = self.category_weight

        weights = np.array([self._parse_weight(p.weight) for p in products], dtype=np.float32).reshape(-1, 1)
        weight_max = float(weights.max())
        if weight_max > 0:
            weights = weights / weight_max

        prices = np.array([p.min_price_kopecks for p in products], dtype=np.float32).reshape(-1, 1)
        pmin = float(prices.min())
        pmax = float(prices.max())
        if pmax > pmin:
            prices = (prices - pmin) / (pmax - pmin)
        else:
            prices = np.zeros_like(prices, dtype=np.float32)
        prices = prices * self.price_weight

        return np.hstack([text_embeddings, nutrition_norm, cat_onehot, weights, prices]).astype(np.float32)

    @staticmethod
    def _product_text(product: ProductData) -> str:
        parts = [product.name]
        if product.description:
            parts.append(product.description)
        if product.composition:
            parts.append(product.composition)
        return " ".join(parts)

    @classmethod
    def _parse_weight(cls, weight: str) -> float:
        """Parses strings like '500 мл', '1.5 кг', '200 г' into grams/ml units."""

        if not weight:
            return 0.0

        match = cls._WEIGHT_RE.search(weight.strip().lower())
        if not match:
            return 0.0

        raw_number = match.group("num").replace(",", ".")
        try:
            value = float(raw_number)
        except ValueError:
            return 0.0

        unit = match.group("unit")
        if unit == "кг":
            return value * 1000.0
        if unit in {"г", "гр", "мл"}:
            return value
        if unit == "л":
            return value * 1000.0
        return 0.0
