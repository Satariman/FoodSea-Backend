from dataclasses import dataclass, field

from src.voice.tokenizer import Token, TokenKind


@dataclass(frozen=True)
class NGram:
    text: str
    start: int
    end: int

    @property
    def length(self) -> int:
        return self.end - self.start


@dataclass(frozen=True)
class Segment:
    quantity: int | float
    unit: str | None
    words: list[str] = field(default_factory=list)


_CONJUNCTION_STOPWORDS: frozenset[str] = frozenset({"и", "а", "плюс"})


def build_segments(tokens: list[Token]) -> list[Segment]:
    segments: list[Segment] = []
    current_quantity: int | float = 1
    current_unit: str | None = None
    current_words: list[str] = []

    def flush() -> None:
        if current_words:
            quantity = current_quantity
            if isinstance(quantity, float) and quantity.is_integer():
                quantity = int(quantity)
            segments.append(Segment(
                quantity=quantity,
                unit=current_unit,
                words=list(current_words),
            ))

    def reset(*, quantity: int | float = 1) -> None:
        nonlocal current_quantity, current_unit, current_words
        current_quantity = quantity
        current_unit = None
        current_words = []

    for token in tokens:
        if token.kind == TokenKind.QUANTITY:
            flush()
            reset(quantity=token.quantity_value if token.quantity_value is not None else 1)
        elif token.kind == TokenKind.UNIT:
            if current_words:
                # Units that appear after already collected words usually start a new
                # product phrase (e.g. "... тунец упаковка яиц ...").
                flush()
                reset()
            current_unit = token.text
        elif token.kind == TokenKind.WORD:
            current_words.append(token.text)
        elif token.kind == TokenKind.STOPWORD:
            if token.text in _CONJUNCTION_STOPWORDS and (
                current_words or current_unit is not None or current_quantity != 1
            ):
                flush()
                reset()
            continue
    flush()

    return segments


def generate_ngrams(segment: Segment, max_n: int = 3) -> list[NGram]:
    words = segment.words
    if not words:
        return []
    n_max = min(max_n, len(words))
    ngrams: list[NGram] = []
    for n in range(1, n_max + 1):
        for i in range(0, len(words) - n + 1):
            text = " ".join(words[i:i + n])
            ngrams.append(NGram(text=text, start=i, end=i + n))
    return ngrams
