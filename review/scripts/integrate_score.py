#!/usr/bin/env python3
"""
integrate_score.py - 将 arc:score 量化数据集成到 arc:review

用法:
    python integrate_score.py --score-dir .arc/score/<project> --review-dir .arc/review/<project>
"""

import argparse
import json
from datetime import datetime
from pathlib import Path


def load_score_data(score_dir: str) -> dict:
    """加载 arc:score 产出数据"""
    score_path = Path(score_dir)

    data = {"overall_score": None, "smell_report": None, "bugfix_report": None}

    # 加载综合评分
    overall_path = score_path / "score" / "overall-score.json"
    if overall_path.exists():
        data["overall_score"] = json.loads(overall_path.read_text(encoding="utf-8"))

    # 加载 Code Smell 报告
    smell_path = score_path / "analysis" / "smell-report.json"
    if smell_path.exists():
        data["smell_report"] = json.loads(smell_path.read_text(encoding="utf-8"))

    # 加载 Bugfix 分级报告
    bugfix_path = score_path / "analysis" / "bugfix-grades.json"
    if bugfix_path.exists():
        data["bugfix_report"] = json.loads(bugfix_path.read_text(encoding="utf-8"))

    return data


def generate_review_input(score_data: dict, project_path: str) -> dict:
    """
    生成供 arc:review 消费的输入数据

    将量化数据映射到七维度框架
    """
    # 维度映射：Code Smell 类别 -> arc:review 维度
    category_to_dimension = {
        "duplication": "code-quality",
        "noise": "code-quality",
        "defensive": "code-quality",
        "error_handling": "code-quality",
        "security": "security",
    }

    # 严重程度转扣分
    severity_penalty = {"critical": 20, "high": 10, "medium": 5, "low": 2}

    # 初始化维度数据
    dimension_data = {
        "architecture": {"score": 75, "issues": [], "penalty": 0},
        "security": {"score": 100, "issues": [], "penalty": 0},
        "code-quality": {"score": 100, "issues": [], "penalty": 0},
        "business": {"score": 75, "issues": [], "penalty": 0},
        "devops": {"score": 75, "issues": [], "penalty": 0},
        "team": {"score": 75, "issues": [], "penalty": 0},
        "tech-debt": {"score": 75, "issues": [], "penalty": 0},
    }

    # 处理 Code Smell 数据
    if score_data.get("smell_report"):
        smell = score_data["smell_report"]

        for violation in smell.get("violations", []):
            category = violation.get("category", "other")
            dimension = category_to_dimension.get(category, "code-quality")
            severity = violation.get("severity", "medium")
            penalty = severity_penalty.get(severity, 5)

            dimension_data[dimension]["penalty"] += penalty
            dimension_data[dimension]["issues"].append(
                {
                    "id": violation.get("id"),
                    "severity": severity,
                    "file": violation.get("file"),
                    "line": violation.get("line"),
                    "message": violation.get("message"),
                }
            )

    # 计算维度分数
    for dim, data in dimension_data.items():
        if data["penalty"] > 0:
            data["score"] = max(0, 100 - data["penalty"])

    # 处理 Bugfix 分级数据
    bugfix_summary = None
    if score_data.get("bugfix_report"):
        bugfix = score_data["bugfix_report"]
        bugfix_summary = {
            "total_commits": bugfix["summary"]["total_commits"],
            "by_grade": bugfix["summary"]["by_grade"],
            "by_tag": bugfix["summary"]["by_tag"],
            "total_lines_changed": bugfix["summary"]["total_lines_changed"],
        }

        # 根据历史修复推断技术债务
        if bugfix["summary"]["by_grade"].get("C", 0) > 10:
            dimension_data["tech-debt"]["score"] = min(
                dimension_data["tech-debt"]["score"], 60
            )
            dimension_data["tech-debt"]["issues"].append(
                {
                    "id": "high_complexity_fixes",
                    "message": f"存在 {bugfix['summary']['by_grade'].get('C', 0)} 个大型修复(C级)，表明较高技术债务",
                }
            )

    # 构建输出
    return {
        "project_path": project_path,
        "scan_timestamp": datetime.now().isoformat(),
        "quantitative_score": score_data.get("overall_score", {}).get("score", 75),
        "quantitative_grade": score_data.get("overall_score", {}).get("grade", "C"),
        "dimension_scores": {
            dim: data["score"] for dim, data in dimension_data.items()
        },
        "dimension_issues": {
            dim: data["issues"] for dim, data in dimension_data.items()
        },
        "smell_summary": {
            "total_violations": score_data.get("smell_report", {})
            .get("summary", {})
            .get("total_violations", 0),
            "by_severity": score_data.get("smell_report", {})
            .get("summary", {})
            .get("by_severity", {}),
            "by_category": score_data.get("smell_report", {})
            .get("summary", {})
            .get("by_category", {}),
        },
        "bugfix_summary": bugfix_summary,
        "integration_version": "1.0.0",
    }


