import asyncio
from unittest.mock import MagicMock

import numpy as np
import pytest

from src.embeddings.gemini_client import GeminiClient


class FakeResult:
    def __init__(self, vectors: list[list[float]]) -> None:
        self.embeddings = [MagicMock(values=v) for v in vectors]


@pytest.fixture
def fake_client() -> MagicMock:
    client = MagicMock()
    client.models.embed_content.return_value = FakeResult([[0.1, 0.2, 0.3]])
    return client


def test_embed_queries_batch_returns_one_vector_per_query(fake_client: MagicMock):
    fake_client.models.embed_content.return_value = FakeResult([[0.1, 0.2], [0.3, 0.4]])
    gc = GeminiClient(api_key="x", model="gemini-embedding-2", output_dim=2, _client=fake_client)
    vecs = asyncio.run(gc.embed_queries_batch(["молоко", "хлеб"]))
    assert len(vecs) == 2
    assert vecs[0].dtype == np.float32
    np.testing.assert_array_almost_equal(vecs[0], np.array([0.1, 0.2]))
    np.testing.assert_array_almost_equal(vecs[1], np.array([0.3, 0.4]))


def test_embed_queries_batch_uses_search_result_task_prefix(fake_client: MagicMock):
    fake_client.models.embed_content.return_value = FakeResult([[0.1]])
    gc = GeminiClient(api_key="x", model="gemini-embedding-2", output_dim=1, _client=fake_client)
    asyncio.run(gc.embed_queries_batch(["молоко"]))
    call_args = fake_client.models.embed_content.call_args
    contents = call_args.kwargs["contents"]
    assert len(contents) == 1
    text = contents[0].parts[0].text
    assert text.startswith("task: search result | query:")
    assert "молоко" in text


def test_embed_product_uses_text_only_when_no_image(fake_client: MagicMock):
    fake_client.models.embed_content.return_value = FakeResult([[0.5]])
    gc = GeminiClient(api_key="x", model="gemini-embedding-2", output_dim=1, _client=fake_client)
    vec = asyncio.run(gc.embed_product(name="Молоко", brand="Простоквашино", category="Молочные", image_bytes=None))
    np.testing.assert_array_almost_equal(vec, np.array([0.5]))
    contents = fake_client.models.embed_content.call_args.kwargs["contents"]
    parts = contents[0].parts
    assert len(parts) == 1
    assert "Молоко" in parts[0].text
    assert "Простоквашино" in parts[0].text


def test_embed_product_includes_image_part_when_provided(fake_client: MagicMock):
    fake_client.models.embed_content.return_value = FakeResult([[0.5]])
    gc = GeminiClient(api_key="x", model="gemini-embedding-2", output_dim=1, _client=fake_client)
    asyncio.run(gc.embed_product(name="X", brand="Y", category="Z", image_bytes=b"\x00\xff"))
    contents = fake_client.models.embed_content.call_args.kwargs["contents"]
    parts = contents[0].parts
    assert len(parts) == 2
    assert "X" in parts[0].text
