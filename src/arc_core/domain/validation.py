from __future__ import annotations
import re
from pathlib import Path
from ..domain.skill import (
    BANNED_FRONTMATTER_KEYS,
    BANNED_TOKENS,
    REQUIRED_HEADINGS,
    is_supported_skill,
)
from ..domain.engineering import (
    iter_skill_markdown,
    validate_description,
    validate_intent_router,
    validate_line_budget,
    validate_red_lines,
    validate_relative_links,
    validate_trigger_terms,
)
from ..infrastructure.markdown import parse_frontmatter, build_skill_document, find_section
from ..infrastructure.schema import validate_skill_schema

SEMVER_RE = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")


def validate_text(
    text: str,
    path_label: str,
    root: Path | None = None,
    skill_path: Path | None = None,
) -> tuple[list[str], list[str]]:
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
    if "version" not in fm or not fm["version"]:
        errors.append(f"{path_label}: missing frontmatter version")
    elif not SEMVER_RE.fullmatch(str(fm["version"])):
        errors.append(f"{path_label}: version must be semver X.Y.Z")
    if "name" in fm and not re.fullmatch(r"[a-z0-9:-]+", str(fm["name"])):
        errors.append(f"{path_label}: name contains unsupported characters")
    if "name" in fm and fm["name"] and not is_supported_skill(str(fm["name"])):
        errors.append(f"{path_label}: skill name must use arc:xxx namespace")

    banned_keys = sorted(key for key in fm.keys() if key in BANNED_FRONTMATTER_KEYS)
    if banned_keys:
        errors.append(
            f"{path_label}: frontmatter contains retired keys: {', '.join(banned_keys)}"
        )

    description = str(fm.get("description", "") or "")
    errors.extend(validate_description(description, path_label))

    skill_name = str(fm.get("name", "") or "")
    if root is not None and skill_path is not None and skill_name:
        errors.extend(validate_trigger_terms(description, skill_name, path_label, root))

    for heading in REQUIRED_HEADINGS:
        if heading not in text:
            errors.append(f"{path_label}: missing heading {heading}")

    errors.extend(validate_intent_router(document, path_label))
    errors.extend(validate_red_lines(document, path_label))
    errors.extend(validate_line_budget(text, path_label, is_skill_md=True))

    if skill_path is not None:
        skill_dir = skill_path.parent
        errors.extend(validate_relative_links(text, path_label, skill_dir))
        for markdown_path in iter_skill_markdown(skill_dir):
            if markdown_path == skill_path:
                continue
            module_text = markdown_path.read_text(encoding="utf-8")
            rel = str(markdown_path)
            errors.extend(validate_line_budget(module_text, rel, is_skill_md=False))
            errors.extend(validate_relative_links(module_text, rel, markdown_path.parent))

    for token in BANNED_TOKENS:
        if token in text:
            errors.append(f"{path_label}: contains banned token '{token}'")

    return errors, warnings
