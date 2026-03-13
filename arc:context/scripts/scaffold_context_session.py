#!/usr/bin/env python3
"""Scaffold an arc:context session directory."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path
from typing import Any


SUBDIRS = ["plan", "sources", "retrieval", "findings", "context", "restore", "handoff"]
DEFAULT_CONTEXT_BUDGET = 1200


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create arc:context workspace")
    parser.add_argument("--project-path", required=True, help="Absolute path of target project")
    parser.add_argument("--task-name", required=True, help="Task name used for the context packet")
    parser.add_argument(
        "--mode",
        choices=["prime", "analyze", "snapshot", "restore"],
        default="prime",
        help="Context workflow mode",
    )
    parser.add_argument("--objective", help="Task objective or resume goal")
    parser.add_argument(
        "--entrypoint",
        action="append",
        default=[],
        help="Relevant file or artifact path; may be passed multiple times",
    )
    parser.add_argument(
        "--data-source",
        action="append",
        default=[],
        help="Large output source such as log, URL, artifact, or command result; may be passed multiple times",
    )
    parser.add_argument(
        "--context-budget",
        type=int,
        default=DEFAULT_CONTEXT_BUDGET,
        help="Approximate token budget for the compressed packet",
    )
    parser.add_argument("--output-dir", help="Case output directory override")
    return parser.parse_args()


def _iso_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")


def _to_relative(project_path: Path, value: str) -> str:
    candidate = Path(value).expanduser()
    if not candidate.is_absolute():
        return value
    resolved = candidate.resolve()
    try:
        return str(resolved.relative_to(project_path))
    except ValueError:
        return str(resolved)


def _parse_timestamp(value: str | None) -> dt.datetime | None:
    if not value:
        return None
    normalized = value.replace("Z", "+00:00")
    try:
        return dt.datetime.fromisoformat(normalized)
    except ValueError:
        return None


def _artifact_status(item: dict[str, Any], now: dt.datetime) -> str:
    expires_at = _parse_timestamp(str(item.get("expires_at", "")) or None)
    if expires_at is None:
        return "unknown"
    if expires_at < now:
        return "stale"
    return "fresh"


def _load_context_hub_artifacts(project_path: Path) -> list[dict[str, Any]]:
    index_path = project_path / ".arc" / "context-hub" / "index.json"
    if not index_path.exists():
        return []

    try:
        data = json.loads(index_path.read_text(encoding="utf-8"))
    except Exception:
        return []

    artifacts = data.get("artifacts")
    if not isinstance(artifacts, list):
        return []

    now = dt.datetime.now(dt.timezone.utc)
    normalized: list[dict[str, Any]] = []
    for item in artifacts:
        if not isinstance(item, dict):
            continue
        normalized.append(
            {
                "name": item.get("name", ""),
                "path": item.get("path", ""),
                "artifact_type": item.get("artifact_type", ""),
                "producer_skill": item.get("producer_skill", ""),
                "generated_at": item.get("generated_at", ""),
                "expires_at": item.get("expires_at", ""),
                "refresh_skill": item.get("refresh_skill", ""),
                "refresh_command_hint": item.get("refresh_command_hint", ""),
                "status": _artifact_status(item, now),
            }
        )
    return normalized


def _write_if_missing(path: Path, content: str) -> None:
    if not path.exists():
        path.write_text(content, encoding="utf-8")


def _render_context_brief(now: str, project_path: Path, task_name: str, mode: str, objective: str) -> str:
    return "\n".join(
        [
            f"# Context Brief: {task_name}",
            "",
            f"- generated_at: {now}",
            f"- project_path: {project_path}",
            f"- mode: {mode}",
            f"- objective: {objective or 'pending'}",
            "- current_status: pending",
            "- blocker_summary: pending",
            "- next_decision: pending",
            "",
        ]
    )


def _render_context_plan(mode: str, objective: str, data_sources: list[str]) -> str:
    mode_hint = {
        "prime": "Prime a bounded packet before deep work expands.",
        "analyze": "Keep large output in files or retrieval artifacts before summarizing.",
        "snapshot": "Compress the current session before interruption or handoff.",
        "restore": "Reopen only the minimum viable working set and verify freshness.",
    }[mode]
    lines = [
        "# Context Plan",
        "",
        f"- mode: {mode}",
        f"- objective: {objective or 'pending'}",
        f"- handling_path: {'file-first / retrieval-first' if mode == 'analyze' else 'bounded packet'}",
        f"- mode_hint: {mode_hint}",
        "",
        "## Data Sources",
    ]
    if data_sources:
        lines.extend(f"- `{item}`" for item in data_sources)
    else:
        lines.append("- pending")
    lines.extend(
        [
            "",
            "## Fallback",
            "- pending",
            "",
        ]
    )
    return "\n".join(lines)


def _render_working_set(entrypoints: list[str], artifacts: list[dict[str, Any]]) -> str:
    lines = [
        "# Working Set",
        "",
        "## Reopen First",
    ]
    if entrypoints:
        lines.extend(f"- `{item}`" for item in entrypoints)
    else:
        lines.append("- pending")

    lines.extend(["", "## Reusable Artifacts"])
    if artifacts:
        for item in artifacts:
            name = item.get("name", "unnamed-artifact")
            path = item.get("path", "")
            status = item.get("status", "unknown")
            lines.append(f"- `{name}` -> `{path}` ({status})")
    else:
        lines.append("- none detected in `.arc/context-hub/index.json`")

    lines.extend(
        [
            "",
            "## Open Questions",
            "- pending",
            "",
            "## Next Action",
            "- pending",
            "",
        ]
    )
    return "\n".join(lines)


def _render_restore_checklist(mode: str) -> str:
    mode_line = {
        "prime": "Capture the first bounded packet before deep implementation starts.",
        "analyze": "Persist raw artifacts first and verify the narrowed query plan before summarizing.",
        "snapshot": "Confirm the packet is sufficient for a clean handoff before the current session ends.",
        "restore": "Reopen only the minimum files and artifacts required for the next step.",
    }[mode]
    return "\n".join(
        [
            "# Restore Checklist",
            "",
            f"1. {mode_line}",
            "2. Read `restore/recovery-manifest.json` and verify freshness markers.",
            "3. Reopen `context/working-set.md` and the listed entrypoints.",
            "4. Separate observed facts from assumptions before resuming changes.",
            "5. Confirm the next action and update the packet if reality has drifted.",
            "",
        ]
    )


def _render_search_queries() -> str:
    return "\n".join(
        [
            "# Search Queries",
            "",
            "## Primary Queries",
            "- pending",
            "",
            "## Follow-up Queries",
            "- pending",
            "",
        ]
    )


def _render_compact_findings() -> str:
    return "\n".join(
        [
            "# Compact Findings",
            "",
            "## Key Signals",
            "- pending",
            "",
            "## Anchors",
            "- pending",
            "",
            "## Next Action",
            "- pending",
            "",
        ]
    )


def _render_handoff_notes(now: str, task_name: str) -> str:
    return "\n".join(
        [
            f"# Handoff Notes: {task_name}",
            "",
            f"- updated_at: {now}",
            "- observed_facts: pending",
            "- assumptions: pending",
            "- blockers: pending",
            "- next_action: pending",
            "",
        ]
    )


def main() -> int:
    args = parse_args()
    project_path = Path(args.project_path).expanduser().resolve()
    case_dir = (
        Path(args.output_dir).expanduser().resolve()
        if args.output_dir
        else project_path / ".arc" / "context" / args.task_name
    )

    for subdir in SUBDIRS:
        (case_dir / subdir).mkdir(parents=True, exist_ok=True)

    now = _iso_now()
    entrypoints = [_to_relative(project_path, value) for value in args.entrypoint]
    data_sources = [_to_relative(project_path, value) for value in args.data_source]
    artifacts = _load_context_hub_artifacts(project_path)

    manifest = {
        "schema_version": "1.0.0",
        "generated_at": now,
        "project_path": str(project_path),
        "task_name": args.task_name,
        "mode": args.mode,
        "objective": args.objective or "",
        "context_budget": args.context_budget,
        "entrypoints": entrypoints,
        "data_sources": data_sources,
        "context_hub_artifacts": artifacts,
        "assumptions": ["pending"],
        "open_questions": ["pending"],
        "next_actions": ["pending"],
    }

    _write_if_missing(
        case_dir / "context" / "context-brief.md",
        _render_context_brief(now, project_path, args.task_name, args.mode, args.objective or ""),
    )
    _write_if_missing(
        case_dir / "plan" / "context-plan.md",
        _render_context_plan(args.mode, args.objective or "", data_sources),
    )
    _write_if_missing(case_dir / "context" / "working-set.md", _render_working_set(entrypoints, artifacts))
    _write_if_missing(case_dir / "retrieval" / "search-queries.md", _render_search_queries())
    _write_if_missing(case_dir / "findings" / "compact-findings.md", _render_compact_findings())
    _write_if_missing(case_dir / "restore" / "restore-checklist.md", _render_restore_checklist(args.mode))
    _write_if_missing(case_dir / "handoff" / "handoff-notes.md", _render_handoff_notes(now, args.task_name))
    _write_if_missing(
        case_dir / "restore" / "recovery-manifest.json",
        json.dumps(manifest, indent=2, ensure_ascii=False) + "\n",
    )

    print(f"Created context case: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
