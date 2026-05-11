from __future__ import annotations

import numpy as np
import grpc

from src.config import Config
from src.index import AnalogIndex
from src.main import build_photo_search
from src.photo_search.index import PhotoProductIndex, PhotoProductMeta
from src.photo_search.service import (
    PhotoSearchEngine,
    PhotoSearchIndexNotReady,
)
from src.proto import analogs_pb2
from src.service import AnalogServicer, PhotoSearchState


class StubEmbeddingProvider:
    provider_name = "stub"
    model = "stub-model"
    dimensions = 3

    def __init__(self, vector: np.ndarray | None = None, error: Exception | None = None) -> None:
        self._vector = vector if vector is not None else np.array([1.0, 0.0, 0.0], dtype=np.float32)
        self._error = error

    def embed_texts(self, texts: list[str]) -> np.ndarray:
        raise NotImplementedError

    def embed_multimodal(self, items: list[dict[str, object]]) -> np.ndarray:
        if self._error is not None:
            raise self._error
        return np.asarray([self._vector], dtype=np.float32)


class DummyContext:
    def __init__(self) -> None:
        self.code = None
        self.details = ""

    def set_code(self, code):  # noqa: ANN001
        self.code = code

    def set_details(self, details: str) -> None:
        self.details = details


def build_photo_index() -> PhotoProductIndex:
    metas = [
        PhotoProductMeta("a", "Coca Cola Zero", "Coca Cola", "Drinks", "Soda", "a.jpg"),
        PhotoProductMeta("b", "Pepsi Max", "Pepsi", "Drinks", "Soda", "b.jpg"),
        PhotoProductMeta("c", "Sprite", "Coca Cola", "Drinks", "Soda", "c.jpg"),
    ]
    vectors = np.array(
        [
            [0.80, 0.20, 0.00],
            [0.99, 0.10, 0.00],
            [0.30, 0.70, 0.00],
        ],
        dtype=np.float32,
    )
    index = PhotoProductIndex()
    index.build(
        metas=metas,
        vectors=vectors,
        provider="stub",
        model="stub-model",
        dimensions=3,
    )
    return index


def test_photo_search_returns_ranked_candidates_with_brand_name_bias() -> None:
    index = build_photo_index()
    provider = StubEmbeddingProvider(vector=np.array([1.0, 0.0, 0.0], dtype=np.float32))
    engine = PhotoSearchEngine(index=index, provider=provider, config=Config())

    result = engine.search(
        image=b"raw-image",
        mime_type="image/jpeg",
        ocr_text="coca cola zero sugar",
        top_k=3,
    )

    assert result.matched_brand == "Coca Cola"
    assert result.matched_name == "Coca Cola Zero"
    assert [candidate.product_id for candidate in result.candidates] == ["a", "b", "c"]
    assert [candidate.score for candidate in result.candidates] == sorted(
        [candidate.score for candidate in result.candidates],
        reverse=True,
    )


def test_photo_search_clamps_boosted_scores_into_unit_interval() -> None:
    index = build_photo_index()
    provider = StubEmbeddingProvider(vector=np.array([0.80, 0.20, 0.00], dtype=np.float32))
    engine = PhotoSearchEngine(index=index, provider=provider, config=Config())

    result = engine.search(
        image=b"raw-image",
        mime_type="image/jpeg",
        ocr_text="coca cola zero sugar",
        top_k=3,
    )

    assert result.candidates[0].product_id == "a"
    assert result.candidates[0].score == 1.0
    assert all(0.0 <= candidate.score <= 1.0 for candidate in result.candidates)


def test_photo_search_raises_when_index_is_missing() -> None:
    provider = StubEmbeddingProvider()
    engine = PhotoSearchEngine(index=None, provider=provider, config=Config())

    try:
        engine.search(image=b"raw-image", mime_type="image/png", ocr_text="sprite", top_k=2)
        assert False, "expected PhotoSearchIndexNotReady"
    except PhotoSearchIndexNotReady:
        pass


