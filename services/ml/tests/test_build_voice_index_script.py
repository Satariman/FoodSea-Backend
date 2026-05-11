from __future__ import annotations

import asyncio
import importlib
import sys
import types


def _install_tenacity_stub() -> None:
    tenacity_mod = types.ModuleType("tenacity")

    def retry(*args, **kwargs):
        def _decorator(fn):
            return fn

        return _decorator

    def retry_if_exception_type(*args, **kwargs):
        return object()

    def stop_after_attempt(*args, **kwargs):
        return object()

    def wait_exponential(*args, **kwargs):
        return object()

    tenacity_mod.retry = retry
    tenacity_mod.retry_if_exception_type = retry_if_exception_type
    tenacity_mod.stop_after_attempt = stop_after_attempt
    tenacity_mod.wait_exponential = wait_exponential
    sys.modules.setdefault("tenacity", tenacity_mod)


def _import_script_with_stubbed_dependencies():
    # Keep existing google namespace package used by protobuf and inject only google.genai
    import google  # noqa: F401

    genai_mod = types.ModuleType("google.genai")

    class _DummyClient:
        def __init__(self, *args, **kwargs) -> None:
            pass

    genai_mod.Client = _DummyClient

    types_mod = types.ModuleType("google.genai.types")

    class _Part:
        @staticmethod
        def from_text(*args, **kwargs):
            return None

        @staticmethod
        def from_bytes(*args, **kwargs):
            return None

    class _Content:
        def __init__(self, *args, **kwargs) -> None:
            pass

    class _EmbedContentConfig:
        def __init__(self, *args, **kwargs) -> None:
            pass

    types_mod.Part = _Part
    types_mod.Content = _Content
    types_mod.EmbedContentConfig = _EmbedContentConfig

    genai_mod.types = types_mod

    sys.modules.setdefault("google.genai", genai_mod)
    sys.modules.setdefault("google.genai.types", types_mod)

    _install_tenacity_stub()

    return importlib.import_module("scripts.build_voice_index")


script = _import_script_with_stubbed_dependencies()


class _Row:
    def __init__(self) -> None:
        self.product_id = "p-1"
        self.name = "Milk"
        self.brand_name = "Prostokvashino"
        self.category_name = "Dairy"
        self.image_url = "https://cdn.example/milk.jpg"
        self.brand_id = "brand-uuid-should-not-be-used"
        self.category_id = "category-uuid-should-not-be-used"


class _DummyCfg:
    CORE_GRPC_ADDR = "core-service:9091"
    GEMINI_API_KEY = "test-key"
    GEMINI_MODEL = "gemini-embedding-2"
    GEMINI_OUTPUT_DIM = 768
    VOICE_INDEX_PATH = "/tmp/voice_index.pkl"


def test_main_builds_product_view_from_human_readable_fields(monkeypatch) -> None:
    captured = {}

    class _Loader:
        def __init__(self, _addr: str) -> None:
            pass

        def load_products(self):
            return [_Row()]

    class _Gemini:
        def __init__(self, **_kwargs) -> None:
            pass

    class _Fetcher:
        def __init__(self, **_kwargs) -> None:
            pass

    class _Index:
        product_ids = ["p-1"]

        def save(self, _path):
            return None

    async def _fake_build_voice_index(products, _gemini, _fetcher):
        captured["products"] = products
        return _Index()

    monkeypatch.setattr(script, "Config", _DummyCfg)
    monkeypatch.setattr(script, "DataLoader", _Loader)
    monkeypatch.setattr(script, "GeminiClient", _Gemini)
    monkeypatch.setattr(script, "HTTPImageFetcher", _Fetcher)
    monkeypatch.setattr(script, "build_voice_index", _fake_build_voice_index)

    asyncio.run(script.main())

    assert len(captured["products"]) == 1
    p = captured["products"][0]
    assert p.brand == "Prostokvashino"
    assert p.category == "Dairy"
    assert p.image_url == "https://cdn.example/milk.jpg"
