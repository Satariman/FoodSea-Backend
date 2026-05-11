"""Runtime configuration for ml-service."""

from __future__ import annotations

import math
import os


def _get_bool_env(name: str, default: bool, *, strict: bool = False) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    value = raw.strip().lower()
    if value in {"1", "true", "yes", "on"}:
        return True
    if value in {"0", "false", "no", "off"}:
        return False
    if strict:
        raise ValueError(
            f"Invalid boolean value for {name}: {raw!r}. "
            "Expected one of: 1,true,yes,on,0,false,no,off"
        )
    return default


def _get_int_env(name: str, default: str, *, positive: bool = False) -> int:
    raw = os.getenv(name, default)
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"Invalid integer value for {name}: {raw!r}") from exc
    if positive and value <= 0:
        raise ValueError(f"{name} must be a positive integer, got {value}")
    return value


def _get_float_env(
    name: str,
    default: str,
    *,
    min_value: float | None = None,
    max_value: float | None = None,
) -> float:
    raw = os.getenv(name, default)
    try:
        value = float(raw)
    except ValueError as exc:
        raise ValueError(f"Invalid float value for {name}: {raw!r}") from exc

    if not math.isfinite(value):
        raise ValueError(f"{name} must be finite, got {value!r}")
    if min_value is not None and value < min_value:
        raise ValueError(f"{name} must be >= {min_value}, got {value}")
    if max_value is not None and value > max_value:
        raise ValueError(f"{name} must be <= {max_value}, got {value}")
    return value


def _get_enum_env(name: str, default: str, *, allowed: tuple[str, ...]) -> str:
    value = os.getenv(name, default)
    if value not in allowed:
        allowed_str = ", ".join(allowed)
        raise ValueError(f"Invalid value for {name}: {value!r}. Allowed: {allowed_str}")
    return value


