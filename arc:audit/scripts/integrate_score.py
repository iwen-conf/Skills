#!/usr/bin/env python3
"""
integrate_score.py - 将 arc:score 量化数据集成到 arc:audit

用法:
    python integrate_score.py --score-dir .arc/score/<project> --review-dir .arc/arc:audit/<project>

或（未传 --score-dir 时尝试从 context-hub 自动发现）:
    python integrate_score.py --project-path <project_root> --review-dir .arc/arc:audit/<project>
"""

import argparse
import json
from html import escape
from datetime import datetime, timezone
from pathlib import Path


def _parse_iso_datetime(value: str) -> datetime | None:
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except Exception:
        return None


def _artifact_is_expired(artifact: dict) -> bool:
    expires_at = artifact.get("expires_at")
    if not expires_at:
        return False

    dt = _parse_iso_datetime(str(expires_at))
    if not dt:
        return False

    return datetime.now(timezone.utc) > dt


def _infer_project_root_from_review_dir(review_dir: Path) -> Path | None:
    try:
        if (
            review_dir.parent.name == "review"
            and review_dir.parent.parent.name == ".arc"
        ):
            return review_dir.parent.parent.parent
    except Exception:
        return None
    return None


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

        candidate = artifact_path.parent.parent
        expected = [
            candidate / "handoff" / "review-input.json",
            candidate / "score" / "overall-score.json",
            candidate / "analysis" / "smell-report.json",
            candidate / "analysis" / "bugfix-grades.json",
        ]
        if any(p.exists() for p in expected):
            return candidate

    return None


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
    生成供 arc:audit 消费的输入数据

    将量化数据映射到七维度框架
    """
    # 维度映射：Code Smell 类别 -> arc:audit 维度
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
    special_scores = {
        "business_maturity": round(dimension_data["business"]["score"] / 10, 1),
        "dependency_health": round(dimension_data["tech-debt"]["score"] / 10, 1),
    }

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
        "special_scores": special_scores,
        "integration_version": "1.0.0",
    }


def _severity_style(by_severity: dict) -> tuple[str, list[str]]:
    """生成严重级别 donut 图的 conic-gradient 样式与图例"""
    severity_order = ["critical", "high", "medium", "low"]
    severity_colors = {
        "critical": "#ef4444",
        "high": "#f97316",
        "medium": "#f59e0b",
        "low": "#22c55e",
    }

    total = sum(int(by_severity.get(level, 0)) for level in severity_order)
    if total <= 0:
        return "conic-gradient(#d1d5db 0 100%)", ["无数据"]

    angle_start = 0.0
    gradient_parts: list[str] = []
    legend_lines: list[str] = []
    for level in severity_order:
        count = int(by_severity.get(level, 0))
        if count <= 0:
            continue
        angle_end = angle_start + (count / total) * 360.0
        color = severity_colors[level]
        gradient_parts.append(f"{color} {angle_start:.2f}deg {angle_end:.2f}deg")
        legend_lines.append(f"{level}: {count}")
        angle_start = angle_end

    return "conic-gradient(" + ", ".join(gradient_parts) + ")", legend_lines


def _dimension_label(dimension_key: str) -> str:
    dimension_labels = {
        "architecture": "架构设计",
        "security": "安全合规",
        "code-quality": "代码质量",
        "business": "业务价值",
        "devops": "运维交付",
        "team": "团队协作",
        "tech-debt": "技术债务",
    }
    return dimension_labels.get(dimension_key, dimension_key)


def _score_level(score: float) -> str:
    if score >= 90:
        return "卓越"
    if score >= 70:
        return "良好"
    if score >= 50:
        return "合格"
    if score >= 30:
        return "警告"
    return "危险"


def _theme_from_now() -> str:
    """按当前本地时间选择主题：07:00-18:59 浅色，其余深色。"""
    hour = datetime.now().hour
    return "light" if 7 <= hour < 19 else "dark"


def _severity_rank(severity: str) -> int:
    level = str(severity or "").lower()
    return {
        "critical": 4,
        "high": 3,
        "medium": 2,
        "low": 1,
    }.get(level, 0)


def _issue_rows_html(
    issues: list[dict], include_dimension: bool, empty_text: str, max_rows: int = 30
) -> str:
    if not issues:
        colspan = 6 if include_dimension else 5
        return f'<tr><td colspan="{colspan}" class="empty-cell">{escape(empty_text)}</td></tr>'

    rows: list[str] = []
    for issue in issues[:max_rows]:
        issue_id = escape(str(issue.get("id", "-")))
        severity = escape(str(issue.get("severity", "-")))
        file_path = escape(str(issue.get("file", "-")))
        line_number = escape(str(issue.get("line", "-")))
        message = escape(str(issue.get("message", "-")))
        if include_dimension:
            dimension = escape(_dimension_label(str(issue.get("dimension", "-"))))
            rows.append(
                "<tr>"
                f"<td>{dimension}</td>"
                f"<td>{issue_id}</td>"
                f"<td>{severity}</td>"
                f"<td>{file_path}</td>"
                f"<td>{line_number}</td>"
                f"<td>{message}</td>"
                "</tr>"
            )
        else:
            rows.append(
                "<tr>"
                f"<td>{issue_id}</td>"
                f"<td>{severity}</td>"
                f"<td>{file_path}</td>"
                f"<td>{line_number}</td>"
                f"<td>{message}</td>"
                "</tr>"
            )
    return "".join(rows)


def _is_business_related_issue(issue: dict) -> bool:
    text = " ".join(
        [
            str(issue.get("id", "")),
            str(issue.get("file", "")),
            str(issue.get("message", "")),
            str(issue.get("dimension", "")),
        ]
    ).lower()
    business_keywords = [
        "业务",
        "流程",
        "链路",
        "规则",
        "状态",
        "订单",
        "支付",
        "结算",
        "库存",
        "客户",
        "用户",
        "domain",
        "business",
        "workflow",
        "process",
        "rule",
        "state",
        "order",
        "payment",
        "checkout",
        "billing",
        "invoice",
    ]
    return any(keyword in text for keyword in business_keywords)


def generate_html_dashboard(review_input: dict, theme: str | None = None) -> str:
    """生成 HTML 可视化看板（图表 + 多维度 Tab 切换）"""
    dimension_order = [
        "architecture",
        "security",
        "code-quality",
        "business",
        "devops",
        "team",
        "tech-debt",
    ]
    dimension_scores = review_input.get("dimension_scores", {})
    dimension_issues = review_input.get("dimension_issues", {})
    smell_summary = review_input.get("smell_summary", {})
    by_severity = smell_summary.get("by_severity", {})
    gradient_style, severity_legend = _severity_style(by_severity)

    available_dimensions = list(dimension_order)
    tab_buttons: list[str] = []
    tab_panels: list[str] = []
    bar_rows: list[str] = []
    dimension_issue_counts: dict[str, int] = {}
    all_issues: list[dict] = []
    dimension_panels: list[dict] = []

    for dimension_key in available_dimensions:
        label = _dimension_label(dimension_key)
        score = float(dimension_scores.get(dimension_key, 0))
        level = _score_level(score)
        raw_issues = dimension_issues.get(dimension_key, [])
        normalized_issues = []
        for issue in raw_issues:
            if isinstance(issue, dict):
                normalized_issues.append({**issue, "dimension": dimension_key})

        issue_count = len(normalized_issues)
        dimension_issue_counts[dimension_key] = issue_count
        all_issues.extend(normalized_issues)

        issue_rows_html = _issue_rows_html(
            normalized_issues,
            include_dimension=False,
            empty_text="该维度暂无量化问题",
        )

        severity_count = {"critical": 0, "high": 0, "medium": 0, "low": 0, "other": 0}
        file_hotspots: dict[str, int] = {}
        for issue in normalized_issues:
            severity = str(issue.get("severity", "")).lower()
            if severity in severity_count:
                severity_count[severity] += 1
            else:
                severity_count["other"] += 1
            file_key = str(issue.get("file", "-")) or "-"
            file_hotspots[file_key] = file_hotspots.get(file_key, 0) + 1

        severity_rows = []
        for severity_key in ["critical", "high", "medium", "low", "other"]:
            severity_rows.append(
                "<tr>"
                f"<td>{escape(severity_key)}</td>"
                f"<td>{severity_count.get(severity_key, 0)}</td>"
                "</tr>"
            )

        hotspot_rows = []
        sorted_hotspots = sorted(
            file_hotspots.items(),
            key=lambda item: item[1],
            reverse=True,
        )[:10]
        for file_path, count in sorted_hotspots:
            hotspot_rows.append(
                "<tr>"
                f"<td>{escape(file_path)}</td>"
                f"<td>{count}</td>"
                "</tr>"
            )
        if not hotspot_rows:
            hotspot_rows.append(
                '<tr><td colspan="2" class="empty-cell">该维度暂无热点文件</td></tr>'
            )

        critical_and_high = severity_count["critical"] + severity_count["high"]

        dimension_panels.append(
            {
                "label": label,
                "score": score,
                "panel_html": f"""
