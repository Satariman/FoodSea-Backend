"""Standalone script to (re)build the voice index from core-service catalog data.

Usage:
    GEMINI_API_KEY=... CORE_GRPC_ADDR=core:9091 python -m scripts.build_voice_index
"""
from __future__ import annotations

import asyncio
import logging
import sys
from dataclasses import dataclass
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from src.config import Config
from src.data_loader import DataLoader
from src.embeddings.gemini_client import GeminiClient
from src.voice_index.builder import build_voice_index
from src.voice_index.image_fetcher import HTTPImageFetcher

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("build_voice_index")


@dataclass
class _ProductView:
    id: str
    name: str
    brand: str
    category: str
    image_url: str | None


async def main() -> None:
    cfg = Config()
    log.info("loading products from core-service at %s", cfg.CORE_GRPC_ADDR)
    raw = DataLoader(cfg.CORE_GRPC_ADDR).load_products()
    products = [
        _ProductView(
            id=str(r.product_id),
            name=r.name,
            brand=r.brand_name or "",
            category=r.category_name or "",
            image_url=r.image_url or None,
        )
        for r in raw
    ]
    log.info("loaded %d products", len(products))

    gemini = GeminiClient(
        api_key=cfg.GEMINI_API_KEY,
        model=cfg.GEMINI_MODEL,
        output_dim=cfg.GEMINI_OUTPUT_DIM,
    )
    fetcher = HTTPImageFetcher(timeout_sec=5.0)

    log.info("building voice index (this may take a while)...")
    index = await build_voice_index(products, gemini, fetcher)
    log.info("indexed %d products", len(index.product_ids))

    out_path = Path(cfg.VOICE_INDEX_PATH)
    index.save(out_path)
    log.info("saved index to %s", out_path)


if __name__ == "__main__":
    asyncio.run(main())
