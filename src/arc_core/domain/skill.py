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

# Estimated tokens = non-CJK characters / 4 + CJK characters.
ARC_SKILL_MAX_TOKENS = 3500
ARC_REFERENCE_MAX_TOKENS = 3500
# Bundled tool packages (for example scripts/dbqual) keep their own documentation
# and are held to the line budget only.
ARC_TOKEN_EXEMPT_DIRS = ("scripts",)
ARC_SHARED_DIR = "_shared"

# These entrypoints explicitly advertise diagnosis, repair, or failure-driven
# optimization. They must load the repository-wide evidence-first repair gate
# instead of re-inventing a weaker local version of it.
ROOT_CAUSE_GATED_SKILLS = frozenset(
    {
        "android-intent-security",
        "arkts-syntax-assistant",
        "arc:audit",
        "arc:code",
        "arc:verify",
        "cnb-code-commit",
        "cnb-code-review",
        "cnb-pipeline",
        "edge-to-edge",
        "engage-sdk-integration",
        "harmonyos-development",
        "perfetto-trace-analysis",
        "r8-analyzer",
    }
)
ROOT_CAUSE_GATE_REFERENCE = "execution-truth.md"
ROOT_CAUSE_GATE_HEADING = "root-cause repair"

ARC_REQUIRED_HEADINGS = [
    "Trigger",
    "Non-trigger",
    "Goal",
    "Inputs",
    "Intent Router",
    "Workflow",
    "Tool Strategy",
    "Decision Rules",
    "Verification",
    "Failure Recovery",
    "Completion Criteria",
]

RETIRED_ARC_SKILLS = (
    "arc:arch",
    "arc:build",
    "arc:clarify",
    "arc:comment",
    "arc:dbqual",
    "arc:define",
    "arc:docs",
    "arc:fix",
    "arc:frontend",
    "arc:idx",
    "arc:prewalk",
    "arc:reliability",
    "arc:sdlc",
    "arc:security",
    "arc:test",
    "arc:trace",
)
RETIRED_ARC_SKILL_RE = re.compile(
    r"(?<![\w-])(?:" + "|".join(re.escape(name) for name in RETIRED_ARC_SKILLS) + r")(?![\w-])"
)

VAGUE_PHRASES = (
    "as needed",
    "as appropriate",
    "when relevant",
    "按需",
    "适当",
    "认真",
    "仔细",
    "酌情",
    "视情况",
)

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
VERTICAL_SKILL_ROOTS = ("Android", "HarmonyOS", "CNB", "Lark")
# Validated source distributions; deliberately excluded from default installation.
SOURCE_SKILL_PATHS: tuple[str, ...] = ()

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
    "arc:explore": "understand",
    "arc:plan": "plan",
    "arc:code": "change",
    "arc:verify": "prove",
    "arc:audit": "evaluate",
}


def is_arc_skill(name: str) -> bool:
    return name.startswith("arc:")


def estimate_tokens(text: str) -> int:
    cjk = len(re.findall(r"[\u3000-\u9fff\uff00-\uffef]", text))
    return round((len(text) - cjk) / 4 + cjk)


def is_supported_skill(name: str) -> bool:
    return any(name.startswith(prefix) for prefix in SUPPORTED_SKILL_PREFIXES)


def get_namespace_dir(name: str) -> str | None:
    namespace, _, _ = name.partition(":")
    return SKILL_NAMESPACE_DIRS.get(namespace)


def contains_cjk(text: str) -> bool:
    return re.search(r"[\u4e00-\u9fff]", text) is not None
