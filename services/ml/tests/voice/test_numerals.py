import pytest
from src.voice.numerals import parse_numeral


@pytest.mark.parametrize("token,expected", [
    ("0", 0), ("1", 1), ("12", 12), ("100", 100),
    ("ноль", 0),
    ("один", 1), ("одну", 1), ("одно", 1), ("одна", 1),
    ("два", 2), ("две", 2), ("двое", 2), ("пара", 2),
    ("три", 3), ("трое", 3),
    ("пять", 5), ("дюжина", 12),
    ("пол", 0.5), ("половина", 0.5),
    ("полтора", 1.5), ("полторы", 1.5),
])
def test_recognises_numerals(token, expected):
    assert parse_numeral(token) == expected


@pytest.mark.parametrize("token", ["", "молоко", "литра", "abc", "12.5", "-1"])
def test_returns_none_for_non_numerals(token):
    assert parse_numeral(token) is None
