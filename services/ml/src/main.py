"""Entrypoint for ml-service gRPC server."""

from __future__ import annotations

import logging
from concurrent import futures

import grpc
import numpy as np

from src.config import Config
from src.data_loader import DataLoader
from src.embeddings.cache import EmbeddingCache
from src.analogs.feature_builder import FeatureBuilder
from src.analogs.index import AnalogIndex
from src.proto import analogs_pb2_grpc, voice_pb2_grpc
from src.photo_search.embeddings import (
    GeminiAPIEmbeddingProvider,
    ProviderNotConfiguredError,
)
from src.photo_search.index import PhotoProductIndex
from src.photo_search.service import PhotoSearchEngine
from src.analogs.servicer import AnalogServicer, PhotoSearchState
from src.voice.servicer import VoiceServicer
from src.voice.matcher import VoiceMatcher
from src.voice.pipeline import VoicePipeline
from src.voice.index import VoiceIndex

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)


def _photo_search_build_weights(config: Config) -> dict[str, float]:
    return {
        "image": float(config.PHOTO_SEARCH_BUILD_WEIGHT_IMAGE),
        "name": float(config.PHOTO_SEARCH_BUILD_WEIGHT_NAME),
        "brand": float(config.PHOTO_SEARCH_BUILD_WEIGHT_BRAND),
        "category": float(config.PHOTO_SEARCH_BUILD_WEIGHT_CATEGORY),
        "subcategory": float(config.PHOTO_SEARCH_BUILD_WEIGHT_SUBCATEGORY),
        "description": float(config.PHOTO_SEARCH_BUILD_WEIGHT_DESCRIPTION),
        "composition": float(config.PHOTO_SEARCH_BUILD_WEIGHT_COMPOSITION),
        "weight": float(config.PHOTO_SEARCH_BUILD_WEIGHT_WEIGHT),
        "full_text": float(config.PHOTO_SEARCH_BUILD_WEIGHT_FULL_TEXT),
    }


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


def register_voice_servicer(server: grpc.Server, config: Config) -> None:
    try:
        voice_index = VoiceIndex.load(config.VOICE_INDEX_PATH)
    except FileNotFoundError:
        logger.warning(
            "voice_index.pkl not found at %s; VoiceServicer NOT registered",
            config.VOICE_INDEX_PATH,
        )
        return
    except Exception as exc:  # noqa: BLE001
        logger.warning(
            "voice_index.pkl cannot be loaded from %s; VoiceServicer NOT registered: %s",
            config.VOICE_INDEX_PATH,
            exc,
        )
        return

    if not config.GEMINI_API_KEY:
        logger.warning("GEMINI_API_KEY is not configured; VoiceServicer NOT registered")
        return

    if not isinstance(voice_index, VoiceIndex):
        logger.warning("voice index payload has invalid type; VoiceServicer NOT registered")
        return

    vectors = getattr(voice_index, "vectors", None)
    if not isinstance(vectors, np.ndarray) or vectors.ndim != 2 or vectors.shape[1] <= 0:
        logger.warning("voice index payload has invalid vectors; VoiceServicer NOT registered")
        return

    source_dimensions = int(getattr(voice_index, "source_dimensions", 0) or 0)
    source_provider = str(getattr(voice_index, "source_provider", "") or "")
    source_model = str(getattr(voice_index, "source_model", "") or "")
    index_dimensions = int(source_dimensions or vectors.shape[1])
    if index_dimensions != config.GEMINI_OUTPUT_DIM:
        logger.warning(
            "voice index embedding dimensions mismatch: index=%d runtime=%d; VoiceServicer NOT registered",
            index_dimensions,
            config.GEMINI_OUTPUT_DIM,
        )
        return
    if source_provider and source_provider != "gemini_api_key":
        logger.warning(
            "voice index provider mismatch: index=%s runtime=gemini_api_key; VoiceServicer NOT registered",
            source_provider,
        )
        return
    if source_model and source_model != config.GEMINI_MODEL:
        logger.warning(
            "voice index model mismatch: index=%s runtime=%s; VoiceServicer NOT registered",
            source_model,
            config.GEMINI_MODEL,
        )
        return

    try:
        from src.embeddings.gemini_client import GeminiClient

        gemini = GeminiClient(
            api_key=config.GEMINI_API_KEY,
            model=config.GEMINI_MODEL,
            output_dim=config.GEMINI_OUTPUT_DIM,
        )
    except ImportError as exc:
        logger.warning("voice embedding dependency is unavailable; VoiceServicer NOT registered: %s", exc)
        return
    matcher = VoiceMatcher(
        index=voice_index,
        gemini=gemini,
        cache=EmbeddingCache(max_size=config.VOICE_EMBEDDING_CACHE_SIZE),
        min_score=config.VOICE_MIN_NGRAM_SCORE,
        max_ngram_len=config.VOICE_MAX_NGRAM_LEN,
        rerank_mode=config.VOICE_RERANK_MODE,
        rerank_candidates_k=config.VOICE_RERANK_CANDIDATES_K,
    )
    voice_pipeline = VoicePipeline(matcher=matcher)
    voice_pb2_grpc.add_VoiceServiceServicer_to_server(
        VoiceServicer(pipeline=voice_pipeline), server,
    )
    logger.info(
        "VoiceServicer registered with index of %d products",
        len(voice_index.product_ids),
    )


def build_photo_search(config: Config) -> tuple[PhotoSearchEngine | None, PhotoSearchState]:
    if not config.PHOTO_SEARCH_ENABLED:
        logger.info("photo search is disabled by config")
        return None, PhotoSearchState.DISABLED

    if (
        config.PHOTO_SEARCH_PROVIDER != config.EMBEDDING_PROVIDER
        or config.PHOTO_SEARCH_MODEL != config.EMBEDDING_MODEL
        or config.PHOTO_SEARCH_DIMENSIONS != config.EMBEDDING_DIMENSIONS
    ):
        logger.warning(
            "photo search embedding config mismatch with shared index config; "
            "PHOTO_SEARCH_PROVIDER/MODEL/DIMENSIONS must match EMBEDDING_PROVIDER/MODEL/DIMENSIONS"
        )
        return None, PhotoSearchState.UNREADY

    try:
        if config.PHOTO_SEARCH_PROVIDER == "gemini_api_key":
            provider = GeminiAPIEmbeddingProvider(
                api_key=config.GEMINI_API_KEY,
                model=config.PHOTO_SEARCH_MODEL,
                dimensions=config.PHOTO_SEARCH_DIMENSIONS,
            )
        elif config.PHOTO_SEARCH_PROVIDER == "vertex_ai":
            logger.warning("photo search provider vertex_ai is not implemented yet")
            return None, PhotoSearchState.UNREADY
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
        expected_profile={
            "index_mode": config.PHOTO_SEARCH_INDEX_MODE,
            "build_weights": _photo_search_build_weights(config),
        },
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
    register_voice_servicer(server, config)

    addr = f"[::]:{config.GRPC_PORT}"
    server.add_insecure_port(addr)
    server.start()
    logger.info("ml-service listening on %s", addr)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
