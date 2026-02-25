#!/usr/bin/env python3
"""Render arc:ip-audit report files from templates."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render ip-audit report artifacts")
    parser.add_argument("--case-dir", required=True, help="Path to .arc/ip-audit/<project>")
    parser.add_argument("--project-name", required=True)
    parser.add_argument("--software-name", default="待命名软件")
    parser.add_argument("--applicant-type", default="enterprise")
    parser.add_argument("--copyright-score", type=int, default=75)
    parser.add_argument("--patent-score", type=int, default=65)
    parser.add_argument("--risk-level", choices=["LOW", "MEDIUM", "HIGH"], default="MEDIUM")
    parser.add_argument("--strategy", default="软著先行，专利补强后推进")
    return parser.parse_args()


def load_template(template_path: Path) -> str:
    return template_path.read_text(encoding="utf-8")


def render(template: str, values: dict[str, str]) -> str:
    output = template
    for key, value in values.items():
        output = output.replace(f"{{{{{key}}}}}", value)
    return output


def infer_context_sources(snapshot_path: Path) -> str:
    if not snapshot_path.exists():
        return "- ace-tool scan (fallback)"
    lines = snapshot_path.read_text(encoding="utf-8").splitlines()
    return "\n".join([f"- {line}" for line in lines[:4] if line.strip()]) or "- pending"


def infer_action_items(copyright_score: int, patent_score: int) -> str:
    actions: list[str] = []
    if copyright_score < 80:
        actions.append("- 补齐软著说明书素材，先冻结名称与版本。")
    else:
        actions.append("- 软著材料准备度较高，可优先提交。")

    if patent_score < 70:
        actions.append("- 先补充技术效果量化指标，再进入专利撰写。")
    else:
        actions.append("- 专利可行性较高，建议同步启动技术交底书。")

    actions.append("- 完成费减资格自检并锁定时间窗口。")
    return "\n".join(actions)


def main() -> int:
    args = parse_args()
    case_dir = Path(args.case_dir).expanduser().resolve()
    reports_dir = case_dir / "reports"
    handoff_dir = case_dir / "handoff"
    reports_dir.mkdir(parents=True, exist_ok=True)
    handoff_dir.mkdir(parents=True, exist_ok=True)

    base_dir = Path(__file__).resolve().parent.parent
    tpl_report = load_template(base_dir / "templates" / "ip-feasibility-report.md.tpl")
    tpl_checklist = load_template(base_dir / "templates" / "filing-readiness-checklist.md.tpl")

    now = dt.datetime.now(dt.timezone.utc).isoformat()
    values = {
        "project_name": args.project_name,
        "software_name": args.software_name,
        "applicant_type": args.applicant_type,
        "generated_at": now,
        "copyright_score": str(args.copyright_score),
        "patent_score": str(args.patent_score),
        "recommended_strategy": args.strategy,
        "asset_summary": "- 资产清单见 analysis/ip-assets.md",
        "risk_summary": f"- 当前整体风险等级：{args.risk_level}",
        "action_items": infer_action_items(args.copyright_score, args.patent_score),
        "context_sources": infer_context_sources(case_dir / "context" / "project-ip-snapshot.md"),
    }

    (reports_dir / "ip-feasibility-report.md").write_text(
        render(tpl_report, values) + "\n",
        encoding="utf-8",
    )
    (reports_dir / "filing-readiness-checklist.md").write_text(
        render(tpl_checklist, values) + "\n",
        encoding="utf-8",
    )

    handoff = {
        "project_name": args.project_name,
        "software_name": args.software_name,
        "software_version": "V1.0",
        "applicant_type": args.applicant_type,
        "copyright_score": args.copyright_score,
        "patent_score": args.patent_score,
        "risk_level": args.risk_level,
        "recommended_strategy": args.strategy,
        "target_assets": [],
        "generated_at": now,
        "source_report": str(reports_dir / "ip-feasibility-report.md"),
    }
    (handoff_dir / "ip-drafting-input.json").write_text(
        json.dumps(handoff, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )

    print(f"Rendered ip-audit reports in: {reports_dir}")
    print(f"Updated handoff: {handoff_dir / 'ip-drafting-input.json'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
