from pathlib import Path

import numpy as np
import pickle
import pytest

from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow
from src.shared_index.store import load_shared_index, save_shared_index


def _sample_profile() -> SharedIndexProfile:
    return SharedIndexProfile(
        provider="gemini",
        model="text-embedding-004",
        dimensions=3,
        index_mode="multi_channel",
        build_weights={"name": 0.8, "brand": 0.2},
    )


def _sample_rows() -> list[SharedIndexRow]:
    return [
        SharedIndexRow(
            meta=SharedIndexMeta(
                product_id="p-1",
                name="Milk 1L",
                brand_name="Acme",
                category_name="Dairy",
                subcategory_name="Milk",
                image_url="https://example.com/1.png",
            ),
            channels={
                "name": np.array([1.0, 0.0, 0.0], dtype=np.float32),
                "brand": np.array([0.0, 1.0, 0.0], dtype=np.float32),
            },
        ),
        SharedIndexRow(
            meta=SharedIndexMeta(
                product_id="p-2",
                name="Bread",
                brand_name="Baker",
                category_name="Bakery",
                subcategory_name="Bread",
                image_url="https://example.com/2.png",
            ),
            channels={
                "name": np.array([0.0, 0.0, 1.0], dtype=np.float32),
                "brand": np.array([1.0, 1.0, 0.0], dtype=np.float32),
            },
        ),
    ]


def test_save_load_roundtrip(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    path = tmp_path / "shared_index.pkl"

    save_shared_index(path, profile, rows)
    loaded_profile, loaded_rows = load_shared_index(path)

    assert loaded_profile == profile
    assert [row.meta.product_id for row in loaded_rows] == ["p-1", "p-2"]
    np.testing.assert_allclose(loaded_rows[0].channels["name"], rows[0].channels["name"])
    np.testing.assert_allclose(loaded_rows[1].channels["brand"], rows[1].channels["brand"])


def test_save_fails_for_non_finite_vector(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    rows[0].channels["name"] = np.array([1.0, np.nan, 0.0], dtype=np.float32)

    with pytest.raises(ValueError, match="finite"):
        save_shared_index(tmp_path / "shared_index.pkl", profile, rows)


def test_save_fails_for_dimension_mismatch(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    rows[1].channels["brand"] = np.array([1.0, 2.0], dtype=np.float32)

    with pytest.raises(ValueError, match="dimensions mismatch"):
        save_shared_index(tmp_path / "shared_index.pkl", profile, rows)


def test_save_fails_for_empty_rows(tmp_path: Path):
    profile = _sample_profile()

    with pytest.raises(ValueError, match="must not be empty"):
        save_shared_index(tmp_path / "shared_index.pkl", profile, [])


def test_load_rejects_invalid_meta_field_type(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    path = tmp_path / "shared_index.pkl"
    save_shared_index(path, profile, rows)

    payload = pickle.loads(path.read_bytes())
    payload["rows"][0]["meta"]["name"] = 123
    path.write_bytes(pickle.dumps(payload))

    with pytest.raises(ValueError, match="invalid row meta field: name"):
        load_shared_index(path)


def test_load_rejects_bool_dimensions(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    path = tmp_path / "shared_index.pkl"
    save_shared_index(path, profile, rows)

    payload = pickle.loads(path.read_bytes())
    payload["profile"]["dimensions"] = True
    path.write_bytes(pickle.dumps(payload))

    with pytest.raises(ValueError, match="invalid profile dimensions"):
        load_shared_index(path)


def test_load_rejects_unexpected_pickle_global(tmp_path: Path):
    class _Evil:
        def __reduce__(self):
            return (eval, ("1 + 1",))

    path = tmp_path / "shared_index.pkl"
    path.write_bytes(pickle.dumps(_Evil()))

    with pytest.raises(ValueError, match="invalid shared index payload"):
        load_shared_index(path)


def test_load_rejects_truncated_pickle(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    path = tmp_path / "shared_index.pkl"
    save_shared_index(path, profile, rows)

    raw = path.read_bytes()
    path.write_bytes(raw[:-1])

    with pytest.raises(ValueError, match="invalid shared index payload"):
        load_shared_index(path)


def test_save_rejects_invalid_build_weights_value_type(tmp_path: Path):
    profile = _sample_profile()
    profile.build_weights["name"] = object()

    with pytest.raises(ValueError, match=r"profile\.build_weights values must be finite"):
        save_shared_index(tmp_path / "shared_index.pkl", profile, _sample_rows())


def test_load_rejects_invalid_build_weights_value_type(tmp_path: Path):
    profile = _sample_profile()
    rows = _sample_rows()
    path = tmp_path / "shared_index.pkl"
    save_shared_index(path, profile, rows)

    payload = pickle.loads(path.read_bytes())
    payload["profile"]["build_weights"]["name"] = []
    path.write_bytes(pickle.dumps(payload))

    with pytest.raises(ValueError, match="invalid build_weights value"):
        load_shared_index(path)
