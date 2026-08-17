from __future__ import annotations

import re
from typing import Any, TypeAlias

KeywordVariant: TypeAlias = str | list[str]
SkillDocument: TypeAlias = dict[str, Any]

SKILL_MD_MAX_LINES = 500
MODULE_MD_MAX_LINES = 500
DESCRIPTION_MIN_CHARS = 80
DESCRIPTION_MAX_CHARS = 1024
INTENT_ROUTER_MIN_ROWS = 2

REQUIRED_HEADINGS = [
    "## Intent Router",
    "## Red Lines",
    "## When to Use",
]

BANNED_FRONTMATTER_KEYS = {
    "enforce_arc_profile",
    "expert_keywords",
}

FIRST_PERSON_RE = re.compile(
    r"(?i)\b(i can|i will|i'm|i am|you can|you should|i'll)\b"
    r"|(?:^|\n)\s*(我可以|你可以|我会)"
)

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

LIFECYCLE_PHASES = {
    "arc:define": "define",
    "arc:clarify": "clarify",
    "arc:arch": "design",
    "arc:sdlc": "plan",
    "arc:build": "implement",
    "arc:frontend": "implement",
    "arc:comment": "implement",
    "arc:idx": "search",
    "arc:test": "verify",
    "arc:security": "secure",
    "arc:audit": "secure",
    "arc:fix": "repair",
    "arc:trace": "operate",
    "arc:docs": "document",
    "arc:prewalk": "route",
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
