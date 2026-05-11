from __future__ import annotations

import json

import numpy as np
import pytest

from src.photo_search.embeddings import (
    GeminiAPIEmbeddingProvider,
    ProviderNotConfiguredError,
    VertexAIEmbeddingProvider,
)


def test_gemini_provider_requires_api_key() -> None:
    with pytest.raises(ProviderNotConfiguredError):
        GeminiAPIEmbeddingProvider(
            api_key="",
            model="gemini-embedding-2",
            dimensions=8,
        )


def test_vertex_provider_not_configured() -> None:
    provider = VertexAIEmbeddingProvider(
        project_id="demo-project",
        location="us-central1",
        model="gemini-embedding-2",
        dimensions=8,
    )
    with pytest.raises(ProviderNotConfiguredError):
        provider.embed_texts(["milk"])


def test_gemini_text_request_shape() -> None:
    provider = GeminiAPIEmbeddingProvider(
        api_key="test-key",
        model="gemini-embedding-2",
        dimensions=32,
    )

    payload = provider._build_texts_payload(["milk", "eggs"])

    assert set(payload.keys()) == {"requests"}
    assert payload["requests"][0]["content"]["parts"] == [{"text": "milk"}]
    assert payload["requests"][1]["content"]["parts"] == [{"text": "eggs"}]
    assert payload["requests"][0]["taskType"] == "SEMANTIC_SIMILARITY"
    assert payload["requests"][0]["outputDimensionality"] == 32
    assert payload["requests"][1]["outputDimensionality"] == 32


def test_gemini_multimodal_request_shape() -> None:
    provider = GeminiAPIEmbeddingProvider(
        api_key="test-key",
        model="gemini-embedding-2",
        dimensions=16,
    )

    payload = provider._build_multimodal_payload(
        [
            {"text": "milk", "image_bytes": b"\x89PNG", "mime_type": "image/png"},
            {"text": "eggs"},
        ]
    )

    assert set(payload.keys()) == {"requests"}
    first_parts = payload["requests"][0]["content"]["parts"]
    assert first_parts[0] == {"text": "milk"}
    assert first_parts[1]["inlineData"]["mimeType"] == "image/png"
    assert isinstance(first_parts[1]["inlineData"]["data"], str)
    assert payload["requests"][1]["content"]["parts"] == [{"text": "eggs"}]


def test_gemini_embed_texts_parses_embeddings(monkeypatch) -> None:
    provider = GeminiAPIEmbeddingProvider(
        api_key="test-key",
        model="gemini-embedding-2",
        dimensions=3,
    )
    captured: dict[str, object] = {}

    def fake_post(payload: dict) -> dict:
        captured["payload"] = payload
        return {
            "embeddings": [
                {"values": [1.0, 0.0, 0.5]},
                {"values": [0.1, 0.2, 0.3]},
            ]
        }

    monkeypatch.setattr(
        GeminiAPIEmbeddingProvider,
        "_post_json",
        lambda self, payload: fake_post(payload),
    )

    vectors = provider.embed_texts(["milk", "eggs"])

    assert np.allclose(vectors, np.array([[1.0, 0.0, 0.5], [0.1, 0.2, 0.3]], dtype=np.float32))
    assert json.dumps(captured["payload"])
