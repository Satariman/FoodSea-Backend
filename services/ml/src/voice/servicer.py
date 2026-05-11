from __future__ import annotations

import asyncio

from src.proto import voice_pb2, voice_pb2_grpc
from src.voice.pipeline import VoicePipeline


class VoiceServicer(voice_pb2_grpc.VoiceServiceServicer):
    def __init__(self, pipeline: VoicePipeline) -> None:
        self.pipeline = pipeline

    def ParseShoppingList(self, request, context):  # noqa: N802 (grpc method naming)
        locale = request.locale or "ru-RU"
        result = asyncio.run(self.pipeline.parse(request.text, locale))
        return voice_pb2.ParseShoppingListResponse(
            items=[
                voice_pb2.VoiceItem(
                    product_id=item.product_id,
                    product_name=item.product_name,
                    quantity=item.quantity,
                    unit=item.unit,
                    confidence=item.confidence,
                    raw_query=item.raw_query,
                )
                for item in result.items
            ],
            unmatched_queries=result.unmatched_queries,
        )
