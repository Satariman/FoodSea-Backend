"""Runtime configuration for ml-service."""

from __future__ import annotations

import os


def _get_bool_env(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    value = raw.strip().lower()
    if value in {"1", "true", "yes", "on"}:
        return True
    if value in {"0", "false", "no", "off"}:
        return False
    return default


class Config:
    """Configuration loaded from environment variables with sane defaults."""

    def __init__(self) -> None:
        self.GRPC_PORT = int(os.getenv("GRPC_PORT", "50051"))
        self.CORE_GRPC_ADDR = os.getenv("CORE_GRPC_ADDR", "localhost:9091")
        self.INDEX_PATH = os.getenv("INDEX_PATH", "data/index.pkl")
        self.TEXT_MODEL = os.getenv("TEXT_MODEL", "all-MiniLM-L6-v2")
        self.TEXT_WEIGHT = float(os.getenv("TEXT_WEIGHT", "1.0"))
        self.CATEGORY_WEIGHT = float(os.getenv("CATEGORY_WEIGHT", "3.0"))
        self.NUTRITION_WEIGHT = float(os.getenv("NUTRITION_WEIGHT", "1.5"))
        self.PRICE_WEIGHT = float(os.getenv("PRICE_WEIGHT", "0.8"))
        self.PRICE_PENALTY = float(os.getenv("PRICE_PENALTY", "0.3"))
        self.MIN_SCORE_THRESHOLD = float(os.getenv("MIN_SCORE_THRESHOLD", "0.3"))
        self.PHOTO_SEARCH_ENABLED = _get_bool_env("PHOTO_SEARCH_ENABLED", True)
        self.PHOTO_SEARCH_INDEX_PATH = os.getenv(
            "PHOTO_SEARCH_INDEX_PATH", "data/photo_search_index.pkl"
        )
        self.PHOTO_SEARCH_PROVIDER = os.getenv(
            "PHOTO_SEARCH_PROVIDER", "gemini_api_key"
        )
        self.GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")
        self.VERTEX_PROJECT_ID = os.getenv("VERTEX_PROJECT_ID")
        self.VERTEX_LOCATION = os.getenv("VERTEX_LOCATION", "us-central1")
        self.PHOTO_SEARCH_MODEL = os.getenv("PHOTO_SEARCH_MODEL", "gemini-embedding-2")
        self.PHOTO_SEARCH_DIMENSIONS = int(
            os.getenv("PHOTO_SEARCH_DIMENSIONS", "768")
        )
        self.PHOTO_SEARCH_MIN_SCORE = float(os.getenv("PHOTO_SEARCH_MIN_SCORE", "0.25"))
        self.PHOTO_SEARCH_BATCH_SIZE = int(os.getenv("PHOTO_SEARCH_BATCH_SIZE", "32"))
