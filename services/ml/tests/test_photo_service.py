from __future__ import annotations

import numpy as np
import grpc

from src.config import Config
from src.index import AnalogIndex
from src.photo_search.index import PhotoProductIndex, PhotoProductMeta
from src.photo_search.service import (
    PhotoSearchEngine,
    PhotoSearchIndexNotReady,
)
from src.proto import analogs_pb2
from src.service import AnalogServicer


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
    servicer = AnalogServicer(index=analog_index, config=cfg, photo_search=None)

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

    failing_engine = PhotoSearchEngine(
        index=build_photo_index(),
        provider=StubEmbeddingProvider(error=RuntimeError("provider down")),
        config=Config(),
    )
    enabled_servicer = AnalogServicer(index=analog_index, config=cfg, photo_search=failing_engine)
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
