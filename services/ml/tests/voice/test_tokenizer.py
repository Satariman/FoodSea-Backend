from src.voice.tokenizer import tokenize, Token, TokenKind


def _kinds(tokens: list[Token]) -> list[tuple[str, str]]:
    return [(t.text, t.kind) for t in tokens]


def test_tokenize_lowercases_and_splits():
    result = tokenize("Молоко Простоквашино")
    assert _kinds(result) == [
        ("молоко", TokenKind.WORD),
        ("простоквашино", TokenKind.WORD),
    ]


def test_tokenize_strips_punctuation():
    result = tokenize("молоко, хлеб; яблоки!")
    assert _kinds(result) == [
        ("молоко", TokenKind.WORD),
        ("хлеб", TokenKind.WORD),
        ("яблоки", TokenKind.WORD),
    ]


def test_tokenize_classifies_numerals():
    result = tokenize("два молока")
    assert result[0].kind == TokenKind.QUANTITY
    assert result[0].quantity_value == 2
    assert result[1].kind == TokenKind.WORD


def test_tokenize_classifies_digits_as_quantity():
    result = tokenize("5 огурцов")
    assert result[0].kind == TokenKind.QUANTITY
    assert result[0].quantity_value == 5


def test_tokenize_classifies_units():
    result = tokenize("молоко литр")
    assert result[0].kind == TokenKind.WORD
    assert result[1].kind == TokenKind.UNIT


def test_tokenize_classifies_stopwords():
    result = tokenize("молоко и хлеб")
    assert _kinds(result) == [
        ("молоко", TokenKind.WORD),
        ("и", TokenKind.STOPWORD),
        ("хлеб", TokenKind.WORD),
    ]


def test_tokenize_empty_returns_empty_list():
    assert tokenize("") == []
    assert tokenize("   ") == []


def test_tokenize_collapses_whitespace():
    result = tokenize("  молоко   хлеб  ")
    assert _kinds(result) == [("молоко", TokenKind.WORD), ("хлеб", TokenKind.WORD)]
