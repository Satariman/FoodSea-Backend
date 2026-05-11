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
    assert parsed.extracted_percentages == (3.2,)
    assert parsed.extracted_volume is None


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
    assert parsed.extracted_percentages == ()
    assert parsed.extracted_volume is None


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


def test_extracts_product_name_percentages_and_volume() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="1",
                name="Сметана 20% 300г",
                brand_name="Эконива",
            ),
            ProductTextMeta(
                product_id="2",
                name="Молоко 3.2% 930мл",
                brand_name="Эконива",
            ),
        ]
    )

    parsed = parser.parse("ЭКОНИВА СМЕТАНА 20% 300г")

    assert parsed.matched_brand == "Эконива"
    assert parsed.extracted_product_name == "сметана"
    assert parsed.extracted_percentages == (20.0,)
    assert parsed.extracted_volume == "300 г"


def test_extracts_name_from_noisy_multiline_ocr() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="1",
                name="Сметана 20% 180г",
                brand_name="Эконива",
            ),
            ProductTextMeta(
                product_id="2",
                name="Молоко 3.2% 930мл",
                brand_name="Эконива",
            ),
        ]
    )

    parsed = parser.parse(
        "EKONIVA®\nЭКОНИВА\nМОЛОКО, КОТОРЫМ МЫ ГОРДИМСЯ\n20%\nСМЕТАНА"
    )

    assert parsed.matched_brand == "Эконива"
    assert parsed.extracted_product_name == "сметана"
    assert parsed.extracted_percentages == (20.0,)
    assert parsed.extracted_volume is None


def test_weak_brand_overlap_does_not_force_brand_scope() -> None:
    parser = OCRProductTextParser(
        products=[
            ProductTextMeta(
                product_id="1",
                name="Молоко ультрапастеризованное 3.2%",
                brand_name="Домик в деревне",
            ),
            ProductTextMeta(
                product_id="2",
                name="Молоко ультрапастеризованное 3.2%",
                brand_name="Простоквашино",
            ),
        ]
    )

    parsed = parser.parse("молоко в пакете 3,2%")

    assert parsed.matched_brand is None


def test_tie_break_is_deterministic_for_equal_product_scores() -> None:
    products = [
        ProductTextMeta(product_id="2", name="Сок яб 1л", brand_name="Бренд"),
        ProductTextMeta(product_id="1", name="Сок апе 1л", brand_name="Бренд"),
    ]

    parser_a = OCRProductTextParser(products=products)
    parser_b = OCRProductTextParser(products=list(reversed(products)))

    parsed_a = parser_a.parse("бренд сок 1л")
    parsed_b = parser_b.parse("бренд сок 1л")

    assert parsed_a.name_confidence == parsed_b.name_confidence
    assert parsed_a.matched_brand == parsed_b.matched_brand
    assert parsed_a.matched_name == parsed_b.matched_name
    assert parsed_a.matched_name == "Сок апе 1л"