def generate_quantitative_section(review_input: dict) -> str:
    """生成量化数据章节（可插入评审报告）"""

    content = """## 量化评分数据（来自 arc:score）

### 综合评分

| 指标 | 值 |
|------|-----|
| **量化分数** | {quantitative_score} / 100 |
| **量化评级** | {quantitative_grade} |

### 维度量化评分

| 维度 | 量化分数 | 问题数 |
|------|---------|--------|
""".format(
        quantitative_score=review_input.get("quantitative_score", 75),
        quantitative_grade=review_input.get("quantitative_grade", "C"),
    )

    for dim, score in review_input.get("dimension_scores", {}).items():
        issues_count = len(review_input.get("dimension_issues", {}).get(dim, []))
        content += f"| {dim} | {score} | {issues_count} |\n"

    content += """
### Code Smell 概览

| 类别 | 数量 |
|------|------|
"""

    smell = review_input.get("smell_summary", {})
    for cat, count in smell.get("by_category", {}).items():
        content += f"| {cat} | {count} |\n"

    if review_input.get("bugfix_summary"):
        bugfix = review_input["bugfix_summary"]
        content += f"""
### Bugfix 历史分析

| 指标 | 值 |
|------|-----|
| 分析提交数 | {bugfix.get("total_commits", 0)} |
| 总改动行数 | {bugfix.get("total_lines_changed", 0)} |

**分级分布：**

| 等级 | 数量 |
|------|------|
"""
        for grade, count in bugfix.get("by_grade", {}).items():
            content += f"| {grade} | {count} |\n"

    return content


def main():
    parser = argparse.ArgumentParser(description="集成 arc:score 数据到 arc:review")
    parser.add_argument("--score-dir", required=True, help="arc:score 输出目录")
    parser.add_argument("--review-dir", required=True, help="arc:review 工作目录")
    parser.add_argument("--project-path", help="项目路径（可选，用于元数据）")

    args = parser.parse_args()

    # 加载评分数据
    score_data = load_score_data(args.score_dir)

    # 生成评审输入
    project_path = args.project_path or args.score_dir
    review_input = generate_review_input(score_data, project_path)

    # 输出到 review 目录
    review_path = Path(args.review_dir)
    review_path.mkdir(parents=True, exist_ok=True)

    # 写入 JSON
    output_path = review_path / "quantitative-input.json"
    output_path.write_text(
        json.dumps(review_input, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    # 写入 Markdown 章节
    md_path = review_path / "quantitative-section.md"
    md_content = generate_quantitative_section(review_input)
    md_path.write_text(md_content, encoding="utf-8")

    print(f"✓ 量化数据已集成")
    print(f"  JSON: {output_path}")
    print(f"  Markdown: {md_path}")

    # 打印摘要
    print(f"\n评分摘要:")
    print(
        f"  综合评分: {review_input['quantitative_score']} ({review_input['quantitative_grade']})"
    )
    print(f"  维度评分:")
    for dim, score in review_input.get("dimension_scores", {}).items():
        issues = len(review_input.get("dimension_issues", {}).get(dim, []))
        print(f"    - {dim}: {score} ({issues} issues)")


if __name__ == "__main__":
    main()
