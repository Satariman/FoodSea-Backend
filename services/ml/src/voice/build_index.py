from __future__ import annotations

from typing import Iterable

import numpy as np

from src.photo_search.fusion import weighted_fuse
from src.shared_index.schema import SharedIndexProfile, SharedIndexRow
from src.voice.index import VoiceIndex

def _coerce_channels(
    row: SharedIndexRow,
    *,
    dimensions: int,
) -> dict[str, np.ndarray]:
    channels: dict[str, np.ndarray] = {}
    for channel_name, vector in row.channels.items():
        arr = np.asarray(vector, dtype=np.float32)
        if arr.ndim != 1:
            continue
        if arr.shape[0] != dimensions:
            continue
        if not np.all(np.isfinite(arr)):
            continue
        channels[channel_name] = arr
    return channels


def _fused_vector_for_row(
    row: SharedIndexRow,
    *,
    dimensions: int,
    weights: dict[str, float],
) -> np.ndarray | None:
    channels = _coerce_channels(row, dimensions=dimensions)
    if not channels:
        return None

    try:
        return weighted_fuse(channels, weights)
    except ValueError as exc:
        raise ValueError(f"voice index fusion failed: {exc}") from exc


def build_voice_index(
    profile: SharedIndexProfile,
    rows: Iterable[SharedIndexRow],
) -> VoiceIndex:
    if profile.dimensions <= 0:
        raise ValueError("shared profile dimensions must be positive")

    weights = {channel_name: float(weight) for channel_name, weight in profile.build_weights.items()}

    vectors: list[np.ndarray] = []
    ids: list[str] = []
    names: list[str] = []
    for row in rows:
        product_id = row.meta.product_id.strip()
        if not product_id:
            continue

        fused = _fused_vector_for_row(
            row,
            dimensions=profile.dimensions,
            weights=weights,
        )
        if fused is None:
            continue

        ids.append(product_id)
        names.append(row.meta.name)
        vectors.append(fused)

    if not vectors:
        raise ValueError("no valid products for voice index rebuild")

    index = VoiceIndex()
    index.fit(
        ids=ids,
        names=names,
        vectors=vectors,
        source_provider=profile.provider,
        source_model=profile.model,
        source_dimensions=profile.dimensions,
    )
    return index
