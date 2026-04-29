from __future__ import annotations

from concurrent import futures

import grpc
import numpy as np

from src.config import Config
from src.index import AnalogIndex
from src.proto import analogs_pb2, analogs_pb2_grpc
from src.service import AnalogServicer


def build_test_index() -> AnalogIndex:
    ids = ["a", "b", "c"]
    names = {"a": "Apple", "b": "Green Apple", "c": "Banana"}
    vectors = np.array(
        [
            [1.0, 0.0, 0.0],
            [0.95, 0.1, 0.0],
            [0.2, 1.0, 0.1],
        ],
        dtype=np.float32,
    )
    offers = {
        "a": {"store1": 1000},
        "b": {"store1": 900},
        "c": {"store2": 800},
    }

    index = AnalogIndex()
    index.build(ids, names, vectors, offers)
    return index


def start_test_server(index: AnalogIndex) -> tuple[grpc.Channel, analogs_pb2_grpc.AnalogServiceStub, grpc.Server]:
    config = Config()
    config.MIN_SCORE_THRESHOLD = 0.0
    config.PRICE_PENALTY = 0.3

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    analogs_pb2_grpc.add_AnalogServiceServicer_to_server(AnalogServicer(index, config), server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()

    channel = grpc.insecure_channel(f"127.0.0.1:{port}")
    grpc.channel_ready_future(channel).result(timeout=5)
    stub = analogs_pb2_grpc.AnalogServiceStub(channel)
    return channel, stub, server


def test_get_analogs_unknown_returns_empty() -> None:
    channel, stub, server = start_test_server(build_test_index())
    try:
        response = stub.GetAnalogs(analogs_pb2.GetAnalogsRequest(product_id="unknown", top_k=3))
        assert response.analogs == []
    finally:
        channel.close()
        server.stop(grace=0)


def test_get_analogs_known_price_aware() -> None:
    channel, stub, server = start_test_server(build_test_index())
    try:
        response = stub.GetAnalogs(
            analogs_pb2.GetAnalogsRequest(product_id="a", top_k=3, price_aware=True)
        )
        assert 1 <= len(response.analogs) <= 3
        assert all(analog.score > 0 for analog in response.analogs)
    finally:
        channel.close()
        server.stop(grace=0)


def test_get_batch_analogs_with_store_filter() -> None:
    channel, stub, server = start_test_server(build_test_index())
    try:
        response = stub.GetBatchAnalogs(
            analogs_pb2.GetBatchAnalogsRequest(
                product_ids=["a", "b"],
                top_k=2,
                filter_store_ids=["store1"],
            )
        )

        assert set(response.analogs_by_product.keys()) == {"a", "b"}
    finally:
        channel.close()
        server.stop(grace=0)
