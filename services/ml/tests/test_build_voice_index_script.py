from __future__ import annotations

import importlib
from pathlib import Path

import numpy as np

from src.shared_index.schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow

script = importlib.import_module("scripts.build_voice_index")


def _profile() -> SharedIndexProfile:
    return SharedIndexProfile(
        provider="gemini_api_key",
        model="gemini-embedding-2",
        dimensions=2,
        index_mode="weighted_multimodal",
        build_weights={"name": 1.0},
    )


def _rows() -> list[SharedIndexRow]:
    return [
        SharedIndexRow(
            meta=SharedIndexMeta(
                product_id="p-1",
                name="Milk",
                brand_name="Brand",
                category_name="Dairy",
                subcategory_name="",
                image_url="",
            ),
            channels={"name": np.array([1.0, 0.0], dtype=np.float32)},
        )
    ]


def test_main_loads_shared_index_and_saves_voice_index(monkeypatch, tmp_path: Path) -> None:
    captured: dict[str, object] = {}
    shared_path = tmp_path / "shared.pkl"
    voice_path = tmp_path / "voice.pkl"
    profile = _profile()
    rows = _rows()

    class _DummyCfg:
        SHARED_INDEX_PATH = str(shared_path)
        VOICE_INDEX_PATH = str(voice_path)

    class _Index:
        product_ids = ["p-1"]

        def save(self, path):
            captured["save_path"] = path

    def _fake_load_shared_index(path):
        captured["shared_path"] = path
        return profile, rows

    def _fake_build_voice_index(profile, rows):
        captured["profile"] = profile
        captured["rows"] = rows
        return _Index()

    monkeypatch.setattr(script, "Config", _DummyCfg)
    monkeypatch.setattr(script, "load_shared_index", _fake_load_shared_index)
    monkeypatch.setattr(script, "build_voice_index", _fake_build_voice_index)

    script.main()

    assert captured["shared_path"] == str(shared_path)
    assert captured["profile"] is profile
    assert captured["rows"] is rows
    assert captured["save_path"] == voice_path


def test_script_does_not_depend_on_catalog_loader_or_gemini_client() -> None:
    assert not hasattr(script, "DataLoader")
    assert not hasattr(script, "GeminiClient")
    assert not hasattr(script, "HTTPImageFetcher")
