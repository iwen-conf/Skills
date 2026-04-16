#!/usr/bin/env python3
"""
review_uml_pack.py - 对 UML 交付目录执行多轮复核

用法:
    python3 review_uml_pack.py --uml-dir <pack-dir>
    python3 review_uml_pack.py --uml-dir <pack-dir> --strict-warnings
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import List


ROOT_DIR = Path(__file__).resolve().parents[3]
VALIDATE_SCRIPT = ROOT_DIR / "Arc" / "arc:uml" / "scripts" / "validate_diagram.py"
RENDER_SEQUENCE_SCRIPT = ROOT_DIR / "Arc" / "arc:uml" / "scripts" / "render_beautiful_mermaid_svg.mjs"
GENERATE_SEQUENCE_SCRIPT = ROOT_DIR / "Arc" / "arc:uml" / "scripts" / "generate_sequence_drawio.py"
DRAFT_DEPLOYMENT_SCRIPT = ROOT_DIR / "Arc" / "arc:uml" / "scripts" / "draft_deployment_spec.py"
GENERATE_DEPLOYMENT_SCRIPT = ROOT_DIR / "Arc" / "arc:uml" / "scripts" / "generate_deployment_drawio.py"


@dataclass
class ReviewIssue:
    severity: str
    stage: str
    message: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="对 UML 交付目录执行多轮复核")
    parser.add_argument("--uml-dir", required=True, help="UML 交付目录，如 .arc/uml/demo")
    parser.add_argument("--summary", help="输出摘要文件，默认 <uml-dir>/validation-summary.md")
    parser.add_argument("--strict-warnings", action="store_true", help="将 warning 也视为失败")
    return parser.parse_args()


def run_command(command: List[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, capture_output=True, text=True, check=False)


def normalize_text(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").strip() + "\n"


def validate_file(path: Path, diagram_type: str) -> subprocess.CompletedProcess[str]:
    return run_command([sys.executable, str(VALIDATE_SCRIPT), str(path), "--type", diagram_type])


def diagram_type_for(path: Path) -> str | None:
    name = path.name.lower()
    if "sequence" in name or path.suffix.lower() in {".mmd", ".mermaid"}:
        return "sequence"
    if "deployment" in name:
        return "deployment"
    if "activity" in name:
        return "activity"
    if "use-case" in name:
        return "use-case"
    if "state-machine" in name:
        return "state-machine"
    if "component" in name:
        return "component"
    if "class" in name:
        return "class"
    return None


def add_result_issues(issues: List[ReviewIssue], stage: str, result: subprocess.CompletedProcess[str]) -> None:
    output = (result.stdout or "") + ("\n" + result.stderr if result.stderr else "")
    lowered = output.lower()
    if result.returncode != 0:
        issues.append(ReviewIssue("error", stage, summarize_output(output)))
        return
    if "[warning]" in lowered or "⚠️" in output:
        issues.append(ReviewIssue("warning", stage, summarize_output(output)))


def summarize_output(output: str) -> str:
    lines = [line.strip() for line in output.splitlines() if line.strip()]
    if not lines:
        return "命令未返回有效输出"
    selected = lines[:8]
    return " | ".join(selected)


def compare_artifacts(issues: List[ReviewIssue], stage: str, expected: Path, actual: Path, label: str) -> None:
    if not actual.exists():
        issues.append(ReviewIssue("warning", stage, f"{label} 缺失：{actual}"))
        return
    if normalize_text(expected) != normalize_text(actual):
        issues.append(ReviewIssue("warning", stage, f"{label} 与复核生成结果不一致：{actual}"))


def extract_mermaid_source(content: str) -> str:
    stripped = content.strip()
    if stripped.startswith("sequenceDiagram"):
        return stripped + "\n"

    lines = content.splitlines()
    capturing = False
    captured: list[str] = []

    for line in lines:
        trimmed = line.strip()
        if trimmed.startswith("sequenceDiagram"):
            capturing = True
        if capturing:
            if trimmed.startswith("```") and captured:
                break
            if not trimmed.startswith("```"):
                captured.append(line.rstrip())

    source = "\n".join(captured).strip()
    if source.startswith("sequenceDiagram"):
        return source + "\n"
    return stripped + ("\n" if stripped else "")


def review_existing_diagrams(issues: List[ReviewIssue], diagrams_dir: Path) -> None:
    for path in sorted(diagrams_dir.glob("*")):
        if path.suffix.lower() not in {".drawio", ".mmd", ".mermaid"}:
            continue
        diagram_type = diagram_type_for(path)
        if diagram_type is None:
            continue
        result = validate_file(path, diagram_type)
        add_result_issues(issues, f"source:{path.name}", result)


def review_sequence(issues: List[ReviewIssue], pack_dir: Path) -> None:
    diagrams_dir = pack_dir / "diagrams"
    source_path = diagrams_dir / "sequence.mmd"
    if not source_path.exists():
        return

    with tempfile.TemporaryDirectory(prefix="uml-sequence-review-") as temp_dir:
        temp_dir_path = Path(temp_dir)
        temp_mmd = temp_dir_path / "sequence.mmd"
        temp_svg = temp_dir_path / "sequence.mmd.svg"
        temp_drawio = temp_dir_path / "sequence.drawio"
        temp_mmd.write_text(extract_mermaid_source(source_path.read_text(encoding="utf-8")), encoding="utf-8")

        render_result = run_command(
            ["node", str(RENDER_SEQUENCE_SCRIPT), "--input", str(temp_mmd), "--output", str(temp_svg)]
        )
        if render_result.returncode != 0:
            issues.append(ReviewIssue("error", "derive:sequence-svg", summarize_output(render_result.stdout + render_result.stderr)))
            return

        drawio_result = run_command(
            [
                sys.executable,
                str(GENERATE_SEQUENCE_SCRIPT),
                "--input",
                str(temp_mmd),
                "--svg",
                str(temp_svg),
                "--output",
                str(temp_drawio),
            ]
        )
        if drawio_result.returncode != 0:
            issues.append(ReviewIssue("error", "derive:sequence-drawio", summarize_output(drawio_result.stdout + drawio_result.stderr)))
            return

        add_result_issues(issues, "derived:sequence-drawio", validate_file(temp_drawio, "sequence"))
        compare_artifacts(issues, "sync:sequence-drawio", temp_drawio, diagrams_dir / "sequence.drawio", "时序图 drawio")
        compare_artifacts(issues, "sync:sequence-svg", temp_svg, diagrams_dir / "sequence.mmd.svg", "时序图 SVG")


def review_deployment(issues: List[ReviewIssue], pack_dir: Path) -> None:
    specs_dir = pack_dir / "diagram-specs"
    diagrams_dir = pack_dir / "diagrams"
    seed_path = specs_dir / "deployment.seed.txt"
    json_path = specs_dir / "deployment.json"

    with tempfile.TemporaryDirectory(prefix="uml-deployment-review-") as temp_dir:
        temp_dir_path = Path(temp_dir)
        temp_json = temp_dir_path / "deployment.json"
        temp_drawio = temp_dir_path / "deployment.drawio"

        if seed_path.exists():
            seed_result = run_command(
                [
                    sys.executable,
                    str(DRAFT_DEPLOYMENT_SCRIPT),
                    "--input",
                    str(seed_path),
                    "--output",
                    str(temp_json),
                ]
            )
            if seed_result.returncode != 0:
                issues.append(ReviewIssue("error", "derive:deployment-json", summarize_output(seed_result.stdout + seed_result.stderr)))
                return

            compare_artifacts(issues, "sync:deployment-json", temp_json, json_path, "部署图 JSON 规格")
        elif not json_path.exists():
            return

        source_json = temp_json if temp_json.exists() else json_path
        generate_result = run_command(
            [
                sys.executable,
                str(GENERATE_DEPLOYMENT_SCRIPT),
                "--spec",
                str(source_json),
                "--output",
                str(temp_drawio),
            ]
        )
        if generate_result.returncode != 0:
            issues.append(ReviewIssue("error", "derive:deployment-drawio", summarize_output(generate_result.stdout + generate_result.stderr)))
            return

        add_result_issues(issues, "derived:deployment-drawio", validate_file(temp_drawio, "deployment"))
        compare_artifacts(issues, "sync:deployment-drawio", temp_drawio, diagrams_dir / "deployment.drawio", "部署图 drawio")


def write_summary(path: Path, issues: List[ReviewIssue]) -> None:
    lines = [
        "# 复核摘要",
        "",
        "## 检查阶段",
        "",
        "- Pass 1: 直接校验现有源文件和交付文件",
        "- Pass 2: 在临时目录重新派生成品",
        "- Pass 3: 对派生成品再次校验，并与工作区产物做一致性比对",
        "",
        "## 结果",
        "",
    ]

    if not issues:
        lines.append("- 所有复核通过，没有发现错误或警告。")
    else:
        for issue in issues:
            lines.append(f"- [{issue.severity.upper()}] {issue.stage}: {issue.message}")

    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    pack_dir = Path(args.uml_dir).resolve()
    diagrams_dir = pack_dir / "diagrams"

    if not pack_dir.exists():
        raise SystemExit(f"UML 目录不存在: {pack_dir}")
    if not diagrams_dir.exists():
        raise SystemExit(f"缺少 diagrams 目录: {diagrams_dir}")

    issues: List[ReviewIssue] = []
    review_existing_diagrams(issues, diagrams_dir)
    review_sequence(issues, pack_dir)
    review_deployment(issues, pack_dir)

    summary_path = Path(args.summary).resolve() if args.summary else pack_dir / "validation-summary.md"
    write_summary(summary_path, issues)

    error_count = sum(1 for issue in issues if issue.severity == "error")
    warning_count = sum(1 for issue in issues if issue.severity == "warning")

    print(f"✓ 已完成 UML 多轮复核: {pack_dir}")
    print(f"  - errors: {error_count}")
    print(f"  - warnings: {warning_count}")
    print(f"  - summary: {summary_path}")

    if error_count > 0:
        return 1
    if args.strict_warnings and warning_count > 0:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
