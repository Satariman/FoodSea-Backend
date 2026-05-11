from __future__ import annotations

from src.main import PhotoSearchState, build_photo_search


class _Cfg:
    PHOTO_SEARCH_ENABLED = True
    PHOTO_SEARCH_PROVIDER = "gemini_api_key"
    PHOTO_SEARCH_MODEL = "gemini-embedding-2"
    PHOTO_SEARCH_DIMENSIONS = 768
    EMBEDDING_PROVIDER = "gemini_api_key"
    EMBEDDING_MODEL = "gemini-embedding-2"
    EMBEDDING_DIMENSIONS = 768
    GEMINI_API_KEY = "secret"
    PHOTO_SEARCH_INDEX_PATH = "/tmp/photo_index.pkl"
    PHOTO_SEARCH_INDEX_MODE = "weighted_multimodal"
    PHOTO_SEARCH_BUILD_WEIGHT_IMAGE = 0.2
    PHOTO_SEARCH_BUILD_WEIGHT_NAME = 0.3
    PHOTO_SEARCH_BUILD_WEIGHT_BRAND = 0.2
    PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY = 0.1
    PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY = 0.08
    PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION = 0.05
    PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION = 0.04
    PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT = 0.01
    PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT = 0.02


def test_build_photo_search_returns_unready_on_embedding_config_mismatch() -> None:
    cfg = _Cfg()
    cfg.PHOTO_SEARCH_DIMENSIONS = 1024

    engine, state = build_photo_search(cfg)

    assert engine is None
    assert state == PhotoSearchState.UNREADY
