from __future__ import annotations

from src.config import Config


def test_photo_search_defaults(monkeypatch) -> None:
    monkeypatch.delenv("PHOTO_SEARCH_ENABLED", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_INDEX_PATH", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_PROVIDER", raising=False)
    monkeypatch.delenv("GEMINI_API_KEY", raising=False)
    monkeypatch.delenv("VERTEX_PROJECT_ID", raising=False)
    monkeypatch.delenv("VERTEX_LOCATION", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_MODEL", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_DIMENSIONS", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_MIN_SCORE", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BATCH_SIZE", raising=False)

    cfg = Config()

    assert cfg.PHOTO_SEARCH_ENABLED is True
    assert cfg.PHOTO_SEARCH_INDEX_PATH == "data/photo_search_index.pkl"
    assert cfg.PHOTO_SEARCH_PROVIDER == "gemini_api_key"
    assert cfg.GEMINI_API_KEY is None
    assert cfg.VERTEX_PROJECT_ID is None
    assert cfg.VERTEX_LOCATION == "us-central1"
    assert cfg.PHOTO_SEARCH_MODEL == "gemini-embedding-2"
    assert cfg.PHOTO_SEARCH_DIMENSIONS == 768
    assert cfg.PHOTO_SEARCH_MIN_SCORE == 0.25
    assert cfg.PHOTO_SEARCH_BATCH_SIZE == 32
