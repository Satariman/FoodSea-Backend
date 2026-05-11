"""Runtime configuration for ml-service."""

from __future__ import annotations

import os


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

        gemini_api_key = os.environ.get("GEMINI_API_KEY")
        if not gemini_api_key:
            raise RuntimeError("GEMINI_API_KEY environment variable is required")
        self.GEMINI_API_KEY = gemini_api_key
        self.GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-embedding-2")
        self.GEMINI_OUTPUT_DIM = int(os.environ.get("GEMINI_OUTPUT_DIM", "768"))
        self.VOICE_INDEX_PATH = os.environ.get("VOICE_INDEX_PATH", "data/voice_index.pkl")
        self.VOICE_MIN_NGRAM_SCORE = float(os.environ.get("VOICE_MIN_NGRAM_SCORE", "0.7"))
        self.VOICE_MAX_NGRAM_LEN = int(os.environ.get("VOICE_MAX_NGRAM_LEN", "3"))
        self.VOICE_GRPC_PORT = int(os.environ.get("VOICE_GRPC_PORT", "9094"))
        self.VOICE_EMBEDDING_CACHE_SIZE = int(os.environ.get("VOICE_EMBEDDING_CACHE_SIZE", "10000"))
