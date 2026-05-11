from __future__ import annotations

from typing import Iterable

import numpy as np

from src.config import Config
from src.data_loader import DataLoader, ProductData
from src.photo_search.embeddings import (
    EmbeddingProvider,
    GeminiAPIEmbeddingProvider,
    VertexAIEmbeddingProvider,
)
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


def build_photo_index(
    products: Iterable[ProductData],
    provider: EmbeddingProvider,
    index_path: str,
    batch_size: int,
) -> PhotoProductIndex:
    metas: list[PhotoProductMeta] = []
    texts: list[str] = []
    skipped_empty_text = 0
    for product in products:
        if not product.product_id:
            continue
        text = product_text(product).strip()
        if not text:
            skipped_empty_text += 1
            continue
        metas.append(meta_from_product(product))
        texts.append(text)

    if not metas:
        if skipped_empty_text:
            raise ValueError("no valid products for photo index rebuild: all products have empty text")
        raise ValueError("no products available for photo index rebuild")

    batch = max(1, int(batch_size))
    vectors_parts: list[np.ndarray] = []
    for start in range(0, len(texts), batch):
        chunk = texts[start : start + batch]
        vectors_parts.append(provider.embed_texts(chunk))

    vectors = np.vstack(vectors_parts).astype(np.float32)
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
    )


if __name__ == "__main__":
    main()
