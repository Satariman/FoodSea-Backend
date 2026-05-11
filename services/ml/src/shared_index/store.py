from __future__ import annotations

import io
import pickle
from pathlib import Path

import numpy as np

from .schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow


class _RestrictedUnpickler(pickle.Unpickler):
    _ALLOWED_GLOBALS: dict[tuple[str, str], object] = {
        ("builtins", "dict"): dict,
        ("builtins", "list"): list,
        ("builtins", "set"): set,
        ("builtins", "tuple"): tuple,
        ("builtins", "str"): str,
        ("builtins", "int"): int,
        ("builtins", "float"): float,
        ("numpy", "dtype"): np.dtype,
        ("numpy", "ndarray"): np.ndarray,
        ("numpy.core.multiarray", "_reconstruct"): np.core.multiarray._reconstruct,
        ("numpy._core.multiarray", "_reconstruct"): np.core.multiarray._reconstruct,
    }

    def find_class(self, module: str, name: str) -> object:
        key = (module, name)
        if key not in self._ALLOWED_GLOBALS:
            raise pickle.UnpicklingError(f"unsupported pickle global: {module}.{name}")
        return self._ALLOWED_GLOBALS[key]


def _restricted_loads(raw: bytes) -> object:
    return _RestrictedUnpickler(io.BytesIO(raw)).load()


def save_shared_index(path: Path | str, profile: SharedIndexProfile, rows: list[SharedIndexRow]) -> None:
    _validate_profile(profile)
    _validate_rows(rows, profile.dimensions)

    payload_rows: list[dict[str, object]] = []
    for row in rows:
        payload_rows.append(
            {
                "meta": {
                    "product_id": row.meta.product_id,
                    "name": row.meta.name,
                    "brand_name": row.meta.brand_name,
                    "category_name": row.meta.category_name,
                    "subcategory_name": row.meta.subcategory_name,
                    "image_url": row.meta.image_url,
                },
                "channels": {
                    name: np.asarray(vector, dtype=np.float32).copy()
                    for name, vector in row.channels.items()
                },
            }
        )

    payload = {
        "profile": {
            "provider": profile.provider,
            "model": profile.model,
            "dimensions": int(profile.dimensions),
            "index_mode": profile.index_mode,
            "build_weights": {k: float(v) for k, v in profile.build_weights.items()},
        },
        "rows": payload_rows,
    }

    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(pickle.dumps(payload))


def load_shared_index(path: Path | str) -> tuple[SharedIndexProfile, list[SharedIndexRow]]:
    source = Path(path)
    try:
        data = _restricted_loads(source.read_bytes())
    except (pickle.UnpicklingError, EOFError) as exc:
        raise ValueError("invalid shared index payload") from exc
    if not isinstance(data, dict):
        raise ValueError("invalid shared index payload")

    profile = _parse_profile(data.get("profile"))
    rows = _parse_rows(data.get("rows"))
    _validate_rows(rows, profile.dimensions)
    return profile, rows


def _validate_profile(profile: SharedIndexProfile) -> None:
    if type(profile) is not SharedIndexProfile:
        raise ValueError("profile must be SharedIndexProfile")
    if not isinstance(profile.provider, str) or not profile.provider:
        raise ValueError("profile.provider must be non-empty string")
    if not isinstance(profile.model, str) or not profile.model:
        raise ValueError("profile.model must be non-empty string")
    if type(profile.dimensions) is not int or profile.dimensions <= 0:
        raise ValueError("profile.dimensions must be positive int")
    if not isinstance(profile.index_mode, str):
        raise ValueError("profile.index_mode must be string")
    if not isinstance(profile.build_weights, dict):
        raise ValueError("profile.build_weights must be dict")
    for key, value in profile.build_weights.items():
        if not isinstance(key, str):
            raise ValueError("profile.build_weights keys must be strings")
        try:
            numeric = float(value)
        except (TypeError, ValueError) as exc:
            raise ValueError("profile.build_weights values must be finite") from exc
        if not np.isfinite(numeric):
            raise ValueError("profile.build_weights values must be finite")


