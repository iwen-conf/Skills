#!/usr/bin/env python3
"""
detect_smell.py - Code Smell 检测脚本

用法:
    python detect_smell.py --project /path/to/project --output smell.json
"""

import argparse
import json
import re
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path


@dataclass
class SmellViolation:
    """Code Smell 违规"""

    id: str
    category: str
    name: str
    severity: str
    file: str
    line: int
    message: str
    suggestion: str


SCHEMA_VERSION = "1.0.0"
PRODUCER_SKILL = "arc:score"


# 内置检测规则 (简化版，无需 AST)
BUILTIN_RULES = {
    # Security
    "hardcoded_credential": {
        "category": "security",
        "name": "硬编码凭证",
        "severity": "critical",
        "patterns": [
            r"password\s*=\s*['\"][^'\"]+['\"]",
            r"api_key\s*=\s*['\"][^'\"]+['\"]",
            r"secret\s*=\s*['\"][^'\"]+['\"]",
            r"token\s*=\s*['\"][^'\"]+['\"]",
            r"API_KEY\s*=\s*['\"][^'\"]+['\"]",
        ],
        "exclude_patterns": [
            r"password\s*=\s*os\.environ",
            r"password\s*=\s*process\.env",
            r"password\s*=\s*getenv",
        ],
        "message": "源码中直接写死凭证",
        "suggestion": "使用环境变量或密钥管理服务",
    },
    "injection_risk": {
        "category": "security",
        "name": "注入风险",
        "severity": "critical",
        "patterns": [
            r"eval\s*\(",
            r"exec\s*\(",
            r"execute\s*\(.*\+",
            r"query\s*\(.*\+",
            r"f[\"'].*SELECT.*\{",
            r"f[\"'].*INSERT.*\{",
            r"f[\"'].*UPDATE.*\{",
            r"f[\"'].*DELETE.*\{",
        ],
        "message": "存在注入风险",
        "suggestion": "使用参数化查询，避免动态执行",
    },
    "weak_crypto": {
        "category": "security",
        "name": "弱加密算法",
        "severity": "high",
        "patterns": [
            r"hashlib\.md5\s*\(",
            r"hashlib\.sha1\s*\(",
            r"from\s+Crypto\.Cipher\s+import\s+DES",
            r"MD5\s*\(",
            r"SHA1\s*\(",
        ],
        "message": "使用弱加密算法",
        "suggestion": "使用 SHA-256+ / AES / ChaCha20",
    },
    "sensitive_data_log": {
        "category": "security",
        "name": "敏感数据泄露",
        "severity": "critical",
        "patterns": [
            r"logger\.\w+\(.*password",
            r"console\.log\(.*password",
            r"print\(.*password",
            r"logger\.\w+\(.*token",
            r"console\.log\(.*token",
        ],
        "message": "日志中打印敏感字段",
        "suggestion": "脱敏或禁止记录敏感字段",
    },
    # Error Handling
    "swallowed_error": {
        "category": "error_handling",
        "name": "吞没异常",
        "severity": "high",
        "patterns": [
            r"except.*:\s*pass",
            r"catch\s*\([^)]*\)\s*\{\s*\}",
            r"except\s*:\s*pass",
        ],
        "message": "静默吞掉错误",
        "suggestion": "至少记录日志或抛出包装异常",
    },
    "broad_catch": {
        "category": "error_handling",
        "name": "过宽捕获",
        "severity": "medium",
        "patterns": [
            r"except\s+Exception\s*:",
            r"except\s*:",
            r"catch\s*\(Exception",
            r"catch\s*\(\s*\)",
        ],
        "message": "通配捕获所有异常",
        "suggestion": "捕获具体异常类型",
    },
    # Noise
    "empty_function_body": {
        "category": "noise",
        "name": "空函数体",
        "severity": "low",
        "patterns": [
            r"def\s+\w+\([^)]*\):\s*pass",
            r"function\s+\w+\s*\([^)]*\)\s*\{\s*\}",
            r"throw\s+new\s+NotImplementedException",
        ],
        "message": "函数体为空或仅含占位符",
        "suggestion": "实现函数逻辑或标记为抽象方法",
    },
    "commented_out_code": {
        "category": "noise",
        "name": "注释掉的代码",
        "severity": "low",
        "patterns": [
            r"#\s*(def|class|import|from|if|for|while|return)",
            r"//\s*(function|const|let|var|if|for|while|return)",
        ],
        "message": "存在注释掉的代码",
        "suggestion": "删除无用代码，使用版本控制追踪历史",
    },
    "dead_branch": {
        "category": "noise",
        "name": "死分支",
        "severity": "high",
        "patterns": [
            r"if\s+False\s*:",
            r"if\s*\(\s*false\s*\)",
            r"if\s*\(\s*False\s*\)",
        ],
        "message": "条件永远为假",
        "suggestion": "移除死代码分支",
    },
    # Defensive
    "leftover_boilerplate": {
        "category": "defensive",
        "name": "残留样板",
        "severity": "low",
        "patterns": [
            r"#\s*TODO",
            r"#\s*FIXME",
            r"#\s*HACK",
            r"#\s*XXX",
            r"//\s*TODO",
            r"//\s*FIXME",
            r"//\s*HACK",
        ],
        "message": "存在 TODO/FIXME/HACK 注释",
        "suggestion": "清理遗留标记或创建 Issue 追踪",
    },
}


