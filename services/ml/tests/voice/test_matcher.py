import asyncio
from unittest.mock import AsyncMock

import numpy as np
import pytest

from src.embeddings.cache import EmbeddingCache
from src.voice.matcher import VoiceMatcher, NGramMatch
from src.voice.ngram import NGram, Segment
from src.voice_index.index import VoiceIndex, Match


def _index_with_two_products() -> VoiceIndex:
    idx = VoiceIndex()
    idx.fit(
        ids=["m", "k"],
        names=["Молоко 1л", "Кефир 900мл"],
        vectors=[np.array([1.0, 0.0]), np.array([0.0, 1.0])],
    )
    return idx


def _gemini_returning(vectors: list[np.ndarray]) -> AsyncMock:
    gm = AsyncMock()
    gm.embed_queries_batch = AsyncMock(return_value=vectors)
    return gm


def test_match_segment_returns_top1_for_each_ngram_above_threshold():
    seg = Segment(quantity=1, unit=None, words=["молоко"])
    gemini = _gemini_returning([np.array([1.0, 0.0])])
    matcher = VoiceMatcher(
        index=_index_with_two_products(),
        gemini=gemini,
        cache=EmbeddingCache(max_size=10),
        min_score=0.5,
    )
    matches = asyncio.run(matcher.match_segment(seg))
    assert len(matches) == 1
    assert matches[0].match.product_id == "m"
    assert matches[0].match.score >= 0.99


def test_match_segment_filters_below_min_score():
    seg = Segment(quantity=1, unit=None, words=["мусор"])
    gemini = _gemini_returning([np.array([0.5, 0.5])])  # cosine ~0.707 to both products
    matcher = VoiceMatcher(
        index=_index_with_two_products(),
        gemini=gemini,
        cache=EmbeddingCache(max_size=10),
        min_score=0.9,  # higher than 0.707
    )
    matches = asyncio.run(matcher.match_segment(seg))
    assert matches == []


def test_match_segment_uses_cache_for_repeated_queries():
    seg = Segment(quantity=1, unit=None, words=["молоко", "молоко"])
    # generate_ngrams produces unigram "молоко" (×2) and bigram "молоко молоко":
    # 2 unique texts. The two unigrams must be deduplicated to a single batch entry.
    gemini = _gemini_returning([np.array([1.0, 0.0]), np.array([1.0, 0.0])])
    cache = EmbeddingCache(max_size=10)
    matcher = VoiceMatcher(
        index=_index_with_two_products(),
        gemini=gemini,
        cache=cache,
        min_score=0.5,
    )
    asyncio.run(matcher.match_segment(seg))
    call_args = gemini.embed_queries_batch.await_args
    batch = call_args.args[0]
    assert len(batch) == len(set(batch))  # batch is deduplicated
    assert "молоко" in batch  # the repeated unigram is in there only once


def test_match_segment_empty_words_returns_empty():
    seg = Segment(quantity=1, unit=None, words=[])
    gemini = _gemini_returning([])
    matcher = VoiceMatcher(
        index=_index_with_two_products(),
        gemini=gemini,
        cache=EmbeddingCache(max_size=10),
        min_score=0.5,
    )
    matches = asyncio.run(matcher.match_segment(seg))
    assert matches == []
    gemini.embed_queries_batch.assert_not_awaited()