<div class="panel-kpi">
  <div class="kpi-box"><span>评分</span><strong>{score:.0f}/100</strong></div>
  <div class="kpi-box"><span>评级</span><strong>{escape(level)}</strong></div>
  <div class="kpi-box"><span>问题数</span><strong>{issue_count}</strong></div>
  <div class="kpi-box"><span>高风险问题(C+H)</span><strong>{critical_and_high}</strong></div>
</div>
<h3>严重级别分布</h3>
<table class="issues-table">
  <thead>
    <tr><th>严重级别</th><th>数量</th></tr>
  </thead>
  <tbody>
    {''.join(severity_rows)}
  </tbody>
</table>
<h3>热点文件 Top 10</h3>
<table class="issues-table">
  <thead>
    <tr><th>文件</th><th>问题数</th></tr>
  </thead>
  <tbody>
    {''.join(hotspot_rows)}
  </tbody>
</table>
<h3>问题明细</h3>
<table class="issues-table">
  <thead>
    <tr><th>ID</th><th>严重级别</th><th>文件</th><th>行</th><th>描述</th></tr>
  </thead>
  <tbody>
    {issue_rows_html}
  </tbody>
</table>
""",
            }
        )

        bar_rows.append(
            f"""
<div class="bar-row">
  <div class="bar-label">{escape(label)}</div>
  <div class="bar-track"><div class="bar-fill" style="width: {max(0, min(score, 100)):.1f}%;"></div></div>
  <div class="bar-value">{score:.0f}</div>
