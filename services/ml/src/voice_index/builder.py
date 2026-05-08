import asyncio
import logging
from typing import Iterable, Protocol

import numpy as np

from src.voice_index.image_fetcher import ImageFetcher
from src.voice_index.index import VoiceIndex

logger = logging.getLogger(__name__)


class _ProductLike(Protocol):
    id: str
    name: str
    brand: str
    category: str
    image_url: str | None


class _GeminiLike(Protocol):
    async def embed_product(
        self,
        *,
        name: str,
        brand: str,
        category: str,
        image_bytes: bytes | None,
    ) -> np.ndarray: ...


async def _embed_one(
    product: _ProductLike,
    gemini: _GeminiLike,
    fetcher: ImageFetcher,
) -> np.ndarray | None:
    image_bytes: bytes | None = None
    if product.image_url:
        image_bytes = await fetcher.fetch_safe(product.image_url)
    try:
        return await gemini.embed_product(
            name=product.name,
            brand=product.brand,
            category=product.category,
            image_bytes=image_bytes,
        )
    except Exception as e:
        logger.warning("multimodal embed failed for %s, retry text-only: %s", product.id, e)
        try:
            return await gemini.embed_product(
                name=product.name,
                brand=product.brand,
                category=product.category,
                image_bytes=None,
            )
        except Exception as e2:
            logger.error("text-only embed also failed for %s: %s", product.id, e2)
            return None


async def build_voice_index(
    products: Iterable[_ProductLike],
    gemini: _GeminiLike,
    fetcher: ImageFetcher,
    batch_size: int = 50,
) -> VoiceIndex:
    products_list = list(products)
    vectors: list[np.ndarray] = []
    ids: list[str] = []
    names: list[str] = []
    for batch_start in range(0, len(products_list), batch_size):
        batch = products_list[batch_start:batch_start + batch_size]
        batch_vecs = await asyncio.gather(*[_embed_one(p, gemini, fetcher) for p in batch])
        for product, vec in zip(batch, batch_vecs):
            if vec is None:
                continue
            ids.append(product.id)
            names.append(product.name)
            vectors.append(vec)
    index = VoiceIndex()
    index.fit(ids=ids, names=names, vectors=vectors)
    return index
