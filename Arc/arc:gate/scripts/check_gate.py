#!/usr/bin/env python3
"""
check_gate.py - CI 门禁检查脚本

用法:
    python check_gate.py --project /path/to/project [--mode strict] [--config gate-config.yaml]
"""

import copy
import argparse
import hashlib
import json
import os
import sys
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
import yaml


@dataclass
class Violation:
    """违规项"""

    rule: str
    severity: str  # warn / fail
    actual: Any
    threshold: Any
    file: str = ""
    line: int = 0
    message: str = ""
    whitelisted: bool = False
    whitelist_id: str = ""


@dataclass
class GateResult:
    """门禁结果"""

    status: str  # pass / fail
    mode: str
    overall_score: float
    violations: list[Violation]
    whitelist_applied: int
    exit_code: int
    checked_at: str


# 默认配置
DEFAULT_CONFIG = {
    "mode": "strict",
    "thresholds": {
        "overall_score": {"warn": 70, "fail": 60},
        "critical_issues": {"warn": 0, "fail": 1},
        "high_issues": {"warn": 5, "fail": 10},
        "security_issues": {"warn": 0, "fail": 1},
    },
    "whitelist": [],
    "dimension_weights": {
        "architecture": 0.2,
        "security": 0.25,
        "code-quality": 0.25,
        "business": 0.1,
        "devops": 0.1,
        "team": 0.05,
        "tech-debt": 0.05,
    },
}


def load_config(config_path: str | None, project_path: str) -> dict:
    """加载门禁配置"""
    config = copy.deepcopy(DEFAULT_CONFIG)

    # 尝试从项目目录加载
    if config_path is None:
        project_config = Path(project_path) / ".arc" / "gate-config.yaml"
        if project_config.exists():
            config_path = str(project_config)

    if config_path:
        try:
            with open(config_path, "r", encoding="utf-8") as f:
                user_config = yaml.safe_load(f)
                if user_config:
                    # 合并配置
                    if "mode" in user_config:
                        config["mode"] = user_config["mode"]

                    if "thresholds" in user_config and isinstance(
                        user_config["thresholds"], dict
                    ):
                        user_thresholds: dict = user_config["thresholds"]

                        thresholds_override: dict = dict(user_thresholds)
                        if (
                            "security_violations" in thresholds_override
                            and "security_issues" not in thresholds_override
                        ):
                            thresholds_override["security_issues"] = (
                                thresholds_override["security_violations"]
                            )

                        for k, v in thresholds_override.items():
                            if isinstance(v, dict) and isinstance(
                                config["thresholds"].get(k), dict
                            ):
                                config["thresholds"][k].update(v)
                            else:
                                config["thresholds"][k] = v

                    if "whitelist" in user_config:
                        config["whitelist"] = user_config["whitelist"]
                    if "dimension_weights" in user_config:
                        config["dimension_weights"] = user_config["dimension_weights"]
        except Exception as e:
            print(f"警告: 配置文件加载失败，使用默认配置: {e}")

    # 环境变量覆盖
    if os.environ.get("GATE_MODE"):
        config["mode"] = os.environ["GATE_MODE"]

    return config


def load_score_data(score_dir: str) -> dict:
    """加载 arc:score 数据"""
    score_path = Path(score_dir)

    data = {}

    # 加载综合评分
    overall_path = score_path / "score" / "overall-score.json"
    if overall_path.exists():
        data["overall"] = json.loads(overall_path.read_text(encoding="utf-8"))

    # 加载 Code Smell 报告
    smell_path = score_path / "analysis" / "smell-report.json"
    if smell_path.exists():
        data["smell"] = json.loads(smell_path.read_text(encoding="utf-8"))

    return data


def _parse_iso_datetime(value: str) -> datetime | None:
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except Exception:
        return None


def _artifact_is_expired(artifact: dict) -> bool:
    expires_at = artifact.get("expires_at")
    if not expires_at:
        return False

    exp = _parse_iso_datetime(str(expires_at))
    if not exp:
        return False

    now = datetime.now(timezone.utc)
    if exp.tzinfo is None:
        exp = exp.replace(tzinfo=timezone.utc)
    return now > exp


def _sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def _normalize_sha256(value: Any) -> str:
    s = str(value).strip() if value is not None else ""
    if not s:
        return ""
    if ":" in s:
        s = s.split(":", 1)[1].strip()
    if len(s) != 64:
        return ""
    try:
        int(s, 16)
    except ValueError:
        return ""
    return s.lower()


