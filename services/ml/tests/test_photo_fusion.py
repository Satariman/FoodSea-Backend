from __future__ import annotations

import numpy as np
import pytest

from src.photo_search.fusion import (
    renormalize_weights_for_present_channels,
    weighted_fuse,
)


def test_weighted_fuse_normal_fusion() -> None:
    vectors = {
        "image": np.array([1.0, 0.0], dtype=np.float32),
        "text": np.array([0.0, 1.0], dtype=np.float32),
    }
    weights = {"image": 0.75, "text": 0.25}

    fused = weighted_fuse(vectors, weights)
    expected = np.array([0.75, 0.25], dtype=np.float32)
    expected /= np.linalg.norm(expected)

    assert np.allclose(fused, expected, atol=1e-6)


def test_weight_renormalization_for_missing_channel() -> None:
    normalized = renormalize_weights_for_present_channels(
        weights={"image": 0.6, "text": 0.3, "ocr": 0.1},
        present_channels={"image", "text"},
    )

    assert set(normalized.keys()) == {"image", "text"}
    assert normalized["image"] == pytest.approx(2.0 / 3.0)
    assert normalized["text"] == pytest.approx(1.0 / 3.0)
    assert sum(normalized.values()) == pytest.approx(1.0)


def test_renormalize_rejects_zero_or_invalid_weights() -> None:
    with pytest.raises(ValueError, match="No positive weights"):
        renormalize_weights_for_present_channels(
            weights={"image": 0.0, "text": -1.0},
            present_channels={"image", "text"},
        )

    with pytest.raises(ValueError, match="No positive weights"):
        renormalize_weights_for_present_channels(
            weights={"image": 0.9},
            present_channels={"text"},
        )


def test_weighted_fuse_rejects_dimension_mismatch() -> None:
    vectors = {
        "image": np.array([1.0, 0.0], dtype=np.float32),
        "text": np.array([0.2, 0.3, 0.4], dtype=np.float32),
    }

    with pytest.raises(ValueError, match="same dimension"):
        weighted_fuse(vectors, {"image": 0.5, "text": 0.5})


def test_weighted_fuse_returns_l2_normalized_vector() -> None:
    vectors = {
        "image": np.array([3.0, 4.0, 0.0], dtype=np.float32),
        "text": np.array([0.0, 0.0, 5.0], dtype=np.float32),
    }
    fused = weighted_fuse(vectors, {"image": 0.5, "text": 0.5})

    assert np.linalg.norm(fused) == pytest.approx(1.0, abs=1e-6)


def test_renormalize_rejects_empty_present_channels() -> None:
    with pytest.raises(ValueError, match="present_channels must not be empty"):
        renormalize_weights_for_present_channels(weights={"image": 1.0}, present_channels=set())


@pytest.mark.parametrize("bad_weight", [np.nan, np.inf, -np.inf])
def test_renormalize_rejects_non_finite_weights(bad_weight: float) -> None:
    with pytest.raises(ValueError, match="must be finite"):
        renormalize_weights_for_present_channels(
            weights={"image": bad_weight},
            present_channels={"image"},
        )


def test_weighted_fuse_rejects_empty_vectors_map() -> None:
    with pytest.raises(ValueError, match="must not be empty"):
        weighted_fuse({}, {"image": 1.0})


def test_weighted_fuse_rejects_non_1d_vectors() -> None:
    with pytest.raises(ValueError, match="must be 1D"):
        weighted_fuse(
            {"image": np.array([[1.0, 2.0]], dtype=np.float32)},
            {"image": 1.0},
        )


@pytest.mark.parametrize("bad_value", [np.nan, np.inf, -np.inf])
def test_weighted_fuse_rejects_vectors_with_non_finite_values(bad_value: float) -> None:
    with pytest.raises(ValueError, match="contains NaN/Inf"):
        weighted_fuse(
            {
                "image": np.array([1.0, bad_value], dtype=np.float32),
                "text": np.array([0.0, 1.0], dtype=np.float32),
            },
            {"image": 0.5, "text": 0.5},
        )


def test_weighted_fuse_zero_norm_cancellation_raises() -> None:
    with pytest.raises(ValueError, match="zero norm"):
        weighted_fuse(
            {
                "image": np.array([1.0, -2.0, 3.0], dtype=np.float32),
                "text": np.array([-1.0, 2.0, -3.0], dtype=np.float32),
            },
            {"image": 0.5, "text": 0.5},
        )
