import asyncio
from dataclasses import dataclass
from unittest.mock import AsyncMock

import numpy as np
import pytest

from src.voice_index.builder import build_voice_index, ImageFetcher
from src.voice_index.index import VoiceIndex


@dataclass
class FakeProduct:
    id: str
    name: str
    brand: str
    category: str
    image_url: str | None


class FakeImageFetcher(ImageFetcher):
    def __init__(self, payload: bytes | None = b"\xff" * 10) -> None:
        self.payload = payload
        self.calls: list[str] = []

    async def fetch_safe(self, url: str) -> bytes | None:
        self.calls.append(url)
        return self.payload


def _make_gemini_mock(vector_per_product: list[np.ndarray]) -> AsyncMock:
    gm = AsyncMock()
    gm.embed_product = AsyncMock(side_effect=vector_per_product)
    return gm


def test_build_voice_index_calls_gemini_per_product():
    products = [
        FakeProduct("a", "Молоко", "Простоквашино", "Молочные", "http://img/a.jpg"),
        FakeProduct("b", "Хлеб", "Бородинский", "Хлеб", None),
    ]
    vectors = [np.array([1.0, 0.0]), np.array([0.0, 1.0])]
    gemini = _make_gemini_mock(vectors)
    fetcher = FakeImageFetcher()
    index = asyncio.run(build_voice_index(products, gemini, fetcher))
    assert isinstance(index, VoiceIndex)
    assert index.product_ids == ["a", "b"]
    assert gemini.embed_product.await_count == 2
    # No image_url for product "b" → fetch_safe NOT called for that one
    assert fetcher.calls == ["http://img/a.jpg"]


def test_build_voice_index_falls_back_to_text_only_on_embed_error():
    products = [FakeProduct("a", "X", "Y", "Z", "http://img/a.jpg")]
    err = RuntimeError("gemini down")
    gemini = AsyncMock()
    gemini.embed_product = AsyncMock(side_effect=[err, np.array([0.5, 0.5])])
    fetcher = FakeImageFetcher()
    index = asyncio.run(build_voice_index(products, gemini, fetcher))
    assert index.product_ids == ["a"]
    # Called twice: first failed (with image), second succeeded (text only)
    assert gemini.embed_product.await_count == 2
    second_call = gemini.embed_product.await_args_list[1]
    assert second_call.kwargs.get("image_bytes") is None
