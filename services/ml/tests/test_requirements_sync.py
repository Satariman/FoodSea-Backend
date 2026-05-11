from __future__ import annotations

from pathlib import Path


def _read_requirement_names() -> set[str]:
    requirements_path = Path(__file__).resolve().parents[1] / "requirements.txt"
    names: set[str] = set()
    for raw in requirements_path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        name = line.split("==", maxsplit=1)[0].strip().lower()
        if name:
            names.add(name)
    return names


def test_runtime_requirements_include_voice_and_embedding_dependencies() -> None:
    names = _read_requirement_names()
    assert "sentence-transformers" in names
    assert "google-genai" in names
    assert "tenacity" in names
