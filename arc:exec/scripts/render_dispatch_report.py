#!/usr/bin/env python3
"""Render arc:exec dispatch artifacts from CLI arguments."""

from __future__ import annotations

import argparse
import datetime as dt
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render arc:exec dispatch artifacts")
    parser.add_argument("--case-dir", required=True, help="Path to .arc/exec/<task-name>")
    parser.add_argument("--task-name", required=True)
    parser.add_argument("--route", default="direct-dispatch", help="Routing result")
    parser.add_argument("--route-reason", default="待补充", help="Why this route was selected")
    parser.add_argument("--risk", default="medium", help="Risk level summary")
    parser.add_argument(
        "--dispatch",
        action="append",
        default=[],
        help='Dispatch row in format "profile|capabilities|description|status|output"',
    )
    parser.add_argument(
        "--next-step",
        action="append",
        default=[],
        help="Next action item, can be repeated",
    )
    return parser.parse_args()


def parse_dispatch_row(raw: str) -> dict[str, str]:
    parts = [part.strip() for part in raw.split("|")]
    while len(parts) < 5:
        parts.append("")
    profile, capabilities, description, status, output = parts[:5]
    return {
        "profile": profile or "pending",
        "capabilities": capabilities or "[]",
        "description": description or "pending",
        "status": status or "pending",
        "output": output or "pending",
    }


def render_dispatch_table(rows: list[dict[str, str]]) -> str:
    lines = [
        "| Wave | capability_profile | capabilities | description | status | output |",
        "|------|--------------------|--------------|-------------|--------|--------|",
    ]
    for index, row in enumerate(rows, start=1):
        lines.append(
            "| {wave} | {profile} | {capabilities} | {description} | {status} | {output} |".format(
                wave=index,
                **row,
            )
        )
    return "\n".join(lines)


def main() -> int:
    args = parse_args()
    case_dir = Path(args.case_dir).expanduser().resolve()
    routing_dir = case_dir / "routing"
    dispatch_dir = case_dir / "dispatch"
    aggregation_dir = case_dir / "aggregation"
    routing_dir.mkdir(parents=True, exist_ok=True)
    dispatch_dir.mkdir(parents=True, exist_ok=True)
    aggregation_dir.mkdir(parents=True, exist_ok=True)

    now = dt.datetime.now(dt.timezone.utc).isoformat()
    rows = [parse_dispatch_row(raw) for raw in args.dispatch]
    if not rows:
        rows = [parse_dispatch_row("pending|[]|pending|pending|pending")]

    table = render_dispatch_table(rows)
    next_steps = args.next_step or ["补充最终交付说明", "执行回归验证"]

    routing_log = "\n".join(
        [
            "# Dispatch Log",
            "",
            f"- task_name: {args.task_name}",
            f"- generated_at: {now}",
            f"- selected_route: {args.route}",
            f"- route_reason: {args.route_reason}",
            "",
            "## Dispatch Waves",
            table,
            "",
        ]
    )
    (routing_dir / "dispatch-log.md").write_text(routing_log, encoding="utf-8")

    (dispatch_dir / "task-board.md").write_text(
        "\n".join(["# Task Board", "", table, ""]),
        encoding="utf-8",
    )

    summary_lines = [
        "# Final Summary",
        "",
        f"- task_name: {args.task_name}",
        f"- generated_at: {now}",
        f"- route: {args.route}",
        f"- risk: {args.risk}",
        f"- dispatch_count: {len(rows)}",
        "",
        "## Next Steps",
    ]
    summary_lines.extend([f"{index}. {item}" for index, item in enumerate(next_steps, start=1)])
    summary_lines.append("")
    (aggregation_dir / "final-summary.md").write_text(
        "\n".join(summary_lines),
        encoding="utf-8",
    )

    print(f"Rendered dispatch artifacts in: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
