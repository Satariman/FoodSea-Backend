__all__ = [
    "build_shared_index",
    "build_weights_from_config",
    "provider_from_config",
    "SharedIndexMeta",
    "SharedIndexProfile",
    "SharedIndexRow",
    "load_shared_index",
    "save_shared_index",
]


def __getattr__(name: str):  # pragma: no cover - thin lazy-export shim
    if name in {"build_shared_index", "build_weights_from_config", "provider_from_config"}:
        from .builder import build_shared_index, build_weights_from_config, provider_from_config

        mapping = {
            "build_shared_index": build_shared_index,
            "build_weights_from_config": build_weights_from_config,
            "provider_from_config": provider_from_config,
        }
        return mapping[name]

    if name in {"SharedIndexMeta", "SharedIndexProfile", "SharedIndexRow"}:
        from .schema import SharedIndexMeta, SharedIndexProfile, SharedIndexRow

        mapping = {
            "SharedIndexMeta": SharedIndexMeta,
            "SharedIndexProfile": SharedIndexProfile,
            "SharedIndexRow": SharedIndexRow,
        }
        return mapping[name]

    if name in {"load_shared_index", "save_shared_index"}:
        from .store import load_shared_index, save_shared_index

        mapping = {
            "load_shared_index": load_shared_index,
            "save_shared_index": save_shared_index,
        }
        return mapping[name]

    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
