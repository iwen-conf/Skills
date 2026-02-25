#!/usr/bin/env python3
"""Render arc:implement delivery documents from templates."""

from __future__ import annotations

import argparse
import datetime as dt
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render arc:implement output files")
    parser.add_argument("--case-dir", required=True, help="Path to .arc/implement/<task>")
    parser.add_argument("--task-name", required=True)
    parser.add_argument("--result", choices=["pass", "fail"], default="pass")
    parser.add_argument("--summary", default="实现完成，详见变更记录。")
    parser.add_argument("--verification", default="待补充验证命令与结果。")
    parser.add_argument("--affected-areas", default="待补充受影响模块。")
    parser.add_argument("--risks", default="待补充剩余风险。")
    return parser.parse_args()


def load_template(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def render(template: str, values: dict[str, str]) -> str:
    output = template
    for key, value in values.items():
        output = output.replace(f"{{{{{key}}}}}", value)
    return output


def main() -> int:
    args = parse_args()
    case_dir = Path(args.case_dir).expanduser().resolve()
    base_tpl = Path(__file__).resolve().parent.parent / "templates"

    reports_dir = case_dir / "reports"
    handoff_dir = case_dir / "handoff"
    reports_dir.mkdir(parents=True, exist_ok=True)
    handoff_dir.mkdir(parents=True, exist_ok=True)

    now = dt.datetime.now(dt.timezone.utc).isoformat()

    values = {
        "task_name": args.task_name,
        "generated_at": now,
        "result": args.result,
        "summary": args.summary,
        "verification": args.verification,
        "affected_areas": args.affected_areas,
        "risks": args.risks,
        "what_changed": "- 待补充关键改动文件\n- 待补充核心实现点",
        "why": "- 对齐实现目标与范围",
        "validation": f"- {args.verification}",
        "next_steps": "1. 执行 arc:review\n2. 执行 arc:simulate（如涉及UI流程）",
    }

    exec_report_tpl = load_template(base_tpl / "execution-report.md.tpl")
    change_summary_tpl = load_template(base_tpl / "change-summary.md.tpl")

    (reports_dir / "execution-report.md").write_text(
        render(exec_report_tpl, values).rstrip() + "\n",
        encoding="utf-8",
    )
    (handoff_dir / "change-summary.md").write_text(
        render(change_summary_tpl, values).rstrip() + "\n",
        encoding="utf-8",
    )

    print(f"Rendered reports in: {reports_dir}")
    print(f"Rendered handoff in: {handoff_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