def _validate_rows(rows: list[SharedIndexRow], dimensions: int) -> None:
    if not isinstance(rows, list):
        raise ValueError("rows must be list")
    if not rows:
        raise ValueError("rows must not be empty")

    for row in rows:
        if type(row) is not SharedIndexRow:
            raise ValueError("rows must contain SharedIndexRow")
        if type(row.meta) is not SharedIndexMeta:
            raise ValueError("row.meta must be SharedIndexMeta")
        if not isinstance(row.meta.product_id, str) or not row.meta.product_id.strip():
            raise ValueError("row.meta.product_id must be non-empty string")
        if not isinstance(row.channels, dict):
            raise ValueError("row.channels must be dict")
        for channel_name, vector in row.channels.items():
            if not isinstance(channel_name, str) or not channel_name:
                raise ValueError("channel name must be non-empty string")
            arr = np.asarray(vector, dtype=np.float32)
            if arr.ndim != 1:
                raise ValueError("channel vector must be 1D")
            if arr.shape[0] != dimensions:
                raise ValueError("channel vector dimensions mismatch")
            if not np.all(np.isfinite(arr)):
                raise ValueError("channel vector must contain finite values")


def _parse_profile(raw_profile: object) -> SharedIndexProfile:
    if not isinstance(raw_profile, dict):
        raise ValueError("invalid profile")

    provider = raw_profile.get("provider")
    model = raw_profile.get("model")
    dimensions = raw_profile.get("dimensions")
    index_mode = raw_profile.get("index_mode")
    build_weights = raw_profile.get("build_weights")

    if not isinstance(provider, str):
        raise ValueError("invalid profile provider")
    if not isinstance(model, str):
        raise ValueError("invalid profile model")
    if type(dimensions) is not int:
        raise ValueError("invalid profile dimensions")
    if not isinstance(index_mode, str):
        raise ValueError("invalid profile index_mode")
    if not isinstance(build_weights, dict):
        raise ValueError("invalid profile build_weights")

    parsed_weights: dict[str, float] = {}
    for key, value in build_weights.items():
        if not isinstance(key, str):
            raise ValueError("invalid build_weights key")
        try:
            numeric = float(value)
        except (TypeError, ValueError) as exc:
            raise ValueError("invalid build_weights value") from exc
        if not np.isfinite(numeric):
            raise ValueError("invalid build_weights value")
        parsed_weights[key] = numeric

    profile = SharedIndexProfile(
        provider=provider,
        model=model,
        dimensions=dimensions,
        index_mode=index_mode,
        build_weights=parsed_weights,
    )
    _validate_profile(profile)
    return profile


def _parse_rows(raw_rows: object) -> list[SharedIndexRow]:
    if not isinstance(raw_rows, list):
        raise ValueError("invalid rows")

    rows: list[SharedIndexRow] = []
    for raw_row in raw_rows:
        if not isinstance(raw_row, dict):
            raise ValueError("invalid row")

        raw_meta = raw_row.get("meta")
        raw_channels = raw_row.get("channels")
        if not isinstance(raw_meta, dict):
            raise ValueError("invalid row meta")
        if not isinstance(raw_channels, dict):
            raise ValueError("invalid row channels")

        meta_fields = (
            "product_id",
            "name",
            "brand_name",
            "category_name",
            "subcategory_name",
            "image_url",
        )
        meta_values: dict[str, str] = {}
        for field in meta_fields:
            value = raw_meta.get(field)
            if not isinstance(value, str):
                raise ValueError(f"invalid row meta field: {field}")
            meta_values[field] = value

        meta = SharedIndexMeta(**meta_values)

        channels: dict[str, np.ndarray] = {}
        for channel_name, raw_vector in raw_channels.items():
            if not isinstance(channel_name, str):
                raise ValueError("invalid channel name")
            channels[channel_name] = np.asarray(raw_vector, dtype=np.float32)

        rows.append(SharedIndexRow(meta=meta, channels=channels))

    return rows
