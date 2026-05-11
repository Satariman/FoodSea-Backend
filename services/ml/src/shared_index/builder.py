from __future__ import annotations

from dataclasses import dataclass, field
import logging
from typing import Iterable

import numpy as np

from src.config import Config
from src.data_loader import ProductData
from src.photo_search.embeddings import (
    EmbeddingProvider,
    GeminiAPIEmbeddingProvider,
    VertexAIEmbeddingProvider,
)
from src.photo_search.build_index import fetch_image_bytes

from .schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow

_TEXT_CHANNELS = (
    "name",
    "brand",
    "category",
    "subcategory",
    "description",
    "composition",
    "weight",
    "full_text",
)
_LOGGER = logging.getLogger(__name__)


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


def meta_from_product(product: ProductData) -> SharedIndexMeta:
    return SharedIndexMeta(
        product_id=product.product_id,
        name=product.name,
        brand_name=product.brand_name,
        category_name=product.category_name,
        subcategory_name=product.subcategory_name,
        image_url=product.image_url,
    )


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


def provider_from_config(config: Config) -> EmbeddingProvider:
    provider_name = config.EMBEDDING_PROVIDER.strip().lower()
    if provider_name == "gemini_api_key":
        return GeminiAPIEmbeddingProvider(
            api_key=config.GEMINI_API_KEY,
            model=config.EMBEDDING_MODEL,
            dimensions=config.EMBEDDING_DIMENSIONS,
        )
    if provider_name in {"vertex", "vertex_ai"}:
        return VertexAIEmbeddingProvider(
            project_id=config.VERTEX_PROJECT_ID,
            location=config.VERTEX_LOCATION,
            model=config.EMBEDDING_MODEL,
            dimensions=config.EMBEDDING_DIMENSIONS,
        )
    raise ValueError(f"unsupported EMBEDDING_PROVIDER: {config.EMBEDDING_PROVIDER}")


@dataclass(slots=True)
class _BuildEntry:
    meta: SharedIndexMeta
    full_text: str
    channels_text: dict[str, str] = field(default_factory=dict)
    channels_vector: dict[str, np.ndarray] = field(default_factory=dict)
    image_bytes: bytes | None = None
    image_mime_type: str = "image/jpeg"


def _normalized_weights(raw_weights: dict[str, float] | None) -> dict[str, float]:
    weights = {name: 0.0 for name in ("image", *_TEXT_CHANNELS)}
    for key, value in (raw_weights or {}).items():
        if key in weights:
            weights[key] = float(value)
    return weights


def _product_channel_texts(product: ProductData) -> dict[str, str]:
    channel_map = {
        "name": product.name,
        "brand": product.brand_name,
        "category": product.category_name,
        "subcategory": product.subcategory_name,
        "description": product.description,
        "composition": product.composition,
        "weight": product.weight,
    }
    cleaned = {
        key: value.strip()
        for key, value in channel_map.items()
        if isinstance(value, str) and value.strip()
    }
    full_text = product_text(product).strip()
    if full_text:
        cleaned["full_text"] = full_text
    return cleaned


def _embed_text_channels(
    entries: list[_BuildEntry],
    provider: EmbeddingProvider,
    batch_size: int,
    weights: dict[str, float],
) -> None:
    for channel in _TEXT_CHANNELS:
        if float(weights.get(channel, 0.0)) <= 0.0:
            continue
        indexed_texts: list[tuple[int, str]] = []
        for idx, entry in enumerate(entries):
            text = entry.channels_text.get(channel, "")
            if text:
                indexed_texts.append((idx, text))
        if not indexed_texts:
            continue

        vectors_chunks: list[np.ndarray] = []
        for start in range(0, len(indexed_texts), batch_size):
            chunk = indexed_texts[start : start + batch_size]
            chunk_texts = [text for _, text in chunk]
            vectors_chunks.append(provider.embed_texts(chunk_texts))
        vectors = np.vstack(vectors_chunks).astype(np.float32)
        if vectors.shape[0] != len(indexed_texts):
            raise ValueError(f"embedding count mismatch for text channel '{channel}'")
        if vectors.shape[1] != provider.dimensions:
            raise ValueError("provider returned invalid dimensions for text channel")
        for row_idx, (entry_idx, _) in enumerate(indexed_texts):
            entries[entry_idx].channels_vector[channel] = vectors[row_idx]


