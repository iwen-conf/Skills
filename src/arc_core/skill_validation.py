from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any, TypeAlias

from jsonschema import ValidationError, validate

KeywordVariant: TypeAlias = str | list[str]
SkillDocument: TypeAlias = dict[str, Any]

REQUIRED_HEADINGS = [
    "## Overview",
    "## When to Use",
]

GENERIC_REQUIRED_HEADINGS = [
    "## Quick Contract",
    "## Announce",
    "## Input Arguments",
    "## The Iron Law",
    "## Workflow",
    "## Quality Gates",
    "## Red Flags",
    "## Outputs",
]

ARC_FUSION_REQUIRED_HEADINGS = [
    "## Quick Contract",
    "## Announce",
    "## The Iron Law",
    "## Workflow",
    "## Quality Gates",
    "## Expert Standards",
    "## Scripts & Commands",
    "## Red Flags",
]

ARC_WHEN_TO_USE_MARKERS_ZH = [
    "**首选触发**",
    "**典型场景**",
    "**边界提示**",
]

ARC_WHEN_TO_USE_MARKERS_EN = [
    ["**Preferred Trigger**", "**Preferred trigger**", "**Primary Trigger**", "**Primary trigger**"],
    ["**Typical scenario**", "**Typical Scenario**", "**Typical scenarios**", "**Typical Scenarios**"],
    ["**Boundary Tip**", "**Boundary Tips**", "**Boundary Note**", "**Border Tip**"],
]

GENERIC_WHEN_TO_USE_MARKERS = [
    "**首选触发**",
    "**典型场景**",
    "**边界提示**",
]

ARC_ROUTING_MATRIX_LINK = "../../docs/arc-routing-matrix.md"
ARC_DECISION_TREE_LINK = "../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree"
ARC_PHASE_VIEW_LINK = "../../docs/arc-routing-matrix.md#phase-routing-view"
ARC_CHEATSHEET_LINK = "../../docs/arc-routing-cheatsheet.md"

SUPPORTED_SKILL_PREFIXES = ("arc:",)
SKILL_NAMESPACE_DIRS = {
    "arc": "Arc",
}

ARC_EXPERT_KEYWORDS: dict[str, list[KeywordVariant]] = {
    "arc:build": ["DoD", "SemVer", "Contract Test", "RTO/RPO", "SBOM"],
    "arc:cartography": ["C4", "ISO/IEC 42010", "churn", ["增量差异清单", "incremental diff"]],
    "arc:clarify": ["IEEE 29148", "INVEST", "Given-When-Then"],
    "arc:context": [
        ["tool-backed context", "工具驱动上下文"],
        ["working set", "工作集"],
        ["recovery manifest", "恢复清单"],
        ["lazy restore", "按需恢复"],
        ["token budget", "上下文预算", "context budget"],
        "FTS5",
        "BM25",
        ["compaction", "压缩恢复"],
        ["sandbox", "沙箱"],
    ],
    "arc:decide": ["ADR", "Pre-Mortem", "Fitness Function"],
    "arc:e2e": ["ISTQB", "OWASP ASVS", "WCAG 2.2 AA"],
    "arc:exec": ["RACI", ["关键路径(CPM)", "Critical Path", "CPM"], ["冲突仲裁规则", "conflict arbitration", "Conflict Arbitration"]],
    "arc:fix": [["SEV", "Severity Level", "severity level", "severity"], "5 Whys", "Fault Tree", "Blameless Postmortem", "Mandatory Hypothesis", "Rationalization Watch"],
    "arc:gate": ["Policy-as-Code", "OWASP", "SBOM", "OPA/Rego"],
    "arc:init": ["schema_version", ["原子更新", "atomic update", "Atomic Update"], ["上一个稳定索引回滚", "stable index rollback", "rollback"]],
    "arc:serve": ["tmux", "single-instance", ["JSON registry", "registry"], ["port", "ports"], ["graceful shutdown", "graceful"]],
    "arc:ip-check": [["新颖性", "novelty", "Novelty"], ["创造性", "inventiveness", "Inventiveness", "creativity", "creativeness"], ["实用性", "utility", "Utility", "practicality", "practicability"], "FTO"],
    "arc:ip-draft": [["宽-中-窄", "broad-medium-narrow", "Broad-Medium-Narrow", "wide-medium-narrow"], ["权利要求", "claims", "Claims"], ["待法务复核清单", "legal review checklist", "Legal Review", "to be reviewed by legal"]],
    "arc:microcopy": [
        ["plain language", "人话", "通俗表达"],
        ["user mental model", "用户心智模型"],
        ["actionable guidance", "可执行指引"],
        ["avoid jargon", "避免术语", "避免技术术语"],
        ["blame-free", "非责怪式", "责怪式"],
    ],
    "arc:uml": ["UML 2.5.1 / ISO 19505", "Chen", "drawio", ["建模假设", "modeling assumption", "Modeling Assumption"]],
    "arc:audit": [["业务成熟度", "Business Maturity", "business maturity"], ["依赖健康度", "Dependency Health", "dependency health"], ["专家评审卡", "Expert Review Card", "expert review card"], "9 Tab"],
    "arc:test": [
        "ISTQB",
        ["boundary value analysis", "边界值分析"],
        ["equivalence partitioning", "等价类划分"],
        ["code coverage", "代码覆盖率"],
        ["test pyramid", "测试金字塔"],
    ],
    "arc:aigc": [
        ["chunked rewrite", "chunked polish", "分段重写"],
        ["citation fidelity", "引用保真"],
        ["two-stage polish", "two-stage rewrite", "两阶段润色", "双阶段润色"],
        ["semantic drift", "语义漂移"],
        ["authorial voice", "作者声音"],
        ["human review", "人工复核"],
    ],
    "arc:learn": [
        ["three-layer verification", "三层交叉验证"],
        ["intellectual genealogy", "知识谱系"],
        ["primary sources", "第一手信源"],
        ["contextual fidelity", "语境保真度"],
    ],
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
    "arc:init:full",
    "arc:init:update",
]

