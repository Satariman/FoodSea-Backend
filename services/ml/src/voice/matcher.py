from dataclasses import dataclass
import re

import numpy as np

from src.embeddings.cache import EmbeddingCache
from src.embeddings.gemini_client import GeminiClient
from src.voice.ngram import NGram, Segment, generate_ngrams
from src.voice.index import Match, VoiceIndex


@dataclass(frozen=True)
class NGramMatch:
    ngram: NGram
    match: Match
    semantic_score: float | None = None
    rerank_score: float | None = None


_TOKEN_RE = re.compile(r"[a-zа-я0-9%]+", flags=re.IGNORECASE)
_PERCENT_RE = re.compile(r"(\d+(?:[.,]\d+)?)\s*%")
_VOLUME_RE = re.compile(r"(\d+(?:[.,]\d+)?)\s*(мл|л|г|гр|кг)\b", flags=re.IGNORECASE)


class VoiceMatcher:
    def __init__(
        self,
        index: VoiceIndex,
        gemini: GeminiClient,
        cache: EmbeddingCache,
        min_score: float,
        max_ngram_len: int = 3,
        rerank_mode: str = "legacy",
        rerank_candidates_k: int = 5,
    ) -> None:
        self.index = index
        self.gemini = gemini
        self.cache = cache
        self.min_score = min_score
        self.max_ngram_len = max_ngram_len
        self.rerank_mode = rerank_mode
        self.rerank_candidates_k = max(1, int(rerank_candidates_k))
        if self.rerank_mode not in {"legacy", "attribute_aware"}:
            raise ValueError(f"unsupported rerank_mode: {self.rerank_mode}")

    async def match_segment(self, segment: Segment) -> list[NGramMatch]:
        ngrams = generate_ngrams(segment, max_n=self.max_ngram_len)
        if not ngrams:
            return []
        min_score = self._min_score_for_segment(segment)

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
            top_k = 1 if self.rerank_mode == "legacy" else self.rerank_candidates_k
            top = self.index.query(vec, top_k=top_k)
            if not top:
                continue
            if self.rerank_mode == "legacy":
                if top[0].score >= min_score:
                    results.append(NGramMatch(ngram=ng, match=top[0]))
                continue

            best = self._best_reranked(query_text=ng.text, candidates=top)
            if best.rerank_score < min_score:
                continue
            results.append(
                NGramMatch(
                    ngram=ng,
                    match=Match(
                        product_id=best.candidate.product_id,
                        product_name=best.candidate.product_name,
                        score=best.rerank_score,
                    ),
                    semantic_score=best.candidate.score,
                    rerank_score=best.rerank_score,
                )
            )
        return results

    def _min_score_for_segment(self, segment: Segment) -> float:
        if self.rerank_mode == "attribute_aware" and segment.unit is not None:
            return min(self.min_score, 0.62)
        return self.min_score

    @dataclass(frozen=True)
    class _RerankCandidate:
        candidate: Match
        rerank_score: float

    def _best_reranked(self, query_text: str, candidates: list[Match]) -> _RerankCandidate:
        query_tokens = _tokenize_text(query_text)
        query_fat = _extract_percent(query_text)
        query_volume = _extract_volume(query_text)

        reranked: list[VoiceMatcher._RerankCandidate] = []
        for candidate in candidates:
            candidate_text = candidate.product_name
            candidate_tokens = _tokenize_text(candidate_text)
            candidate_fat = _extract_percent(candidate_text)
            candidate_volume = _extract_volume(candidate_text)
            score = _rerank_score(
                semantic_score=candidate.score,
                query_tokens=query_tokens,
                product_tokens=candidate_tokens,
                query_fat=query_fat,
                product_fat=candidate_fat,
                query_volume=query_volume,
                product_volume=candidate_volume,
            )
            reranked.append(VoiceMatcher._RerankCandidate(candidate=candidate, rerank_score=score))

        reranked.sort(
            key=lambda item: (
                item.rerank_score,
                item.candidate.score,
                item.candidate.product_name,
                item.candidate.product_id,
            ),
            reverse=True,
        )
        return reranked[0]


def _tokenize_text(text: str) -> tuple[str, ...]:
    return tuple(_TOKEN_RE.findall(text.lower().replace("ё", "е")))


def _extract_percent(text: str) -> float | None:
    match = _PERCENT_RE.search(text.lower().replace(",", "."))
    if match is None:
        return None
    return float(match.group(1))


def _extract_volume(text: str) -> tuple[float, str] | None:
    match = _VOLUME_RE.search(text.lower().replace(",", "."))
    if match is None:
        return None
    value = float(match.group(1))
    unit = match.group(2).lower()
    if unit == "гр":
        unit = "г"
    return value, unit


def _token_overlap(query_tokens: tuple[str, ...], product_tokens: tuple[str, ...]) -> float:
    if not query_tokens or not product_tokens:
        return 0.0
    q = set(query_tokens)
    p = set(product_tokens)
    if not q or not p:
        return 0.0
    return len(q & p) / len(q)


def _normalize_volume(value: float, unit: str) -> tuple[float, str]:
    if unit == "кг":
        return value * 1000.0, "г"
    if unit == "л":
        return value * 1000.0, "мл"
    return value, unit


def _volume_similarity(
    query_volume: tuple[float, str] | None,
    product_volume: tuple[float, str] | None,
) -> float:
    if query_volume is None:
        return 0.0
    if product_volume is None:
        return -0.02

    qv, qu = _normalize_volume(*query_volume)
    pv, pu = _normalize_volume(*product_volume)
    if qu != pu:
        return -0.06

    if qv <= 0.0 or pv <= 0.0:
        return 0.0
    ratio = min(qv, pv) / max(qv, pv)
    if ratio >= 0.9:
        return 0.08
    if ratio >= 0.75:
        return 0.04
    return -0.06


def _fat_similarity(query_fat: float | None, product_fat: float | None) -> float:
    if query_fat is None:
        return 0.0
    if product_fat is None:
        return -0.03
    if abs(query_fat - product_fat) < 0.01:
        return 0.14
    return -0.12


def _rerank_score(
    *,
    semantic_score: float,
    query_tokens: tuple[str, ...],
    product_tokens: tuple[str, ...],
    query_fat: float | None,
    product_fat: float | None,
    query_volume: tuple[float, str] | None,
    product_volume: tuple[float, str] | None,
) -> float:
    token_bonus = 0.06 * _token_overlap(query_tokens, product_tokens)
    fat_bonus = _fat_similarity(query_fat, product_fat)
    volume_bonus = _volume_similarity(query_volume, product_volume)
    score = semantic_score + token_bonus + fat_bonus + volume_bonus
    return float(max(0.0, min(1.0, score)))
