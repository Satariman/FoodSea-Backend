import numpy as np
import pytest

from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow
from src.voice.build_index import build_voice_index
from src.voice.index import VoiceIndex


def _meta(product_id: str, name: str) -> SharedIndexMeta:
    return SharedIndexMeta(
        product_id=product_id,
        name=name,
        brand_name="Brand",
        category_name="Category",
        subcategory_name="Subcategory",
        image_url="",
    )


def _profile(dimensions: int = 2) -> SharedIndexProfile:
    return SharedIndexProfile(
        provider="gemini_api_key",
        model="gemini-embedding-2",
        dimensions=dimensions,
        index_mode="weighted_multimodal",
        build_weights={"name": 0.7, "brand": 0.3, "image": 0.2},
    )


def test_build_voice_index_fuses_shared_channels_with_profile_weights() -> None:
    profile = _profile()
    rows = [
        SharedIndexRow(
            meta=_meta("a", "Milk"),
            channels={
                "name": np.array([1.0, 0.0], dtype=np.float32),
                "brand": np.array([0.0, 1.0], dtype=np.float32),
            },
        ),
        SharedIndexRow(
            meta=_meta("b", "Bread"),
            channels={
                "name": np.array([0.0, 1.0], dtype=np.float32),
            },
        ),
    ]

    index = build_voice_index(profile, rows)

    assert isinstance(index, VoiceIndex)
    assert index.product_ids == ["a", "b"]
    assert index.product_names == ["Milk", "Bread"]

    expected_a = np.array([0.7, 0.3], dtype=np.float32)
    expected_a /= np.linalg.norm(expected_a)
    expected_b = np.array([0.0, 1.0], dtype=np.float32)
    np.testing.assert_allclose(index.vectors[0], expected_a, atol=1e-6)
    np.testing.assert_allclose(index.vectors[1], expected_b, atol=1e-6)


def test_build_voice_index_skips_rows_without_valid_channel_vectors() -> None:
    profile = _profile(dimensions=3)
    rows = [
        SharedIndexRow(
            meta=_meta("a", "Milk"),
            channels={"name": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
        ),
        SharedIndexRow(
            meta=_meta("b", "Broken"),
            channels={"name": np.array([1.0, 0.0], dtype=np.float32)},
        ),
        SharedIndexRow(
            meta=_meta("c", "NaN"),
            channels={"name": np.array([np.nan, 0.0, 1.0], dtype=np.float32)},
        ),
    ]

    index = build_voice_index(profile, rows)
    assert index.product_ids == ["a"]
    assert index.product_names == ["Milk"]


def test_build_voice_index_raises_when_no_rows_can_be_indexed() -> None:
    profile = _profile(dimensions=2)
    rows = [
        SharedIndexRow(
            meta=_meta("x", "Bad"),
            channels={"name": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
        )
    ]

    with pytest.raises(ValueError, match="no valid products for voice index rebuild"):
        build_voice_index(profile, rows)


@pytest.mark.parametrize("dimensions", [0, -1])
def test_build_voice_index_raises_for_non_positive_profile_dimensions(dimensions: int) -> None:
    profile = _profile(dimensions=dimensions)
    rows = [
        SharedIndexRow(
            meta=_meta("a", "Milk"),
            channels={"name": np.array([1.0, 0.0], dtype=np.float32)},
        )
    ]

    with pytest.raises(ValueError, match="shared profile dimensions must be positive"):
        build_voice_index(profile, rows)


@pytest.mark.parametrize(
    "build_weights, expected_error",
    [
        ({"name": 0.0, "brand": 0.0}, "No positive weights for present channels"),
        ({"name": float("nan")}, "must be finite"),
    ],
)
def test_build_voice_index_weighted_fuse_weight_errors_fail_fast(
    build_weights: dict[str, float],
    expected_error: str,
) -> None:
    profile = SharedIndexProfile(
        provider="gemini_api_key",
        model="gemini-embedding-2",
        dimensions=2,
        index_mode="weighted_multimodal",
        build_weights=build_weights,
    )
    rows = [
        SharedIndexRow(
            meta=_meta("a", "Milk"),
            channels={"name": np.array([1.0, 0.0], dtype=np.float32)},
        )
    ]

    with pytest.raises(ValueError, match=expected_error):
        build_voice_index(profile, rows)
