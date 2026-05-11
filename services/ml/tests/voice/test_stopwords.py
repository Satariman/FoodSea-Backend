from src.voice.stopwords import is_stopword, parse_unit


def test_is_stopword_recognises_filler_words():
    for w in ["и", "а", "ещё", "также", "плюс", "пожалуйста", "ну", "вот"]:
        assert is_stopword(w), f"{w!r} should be a stopword"


def test_is_stopword_returns_false_for_content_words():
    for w in ["молоко", "хлеб", "два", "литр"]:
        assert not is_stopword(w), f"{w!r} should not be a stopword"


def test_parse_unit_normalises_synonyms():
    assert parse_unit("кг") == "кг"
    assert parse_unit("килограмм") == "кг"
    assert parse_unit("грамм") == "г"
    assert parse_unit("г") == "г"
    assert parse_unit("литр") == "л"
    assert parse_unit("литра") == "л"
    assert parse_unit("литров") == "л"
    assert parse_unit("л") == "л"
    assert parse_unit("мл") == "мл"
    assert parse_unit("шт") == "шт"
    assert parse_unit("штук") == "шт"
    assert parse_unit("упаковка") == "упаковка"
    assert parse_unit("пачка") == "пачка"


def test_parse_unit_returns_none_for_non_units():
    assert parse_unit("молоко") is None
    assert parse_unit("") is None
