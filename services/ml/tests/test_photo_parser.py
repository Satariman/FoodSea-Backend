from src.photo_search.parser import OCRProductTextParser, ProductTextMeta


def test_extracts_brand_and_name_inside_brand_scope() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="1",
                name="Молоко ультрапастеризованное 3.2%",
                brand_name="Простоквашино",
            ),
            ProductTextMeta(
                product_id="2",
                name="Молоко ультрапастеризованное 3.2%",
                brand_name="Домик в деревне",
            ),
        ]
    )

    parsed = parser.parse("АКЦИЯ! Простоквашино молоко ультрапастеризованное 3,2%")

    assert parsed.matched_brand == "Простоквашино"
    assert parsed.matched_name == "Молоко ультрапастеризованное 3.2%"
    assert parsed.name_confidence >= 0.55


def test_handles_yo_equivalence_and_punctuation() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="10",
                name="Йогурт питьевой клубничный",
                brand_name="Ежик",
            )
        ]
    )

    parsed = parser.parse("ёжик!!! йогурт, питьевой: клубничный")

    assert parsed.matched_brand == "Ежик"
    assert parsed.matched_name == "Йогурт питьевой клубничный"
    assert parsed.normalized_ocr == "ежик йогурт питьевой клубничный"


def test_falls_back_when_brand_missing() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="100",
                name="Сыр плавленый сливочный 200г",
                brand_name="Brand A",
            ),
            ProductTextMeta(
                product_id="200",
                name="Сыр твердый 45%",
                brand_name="Brand B",
            ),
        ]
    )

    parsed = parser.parse("сыр сливочный")

    assert parsed.matched_brand is None
    assert parsed.name_confidence < 0.55
    assert parsed.matched_name == "сыр сливочный"
