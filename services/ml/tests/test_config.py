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
    monkeypatch.delenv("SHARED_INDEX_PATH", raising=False)
    monkeypatch.delenv("EMBEDDING_PROVIDER", raising=False)
    monkeypatch.delenv("EMBEDDING_MODEL", raising=False)
    monkeypatch.delenv("EMBEDDING_DIMENSIONS", raising=False)
    monkeypatch.delenv("VOICE_RERANK_MODE", raising=False)
    monkeypatch.delenv("VOICE_RERANK_CANDIDATES_K", raising=False)

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
    assert cfg.SHARED_INDEX_PATH == "data/shared_embedding_index.pkl"
    assert cfg.EMBEDDING_PROVIDER == "gemini_api_key"
    assert cfg.EMBEDDING_MODEL == "gemini-embedding-2"
    assert cfg.EMBEDDING_DIMENSIONS == 768
    assert cfg.VOICE_RERANK_MODE == "legacy"
    assert cfg.VOICE_RERANK_CANDIDATES_K == 5


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
    monkeypatch.setenv("SHARED_INDEX_PATH", "/tmp/shared.pkl")
    monkeypatch.setenv("EMBEDDING_PROVIDER", "vertex_ai")
    monkeypatch.setenv("EMBEDDING_MODEL", "embed-custom")
    monkeypatch.setenv("EMBEDDING_DIMENSIONS", "512")
    monkeypatch.setenv("VOICE_RERANK_MODE", "attribute_aware")
    monkeypatch.setenv("VOICE_RERANK_CANDIDATES_K", "7")

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
    assert cfg.SHARED_INDEX_PATH == "/tmp/shared.pkl"
    assert cfg.EMBEDDING_PROVIDER == "vertex_ai"
    assert cfg.EMBEDDING_MODEL == "embed-custom"
    assert cfg.EMBEDDING_DIMENSIONS == 512
    assert cfg.VOICE_RERANK_MODE == "attribute_aware"
    assert cfg.VOICE_RERANK_CANDIDATES_K == 7


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


def test_photo_search_enabled_invalid_bool_fails_fast(monkeypatch) -> None:
    monkeypatch.setenv("PHOTO_SEARCH_ENABLED", "maybe")

    with pytest.raises(ValueError, match="PHOTO_SEARCH_ENABLED"):
        Config()


@pytest.mark.parametrize(
    ("env_name", "value"),
    [
        ("PHOTO_SEARCH_MIN_SCORE", "-0.1"),
        ("PHOTO_SEARCH_MIN_SCORE", "1.1"),
        ("MIN_SCORE_THRESHOLD", "-0.2"),
        ("MIN_SCORE_THRESHOLD", "1.2"),
    ],
)
def test_thresholds_must_be_between_zero_and_one(monkeypatch, env_name: str, value: str) -> None:
    monkeypatch.setenv(env_name, value)

    with pytest.raises(ValueError, match=env_name):
        Config()


@pytest.mark.parametrize(
    ("env_name", "value"),
    [
        ("TEXT_WEIGHT", "-1"),
        ("PHOTO_SEARCH_BUILD_WEIGHT_IMAGE", "-0.01"),
        ("PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW", "-0.1"),
    ],
)
def test_weights_must_be_non_negative(monkeypatch, env_name: str, value: str) -> None:
    monkeypatch.setenv(env_name, value)

    with pytest.raises(ValueError, match=env_name):
        Config()


@pytest.mark.parametrize(
    ("env_name", "value"),
    [
        ("TEXT_WEIGHT", "NaN"),
        ("CATEGORY_WEIGHT", "inf"),
        ("PHOTO_SEARCH_MIN_SCORE", "-inf"),
    ],
)
def test_float_values_must_be_finite(monkeypatch, env_name: str, value: str) -> None:
    monkeypatch.setenv(env_name, value)

    with pytest.raises(ValueError, match=env_name):
        Config()


@pytest.mark.parametrize(
    ("env_name", "value"),
    [
        ("GRPC_PORT", "0"),
        ("PHOTO_SEARCH_DIMENSIONS", "-10"),
        ("PHOTO_SEARCH_BATCH_SIZE", "0"),
        ("EMBEDDING_DIMENSIONS", "0"),
        ("VOICE_RERANK_CANDIDATES_K", "0"),
    ],
)
def test_positive_integer_fields(monkeypatch, env_name: str, value: str) -> None:
    monkeypatch.setenv(env_name, value)

    with pytest.raises(ValueError, match=env_name):
        Config()


def test_photo_search_index_mode_enum_validation(monkeypatch) -> None:
    monkeypatch.setenv("PHOTO_SEARCH_INDEX_MODE", "bad_mode")

    with pytest.raises(ValueError, match="PHOTO_SEARCH_INDEX_MODE"):
        Config()


def test_photo_search_index_mode_accepted_values(monkeypatch) -> None:
    monkeypatch.setenv("PHOTO_SEARCH_INDEX_MODE", "legacy_image_only")
    assert Config().PHOTO_SEARCH_INDEX_MODE == "legacy_image_only"

    monkeypatch.setenv("PHOTO_SEARCH_INDEX_MODE", "weighted_multimodal")
    assert Config().PHOTO_SEARCH_INDEX_MODE == "weighted_multimodal"


def test_voice_rerank_mode_enum_validation(monkeypatch) -> None:
    monkeypatch.setenv("VOICE_RERANK_MODE", "bad_mode")

    with pytest.raises(ValueError, match="VOICE_RERANK_MODE"):
        Config()


def test_voice_rerank_mode_accepted_values(monkeypatch) -> None:
    monkeypatch.setenv("VOICE_RERANK_MODE", "legacy")
    assert Config().VOICE_RERANK_MODE == "legacy"

    monkeypatch.setenv("VOICE_RERANK_MODE", "attribute_aware")
    assert Config().VOICE_RERANK_MODE == "attribute_aware"
