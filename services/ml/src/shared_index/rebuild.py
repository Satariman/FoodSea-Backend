from __future__ import annotations

from src.config import Config
from src.data_loader import DataLoader
from src.shared_index.builder import (
    build_shared_index,
    build_weights_from_config,
    provider_from_config,
)
from src.shared_index.store import save_shared_index


def main() -> None:
    config = Config()
    loader = DataLoader(config.CORE_GRPC_ADDR)
    products = loader.load_products()
    provider = provider_from_config(config)

    profile, rows = build_shared_index(
        products=products,
        provider=provider,
        batch_size=config.PHOTO_SEARCH_BATCH_SIZE,
        build_weights=build_weights_from_config(config),
        index_mode=config.PHOTO_SEARCH_INDEX_MODE,
    )
    save_shared_index(config.SHARED_INDEX_PATH, profile, rows)


if __name__ == "__main__":
    main()
