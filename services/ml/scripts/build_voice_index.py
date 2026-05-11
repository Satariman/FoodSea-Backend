"""Standalone script to (re)build the voice index from shared index rows.

Usage:
    python -m scripts.build_voice_index
"""
from __future__ import annotations

import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from src.config import Config
from src.shared_index.store import load_shared_index
from src.voice.build_index import build_voice_index

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("build_voice_index")


def main() -> None:
    cfg = Config()
    log.info("loading shared index from %s", cfg.SHARED_INDEX_PATH)
    profile, rows = load_shared_index(cfg.SHARED_INDEX_PATH)
    log.info("loaded %d shared rows", len(rows))

    log.info("building voice index from shared vectors...")
    index = build_voice_index(profile, rows)
    log.info("indexed %d products", len(index.product_ids))

    out_path = Path(cfg.VOICE_INDEX_PATH)
    index.save(out_path)
    log.info("saved index to %s", out_path)


if __name__ == "__main__":
    main()
