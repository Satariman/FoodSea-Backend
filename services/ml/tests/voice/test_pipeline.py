import asyncio
from unittest.mock import AsyncMock

import numpy as np

from src.embeddings.cache import EmbeddingCache
from src.voice.matcher import VoiceMatcher
from src.voice.pipeline import VoicePipeline, VoiceItem
from src.voice_index.index import VoiceIndex


def _index_three_products() -> VoiceIndex:
    idx = VoiceIndex()
    idx.fit(
        ids=["m", "h", "y"],
        names=["Молоко 1л", "Хлеб 400г", "Яблоки 1кг"],
        vectors=[
            np.array([1.0, 0.0, 0.0]),
            np.array([0.0, 1.0, 0.0]),
            np.array([0.0, 0.0, 1.0]),
        ],
    )
    return idx


def _gemini_for_words(mapping: dict[str, np.ndarray]) -> AsyncMock:
    gm = AsyncMock()

    async def fake_batch(queries: list[str]) -> list[np.ndarray]:
        return [mapping.get(q, np.array([0.33, 0.33, 0.33])) for q in queries]

    gm.embed_queries_batch = AsyncMock(side_effect=fake_batch)
    return gm


def _make_pipeline(index, gemini, min_score=0.7):
    return VoicePipeline(
        matcher=VoiceMatcher(
            index=index,
            gemini=gemini,
            cache=EmbeddingCache(max_size=100),
            min_score=min_score,
        ),
    )


def test_parse_extracts_three_products_from_continuous_speech():
    mapping = {
        "молоко": np.array([1.0, 0.0, 0.0]),
        "хлеб": np.array([0.0, 1.0, 0.0]),
        "яблоки": np.array([0.0, 0.0, 1.0]),
    }
    p = _make_pipeline(_index_three_products(), _gemini_for_words(mapping))
    resp = asyncio.run(p.parse("молоко хлеб яблоки", "ru-RU"))
    ids = [i.product_id for i in resp.items]
    assert "m" in ids and "h" in ids and "y" in ids
    assert len(resp.items) == 3
    assert all(i.quantity == 1 for i in resp.items)
    assert resp.unmatched_queries == []


def test_parse_attaches_quantities_to_segments():
    mapping = {
        "молока": np.array([1.0, 0.0, 0.0]),
        "огурцов": np.array([0.0, 0.0, 1.0]),  # alias for яблоки in this test
    }
    idx = VoiceIndex()
    idx.fit(
        ids=["m", "o"],
        names=["Молоко", "Огурцы"],
        vectors=[np.array([1.0, 0.0, 0.0]), np.array([0.0, 0.0, 1.0])],
    )
    p = _make_pipeline(idx, _gemini_for_words(mapping))
    resp = asyncio.run(p.parse("два молока пять огурцов", "ru-RU"))
    by_id = {i.product_id: i for i in resp.items}
    assert by_id["m"].quantity == 2
    assert by_id["o"].quantity == 5


def test_parse_returns_unmatched_for_garbage_text():
    p = _make_pipeline(_index_three_products(), _gemini_for_words({}), min_score=0.99)
    resp = asyncio.run(p.parse("погода ужасная", "ru-RU"))
    assert resp.items == []
    assert "погода ужасная" in resp.unmatched_queries


def test_parse_greedy_picks_compound_brand_over_unigrams():
    mapping = {
        "молоко": np.array([0.95, 0.05, 0.0]),
        "простоквашино": np.array([0.95, 0.0, 0.05]),
        "молоко простоквашино": np.array([1.0, 0.0, 0.0]),  # highest, exact match
    }
    p = _make_pipeline(_index_three_products(), _gemini_for_words(mapping))
    resp = asyncio.run(p.parse("молоко простоквашино", "ru-RU"))
    assert len(resp.items) == 1
    assert resp.items[0].product_id == "m"
    assert resp.items[0].raw_query == "молоко простоквашино"


def test_parse_deduplicates_same_product_id_and_sums_quantity():
    mapping = {"молоко": np.array([1.0, 0.0, 0.0])}
    p = _make_pipeline(_index_three_products(), _gemini_for_words(mapping))
    resp = asyncio.run(p.parse("молоко молоко молоко", "ru-RU"))
    assert len(resp.items) == 1
    assert resp.items[0].product_id == "m"
    assert resp.items[0].quantity == 3
    assert resp.items[0].raw_query == "молоко, молоко, молоко"


def test_parse_keeps_separate_when_units_differ():
    # Same product matched twice but under different units → keep distinct.
    mapping = {"молока": np.array([1.0, 0.0, 0.0])}
    p = _make_pipeline(_index_three_products(), _gemini_for_words(mapping))
    resp = asyncio.run(p.parse("два литра молока три кило молока", "ru-RU"))
    assert len(resp.items) == 2
    units = sorted(item.unit for item in resp.items)
    assert units == ["кг", "л"]
