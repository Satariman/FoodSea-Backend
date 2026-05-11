from __future__ import annotations

from pathlib import Path

import numpy as np

from src.data_loader import ProductData
from src.shared_index.builder import build_shared_index
from src.shared_index.store import load_shared_index, save_shared_index


class FakeProvider:
    def __init__(self, dimensions: int = 4) -> None:
        self.provider_name = "fake"
        self.model = "fake-v1"
        self.dimensions = dimensions
        self.text_calls: list[list[str]] = []
        self.mm_calls: list[list[dict[str, object]]] = []

    def embed_texts(self, texts: list[str]) -> np.ndarray:
        self.text_calls.append(list(texts))
        rows = []
        for i, _ in enumerate(texts):
            row = np.zeros((self.dimensions,), dtype=np.float32)
            row[i % self.dimensions] = 1.0
            rows.append(row)
        return np.stack(rows, axis=0) if rows else np.empty((0, self.dimensions), dtype=np.float32)

    def embed_multimodal(self, items: list[dict[str, object]]) -> np.ndarray:
        self.mm_calls.append(list(items))
        rows = []
        for i, _ in enumerate(items):
            row = np.zeros((self.dimensions,), dtype=np.float32)
            row[(i + 1) % self.dimensions] = 1.0
            rows.append(row)
        return np.stack(rows, axis=0) if rows else np.empty((0, self.dimensions), dtype=np.float32)


def _product(
    product_id: str,
    name: str = "Milk 3.2%",
    brand_name: str = "Brand",
    category_name: str = "Dairy",
    subcategory_name: str = "Milk",
    description: str = "Ultra pasteurized",
    composition: str = "Milk",
    weight: str = "900 ml",
    image_url: str = "https://example/milk.jpg",
) -> ProductData:
    return ProductData(
        product_id=product_id,
        name=name,
        description=description,
        composition=composition,
        category_id="c1",
        subcategory_id="s1",
        brand_id="b1",
        weight=weight,
        calories=50.0,
        protein=3.0,
        fat=3.2,
        carbohydrates=4.7,
        offers={"store-1": 10000},
        min_price_kopecks=10000,
        category_name=category_name,
        subcategory_name=subcategory_name,
        brand_name=brand_name,
        image_url=image_url,
    )


def _weights(image: float = 0.0) -> dict[str, float]:
    return {
        "image": image,
        "name": 1.0,
        "brand": 1.0,
        "category": 1.0,
        "subcategory": 1.0,
        "description": 1.0,
        "composition": 1.0,
        "weight": 1.0,
        "full_text": 1.0,
    }


def test_build_shared_index_writes_profile_and_rows_with_expected_channels(monkeypatch, tmp_path: Path) -> None:
    provider = FakeProvider(dimensions=4)
    monkeypatch.setattr("src.shared_index.builder.fetch_image_bytes", lambda _url: (b"img", "image/jpeg"))

    profile, rows = build_shared_index(
        products=[_product("p-1")],
        provider=provider,
        batch_size=8,
        build_weights=_weights(image=0.25),
    )

    assert profile.provider == "fake"
    assert profile.model == "fake-v1"
    assert profile.dimensions == 4
    assert len(rows) == 1
    assert rows[0].meta.product_id == "p-1"
    assert set(rows[0].channels) == {
        "name",
        "brand",
        "category",
        "subcategory",
        "description",
        "composition",
        "weight",
        "full_text",
        "image",
    }

    path = tmp_path / "shared_index.pkl"
    save_shared_index(path, profile, rows)
    loaded_profile, loaded_rows = load_shared_index(path)

    assert loaded_profile == profile
    assert loaded_rows[0].meta.product_id == "p-1"
    assert set(loaded_rows[0].channels) == set(rows[0].channels)


def test_build_shared_index_skips_invalid_or_empty_products() -> None:
    provider = FakeProvider(dimensions=4)

    profile, rows = build_shared_index(
        products=[
            _product("", name="Should skip by id"),
            _product(
                "p-empty",
                name=" ",
                brand_name=" ",
                category_name=" ",
                subcategory_name=" ",
                description=" ",
                composition=" ",
                weight=" ",
                image_url="",
            ),
            _product("p-valid", name="Yogurt"),
        ],
        provider=provider,
        batch_size=4,
        build_weights={
            "image": 0.0,
            "name": 1.0,
            "brand": 0.0,
            "category": 0.0,
            "subcategory": 0.0,
            "description": 0.0,
            "composition": 0.0,
            "weight": 0.0,
            "full_text": 0.0,
        },
    )

    assert profile.provider == "fake"
    assert [row.meta.product_id for row in rows] == ["p-valid"]
    assert provider.text_calls == [["Yogurt"]]


def test_build_shared_index_adds_image_channel_only_when_image_weight_is_positive(monkeypatch) -> None:
    provider = FakeProvider(dimensions=4)
    fetch_calls: list[str] = []

    def _fetch(url: str):
        fetch_calls.append(url)
        return b"img", "image/jpeg"

    monkeypatch.setattr("src.shared_index.builder.fetch_image_bytes", _fetch)

    _, rows_no_image = build_shared_index(
        products=[_product("p-1")],
        provider=provider,
        batch_size=4,
        build_weights=_weights(image=0.0),
    )
    assert "image" not in rows_no_image[0].channels
    assert fetch_calls == []

    _, rows_with_image = build_shared_index(
        products=[_product("p-2")],
        provider=provider,
        batch_size=4,
        build_weights=_weights(image=0.4),
    )
    assert "image" in rows_with_image[0].channels
    assert fetch_calls == ["https://example/milk.jpg"]