def find_score_dir_from_context_hub(project_root: Path) -> Path | None:
    index_path = project_root / ".arc" / "context-hub" / "index.json"
    if not index_path.exists():
        return None

    try:
        index = json.loads(index_path.read_text(encoding="utf-8"))
    except Exception:
        return None

    artifacts = index.get("artifacts")
    if not isinstance(artifacts, list):
        return None

    score_artifacts: list[dict] = [
        a
        for a in artifacts
        if isinstance(a, dict) and a.get("producer_skill") == "arc:score"
    ]
    if not score_artifacts:
        return None

    def preference(a: dict) -> int:
        path_str = str(a.get("path", ""))
        if path_str.endswith("handoff/review-input.json"):
            return 0
        if path_str.endswith("score/overall-score.json"):
            return 1
        if path_str.endswith("analysis/smell-report.json"):
            return 2
        if path_str.endswith("analysis/bugfix-grades.json"):
            return 3
        return 10

    for a in sorted(score_artifacts, key=preference):
        if _artifact_is_expired(a):
            continue

        p_raw = a.get("path")
        if not p_raw:
            continue

        artifact_path = Path(str(p_raw))
        if not artifact_path.is_absolute():
            artifact_path = (project_root / artifact_path).resolve()

        if not artifact_path.exists():
            continue

        expected_hash = _normalize_sha256(a.get("content_hash"))
        if expected_hash:
            if not artifact_path.is_file():
                continue
            actual_hash = _sha256_file(artifact_path)
            if actual_hash != expected_hash:
                continue

        candidate = artifact_path.parent.parent
        if (candidate / "score" / "overall-score.json").exists():
            return candidate

    return None


def check_whitelist(violation: Violation, whitelist: list[dict]) -> tuple[bool, str]:
    """检查违规是否在豁免清单中"""
    now = datetime.now(timezone.utc)

    for item in whitelist:
        # 检查规则匹配
        if item.get("rule") != violation.rule:
            continue

        # 检查文件匹配
        if item.get("file") and item.get("file") != violation.file:
            continue

        # 检查过期时间
        expires_at = item.get("expires_at")
        if expires_at:
            try:
                exp_time = datetime.fromisoformat(expires_at.replace("Z", "+00:00"))
                if now > exp_time:
                    continue  # 豁免已过期
            except Exception:
                pass

        return True, item.get("id", "")

    return False, ""


def run_gate_check(
    project_path: str,
    score_dir: str,
    mode: str | None = None,
    config_path: str | None = None,
) -> GateResult:
    """
    执行门禁检查

    Args:
        project_path: 项目根目录
        score_dir: arc:score 输出目录
        mode: 门禁模式 (覆盖配置)
        config_path: 配置文件路径

    Returns:
        门禁结果
    """
    # 加载配置
    config = load_config(config_path, project_path)

    # 命令行模式覆盖配置
    if mode:
        config["mode"] = mode

    effective_mode = config["mode"]

    # 加载评分数据
    score_data = load_score_data(score_dir)

    violations = []
    whitelist_applied = 0

    overall = score_data.get("overall")
    has_overall = isinstance(overall, dict)
    if not has_overall:
        violations.append(
            Violation(
                rule="missing_overall_score",
                severity="fail",
                actual="missing",
                threshold="required",
                message="未找到 score/overall-score.json，无法执行门禁检查",
            )
        )

    overall_score = overall.get("score", 0) if has_overall else 0
    by_severity = overall.get("by_severity", {}) if has_overall else {}

    # 检查总分阈值
    thresholds = config["thresholds"]

    if has_overall:
        if overall_score < thresholds["overall_score"]["fail"]:
            violation = Violation(
                rule="overall_score_threshold",
                severity="fail",
                actual=overall_score,
                threshold=thresholds["overall_score"]["fail"],
                message=f"综合评分 {overall_score} 低于失败阈值 {thresholds['overall_score']['fail']}",
            )
            violations.append(violation)
        elif overall_score < thresholds["overall_score"]["warn"]:
            violation = Violation(
                rule="overall_score_threshold",
                severity="warn",
                actual=overall_score,
                threshold=thresholds["overall_score"]["warn"],
                message=f"综合评分 {overall_score} 低于告警阈值 {thresholds['overall_score']['warn']}",
            )
            violations.append(violation)

    # 检查严重问题
    if has_overall:
        critical_count = int(by_severity.get("critical", 0) or 0)
        if critical_count > thresholds["critical_issues"]["warn"]:
            severity = (
                "fail"
                if critical_count >= thresholds["critical_issues"]["fail"]
                else "warn"
            )
            violations.append(
                Violation(
                    rule="critical_issues_threshold",
                    severity=severity,
                    actual=critical_count,
                    threshold=thresholds["critical_issues"][severity],
                    message=f"存在 {critical_count} 个严重问题",
                )
            )

        high_count = int(by_severity.get("high", 0) or 0)
        if high_count > thresholds["high_issues"]["warn"]:
            severity = (
                "fail" if high_count >= thresholds["high_issues"]["fail"] else "warn"
            )
            violations.append(
                Violation(
                    rule="high_issues_threshold",
                    severity=severity,
                    actual=high_count,
                    threshold=thresholds["high_issues"][severity],
                    message=f"存在 {high_count} 个高优先级问题",
                )
            )

    # 检查安全问题
    smell = score_data.get("smell")
    has_smell = isinstance(smell, dict)
    if not has_smell:
        violations.append(
            Violation(
                rule="missing_smell_report",
                severity="fail",
                actual="missing",
                threshold="required",
                message="未找到 analysis/smell-report.json，无法检查安全问题",
            )
        )

    security_count = 0
    if has_smell:
        security_violations = [
            v
            for v in smell.get("violations", [])
            if isinstance(v, dict) and v.get("category") == "security"
        ]
        security_count = len(security_violations)

    if security_count > thresholds["security_issues"]["warn"]:
        severity = (
            "fail"
            if security_count >= thresholds["security_issues"]["fail"]
            else "warn"
        )
        violations.append(
            Violation(
                rule="security_issues_threshold",
                severity=severity,
                actual=security_count,
                threshold=thresholds["security_issues"][severity],
                message=f"存在 {security_count} 个安全问题",
            )
        )

    # 处理豁免
    whitelist = config.get("whitelist", [])
    for v in violations:
        whitelisted, whitelist_id = check_whitelist(v, whitelist)
        v.whitelisted = whitelisted
        v.whitelist_id = whitelist_id
        if whitelisted:
            whitelist_applied += 1

    # 计算最终状态
    # 未豁免的失败违规
    unwhitelisted_fail = [
        v for v in violations if v.severity == "fail" and not v.whitelisted
    ]

    if effective_mode == "warn":
        status = "pass"  # warn 模式总是通过
        exit_code = 0
    elif effective_mode == "strict":
        status = "fail" if unwhitelisted_fail else "pass"
        exit_code = 1 if unwhitelisted_fail else 0
    else:  # strict_dangerous
        dangerous_rules = {"security_issues_threshold", "critical_issues_threshold"}
        dangerous_blocker = any(
            (not v.whitelisted) and (v.rule in dangerous_rules) for v in violations
        )
        blockers = bool(unwhitelisted_fail) or dangerous_blocker
        status = "fail" if blockers else "pass"
        exit_code = 1 if blockers else 0

    return GateResult(
        status=status,
        mode=effective_mode,
        overall_score=overall_score,
        violations=violations,
        whitelist_applied=whitelist_applied,
        exit_code=exit_code,
        checked_at=datetime.now().isoformat(),
    )