</div>
"""
        )

    total_issue_count = len(all_issues)
    critical_issue_count = sum(
        1 for issue in all_issues if str(issue.get("severity", "")).lower() == "critical"
    )
    high_issue_count = sum(
        1 for issue in all_issues if str(issue.get("severity", "")).lower() == "high"
    )

    dimension_summary_rows = []
    for dimension_key in available_dimensions:
        dimension_summary_rows.append(
            "<tr>"
            f"<td>{escape(_dimension_label(dimension_key))}</td>"
            f"<td>{float(dimension_scores.get(dimension_key, 0)):.0f}/100</td>"
            f"<td>{dimension_issue_counts.get(dimension_key, 0)}</td>"
            "</tr>"
        )

    severity_summary_rows = []
    for severity_key in ["critical", "high", "medium", "low"]:
        severity_summary_rows.append(
            "<tr>"
            f"<td>{escape(severity_key)}</td>"
            f"<td>{int(by_severity.get(severity_key, 0))}</td>"
            "</tr>"
        )

    category_summary_rows = []
    for category_key, count in sorted(
        smell_summary.get("by_category", {}).items(),
        key=lambda item: str(item[0]),
    ):
        category_summary_rows.append(
            "<tr>"
            f"<td>{escape(str(category_key))}</td>"
            f"<td>{int(count)}</td>"
            "</tr>"
        )
    if not category_summary_rows:
        category_summary_rows.append(
            '<tr><td colspan="2" class="empty-cell">暂无类别统计数据</td></tr>'
        )

    bugfix_grade_rows = []
    bugfix_summary = review_input.get("bugfix_summary") or {}
    bugfix_by_grade = bugfix_summary.get("by_grade", {})
    for grade_key, count in sorted(
        bugfix_by_grade.items(),
        key=lambda item: str(item[0]),
    ):
        bugfix_grade_rows.append(
            "<tr>"
            f"<td>{escape(str(grade_key))}</td>"
            f"<td>{int(count)}</td>"
            "</tr>"
        )
    if not bugfix_grade_rows:
        bugfix_grade_rows.append(
            '<tr><td colspan="2" class="empty-cell">暂无修复等级分布</td></tr>'
        )

    overview_panel_html = f"""