FUSION_GENERIC_SKILLS: set[str] = {
    "terminal-table-output",
}

ARC_ROUTED_SKILLS = {
    "arc:exec",
    "arc:cartography",
    "arc:decide",
    "arc:gate",
    "arc:build",
    "arc:context",
    "arc:init",
    "arc:ip-check",
    "arc:ip-draft",
    "arc:clarify",
    "arc:audit",
    "arc:e2e",
    "arc:fix",
    "arc:uml",
    "arc:test",
    "arc:serve",
    "arc:aigc",
    "arc:microcopy",
}

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

WHEN_TO_USE_MARKER_VARIANTS = [
    ["**首选触发**", "**Preferred Trigger**", "**Preferred trigger**", "**Primary Trigger**", "**Primary trigger**"],
    ["**典型场景**", "**Typical scenario**", "**Typical Scenario**", "**Typical scenarios**", "**Typical Scenarios**"],
    ["**边界提示**", "**Boundary Tip**", "**Boundary Tips**", "**Boundary Note**", "**Border Tip**"],
]



import yaml

def parse_frontmatter(text: str) -> tuple[dict[str, Any], str | None]:
    if not text.startswith("---\n"):
        return {}, "missing frontmatter start"
    end = text.find("\n---\n", 4)
    if end == -1:
        return {}, "missing frontmatter end"
    block = text[4:end]
    try:
        data = yaml.safe_load(block) or {}
    except yaml.YAMLError as exc:
        return {}, f"yaml parsing error: {exc}"
    return data, None



def is_arc_skill(name: str) -> bool:
    return name.startswith("arc:")
def is_supported_skill(name: str) -> bool:
    return any(name.startswith(prefix) for prefix in SUPPORTED_SKILL_PREFIXES)


def get_namespace_dir(name: str) -> str | None:
    namespace, _, _ = name.partition("-")
    return SKILL_NAMESPACE_DIRS.get(namespace)



def contains_cjk(text: str) -> bool:
    return re.search(r"[\u4e00-\u9fff]", text) is not None



def normalize_heading_title(heading: str) -> str:
    title = heading.removeprefix("##").strip()
    if title.startswith("**") and title.endswith("**"):
        title = title[2:-2].strip()
    return title



