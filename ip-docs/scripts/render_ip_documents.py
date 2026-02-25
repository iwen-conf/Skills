#!/usr/bin/env python3
"""Render ip-docs drafting artifacts from handoff and templates."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Render ip-docs draft files")
    parser.add_argument("--case-dir", required=True)
    parser.add_argument("--handoff-json", help="Path to ip-audit handoff json")
    parser.add_argument("--software-name")
    parser.add_argument("--software-version", default="V1.0")
    parser.add_argument("--applicant-name", default="待补充申请主体")
    parser.add_argument("--target-docs", choices=["copyright", "patent", "both"], default="both")
    return parser.parse_args()


def load_template(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def render(template: str, values: dict[str, str]) -> str:
    out = template
    for key, value in values.items():
        out = out.replace(f"{{{{{key}}}}}", value)
    return out


def load_handoff(path: Path | None) -> dict:
    if not path or not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content.rstrip() + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    case_dir = Path(args.case_dir).expanduser().resolve()
    handoff_data = load_handoff(Path(args.handoff_json).expanduser().resolve() if args.handoff_json else None)

    base_dir = Path(__file__).resolve().parent.parent / "templates"
    now = dt.datetime.now(dt.timezone.utc).isoformat()

    project_name = handoff_data.get("project_name", case_dir.name)
    software_name = args.software_name or handoff_data.get("software_name", "待命名软件")
    applicant_name = args.applicant_name
    invention_name = f"一种{software_name}的数据处理方法、系统及计算机程序产品"

    common_values = {
        "project_name": project_name,
        "software_name": software_name,
        "software_version": args.software_version,
        "applicant_name": applicant_name,
        "generated_at": now,
        "software_overview": handoff_data.get("recommended_strategy", "根据审查报告补充。"),
        "core_features": "- 待根据项目模块补充\n- 待根据业务流程补充",
        "technical_overview": "- 待结合代码证据补充",
        "invention_name": invention_name,
        "tech_field": "本发明涉及计算机软件与数据处理技术领域。",
        "background": "现有技术在性能、稳定性或可扩展性方面存在不足。",
        "objective": "提出一种可复现的软件技术方案以提升系统能力。",
        "technical_solution": "S101 获取输入数据；S102 执行核心处理；S103 输出处理结果。",
        "embodiments": "实施例1与实施例2可基于不同负载场景展开。",
        "benefits": "在吞吐、延迟或资源占用方面具有可量化改善。",
        "claim_subject": "数据处理",
        "step_1": "获取待处理数据",
        "step_2": "执行规则/模型计算",
        "step_3": "输出处理结果",
        "dependent_claim_1": "S102 采用并发调度策略",
        "dependent_claim_2": "S103 包含结果校验机制",
        "figure_1": "描述系统模块组成及连接关系。",
        "figure_2": "描述方法步骤序列与分支。",
        "figure_3": "描述模块间数据流转关系。",
    }

    if args.target_docs in {"copyright", "both"}:
        write(
            case_dir / "copyright" / "software-summary.md",
            render(load_template(base_dir / "software-summary.md.tpl"), common_values),
        )
        write(
            case_dir / "copyright" / "manual-outline.md",
            render(load_template(base_dir / "manual-outline.md.tpl"), common_values),
        )
        write(
            case_dir / "copyright" / "source-code-package-notes.md",
            "\n".join(
                [
                    "# 源代码材料打包说明（草稿）",
                    "",
                    f"- 软件名称：{software_name}",
                    "- 代码页策略：前30页 + 后30页（或不足60页提交全部）",
                    "- 页眉：软件名称 + 版本 + 页码",
                    "- 页脚：申请主体名称",
                    "- 待补充：代码样本文件清单",
                    "",
                ]
            ),
        )

    if args.target_docs in {"patent", "both"}:
        write(
            case_dir / "patent" / "disclosure-draft.md",
            render(load_template(base_dir / "patent-disclosure-draft.md.tpl"), common_values),
        )
        write(
            case_dir / "patent" / "claims-draft.md",
            render(load_template(base_dir / "patent-claims-draft.md.tpl"), common_values),
        )
        write(
            case_dir / "patent" / "drawings-description.md",
            render(load_template(base_dir / "patent-drawings-description.md.tpl"), common_values),
        )

    write(
        case_dir / "reports" / "doc-writing-log.md",
        "\n".join(
            [
                "# 文档写作日志",
                "",
                f"- generated_at: {now}",
                f"- project_name: {project_name}",
                f"- software_name: {software_name}",
                f"- target_docs: {args.target_docs}",
                f"- handoff_source: {args.handoff_json or 'none'}",
                "- assumptions: 若 handoff 缺失，使用默认占位文本并需人工补全。",
                "",
            ]
        ),
    )

    print(f"Rendered ip-docs drafts in: {case_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
