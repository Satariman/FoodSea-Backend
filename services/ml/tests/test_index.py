from __future__ import annotations

import numpy as np

from src.index import AnalogIndex


def build_sample_index() -> AnalogIndex:
    ids = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"]
    names = {pid: f"name-{pid}" for pid in ids}

    vectors = np.array(
        [
            [1.0, 0.0, 0.0],
            [0.97, 0.2, 0.0],
            [0.999, 0.04, 0.0],
            [0.0, 1.0, 0.0],
            [0.0, 0.97, 0.2],
            [0.1, 0.0, 1.0],
            [0.2, 0.1, 0.95],
            [0.05, 0.95, 0.05],
            [0.4, 0.5, 0.1],
            [0.3, 0.2, 0.8],
        ],
        dtype=np.float32,
    )

    offers = {
        "a": {"store1": 1000, "store2": 1050},
        "b": {"store1": 800},
        "c": {"store1": 1400},
        "d": {"store2": 700},
        "e": {"store1": 900, "store2": 890},
        "f": {"store2": 600},
        "g": {"store1": 650},
        "h": {"store1": 780},
        "i": {"store2": 999},
        "j": {"store1": 1100},
    }

    index = AnalogIndex()
    index.build(ids, names, vectors, offers)
    return index


def test_query_excludes_source_product() -> None:
    index = build_sample_index()
    results = index.query("a", top_k=3)

    assert len(results) == 3
    assert all(item[0] != "a" for item in results)


def test_price_aware_promotes_cheaper_item() -> None:
    index = build_sample_index()

    plain = index.query("a", top_k=3, price_aware=False, price_penalty=0.5)
    priced = index.query("a", top_k=3, price_aware=True, price_penalty=0.5)

    plain_ids = [item[0] for item in plain]
    priced_ids = [item[0] for item in priced]

    assert "c" in plain_ids
    assert "b" in priced_ids
    assert priced_ids.index("b") < priced_ids.index("c")


def test_store_filter_keeps_only_available_offers() -> None:
    index = build_sample_index()

    results = index.query("a", top_k=5, filter_store_ids={"store1"})
    assert results
    for product_id, _, _, _ in results:
        assert "store1" in index.product_offers[product_id]


def test_save_and_load_preserves_results(tmp_path) -> None:
    index = build_sample_index()
    path = tmp_path / "index.pkl"

    before = index.query("a", top_k=5, price_aware=True, filter_store_ids={"store1"})

    index.save(str(path))
    restored = AnalogIndex()
    assert restored.load(str(path)) is True

    after = restored.query("a", top_k=5, price_aware=True, filter_store_ids={"store1"})
    assert before == after