def extract_sections(text: str) -> list[dict[str, str]]:
    matches = re.finditer(r"^(##\s+.+)$", text, re.MULTILINE)
    sections: list[dict[str, str]] = []
    positions = [(match.start(), match.group(1)) for match in matches]
    for index, (start, heading) in enumerate(positions):
        body_start = start + len(heading) + 1
        end = positions[index + 1][0] if index + 1 < len(positions) else len(text)
        body = text[body_start:end].strip()
        sections.append(
            {
                "heading": heading,
                "title": normalize_heading_title(heading),
                "body": body,
            }
        )
    return sections



def find_section(sections: list[dict[str, str]], title: str) -> dict[str, str] | None:
    for section in sections:
        if section["title"].lower() == title.lower():
            return section
    return None



def parse_quick_contract(body: str) -> dict[str, str] | None:
    contract: dict[str, str] = {}
    for line in body.splitlines():
        match = re.match(r"^\s*-\s+\*\*(.+?)\*\*:\s*(.+?)\s*$", line)
        if not match:
            continue
        raw_key = match.group(1).strip().lower()
        mapped_key = QUICK_CONTRACT_KEY_MAP.get(raw_key)
        if mapped_key:
            contract[mapped_key] = match.group(2).strip()
    return contract or None



def _parse_pipe_row(line: str) -> list[str]:
    parts = [cell.strip() for cell in line.strip().split("|")]
    if parts and parts[0] == "":
        parts = parts[1:]
    if parts and parts[-1] == "":
        parts = parts[:-1]
    return parts



def _strip_inline_code(text: str) -> str:
    value = text.strip()
    if value.startswith("`") and value.endswith("`") and len(value) >= 2:
        return value[1:-1]
    return value



def parse_input_arguments(body: str) -> list[dict[str, str]] | None:
    table_lines = [line.strip() for line in body.splitlines() if line.strip().startswith("|")]
    if len(table_lines) >= 3:
        header_cells = _parse_pipe_row(table_lines[0])
        header_keys = [INPUT_ARGUMENT_HEADER_MAP.get(cell.strip().lower()) for cell in header_cells]
        if not any(key is None for key in header_keys) and list(header_keys)[:3] == ["parameter", "type", "required"]:
            parsed_table_rows: list[dict[str, str]] = []
            for line in table_lines[2:]:
                cells = _parse_pipe_row(line)
                if len(cells) != len(header_keys):
                    continue
                row = {
                    key: _strip_inline_code(value)
                    for key, value in zip(header_keys, cells)
                    if key is not None
                }
                if row:
                    parsed_table_rows.append(row)
            if parsed_table_rows:
                return parsed_table_rows

    pattern = re.compile(
        r"^\d+\.\s+\*\*(?P<parameter>[^*]+)\*\*\s*\((?P<type>[^,]+),\s*(?P<required>[^)]+)\)",
        re.MULTILINE,
    )
    matches = list(pattern.finditer(body))
    if not matches:
        return None

    rows: list[dict[str, str]] = []
    for index, match in enumerate(matches):
        start = match.end()
        end = matches[index + 1].start() if index + 1 < len(matches) else len(body)
        block = body[start:end]
        description_match = re.search(r"\*\s*Description:\s*(.+)", block)
        row = {
            "parameter": _strip_inline_code(match.group("parameter").strip()),
            "type": match.group("type").strip(),
            "required": match.group("required").strip(),
            "description": description_match.group(1).strip() if description_match else "",
        }
        if row["description"]:
            rows.append(row)
    return rows or None



def parse_outputs_section(body: str) -> dict[str, str] | None:
    fenced_match = re.search(r"```([a-zA-Z0-9_-]*)\n(.*?)\n```", body, re.DOTALL)
    if not fenced_match:
        return None
    language = fenced_match.group(1).strip() or "text"
    content = fenced_match.group(2).strip()
    return {
        "format": language,
        "content": content,
    }



