"""gRPC service implementation for analog recommendations."""

from __future__ import annotations

import grpc

from src.config import Config
from src.index import AnalogIndex
from src.photo_search.service import PhotoSearchEngine, PhotoSearchIndexNotReady
from src.proto import analogs_pb2, analogs_pb2_grpc


class AnalogServicer(analogs_pb2_grpc.AnalogServiceServicer):
    def __init__(
        self,
        index: AnalogIndex,
        config: Config,
        photo_search: PhotoSearchEngine | None = None,
    ) -> None:
        self.index = index
        self.config = config
        self.photo_search = photo_search

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

    def SearchByPhoto(self, request, context):  # noqa: N802 (grpc method naming)
        if not request.image:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("image is required")
            return analogs_pb2.SearchByPhotoResponse()
        if request.image_mime_type not in {"image/jpeg", "image/png"}:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("image_mime_type must be image/jpeg or image/png")
            return analogs_pb2.SearchByPhotoResponse()
        if not request.ocr_text:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("ocr_text is required")
            return analogs_pb2.SearchByPhotoResponse()
        if self.photo_search is None:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            context.set_details("photo search is disabled")
            return analogs_pb2.SearchByPhotoResponse()

        top_k = request.top_k if request.top_k > 0 else 5
        try:
            result = self.photo_search.search(
                image=bytes(request.image),
                mime_type=request.image_mime_type,
                ocr_text=request.ocr_text,
                top_k=top_k,
            )
        except PhotoSearchIndexNotReady:
            context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
            context.set_details("photo search index is not ready")
            return analogs_pb2.SearchByPhotoResponse()
        except Exception as exc:  # noqa: BLE001
            context.set_code(grpc.StatusCode.UNAVAILABLE)
            context.set_details(f"photo search provider error: {exc}")
            return analogs_pb2.SearchByPhotoResponse()

        return analogs_pb2.SearchByPhotoResponse(
            matched_name=result.matched_name,
            matched_brand=result.matched_brand,
            candidates=[
                analogs_pb2.PhotoSearchCandidate(
                    product_id=item.product_id,
                    score=item.score,
                )
                for item in result.candidates
            ],
        )