class Config:
    """Configuration loaded from environment variables with sane defaults."""

    def __init__(self) -> None:
        self.GRPC_PORT = _get_int_env("GRPC_PORT", "50051", positive=True)
        self.CORE_GRPC_ADDR = os.getenv("CORE_GRPC_ADDR", "localhost:9091")
        self.INDEX_PATH = os.getenv("INDEX_PATH", "data/index.pkl")
        self.TEXT_MODEL = os.getenv("TEXT_MODEL", "all-MiniLM-L6-v2")
        self.TEXT_WEIGHT = _get_float_env("TEXT_WEIGHT", "1.0", min_value=0.0)
        self.CATEGORY_WEIGHT = _get_float_env("CATEGORY_WEIGHT", "3.0", min_value=0.0)
        self.NUTRITION_WEIGHT = _get_float_env("NUTRITION_WEIGHT", "1.5", min_value=0.0)
        self.PRICE_WEIGHT = _get_float_env("PRICE_WEIGHT", "0.8", min_value=0.0)
        self.PRICE_PENALTY = _get_float_env("PRICE_PENALTY", "0.3", min_value=0.0)
        self.MIN_SCORE_THRESHOLD = _get_float_env(
            "MIN_SCORE_THRESHOLD", "0.3", min_value=0.0, max_value=1.0
        )
        self.GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")
        self.GEMINI_MODEL = os.getenv("GEMINI_MODEL", "gemini-embedding-2")
        self.GEMINI_OUTPUT_DIM = _get_int_env("GEMINI_OUTPUT_DIM", "768", positive=True)
        self.VOICE_INDEX_PATH = os.getenv("VOICE_INDEX_PATH", "data/voice_index.pkl")
        self.VOICE_MIN_NGRAM_SCORE = _get_float_env(
            "VOICE_MIN_NGRAM_SCORE", "0.7", min_value=0.0, max_value=1.0
        )
        self.VOICE_MAX_NGRAM_LEN = _get_int_env("VOICE_MAX_NGRAM_LEN", "3", positive=True)
        self.VOICE_GRPC_PORT = _get_int_env("VOICE_GRPC_PORT", "9094", positive=True)
        self.VOICE_EMBEDDING_CACHE_SIZE = _get_int_env(
            "VOICE_EMBEDDING_CACHE_SIZE", "10000", positive=True
        )
        self.VOICE_RERANK_MODE = _get_enum_env(
            "VOICE_RERANK_MODE",
            "legacy",
            allowed=("legacy", "attribute_aware"),
        )
        self.VOICE_RERANK_CANDIDATES_K = _get_int_env(
            "VOICE_RERANK_CANDIDATES_K",
            "5",
            positive=True,
        )
        self.PHOTO_SEARCH_ENABLED = _get_bool_env(
            "PHOTO_SEARCH_ENABLED", True, strict=True
        )
        self.PHOTO_SEARCH_INDEX_PATH = os.getenv(
            "PHOTO_SEARCH_INDEX_PATH", "data/photo_search_index.pkl"
        )
        self.PHOTO_SEARCH_PROVIDER = os.getenv("PHOTO_SEARCH_PROVIDER", "gemini_api_key")
        self.VERTEX_PROJECT_ID = os.getenv("VERTEX_PROJECT_ID")
        self.VERTEX_LOCATION = os.getenv("VERTEX_LOCATION", "us-central1")
        self.PHOTO_SEARCH_MODEL = os.getenv("PHOTO_SEARCH_MODEL", "gemini-embedding-2")
        self.PHOTO_SEARCH_DIMENSIONS = _get_int_env(
            "PHOTO_SEARCH_DIMENSIONS", "768", positive=True
        )
        self.PHOTO_SEARCH_INDEX_MODE = _get_enum_env(
            "PHOTO_SEARCH_INDEX_MODE",
            "weighted_multimodal",
            allowed=("legacy_image_only", "weighted_multimodal"),
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_IMAGE = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_IMAGE", "0.18", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_NAME = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_NAME", "0.30", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_BRAND = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_BRAND", "0.18", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY", "0.10", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY", "0.08", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION", "0.06", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION", "0.05", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT", "0.03", min_value=0.0
        )
        self.PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT = _get_float_env(
            "PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT", "0.02", min_value=0.0
        )
        self.PHOTO_SEARCH_QUERY_WEIGHT_IMAGE = _get_float_env(
            "PHOTO_SEARCH_QUERY_WEIGHT_IMAGE", "0.20", min_value=0.0
        )
        self.PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW = _get_float_env(
            "PHOTO_SEARCH_QUERY_WEIGHT_OCR_RAW", "0.25", min_value=0.0
        )
        self.PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME = _get_float_env(
            "PHOTO_SEARCH_QUERY_WEIGHT_OCR_NAME", "0.25", min_value=0.0
        )
        self.PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND = _get_float_env(
            "PHOTO_SEARCH_QUERY_WEIGHT_OCR_BRAND", "0.10", min_value=0.0
        )
        self.PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES = _get_float_env(
            "PHOTO_SEARCH_QUERY_WEIGHT_OCR_PERCENTAGES", "0.10", min_value=0.0
        )
        self.PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME = _get_float_env(
            "PHOTO_SEARCH_QUERY_WEIGHT_OCR_VOLUME", "0.10", min_value=0.0
        )
        self.PHOTO_SEARCH_MIN_SCORE = _get_float_env(
            "PHOTO_SEARCH_MIN_SCORE", "0.25", min_value=0.0, max_value=1.0
        )
        self.PHOTO_SEARCH_BATCH_SIZE = _get_int_env(
            "PHOTO_SEARCH_BATCH_SIZE", "32", positive=True
        )
        self.SHARED_INDEX_PATH = os.getenv(
            "SHARED_INDEX_PATH", "data/shared_embedding_index.pkl"
        )
        self.EMBEDDING_PROVIDER = os.getenv(
            "EMBEDDING_PROVIDER",
            self.PHOTO_SEARCH_PROVIDER,
        )
        self.EMBEDDING_MODEL = os.getenv(
            "EMBEDDING_MODEL",
            self.PHOTO_SEARCH_MODEL,
        )
        self.EMBEDDING_DIMENSIONS = _get_int_env(
            "EMBEDDING_DIMENSIONS",
            str(self.PHOTO_SEARCH_DIMENSIONS),
            positive=True,
        )
