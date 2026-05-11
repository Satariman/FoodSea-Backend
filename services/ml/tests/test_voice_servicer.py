from src.proto import voice_pb2
from src.service import VoiceServicer
from src.voice.pipeline import ParseResponse, VoiceItem


class FakePipeline:
    def __init__(self, response: ParseResponse) -> None:
        self.response = response
        self.calls: list[tuple[str, str]] = []

    async def parse(self, text: str, locale: str) -> ParseResponse:
        self.calls.append((text, locale))
        return self.response


def test_servicer_returns_response_with_items():
    pipeline = FakePipeline(ParseResponse(
        items=[VoiceItem(
            product_id="x", product_name="Молоко 1л",
            quantity=2, unit="шт", confidence=0.91,
            raw_query="молоко",
        )],
        unmatched_queries=["погода"],
    ))
    servicer = VoiceServicer(pipeline=pipeline)
    request = voice_pb2.ParseShoppingListRequest(text="два молока погода", locale="ru-RU")
    response = servicer.ParseShoppingList(request, context=None)
    assert len(response.items) == 1
    assert response.items[0].product_id == "x"
    assert response.items[0].quantity == 2
    assert list(response.unmatched_queries) == ["погода"]
    assert pipeline.calls == [("два молока погода", "ru-RU")]


def test_servicer_defaults_locale_when_missing():
    pipeline = FakePipeline(ParseResponse(items=[], unmatched_queries=[]))
    servicer = VoiceServicer(pipeline=pipeline)
    request = voice_pb2.ParseShoppingListRequest(text="что-нибудь")
    servicer.ParseShoppingList(request, context=None)
    assert pipeline.calls == [("что-нибудь", "ru-RU")]
