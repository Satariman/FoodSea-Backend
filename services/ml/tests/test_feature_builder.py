from __future__ import annotations

import numpy as np

from src.data_loader import ProductData
from src.feature_builder import FeatureBuilder


class DummyEncoder:
    def encode(self, texts: list[str], **_: object) -> np.ndarray:
        base = np.linspace(0.0, 1.0, 384, dtype=np.float32)
        return np.tile(base, (len(texts), 1))


def make_product(
    product_id: str,
    category_id: str,
    weight: str,
    min_price_kopecks: int,
    calories: float,
    protein: float,
    fat: float,
    carbs: float,
) -> ProductData:
    return ProductData(
        product_id=product_id,
        name=f"product-{product_id}",
        description="desc",
        composition="comp",
        category_id=category_id,
        subcategory_id="",
        brand_id="",
        weight=weight,
        calories=calories,
        protein=protein,
        fat=fat,
        carbohydrates=carbs,
        offers={"store1": min_price_kopecks},
        min_price_kopecks=min_price_kopecks,
    )


def test_feature_vector_shape_and_ranges() -> None:
    products = [
        make_product("a", "cat1", "500 мл", 1000, 10, 1, 0.5, 2),
        make_product("b", "cat2", "1 кг", 1750, 50, 5, 2.5, 10),
        make_product("c", "cat1", "200 г", 1400, 30, 3, 1.5, 7),
    ]

    builder = FeatureBuilder(
        text_model_name="all-MiniLM-L6-v2",
        text_weight=1.2,
        category_weight=3.0,
        nutrition_weight=1.5,
        price_weight=0.8,
        encoder=DummyEncoder(),
    )

    vectors = builder.build(products)

    expected_dim = 384 + 4 + 2 + 1 + 1
    assert vectors.shape == (3, expected_dim)

    nutrition_slice = vectors[:, 384:388]
    assert np.all(nutrition_slice >= 0.0)
    assert np.all(nutrition_slice <= 1.5 + 1e-6)

    price_slice = vectors[:, -1]
    assert np.all(price_slice >= 0.0)
    assert np.all(price_slice <= 0.8 + 1e-6)


def test_parse_weight_handles_common_formats() -> None:
    assert FeatureBuilder._parse_weight("500 мл") == 500.0
    assert FeatureBuilder._parse_weight("1.5 кг") == 1500.0
    assert FeatureBuilder._parse_weight("0.5 кг") == 500.0
    assert FeatureBuilder._parse_weight("200 г") == 200.0
    assert FeatureBuilder._parse_weight("unknown") == 0.0
