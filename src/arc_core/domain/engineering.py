from __future__ import annotations

import re
from pathlib import Path

from .skill import (
    ARC_REQUIRED_HEADINGS,
    DESCRIPTION_MAX_CHARS,
    DESCRIPTION_MIN_CHARS,
    FIRST_PERSON_RE,
    INTENT_ROUTER_MIN_ROWS,
    MODULE_MD_MAX_LINES,
    RETIRED_ARC_SKILL_RE,
    SKILL_MD_MAX_LINES,
    VAGUE_PHRASES,
    estimate_tokens,
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


def strip_code(text: str) -> str:
    without_fences = re.sub(r"^(```|~~~).*?^\1[^\n]*$", "", text, flags=re.S | re.M)
    return re.sub(r"`[^`\n]*`", "", without_fences)


def h2_sections(text: str) -> list[tuple[str, str]]:
    """Return (title, body) for every H2 outside fenced code blocks."""
    sections: list[tuple[str, list[str]]] = []
    fence: str | None = None
    for line in text.splitlines():
        marker = re.match(r"^\s*(```|~~~)", line)
        if marker:
            fence = None if fence == marker.group(1) else (fence or marker.group(1))
        if fence is None and (heading := re.match(r"^##\s+(.+?)\s*#*\s*$", line)):
            sections.append((heading.group(1).strip(), []))
        elif sections:
            sections[-1][1].append(line)
    return [(title, "\n".join(body).strip()) for title, body in sections]


def validate_arc_structure(text: str, path_label: str) -> list[str]:
    sections = h2_sections(text)
    titles = [title for title, _ in sections]
    if titles != ARC_REQUIRED_HEADINGS:
        missing = [title for title in ARC_REQUIRED_HEADINGS if title not in titles]
        extra = [title for title in titles if title not in ARC_REQUIRED_HEADINGS]
        detail = []
        if missing:
            detail.append("missing " + ", ".join(f"## {title}" for title in missing))
        if extra:
            detail.append("unexpected " + ", ".join(f"## {title}" for title in extra))
        if not detail:
            detail.append("headings out of order")
        return [
            f"{path_label}: Arc SKILL.md must use the H2 headings "
            + " → ".join(ARC_REQUIRED_HEADINGS)
            + f" ({'; '.join(detail)})"
        ]
    return [f"{path_label}: ## {title} must not be empty" for title, body in sections if not body]


def validate_not_for(description: str, path_label: str) -> list[str]:
    if "not for" not in description.lower():
        return [f"{path_label}: Arc description must name adjacent skills in a 'Not for … (use X)' clause"]
    return []


def validate_token_budget(text: str, path_label: str, limit: int, kind: str) -> list[str]:
    tokens = estimate_tokens(text)
    if tokens > limit:
        return [f"{path_label}: {kind} exceeds {limit} estimated tokens ({tokens}); move detail into references"]
    return []


def validate_retired_names(text: str, path_label: str) -> list[str]:
    found = sorted(set(RETIRED_ARC_SKILL_RE.findall(text)))
    if found:
        return [f"{path_label}: references retired Arc skills: {', '.join(found)}"]
    return []


def vague_phrase_warnings(text: str, path_label: str) -> list[str]:
    prose = strip_code(text).lower()
    found = [phrase for phrase in VAGUE_PHRASES if phrase in prose]
    if found:
        return [f"{path_label}: vague wording ({', '.join(found)}); state an IF/THEN rule or threshold"]
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