def _embed_image_channel(
    entries: list[_BuildEntry], provider: EmbeddingProvider, batch_size: int
) -> None:
    indexed_items: list[tuple[int, dict[str, object]]] = []
    for idx, entry in enumerate(entries):
        if not entry.image_bytes:
            continue
        indexed_items.append(
            (
                idx,
                {
                    "text": entry.full_text,
                    "image_bytes": entry.image_bytes,
                    "mime_type": entry.image_mime_type,
                },
            )
        )
    for start in range(0, len(indexed_items), batch_size):
        chunk = indexed_items[start : start + batch_size]
        payload = [item for _, item in chunk]
        try:
            vectors = provider.embed_multimodal(payload).astype(np.float32)
            if vectors.shape[0] != len(chunk):
                raise ValueError("embedding count mismatch for image channel")
            if vectors.shape[1] != provider.dimensions:
                raise ValueError("provider returned invalid dimensions for image channel")
            for row_idx, (entry_idx, _) in enumerate(chunk):
                entries[entry_idx].channels_vector["image"] = vectors[row_idx]
            continue
        except Exception as exc:
            _LOGGER.warning(
                "image batch embedding failed; falling back to per-item embedding (chunk_size=%s): %s",
                len(chunk),
                exc,
            )

        for entry_idx, item in chunk:
            try:
                vec = provider.embed_multimodal([item]).astype(np.float32)
                if vec.shape == (1, provider.dimensions):
                    entries[entry_idx].channels_vector["image"] = vec[0]
            except Exception as exc:
                product_id = entries[entry_idx].meta.product_id
                _LOGGER.warning(
                    "image embedding failed for product_id=%s: %s",
                    product_id,
                    exc,
                )
                continue


def build_shared_index(
    *,
    products: Iterable[ProductData],
    provider: EmbeddingProvider,
    batch_size: int,
    build_weights: dict[str, float] | None = None,
    index_mode: str = "weighted_multimodal",
) -> tuple[SharedIndexProfile, list[SharedIndexRow]]:
    effective_weights = _normalized_weights(build_weights)
    batch = max(1, int(batch_size))
    use_image = float(effective_weights.get("image", 0.0)) > 0.0

    entries: list[_BuildEntry] = []
    for product in products:
        if not product.product_id:
            continue
        channel_texts = _product_channel_texts(product)
        if not channel_texts and not (use_image and product.image_url):
            continue
        entry = _BuildEntry(
            meta=meta_from_product(product),
            full_text=product_text(product).strip(),
            channels_text=channel_texts,
        )
        if use_image and product.image_url:
            try:
                image_bytes, mime_type = fetch_image_bytes(product.image_url)
                entry.image_bytes = image_bytes
                entry.image_mime_type = mime_type
            except Exception as exc:
                _LOGGER.warning(
                    "image fetch failed for product_id=%s url=%s: %s",
                    entry.meta.product_id,
                    product.image_url,
                    exc,
                )
        entries.append(entry)

    if not entries:
        raise ValueError("no products available for shared index rebuild")

    _embed_text_channels(
        entries=entries,
        provider=provider,
        batch_size=batch,
        weights=effective_weights,
    )
    if use_image:
        _embed_image_channel(entries=entries, provider=provider, batch_size=batch)

    rows: list[SharedIndexRow] = []
    for entry in entries:
        if not entry.channels_vector:
            continue
        channels = {}
        for channel_name, vector in entry.channels_vector.items():
            arr = np.asarray(vector, dtype=np.float32)
            if arr.shape != (provider.dimensions,):
                continue
            if not np.all(np.isfinite(arr)):
                continue
            channels[channel_name] = arr
        if not channels:
            continue
        rows.append(SharedIndexRow(meta=entry.meta, channels=channels))

    if not rows:
        raise ValueError("no valid products for shared index rebuild")

    profile = SharedIndexProfile(
        provider=provider.provider_name,
        model=provider.model,
        dimensions=provider.dimensions,
        index_mode=index_mode,
        build_weights=effective_weights,
    )
    return profile, rows
