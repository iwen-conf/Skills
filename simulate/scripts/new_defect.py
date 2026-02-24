#!/usr/bin/env python3
"""
Create a new failure/defect markdown file under <run_dir>/failures/.

Optionally appends a row into <run_dir>/execution/defect-log.md if it exists.

Always run with --help first.
"""

from __future__ import annotations

import argparse
import re
from datetime import datetime
from pathlib import Path


_FAILURE_RE = re.compile(r"failure-(\d{4})\.md$")


def _next_failure_id(failures_dir: Path) -> int:
    max_id = 0
    for p in failures_dir.glob("failure-*.md"):
        m = _FAILURE_RE.search(p.name)
        if not m:
            continue
        max_id = max(max_id, int(m.group(1)))
    return max_id + 1


def _append_defect_log(defect_log: Path, *, defect_id: str, title: str, severity: str, status: str, evidence: str) -> None:
    if not defect_log.exists():
        return

    row = f"| {defect_id} | {title} | {severity} | P? | <env> | {status} | <owner> | <run_id> | {evidence} | <link> |"

    text = defect_log.read_text(encoding="utf-8")
    if "| Defect ID |" not in text:
        return

    # Append after the table header separator line if present; otherwise append to end.
    lines = text.splitlines()
    insert_at = len(lines)
    for i, line in enumerate(lines):
        if line.strip().startswith("|---") or line.strip().startswith("| ---"):
            insert_at = i + 1
            break
    lines.insert(insert_at, row)
    defect_log.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Create a new failure report under failures/")
    parser.add_argument("--run-dir", required=True, help="Run directory (e.g. reports/<run_id>/)")
    parser.add_argument("--step", default="", help="Step ID (e.g. 0007)")
    parser.add_argument("--title", required=True, help="One-line failure title")
    parser.add_argument("--role", default="", help="Persona/role")
    parser.add_argument("--url", default="", help="Current URL")
    parser.add_argument("--user", default="", help="Plaintext username")
    parser.add_argument("--password", default="", help="Plaintext password")
    parser.add_argument("--token", default="", help="Plaintext token (optional)")
    parser.add_argument("--screenshot", default="", help="Screenshot path (relative, e.g. screenshots/s0007_after-submit.png)")
    parser.add_argument("--severity", default="S2", help="Severity (default: S2)")
    parser.add_argument("--status", default="OPEN", help="Status (default: OPEN)")
    args = parser.parse_args()

    run_dir = Path(args.run_dir)
    failures_dir = run_dir / "failures"
    failures_dir.mkdir(parents=True, exist_ok=True)

    failure_id_int = _next_failure_id(failures_dir)
    failure_id = f"{failure_id_int:04d}"
    filename = failures_dir / f"failure-{failure_id}.md"

    token_line = f"* **Token**: `{args.token}`" if args.token else "* **Token**: `<token>`"
    screenshot_line = (
        f"* **Path**: `{args.screenshot}`" if args.screenshot else "* **Path**: `screenshots/<sXXXX_slug>.png`"
    )

    md = f"""# 🛑 E2E Test Failure Report

## Context
* **Title**: {args.title}
* **Task/Step**: {args.step or "<step_id>"}
* **Role**: {args.role or "<role>"}
* **Time**: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}
* **URL**: {args.url or "<url>"}
* **Severity**: {args.severity}
* **Status**: {args.status}

## Debug Artifacts (PLAIN TEXT)
* **Accounts file**: `accounts.jsonc`
* **User**: `{args.user or "<user>"}`
* **Password**: `{args.password or "<password>"}`
{token_line}

## Screenshot Evidence
* **Step**: `{args.step or "<step_id>"}`
{screenshot_line}
* **Description**: <what happened on screen>
* **Expected**: <what should have happened>

## Reproduction Steps
1. <step 1>
2. <step 2>
3. <step 3>

## Evidence
* **UI State**: <toast/modal/error text>
* **DB State (optional)**:
  - Query: `db/query-0001.txt`
  - Result: `db/result-0001.txt`
"""

    filename.write_text(md, encoding="utf-8")

    _append_defect_log(
        run_dir / "execution" / "defect-log.md",
        defect_id=f"D-{failure_id}",
        title=args.title,
        severity=args.severity,
        status=args.status,
        evidence=f"`{args.screenshot}`" if args.screenshot else "`<evidence>`",
    )

    print(str(filename))


if __name__ == "__main__":
    main()