<div class="panel-kpi">
  <div class="kpi-box"><span>综合分</span><strong>{float(review_input.get('quantitative_score', 0)):.0f}/100</strong></div>
  <div class="kpi-box"><span>总问题数</span><strong>{total_issue_count}</strong></div>
  <div class="kpi-box"><span>Critical</span><strong>{critical_issue_count}</strong></div>
  <div class="kpi-box"><span>High</span><strong>{high_issue_count}</strong></div>
</div>
<h3>七维摘要</h3>
<table class="issues-table">
  <thead>
    <tr><th>维度</th><th>评分</th><th>问题数</th></tr>
  </thead>
  <tbody>
    {''.join(dimension_summary_rows)}
  </tbody>
</table>
<h3>严重级别分布</h3>
<table class="issues-table">
  <thead>
    <tr><th>严重级别</th><th>数量</th></tr>
  </thead>
  <tbody>
    {''.join(severity_summary_rows)}
  </tbody>
</table>
<h3>问题类别分布</h3>
<table class="issues-table">
  <thead>
    <tr><th>类别</th><th>数量</th></tr>
  </thead>
  <tbody>
    {''.join(category_summary_rows)}
  </tbody>
</table>
<h3>修复历史等级分布</h3>
<table class="issues-table">
  <thead>
    <tr><th>等级</th><th>数量</th></tr>
  </thead>
  <tbody>
    {''.join(bugfix_grade_rows)}
  </tbody>