def build_skill_document(text: str) -> SkillDocument:
    frontmatter, _ = parse_frontmatter(text)
    sections = extract_sections(text)
    section_index = {section["heading"]: section for section in sections}
    document: SkillDocument = {
        "frontmatter": frontmatter,
        "sections": sections,
        "section_index": section_index,
    }

    quick_contract_section = find_section(sections, "Quick Contract")
    if quick_contract_section is not None:
        quick_contract = parse_quick_contract(quick_contract_section["body"])
        if quick_contract is not None:
            document["quick_contract"] = quick_contract

    input_arguments_section = find_section(sections, "Input Arguments")
    if input_arguments_section is not None:
        input_arguments = parse_input_arguments(input_arguments_section["body"])
        if input_arguments is not None:
            document["input_arguments"] = input_arguments

    outputs_section = find_section(sections, "Outputs")
    if outputs_section is not None:
        parsed_outputs = parse_outputs_section(outputs_section["body"])
        if parsed_outputs is not None:
            document["outputs_section"] = parsed_outputs

    return document



def load_skill_schema(root: Path) -> dict[str, Any]:
    schema_path = root / "schemas" / "skill.schema.json"
    return json.loads(schema_path.read_text(encoding="utf-8"))



def validate_skill_schema(document: SkillDocument, path_label: str, root: Path) -> list[str]:
    try:
        validate(instance=document, schema=load_skill_schema(root))
    except ValidationError as exc:
        return [f"{path_label}: schema validation failed: {exc.message}"]
    return []



def extract_section(text: str, heading: str) -> str | None:
    escaped = re.escape(heading)
    pattern = rf"^{escaped}\n(.*?)(?=^## |\Z)"
    match = re.search(pattern, text, re.MULTILINE | re.DOTALL)
    if not match:
        return None
    return match.group(1)



def validate_text(text: str, path_label: str, root: Path | None = None) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []
    fm, err = parse_frontmatter(text)
    if err:
        errors.append(f"{path_label}: {err}")
        return errors, warnings

    document = build_skill_document(text)
    if root is not None:
        errors.extend(validate_skill_schema(document, path_label, root))

    if find_section(document["sections"], "Quick Contract") is not None and "quick_contract" not in document:
        errors.append(f"{path_label}: unable to parse structured quick contract")
    if find_section(document["sections"], "Input Arguments") is not None and "input_arguments" not in document:
        errors.append(f"{path_label}: unable to parse structured input arguments")
    if find_section(document["sections"], "Outputs") is not None and "outputs_section" not in document:
        errors.append(f"{path_label}: unable to parse structured outputs section")

    if "name" not in fm or not fm["name"]:
        errors.append(f"{path_label}: missing frontmatter name")
    if "description" not in fm or not fm["description"]:
        errors.append(f"{path_label}: missing frontmatter description")
    if "name" in fm and not re.fullmatch(r"[a-z0-9:-]+", fm["name"]):
        errors.append(f"{path_label}: name contains unsupported characters")
    if "name" in fm and fm["name"] and not is_supported_skill(fm["name"]) and fm["name"] not in FUSION_GENERIC_SKILLS:
        errors.append(f"{path_label}: skill name must use arc-xxx namespace")
    description = fm.get("description", "")
    if description and not contains_cjk(description):
        errors.append(f"{path_label}: description must contain Chinese text")

    skill_name = fm.get("name", "")
    enforce_fusion_profile = skill_name in FUSION_GENERIC_SKILLS
    enforce_arc_profile = skill_name in ARC_ROUTED_SKILLS

    if enforce_arc_profile:
        allowed_keys = {"name", "description", "version", "allowed_tools", "hooks"}
        extra_keys = sorted(key for key in fm.keys() if key not in allowed_keys)
        if extra_keys:
            errors.append(
                f"{path_label}: arc frontmatter allows only name/description, found extra keys: {', '.join(extra_keys)}"
            )

    for heading in REQUIRED_HEADINGS:
        if heading not in text:
            message = f"{path_label}: missing heading {heading}"
            if enforce_fusion_profile or enforce_arc_profile:
                errors.append(message)
            else:
                warnings.append(message)

    if enforce_fusion_profile:
        for heading in GENERIC_REQUIRED_HEADINGS:
            if heading not in text:
                errors.append(f"{path_label}: missing heading {heading}")
        for marker in GENERIC_WHEN_TO_USE_MARKERS:
            if marker not in text:
                errors.append(f"{path_label}: generic when-to-use missing marker {marker}")

    if enforce_arc_profile:
        for heading in ARC_FUSION_REQUIRED_HEADINGS:
            if heading not in text:
                errors.append(f"{path_label}: missing heading {heading}")
        for marker_variants in WHEN_TO_USE_MARKER_VARIANTS:
            if not any(variant in text for variant in marker_variants):
                errors.append(f"{path_label}: arc when-to-use missing marker {marker_variants[0]} (or English equivalent)")
        if ARC_ROUTING_MATRIX_LINK not in text:
            errors.append(f"{path_label}: missing routing matrix link {ARC_ROUTING_MATRIX_LINK}")
        if ARC_DECISION_TREE_LINK not in text:
            errors.append(f"{path_label}: missing decision tree link {ARC_DECISION_TREE_LINK}")
        if ARC_PHASE_VIEW_LINK not in text:
            errors.append(f"{path_label}: missing phase view link {ARC_PHASE_VIEW_LINK}")
        if ARC_CHEATSHEET_LINK not in text:
            errors.append(f"{path_label}: missing cheatsheet link {ARC_CHEATSHEET_LINK}")
        expert_section = extract_section(text, "## Expert Standards")
        if expert_section is None:
            errors.append(f"{path_label}: missing expert standards section body")
        else:
            required_keywords = ARC_EXPERT_KEYWORDS.get(skill_name, [])
            missing_keywords: list[str] = []
            expert_lower = expert_section.lower()
            for kw in required_keywords:
                if isinstance(kw, list):
                    if not any(variant.lower() in expert_lower for variant in kw):
                        missing_keywords.append(kw[0])
                else:
                    if kw.lower() not in expert_lower:
                        missing_keywords.append(kw)
            if missing_keywords:
                errors.append(
                    f"{path_label}: expert standards missing skill-specific keywords: {', '.join(missing_keywords)}"
                )

    for token in BANNED_TOKENS:
        if token in text:
            errors.append(f"{path_label}: contains banned token '{token}'")

    return errors, warnings



