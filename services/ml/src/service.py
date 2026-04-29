"""gRPC service implementation for analog recommendations."""

from __future__ import annotations

import grpc

from src.config import Config
from src.index import AnalogIndex
from src.proto import analogs_pb2, analogs_pb2_grpc


class AnalogServicer(analogs_pb2_grpc.AnalogServiceServicer):
    def __init__(self, index: AnalogIndex, config: Config) -> None:
        self.index = index
        self.config = config

    def GetAnalogs(self, request, context):  # noqa: N802 (grpc method naming)
        if not request.product_id:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("product_id is required")
            return analogs_pb2.GetAnalogsResponse()

        top_k = request.top_k if request.top_k > 0 else 5
        filter_stores = set(request.filter_store_ids) if request.filter_store_ids else None

        results = self.index.query(
            product_id=request.product_id,
            top_k=top_k,
            price_aware=request.price_aware,
            filter_store_ids=filter_stores,
            price_penalty=self.config.PRICE_PENALTY,
        )

        analogs = [
            analogs_pb2.AnalogProto(
                product_id=product_id,
                product_name=product_name,
                score=score,
                min_price_kopecks=min_price,
            )
            for product_id, product_name, score, min_price in results
            if score >= self.config.MIN_SCORE_THRESHOLD
        ]

        return analogs_pb2.GetAnalogsResponse(analogs=analogs)

    def GetBatchAnalogs(self, request, context):  # noqa: N802 (grpc method naming)
        if not request.product_ids:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("product_ids must not be empty")
            return analogs_pb2.GetBatchAnalogsResponse()

        top_k = request.top_k if request.top_k > 0 else 5
        filter_stores = set(request.filter_store_ids) if request.filter_store_ids else None

        response = analogs_pb2.GetBatchAnalogsResponse()
        for source_product_id in request.product_ids:
            results = self.index.query(
                product_id=source_product_id,
                top_k=top_k,
                price_aware=False,
                filter_store_ids=filter_stores,
                price_penalty=self.config.PRICE_PENALTY,
            )

            analogs = [
                analogs_pb2.AnalogProto(
                    product_id=product_id,
                    product_name=product_name,
                    score=score,
                    min_price_kopecks=min_price,
                )
                for product_id, product_name, score, min_price in results
                if score >= self.config.MIN_SCORE_THRESHOLD
            ]
            response.analogs_by_product[source_product_id].analogs.extend(analogs)

        return response
