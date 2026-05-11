from __future__ import annotations

from typing import Dict, Set

import numpy as np


def renormalize_weights_for_present_channels(
    weights: Dict[str, float], present_channels: Set[str]
) -> Dict[str, float]:
    if not present_channels:
        raise ValueError("present_channels must not be empty")

    filtered: Dict[str, float] = {}
    for channel in present_channels:
        weight = float(weights.get(channel, 0.0))
        if not np.isfinite(weight):
            raise ValueError(
                f"Weight for channel '{channel}' must be finite, got {weight!r}"
            )
        if weight > 0.0:
            filtered[channel] = weight

    if not filtered:
        raise ValueError("No positive weights for present channels")

    total = sum(filtered.values())
    if not np.isfinite(total) or total <= 0.0:
        raise ValueError(
            f"Sum of positive weights must be finite and > 0, got {total!r}"
        )
    return {channel: weight / total for channel, weight in filtered.items()}


def weighted_fuse(
    vectors_by_channel: Dict[str, np.ndarray], weights: Dict[str, float]
) -> np.ndarray:
    if not vectors_by_channel:
        raise ValueError("vectors_by_channel must not be empty")

    vectors_1d: Dict[str, np.ndarray] = {}
    expected_dim: int | None = None
    for channel, vector in vectors_by_channel.items():
        arr = np.asarray(vector, dtype=np.float32)
        if arr.ndim != 1:
            raise ValueError(
                f"Vector for channel '{channel}' must be 1D, got shape {arr.shape}"
            )
        if not np.all(np.isfinite(arr)):
            raise ValueError(f"Vector for channel '{channel}' contains NaN/Inf values")
        if expected_dim is None:
            expected_dim = arr.shape[0]
        elif arr.shape[0] != expected_dim:
            raise ValueError(
                "All vectors must have the same dimension; "
                f"channel '{channel}' has {arr.shape[0]}, expected {expected_dim}"
            )
        vectors_1d[channel] = arr

    assert expected_dim is not None
    renormalized = renormalize_weights_for_present_channels(
        weights=weights, present_channels=set(vectors_1d.keys())
    )

    fused = np.zeros(expected_dim, dtype=np.float32)
    for channel, weight in renormalized.items():
        fused += vectors_1d[channel] * np.float32(weight)

    norm = float(np.linalg.norm(fused))
    if not np.isfinite(norm):
        raise ValueError("Fused vector norm is NaN/Inf and cannot be normalized")
    if norm <= 0.0:
        raise ValueError("Fused vector has zero norm and cannot be normalized")
    return fused / np.float32(norm)
