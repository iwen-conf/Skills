from __future__ import annotations

from pathlib import Path
from typing import Any

from arc_policy.paths import repo_root

LANGUAGE_ORDER = ("go", "rust", "android", "harmonyos", "python", "typescript")


def detect_languages(cwd: Path | None = None, policy: dict[str, Any] | None = None) -> list[str]:
    root = repo_root(cwd)
    if policy is None:
        from arc_policy.policy import load_named

        policy = load_named("default")
    languages = policy.get("languages") or {}
    found: list[str] = []
    for name in LANGUAGE_ORDER:
        spec = languages.get(name) or {}
        markers = spec.get("markers") or []
        if any((root / marker).exists() for marker in markers):
            found.append(name)
    for name, spec in languages.items():
        if name in found:
            continue
        markers = spec.get("markers") or []
        if any((root / marker).exists() for marker in markers):
            found.append(name)
    return found


def recommended_skill(languages: list[str], policy: dict[str, Any]) -> str:
    specs = policy.get("languages") or {}
    for language in languages:
        skill = (specs.get(language) or {}).get("skill")
        if isinstance(skill, str) and skill:
            return skill
    return "arc-explore"


def resolve_skill_name(name: str, policy: dict[str, Any]) -> str:
    aliases = policy.get("skill_aliases") or {}
    if name in aliases:
        return name
    for canonical, names in aliases.items():
        if name == canonical or name in (names or []):
            return canonical
    return name
