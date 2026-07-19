from __future__ import annotations
import re
from typing import Any, TypeAlias

KeywordVariant: TypeAlias = str | list[str]
SkillDocument: TypeAlias = dict[str, Any]

REQUIRED_HEADINGS = [
    "## Overview",
    "## When to Use",
]

ARC_REQUIRED_HEADINGS = [
    "## Quick Contract",
    "## Announce",
    "## The Iron Law",
    "## Workflow",
    "## Quality Gates",
    "## Expert Standards",
    "## Scripts & Commands",
    "## Red Flags",
]

ARC_WHEN_TO_USE_MARKERS_EN = [
    ["**Preferred Trigger**", "**Preferred trigger**", "**Primary Trigger**", "**Primary trigger**"],
    ["**Typical scenario**", "**Typical Scenario**", "**Typical scenarios**", "**Typical Scenarios**"],
    ["**Boundary Tip**", "**Boundary Tips**", "**Boundary Note**", "**Border Tip**"],
]

ARC_ROUTING_MATRIX_LINK = "../../docs/arc-routing-matrix.md"

SUPPORTED_SKILL_PREFIXES = ("arc:", "lark-", "wxskills:")
SKILL_NAMESPACE_DIRS = {
    "arc": "Arc",
}

LEGACY_TOKEN_PARTS = [
    ("Ta", "sk("),
    ("subagent", "_type"),
    ("load", "_skills"),
    ("run", "_in_background"),
]

BANNED_TOKENS = [
    *("".join(parts) for parts in LEGACY_TOKEN_PARTS),
    "session_id",
    "arc-estimate",
    "arc-retest",
]

WHEN_TO_USE_MARKER_VARIANTS = ARC_WHEN_TO_USE_MARKERS_EN

QUICK_CONTRACT_KEY_MAP = {
    "trigger": "trigger",
    "inputs": "inputs",
    "outputs": "outputs",
    "quality gate": "quality_gate",
    "decision tree": "decision_tree",
}

INPUT_ARGUMENT_HEADER_MAP = {
    "parameter": "parameter",
    "type": "type",
    "required": "required",
    "illustrate": "description",
    "description": "description",
}

def is_arc_skill(name: str) -> bool:
    return name.startswith("arc:")

def is_supported_skill(name: str) -> bool:
    return any(name.startswith(prefix) for prefix in SUPPORTED_SKILL_PREFIXES)

def get_namespace_dir(name: str) -> str | None:
    namespace, _, _ = name.partition(":")
    return SKILL_NAMESPACE_DIRS.get(namespace)

def contains_cjk(text: str) -> bool:
    return re.search(r"[\u4e00-\u9fff]", text) is not None
