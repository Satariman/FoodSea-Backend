from __future__ import annotations

import numpy as np

from src.photo_search.index import PhotoProductIndex, PhotoProductMeta


def build_index() -> PhotoProductIndex:
    metas = [
        PhotoProductMeta("a", "A", "Brand A", "Cat 1", "Sub 1", "a.jpg"),
        PhotoProductMeta("b", "B", "Brand B", "Cat 1", "Sub 1", "b.jpg"),
        PhotoProductMeta("c", "C", "Brand C", "Cat 2", "Sub 2", "c.jpg"),
        PhotoProductMeta("d", "D", "Brand D", "Cat 2", "Sub 2", "d.jpg"),
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
    index = PhotoProductIndex()
    index.build(
        metas=metas,
        vectors=vectors,
        provider="clip",
        model="ViT-B/32",
        dimensions=3,
    )
    return index


def test_query_returns_sorted_candidates() -> None:
    index = build_index()
    query = np.array([1.0, 0.0, 0.0], dtype=np.float32)

    results = index.query(query_vector=query, top_k=3, min_score=0.0)

    assert [r.product_id for r in results] == ["a", "b", "c"]
    assert [r.score for r in results] == sorted([r.score for r in results], reverse=True)
    assert results[0].meta.name == "A"


def test_save_load_roundtrip(tmp_path) -> None:
    index = build_index()
    query = np.array([0.8, 0.2, 0.0], dtype=np.float32)
    expected = index.query(query_vector=query, top_k=3)
    path = tmp_path / "photo_index.pkl"

    index.save(str(path))

    restored = PhotoProductIndex()
    loaded = restored.load(
        str(path),
        provider="clip",
        model="ViT-B/32",
        dimensions=3,
    )
    actual = restored.query(query_vector=query, top_k=3)

    assert loaded is True
    assert [r.product_id for r in actual] == [r.product_id for r in expected]
    assert np.allclose([r.score for r in actual], [r.score for r in expected], atol=1e-6)
    assert restored.product_metas() == index.product_metas()


def test_load_rejects_incompatible_metadata(tmp_path) -> None:
    index = build_index()
    path = tmp_path / "photo_index.pkl"
    index.save(str(path))

    mismatched_provider = PhotoProductIndex()
    assert (
        mismatched_provider.load(
            str(path), provider="openclip", model="ViT-B/32", dimensions=3
        )
        is False
    )

    mismatched_model = PhotoProductIndex()
    assert (
        mismatched_model.load(str(path), provider="clip", model="ViT-L/14", dimensions=3)
        is False
    )

    mismatched_dimensions = PhotoProductIndex()
    assert (
        mismatched_dimensions.load(
            str(path), provider="clip", model="ViT-B/32", dimensions=4
        )
        is False
    )
