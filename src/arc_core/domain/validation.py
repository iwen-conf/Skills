from __future__ import annotations
import re
from pathlib import Path
from ..domain.skill import (
    is_supported_skill, REQUIRED_HEADINGS, ARC_REQUIRED_HEADINGS,
    WHEN_TO_USE_MARKER_VARIANTS, ARC_ROUTING_MATRIX_LINK, BANNED_TOKENS
)
from ..infrastructure.markdown import parse_frontmatter, build_skill_document, find_section, extract_section
from ..infrastructure.schema import validate_skill_schema

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
    if "name" in fm and fm["name"] and not is_supported_skill(fm["name"]):
        errors.append(f"{path_label}: skill name must use arc:xxx namespace")
    description = fm.get("description", "")
    if description and len(description) > 120:
        errors.append(f"{path_label}: description must be short, at most 120 characters")

    skill_name = fm.get("name", "")
    enforce_arc_profile = fm.get("enforce_arc_profile", False)

    if enforce_arc_profile:
        allowed_keys = {"name", "description", "version", "allowed_tools", "hooks", "enforce_arc_profile", "expert_keywords"}
        extra_keys = sorted(key for key in fm.keys() if key not in allowed_keys)
        if extra_keys:
            errors.append(
                f"{path_label}: arc frontmatter contains unsupported keys: {', '.join(extra_keys)}"
            )

    for heading in REQUIRED_HEADINGS:
        if heading not in text:
            message = f"{path_label}: missing heading {heading}"
            if enforce_arc_profile:
                errors.append(message)
            else:
                warnings.append(message)

    if enforce_arc_profile:
        for heading in ARC_REQUIRED_HEADINGS:
            if heading not in text:
                errors.append(f"{path_label}: missing heading {heading}")
        for marker_variants in WHEN_TO_USE_MARKER_VARIANTS:
            if not any(variant in text for variant in marker_variants):
                errors.append(f"{path_label}: arc when-to-use missing marker {marker_variants[0]} (or English equivalent)")
        if ARC_ROUTING_MATRIX_LINK not in text:
            errors.append(f"{path_label}: missing routing matrix link {ARC_ROUTING_MATRIX_LINK}")
        expert_section = extract_section(text, "## Expert Standards")
        if expert_section is None:
            errors.append(f"{path_label}: missing expert standards section body")
        else:
            required_keywords = fm.get("expert_keywords", [])
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
