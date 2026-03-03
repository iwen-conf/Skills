#!/usr/bin/env python3
"""
grade_bugfix.py - Bugfix 分级脚本

用法:
    python grade_bugfix.py --project /path/to/project --output bugfix.json
"""

import argparse
import json
import re
import subprocess
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
from typing import Optional


@dataclass
class BugfixCommit:
    """Bugfix 提交"""

    hash: str
    short_hash: str
    message: str
    author: str
    date: str
    files_changed: int
    lines_added: int
    lines_deleted: int
    lines_total: int
    grade: str
    tags: list[str]
    complexity_score: float


SCHEMA_VERSION = "1.0.0"
PRODUCER_SKILL = "arc:score"


# 分级阈值
GRADE_THRESHOLDS = {
    "A": {"max_lines": 10},
    "B": {"max_lines": 50},
    "C": {"min_lines": 51},
}

# 自动打标规则
TAG_RULES = {
    "security": [
        "xss",
        "漏洞",
        "注入",
        "安全",
        "越权",
        "认证",
        "授权",
        "加密",
        "解密",
        "敏感",
    ],
    "pagination": ["分页", "翻页", "page", "offset", "limit", "cursor"],
    "export": ["导出", "导入", "下载", "export", "import", "download"],
    "datetime": ["时间", "日期", "时区", "utc", "timezone", "date", "time"],
    "null_safety": ["空", "null", "nil", "为空", "类型", "空指针", "NPE", "undefined"],
    "transaction": [
        "事务",
        "commit",
        "回滚",
        "rollback",
        "transaction",
        "原子",
        "atomic",
    ],
    "concurrency": [
        "并发",
        "锁",
        "重复",
        "concurrent",
        "thread",
        "mutex",
        "race",
        "死锁",
        "deadlock",
    ],
    "performance": [
        "性能",
        "慢",
        "超时",
        "timeout",
        "优化",
        "optimize",
        "缓慢",
        "卡顿",
        "内存泄漏",
    ],
    "authentication": [
        "认证",
        "登录",
        "auth",
        "token",
        "jwt",
        "session",
        "cookie",
        "oauth",
    ],
    "data_integrity": [
        "数据",
        "完整性",
        "校验",
        "验证",
        "validation",
        "consistent",
        "一致性",
    ],
    "api": ["接口", "api", "endpoint", "请求", "响应", "request", "response"],
    "ui": ["界面", "显示", "样式", "UI", "前端", "展示"],
}

# Fix commit 匹配模式
FIX_PATTERNS = [
    r"^fix\([^)]+\):",
    r"^fix:",
    r"^\[fix\]",
    r"^修复",
    r"^修正",
    r"^bugfix",
]

# 排除模式
EXCLUDE_PATTERNS = [
    r"^Merge",
    r"^Revert",
    r"^chore:",
    r"^docs:",
    r"^style:",
    r"^test:",
    r"^ci:",
    r"^refactor:",
]


def run_git_command(project_path: str, args: list[str]) -> str:
    """执行 git 命令"""
    try:
        result = subprocess.run(
            ["git"] + args, cwd=project_path, capture_output=True, text=True, timeout=30
        )
        return result.stdout.strip()
    except Exception:
        return ""


def is_fix_commit(message: str) -> bool:
    """判断是否为 fix commit"""
    for pattern in FIX_PATTERNS:
        if re.search(pattern, message, re.IGNORECASE):
            return True
    return False


def should_exclude(message: str) -> bool:
    """判断是否应排除"""
    for pattern in EXCLUDE_PATTERNS:
        if re.search(pattern, message, re.IGNORECASE):
            return True
    return False


def get_grade(lines_total: int) -> str:
    """根据行数分级"""
    if lines_total <= GRADE_THRESHOLDS["A"]["max_lines"]:
        return "A"
    elif lines_total <= GRADE_THRESHOLDS["B"]["max_lines"]:
        return "B"
    else:
        return "C"


def get_tags(message: str) -> list[str]:
    """根据 commit message 提取标签"""
    tags = []
    message_lower = message.lower()

    for tag, keywords in TAG_RULES.items():
        for keyword in keywords:
            if keyword.lower() in message_lower:
                tags.append(tag)
                break

    return list(set(tags))