def generate_summary(result: GateResult) -> str:
    """生成 Markdown 摘要"""
    status_emoji = "✅" if result.status == "pass" else "❌"

    content = f"""# CI 门禁报告

## 执行结果

| 指标 | 值 |
|------|-----|
| **状态** | {status_emoji} {result.status.upper()} |
| **模式** | {result.mode} |
| **综合评分** | {result.overall_score} / 100 |
| **豁免应用** | {result.whitelist_applied} |
| **检查时间** | {result.checked_at} |

## 违规详情

| 规则 | 严重程度 | 实际值 | 阈值 | 豁免 |
|------|---------|--------|------|------|
"""

    for v in result.violations:
        whitelisted_str = "✓" if v.whitelisted else "-"
        content += f"| {v.rule} | {v.severity} | {v.actual} | {v.threshold} | {whitelisted_str} |\n"

    if not result.violations:
        content += "| (无违规) | - | - | - | - |\n"

    return content


def main():
    parser = argparse.ArgumentParser(description="CI 门禁检查")
    parser.add_argument("--project", required=True, help="项目根目录")
    parser.add_argument("--score-dir", help="arc:score 输出目录")
    parser.add_argument(
        "--mode", choices=["warn", "strict", "strict_dangerous"], help="门禁模式"
    )
    parser.add_argument("--config", help="配置文件路径")
    parser.add_argument("--output-dir", default=".arc/gate-reports", help="输出目录")
    parser.add_argument(
        "--exit-code", action="store_true", help="以 exit code 返回结果"
    )

    args = parser.parse_args()

    # 确定 score 目录
    project_root = Path(args.project).resolve()
    score_dir = args.score_dir
    if not score_dir:
        ctx_score_dir = find_score_dir_from_context_hub(project_root)
        if ctx_score_dir:
            score_dir = str(ctx_score_dir)
        else:
            score_dir = str(project_root / ".arc" / "score" / project_root.name)

    # 执行检查
    result = run_gate_check(args.project, score_dir, args.mode, args.config)

    # 输出结果
    output_dir = project_root / args.output_dir
    output_dir.mkdir(parents=True, exist_ok=True)

    # JSON 结果
    result_dict = {
        "status": result.status,
        "mode": result.mode,
        "overall_score": result.overall_score,
        "violations": [asdict(v) for v in result.violations],
        "whitelist_applied": result.whitelist_applied,
        "exit_code": result.exit_code,
        "checked_at": result.checked_at,
    }

    json_path = output_dir / "gate-result.json"
    json_path.write_text(
        json.dumps(result_dict, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    # Markdown 摘要
    summary = generate_summary(result)
    summary_path = output_dir / "summary.md"
    summary_path.write_text(summary, encoding="utf-8")

    # 打印结果
    status_emoji = "✅" if result.status == "pass" else "❌"
    print(f"\n{status_emoji} 门禁{'通过' if result.status == 'pass' else '失败'}")
    print(f"  模式: {result.mode}")
    print(f"  评分: {result.overall_score}")
    print(f"  违规: {len(result.violations)} ({result.whitelist_applied} 已豁免)")
    print(f"\n报告: {json_path}")

    # Exit code
    if args.exit_code:
        sys.exit(result.exit_code)


if __name__ == "__main__":
    main()
