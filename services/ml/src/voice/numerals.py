import re

_NUMERALS_RU: dict[str, int | float] = {
    "ноль": 0,
    "один": 1, "одну": 1, "одно": 1, "одна": 1,
    "два": 2, "две": 2, "двое": 2, "пара": 2,
    "три": 3, "трое": 3,
    "четыре": 4, "четверо": 4,
    "пять": 5, "пятёрка": 5,
    "шесть": 6, "семь": 7, "восемь": 8, "девять": 9, "десять": 10,
    "одиннадцать": 11, "двенадцать": 12, "дюжина": 12,
    "пол": 0.5, "половина": 0.5,
    "полтора": 1.5, "полторы": 1.5,
}

_DIGIT_RE = re.compile(r"^\d+$")


def parse_numeral(token: str) -> int | float | None:
    if not token:
        return None
    if _DIGIT_RE.match(token):
        return int(token)
    return _NUMERALS_RU.get(token)
