#!/usr/bin/env python3
"""Scaffold an arc:ip-docs case directory."""

from __future__ import annotations

import argparse
import datetime as dt
from pathlib import Path


SUBDIRS = [
    "context",
    "copyright",
    "patent",
    "reports",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create ip-docs workspace")
    parser.add_argument("--project-path", required=True, help="Absolute path of target project")
    parser.add_argument("--project-name", help="Project name override")
    parser.add_argument("--output-dir", help="Case output directory override")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    project_path = Path(args.project_path).expanduser().resolve()
    project_name = args.project_name.strip() if args.project_name else project_path.name
    case_dir = (
        Path(args.output_dir).expanduser().resolve()
        if args.output_dir
        else project_path / ".arc" / "ip-docs" / project_name
    )

    for sub in SUBDIRS:
        (case_dir / sub).mkdir(parents=True, exist_ok=True)

    now = dt.datetime.now(dt.timezone.utc).isoformat()
    context_file = case_dir / "context" / "doc-context.md"
    if not context_file.exists():
        context_file.write_text(
            "\n".join(
                [
                    f"# Doc Context: {project_name}",
                    "",
                    f"- generated_at: {now}",
                    f"- project_path: {project_path}",
                    "- audit_handoff: pending",
                    "- notes: pending",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    log_file = case_dir / "reports" / "doc-writing-log.md"
    if not log_file.exists():
        log_file.write_text(
            "\n".join(
                [
                    "# 文档写作日志",
                    "",
                    f"- created_at: {now}",
                    "- source: pending",
                    "- assumptions: pending",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    print(f"Created ip-docs case: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
