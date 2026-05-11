import numpy as np
import pytest

from src.embeddings.cache import EmbeddingCache


def test_get_returns_none_for_missing_key():
    cache = EmbeddingCache(max_size=4)
    assert cache.get("молоко") is None


def test_put_then_get_returns_value():
    cache = EmbeddingCache(max_size=4)
    vec = np.array([1.0, 2.0, 3.0])
    cache.put("молоко", vec)
    np.testing.assert_array_equal(cache.get("молоко"), vec)


def test_lru_evicts_oldest_when_full():
    cache = EmbeddingCache(max_size=2)
    cache.put("a", np.array([1.0]))
    cache.put("b", np.array([2.0]))
    cache.put("c", np.array([3.0]))
    assert cache.get("a") is None
    assert cache.get("b") is not None
    assert cache.get("c") is not None


def test_get_refreshes_recency():
    cache = EmbeddingCache(max_size=2)
    cache.put("a", np.array([1.0]))
    cache.put("b", np.array([2.0]))
    cache.get("a")          # touch a
    cache.put("c", np.array([3.0]))  # should evict b, not a
    assert cache.get("a") is not None
    assert cache.get("b") is None
    assert cache.get("c") is not None


def test_put_existing_key_updates_value():
    cache = EmbeddingCache(max_size=2)
    cache.put("a", np.array([1.0]))
    cache.put("a", np.array([2.0]))
    np.testing.assert_array_equal(cache.get("a"), np.array([2.0]))


def test_max_size_must_be_positive():
    with pytest.raises(ValueError):
        EmbeddingCache(max_size=0)
