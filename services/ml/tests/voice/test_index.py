import tempfile
from pathlib import Path

import numpy as np
import pytest

from src.voice.index import VoiceIndex, Match


@pytest.fixture
def sample_index() -> VoiceIndex:
    idx = VoiceIndex()
    vectors = [
        np.array([1.0, 0.0, 0.0]),  # "Молоко"
        np.array([0.0, 1.0, 0.0]),  # "Хлеб"
        np.array([0.0, 0.0, 1.0]),  # "Яблоки"
    ]
    idx.fit(
        ids=["m1", "h1", "y1"],
        names=["Молоко 1л", "Хлеб 400г", "Яблоки 1кг"],
        vectors=vectors,
        source_provider="gemini_api_key",
        source_model="gemini-embedding-2",
        source_dimensions=3,
    )
    return idx


def test_query_returns_top_k_sorted_by_score(sample_index: VoiceIndex):
    matches = sample_index.query(np.array([1.0, 0.0, 0.0]), top_k=2)
    assert len(matches) == 2
    assert matches[0].product_id == "m1"
    assert matches[0].score > matches[1].score


def test_query_perfect_match_score_close_to_one(sample_index: VoiceIndex):
    matches = sample_index.query(np.array([1.0, 0.0, 0.0]), top_k=1)
    assert matches[0].score == pytest.approx(1.0, abs=1e-5)


def test_query_orthogonal_vector_score_zero(sample_index: VoiceIndex):
    matches = sample_index.query(np.array([0.0, 1.0, 0.0]), top_k=3)
    last = next(m for m in matches if m.product_id != "h1")
    assert last.score == pytest.approx(0.0, abs=1e-5)


def test_save_and_load_roundtrip(sample_index: VoiceIndex):
    with tempfile.TemporaryDirectory() as d:
        path = Path(d) / "voice_index.pkl"
        sample_index.save(path)
        loaded = VoiceIndex.load(path)
        assert loaded.source_provider == "gemini_api_key"
        assert loaded.source_model == "gemini-embedding-2"
        assert loaded.source_dimensions == 3
        matches = loaded.query(np.array([1.0, 0.0, 0.0]), top_k=1)
        assert matches[0].product_id == "m1"
