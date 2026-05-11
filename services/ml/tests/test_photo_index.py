from __future__ import annotations

import pickle

import numpy as np
import pytest

from src.photo_search.index import PhotoProductIndex, PhotoProductMeta


def build_index() -> PhotoProductIndex:
    metas = [
        PhotoProductMeta(product_id="a", image_url="a.jpg"),
        PhotoProductMeta(product_id="b", image_url="b.jpg"),
        PhotoProductMeta(product_id="c", image_url="c.jpg"),
        PhotoProductMeta(product_id="d", image_url="d.jpg"),
    ]
    vectors = np.array(
        [
            [1.0, 0.0, 0.0],
            [0.9, 0.1, 0.0],
            [0.5, 0.5, 0.0],
            [0.0, 1.0, 0.0],
        ],
        dtype=np.float32,
    )
    index = PhotoProductIndex(provider="clip", model="ViT-B/32")
    index.build(metas=metas, vectors=vectors)
    return index


def test_query_returns_sorted_candidates() -> None:
    index = build_index()
    query = np.array([1.0, 0.0, 0.0], dtype=np.float32)

    results = index.query(query_vector=query, top_k=3)

    assert [r.product_id for r in results] == ["a", "b", "c"]
    assert [r.score for r in results] == sorted([r.score for r in results], reverse=True)


def test_save_load_roundtrip(tmp_path) -> None:
    index = build_index()
    query = np.array([0.8, 0.2, 0.0], dtype=np.float32)
    expected = index.query(query_vector=query, top_k=3)
    path = tmp_path / "photo_index.pkl"

    index.save(str(path))

    restored = PhotoProductIndex(provider="clip", model="ViT-B/32")
    restored.load(str(path))
    actual = restored.query(query_vector=query, top_k=3)

    assert [r.product_id for r in actual] == [r.product_id for r in expected]
    assert np.allclose([r.score for r in actual], [r.score for r in expected], atol=1e-6)
    assert restored.product_metas == index.product_metas


def test_load_rejects_incompatible_metadata(tmp_path) -> None:
    index = build_index()
    path = tmp_path / "photo_index.pkl"
    index.save(str(path))

    mismatched_provider = PhotoProductIndex(provider="openclip", model="ViT-B/32")
    with pytest.raises(ValueError, match="provider"):
        mismatched_provider.load(str(path))

    mismatched_model = PhotoProductIndex(provider="clip", model="ViT-L/14")
    with pytest.raises(ValueError, match="model"):
        mismatched_model.load(str(path))

    payload = pickle.loads(path.read_bytes())
    payload["dimensions"] = int(payload["dimensions"]) + 1
    path.write_bytes(pickle.dumps(payload))

    mismatched_dimensions = PhotoProductIndex(provider="clip", model="ViT-B/32")
    with pytest.raises(ValueError, match="dimensions"):
        mismatched_dimensions.load(str(path))
