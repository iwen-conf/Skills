#!/usr/bin/env python3
"""Scaffold an arc:ip-audit case directory."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path


SUBDIRS = [
    "context",
    "analysis",
    "reports",
    "handoff",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create ip-audit workspace")
    parser.add_argument("--project-path", required=True, help="Absolute path of target project")
    parser.add_argument("--project-name", help="Project name override")
    parser.add_argument("--output-dir", help="Case output directory override")
    return parser.parse_args()


def infer_project_name(project_path: Path, project_name: str | None) -> str:
    if project_name:
        return project_name.strip()
    return project_path.name


def build_case_dir(project_path: Path, project_name: str, output_dir: str | None) -> Path:
    if output_dir:
        return Path(output_dir).expanduser().resolve()
    return project_path / ".arc" / "ip-audit" / project_name


def main() -> int:
    args = parse_args()
    project_path = Path(args.project_path).expanduser().resolve()
    project_name = infer_project_name(project_path, args.project_name)
    case_dir = build_case_dir(project_path, project_name, args.output_dir)

    for sub in SUBDIRS:
        (case_dir / sub).mkdir(parents=True, exist_ok=True)

    now = dt.datetime.now(dt.timezone.utc).isoformat()

    snapshot = case_dir / "context" / "project-ip-snapshot.md"
    if not snapshot.exists():
        snapshot.write_text(
            "\n".join(
                [
                    f"# Project IP Snapshot: {project_name}",
                    "",
                    f"- generated_at: {now}",
                    f"- project_path: {project_path}",
                    "- context_source: pending",
                    "- notes: pending",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    assets = case_dir / "analysis" / "ip-assets.md"
    if not assets.exists():
        assets.write_text(
            "\n".join(
                [
                    "# IP Assets",
                    "",
                    "| Asset ID | Type | Evidence Path | Copyright Feasibility | Patent Feasibility | Risk |",
                    "|---|---|---|---|---|---|",
                    "| IPA-001 | pending | pending | pending | pending | pending |",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    handoff = case_dir / "handoff" / "ip-drafting-input.json"
    if not handoff.exists():
        handoff.write_text(
            json.dumps(
                {
                    "project_name": project_name,
                    "project_path": str(project_path),
                    "software_name": "待命名软件",
                    "software_version": "V1.0",
                    "applicant_type": "enterprise",
                    "copyright_score": 0,
                    "patent_score": 0,
                    "recommended_strategy": "pending",
                    "target_assets": [],
                    "context_sources": [],
                    "generated_at": now,
                },
                ensure_ascii=False,
                indent=2,
            )
            + "\n",
            encoding="utf-8",
        )

    print(f"Created ip-audit case: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
