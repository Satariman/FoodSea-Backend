from __future__ import annotations

import pytest

from src.config import Config, _get_bool_env


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


def test_photo_search_env_overrides(monkeypatch) -> None:
    monkeypatch.setenv("PHOTO_SEARCH_ENABLED", "false")
    monkeypatch.setenv("PHOTO_SEARCH_INDEX_PATH", "/tmp/photo.pkl")
    monkeypatch.setenv("PHOTO_SEARCH_PROVIDER", "vertex")
    monkeypatch.setenv("GEMINI_API_KEY", "secret")
    monkeypatch.setenv("VERTEX_PROJECT_ID", "foodsea-ml")
    monkeypatch.setenv("VERTEX_LOCATION", "europe-west1")
    monkeypatch.setenv("PHOTO_SEARCH_MODEL", "gemini-custom")
    monkeypatch.setenv("PHOTO_SEARCH_DIMENSIONS", "1024")
    monkeypatch.setenv("PHOTO_SEARCH_MIN_SCORE", "0.4")
    monkeypatch.setenv("PHOTO_SEARCH_BATCH_SIZE", "64")

    cfg = Config()

    assert cfg.PHOTO_SEARCH_ENABLED is False
    assert cfg.PHOTO_SEARCH_INDEX_PATH == "/tmp/photo.pkl"
    assert cfg.PHOTO_SEARCH_PROVIDER == "vertex"
    assert cfg.GEMINI_API_KEY == "secret"
    assert cfg.VERTEX_PROJECT_ID == "foodsea-ml"
    assert cfg.VERTEX_LOCATION == "europe-west1"
    assert cfg.PHOTO_SEARCH_MODEL == "gemini-custom"
    assert cfg.PHOTO_SEARCH_DIMENSIONS == 1024
    assert cfg.PHOTO_SEARCH_MIN_SCORE == 0.4
    assert cfg.PHOTO_SEARCH_BATCH_SIZE == 64


@pytest.mark.parametrize("raw", ["1", "true", "yes", "on"])
def test_get_bool_env_true_forms(monkeypatch, raw: str) -> None:
    monkeypatch.setenv("BOOL_VAR", raw)
    assert _get_bool_env("BOOL_VAR", False) is True


@pytest.mark.parametrize("raw", ["0", "false", "no", "off"])
def test_get_bool_env_false_forms(monkeypatch, raw: str) -> None:
    monkeypatch.setenv("BOOL_VAR", raw)
    assert _get_bool_env("BOOL_VAR", True) is False


def test_get_bool_env_uses_default_when_missing(monkeypatch) -> None:
    monkeypatch.delenv("BOOL_VAR", raising=False)
    assert _get_bool_env("BOOL_VAR", True) is True
    assert _get_bool_env("BOOL_VAR", False) is False
