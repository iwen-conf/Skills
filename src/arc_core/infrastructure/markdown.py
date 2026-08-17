from __future__ import annotations
import re
import yaml  # type: ignore[import-untyped]
from typing import Any
from ..domain.skill import SkillDocument, QUICK_CONTRACT_KEY_MAP, INPUT_ARGUMENT_HEADER_MAP

MARKDOWN_LINK_RE = re.compile(r"\[[^\]]*\]\(([^)]+)\)")

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

def parse_intent_router(body: str) -> list[dict[str, str]] | None:
    table_lines = [line.strip() for line in body.splitlines() if line.strip().startswith("|")]
    if len(table_lines) < 3:
        return None
    header_cells = [_strip_inline_code(cell) for cell in _parse_pipe_row(table_lines[0])]
    if len(header_cells) < 2:
        return None
    rows: list[dict[str, str]] = []
    for line in table_lines[2:]:
        cells = [_strip_inline_code(cell) for cell in _parse_pipe_row(line)]
        if len(cells) != len(header_cells):
            continue
        row = {header_cells[index]: cells[index] for index in range(len(header_cells))}
        if sum(1 for value in row.values() if value) >= 2:
            rows.append(row)
    return rows or None


def extract_relative_links(text: str) -> list[str]:
    stripped = re.sub(r"```.*?```", "", text, flags=re.S)
    links: list[str] = []
    seen: set[str] = set()
    for raw in MARKDOWN_LINK_RE.findall(stripped):
        target = raw.split()[0].strip("<>")
        target = target.split("#", 1)[0]
        if not target or target.startswith(("#", "http://", "https://", "mailto:")):
            continue
        if "://" in target:
            continue
        if target in seen:
            continue
        seen.add(target)
        links.append(target)
    return links


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

    intent_router_section = find_section(sections, "Intent Router")
    if intent_router_section is not None:
        intent_router = parse_intent_router(intent_router_section["body"])
        if intent_router is not None:
            document["intent_router"] = intent_router

    return document

def extract_section(text: str, heading: str) -> str | None:
    escaped = re.escape(heading)
    pattern = rf"^{escaped}\n(.*?)(?=^## |\Z)"
    match = re.search(pattern, text, re.MULTILINE | re.DOTALL)
    if not match:
        return None
    return match.group(1)
