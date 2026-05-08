import asyncio
from typing import Any

import numpy as np
from google import genai
from google.genai import types
from tenacity import (
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_exponential,
)


class GeminiClient:
    def __init__(
        self,
        api_key: str,
        model: str,
        output_dim: int,
        _client: Any | None = None,
    ) -> None:
        self.model = model
        self.output_dim = output_dim
        self._client = _client if _client is not None else genai.Client(api_key=api_key)

    async def embed_queries_batch(self, queries: list[str]) -> list[np.ndarray]:
        contents = [
            types.Content(parts=[types.Part.from_text(text=f"task: search result | query: {q}")])
            for q in queries
        ]
        result = await asyncio.to_thread(self._embed_with_retry, contents)
        return [np.asarray(e.values, dtype=np.float32) for e in result.embeddings]

    async def embed_product(
        self,
        name: str,
        brand: str,
        category: str,
        image_bytes: bytes | None,
    ) -> np.ndarray:
        text = f"title: {category} | text: {name} {brand}".strip()
        parts = [types.Part.from_text(text=text)]
        if image_bytes:
            parts.append(types.Part.from_bytes(data=image_bytes, mime_type="image/jpeg"))
        contents = [types.Content(parts=parts)]
        result = await asyncio.to_thread(self._embed_with_retry, contents)
        return np.asarray(result.embeddings[0].values, dtype=np.float32)

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=0.5, min=0.5, max=4),
        retry=retry_if_exception_type(Exception),
        reraise=True,
    )
    def _embed_with_retry(self, contents: list[Any]) -> Any:
        return self._client.models.embed_content(
            model=self.model,
            contents=contents,
            config=types.EmbedContentConfig(output_dimensionality=self.output_dim),
        )
