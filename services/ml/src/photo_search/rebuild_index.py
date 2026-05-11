from __future__ import annotations

from dataclasses import dataclass, field
from typing import Iterable
from urllib import request

import numpy as np

from src.config import Config
from src.data_loader import DataLoader, ProductData
from src.photo_search.embeddings import (
    EmbeddingProvider,
    GeminiAPIEmbeddingProvider,
    VertexAIEmbeddingProvider,
)
from src.photo_search.fusion import weighted_fuse
from src.photo_search.index import PhotoProductIndex, PhotoProductMeta


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
    req = request.Request(image_url, method="GET")
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


@dataclass(slots=True)
class _BuildEntry:
    meta: PhotoProductMeta
    full_text: str
    channels_text: dict[str, str] = field(default_factory=dict)
    channels_vector: dict[str, np.ndarray] = field(default_factory=dict)
    image_bytes: bytes | None = None
    image_mime_type: str = "image/jpeg"


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
    channels = ("name", "brand", "category", "subcategory", "description", "composition", "weight", "full_text")
    for channel in channels:
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
            for row_idx, (entry_idx, _) in enumerate(chunk):
                entries[entry_idx].channels_vector["image"] = vectors[row_idx]
            continue
        except Exception:
            pass

        # fallback: try each product independently to keep text-only behavior
        for entry_idx, item in chunk:
            try:
                vec = provider.embed_multimodal([item]).astype(np.float32)
                if vec.shape == (1, provider.dimensions):
                    entries[entry_idx].channels_vector["image"] = vec[0]
            except Exception:
                continue


def build_photo_index(
    products: Iterable[ProductData],
    provider: EmbeddingProvider,
    index_path: str,
    batch_size: int,
    index_mode: str = "legacy_image_only",
    build_weights: dict[str, float] | None = None,
) -> PhotoProductIndex:
    if index_mode not in {"legacy_image_only", "weighted_multimodal"}:
        raise ValueError(f"unsupported photo index mode: {index_mode}")
    metas: list[PhotoProductMeta] = []
    vectors: np.ndarray | None = None
    skipped_empty_text = 0
    batch = max(1, int(batch_size))

    if index_mode == "legacy_image_only":
        items: list[tuple[PhotoProductMeta, dict[str, object]]] = []
        for product in products:
            if not product.product_id:
                continue
            try:
                image_bytes, mime_type = fetch_image_bytes(product.image_url)
            except Exception:
                continue
            meta = meta_from_product(product)
            items.append((meta, {"text": "", "image_bytes": image_bytes, "mime_type": mime_type}))

        if not items:
            raise ValueError("no products available for photo index rebuild")

        vectors_parts: list[np.ndarray] = []
        for start in range(0, len(items), batch):
            chunk = items[start : start + batch]
            payload = [item for _, item in chunk]
            vectors_parts.append(provider.embed_multimodal(payload))
            metas.extend(meta for meta, _ in chunk)
        vectors = np.vstack(vectors_parts).astype(np.float32)

    else:
        effective_weights = build_weights or {}
        entries: list[_BuildEntry] = []
        for product in products:
            if not product.product_id:
                continue
            channel_texts = _product_channel_texts(product)
            if not channel_texts and not product.image_url:
                skipped_empty_text += 1
                continue
            entry = _BuildEntry(
                meta=meta_from_product(product),
                full_text=product_text(product).strip(),
                channels_text=channel_texts,
            )
            if product.image_url:
                try:
                    image_bytes, mime_type = fetch_image_bytes(product.image_url)
                    entry.image_bytes = image_bytes
                    entry.image_mime_type = mime_type
                except Exception:
                    pass
            entries.append(entry)

        if not entries:
            if skipped_empty_text:
                raise ValueError("no valid products for photo index rebuild: all products have empty text")
            raise ValueError("no products available for photo index rebuild")

        _embed_text_channels(
            entries=entries,
            provider=provider,
            batch_size=batch,
            weights=effective_weights,
        )
        if float(effective_weights.get("image", 0.0)) > 0.0:
            _embed_image_channel(entries=entries, provider=provider, batch_size=batch)

        fused_rows: list[np.ndarray] = []
        for entry in entries:
            if not entry.channels_vector:
                continue
            try:
                fused = weighted_fuse(entry.channels_vector, effective_weights)
            except ValueError:
                continue
            metas.append(entry.meta)
            fused_rows.append(fused)

        if not fused_rows:
            raise ValueError("no valid products for photo index rebuild")
        vectors = np.vstack(fused_rows).astype(np.float32)

    assert vectors is not None
    if vectors.ndim != 2:
        raise ValueError("provider returned invalid vector tensor")
    if vectors.shape[0] != len(metas):
        raise ValueError("number of vectors does not match products")

    index = PhotoProductIndex()
    index.build(
        metas=metas,
        vectors=vectors,
        provider=provider.provider_name,
        model=provider.model,
        dimensions=provider.dimensions,
        index_mode=index_mode,
        build_weights=build_weights,
    )
    index.save(index_path)
    return index


def provider_from_config(config: Config) -> EmbeddingProvider:
    provider_name = config.PHOTO_SEARCH_PROVIDER.strip().lower()
    if provider_name == "gemini_api_key":
        return GeminiAPIEmbeddingProvider(
            api_key=config.GEMINI_API_KEY,
            model=config.PHOTO_SEARCH_MODEL,
            dimensions=config.PHOTO_SEARCH_DIMENSIONS,
        )
    if provider_name in {"vertex", "vertex_ai"}:
        return VertexAIEmbeddingProvider(
            project_id=config.VERTEX_PROJECT_ID,
            location=config.VERTEX_LOCATION,
            model=config.PHOTO_SEARCH_MODEL,
            dimensions=config.PHOTO_SEARCH_DIMENSIONS,
        )
    raise ValueError(f"unsupported PHOTO_SEARCH_PROVIDER: {config.PHOTO_SEARCH_PROVIDER}")


def main() -> None:
    config = Config()
    provider = provider_from_config(config)
    loader = DataLoader(config.CORE_GRPC_ADDR)
    products = loader.load_products()
    build_photo_index(
        products=products,
        provider=provider,
        index_path=config.PHOTO_SEARCH_INDEX_PATH,
        batch_size=config.PHOTO_SEARCH_BATCH_SIZE,
        index_mode=config.PHOTO_SEARCH_INDEX_MODE,
        build_weights=build_weights_from_config(config),
    )


if __name__ == "__main__":
    main()
