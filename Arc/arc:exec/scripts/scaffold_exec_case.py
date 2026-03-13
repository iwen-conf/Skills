#!/usr/bin/env python3
"""Scaffold an arc:exec orchestration case workspace."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path


SUBDIRS = [
    "context",
    "routing",
    "preview",
    "dispatch",
    "aggregation",
    "snapshots",
    "rollback",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create arc:exec workspace")
    parser.add_argument("--workdir", required=True, help="Absolute path of target workdir")
    parser.add_argument("--task-name", required=True, help="Orchestration case name")
    parser.add_argument("--output-dir", help="Case output directory override")
    return parser.parse_args()


def write_if_missing(path: Path, content: str) -> None:
    if not path.exists():
        path.write_text(content.rstrip() + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    workdir = Path(args.workdir).expanduser().resolve()
    case_dir = (
        Path(args.output_dir).expanduser().resolve()
        if args.output_dir
        else workdir / ".arc" / "exec" / args.task_name
    )

    for subdir in SUBDIRS:
        (case_dir / subdir).mkdir(parents=True, exist_ok=True)

    now = dt.datetime.now(dt.timezone.utc).isoformat()

    manifest_path = case_dir / "manifest.json"
    if not manifest_path.exists():
        manifest = {
            "task_name": args.task_name,
            "created_at": now,
            "workdir": str(workdir),
            "owner_skill": "arc:exec",
            "status": "initialized",
        }
        manifest_path.write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )

    write_if_missing(
        case_dir / "context" / "task-brief.md",
        "\n".join(
            [
                f"# Task Brief: {args.task_name}",
                "",
                f"- generated_at: {now}",
                f"- workdir: {workdir}",
                "- task_description: pending",
                "- constraints: pending",
            ]
        ),
    )
    write_if_missing(
        case_dir / "routing" / "dispatch-log.md",
        "\n".join(
            [
                "# Dispatch Log",
                "",
                f"- task_name: {args.task_name}",
                f"- created_at: {now}",
                "- selected_path: pending",
                "- reason: pending",
            ]
        ),
    )
    write_if_missing(
        case_dir / "preview" / "execution-preview.md",
        "\n".join(
            [
                "# Execution Preview",
                "",
                f"- task_name: {args.task_name}",
                "- planned_actions:",
                "  - pending",
                "- estimated_risk: pending",
            ]
        ),
    )
    write_if_missing(
        case_dir / "dispatch" / "task-board.md",
        "\n".join(
            [
                "# Task Board",
                "",
                "| Wave | capability_profile | capabilities | status | output |",
                "|------|--------------------|--------------|--------|--------|",
                "| 1 | pending | pending | pending | pending |",
            ]
        ),
    )
    write_if_missing(
        case_dir / "aggregation" / "final-summary.md",
        "\n".join(
            [
                "# Final Summary",
                "",
                f"- task_name: {args.task_name}",
                "- result: pending",
                "- conflicts: pending",
                "- next_steps: pending",
            ]
        ),
    )
    write_if_missing(
        case_dir / "rollback" / "restore-notes.md",
        "\n".join(
            [
                "# Restore Notes",
                "",
                "- snapshot_path: pending",
                "- rollback_command: pending",
                "- trigger_condition: pending",
            ]
        ),
    )

    print(f"Created exec case: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
