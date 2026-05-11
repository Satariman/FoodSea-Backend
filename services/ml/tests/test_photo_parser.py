from src.photo_search.parser import OCRProductTextParser, ProductTextMeta


def test_parse_extracts_product_name_inside_brand_scope() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="1",
                brand="Простоквашино",
                name="Молоко ультрапастеризованное 3.2%",
            ),
            ProductTextMeta(
                product_id="2",
                brand="Домик в деревне",
                name="Молоко ультрапастеризованное 3.2%",
            ),
        ]
    )

    parsed = parser.parse("АКЦИЯ! Простоквашино молоко ультрапастеризованное 3,2%")

    assert parsed.brand == "Простоквашино"
    assert parsed.product_name == "Молоко ультрапастеризованное 3.2%"
    assert parsed.product_id == "1"


def test_parse_handles_yo_e_and_punctuation_normalization() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="10",
                brand="Ежик",
                name="Йогурт питьевой клубничный",
            )
        ]
    )

    parsed = parser.parse("ёжик!!! йогурт, питьевой: клубничный")

    assert parsed.brand == "Ежик"
    assert parsed.product_name == "Йогурт питьевой клубничный"
    assert parsed.product_id == "10"


def test_parse_fallback_when_brand_is_missing() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="100",
                brand="Brand A",
                name="Сыр плавленый сливочный 200г",
            ),
            ProductTextMeta(
                product_id="200",
                brand="Brand B",
                name="Сыр твердый 45%",
            ),
        ]
    )

    parsed = parser.parse("СЫР ПЛАВЛЕНЫЙ СЛИВОЧНЫЙ 200г")

    assert parsed.brand is None
    assert parsed.product_name == "Сыр плавленый сливочный 200г"
    assert parsed.product_id == "100"
