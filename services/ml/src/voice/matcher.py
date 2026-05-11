from dataclasses import dataclass

import numpy as np

from src.embeddings.cache import EmbeddingCache
from src.embeddings.gemini_client import GeminiClient
from src.voice.ngram import NGram, Segment, generate_ngrams
from src.voice_index.index import Match, VoiceIndex


@dataclass(frozen=True)
class NGramMatch:
    ngram: NGram
    match: Match


class VoiceMatcher:
    def __init__(
        self,
        index: VoiceIndex,
        gemini: GeminiClient,
        cache: EmbeddingCache,
        min_score: float,
        max_ngram_len: int = 3,
    ) -> None:
        self.index = index
        self.gemini = gemini
        self.cache = cache
        self.min_score = min_score
        self.max_ngram_len = max_ngram_len

    async def match_segment(self, segment: Segment) -> list[NGramMatch]:
        ngrams = generate_ngrams(segment, max_n=self.max_ngram_len)
        if not ngrams:
            return []

        unique_missing: list[str] = []
        seen: set[str] = set()
        cached_vecs: dict[str, np.ndarray] = {}
        for ng in ngrams:
            cached = self.cache.get(ng.text)
            if cached is not None:
                cached_vecs[ng.text] = cached
            elif ng.text not in seen:
                unique_missing.append(ng.text)
                seen.add(ng.text)

        if unique_missing:
            new_vecs = await self.gemini.embed_queries_batch(unique_missing)
            for text, vec in zip(unique_missing, new_vecs):
                self.cache.put(text, vec)
                cached_vecs[text] = vec

        results: list[NGramMatch] = []
        for ng in ngrams:
            vec = cached_vecs[ng.text]
            top = self.index.query(vec, top_k=1)
            if top and top[0].score >= self.min_score:
                results.append(NGramMatch(ngram=ng, match=top[0]))
        return results
