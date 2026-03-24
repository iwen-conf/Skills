#!/usr/bin/env python3
"""
aggregate_score.py - 评分聚合算法

用法:
    python aggregate_score.py --smell-report smell.json --bugfix-report bugfix.json --output score.json
"""

import argparse
import json
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path


# 严重程度扣分权重
SEVERITY_PENALTY = {"critical": 20, "high": 10, "medium": 5, "low": 2}

SCHEMA_VERSION = "1.0.0"
PRODUCER_SKILL = "arc-score"

# 维度映射 (Code Smell 类别 -> 评审维度)
CATEGORY_TO_DIMENSION = {
    "duplication": "code_quality",
    "noise": "code_quality",
    "defensive": "code_quality",
    "error_handling": "code_quality",
    "security": "security",
}

# 维度权重
DIMENSION_WEIGHTS = {
    "architecture": 0.2,
    "security": 0.25,  # 安全权重较高
    "code_quality": 0.25,
    "business": 0.1,
    "devops": 0.1,
    "team": 0.05,
    "tech_debt": 0.05,
}


@dataclass
class Violation:
    """违规项"""

    id: str
    category: str
    severity: str
    count: int = 1
    file: str = ""
    line: int = 0
    message: str = ""


def load_smell_report(path: str) -> list[Violation]:
    """加载 Code Smell 报告"""
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)

    violations = []
    for v in data.get("violations", []):
        violations.append(
            Violation(
                id=v.get("id", "unknown"),
                category=v.get("category", "other"),
                severity=v.get("severity", "medium"),
                count=1,
                file=v.get("file", ""),
                line=v.get("line", 0),
                message=v.get("message", ""),
            )
        )

    return violations


def load_bugfix_report(path: str) -> dict:
    """加载 Bugfix 分级报告"""
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data


def adjust_dimension_scores_for_bugfix(
    dimension_scores: dict[str, float], bugfix_data: dict | None
) -> dict[str, float]:
    if not bugfix_data:
        return dimension_scores

    by_grade = bugfix_data.get("summary", {}).get("by_grade", {})
    c_raw = by_grade.get("C", 0)
    try:
        c_count = int(c_raw)
    except (TypeError, ValueError):
        c_count = 0

    if c_count <= 10:
        return dimension_scores

    adjusted = dict(dimension_scores)
    adjusted["tech_debt"] = min(adjusted.get("tech_debt", 75), 60)
    return adjusted


def aggregate_score(
    smell_violations: list[Violation],
    bugfix_data: dict | None = None,
    weights: dict | None = None,
) -> dict:
    """
    聚合评分

    Args:
        smell_violations: Code Smell 违规列表
        bugfix_data: Bugfix 分级数据
        weights: 自定义权重

    Returns:
        评分结果
    """
    if weights is None:
        weights = SEVERITY_PENALTY

    # 计算扣分
    total_penalty = 0
    by_category = {}
    by_severity = {"critical": 0, "high": 0, "medium": 0, "low": 0}
    by_dimension = {}

    for v in smell_violations:
        severity = v.severity
        count = v.count
        penalty = weights.get(severity, 5) * count
        total_penalty += penalty

        # 按类别统计
        category = v.category
        by_category[category] = by_category.get(category, 0) + penalty

        # 按严重程度统计
        by_severity[severity] += count

        # 按维度统计
        dimension = CATEGORY_TO_DIMENSION.get(category, "code_quality")
        by_dimension[dimension] = by_dimension.get(dimension, 0) + penalty

    # 计算最终分数
    final_score = max(0, 100 - total_penalty)

    # 计算各维度分数
    dimension_scores = {}
    for dim, dim_weight in DIMENSION_WEIGHTS.items():
        if dim == "security":
            # 安全维度：直接扣分
            security_penalty = by_dimension.get("security", 0)
            dimension_scores[dim] = max(0, 100 - security_penalty)
        elif dim == "code_quality":
            # 代码质量维度
            cq_penalty = by_dimension.get("code_quality", 0)
            dimension_scores[dim] = max(0, 100 - cq_penalty)
        else:
            # 其他维度：默认基准分
            dimension_scores[dim] = 75  # 默认值，需 Agent 评审修正

    dimension_scores = adjust_dimension_scores_for_bugfix(dimension_scores, bugfix_data)

    # 综合评分（加权平均）
    weighted_score = sum(
        dimension_scores.get(dim, 75) * weight
        for dim, weight in DIMENSION_WEIGHTS.items()
    )

    return {
        "schema_version": SCHEMA_VERSION,
        "producer_skill": PRODUCER_SKILL,
        "score": round(final_score, 1),
        "weighted_score": round(weighted_score, 1),
        "grade": score_to_grade(final_score),
        "total_penalty": total_penalty,
        "by_category": by_category,
        "by_severity": by_severity,
        "by_dimension": by_dimension,
        "dimension_scores": dimension_scores,
        "violation_count": sum(v.count for v in smell_violations),
        "bugfix_summary": bugfix_data.get("summary") if bugfix_data else None,
        "calculated_at": datetime.now().isoformat(),
    }


