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

    def embed_multimodal(self, items):  # pragma: no cover - overridden where needed
        rows = []
        for item in items:
            seed = len(str(item.get("text") or "")) + len(bytes(item.get("image_bytes") or b""))
            row = np.zeros((self.dimensions,), dtype=np.float32)
            row[seed % self.dimensions] = 1.0
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
        index_mode="weighted_multimodal",
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
        index_mode="weighted_multimodal",
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

    assert index_path.exists()
    assert index.product_ids == ["p-valid"]
    assert provider.calls == [["Yogurt"]]


def test_build_photo_index_fails_when_all_products_have_empty_text(tmp_path) -> None:
    provider = FakeProvider(dimensions=4)
    products = [
        _product("p-empty-1", name=" ", brand_name=" ", category_name=" ", subcategory_name=" ", description=" ", composition=" ", weight=" "),
        _product("p-empty-2", name="\t", brand_name="\n", category_name=" ", subcategory_name=" ", description=" ", composition=" ", weight=" "),
    ]
    index_path = tmp_path / "photo_index_empty.pkl"

    with pytest.raises(ValueError, match="no valid products for photo index rebuild"):
        build_photo_index(
            products=products,
            provider=provider,
            index_path=str(index_path),
            batch_size=8,
            index_mode="weighted_multimodal",
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


def test_build_photo_index_weighted_multimodal_fallback_to_text_when_image_unavailable(
    monkeypatch, tmp_path
) -> None:
    class FailingImageProvider(FakeProvider):
        def embed_multimodal(self, items):
            raise RuntimeError("image embedding failed")

    provider = FailingImageProvider(dimensions=4)
    products = [_product("p-1"), _product("p-2", name="Yogurt")]
    index_path = tmp_path / "photo_index_weighted.pkl"
    monkeypatch.setattr(
        "src.photo_search.rebuild_index.fetch_image_bytes",
        lambda url: (_ for _ in ()).throw(RuntimeError("download error")),
    )

    index = build_photo_index(
        products=products,
        provider=provider,
        index_path=str(index_path),
        batch_size=2,
        index_mode="weighted_multimodal",
        build_weights={
            "image": 0.4,
            "name": 0.6,
            "brand": 0.0,
            "category": 0.0,
            "subcategory": 0.0,
            "description": 0.0,
            "composition": 0.0,
            "weight": 0.0,
            "full_text": 0.0,
        },
    )

    assert index.product_ids == ["p-1", "p-2"]
    assert index_path.exists()


def test_build_photo_index_weighted_multimodal_skips_product_without_any_channels(tmp_path) -> None:
    provider = FakeProvider(dimensions=4)
    products = [
        _product(
            "p-invalid",
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
    ]
    index = build_photo_index(
        products=products,
        provider=provider,
        index_path=str(tmp_path / "photo_index_skip_invalid.pkl"),
        batch_size=4,
        index_mode="weighted_multimodal",
        build_weights={
            "image": 0.1,
            "name": 0.9,
            "brand": 0.0,
            "category": 0.0,
            "subcategory": 0.0,
            "description": 0.0,
            "composition": 0.0,
            "weight": 0.0,
            "full_text": 0.0,
        },
    )
    assert index.product_ids == ["p-valid"]


def test_build_photo_index_legacy_image_only_path(monkeypatch, tmp_path) -> None:
    class LegacyProvider(FakeProvider):
        def __init__(self, dimensions: int = 4) -> None:
            super().__init__(dimensions=dimensions)
            self.mm_calls = 0

        def embed_multimodal(self, items):
            self.mm_calls += 1
            rows = []
            for i, _ in enumerate(items):
                row = np.zeros((self.dimensions,), dtype=np.float32)
                row[i % self.dimensions] = 1.0
                rows.append(row)
            return np.stack(rows, axis=0)

    provider = LegacyProvider(dimensions=4)
    monkeypatch.setattr(
        "src.photo_search.rebuild_index.fetch_image_bytes",
        lambda url: (b"img", "image/jpeg"),
    )
    products = [_product("p-1"), _product("p-2", name="Yogurt")]
    index = build_photo_index(
        products=products,
        provider=provider,
        index_path=str(tmp_path / "photo_index_legacy.pkl"),
        batch_size=1,
        index_mode="legacy_image_only",
    )
    assert index.product_ids == ["p-1", "p-2"]
    assert provider.mm_calls == 2


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
        PHOTO_SEARCH_INDEX_MODE = "weighted_multimodal"
        PHOTO_SEARCH_BUILD_WEIGHT_IMAGE = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_NAME = 1.0
        PHOTO_SEARCH_BUILD_WEIGHT_BRAND = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT = 0.0

    fake_provider = FakeProvider(dimensions=4)

    monkeypatch.setattr("src.photo_search.rebuild_index.DataLoader", FakeLoader)
    monkeypatch.setattr("src.photo_search.rebuild_index.Config", FakeConfig)
    monkeypatch.setattr(
        "src.photo_search.rebuild_index.provider_from_config",
        lambda cfg: fake_provider,
    )

    main()

    assert (tmp_path / "rebuilt.pkl").exists()
