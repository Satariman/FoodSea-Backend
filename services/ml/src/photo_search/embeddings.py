from __future__ import annotations

import base64
import json
from dataclasses import dataclass, field
from typing import Protocol
from urllib import error, request

import numpy as np


class ProviderNotConfiguredError(RuntimeError):
    pass


class EmbeddingProvider(Protocol):
    provider_name: str
    model: str
    dimensions: int

    def embed_texts(self, texts: list[str]) -> np.ndarray: ...

    def embed_multimodal(self, items: list[dict[str, object]]) -> np.ndarray: ...


@dataclass(slots=True)
class GeminiAPIEmbeddingProvider:
    api_key: str | None
    model: str
    dimensions: int
    task_type: str = "SEMANTIC_SIMILARITY"
    provider_name: str = field(init=False, default="gemini_api_key")
    _endpoint: str = field(init=False)

    def __post_init__(self) -> None:
        if not self.api_key:
            raise ProviderNotConfiguredError("GEMINI_API_KEY is required for gemini_api_key provider")
        self._endpoint = (
            "https://generativelanguage.googleapis.com/v1beta/models/"
            f"{self.model}:batchEmbedContents?key={self.api_key}"
        )

    def _build_texts_payload(self, texts: list[str]) -> dict[str, object]:
        requests = []
        for text in texts:
            requests.append(
                {
                    "model": f"models/{self.model}",
                    "content": {"parts": [{"text": text}]},
                    "taskType": self.task_type,
                    "outputDimensionality": int(self.dimensions),
                }
            )
        return {
            "model": f"models/{self.model}",
            "config": {"outputDimensionality": int(self.dimensions)},
            "requests": requests,
        }

    def _build_multimodal_payload(self, items: list[dict[str, object]]) -> dict[str, object]:
        requests = []
        for item in items:
            parts: list[dict[str, object]] = []
            text = item.get("text")
            if isinstance(text, str) and text:
                parts.append({"text": text})
            image_bytes = item.get("image_bytes")
            if isinstance(image_bytes, (bytes, bytearray)):
                mime_type = str(item.get("mime_type") or "image/jpeg")
                parts.append(
                    {
                        "inlineData": {
                            "mimeType": mime_type,
                            "data": base64.b64encode(bytes(image_bytes)).decode("ascii"),
                        }
                    }
                )
            requests.append(
                {
                    "model": f"models/{self.model}",
                    "content": {"parts": parts or [{"text": ""}]},
                    "taskType": self.task_type,
                    "outputDimensionality": int(self.dimensions),
                }
            )
        return {
            "model": f"models/{self.model}",
            "config": {"outputDimensionality": int(self.dimensions)},
            "requests": requests,
        }

    def _post_json(self, payload: dict[str, object]) -> dict[str, object]:
        body = json.dumps(payload).encode("utf-8")
        req = request.Request(
            self._endpoint,
            data=body,
            method="POST",
            headers={"Content-Type": "application/json"},
        )
        try:
            with request.urlopen(req, timeout=60) as resp:
                raw = resp.read()
        except error.URLError as exc:
            raise RuntimeError("gemini embedding request failed") from exc
        data = json.loads(raw.decode("utf-8"))
        if not isinstance(data, dict):
            raise RuntimeError("invalid gemini embedding response")
        return data

    def _parse_embeddings(self, data: dict[str, object]) -> np.ndarray:
        raw_embeddings = data.get("embeddings")
        if not isinstance(raw_embeddings, list):
            raise RuntimeError("missing embeddings in response")
        vectors: list[np.ndarray] = []
        for item in raw_embeddings:
            if not isinstance(item, dict):
                raise RuntimeError("invalid embedding item shape")
            values = item.get("values")
            if not isinstance(values, list):
                raise RuntimeError("invalid embedding values")
            vec = np.asarray(values, dtype=np.float32)
            if vec.ndim != 1:
                raise RuntimeError("embedding vector must be 1D")
            vectors.append(vec)
        if not vectors:
            return np.empty((0, int(self.dimensions)), dtype=np.float32)
        matrix = np.stack(vectors, axis=0)
        if matrix.shape[1] != int(self.dimensions):
            raise RuntimeError("embedding dimensions mismatch")
        return matrix

    def embed_texts(self, texts: list[str]) -> np.ndarray:
        if not texts:
            return np.empty((0, int(self.dimensions)), dtype=np.float32)
        payload = self._build_texts_payload(texts)
        data = self._post_json(payload)
        matrix = self._parse_embeddings(data)
        if matrix.shape[0] != len(texts):
            raise RuntimeError("embedding count mismatch for text batch")
        return matrix

    def embed_multimodal(self, items: list[dict[str, object]]) -> np.ndarray:
        if not items:
            return np.empty((0, int(self.dimensions)), dtype=np.float32)
        payload = self._build_multimodal_payload(items)
        data = self._post_json(payload)
        matrix = self._parse_embeddings(data)
        if matrix.shape[0] != len(items):
            raise RuntimeError("embedding count mismatch for multimodal batch")
        return matrix


@dataclass(slots=True)
class VertexAIEmbeddingProvider:
    project_id: str | None
    location: str
    model: str
    dimensions: int
    provider_name: str = field(init=False, default="vertex_ai")

    def embed_texts(self, texts: list[str]) -> np.ndarray:
        raise ProviderNotConfiguredError("vertex provider is not configured yet")

    def embed_multimodal(self, items: list[dict[str, object]]) -> np.ndarray:
        raise ProviderNotConfiguredError("vertex provider is not configured yet")
