from __future__ import annotations

from pathlib import Path
from typing import Iterable
from urllib.parse import quote, urlsplit, urlunsplit
from urllib import request

import numpy as np

from src.config import Config
from src.data_loader import ProductData
from src.photo_search.fusion import weighted_fuse
from src.photo_search.index import PhotoProductIndex, PhotoProductMeta
from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow
from src.shared_index.store import load_shared_index


def product_text(product: ProductData) -> str:
    parts = [
        product.name,
        product.brand_name,
        product.category_name,
        product.subcategory_name,
        product.description,
        product.composition,
        product.weight,
    ]
    return " | ".join(part.strip() for part in parts if isinstance(part, str) and part.strip())


def meta_from_product(product: ProductData) -> PhotoProductMeta:
    return PhotoProductMeta(
        product_id=product.product_id,
        name=product.name,
        brand_name=product.brand_name,
        category_name=product.category_name,
        subcategory_name=product.subcategory_name,
        image_url=product.image_url,
    )


def fetch_image_bytes(image_url: str, timeout_seconds: int = 10) -> tuple[bytes, str]:
    split = urlsplit(image_url)
    encoded_url = urlunsplit(
        (
            split.scheme,
            split.netloc,
            quote(split.path, safe="/%"),
            quote(split.query, safe="=&%:+,;"),
            split.fragment,
        )
    )
    req = request.Request(encoded_url, method="GET")
    with request.urlopen(req, timeout=timeout_seconds) as response:
        body = response.read()
        content_type = response.headers.get("Content-Type", "image/jpeg")
    if not body:
        raise ValueError("empty image payload")
    mime_type = str(content_type).split(";")[0].strip() or "image/jpeg"
    return body, mime_type


def build_weights_from_config(config: Config) -> dict[str, float]:
    return {
        "image": float(config.PHOTO_SEARCH_BUILD_WEIGHT_IMAGE),
        "name": float(config.PHOTO_SEARCH_BUILD_WEIGHT_NAME),
        "brand": float(config.PHOTO_SEARCH_BUILD_WEIGHT_BRAND),
        "category": float(config.PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY),
        "subcategory": float(config.PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY),
        "description": float(config.PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION),
        "composition": float(config.PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION),
        "weight": float(config.PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT),
        "full_text": float(config.PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT),
    }


def meta_from_shared_meta(shared_meta: SharedIndexMeta) -> PhotoProductMeta:
    return PhotoProductMeta(
        product_id=shared_meta.product_id,
        name=shared_meta.name,
        brand_name=shared_meta.brand_name,
        category_name=shared_meta.category_name,
        subcategory_name=shared_meta.subcategory_name,
        image_url=shared_meta.image_url,
    )


def _coerce_channels(channels: dict[str, np.ndarray]) -> dict[str, np.ndarray]:
    coerced: dict[str, np.ndarray] = {}
    for channel_name, vector in channels.items():
        arr = np.asarray(vector, dtype=np.float32)
        if arr.ndim != 1:
            continue
        if not np.all(np.isfinite(arr)):
            continue
        coerced[channel_name] = arr
    return coerced


def _collect_rows_for_mode(
    *,
    rows: Iterable[SharedIndexRow],
    index_mode: str,
    build_weights: dict[str, float],
) -> tuple[list[PhotoProductMeta], np.ndarray]:
    if index_mode not in {"legacy_image_only", "weighted_multimodal"}:
        raise ValueError(f"unsupported photo index mode: {index_mode}")

    metas: list[PhotoProductMeta] = []
    vectors: list[np.ndarray] = []

    for row in rows:
        channels = _coerce_channels(row.channels)
        if not channels:
            continue

        if index_mode == "legacy_image_only":
            image_vector = channels.get("image")
            if image_vector is None:
                continue
            fused = image_vector
        else:
            try:
                fused = weighted_fuse(channels, build_weights)
            except ValueError:
                continue

        metas.append(meta_from_shared_meta(row.meta))
        vectors.append(np.asarray(fused, dtype=np.float32))

    if not vectors:
        raise ValueError("no valid products for photo index rebuild")

    matrix = np.vstack(vectors).astype(np.float32)
    if matrix.ndim != 2:
        raise ValueError("shared index rows produced invalid vector tensor")
    if matrix.shape[0] != len(metas):
        raise ValueError("number of vectors does not match products")
    return metas, matrix


def build_photo_index(
    *,
    profile: SharedIndexProfile,
    rows: Iterable[SharedIndexRow],
    index_path: str,
    index_mode: str | None = None,
    build_weights: dict[str, float] | None = None,
) -> PhotoProductIndex:
    effective_mode = (index_mode or profile.index_mode or "weighted_multimodal").strip()
    effective_weights = dict(build_weights if build_weights is not None else profile.build_weights)
    metas, vectors = _collect_rows_for_mode(
        rows=rows,
        index_mode=effective_mode,
        build_weights=effective_weights,
    )

    index = PhotoProductIndex()
    index.build(
        metas=metas,
        vectors=vectors,
        provider=profile.provider,
        model=profile.model,
        dimensions=profile.dimensions,
        index_mode=effective_mode,
        build_weights=effective_weights,
    )
    index.save(index_path)
    return index


def build_photo_index_from_shared_path(
    *,
    shared_index_path: str | Path,
    index_path: str,
    index_mode: str,
    build_weights: dict[str, float] | None = None,
) -> PhotoProductIndex:
    profile, rows = load_shared_index(shared_index_path)
    return build_photo_index(
        profile=profile,
        rows=rows,
        index_path=index_path,
        index_mode=index_mode,
        build_weights=build_weights,
    )


def main() -> None:
    config = Config()
    build_photo_index_from_shared_path(
        shared_index_path=config.SHARED_INDEX_PATH,
        index_path=config.PHOTO_SEARCH_INDEX_PATH,
        index_mode=config.PHOTO_SEARCH_INDEX_MODE,
        build_weights=build_weights_from_config(config),
    )


if __name__ == "__main__":
    main()
