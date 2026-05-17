from __future__ import annotations

from pathlib import Path


def _read_requirement_names(filename: str) -> set[str]:
    requirements_path = Path(__file__).resolve().parents[1] / filename
    names: set[str] = set()
    for raw in requirements_path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or line.startswith("-r "):
            continue
        name = line.split("==", maxsplit=1)[0].strip().lower()
        if name:
            names.add(name)
    return names


def test_runtime_requirements_are_lightweight_and_keep_voice_deps() -> None:
    names = _read_requirement_names("requirements-runtime.txt")
    assert "google-genai" in names
    assert "tenacity" in names
    assert "sentence-transformers" not in names
    assert "grpcio-tools" not in names
    assert "pytest" not in names


def test_builder_requirements_include_index_building_deps() -> None:
    names = _read_requirement_names("requirements-builder.txt")
    assert "sentence-transformers" in names
    assert "grpcio-tools" in names
    assert "pytest" in names
