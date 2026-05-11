from __future__ import annotations

import numpy as np
import pytest

from src.data_loader import ProductData
from src.photo_search.embeddings import GeminiAPIEmbeddingProvider, ProviderNotConfiguredError
from src.photo_search.rebuild_index import (
    build_photo_index,
    main,
    meta_from_product,
    product_text,
    provider_from_config,
)


class FakeProvider:
    def __init__(self, dimensions: int = 4) -> None:
        self.provider_name = "fake"
        self.model = "fake-v1"
        self.dimensions = dimensions
        self.calls: list[list[str]] = []

    def embed_texts(self, texts: list[str]) -> np.ndarray:
        self.calls.append(list(texts))
        rows = []
        for i, _ in enumerate(texts):
            row = np.zeros((self.dimensions,), dtype=np.float32)
            row[i % self.dimensions] = 1.0
            rows.append(row)
        return np.stack(rows, axis=0)

    def embed_multimodal(self, items):  # pragma: no cover - not used in rebuild flow
        raise NotImplementedError


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


def test_product_text_and_meta_shape() -> None:
    product = _product("p-1")

    text = product_text(product)
    meta = meta_from_product(product)

    assert "Milk 3.2%" in text
    assert "Brand" in text
    assert "Ultra pasteurized" in text
    assert meta.product_id == "p-1"
    assert meta.image_url == "https://example/milk.jpg"


def test_build_photo_index_with_fake_provider(tmp_path) -> None:
    provider = FakeProvider(dimensions=4)
    products = [_product("p-1"), _product("p-2", name="Yogurt")]
    index_path = tmp_path / "photo_index.pkl"

    index = build_photo_index(
        products=products,
        provider=provider,
        index_path=str(index_path),
        batch_size=1,
    )

    assert index_path.exists()
    assert index.provider == "fake"
    assert index.model == "fake-v1"
    assert index.dimensions == 4
    assert index.product_ids == ["p-1", "p-2"]
    assert provider.calls and len(provider.calls) == 2


def test_build_photo_index_skips_products_with_empty_text(tmp_path) -> None:
    provider = FakeProvider(dimensions=4)
    products = [
        _product("p-empty", name=" ", brand_name=" ", category_name=" ", subcategory_name=" ", description=" ", composition=" ", weight=" "),
        _product("p-valid", name="Yogurt"),
    ]
    index_path = tmp_path / "photo_index_skip_empty.pkl"

    index = build_photo_index(
        products=products,
        provider=provider,
        index_path=str(index_path),
        batch_size=16,
    )

    assert index_path.exists()
    assert index.product_ids == ["p-valid"]
    assert provider.calls == [["Yogurt | Brand | Dairy | Milk | Ultra pasteurized | Milk | 900 ml"]]


def test_build_photo_index_fails_when_all_products_have_empty_text(tmp_path) -> None:
    provider = FakeProvider(dimensions=4)
    products = [
        _product("p-empty-1", name=" ", brand_name=" ", category_name=" ", subcategory_name=" ", description=" ", composition=" ", weight=" "),
        _product("p-empty-2", name="\t", brand_name="\n", category_name=" ", subcategory_name=" ", description=" ", composition=" ", weight=" "),
    ]
    index_path = tmp_path / "photo_index_empty.pkl"

    with pytest.raises(ValueError, match="all products have empty text"):
        build_photo_index(
            products=products,
            provider=provider,
            index_path=str(index_path),
            batch_size=8,
        )


def test_provider_from_config_gemini_requires_key() -> None:
    class Cfg:
        PHOTO_SEARCH_PROVIDER = "gemini_api_key"
        GEMINI_API_KEY = None
        PHOTO_SEARCH_MODEL = "gemini-embedding-2"
        PHOTO_SEARCH_DIMENSIONS = 32
        VERTEX_PROJECT_ID = "project"
        VERTEX_LOCATION = "us-central1"

    with pytest.raises(ProviderNotConfiguredError):
        provider_from_config(Cfg())


def test_provider_from_config_gemini_ok() -> None:
    class Cfg:
        PHOTO_SEARCH_PROVIDER = "gemini_api_key"
        GEMINI_API_KEY = "secret"
        PHOTO_SEARCH_MODEL = "gemini-embedding-2"
        PHOTO_SEARCH_DIMENSIONS = 32
        VERTEX_PROJECT_ID = "project"
        VERTEX_LOCATION = "us-central1"

    provider = provider_from_config(Cfg())
    assert isinstance(provider, GeminiAPIEmbeddingProvider)
    assert provider.model == "gemini-embedding-2"


def test_main_loads_products_and_saves_index(monkeypatch, tmp_path) -> None:
    class FakeLoader:
        def __init__(self, addr: str) -> None:
            self.addr = addr

        def load_products(self):
            return [_product("p-1"), _product("p-2")]

    class FakeConfig:
        CORE_GRPC_ADDR = "localhost:9091"
        PHOTO_SEARCH_INDEX_PATH = str(tmp_path / "rebuilt.pkl")
        PHOTO_SEARCH_BATCH_SIZE = 2

    fake_provider = FakeProvider(dimensions=4)

    monkeypatch.setattr("src.photo_search.rebuild_index.DataLoader", FakeLoader)
    monkeypatch.setattr("src.photo_search.rebuild_index.Config", FakeConfig)
    monkeypatch.setattr(
        "src.photo_search.rebuild_index.provider_from_config",
        lambda cfg: fake_provider,
    )

    main()

    assert (tmp_path / "rebuilt.pkl").exists()