def calculate_complexity(
    lines_total: int, files_changed: int, tags: list[str]
) -> float:
    """计算复杂度分数"""
    base = lines_total * 0.5 + files_changed * 5

    # 安全问题复杂度加成
    if "security" in tags:
        base *= 1.5

    return round(base, 1)


def grade_bugfix(project_path: str, branch: str = "main", limit: int = 100) -> dict:
    """
    执行 Bugfix 分级

    Args:
        project_path: 项目根目录
        branch: 分支名
        limit: 最大分析 commit 数

    Returns:
        分级结果
    """
    project_root = Path(project_path)

    # 获取 commit 列表
    log_format = "%H|%h|%s|%an|%ad"
    log_output = run_git_command(
        str(project_root),
        ["log", branch, f"--format={log_format}", f"-{limit}", "--date=short"],
    )

    if not log_output:
        # 尝试其他分支
        for fallback in ["master", "develop"]:
            log_output = run_git_command(
                str(project_root),
                [
                    "log",
                    fallback,
                    f"--format={log_format}",
                    f"-{limit}",
                    "--date=short",
                ],
            )
            if log_output:
                break

    commits = []

    for line in log_output.split("\n"):
        if not line.strip():
            continue

        parts = line.split("|")
        if len(parts) < 5:
            continue

        full_hash, short_hash, message, author, date = parts

        # 过滤非 fix commit
        if not is_fix_commit(message):
            continue

        # 过滤排除的 commit
        if should_exclude(message):
            continue

        # 获取改动统计
        stat_output = run_git_command(
            str(project_root), ["show", "--stat", "--format=", full_hash]
        )

        lines_added = 0
        lines_deleted = 0
        files_changed = 0

        for stat_line in stat_output.split("\n"):
            if "insertion" in stat_line or "insert" in stat_line:
                match = re.search(r"(\d+) insertion", stat_line)
                if match:
                    lines_added = int(match.group(1))
            if "deletion" in stat_line or "delete" in stat_line:
                match = re.search(r"(\d+) deletion", stat_line)
                if match:
                    lines_deleted = int(match.group(1))

        # 统计文件数
        files_changed = len([l for l in stat_output.split("\n") if "|" in l])

        lines_total = lines_added + lines_deleted
        grade = get_grade(lines_total)
        tags = get_tags(message)
        complexity = calculate_complexity(lines_total, files_changed, tags)

        commits.append(
            BugfixCommit(
                hash=full_hash,
                short_hash=short_hash,
                message=message,
                author=author,
                date=date,
                files_changed=files_changed,
                lines_added=lines_added,
                lines_deleted=lines_deleted,
                lines_total=lines_total,
                grade=grade,
                tags=tags,
                complexity_score=complexity,
            )
        )

    # 汇总统计
    summary = {
        "total_commits": len(commits),
        "by_grade": {},
        "by_tag": {},
        "total_lines_changed": sum(c.lines_total for c in commits),
    }

    for c in commits:
        summary["by_grade"][c.grade] = summary["by_grade"].get(c.grade, 0) + 1
        for tag in c.tags:
            summary["by_tag"][tag] = summary["by_tag"].get(tag, 0) + 1

    return {
        "schema_version": SCHEMA_VERSION,
        "producer_skill": PRODUCER_SKILL,
        "summary": summary,
        "commits": [asdict(c) for c in commits],
        "analysis_timestamp": datetime.now().isoformat(),
        "project_path": str(project_root),
        "branch": branch,
    }


def main():
    parser = argparse.ArgumentParser(description="Bugfix 分级")
    parser.add_argument("--project", required=True, help="项目根目录")
    parser.add_argument("--output", required=True, help="输出 JSON 路径")
    parser.add_argument("--branch", default="main", help="分析分支")
    parser.add_argument("--limit", type=int, default=100, help="最大分析 commit 数")

    args = parser.parse_args()

    result = grade_bugfix(args.project, args.branch, args.limit)

    # 输出结果
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    print(f"✓ 分析完成: {result['summary']['total_commits']} 个 fix commit")
    print(f"✓ 分级分布: {result['summary']['by_grade']}")


if __name__ == "__main__":
    main()