def test_search_by_photo_servicer_validation_and_error_mapping() -> None:
    cfg = Config()
    analog_index = AnalogIndex()
    analog_index.build(
        product_ids=["x"],
        names={"x": "X"},
        vectors=np.array([[1.0, 0.0, 0.0]], dtype=np.float32),
        offers={"x": {"store1": 100}},
    )
    servicer = AnalogServicer(
        index=analog_index,
        config=cfg,
        photo_search=None,
        photo_search_state=PhotoSearchState.DISABLED,
    )

    no_image_ctx = DummyContext()
    no_image_resp = servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(image=b"", image_mime_type="image/jpeg", ocr_text="x"),
        no_image_ctx,
    )
    assert isinstance(no_image_resp, analogs_pb2.SearchByPhotoResponse)
    assert no_image_ctx.code == grpc.StatusCode.INVALID_ARGUMENT
    assert "image is required" in no_image_ctx.details

    bad_mime_ctx = DummyContext()
    servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(image=b"1", image_mime_type="image/webp", ocr_text="x"),
        bad_mime_ctx,
    )
    assert bad_mime_ctx.code == grpc.StatusCode.INVALID_ARGUMENT
    assert "image_mime_type must be image/jpeg or image/png" in bad_mime_ctx.details

    no_ocr_ctx = DummyContext()
    servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(image=b"1", image_mime_type="image/png", ocr_text=""),
        no_ocr_ctx,
    )
    assert no_ocr_ctx.code == grpc.StatusCode.INVALID_ARGUMENT
    assert "ocr_text is required" in no_ocr_ctx.details

    disabled_ctx = DummyContext()
    servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(image=b"1", image_mime_type="image/png", ocr_text="x"),
        disabled_ctx,
    )
    assert disabled_ctx.code == grpc.StatusCode.FAILED_PRECONDITION
    assert disabled_ctx.details == "photo search is disabled"

    unready_servicer = AnalogServicer(
        index=analog_index,
        config=cfg,
        photo_search=None,
        photo_search_state=PhotoSearchState.UNREADY,
    )
    unready_ctx = DummyContext()
    unready_servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(image=b"1", image_mime_type="image/png", ocr_text="x"),
        unready_ctx,
    )
    assert unready_ctx.code == grpc.StatusCode.FAILED_PRECONDITION
    assert unready_ctx.details == "photo search is not ready"

    failing_engine = PhotoSearchEngine(
        index=build_photo_index(),
        provider=StubEmbeddingProvider(error=ConnectionError("provider down")),
        config=Config(),
    )
    enabled_servicer = AnalogServicer(
        index=analog_index,
        config=cfg,
        photo_search=failing_engine,
        photo_search_state=PhotoSearchState.READY,
    )
    provider_error_ctx = DummyContext()
    enabled_servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(
            image=b"1",
            image_mime_type="image/png",
            ocr_text="coca cola",
        ),
        provider_error_ctx,
    )
    assert provider_error_ctx.code == grpc.StatusCode.UNAVAILABLE
    assert "photo search provider error" in provider_error_ctx.details

    local_error_engine = PhotoSearchEngine(
        index=build_photo_index(),
        provider=StubEmbeddingProvider(error=ValueError("bad local transform")),
        config=Config(),
    )
    local_error_servicer = AnalogServicer(
        index=analog_index,
        config=cfg,
        photo_search=local_error_engine,
        photo_search_state=PhotoSearchState.READY,
    )
    local_error_ctx = DummyContext()
    local_error_servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(
            image=b"1",
            image_mime_type="image/png",
            ocr_text="coca cola",
        ),
        local_error_ctx,
    )
    assert local_error_ctx.code == grpc.StatusCode.INTERNAL
    assert local_error_ctx.details == "photo search internal error"


def test_search_by_photo_servicer_maps_index_not_ready() -> None:
    cfg = Config()
    analog_index = AnalogIndex()
    analog_index.build(
        product_ids=["x"],
        names={"x": "X"},
        vectors=np.array([[1.0, 0.0, 0.0]], dtype=np.float32),
        offers={"x": {"store1": 100}},
    )
    engine = PhotoSearchEngine(index=None, provider=StubEmbeddingProvider(), config=cfg)
    servicer = AnalogServicer(
        index=analog_index,
        config=cfg,
        photo_search=engine,
        photo_search_state=PhotoSearchState.READY,
    )

    ctx = DummyContext()
    response = servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(
            image=b"1",
            image_mime_type="image/png",
            ocr_text="sprite",
            top_k=2,
        ),
        ctx,
    )
    assert isinstance(response, analogs_pb2.SearchByPhotoResponse)
    assert ctx.code == grpc.StatusCode.FAILED_PRECONDITION
    assert ctx.details == "photo search index is not ready"


def test_search_by_photo_servicer_success_payload() -> None:
    cfg = Config()
    analog_index = AnalogIndex()
    analog_index.build(
        product_ids=["x"],
        names={"x": "X"},
        vectors=np.array([[1.0, 0.0, 0.0]], dtype=np.float32),
        offers={"x": {"store1": 100}},
    )
    engine = PhotoSearchEngine(
        index=build_photo_index(),
        provider=StubEmbeddingProvider(vector=np.array([1.0, 0.0, 0.0], dtype=np.float32)),
        config=cfg,
    )
    servicer = AnalogServicer(
        index=analog_index,
        config=cfg,
        photo_search=engine,
        photo_search_state=PhotoSearchState.READY,
    )

    ctx = DummyContext()
    response = servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(
            image=b"raw-image",
            image_mime_type="image/jpeg",
            ocr_text="coca cola zero sugar",
            top_k=3,
        ),
        ctx,
    )
    assert isinstance(response, analogs_pb2.SearchByPhotoResponse)
    assert ctx.code is None
    assert ctx.details == ""
    assert response.matched_name == "Coca Cola Zero"
    assert response.matched_brand == "Coca Cola"
    assert [candidate.product_id for candidate in response.candidates] == ["a", "b", "c"]
    assert len(response.candidates) == 3


def test_build_photo_search_vertex_provider_is_unready_and_servicer_returns_not_ready(
    monkeypatch,
) -> None:
    monkeypatch.setenv("PHOTO_SEARCH_ENABLED", "true")
    monkeypatch.setenv("PHOTO_SEARCH_PROVIDER", "vertex_ai")

    photo_search, state = build_photo_search(Config())

    assert photo_search is None
    assert state == PhotoSearchState.UNREADY

    analog_index = AnalogIndex()
    analog_index.build(
        product_ids=["x"],
        names={"x": "X"},
        vectors=np.array([[1.0, 0.0, 0.0]], dtype=np.float32),
        offers={"x": {"store1": 100}},
    )
    servicer = AnalogServicer(
        index=analog_index,
        config=Config(),
        photo_search=photo_search,
        photo_search_state=state,
    )

    ctx = DummyContext()
    servicer.SearchByPhoto(
        analogs_pb2.SearchByPhotoRequest(
            image=b"1",
            image_mime_type="image/png",
            ocr_text="x",
        ),
        ctx,
    )
    assert ctx.code == grpc.StatusCode.FAILED_PRECONDITION
    assert ctx.details == "photo search is not ready"
