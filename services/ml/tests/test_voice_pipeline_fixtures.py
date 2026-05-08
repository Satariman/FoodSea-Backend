import asyncio
import json
from pathlib import Path
from unittest.mock import AsyncMock

import numpy as np
import pytest

from src.embeddings.cache import EmbeddingCache
from src.voice.matcher import VoiceMatcher
from src.voice.pipeline import VoicePipeline
from src.voice_index.index import VoiceIndex


CATALOG = [
    ("mp", "Молоко Простоквашино",   "молок",        np.array([1.0, 0, 0, 0, 0, 0, 0])),
    ("sr", "Сыр Российский",          "сыр росси",   np.array([0, 1.0, 0, 0, 0, 0, 0])),
    ("bn", "Бананы Эквадор",          "банан",        np.array([0, 0, 1.0, 0, 0, 0, 0])),
    ("fk", "Филе куриное Мираторг",   "филе кур",    np.array([0, 0, 0, 1.0, 0, 0, 0])),
    ("og", "Огурцы свежие",           "огурц",        np.array([0, 0, 0, 0, 1.0, 0, 0])),
    ("bt", "Батон Нарезной",          "батон",        np.array([0, 0, 0, 0, 0, 1.0, 0])),
    ("ya", "Яблоки Голден",           "яблок",        np.array([0, 0, 0, 0, 0, 0, 1.0])),
]


def _build_index() -> VoiceIndex:
    idx = VoiceIndex()
    idx.fit(
        ids=[p[0] for p in CATALOG],
        names=[p[1] for p in CATALOG],
        vectors=[p[3] for p in CATALOG],
    )
    return idx


def _fake_gemini() -> AsyncMock:
    gm = AsyncMock()

    async def fake_batch(queries: list[str]) -> list[np.ndarray]:
        out = []
        for q in queries:
            matched = next((p[3] for p in CATALOG if p[2] in q), np.zeros(7))
            out.append(matched)
        return out

    gm.embed_queries_batch = AsyncMock(side_effect=fake_batch)
    return gm


def _load_fixtures() -> list[dict]:
    path = Path(__file__).parent / "fixtures" / "russian_phrases.json"
    return json.loads(path.read_text(encoding="utf-8"))


@pytest.mark.parametrize("case", _load_fixtures(), ids=lambda c: c["name"])
def test_pipeline_against_fixtures(case: dict):
    pipeline = VoicePipeline(
        matcher=VoiceMatcher(
            index=_build_index(),
            gemini=_fake_gemini(),
            cache=EmbeddingCache(max_size=100),
            min_score=0.5,
        ),
    )
    resp = asyncio.run(pipeline.parse(case["input"], "ru-RU"))
    actual_ids = [i.product_id for i in resp.items]
    actual_qty = [i.quantity for i in resp.items]
    assert sorted(actual_ids) == sorted(case["expected_product_ids"]), (
        f"\ninput: {case['input']!r}\nexpected: {case['expected_product_ids']}\nactual:   {actual_ids}"
    )
    if case["expected_quantities"]:
        expected_by_id = dict(zip(case["expected_product_ids"], case["expected_quantities"]))
        actual_by_id = dict(zip(actual_ids, actual_qty))
        for pid, q in expected_by_id.items():
            assert actual_by_id.get(pid) == q, f"qty mismatch for {pid}: expected {q}, got {actual_by_id.get(pid)}"
