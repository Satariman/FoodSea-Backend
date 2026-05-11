from __future__ import annotations

import numpy as np
import pytest

from src.data_loader import ProductData
from src.photo_search.fusion import weighted_fuse
from src.photo_search.build_index import (
    build_photo_index,
    build_photo_index_from_shared_path,
    fetch_image_bytes,
    main,
    meta_from_product,
    product_text,
)
from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow
from src.shared_index.store import save_shared_index


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


def _meta(product_id: str, name: str) -> SharedIndexMeta:
    return SharedIndexMeta(
        product_id=product_id,
        name=name,
        brand_name="Brand",
        category_name="Dairy",
        subcategory_name="Milk",
        image_url=f"https://example/{product_id}.jpg",
    )


def _profile(dimensions: int = 3) -> SharedIndexProfile:
    return SharedIndexProfile(
        provider="fake",
        model="fake-v1",
        dimensions=dimensions,
        index_mode="weighted_multimodal",
        build_weights={"image": 0.5, "name": 0.5},
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


def test_build_photo_index_from_shared_rows_weighted_mode(tmp_path) -> None:
    rows = [
        SharedIndexRow(
            meta=_meta("p-1", "Milk"),
            channels={
                "image": np.array([1.0, 0.0, 0.0], dtype=np.float32),
                "name": np.array([0.0, 1.0, 0.0], dtype=np.float32),
            },
        ),
        SharedIndexRow(
            meta=_meta("p-2", "Yogurt"),
            channels={
                "image": np.array([0.0, 1.0, 0.0], dtype=np.float32),
                "name": np.array([1.0, 0.0, 0.0], dtype=np.float32),
            },
        ),
    ]
    weights = {"image": 0.8, "name": 0.2}

    index = build_photo_index(
        profile=_profile(),
        rows=rows,
        index_path=str(tmp_path / "photo_index_weighted.pkl"),
        index_mode="weighted_multimodal",
        build_weights=weights,
    )

    expected = np.vstack([weighted_fuse(row.channels, weights) for row in rows])
    assert index.product_ids == ["p-1", "p-2"]
    assert index.product_metas()[0].name == "Milk"
    assert index.index_mode == "weighted_multimodal"
    assert index.build_weights == {"image": 0.8, "name": 0.2}
    assert np.allclose(index.vectors, expected, atol=1e-6)


def test_build_photo_index_from_shared_rows_legacy_mode_uses_image_only(tmp_path) -> None:
    rows = [
        SharedIndexRow(
            meta=_meta("p-1", "Milk"),
            channels={
                "image": np.array([0.0, 2.0, 0.0], dtype=np.float32),
                "name": np.array([1.0, 0.0, 0.0], dtype=np.float32),
            },
        ),
        SharedIndexRow(
            meta=_meta("p-2", "Yogurt"),
            channels={"name": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
        ),
    ]

    index = build_photo_index(
        profile=_profile(),
        rows=rows,
        index_path=str(tmp_path / "photo_index_legacy.pkl"),
        index_mode="legacy_image_only",
        build_weights={"image": 1.0, "name": 0.0},
    )

    assert index.product_ids == ["p-1"]
    assert index.index_mode == "legacy_image_only"
    assert np.allclose(index.vectors, np.array([[0.0, 1.0, 0.0]], dtype=np.float32), atol=1e-6)


def test_build_photo_index_from_shared_rows_fails_when_no_rows_match_mode(tmp_path) -> None:
    rows = [
        SharedIndexRow(
            meta=_meta("p-1", "Milk"),
            channels={"name": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
        ),
        SharedIndexRow(
            meta=_meta("p-2", "Yogurt"),
            channels={},
        ),
    ]

    with pytest.raises(ValueError, match="no valid products for photo index rebuild"):
        build_photo_index(
            profile=_profile(),
            rows=rows,
            index_path=str(tmp_path / "photo_index_no_valid_rows.pkl"),
            index_mode="legacy_image_only",
            build_weights={"image": 1.0},
        )


def test_build_photo_index_raises_for_unsupported_index_mode(tmp_path) -> None:
    rows = [
        SharedIndexRow(
            meta=_meta("p-1", "Milk"),
            channels={"image": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
        )
    ]

    with pytest.raises(ValueError, match="unsupported photo index mode"):
        build_photo_index(
            profile=_profile(),
            rows=rows,
            index_path=str(tmp_path / "photo_index_invalid_mode.pkl"),
            index_mode="unknown_mode",
            build_weights={"image": 1.0},
        )


def test_build_photo_index_from_shared_path_uses_loader_rows(tmp_path) -> None:
    shared_path = tmp_path / "shared.pkl"
    photo_path = tmp_path / "photo.pkl"
    profile = _profile()
    rows = [
        SharedIndexRow(
            meta=_meta("p-1", "Milk"),
            channels={"image": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
        )
    ]
    save_shared_index(shared_path, profile, rows)

    index = build_photo_index_from_shared_path(
        shared_index_path=shared_path,
        index_path=str(photo_path),
        index_mode="legacy_image_only",
        build_weights={"image": 1.0},
    )

    assert photo_path.exists()
    assert index.product_ids == ["p-1"]


def test_build_photo_index_from_shared_path_propagates_invalid_shared_payload(tmp_path) -> None:
    shared_path = tmp_path / "shared_corrupted.pkl"
    shared_path.write_bytes(b"not-a-pickle")

    with pytest.raises(ValueError, match="invalid shared index payload"):
        build_photo_index_from_shared_path(
            shared_index_path=shared_path,
            index_path=str(tmp_path / "photo.pkl"),
            index_mode="legacy_image_only",
            build_weights={"image": 1.0},
        )


def test_build_photo_index_from_shared_path_propagates_missing_shared_data(tmp_path) -> None:
    import pickle

    shared_path = tmp_path / "shared_missing_rows.pkl"
    shared_path.write_bytes(
        pickle.dumps(
            {
                "profile": {
                    "provider": "fake",
                    "model": "fake-v1",
                    "dimensions": 3,
                    "index_mode": "weighted_multimodal",
                    "build_weights": {"image": 1.0},
                },
            },
        ),
    )

    with pytest.raises(ValueError, match="invalid rows"):
        build_photo_index_from_shared_path(
            shared_index_path=shared_path,
            index_path=str(tmp_path / "photo.pkl"),
            index_mode="legacy_image_only",
            build_weights={"image": 1.0},
        )


def test_main_reads_shared_index_and_writes_photo_index_without_provider_calls(
    monkeypatch,
    tmp_path,
) -> None:
    shared_path = tmp_path / "shared.pkl"
    photo_path = tmp_path / "photo.pkl"
    profile = _profile()
    rows = [
        SharedIndexRow(
            meta=_meta("p-1", "Milk"),
            channels={
                "image": np.array([1.0, 0.0, 0.0], dtype=np.float32),
                "name": np.array([0.0, 1.0, 0.0], dtype=np.float32),
            },
        ),
    ]
    save_shared_index(shared_path, profile, rows)

    class FakeConfig:
        SHARED_INDEX_PATH = str(shared_path)
        PHOTO_SEARCH_INDEX_PATH = str(photo_path)
        PHOTO_SEARCH_INDEX_MODE = "weighted_multimodal"
        PHOTO_SEARCH_BUILD_WEIGHT_IMAGE = 0.7
        PHOTO_SEARCH_BUILD_WEIGHT_NAME = 0.3
        PHOTO_SEARCH_BUILD_WEIGHT_BRAND = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT = 0.0
        PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT = 0.0

    monkeypatch.setattr("src.photo_search.build_index.Config", FakeConfig)

    main()

    assert photo_path.exists()


def test_fetch_image_bytes_percent_encodes_non_ascii_url(monkeypatch) -> None:
    captured: dict[str, str] = {}

    class _Response:
        headers = {"Content-Type": "image/png"}

        def read(self) -> bytes:
            return b"img-bytes"

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb) -> None:  # noqa: ANN001
            return None

    def _urlopen(req, timeout):  # noqa: ANN001
        captured["url"] = req.full_url
        return _Response()

    monkeypatch.setattr("src.photo_search.build_index.request.urlopen", _urlopen)

    body, mime_type = fetch_image_bytes(
        "http://localhost:9000/product-images/products/x/сметана-домик-в-деревне-20-315-г.png"
    )

    assert body == b"img-bytes"
    assert mime_type == "image/png"
    assert "сметана" not in captured["url"]
    assert "%D1%81%D0%BC%D0%B5%D1%82%D0%B0%D0%BD%D0%B0" in captured["url"]
