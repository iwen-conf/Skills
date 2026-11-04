from __future__ import annotations

from pathlib import Path
from typing import Any

from arc_policy.paths import repo_root

LANGUAGE_ORDER = ("go", "rust", "android", "harmonyos", "python", "typescript")
ARC_SKILLS = frozenset(
    {"arc-explore", "arc-plan", "arc-code", "arc-verify", "arc-audit", "arc-vet"}
)


def normalize_skill_name(name: str) -> str:
    """Use one identity for the six Arc source and packaged skill names."""
    candidate = name.replace("arc:", "arc-", 1) if name.startswith("arc:") else name
    return candidate if candidate in ARC_SKILLS else name


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
            return normalize_skill_name(skill)
    return "arc-explore"


def resolve_skill_name(name: str, policy: dict[str, Any]) -> str:
    name = normalize_skill_name(name)
    aliases = policy.get("skill_aliases") or {}
    if name in ARC_SKILLS or name in aliases:
        return name
    for canonical, names in aliases.items():
        if name == normalize_skill_name(canonical) or name in (names or []):
            return normalize_skill_name(canonical)
    return name
