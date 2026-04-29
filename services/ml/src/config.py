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
