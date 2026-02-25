#!/usr/bin/env python3
"""Scaffold an arc:implement case directory."""

from __future__ import annotations

import argparse
import datetime as dt
from pathlib import Path


SUBDIRS = ["context", "plan", "execution", "reports", "handoff"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create arc:implement workspace")
    parser.add_argument("--project-path", required=True, help="Absolute path of target project")
    parser.add_argument("--task-name", required=True, help="Implementation task name")
    parser.add_argument("--output-dir", help="Case output directory override")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    project_path = Path(args.project_path).expanduser().resolve()
    case_dir = (
        Path(args.output_dir).expanduser().resolve()
        if args.output_dir
        else project_path / ".arc" / "implement" / args.task_name
    )

    for subdir in SUBDIRS:
        (case_dir / subdir).mkdir(parents=True, exist_ok=True)

    now = dt.datetime.now(dt.timezone.utc).isoformat()

    brief = case_dir / "context" / "implementation-brief.md"
    if not brief.exists():
        brief.write_text(
            "\n".join(
                [
                    f"# Implementation Brief: {args.task_name}",
                    "",
                    f"- generated_at: {now}",
                    f"- project_path: {project_path}",
                    "- input_source: pending",
                    "- notes: pending",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    plan = case_dir / "plan" / "implementation-plan.md"
    if not plan.exists():
        plan.write_text(
            "\n".join(
                [
                    "# Implementation Plan",
                    "",
                    f"- task_name: {args.task_name}",
                    "- goal: pending",
                    "- scope: pending",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    log_file = case_dir / "execution" / "execution-log.md"
    if not log_file.exists():
        log_file.write_text(
            "\n".join(
                [
                    "# Execution Log",
                    "",
                    f"- task_name: {args.task_name}",
                    f"- created_at: {now}",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    print(f"Created implement case: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
