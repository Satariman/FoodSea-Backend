from __future__ import annotations

import sys
import types
from concurrent import futures
from pathlib import Path

import grpc
import numpy as np

from src.main import register_voice_servicer
from src.voice.index import VoiceIndex


class _Config:
    VOICE_INDEX_PATH: str
    GEMINI_API_KEY = "secret"
    GEMINI_MODEL = "gemini-embedding-2"
    GEMINI_OUTPUT_DIM = 2
    VOICE_EMBEDDING_CACHE_SIZE = 100
    VOICE_MIN_NGRAM_SCORE = 0.5
    VOICE_MAX_NGRAM_LEN = 3
    VOICE_RERANK_MODE = "legacy"
    VOICE_RERANK_CANDIDATES_K = 5


def _build_voice_index(path: Path, *, provider: str, model: str, dimensions: int) -> None:
    index = VoiceIndex()
    index.fit(
        ids=["p1"],
        names=["Milk"],
        vectors=[np.array([1.0, 0.0], dtype=np.float32)],
        source_provider=provider,
        source_model=model,
        source_dimensions=dimensions,
    )
    index.save(path)


def test_register_voice_servicer_skips_on_dimension_mismatch(monkeypatch, tmp_path: Path) -> None:
    path = tmp_path / "voice_index.pkl"
    _build_voice_index(path, provider="gemini_api_key", model="gemini-embedding-2", dimensions=2)

    cfg = _Config()
    cfg.VOICE_INDEX_PATH = str(path)
    cfg.GEMINI_OUTPUT_DIM = 3

    registered: list[object] = []
    monkeypatch.setattr(
        "src.main.voice_pb2_grpc.add_VoiceServiceServicer_to_server",
        lambda servicer, server: registered.append(servicer),
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    try:
        register_voice_servicer(server, cfg)
    finally:
        server.stop(grace=0)

    assert registered == []


def test_register_voice_servicer_registers_when_compatible(monkeypatch, tmp_path: Path) -> None:
    path = tmp_path / "voice_index.pkl"
    _build_voice_index(path, provider="gemini_api_key", model="gemini-embedding-2", dimensions=2)

    cfg = _Config()
    cfg.VOICE_INDEX_PATH = str(path)

    class _GeminiClient:
        def __init__(self, **kwargs) -> None:
            self.kwargs = kwargs

        async def embed_queries_batch(self, queries):  # pragma: no cover - not used in this test
            return [np.array([1.0, 0.0], dtype=np.float32) for _ in queries]

    fake_module = types.ModuleType("src.embeddings.gemini_client")
    fake_module.GeminiClient = _GeminiClient
    monkeypatch.setitem(sys.modules, "src.embeddings.gemini_client", fake_module)

    registered: list[object] = []
    monkeypatch.setattr(
        "src.main.voice_pb2_grpc.add_VoiceServiceServicer_to_server",
        lambda servicer, server: registered.append(servicer),
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    try:
        register_voice_servicer(server, cfg)
    finally:
        server.stop(grace=0)

    assert len(registered) == 1


def test_register_voice_servicer_handles_legacy_index_without_source_metadata(
    monkeypatch,
    tmp_path: Path,
) -> None:
    path = tmp_path / "voice_index.pkl"
    index = VoiceIndex()
    index.fit(
        ids=["p1"],
        names=["Milk"],
        vectors=[np.array([1.0, 0.0], dtype=np.float32)],
    )
    # Simulate a legacy payload without source_* metadata.
    delattr(index, "source_provider")
    delattr(index, "source_model")
    delattr(index, "source_dimensions")
    index.save(path)

    cfg = _Config()
    cfg.VOICE_INDEX_PATH = str(path)

    class _GeminiClient:
        def __init__(self, **kwargs) -> None:
            self.kwargs = kwargs

        async def embed_queries_batch(self, queries):  # pragma: no cover - not used in this test
            return [np.array([1.0, 0.0], dtype=np.float32) for _ in queries]

    fake_module = types.ModuleType("src.embeddings.gemini_client")
    fake_module.GeminiClient = _GeminiClient
    monkeypatch.setitem(sys.modules, "src.embeddings.gemini_client", fake_module)

    registered: list[object] = []
    monkeypatch.setattr(
        "src.main.voice_pb2_grpc.add_VoiceServiceServicer_to_server",
        lambda servicer, server: registered.append(servicer),
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    try:
        register_voice_servicer(server, cfg)
    finally:
        server.stop(grace=0)

    assert len(registered) == 1


def test_register_voice_servicer_skips_on_corrupted_voice_index(
    monkeypatch,
    tmp_path: Path,
) -> None:
    path = tmp_path / "voice_index.pkl"
    path.write_bytes(b"not-a-pickle")

    cfg = _Config()
    cfg.VOICE_INDEX_PATH = str(path)

    registered: list[object] = []
    monkeypatch.setattr(
        "src.main.voice_pb2_grpc.add_VoiceServiceServicer_to_server",
        lambda servicer, server: registered.append(servicer),
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    try:
        register_voice_servicer(server, cfg)
    finally:
        server.stop(grace=0)

    assert registered == []


def test_register_voice_servicer_skips_on_invalid_loaded_object(monkeypatch, tmp_path: Path) -> None:
    cfg = _Config()
    cfg.VOICE_INDEX_PATH = str(tmp_path / "voice_index.pkl")

    monkeypatch.setattr("src.main.VoiceIndex.load", lambda _path: object())

    registered: list[object] = []
    monkeypatch.setattr(
        "src.main.voice_pb2_grpc.add_VoiceServiceServicer_to_server",
        lambda servicer, server: registered.append(servicer),
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    try:
        register_voice_servicer(server, cfg)
    finally:
        server.stop(grace=0)

    assert registered == []
