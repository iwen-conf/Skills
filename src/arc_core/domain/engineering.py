from __future__ import annotations

from pathlib import Path

from .skill import (
    DESCRIPTION_MAX_CHARS,
    DESCRIPTION_MIN_CHARS,
    FIRST_PERSON_RE,
    INTENT_ROUTER_MIN_ROWS,
    MODULE_MD_MAX_LINES,
    SKILL_MD_MAX_LINES,
)
from .triggers import load_trigger_corpus, missing_trigger_terms
from ..infrastructure.markdown import extract_relative_links

SKIP_LINK_PREFIXES = (
    "http://",
    "https://",
    "mailto:",
    "ftp://",
)


def count_lines(text: str) -> int:
    if not text:
        return 0
    return text.count("\n") + (0 if text.endswith("\n") else 1)


def validate_description(description: str, path_label: str) -> list[str]:
    errors: list[str] = []
    compact = description.strip()
    if len(compact) < DESCRIPTION_MIN_CHARS:
        errors.append(
            f"{path_label}: description must include WHAT and WHEN, at least {DESCRIPTION_MIN_CHARS} characters"
        )
    if len(compact) > DESCRIPTION_MAX_CHARS:
        errors.append(
            f"{path_label}: description must be at most {DESCRIPTION_MAX_CHARS} characters"
        )
    if FIRST_PERSON_RE.search(compact):
        errors.append(f"{path_label}: description must be third person, not first/second person")
    return errors


def validate_intent_router(document: dict, path_label: str) -> list[str]:
    rows = document.get("intent_router")
    if not rows:
        return [f"{path_label}: ## Intent Router must contain a markdown table"]
    if len(rows) < INTENT_ROUTER_MIN_ROWS:
        return [
            f"{path_label}: ## Intent Router table must have at least {INTENT_ROUTER_MIN_ROWS} data rows"
        ]
    for index, row in enumerate(rows, start=1):
        values = [value.strip() for value in row.values() if value.strip()]
        if len(values) < 2:
            return [f"{path_label}: ## Intent Router row {index} must have at least two cells"]
    return []


def validate_red_lines(document: dict, path_label: str) -> list[str]:
    section = None
    for item in document.get("sections", []):
        if item.get("title", "").lower() == "red lines":
            section = item
            break
    if section is None or not section.get("body", "").strip():
        return [f"{path_label}: ## Red Lines must contain enforceable rules"]
    return []


def validate_line_budget(text: str, path_label: str, *, is_skill_md: bool) -> list[str]:
    limit = SKILL_MD_MAX_LINES if is_skill_md else MODULE_MD_MAX_LINES
    lines = count_lines(text)
    if lines > limit:
        kind = "SKILL.md" if is_skill_md else "module markdown"
        return [f"{path_label}: {kind} exceeds {limit} lines ({lines}); split by progressive disclosure"]
    return []


def validate_relative_links(text: str, path_label: str, skill_dir: Path) -> list[str]:
    errors: list[str] = []
    for target in extract_relative_links(text):
        if target.startswith(SKIP_LINK_PREFIXES) or target.startswith("#"):
            continue
        resolved = (skill_dir / target).resolve()
        if not resolved.exists():
            errors.append(f"{path_label}: broken relative link {target}")
    return errors


def validate_trigger_terms(description: str, skill_name: str, path_label: str, root: Path) -> list[str]:
    corpus = load_trigger_corpus(root)
    entry = corpus.get(skill_name)
    if not entry:
        return [f"{path_label}: missing trigger corpus entry for {skill_name}"]
    missing = missing_trigger_terms(description, entry)
    if missing:
        return [
            f"{path_label}: description missing trigger terms: {', '.join(missing)}"
        ]
    return []


def iter_skill_markdown(skill_dir: Path) -> list[Path]:
    return sorted(
        path
        for path in skill_dir.rglob("*.md")
        if path.is_file() and ".git" not in path.parts
    )
