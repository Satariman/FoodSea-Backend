import asyncio
from collections import OrderedDict
from dataclasses import dataclass
import re

from src.voice.matcher import NGramMatch, VoiceMatcher
from src.voice.ngram import Segment, build_segments
from src.voice.tokenizer import tokenize


@dataclass(frozen=True)
class VoiceItem:
    product_id: str
    product_name: str
    quantity: int
    unit: str
    confidence: float
    raw_query: str


@dataclass(frozen=True)
class ParseResponse:
    items: list[VoiceItem]
    unmatched_queries: list[str]


class VoicePipeline:
    def __init__(self, matcher: VoiceMatcher) -> None:
        self.matcher = matcher

    async def parse(self, text: str, locale: str) -> ParseResponse:
        tokens = tokenize(text)
        segments = self._refine_segments(build_segments(tokens))
        if not segments:
            return ParseResponse(items=[], unmatched_queries=[])

        seg_results = await asyncio.gather(*[self.matcher.match_segment(s) for s in segments])

        items: list[VoiceItem] = []
        unmatched: list[str] = []
        for seg, ngram_matches in zip(segments, seg_results):
            chosen = self._greedy_assign(ngram_matches)
            if not chosen:
                unmatched.append(" ".join(seg.words))
                continue
            for nm in chosen:
                quantity = seg.quantity
                if isinstance(quantity, float):
                    quantity_int = int(quantity)
                else:
                    quantity_int = quantity
                items.append(VoiceItem(
                    product_id=nm.match.product_id,
                    product_name=nm.match.product_name,
                    quantity=quantity_int,
                    unit=seg.unit or "шт",
                    confidence=nm.match.score,
                    raw_query=nm.ngram.text,
                ))
        return ParseResponse(items=self._deduplicate(items), unmatched_queries=unmatched)

    @staticmethod
    def _refine_segments(segments: list[Segment]) -> list[Segment]:
        refined: list[Segment] = []
        for segment in segments:
            refined.extend(VoicePipeline._split_unit_segment_before_percent_tail(segment))
        return refined

    @staticmethod
    def _split_unit_segment_before_percent_tail(segment: Segment) -> list[Segment]:
        if segment.unit is None or len(segment.words) < 3:
            return [segment]

        percent_idx = next(
            (idx for idx, word in enumerate(segment.words) if _looks_like_percent_token(word)),
            None,
        )
        if percent_idx is None or percent_idx < 2:
            return [segment]

        split_idx = percent_idx - 1
        left_words = segment.words[:split_idx]
        right_words = segment.words[split_idx:]
        if not left_words or not right_words:
            return [segment]

        return [
            Segment(
                quantity=segment.quantity,
                unit=segment.unit,
                words=left_words,
            ),
            Segment(
                quantity=1,
                unit=None,
                words=right_words,
            ),
        ]

    @staticmethod
    def _greedy_assign(matches: list[NGramMatch]) -> list[NGramMatch]:
        sorted_ms = sorted(matches, key=lambda nm: -nm.match.score)
        covered: set[int] = set()
        chosen: list[NGramMatch] = []
        for nm in sorted_ms:
            span = set(range(nm.ngram.start, nm.ngram.end))
            if span & covered:
                continue
            chosen.append(nm)
            covered |= span
        chosen.sort(key=lambda nm: nm.ngram.start)
        return chosen

    @staticmethod
    def _deduplicate(items: list[VoiceItem]) -> list[VoiceItem]:
        """Merge items with the same (product_id, unit): sum quantities, keep
        highest confidence, concatenate raw queries. Preserves first-seen order.
        """
        merged: OrderedDict[tuple[str, str], VoiceItem] = OrderedDict()
        for item in items:
            key = (item.product_id, item.unit)
            existing = merged.get(key)
            if existing is None:
                merged[key] = item
                continue
            merged[key] = VoiceItem(
                product_id=existing.product_id,
                product_name=existing.product_name,
                quantity=existing.quantity + item.quantity,
                unit=existing.unit,
                confidence=max(existing.confidence, item.confidence),
                raw_query=f"{existing.raw_query}, {item.raw_query}",
            )
        return list(merged.values())


_PERCENT_TOKEN_RE = re.compile(r"^\d+(?:[.,]\d+)?%$")


def _looks_like_percent_token(word: str) -> bool:
    return bool(_PERCENT_TOKEN_RE.match(word))
