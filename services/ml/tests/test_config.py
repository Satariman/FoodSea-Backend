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
    monkeypatch.delenv("PHOTO_SEARCH_INDEX_MODE", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_IMAGE", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_NAME", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_BRAND", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_QUERY_WEIGHT_IMAGE", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES", raising=False)
    monkeypatch.delenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME", raising=False)
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
    assert cfg.PHOTO_SEARCH_INDEX_MODE == "weighted_multimodal"
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_IMAGE == 0.18
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_NAME == 0.30
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_BRAND == 0.18
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY == 0.10
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY == 0.08
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION == 0.06
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION == 0.05
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT == 0.03
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT == 0.02
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_IMAGE == 0.20
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW == 0.25
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME == 0.25
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND == 0.10
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES == 0.10
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME == 0.10
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
    monkeypatch.setenv("PHOTO_SEARCH_INDEX_MODE", "legacy_image_only")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_IMAGE", "0.11")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_NAME", "0.21")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_BRAND", "0.31")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY", "0.41")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY", "0.51")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION", "0.61")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION", "0.71")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT", "0.81")
    monkeypatch.setenv("PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT", "0.91")
    monkeypatch.setenv("PHOTO_SEARCH_QUERY_WEIGHT_IMAGE", "0.12")
    monkeypatch.setenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW", "0.22")
    monkeypatch.setenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME", "0.32")
    monkeypatch.setenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND", "0.42")
    monkeypatch.setenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES", "0.52")
    monkeypatch.setenv("PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME", "0.62")
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
    assert cfg.PHOTO_SEARCH_INDEX_MODE == "legacy_image_only"
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_IMAGE == 0.11
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_NAME == 0.21
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_BRAND == 0.31
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY == 0.41
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY == 0.51
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION == 0.61
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION == 0.71
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT == 0.81
    assert cfg.PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT == 0.91
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_IMAGE == 0.12
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW == 0.22
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME == 0.32
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND == 0.42
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES == 0.52
    assert cfg.PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME == 0.62
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
