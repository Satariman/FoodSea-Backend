import asyncio
from dataclasses import dataclass

from src.voice.matcher import NGramMatch, VoiceMatcher
from src.voice.ngram import build_segments
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
        segments = build_segments(tokens)
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
        return ParseResponse(items=items, unmatched_queries=unmatched)

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