def validate_file(path: Path, root: Path | None = None) -> tuple[list[str], list[str]]:
    text = path.read_text(encoding="utf-8")
    return validate_text(text, str(path), root=root)


def validate_repo_policies(root: Path) -> list[str]:
    errors: list[str] = []
    workflows_dir = root / ".github" / "workflows"
    if workflows_dir.exists():
        workflow_files = sorted(
            path.relative_to(root)
            for path in workflows_dir.rglob("*")
            if path.is_file()
        )
        if workflow_files:
            listed = ", ".join(str(path) for path in workflow_files[:5])
            if len(workflow_files) > 5:
                listed += ", ..."
            errors.append(
                "repository policy violation: GitHub Actions workflows are not allowed in this skills repository; "
                f"remove {listed}"
            )
        else:
            errors.append(
                "repository policy violation: empty .github/workflows directory is not allowed in this skills repository"
            )
    return errors



def collect_skill_files(root: Path) -> list[Path]:
    collected: list[Path] = []
    for path in sorted(root.rglob("SKILL.md")):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        skill_name = frontmatter.get("name", "")
        if is_supported_skill(skill_name) or skill_name in FUSION_GENERIC_SKILLS:
            collected.append(path)
    return collected


def find_skill_file(root: Path, skill_name: str) -> Path | None:
    namespace_dir = get_namespace_dir(skill_name)
    candidates: list[Path] = []
    if namespace_dir:
        candidates.append(root / namespace_dir / skill_name / "SKILL.md")
    candidates.append(root / skill_name / "SKILL.md")

    seen: set[Path] = set()
    for candidate in candidates:
        if candidate in seen:
            continue
        seen.add(candidate)
        if candidate.exists():
            return candidate

    for path in collect_skill_files(root):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        if frontmatter.get("name", "") == skill_name:
            return path
    return None



def run_validation(root: Path) -> tuple[list[str], list[str], int]:
    skill_files = collect_skill_files(root)
    all_errors: list[str] = validate_repo_policies(root)
    all_warnings: list[str] = []
    for path in skill_files:
        errors, warnings = validate_file(path, root=root)
        all_errors.extend(errors)
        all_warnings.extend(warnings)
    return all_errors, all_warnings, len(skill_files)
