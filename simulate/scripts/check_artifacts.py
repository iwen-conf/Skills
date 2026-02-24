#!/usr/bin/env python3
"""
Quality-gate checks for ui-ux-simulation run artifacts.

Checks (non-exhaustive):
  - Required files/dirs exist
  - Screenshot paths referenced in manifests exist
  - events.jsonl is parseable (optional)
  - accounts.jsonc exists and is parseable
  - Markdown table format validation:
    - Header row, separator row, and data rows must have consistent column counts
    - Separator row cells must match pattern: ---, :---, ---:, or :---:

Always run with --help first.
"""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


_BACKTICK_PATH_RE = re.compile(r"`([^`]+)`")


def _strip_jsonc(text: str) -> str:
    """
    Remove // and /* */ comments from JSONC, while preserving string literals.
    """
    out: list[str] = []
    i = 0
    in_string = False
    escape = False
    in_line_comment = False
    in_block_comment = False

    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""

        if in_line_comment:
            if ch in "\r\n":
                in_line_comment = False
                out.append(ch)
            i += 1
            continue

        if in_block_comment:
            if ch == "*" and nxt == "/":
                in_block_comment = False
                i += 2
            else:
                i += 1
            continue

        if in_string:
            out.append(ch)
            if escape:
                escape = False
            else:
                if ch == "\\":
                    escape = True
                elif ch == '"':
                    in_string = False
            i += 1
            continue

        if ch == '"':
            in_string = True
            out.append(ch)
            i += 1
            continue

        if ch == "/" and nxt == "/":
            in_line_comment = True
            i += 2
            continue

        if ch == "/" and nxt == "*":
            in_block_comment = True
            i += 2
            continue

        out.append(ch)
        i += 1

    return "".join(out)


def _read_jsonl(path: Path) -> list[str]:
    errors: list[str] = []
    if not path.exists():
        return errors
    for i, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            json.loads(line)
        except json.JSONDecodeError as e:
            errors.append(f"{path.name}:{i}: invalid JSON ({e})")
    return errors


def _extract_paths_from_markdown(md: str) -> list[str]:
    return [m.group(1) for m in _BACKTICK_PATH_RE.finditer(md)]


def _validate_markdown_tables(path: Path) -> list[str]:
    """
    Validate that all Markdown tables have consistent column counts.
    Returns a list of error messages (empty if all tables are valid).
    """
    errors: list[str] = []
    if not path.exists():
        return errors

    lines = path.read_text(encoding="utf-8").splitlines()
    table_start_line = -1
    header_col_count = 0

    for i, line in enumerate(lines, start=1):
        stripped = line.strip()

        # Detect table row (starts and ends with |, or just contains multiple |)
        if "|" in stripped:
            # Count columns: split by | and filter empty edge parts
            parts = stripped.split("|")
            # Remove empty strings at start/end caused by leading/trailing |
            if parts and parts[0].strip() == "":
                parts = parts[1:]
            if parts and parts[-1].strip() == "":
                parts = parts[:-1]
            col_count = len(parts)

            if table_start_line == -1:
                # First row of a new table (header row)
                table_start_line = i
                header_col_count = col_count
            else:
                # Check separator row format (second row of table)
                if i == table_start_line + 1:
                    # This should be the separator row
                    sep_pattern = re.compile(r"^:?-+:?$")
                    for j, cell in enumerate(parts):
                        cell = cell.strip()
                        if not sep_pattern.match(cell):
                            errors.append(
                                f"{path.name}:{i}: table separator row column {j+1} is invalid: '{cell}' "
                                f"(expected pattern like '---', ':---', '---:', ':---:')"
                            )

                # Check column count consistency
                if col_count != header_col_count:
                    errors.append(
                        f"{path.name}:{i}: table column count mismatch - "
                        f"expected {header_col_count} columns (from header at line {table_start_line}), "
                        f"but found {col_count} columns"
                    )
        else:
            # Non-table line: reset table state
            if stripped == "" or not stripped.startswith("|"):
                table_start_line = -1
                header_col_count = 0

    return errors


def main() -> None:
    parser = argparse.ArgumentParser(description="Validate ui-ux-simulation run artifacts")
    parser.add_argument("--run-dir", required=True, help="Run directory (e.g. reports/<run_id>/)")
    parser.add_argument("--strict", action="store_true", help="Fail on missing optional artifacts and JSONL errors")
    args = parser.parse_args()

    run_dir = Path(args.run_dir)
    problems: list[str] = []

    required_dirs = ["screenshots", "failures", "db"]
    required_files = ["accounts.jsonc", "report.md", "action-log.md", "screenshot-manifest.md"]

    for d in required_dirs:
        p = run_dir / d
        if not p.exists() or not p.is_dir():
            problems.append(f"missing dir: {d}/")

    for f in required_files:
        p = run_dir / f
        if not p.exists() or not p.is_file():
            problems.append(f"missing file: {f}")

    # Validate accounts.jsonc is parseable JSONC (comments allowed)
    accounts_path = run_dir / "accounts.jsonc"
    if accounts_path.exists():
        try:
            json.loads(_strip_jsonc(accounts_path.read_text(encoding="utf-8")))
        except json.JSONDecodeError as e:
            problems.append(f"invalid accounts.jsonc: {e}")

    # Validate screenshot paths referenced in screenshot-manifest*.md
    manifest_candidates = [
        run_dir / "screenshot-manifest.md",
        run_dir / "screenshot-manifest.compiled.md",
    ]
    for manifest in manifest_candidates:
        if not manifest.exists():
            continue
        md = manifest.read_text(encoding="utf-8")
        for path_str in _extract_paths_from_markdown(md):
            if not path_str.startswith("screenshots/"):
                continue
            if not (run_dir / path_str).exists():
                problems.append(f"missing screenshot file referenced in {manifest.name}: {path_str}")

    # Optional: events.jsonl parse
    jsonl_errors = _read_jsonl(run_dir / "events.jsonl")
    if jsonl_errors and args.strict:
        problems.extend([f"events.jsonl: {e}" for e in jsonl_errors])

    # Validate Markdown table format in key files
    md_files_to_check = [
        run_dir / "report.md",
        run_dir / "screenshot-manifest.md",
        run_dir / "screenshot-manifest.compiled.md",
        run_dir / "action-log.md",
        run_dir / "action-log.compiled.md",
    ]
    for md_file in md_files_to_check:
        table_errors = _validate_markdown_tables(md_file)
        if table_errors:
            problems.extend(table_errors)

    if problems:
        for p in problems:
            print(f"[FAIL] {p}")
        raise SystemExit(1)

    if jsonl_errors:
        for e in jsonl_errors:
            print(f"[WARN] {e}")

    print("OK")


if __name__ == "__main__":
    main()
