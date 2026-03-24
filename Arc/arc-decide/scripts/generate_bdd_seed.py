#!/usr/bin/env python3
"""
generate_bdd_seed.py - 从 arc-decide 共识报告生成 BDD 场景种子

用法:
    python generate_bdd_seed.py --consensus-report consensus.md --output bdd-seed.yaml
"""

import argparse
import re
import yaml
from datetime import datetime
from pathlib import Path
from typing import Optional


def extract_scenarios_from_report(report_content: str) -> list[dict]:
    """
    从共识报告提取 BDD 场景

    寻找模式：
    1. "验证..." / "检查..." / "确保..." 等可验证陈述
    2. "当...时，应..." 条件语句
    3. 列表项中的具体要求
    """
    scenarios = []

    # 模式 1: 验证陈述
    verify_pattern = r"(?:验证|检查|确保|要求)[：:]\s*(.+?)(?:\n|$)"
    for match in re.finditer(verify_pattern, report_content):
        statement = match.group(1).strip()
        scenario = statement_to_scenario(statement)
        if scenario:
            scenarios.append(scenario)

    # 模式 2: 条件语句
    conditional_pattern = (
        r"当\s*(.+?)\s*[时时候候]，\s*(?:应|需要|必须)\s*(.+?)(?:\n|$)"
    )
    for match in re.finditer(conditional_pattern, report_content):
        condition = match.group(1).strip()
        action = match.group(2).strip()
        scenarios.append(
            {
                "scenario": f"条件验证: {condition}",
                "given": "系统处于正常状态",
                "when": condition,
                "then": action,
                "priority": "medium",
                "source": "consensus_report",
            }
        )

    # 模式 3: 列表项要求 (- [ ] 或 - 开头)
    list_pattern = r"^\s*[-*]\s*(?:\[[ x]\])?\s*(.+?)$"
    for match in re.finditer(list_pattern, report_content, re.MULTILINE):
        item = match.group(1).strip()
        # 过滤非要求项
        if len(item) > 5 and not item.startswith("#"):
            scenario = statement_to_scenario(item)
            if scenario:
                scenarios.append(scenario)

    return scenarios


def statement_to_scenario(statement: str) -> Optional[dict]:
    """将陈述转换为 BDD 场景"""

    # 关键词分类
    security_keywords = ["安全", "认证", "授权", "权限", "密码", "token", "加密"]
    architecture_keywords = ["架构", "模块", "依赖", "分层", "接口", "API"]
    quality_keywords = ["代码", "测试", "质量", "规范", "风格", "lint"]
    performance_keywords = ["性能", "响应", "超时", "并发", "缓存"]

    # 确定类别
    category = "general"
    for kw in security_keywords:
        if kw in statement.lower():
            category = "security"
            break
    else:
        for kw in architecture_keywords:
            if kw in statement.lower():
                category = "architecture"
                break
        else:
            for kw in quality_keywords:
                if kw in statement.lower():
                    category = "quality"
                    break
            else:
                for kw in performance_keywords:
                    if kw in statement.lower():
                        category = "performance"
                        break

    # 生成场景
    scenario = {
        "scenario": statement[:100],  # 截断过长描述
        "given": "系统处于正常状态",
        "when": "执行相关操作",
        "then": statement,
        "category": category,
        "priority": "medium",
        "source": "auto_generated",
    }

    return scenario


def generate_bdd_seed(consensus_path: str, output_path: str) -> dict:
    """
    生成 BDD 场景种子

    Args:
        consensus_path: 共识报告路径
        output_path: 输出路径

    Returns:
        BDD 种子数据
    """
    # 读取共识报告
    report_path = Path(consensus_path)
    if not report_path.exists():
        raise FileNotFoundError(f"共识报告不存在: {consensus_path}")

    report_content = report_path.read_text(encoding="utf-8")

    # 提取场景
    scenarios = extract_scenarios_from_report(report_content)

    # 去重
    unique_scenarios = []
    seen = set()
    for s in scenarios:
        key = s["scenario"]
        if key not in seen:
            seen.add(key)
            unique_scenarios.append(s)

    # 构建输出
    bdd_seed = {
        "version": "1.0.0",
        "generated_at": datetime.now().isoformat(),
        "source": consensus_path,
        "scenarios": unique_scenarios[:50],  # 限制最多 50 个场景
        "statistics": {"total_scenarios": len(unique_scenarios), "by_category": {}},
    }

    # 统计
    for s in unique_scenarios:
        cat = s.get("category", "general")
        bdd_seed["statistics"]["by_category"][cat] = (
            bdd_seed["statistics"]["by_category"].get(cat, 0) + 1
        )

    # 输出
    output = Path(output_path)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        yaml.dump(bdd_seed, allow_unicode=True, sort_keys=False), encoding="utf-8"
    )

    return bdd_seed


def main():
    parser = argparse.ArgumentParser(description="生成 BDD 场景种子")
    parser.add_argument("--consensus-report", required=True, help="共识报告路径")
    parser.add_argument("--output", required=True, help="输出 YAML 路径")

    args = parser.parse_args()

    result = generate_bdd_seed(args.consensus_report, args.output)

    print(f"✓ 生成 {len(result['scenarios'])} 个 BDD 场景")
    print(f"✓ 输出: {args.output}")

    # 打印分类统计
    print("\n场景分类:")
    for cat, count in result["statistics"]["by_category"].items():
        print(f"  - {cat}: {count}")


if __name__ == "__main__":
    main()