</table>
"""

    business_related_issues = []
    seen_business_issue_keys = set()
    for issue in all_issues:
        is_related = str(issue.get("dimension", "")) == "business" or _is_business_related_issue(issue)
        if not is_related:
            continue
        dedup_key = (
            str(issue.get("id", "")),
            str(issue.get("severity", "")),
            str(issue.get("file", "")),
            str(issue.get("line", "")),
            str(issue.get("message", "")),
            str(issue.get("dimension", "")),
        )
        if dedup_key in seen_business_issue_keys:
            continue
        seen_business_issue_keys.add(dedup_key)
        business_related_issues.append(issue)

    business_keywords_bug = ["bug", "错误", "异常", "失败", "缺陷", "fault", "defect"]
    business_keywords_logic = [
        "逻辑",
        "流程",
        "链路",
        "状态",
        "规则",
        "不一致",
        "workflow",
        "state",
        "rule",
        "condition",
    ]

    business_bug_signal_count = 0
    business_logic_signal_count = 0
    business_critical_count = 0
    business_high_count = 0
    for issue in business_related_issues:
        severity = str(issue.get("severity", "")).lower()
        issue_text = " ".join(
            [
                str(issue.get("id", "")),
                str(issue.get("file", "")),
                str(issue.get("message", "")),
            ]
        ).lower()
        if severity == "critical":
            business_critical_count += 1
        if severity == "high":
            business_high_count += 1
        if any(keyword in issue_text for keyword in business_keywords_bug):
            business_bug_signal_count += 1
        if any(keyword in issue_text for keyword in business_keywords_logic):
            business_logic_signal_count += 1

    business_score = float(dimension_scores.get("business", 0))
    flow_score = int(
        max(
            0,
            min(
                100,
                round(
                    business_score
                    - business_critical_count * 12
                    - business_high_count * 6
                    - business_bug_signal_count * 3
                    - business_logic_signal_count * 2
                ),
            ),
        )
    )
    if flow_score >= 85:
        flow_status = "通畅"
    elif flow_score >= 65:
        flow_status = "基本通畅"
    elif flow_score >= 45:
        flow_status = "存在断点"
    else:
        flow_status = "阻塞明显"

    cross_dimension_business_issues = [
        issue
        for issue in business_related_issues
        if str(issue.get("dimension", "")) != "business"
    ]

    business_related_rows_html = _issue_rows_html(
        sorted(
            cross_dimension_business_issues,
            key=lambda issue: _severity_rank(str(issue.get("severity", ""))),
            reverse=True,
        ),
        include_dimension=True,
        empty_text="未发现跨维度业务链路问题",
        max_rows=30,
    )

    special_scores = review_input.get("special_scores", {})
    business_maturity = float(special_scores.get("business_maturity", 0))
    dependency_health = float(special_scores.get("dependency_health", 0))
    selected_theme = theme if theme in {"light", "dark"} else _theme_from_now()
    theme_mode_label = (
        "浅色（基于 time now()）"
        if selected_theme == "light"
        else "深色（基于 time now()）"
    )

    if business_maturity >= 8.5:
        business_completion_level = "高"
    elif business_maturity >= 6.5:
        business_completion_level = "中"
    else:
        business_completion_level = "低"

    chain_signals = [
        ("需求到规则一致性", business_logic_signal_count + business_bug_signal_count),
        (
            "服务编排链路",
            sum(
                1
                for issue in cross_dimension_business_issues
                if str(issue.get("dimension", "")) in {"architecture", "devops", "team"}
            ),
        ),
        (
            "数据状态一致性",
            sum(
                1
                for issue in cross_dimension_business_issues
                if any(
                    token in " ".join(
                        [
                            str(issue.get("id", "")),
                            str(issue.get("file", "")),
                            str(issue.get("message", "")),
                        ]
                    ).lower()
                    for token in ["state", "status", "db", "database", "transaction", "状态", "一致", "数据"]
                )
            ),
        ),
        ("异常与回滚闭环", business_critical_count + business_high_count),
    ]

    chain_rows = []
    for chain_name, signal_count in chain_signals:
        if signal_count <= 0:
            chain_status = "通畅"
        elif signal_count <= 2:
            chain_status = "需关注"
        else:
            chain_status = "存在断点"
        chain_rows.append(
            "<tr>"
            f"<td>{escape(chain_name)}</td>"
            f"<td>{signal_count}</td>"
            f"<td>{chain_status}</td>"
            "</tr>"
        )

    business_panel_html = f"""
<div class="panel-kpi">
  <div class="kpi-box"><span>业务完成度</span><strong>{business_maturity:.1f}/10 ({business_completion_level})</strong></div>
  <div class="kpi-box"><span>业务链路通畅度</span><strong>{flow_score}/100 ({flow_status})</strong></div>
  <div class="kpi-box"><span>明显业务 Bug 信号</span><strong>{business_bug_signal_count}</strong></div>
  <div class="kpi-box"><span>业务逻辑不通顺信号</span><strong>{business_logic_signal_count}</strong></div>
</div>
<h3>业务链路分段评估</h3>
<table class="issues-table">
  <thead>
    <tr><th>链路段</th><th>信号数</th><th>状态</th></tr>
  </thead>
  <tbody>
    {''.join(chain_rows)}
  </tbody>
</table>
<h3>跨维度业务链路问题（与业务维度 Tab 去重）</h3>
<table class="issues-table">
  <thead>
    <tr><th>来源维度</th><th>ID</th><th>严重级别</th><th>文件</th><th>行</th><th>描述</th></tr>
  </thead>
  <tbody>
    {business_related_rows_html}
  </tbody>
