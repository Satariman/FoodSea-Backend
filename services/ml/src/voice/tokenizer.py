import re
from dataclasses import dataclass
from enum import StrEnum

from src.voice.numerals import parse_numeral
from src.voice.stopwords import is_stopword, parse_unit


class TokenKind(StrEnum):
    WORD = "word"
    QUANTITY = "quantity"
    UNIT = "unit"
    STOPWORD = "stopword"


@dataclass(frozen=True)
class Token:
    text: str
    kind: TokenKind
    quantity_value: int | float | None = None


_PUNCT_RE = re.compile(r"[.,;:!?\"'\(\)\[\]{}—–\-]+")
_WHITESPACE_RE = re.compile(r"\s+")


def tokenize(text: str) -> list[Token]:
    if not text or not text.strip():
        return []
    cleaned = _PUNCT_RE.sub(" ", text.lower())
    parts = [p for p in _WHITESPACE_RE.split(cleaned) if p]
    tokens: list[Token] = []
    for part in parts:
        numeral = parse_numeral(part)
        if numeral is not None:
            tokens.append(Token(text=part, kind=TokenKind.QUANTITY, quantity_value=numeral))
            continue
        unit = parse_unit(part)
        if unit is not None:
            tokens.append(Token(text=unit, kind=TokenKind.UNIT))
            continue
        if is_stopword(part):
            tokens.append(Token(text=part, kind=TokenKind.STOPWORD))
            continue
        tokens.append(Token(text=part, kind=TokenKind.WORD))
    return tokens