def detect_smell(
    project_path: str,
    languages: list[str] | None = None,
    severity_threshold: str = "low",
    exclude_patterns: list[str] | None = None,
) -> dict:
    """
    执行 Code Smell 检测

    Args:
        project_path: 项目根目录
        languages: 目标语言列表
        severity_threshold: 最低严重程度
        exclude_patterns: 排除模式

    Returns:
        检测结果
    """
    project_root = Path(project_path)

    if languages is None:
        languages = ["python", "typescript", "javascript", "go", "java"]

    if exclude_patterns is None:
        exclude_patterns = [
            "node_modules/**",
            "vendor/**",
            "**/test/**",
            "**/.git/**",
            "**/migrations/**",
        ]

    # 收集文件
    extensions = {
        "python": [".py"],
        "typescript": [".ts", ".tsx"],
        "javascript": [".js", ".jsx"],
        "go": [".go"],
        "java": [".java"],
    }

    files = []
    for lang in languages:
        for ext in extensions.get(lang, []):
            files.extend(project_root.rglob(f"*{ext}"))

    # 过滤排除的文件
    def should_exclude(file_path: Path) -> bool:
        rel_path = str(file_path.relative_to(project_root))
        for pattern in exclude_patterns:
            if pattern.endswith("/**"):
                if rel_path.startswith(pattern[:-3]):
                    return True
            elif pattern.startswith("**/"):
                if file_path.name == pattern[3:]:
                    return True
        return False

    files = [f for f in files if not should_exclude(f)]

    # 执行检测
    violations = []
    severity_order = ["low", "medium", "high", "critical"]
    threshold_idx = severity_order.index(severity_threshold)

    for file_path in files:
        try:
            content = file_path.read_text(encoding="utf-8", errors="ignore")
            lines = content.split("\n")

            for rule_id, rule in BUILTIN_RULES.items():
                # 检查严重程度阈值
                rule_severity_idx = severity_order.index(rule["severity"])
                if rule_severity_idx < threshold_idx:
                    continue

                # 检查模式
                for pattern in rule["patterns"]:
                    regex = re.compile(pattern, re.IGNORECASE)

                    for i, line in enumerate(lines, 1):
                        if regex.search(line):
                            # 检查排除模式
                            excluded = False
                            for exclude in rule.get("exclude_patterns", []):
                                if re.search(exclude, line, re.IGNORECASE):
                                    excluded = True
                                    break

                            if not excluded:
                                violations.append(
                                    SmellViolation(
                                        id=rule_id,
                                        category=rule["category"],
                                        name=rule["name"],
                                        severity=rule["severity"],
                                        file=str(file_path.relative_to(project_root)),
                                        line=i,
                                        message=f"{rule['message']}: {line.strip()[:50]}",
                                        suggestion=rule["suggestion"],
                                    )
                                )
        except Exception:
            continue

    # 汇总结果
    summary = {
        "total_violations": len(violations),
        "by_severity": {},
        "by_category": {},
        "files_scanned": len(files),
    }

    for v in violations:
        summary["by_severity"][v.severity] = (
            summary["by_severity"].get(v.severity, 0) + 1
        )
        summary["by_category"][v.category] = (
            summary["by_category"].get(v.category, 0) + 1
        )

    return {
        "schema_version": SCHEMA_VERSION,
        "producer_skill": PRODUCER_SKILL,
        "summary": summary,
        "violations": [asdict(v) for v in violations],
        "scan_timestamp": datetime.now().isoformat(),
        "project_path": str(project_root),
    }


