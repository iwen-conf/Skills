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

# Typical-task load ceilings: everything an Arc task reads for one scenario, summed with
# estimate_tokens. Per-file budgets alone let shared documents grow until a task loads as much
# as the pre-2.0 skill mesh; these ceilings make that regression fail validation.
ARC_LOAD_SCENARIOS: dict[str, tuple[int, tuple[str, ...]]] = {
    "go-repair": (15500, (
        "Arc/arc:code/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/_shared/coding-discipline.md", "docs/execution-truth.md", "Arc/arc:code/references/repair.md",
        "Arc/_shared/conventions/go-backend.md", "Arc/_shared/conventions/go-type-safety.md",
    )),
    "go-feature": (12000, (
        "Arc/arc:code/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/_shared/coding-discipline.md", "Arc/_shared/conventions/go-backend.md",
        "Arc/_shared/conventions/go-type-safety.md",
    )),
    "fullstack-api": (18000, (
        "Arc/arc:code/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/_shared/coding-discipline.md", "Arc/_shared/conventions/go-backend.md",
        "Arc/_shared/conventions/go-type-safety.md", "Arc/_shared/conventions/frontend.md",
        "Arc/_shared/api-contract.md", "Arc/_shared/http-api-design.md",
    )),
    "pr-review": (9500, (
        "Arc/arc:audit/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/arc:audit/references/diff-review.md",
    )),
    "pr-re-review": (10500, (
        "Arc/arc:audit/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/arc:audit/references/diff-review.md", "Arc/arc:audit/references/review-loop.md",
    )),
    "appsec-campaign": (19000, (
        "Arc/arc:audit/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/arc:audit/references/appsec.md", "Arc/arc:audit/references/attack-classes.md",
        "Arc/arc:audit/references/scanners.md", "Arc/_shared/finding-campaign.md",
    )),
    "pg-reliability": (15500, (
        "Arc/arc:verify/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/arc:verify/references/txn-faults.md", "Arc/arc:verify/references/case-design.md",
        "Arc/arc:verify/references/db.md",
    )),
    "verify-flow": (14000, (
        "Arc/arc:verify/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/arc:verify/references/flow-model.md", "Arc/arc:verify/references/flow-matrix.md",
        "Arc/arc:verify/references/case-design.md",
    )),
    "plan-model": (13000, (
        "Arc/arc:plan/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "docs/execution-truth.md", "Arc/arc:plan/references/closure-design.md",
        "Arc/arc:plan/references/closure-spec-format.md",
    )),
    "plan-tasks": (15500, (
        "Arc/arc:plan/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "docs/execution-truth.md", "Arc/arc:plan/references/task-docs.md",
        "Arc/arc:plan/references/task-templates.md",
    )),
    "vet-select": (9000, (
        "Arc/arc:vet/SKILL.md", "Arc/_shared/contract.md", "Arc/_shared/code-intel.md",
        "Arc/arc:vet/references/search-playbook.md", "Arc/arc:vet/references/decision-template.md",
    )),
}

# These entrypoints explicitly advertise diagnosis, repair, or failure-driven
# optimization. They must load the repository-wide evidence-first repair gate
# instead of re-inventing a weaker local version of it.
ROOT_CAUSE_GATED_SKILLS = frozenset(
    {
        "android-intent-security",
        "cnb",
        "arc:audit",
        "arc:code",
        "arc:verify",
        "edge-to-edge",
        "engage-sdk-integration",
        "harmonyos",
        "perfetto-trace-analysis",
        "r8-analyzer",
    }
)
ROOT_CAUSE_GATE_REFERENCE = "execution-truth.md"
ROOT_CAUSE_GATE_HEADING = "root-cause repair"

# Trigger words live only in the description: the body is read after selection, so a
# body Trigger section cannot affect routing.
ARC_REQUIRED_HEADINGS = [
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
    "arc:closure",
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
VERTICAL_SKILL_ROOTS = ("Android", "HarmonyOS", "CNB", "Lark", "Waza", "Lazycat", "Design", "Cloudflare", "ThreeJS")
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
    "arc:vet": "select",
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