</table>
"""

    def append_tab(title: str, content_html: str) -> None:
        index = len(tab_buttons)
        tab_id = f"tab-{index}"
        panel_id = f"panel-{index}"
        tab_active_class = "active" if index == 0 else ""
        tab_selected = "true" if index == 0 else "false"
        panel_active_class = "active" if index == 0 else ""
        panel_hidden = "" if index == 0 else "hidden"
        tab_buttons.append(
            f'<button type="button" class="tab-btn {tab_active_class}" '
            f'role="tab" id="{tab_id}" aria-selected="{tab_selected}" '
            f'aria-controls="{panel_id}" data-tab="{index}">{escape(title)}</button>'
        )
        tab_panels.append(
            f'<section class="tab-panel {panel_active_class}" role="tabpanel" id="{panel_id}" '
            f'aria-labelledby="{tab_id}" {panel_hidden}>{content_html}</section>'
        )

    append_tab("总览", overview_panel_html)
    for item in dimension_panels:
        append_tab(f"{item['label']} ({item['score']:.0f})", item["panel_html"])
    append_tab("业务完成度", business_panel_html)

    return f"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>评估可视化看板</title>
  <style>
    :root {{
      --bg: #f8fafc;
      --panel: #ffffff;
      --text: #0f172a;
      --muted: #475569;
      --primary: #3b82f6;
      --border: #e2e8f0;
      --good: #22c55e;
      --warn: #f59e0b;
      --bad: #ef4444;
      --chip: #f1f5f9;
      --chip-active: #dbeafe;
      --chip-active-border: #93c5fd;
      --chip-active-text: #1d4ed8;
    }}
    [data-theme="dark"] {{
      --bg: #0b1220;
      --panel: #111827;
      --text: #e5e7eb;
      --muted: #9ca3af;
      --border: #2b3548;
      --primary: #60a5fa;
      --good: #34d399;
      --warn: #fbbf24;
      --bad: #f87171;
      --chip: #1f2937;
      --chip-active: #1e3a8a;
      --chip-active-border: #3b82f6;
      --chip-active-text: #dbeafe;
    }}
    body {{
      margin: 0;
      padding: 24px;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      color: var(--text);
      background: var(--bg);
    }}
    .container {{
      max-width: 1200px;
      margin: 0 auto;
    }}
    .header, .panel {{
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 18px;
      margin-bottom: 16px;
      box-shadow: 0 4px 16px rgba(15, 23, 42, 0.04);
    }}
    h1, h2 {{
      margin: 0 0 10px 0;
      font-weight: 700;
    }}
    h3 {{
      margin: 14px 0 8px 0;
      font-size: 15px;
      font-weight: 700;
    }}
    .header-top {{
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
      flex-wrap: wrap;
    }}
    .toolbar {{
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
    }}
    .tool-btn {{
      border: 1px solid var(--border);
      background: var(--chip);
      color: var(--text);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      cursor: pointer;
    }}
    .tool-btn:hover {{
      border-color: var(--primary);
    }}
    .sub-text {{
      color: var(--muted);
      font-size: 14px;
    }}
    .summary-grid {{
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 12px;
      margin-top: 12px;
    }}
    .summary-card {{
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px;
      background: color-mix(in srgb, var(--panel) 80%, var(--bg) 20%);
    }}
    .summary-card span {{
      display: block;
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 6px;
    }}
    .summary-card strong {{
      font-size: 20px;
    }}
    .charts-grid {{
      display: grid;
      grid-template-columns: 2fr 1fr;
      gap: 14px;
    }}
    .bar-row {{
      display: grid;
      grid-template-columns: 110px 1fr 36px;
      align-items: center;
      gap: 10px;
      margin: 10px 0;
    }}
    .bar-label {{
      font-size: 13px;
      color: var(--muted);
    }}
    .bar-track {{
      height: 10px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--border) 60%, var(--panel) 40%);
      overflow: hidden;
    }}
    .bar-fill {{
      height: 100%;
      border-radius: 999px;
      background: linear-gradient(90deg, #60a5fa 0%, #2563eb 100%);
    }}
    .bar-value {{
      text-align: right;
      font-weight: 600;
      font-size: 13px;
    }}
    .donut-wrap {{
      display: flex;
      align-items: center;
      gap: 12px;
      margin-top: 10px;
    }}
    .donut {{
      width: 120px;
      height: 120px;
      border-radius: 50%;
      background: {gradient_style};
      position: relative;
    }}
    .donut::after {{
      content: "";
      position: absolute;
      inset: 22px;
      background: var(--panel);
      border-radius: 50%;
    }}
    .legend {{
      margin: 0;
      padding-left: 18px;
      color: var(--muted);
      font-size: 13px;
    }}
    .tabs {{
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-bottom: 12px;
    }}
    .tab-btn {{
      border: 1px solid var(--border);
      background: var(--chip);
      color: var(--text);
      border-radius: 8px;
      padding: 8px 12px;
      cursor: pointer;
      font-size: 13px;
    }}
    .tab-btn.active {{
      background: var(--chip-active);
      border-color: var(--chip-active-border);
      color: var(--chip-active-text);
      font-weight: 600;
    }}
    .tab-panel {{
      display: none;
    }}
    .tab-panel.active {{
      display: block;
    }}
    .panel-kpi {{
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
      gap: 10px;
      margin-bottom: 12px;
    }}
    .kpi-box {{
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 10px;
      background: color-mix(in srgb, var(--panel) 82%, var(--bg) 18%);
    }}
    .kpi-box span {{
      display: block;
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 4px;
    }}
    .kpi-box strong {{
      font-size: 18px;
    }}
    .issues-table {{
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }}
    .issues-table th, .issues-table td {{
      border: 1px solid var(--border);
      padding: 8px;
      text-align: left;
      vertical-align: top;
    }}
    .issues-table th {{
      background: color-mix(in srgb, var(--panel) 76%, var(--bg) 24%);
    }}
    .empty-cell {{
      text-align: center;
      color: var(--muted);
      font-style: italic;
    }}
    @media (max-width: 920px) {{
      .charts-grid {{
        grid-template-columns: 1fr;
      }}
    }}
    @media print {{
      body {{
        padding: 0;
      }}
      .toolbar, .tabs {{
        display: none !important;
      }}
      .header, .panel {{
        box-shadow: none;
        break-inside: avoid;
      }}
      .tab-panel {{
        display: block !important;
        margin-bottom: 18px;
      }}
    }}
  </style>
</head>
<body data-theme="{selected_theme}">
  <div class="container">
    <section class="header">
      <div class="header-top">
        <h1>评估可视化看板（HTML）</h1>
        <div class="toolbar">
          <button id="export-png" class="tool-btn">导出 PNG</button>
          <button id="export-pdf" class="tool-btn">导出 PDF</button>
        </div>
      </div>
      <div class="sub-text">项目：{escape(str(review_input.get('project_path', '-')))}</div>
      <div class="sub-text">扫描时间：{escape(str(review_input.get('scan_timestamp', '-')))}</div>
      <div class="sub-text">主题模式：{escape(theme_mode_label)}</div>
      <div class="summary-grid">
        <div class="summary-card"><span>综合分</span><strong>{float(review_input.get('quantitative_score', 0)):.0f}/100</strong></div>
        <div class="summary-card"><span>评级</span><strong>{escape(str(review_input.get('quantitative_grade', '-')))}</strong></div>
        <div class="summary-card"><span>业务成熟度</span><strong>{business_maturity:.1f}/10</strong></div>
        <div class="summary-card"><span>依赖健康度</span><strong>{dependency_health:.1f}/10</strong></div>
      </div>
    </section>

    <section class="panel charts-grid">
      <div>
        <h2>维度评分图</h2>
        {''.join(bar_rows)}
      </div>
      <div>
        <h2>严重级别分布</h2>
        <div class="donut-wrap">
          <div class="donut"></div>
          <ul class="legend">
            {''.join([f"<li>{escape(item)}</li>" for item in severity_legend])}
          </ul>
        </div>
      </div>
    </section>

    <section class="panel">
      <h2>多维度评估（9 Tab：总览 + 7维 + 业务完成度）</h2>
      <div class="tabs">
        {''.join(tab_buttons)}
      </div>
      {''.join(tab_panels)}
    </section>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/html2canvas@1.4.1/dist/html2canvas.min.js"></script>
  <script>
    const body = document.body;
    const exportPngBtn = document.getElementById('export-png');
    const exportPdfBtn = document.getElementById('export-pdf');

    exportPdfBtn.addEventListener('click', () => {{
      window.print();
    }});

    exportPngBtn.addEventListener('click', async () => {{
      if (typeof window.html2canvas !== 'function') {{
        alert('导出 PNG 失败：html2canvas 加载失败');
        return;
      }}
      const container = document.querySelector('.container');
      if (!container) {{
        return;
      }}
      const originalText = exportPngBtn.textContent;
      exportPngBtn.disabled = true;
      exportPngBtn.textContent = '生成中...';
      try {{
        const canvas = await window.html2canvas(container, {{
          backgroundColor: getComputedStyle(body).backgroundColor,
          scale: 2,
          useCORS: true,
        }});
        const link = document.createElement('a');
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
        link.download = `quantitative-dashboard-${{timestamp}}.png`;
        link.href = canvas.toDataURL('image/png');
        link.click();
      }} catch (error) {{
        console.error(error);
        alert('导出 PNG 失败，请重试');
      }} finally {{
        exportPngBtn.disabled = false;
        exportPngBtn.textContent = originalText;
      }}
    }});

    const tabButtons = document.querySelectorAll('.tab-btn');
    const tabPanels = document.querySelectorAll('.tab-panel');
    tabButtons.forEach((button) => {{
      button.addEventListener('click', () => {{
        const target = button.getAttribute('data-tab');
        tabButtons.forEach((item) => {{
          item.classList.remove('active');
          item.setAttribute('aria-selected', 'false');
        }});
        tabPanels.forEach((panel) => {{
          panel.classList.remove('active');
          panel.setAttribute('hidden', '');
        }});
        button.classList.add('active');
        button.setAttribute('aria-selected', 'true');
        const targetPanel = document.getElementById(`panel-${{target}}`);
        if (targetPanel) {{
          targetPanel.classList.add('active');
          targetPanel.removeAttribute('hidden');
        }}
      }});
    }});
  </script>
</body>
</html>
"""


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
    parser = argparse.ArgumentParser(description="集成 arc:score 数据到 arc:audit")
    parser.add_argument(
        "--score-dir", help="arc:score 输出目录（可选，未提供时尝试自动发现）"
    )
    parser.add_argument("--review-dir", required=True, help="arc:audit 工作目录")
    parser.add_argument(
        "--project-path",
        help="项目根目录（可选：用于元数据；且在未传 --score-dir 时用于 context-hub 自动发现）",
    )

    args = parser.parse_args()

    review_dir = Path(args.review_dir).resolve()
    review_dir.mkdir(parents=True, exist_ok=True)

    project_root: Path | None = None
    if args.project_path:
        project_root = Path(args.project_path).resolve()
    else:
        project_root = _infer_project_root_from_review_dir(review_dir)

    score_dir: Path | None = None
    if args.score_dir:
        score_dir = Path(args.score_dir).resolve()
    elif project_root is not None:
        score_dir = find_score_dir_from_context_hub(project_root)
        if score_dir is None:
            score_dir = project_root / ".arc" / "score" / project_root.name

    if score_dir is None:
        raise SystemExit(
            "未提供 --score-dir，且无法推断项目根目录（请传 --project-path 或 --score-dir）"
        )

    # 加载评分数据
    score_data = load_score_data(str(score_dir))

    # 生成评审输入
    project_path = args.project_path
    if not project_path:
        project_path = (
            (score_data.get("overall_score") or {}).get("project_path")
            or (score_data.get("smell_report") or {}).get("project_path")
            or (score_data.get("bugfix_report") or {}).get("project_path")
        )
    if not project_path:
        project_path = str(project_root or score_dir)
    review_input = generate_review_input(score_data, project_path)

    # 输出到 review 目录
    review_path = review_dir

    # 写入 JSON
    output_path = review_path / "quantitative-input.json"
    output_path.write_text(
        json.dumps(review_input, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    # 写入 Markdown 章节
    md_path = review_path / "quantitative-section.md"
    md_content = generate_quantitative_section(review_input)
    md_path.write_text(md_content, encoding="utf-8")

    # 写入 HTML 可视化看板（图表 + 多维度 Tab）
    # 仅输出一份：按执行时 time now() 自动选择浅/深主题
    html_auto_path = review_path / "quantitative-dashboard.html"
    html_auto_content = generate_html_dashboard(review_input)
    html_auto_path.write_text(html_auto_content, encoding="utf-8")

    print(f"✓ 量化数据已集成")
    print(f"  JSON: {output_path}")
    print(f"  Markdown: {md_path}")
    print(f"  HTML Dashboard: {html_auto_path}")

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
