#!/usr/bin/env python3
from pathlib import Path
import re
import sys


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
    "## Red Flags",
]

ARC_WHEN_TO_USE_MARKERS = [
    "**首选触发**",
    "**典型场景**",
    "**边界提示**",
]

GENERIC_WHEN_TO_USE_MARKERS = [
    "**首选触发**",
    "**典型场景**",
    "**边界提示**",
]

ARC_ROUTING_MATRIX_LINK = "../docs/arc-routing-matrix.md"
ARC_DECISION_TREE_LINK = "../docs/arc-routing-matrix.md#signal-to-skill-decision-tree"
ARC_PHASE_VIEW_LINK = "../docs/arc-routing-matrix.md#phase-routing-view"
ARC_CHEATSHEET_LINK = "../docs/arc-routing-cheatsheet.md"

LEGACY_TOKEN_PARTS = [
    ("Ta", "sk("),
    ("subagent", "_type"),
    ("load", "_skills"),
    ("run", "_in_background"),
]

BANNED_TOKENS = [
    *(("".join(parts)) for parts in LEGACY_TOKEN_PARTS),
    "session_id",
    "arc:estimate",
    "arc:retest",
    "arc:init:full",
    "arc:init:update",
]

FUSION_GENERIC_SKILLS = set()

ARC_ROUTED_SKILLS = {
    "arc:exec",
    "arc:cartography",
    "arc:decide",
    "arc:release",
    "arc:build",
    "arc:init",
    "arc:ip-check",
    "arc:ip-draft",
    "arc:clarify",
    "arc:audit",
    "arc:e2e",
    "arc:fix",
    "arc:model",
}


def parse_frontmatter(text: str):
    if not text.startswith("---\n"):
        return {}, "missing frontmatter start"
    end = text.find("\n---\n", 4)
    if end == -1:
        return {}, "missing frontmatter end"
    block = text[4:end].strip().splitlines()
    data = {}
    for line in block:
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        data[key.strip()] = value.strip().strip('"').strip("'")
    return data, None


def is_generic_skill(name: str):
    return ":" not in name


def is_arc_skill(name: str):
    return name.startswith("arc:")


def contains_cjk(text: str):
    return re.search(r"[\u4e00-\u9fff]", text) is not None


def validate_file(path: Path):
    errors = []
    warnings = []
    text = path.read_text(encoding="utf-8")
    fm, err = parse_frontmatter(text)
    if err:
        errors.append(f"{path}: {err}")
        return errors, warnings
    if "name" not in fm or not fm["name"]:
        errors.append(f"{path}: missing frontmatter name")
    if "description" not in fm or not fm["description"]:
        errors.append(f"{path}: missing frontmatter description")
    if "name" in fm and not re.fullmatch(r"[a-z0-9:-]+", fm["name"]):
        errors.append(f"{path}: name contains unsupported characters")
    if "name" in fm and fm["name"] and not is_arc_skill(fm["name"]):
        errors.append(f"{path}: skill name must use arc:xxx namespace")
    description = fm.get("description", "")
    if description and not contains_cjk(description):
        errors.append(f"{path}: description must contain Chinese text")
    skill_name = fm.get("name", "")
    enforce_fusion_profile = skill_name in FUSION_GENERIC_SKILLS
    enforce_arc_profile = skill_name in ARC_ROUTED_SKILLS
    if enforce_arc_profile:
        allowed_keys = {"name", "description"}
        extra_keys = sorted(key for key in fm.keys() if key not in allowed_keys)
        if extra_keys:
            errors.append(
                f"{path}: arc frontmatter allows only name/description, found extra keys: {', '.join(extra_keys)}"
            )
    for heading in REQUIRED_HEADINGS:
        if heading not in text:
            message = f"{path}: missing heading {heading}"
            if enforce_fusion_profile or enforce_arc_profile:
                errors.append(message)
            else:
                warnings.append(message)
    if enforce_fusion_profile:
        for heading in GENERIC_REQUIRED_HEADINGS:
            if heading not in text:
                errors.append(f"{path}: missing heading {heading}")
        for marker in GENERIC_WHEN_TO_USE_MARKERS:
            if marker not in text:
                errors.append(f"{path}: generic when-to-use missing marker {marker}")
    if enforce_arc_profile:
        for heading in ARC_FUSION_REQUIRED_HEADINGS:
            if heading not in text:
                errors.append(f"{path}: missing heading {heading}")
        for marker in ARC_WHEN_TO_USE_MARKERS:
            if marker not in text:
                errors.append(f"{path}: arc when-to-use missing marker {marker}")
        if ARC_ROUTING_MATRIX_LINK not in text:
            errors.append(f"{path}: missing routing matrix link {ARC_ROUTING_MATRIX_LINK}")
        if ARC_DECISION_TREE_LINK not in text:
            errors.append(f"{path}: missing decision tree link {ARC_DECISION_TREE_LINK}")
        if ARC_PHASE_VIEW_LINK not in text:
            errors.append(f"{path}: missing phase view link {ARC_PHASE_VIEW_LINK}")
        if ARC_CHEATSHEET_LINK not in text:
            errors.append(f"{path}: missing cheatsheet link {ARC_CHEATSHEET_LINK}")
    for token in BANNED_TOKENS:
        if token in text:
            errors.append(f"{path}: contains banned token '{token}'")
    return errors, warnings


def main():
    root = Path(__file__).resolve().parents[1]
    skill_files = []
    for path in root.rglob("SKILL.md"):
        skill_files.append(path)
    all_errors = []
    all_warnings = []
    for path in sorted(skill_files):
        errors, warnings = validate_file(path)
        all_errors.extend(errors)
        all_warnings.extend(warnings)
    if all_errors:
        print("Skill validation failed:")
        for item in all_errors:
            print(f"- {item}")
        if all_warnings:
            print("\nSkill validation warnings:")
            for item in all_warnings:
                print(f"- {item}")
        return 1
    if all_warnings:
        print("Skill validation warnings:")
        for item in all_warnings:
            print(f"- {item}")
    print(f"Skill validation passed for {len(skill_files)} files.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
