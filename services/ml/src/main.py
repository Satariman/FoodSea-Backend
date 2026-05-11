"""Entrypoint for ml-service gRPC server."""

from __future__ import annotations

import logging
from concurrent import futures

import grpc

from src.config import Config
from src.data_loader import DataLoader
from src.feature_builder import FeatureBuilder
from src.index import AnalogIndex
from src.photo_search.embeddings import (
    GeminiAPIEmbeddingProvider,
    ProviderNotConfiguredError,
    VertexAIEmbeddingProvider,
)
from src.photo_search.index import PhotoProductIndex
from src.photo_search.service import PhotoSearchEngine
from src.service import AnalogServicer, PhotoSearchState
from src.proto import analogs_pb2_grpc

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)


def build_index(config: Config) -> AnalogIndex:
    index = AnalogIndex()

    if index.load(config.INDEX_PATH):
        logger.info("loaded index from %s (%d products)", config.INDEX_PATH, len(index.product_ids))
        return index

    try:
        logger.info("loading products from core-service at %s", config.CORE_GRPC_ADDR)
        loader = DataLoader(config.CORE_GRPC_ADDR)
        products = loader.load_products()
    except Exception as exc:  # noqa: BLE001 - service should stay up with empty index
        logger.warning("failed to load products from core-service: %s", exc)
        return index

    logger.info("loaded %d products", len(products))
    if not products:
        logger.warning("no products loaded, starting with empty index")
        return index

    logger.info("building feature vectors using model %s", config.TEXT_MODEL)
    try:
        builder = FeatureBuilder(
            text_model_name=config.TEXT_MODEL,
            text_weight=config.TEXT_WEIGHT,
            category_weight=config.CATEGORY_WEIGHT,
            nutrition_weight=config.NUTRITION_WEIGHT,
            price_weight=config.PRICE_WEIGHT,
        )
    except ImportError as exc:
        logger.warning("text embedding dependency is unavailable, starting with empty index: %s", exc)
        return index

    vectors = builder.build(products)

    product_ids = [p.product_id for p in products]
    names = {p.product_id: p.name for p in products}
    offers = {p.product_id: p.offers for p in products}
    index.build(product_ids, names, vectors, offers)
    index.save(config.INDEX_PATH)
    logger.info("index built and saved to %s", config.INDEX_PATH)

    return index


def build_photo_search(config: Config) -> tuple[PhotoSearchEngine | None, PhotoSearchState]:
    if not config.PHOTO_SEARCH_ENABLED:
        logger.info("photo search is disabled by config")
        return None, PhotoSearchState.DISABLED

    try:
        if config.PHOTO_SEARCH_PROVIDER == "gemini_api_key":
            provider = GeminiAPIEmbeddingProvider(
                api_key=config.GEMINI_API_KEY,
                model=config.PHOTO_SEARCH_MODEL,
                dimensions=config.PHOTO_SEARCH_DIMENSIONS,
            )
        elif config.PHOTO_SEARCH_PROVIDER == "vertex_ai":
            provider = VertexAIEmbeddingProvider(
                project_id=config.VERTEX_PROJECT_ID,
                location=config.VERTEX_LOCATION,
                model=config.PHOTO_SEARCH_MODEL,
                dimensions=config.PHOTO_SEARCH_DIMENSIONS,
            )
        else:
            logger.warning("unknown photo search provider: %s", config.PHOTO_SEARCH_PROVIDER)
            return None, PhotoSearchState.UNREADY
    except ProviderNotConfiguredError as exc:
        logger.warning("photo search provider is not configured: %s", exc)
        return None, PhotoSearchState.UNREADY
    except Exception as exc:  # noqa: BLE001
        logger.warning("failed to initialize photo search provider: %s", exc)
        return None, PhotoSearchState.UNREADY

    index = PhotoProductIndex()
    loaded = index.load(
        config.PHOTO_SEARCH_INDEX_PATH,
        provider=provider.provider_name,
        model=provider.model,
        dimensions=provider.dimensions,
    )
    if not loaded:
        logger.warning("photo search index not loaded from %s", config.PHOTO_SEARCH_INDEX_PATH)
        return None, PhotoSearchState.UNREADY

    logger.info(
        "photo search enabled with provider=%s model=%s size=%d",
        provider.provider_name,
        provider.model,
        len(index.product_ids),
    )
    return PhotoSearchEngine(index=index, provider=provider, config=config), PhotoSearchState.READY


def serve() -> None:
    config = Config()
    index = build_index(config)
    photo_search, photo_search_state = build_photo_search(config)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    analogs_pb2_grpc.add_AnalogServiceServicer_to_server(
        AnalogServicer(index, config, photo_search, photo_search_state=photo_search_state),
        server,
    )

    addr = f"[::]:{config.GRPC_PORT}"
    server.add_insecure_port(addr)
    server.start()
    logger.info("ml-service listening on %s", addr)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