def main():
    parser = argparse.ArgumentParser(description="Code Smell 检测")
    parser.add_argument("--project", required=True, help="项目根目录")
    parser.add_argument("--output", required=True, help="输出 JSON 路径")
    parser.add_argument("--languages", nargs="+", help="目标语言")
    parser.add_argument(
        "--severity-threshold",
        default="low",
        choices=["low", "medium", "high", "critical"],
        help="最低严重程度",
    )
    parser.add_argument("--markdown", help="Markdown 报告输出路径 (可选)")

    args = parser.parse_args()

    result = detect_smell(args.project, args.languages, args.severity_threshold)

    # 输出 JSON
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    print(f"✓ 扫描完成: {result['summary']['files_scanned']} 个文件")
    print(f"✓ 发现违规: {result['summary']['total_violations']} 项")

    # 输出 Markdown
    if args.markdown:
        md_content = generate_markdown_report(result)
        md_path = Path(args.markdown)
        md_path.write_text(md_content, encoding="utf-8")
        print(f"✓ Markdown 报告: {md_path}")


def generate_markdown_report(result: dict) -> str:
    """生成 Markdown 格式报告"""
    summary = result["summary"]

    content = f"""# Code Smell 检测报告

## 概览

| 指标 | 值 |
|------|-----|
| 扫描文件数 | {summary["files_scanned"]} |
| 违规总数 | {summary["total_violations"]} |

## 严重程度分布

| 严重程度 | 数量 |
|----------|------|
"""

    for sev in ["critical", "high", "medium", "low"]:
        count = summary["by_severity"].get(sev, 0)
        emoji = {"critical": "🔴", "high": "🟠", "medium": "🟡", "low": "🟢"}[sev]
        content += f"| {emoji} {sev} | {count} |\n"

    content += "\n## 类别分布\n\n| 类别 | 数量 |\n|------|------|\n"

    for cat, count in sorted(summary["by_category"].items(), key=lambda x: -x[1]):
        content += f"| {cat} | {count} |\n"

    content += "\n## 违规详情\n\n"

    # 按严重程度分组
    for sev in ["critical", "high", "medium", "low"]:
        sev_violations = [v for v in result["violations"] if v["severity"] == sev]
        if sev_violations:
            emoji = {"critical": "🔴", "high": "🟠", "medium": "🟡", "low": "🟢"}[sev]
            content += f"### {emoji} {sev.upper()}\n\n"

            for v in sev_violations[:20]:  # 限制每类显示 20 条
                content += f"- **{v['name']}** ({v['id']})\n"
                content += f"  - 文件: `{v['file']}:{v['line']}`\n"
                content += f"  - 消息: {v['message']}\n"
                content += f"  - 建议: {v['suggestion']}\n\n"

    return content


if __name__ == "__main__":
    main()
