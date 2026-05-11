from __future__ import annotations

import numpy as np

from src.shared_index.rebuild import main
from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow
from src.shared_index.store import load_shared_index


def test_main_loads_products_builds_and_saves_shared_index(monkeypatch, tmp_path) -> None:
    captured: dict[str, object] = {}

    class FakeLoader:
        def __init__(self, addr: str) -> None:
            captured["loader_addr"] = addr

        def load_products(self):
            return ["p-1", "p-2"]

    class FakeConfig:
        CORE_GRPC_ADDR = "localhost:9091"
        SHARED_INDEX_PATH = str(tmp_path / "shared.pkl")
        PHOTO_SEARCH_BATCH_SIZE = 7
        PHOTO_SEARCH_INDEX_MODE = "flat"

    fake_provider = object()

    def fake_provider_from_config(cfg):
        captured["provider_cfg"] = cfg
        return fake_provider

    def fake_build_weights_from_config(cfg):
        captured["weights_cfg"] = cfg
        return {"name": 1.0, "image": 0.0}

    def fake_build_shared_index(*, products, provider, batch_size, build_weights, index_mode):
        captured["products"] = products
        captured["provider"] = provider
        captured["batch_size"] = batch_size
        captured["build_weights"] = build_weights
        captured["index_mode"] = index_mode
        return (
            SharedIndexProfile(
                provider="fake",
                model="fake-v1",
                dimensions=3,
                index_mode="weighted_multimodal",
                build_weights={"name": 1.0, "image": 0.0},
            ),
            [
                SharedIndexRow(
                    meta=SharedIndexMeta(
                        product_id="p-1",
                        name="Milk",
                        brand_name="Brand",
                        category_name="Dairy",
                        subcategory_name="Milk",
                        image_url="https://example/milk.jpg",
                    ),
                    channels={"name": np.array([1.0, 0.0, 0.0], dtype=np.float32)},
                )
            ],
        )

    monkeypatch.setattr("src.shared_index.rebuild.Config", FakeConfig)
    monkeypatch.setattr("src.shared_index.rebuild.DataLoader", FakeLoader)
    monkeypatch.setattr("src.shared_index.rebuild.provider_from_config", fake_provider_from_config)
    monkeypatch.setattr("src.shared_index.rebuild.build_weights_from_config", fake_build_weights_from_config)
    monkeypatch.setattr("src.shared_index.rebuild.build_shared_index", fake_build_shared_index)

    main()

    assert captured["loader_addr"] == "localhost:9091"
    assert isinstance(captured["provider_cfg"], FakeConfig)
    assert isinstance(captured["weights_cfg"], FakeConfig)
    assert captured["products"] == ["p-1", "p-2"]
    assert captured["provider"] is fake_provider
    assert captured["batch_size"] == 7
    assert captured["build_weights"] == {"name": 1.0, "image": 0.0}
    assert captured["index_mode"] == "flat"

    profile, rows = load_shared_index(tmp_path / "shared.pkl")
    assert profile.provider == "fake"
    assert [row.meta.product_id for row in rows] == ["p-1"]