def score_to_grade(score: float) -> str:
    """分数转评级"""
    if score >= 90:
        return "A"
    elif score >= 80:
        return "B"
    elif score >= 70:
        return "C"
    elif score >= 60:
        return "D"
    else:
        return "F"


def generate_scorecard(score_data: dict) -> str:
    """生成评分卡 Markdown"""
    score = score_data["score"]
    grade = score_data["grade"]

    # 评级颜色
    grade_colors = {"A": "🟢", "B": "🔵", "C": "🟡", "D": "🟠", "F": "🔴"}

    content = f"""# 评分卡

## 综合评分

| 指标 | 值 |
|------|-----|
| **分数** | {score} / 100 |
| **评级** | {grade_colors.get(grade, "⚪")} {grade} |
| **违规数** | {score_data["violation_count"]} |
| **扣分** | {score_data["total_penalty"]} |

## 严重程度分布

| 严重程度 | 数量 |
|----------|------|
| 🔴 Critical | {score_data["by_severity"]["critical"]} |
| 🟠 High | {score_data["by_severity"]["high"]} |
| 🟡 Medium | {score_data["by_severity"]["medium"]} |
| 🟢 Low | {score_data["by_severity"]["low"]} |

## 维度评分

| 维度 | 分数 |
|------|------|
"""

    for dim, dim_score in score_data["dimension_scores"].items():
        content += f"| {dim} | {dim_score:.1f} |\n"

    content += """
## 类别分布

| 类别 | 扣分 |
|------|------|
"""

    for cat, penalty in sorted(score_data["by_category"].items(), key=lambda x: -x[1]):
        content += f"| {cat} | {penalty} |\n"

    return content


def main():
    parser = argparse.ArgumentParser(description="评分聚合")
    parser.add_argument("--smell-report", required=True, help="Code Smell 报告路径")
    parser.add_argument("--bugfix-report", help="Bugfix 分级报告路径 (可选)")
    parser.add_argument("--output", required=True, help="输出路径")
    parser.add_argument("--scorecard", help="评分卡输出路径 (可选)")
    parser.add_argument("--dimension-scores", help="维度分数输出路径 (可选)")

    args = parser.parse_args()

    # 加载数据
    violations = load_smell_report(args.smell_report)

    bugfix_data = None
    if args.bugfix_report:
        bugfix_data = load_bugfix_report(args.bugfix_report)

    # 聚合评分
    result = aggregate_score(violations, bugfix_data)

    # 输出结果
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    print(f"✓ 评分完成: {result['score']} ({result['grade']})")
    print(f"✓ 结果已保存: {output_path}")

    # 生成评分卡
    if args.scorecard:
        scorecard = generate_scorecard(result)
        scorecard_path = Path(args.scorecard)
        scorecard_path.write_text(scorecard, encoding="utf-8")
        print(f"✓ 评分卡已保存: {scorecard_path}")

    if args.dimension_scores:
        dim_path = Path(args.dimension_scores)
        dim_path.parent.mkdir(parents=True, exist_ok=True)
        dim_payload = {
            "schema_version": SCHEMA_VERSION,
            "producer_skill": PRODUCER_SKILL,
            "generated_at": datetime.now().isoformat(),
            "dimension_scores": result.get("dimension_scores", {}),
            "dimension_weights": DIMENSION_WEIGHTS,
            "weighted_score": result.get("weighted_score"),
            "grade": result.get("grade"),
        }
        dim_path.write_text(
            json.dumps(dim_payload, indent=2, ensure_ascii=False), encoding="utf-8"
        )
        print(f"✓ 维度分数已保存: {dim_path}")


if __name__ == "__main__":
    main()
