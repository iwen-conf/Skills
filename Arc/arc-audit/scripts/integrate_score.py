#!/usr/bin/env python3
"""
integrate_score.py - 将 arc:score 量化数据集成到 arc:audit

用法:
    python integrate_score.py --score-dir .arc/score/<project> --review-dir .arc/audit/<project>

或（未传 --score-dir 时尝试从 context-hub 自动发现）:
    python integrate_score.py --project-path <project_root> --review-dir .arc/audit/<project>
"""

import argparse
import json
import re
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
            review_dir.parent.name == "audit"
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
    overall_score = score_data.get("overall_score") or {}
    smell_report = score_data.get("smell_report") or {}
    smell_summary = smell_report.get("summary") or {}

    return {
        "project_path": project_path,
        "scan_timestamp": datetime.now().isoformat(),
        "quantitative_score": overall_score.get("score", 75),
        "quantitative_grade": overall_score.get("grade", "C"),
        "dimension_scores": {
            dim: data["score"] for dim, data in dimension_data.items()
        },
        "dimension_issues": {
            dim: data["issues"] for dim, data in dimension_data.items()
        },
        "smell_summary": {
            "total_violations": smell_summary.get("total_violations", 0),
            "by_severity": smell_summary.get("by_severity", {}),
            "by_category": smell_summary.get("by_category", {}),
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


def _judge_tab_status(
    score: float, critical_count: int, high_count: int
) -> tuple[str, str, str, str]:
    if critical_count > 0 or score < 60:
        return "Fail", "No-Go", "高", "P0（24h）"
    if high_count >= 3 or score < 75:
        return "Concern", "Conditional Go", "中", "P1（72h）"
    return "Pass", "Go", "低", "P2（2周）"


def _top_evidence(issues: list[dict], limit: int = 3) -> str:
    if not issues:
        return "无"
    ranked = sorted(
        issues,
        key=lambda issue: _severity_rank(str(issue.get("severity", ""))),
        reverse=True,
    )[:limit]
    evidence = []
    for issue in ranked:
        issue_id = str(issue.get("id", "-"))
        file_path = str(issue.get("file", "-"))
        line = str(issue.get("line", "-"))
        evidence.append(f"{issue_id}@{file_path}:{line}")
    return "；".join(evidence)


def _safe_read_text(path: Path) -> str | None:
    try:
        if path.exists() and path.is_file():
            return path.read_text(encoding="utf-8")
    except Exception:
        return None
    return None


def _extract_section(markdown: str, heading: str) -> str:
    pattern = re.compile(
        rf"^##\s+{re.escape(heading)}\s*$([\s\S]*?)(?=^##\s+|\Z)",
        re.MULTILINE,
    )
    match = pattern.search(markdown)
    return match.group(1).strip() if match else ""


def _extract_bullets(section: str) -> list[str]:
    bullets: list[str] = []
    for line in section.splitlines():
        stripped = line.strip()
        if stripped.startswith("- ") or stripped.startswith("* "):
            bullets.append(stripped[2:].strip())
    return bullets


def _extract_code_refs(text: str) -> list[str]:
    return re.findall(r"`([^`]+)`", text)


def _extract_numbered_items(section: str) -> list[str]:
    items: list[str] = []
    for line in section.splitlines():
        stripped = line.strip()
        if re.match(r"\d+\.\s+", stripped):
            items.append(re.sub(r"^\d+\.\s+", "", stripped).strip())
    return items


def _meaningful_code_refs(text: str, limit: int = 3) -> list[str]:
    refs: list[str] = []
    for ref in _extract_code_refs(text):
        candidate = ref.strip()
        if not candidate:
            continue
        if "/" in candidate or ":" in candidate or "." in candidate or "=" in candidate:
            refs.append(candidate)
    return refs[:limit]


def _contract_ref_rank(ref: str) -> int:
    candidate = str(ref).strip()
    if not candidate:
        return -999
    if candidate in {"arc:", "SKILL.md", "scripts/", "references/"}:
        return -999

    score = 0
    if re.search(r"[A-Za-z0-9_./-]+:\d+(?:-\d+)?$", candidate):
        score += 80
    if "/" in candidate and "." in candidate:
        score += 40
    if candidate.endswith((".json", ".md", ".py", ".yml", ".yaml")):
        score += 20
    if "=" in candidate:
        score += 12
    if candidate.startswith("arc:") and candidate != "arc:":
        score += 8
    if candidate.endswith("/"):
        score -= 15
    return score


def _select_contract_refs(text: str, limit: int = 4) -> list[str]:
    unique_refs: list[str] = []
    seen: set[str] = set()
    for ref in _extract_code_refs(text):
        candidate = ref.strip()
        if not candidate or candidate in seen:
            continue
        seen.add(candidate)
        unique_refs.append(candidate)

    ranked = sorted(
        unique_refs,
        key=lambda ref: (_contract_ref_rank(ref), -unique_refs.index(ref)),
        reverse=True,
    )
    filtered = [ref for ref in ranked if _contract_ref_rank(ref) > 0]
    return filtered[:limit]


def _business_capability_name(text: str, index: int) -> str:
    raw = _strip_markdown_inline(text)
    if ":" in raw:
        head = raw.split(":", 1)[0].strip()
        if 2 <= len(head) <= 40:
            return head
    if "—" in raw:
        head = raw.split("—", 1)[0].strip()
        if 2 <= len(head) <= 40:
            return head
    words = raw.split()
    if words:
        return " ".join(words[:4]).strip()[:40]
    return f"能力项 {index}"


def _flow_metric_value(metric: dict | None) -> float | None:
    if not metric:
        return None
    headline = str(metric.get("headline", "")).strip()
    match = re.search(r"([\d.]+)\s*%", headline)
    if match:
        return float(match.group(1))
    match = re.search(r"([\d.]+)\s*/\s*10", headline)
    if match:
        return float(match.group(1)) * 10
    return None


def _infer_skill_contract_row(text: str) -> dict:
    summary = _strip_markdown_inline(text)
    refs = _select_contract_refs(text)
    raw_refs = _extract_code_refs(text)
    lowered = summary.lower()
    domain = "技能契约"
    consumer = "下游技能"

    if "routing" in lowered:
        domain = "路由矩阵"
        consumer = "13 个 routed skills"
    elif "context-hub" in lowered:
        domain = "上下文枢纽"
        consumer = "arc:audit / arc:gate"
    elif "producer/consumer" in lowered or "producer_skill" in lowered:
        domain = "生产消费契约"
        consumer = "arc:audit / arc:gate"
    elif "validate" in lowered or "schema" in lowered:
        domain = "校验契约"
        consumer = "arc:audit / arc:gate"
    elif "skill-based architecture" in lowered or "independent skills" in lowered:
        domain = "技能结构约定"
        consumer = "所有 arc:* 技能"

    consumer_refs = [ref for ref in raw_refs if ref.startswith("arc:") and ref != "arc:"]
    if consumer_refs and domain not in {"技能结构约定", "路由矩阵"}:
        consumer = " / ".join(consumer_refs[:2])

    contract = refs[0] if refs else "观测文本"
    validator = refs[1] if len(refs) > 1 else (refs[0] if refs else "-")
    if len(refs) > 2 and domain not in {"技能结构约定", "路由矩阵", "生产消费契约", "上下文枢纽", "校验契约"}:
        consumer = refs[2]

    return {
        "domain": domain,
        "contract": contract,
        "validator": validator,
        "consumer": consumer,
        "summary": summary,
    }


def _strip_markdown_inline(text: str) -> str:
    value = re.sub(r"`([^`]+)`", r"\1", str(text))
    value = value.replace("**", "")
    value = value.replace("__", "")
    return value.strip()


def _extract_special_metric(markdown: str, metric_key: str) -> dict | None:
    pattern = re.compile(
        rf"-\s+\*\*{re.escape(metric_key)}\*\*:\s*([^\n]+)([\s\S]*?)(?=\n-\s+\*\*|\Z)",
        re.MULTILINE,
    )
    match = pattern.search(markdown)
    if not match:
        return None
    return {
        "headline": match.group(1).strip(),
        "details": [line.strip()[2:].strip() for line in match.group(2).splitlines() if line.strip().startswith("- ")],
    }


def _parse_markdown_table(table_text: str) -> list[dict[str, str]]:
    lines = [line.strip() for line in table_text.splitlines() if line.strip()]
    header_index = next((idx for idx, line in enumerate(lines) if line.startswith("|") and "|" in line[1:]), -1)
    if header_index < 0 or len(lines) < header_index + 3:
        return []
    headers = [cell.strip() for cell in lines[header_index].strip("|").split("|")]
    rows: list[dict[str, str]] = []
    for line in lines[header_index + 2:]:
        if not line.startswith("|"):
            break
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if len(cells) != len(headers):
            continue
        rows.append(dict(zip(headers, cells)))
    return rows


def _parse_scorecard(scorecard_text: str | None) -> dict:
    parsed = {
        "dimension_rows": [],
        "special_rows": [],
        "overall_score_10": None,
        "overall_score_100": None,
    }
    if not scorecard_text:
        return parsed

    sections = scorecard_text.split("## ")
    for raw in sections:
        section = raw.strip()
        if section.startswith("Seven Dimensions Score"):
            parsed["dimension_rows"] = _parse_markdown_table(section)
        elif section.startswith("Special Scoring"):
            parsed["special_rows"] = _parse_markdown_table(section)

    final_scores: list[float] = []
    for row in parsed["dimension_rows"]:
        final_value = row.get("Final", "").split("/")[0].strip()
        try:
            final_scores.append(float(final_value))
        except Exception:
            continue

    if final_scores:
        avg_10 = round(sum(final_scores) / len(final_scores), 1)
        parsed["overall_score_10"] = avg_10
        parsed["overall_score_100"] = round(avg_10 * 10, 1)

    return parsed


def _parse_recommendations(recommendations_text: str | None) -> list[dict]:
    if not recommendations_text:
        return []

    items: list[dict] = []
    current_priority: str | None = None
    current_label: str | None = None

    for line in recommendations_text.splitlines():
        stripped = line.strip()
        if stripped.startswith("## "):
            heading = stripped[3:]
            match = re.match(r"(P\d)\s*-\s*(.+)", heading)
            if match:
                current_priority = match.group(1)
                current_label = match.group(2).strip()
            continue
        if not stripped.startswith("- [ ] "):
            continue

        body = stripped[6:].strip()
        dimension_match = re.search(r"dimension:\s*([^,]+)", body)
        evidence_match = re.search(r"evidence:\s*(.+)$", body)
        task_text = re.sub(r"\s+—\s+dimension:.*$", "", body).strip()
        task_text = re.sub(r"\s+-\s+dimension:.*$", "", task_text).strip()

        items.append(
            {
                "priority": current_priority or "P2",
                "priority_label": current_label or "未分类",
                "task": task_text,
                "dimension": (dimension_match.group(1).strip() if dimension_match else "未知"),
                "evidence": (evidence_match.group(1).strip() if evidence_match else "-"),
            }
        )

    return items


def _dimension_key_from_label(label: str) -> str | None:
    normalized = str(label).strip().lower()
    mapping = {
        "architecture design": "architecture",
        "security compliance": "security",
        "code quality": "code-quality",
        "business value": "business",
        "devops": "devops",
        "team collaboration": "team",
        "technical debt": "tech-debt",
        "架构设计": "architecture",
        "安全合规": "security",
        "代码质量": "code-quality",
        "业务价值": "business",
        "运维交付": "devops",
        "团队协作": "team",
        "技术债务": "tech-debt",
    }
    return mapping.get(normalized)


def _scorecard_dimension_scores(scorecard: dict) -> dict[str, float]:
    scores: dict[str, float] = {}
    for row in scorecard.get("dimension_rows", []):
        dim_key = _dimension_key_from_label(row.get("Dimension", ""))
        if not dim_key:
            continue
        try:
            scores[dim_key] = float(str(row.get("Final", "")).split("/")[0].strip()) * 10
        except Exception:
            continue
    return scores


def _scorecard_special_scores(scorecard: dict) -> dict[str, float]:
    scores: dict[str, float] = {}
    for row in scorecard.get("special_rows", []):
        index_name = str(row.get("Index", "")).strip().lower()
        score_value = str(row.get("Score", "")).split("/")[0].strip()
        try:
            numeric = float(score_value)
        except Exception:
            continue
        if "business maturity" in index_name:
            scores["business_maturity"] = numeric
        elif "dependency health" in index_name:
            scores["dependency_health"] = numeric
    return scores


def _map_dimension_title(title: str) -> str | None:
    normalized = str(title).strip().lower()
    if "architecture" in normalized or "架构" in normalized:
        return "architecture"
    if "security" in normalized or "安全" in normalized:
        return "security"
    if "code quality" in normalized or "代码质量" in normalized:
        return "code-quality"
    if "business" in normalized or "业务" in normalized:
        return "business"
    if "devops" in normalized or "运维" in normalized or "delivery operations" in normalized:
        return "devops"
    if "team" in normalized or "协作" in normalized:
        return "team"
    if "technical debt" in normalized or "技术债" in normalized:
        return "tech-debt"
    return None


def _parse_dimension_sections(diagnostic_text: str | None) -> dict[str, dict]:
    if not diagnostic_text:
        return {}

    pattern = re.compile(
        r"^##\s+Dimension\s+\d+:\s+(.+?)\s+\(([\d.]+)/10\)\s*$([\s\S]*?)(?=^##\s+Dimension\s+\d+:|^##\s+Methodology|\Z)",
        re.MULTILINE,
    )
    sections: dict[str, dict] = {}
    for match in pattern.finditer(diagnostic_text):
        title = match.group(1).strip()
        key = _map_dimension_title(title)
        if not key:
            continue
        body = match.group(3).strip()
        sections[key] = {
            "title": title,
            "score_10": float(match.group(2)),
            "observations": _extract_bullets(_extract_section(body, "Observations")),
            "analysis": _extract_section(body, "Analysis"),
            "suggestions": _extract_bullets(_extract_section(body, "Suggestions")),
            "special_scores": _extract_bullets(_extract_section(body, "Special Scores")),
        }
    return sections


def _priority_rank(priority: str) -> int:
    return {"P0": 4, "P1": 3, "P2": 2, "P3": 1}.get(str(priority).upper(), 0)


def _priority_class(priority: str) -> str:
    value = str(priority).upper()
    return {
        "P0": "p0",
        "P1": "p1",
        "P2": "p2",
        "P3": "p2",
    }.get(value, "p2")


def _score_css(score: float) -> str:
    if score >= 75:
        return "ok"
    if score >= 50:
        return "warn"
    return "danger"


def _provenance_label(kind: str) -> str:
    labels = {
        "observed": "Observed",
        "derived": "Derived",
        "heuristic": "Heuristic",
        "na": "N/A",
    }
    return labels.get(kind, kind)


def _display_verdict(verdict: str) -> str:
    mapping = {
        "Pass": "通过",
        "Concern": "需关注",
        "Fail": "阻断",
    }
    return mapping.get(str(verdict), str(verdict))


def _display_gate(gate: str) -> str:
    mapping = {
        "Go": "Go（可推进）",
        "Conditional Go": "Conditional Go（有条件推进）",
        "No-Go": "No-Go（暂不推进）",
    }
    return mapping.get(str(gate), str(gate))


def _display_rating(rating: str) -> str:
    mapping = {
        "excellence": "卓越",
        "good": "良好",
        "qualified": "合格",
        "warn": "警告",
        "danger": "危险",
    }
    return mapping.get(str(rating).lower(), str(rating))


def _translate_priority_label(label: str) -> str:
    mapping = {
        "Fix Now (blocking)": "立即修复（阻断）",
        "Short-term improvement (1-2 iterations)": "短期改进（1-2 迭代）",
        "Medium-term (quarterly)": "中期治理（季度）",
        "Long-term (6 months+)": "长期规划（6个月+）",
    }
    return mapping.get(str(label), str(label))


def _display_dimension_name(label: str) -> str:
    dim_key = _dimension_key_from_label(label)
    if dim_key:
        return _dimension_label(dim_key)
    return str(label)


def _build_review_context(review_input: dict) -> dict:
    review_dir_raw = review_input.get("review_dir")
    review_dir = Path(str(review_dir_raw)).resolve() if review_dir_raw else None

    scorecard_text = _safe_read_text(review_dir / "scorecard.md") if review_dir else None
    recommendations_text = _safe_read_text(review_dir / "recommendations.md") if review_dir else None
    diagnostic_text = _safe_read_text(review_dir / "diagnostic-report.md") if review_dir else None
    business_value_text = (
        _safe_read_text(review_dir / "deep-business" / "dim-4-business-value.md")
        if review_dir
        else None
    )
    architecture_text = (
        _safe_read_text(review_dir / "deep" / "dim-1-architecture.md") if review_dir else None
    )
    business_maturity_text = (
        _safe_read_text(review_dir / "business-maturity.md") if review_dir else None
    )
    dependency_health_text = (
        _safe_read_text(review_dir / "dependency-health.md") if review_dir else None
    )

    scorecard = _parse_scorecard(scorecard_text)
    recommendations = _parse_recommendations(recommendations_text)
    dimension_sections = _parse_dimension_sections(diagnostic_text)

    return {
        "scorecard": scorecard,
        "recommendations": recommendations,
        "executive_summary": _extract_bullets(_extract_section(diagnostic_text or "", "Executive Summary")),
        "executive_risks": _extract_numbered_items(_extract_section(diagnostic_text or "", "Executive Summary")),
        "business_observations": _extract_bullets(_extract_section(business_value_text or "", "Observations")),
        "business_analysis": _extract_section(business_value_text or "", "Analysis"),
        "architecture_observations": _extract_bullets(_extract_section(architecture_text or "", "Observations")),
        "architecture_analysis": _extract_section(architecture_text or "", "Analysis"),
        "business_special": _extract_special_metric(business_value_text or "", "business_maturity"),
        "dependency_special": _extract_special_metric(diagnostic_text or "", "dependency_health"),
        "dimension_sections": dimension_sections,
        "has_business_maturity_artifact": bool(business_maturity_text),
        "has_dependency_health_artifact": bool(dependency_health_text),
    }


def _resolve_display_summary(review_input: dict) -> dict:
    dimension_order = [
        "architecture",
        "security",
        "code-quality",
        "business",
        "devops",
        "team",
        "tech-debt",
    ]
    review_ctx = _build_review_context(review_input)
    scorecard = review_ctx.get("scorecard", {})
    scorecard_dim_scores = _scorecard_dimension_scores(scorecard)
    scorecard_special = _scorecard_special_scores(scorecard)
    dimension_scores = {
        dim: float(scorecard_dim_scores.get(dim, review_input.get("dimension_scores", {}).get(dim, 0)))
        for dim in dimension_order
    }
    overall_score = float(scorecard.get("overall_score_100") or review_input.get("quantitative_score", 0))
    business_maturity = float(scorecard_special.get("business_maturity", review_input.get("special_scores", {}).get("business_maturity", 0)))
    dependency_health = float(scorecard_special.get("dependency_health", review_input.get("special_scores", {}).get("dependency_health", 0)))
    issues_by_dim = review_input.get("dimension_issues", {})
    all_issues = [issue for dim in dimension_order for issue in issues_by_dim.get(dim, []) if isinstance(issue, dict)]
    critical_count = sum(1 for issue in all_issues if str(issue.get("severity", "")).lower() == "critical")
    high_count = sum(1 for issue in all_issues if str(issue.get("severity", "")).lower() == "high")
    verdict, gate, risk, sla = _judge_tab_status(overall_score, critical_count, high_count)
    return {
        "review_ctx": review_ctx,
        "dimension_scores": dimension_scores,
        "overall_score": overall_score,
        "overall_rating": _score_level(overall_score),
        "business_maturity": business_maturity,
        "dependency_health": dependency_health,
        "verdict": verdict,
        "gate": gate,
        "risk": risk,
        "sla": sla,
        "uses_scorecard": bool(scorecard.get("dimension_rows")),
    }


def generate_html_dashboard(review_input: dict, theme: str | None = None) -> str:
    """Generate HTML dashboard following SPA 9-Tab Standard (1 overview + 4 business + 4 technical)."""
    return _generate_html_dashboard_v2(review_input, theme)

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
    special_scores = review_input.get("special_scores", {})
    business_maturity = float(special_scores.get("business_maturity", 0))
    dependency_health = float(special_scores.get("dependency_health", 0))
    selected_theme = theme if theme in {"light", "dark"} else _theme_from_now()
    theme_mode_label = (
        "浅色（基于 time now()）"
        if selected_theme == "light"
        else "深色（基于 time now()）"
    )

    # Collect all issues per dimension with normalization
    all_issues: list[dict] = []
    dim_issue_map: dict[str, list[dict]] = {}
    dim_severity_counts: dict[str, dict[str, int]] = {}
    for dk in dimension_order:
        raw = dimension_issues.get(dk, [])
        normalized = [{**i, "dimension": dk} for i in raw if isinstance(i, dict)]
        dim_issue_map[dk] = normalized
        all_issues.extend(normalized)
        counts = {"critical": 0, "high": 0, "medium": 0, "low": 0}
        for i in normalized:
            s = str(i.get("severity", "")).lower()
            if s in counts:
                counts[s] += 1
        dim_severity_counts[dk] = counts

    total_issues = len(all_issues)
    total_critical = sum(c["critical"] for c in dim_severity_counts.values())
    total_high = sum(c["high"] for c in dim_severity_counts.values())

    overall_score = float(review_input.get("quantitative_score", 0))
    overall_grade = str(review_input.get("quantitative_grade", "-"))

    # ---------- Tab builder ----------
    tab_buttons: list[str] = []
    tab_panels: list[str] = []

    def append_tab(title: str, content_html: str) -> None:
        idx = len(tab_buttons)
        tid = f"tab-{idx}"
        pid = f"panel-{idx}"
        active = idx == 0
        tab_buttons.append(
            f'<button type="button" class="tab-btn {"active" if active else ""}" '
            f'role="tab" id="{tid}" aria-selected="{"true" if active else "false"}" '
            f'aria-controls="{pid}" data-tab="{idx}">{escape(title)}</button>'
        )
        tab_panels.append(
            f'<section class="tab-panel {"active" if active else ""}" role="tabpanel" '
            f'id="{pid}" aria-labelledby="{tid}" {"" if active else "hidden"}>'
            f'{content_html}</section>'
        )

    def _expert_card(dims: list[str]) -> str:
        issues = []
        for d in dims:
            issues.extend(dim_issue_map.get(d, []))
        score_avg = sum(float(dimension_scores.get(d, 0)) for d in dims) / max(len(dims), 1)
        cc = sum(dim_severity_counts.get(d, {}).get("critical", 0) for d in dims)
        hc = sum(dim_severity_counts.get(d, {}).get("high", 0) for d in dims)
        verdict, gate, risk, sla = _judge_tab_status(score_avg, cc, hc)
        evidence = _top_evidence(issues, limit=3)
        return f"""
<h3>专家评审卡</h3>
<table class="issues-table">
  <thead><tr><th>项</th><th>结论</th></tr></thead>
  <tbody>
    <tr><td>审核结论</td><td>{verdict}</td></tr>
    <tr><td>Gate 建议</td><td>{gate}</td></tr>
    <tr><td>风险等级</td><td>{risk}</td></tr>
    <tr><td>整改时限</td><td>{sla}</td></tr>
    <tr><td>关键证据</td><td>{escape(evidence)}</td></tr>
  </tbody>
</table>"""

    def _dim_kpi(dims: list[str]) -> str:
        issues = []
        for d in dims:
            issues.extend(dim_issue_map.get(d, []))
        score_avg = sum(float(dimension_scores.get(d, 0)) for d in dims) / max(len(dims), 1)
        cc = sum(dim_severity_counts.get(d, {}).get("critical", 0) for d in dims)
        hc = sum(dim_severity_counts.get(d, {}).get("high", 0) for d in dims)
        ch = cc + hc
        return f"""
<div class="panel-kpi">
  <div class="kpi-box"><span>评分</span><strong>{score_avg:.0f}/100</strong></div>
  <div class="kpi-box"><span>评级</span><strong>{escape(_score_level(score_avg))}</strong></div>
  <div class="kpi-box"><span>问题数</span><strong>{len(issues)}</strong></div>
  <div class="kpi-box"><span>高风险(C+H)</span><strong>{ch}</strong></div>
</div>"""

    def _issue_detail(dims: list[str], include_dim: bool = False) -> str:
        issues = []
        for d in dims:
            issues.extend(dim_issue_map.get(d, []))
        rows = _issue_rows_html(
            sorted(issues, key=lambda i: _severity_rank(str(i.get("severity", ""))), reverse=True),
            include_dimension=include_dim,
            empty_text="暂无相关问题",
            max_rows=30,
        )
        hdr = "<th>来源维度</th>" if include_dim else ""
        return f"""
<h3>问题明细</h3>
<table class="issues-table">
  <thead><tr>{hdr}<th>ID</th><th>严重级别</th><th>文件</th><th>行</th><th>描述</th></tr></thead>
  <tbody>{rows}</tbody>
</table>"""

    # ===== Tab 1: Executive Overview =====
    radar_labels = [escape(_dimension_label(d)) for d in dimension_order]
    radar_values = [float(dimension_scores.get(d, 0)) for d in dimension_order]
    radar_labels_js = json.dumps(radar_labels, ensure_ascii=False)
    radar_values_js = json.dumps(radar_values)

    dim_summary_rows = "".join(
        f"<tr><td>{escape(_dimension_label(d))}</td>"
        f"<td>{float(dimension_scores.get(d, 0)):.0f}/100</td>"
        f"<td>{len(dim_issue_map.get(d, []))}</td></tr>"
        for d in dimension_order
    )
    sev_summary_rows = "".join(
        f"<tr><td>{escape(s)}</td><td>{int(by_severity.get(s, 0))}</td></tr>"
        for s in ["critical", "high", "medium", "low"]
    )
    overall_verdict, overall_gate, overall_risk, overall_sla = _judge_tab_status(
        overall_score, total_critical, total_high
    )
    overall_evidence = _top_evidence(all_issues, limit=5)

    overview_html = f"""
<div class="panel-kpi">
  <div class="kpi-box"><span>综合分 (Apdex)</span><strong>{overall_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>总问题数</span><strong>{total_issues}</strong></div>
  <div class="kpi-box"><span>业务成熟度</span><strong>{business_maturity:.1f}/10</strong></div>
  <div class="kpi-box"><span>依赖健康度</span><strong>{dependency_health:.1f}/10</strong></div>
</div>
<h3>专家总评卡</h3>
<table class="issues-table">
  <thead><tr><th>项</th><th>结论</th></tr></thead>
  <tbody>
    <tr><td>总评结论</td><td>{overall_verdict}</td></tr>
    <tr><td>最终 Gate 建议</td><td>{overall_gate}</td></tr>
    <tr><td>整体风险等级</td><td>{overall_risk}</td></tr>
    <tr><td>全局整改时限</td><td>{overall_sla}</td></tr>
    <tr><td>关键证据</td><td>{escape(overall_evidence)}</td></tr>
  </tbody>
</table>
<div class="charts-row">
  <div class="chart-box"><div id="chart-radar" style="width:100%;height:360px;"></div></div>
  <div class="chart-box"><div id="chart-gauge" style="width:100%;height:360px;"></div></div>
</div>
<h3>七维摘要</h3>
<table class="issues-table">
  <thead><tr><th>维度</th><th>评分</th><th>问题数</th></tr></thead>
  <tbody>{dim_summary_rows}</tbody>
</table>
<h3>严重级别分布</h3>
<table class="issues-table">
  <thead><tr><th>严重级别</th><th>数量</th></tr></thead>
  <tbody>{sev_summary_rows}</tbody>
</table>"""
    append_tab("全局总览", overview_html)

    # ===== Tab 2: Business Completion =====
    biz_score = float(dimension_scores.get("business", 0))
    team_score = float(dimension_scores.get("team", 0))
    bm_level = "高" if business_maturity >= 8.5 else ("中" if business_maturity >= 6.5 else "低")
    biz_completion_html = f"""
{_dim_kpi(["business", "team"])}
<div class="panel-kpi">
  <div class="kpi-box"><span>业务成熟度</span><strong>{business_maturity:.1f}/10 ({bm_level})</strong></div>
  <div class="kpi-box"><span>业务维度</span><strong>{biz_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>团队效能</span><strong>{team_score:.0f}/100</strong></div>
</div>
{_expert_card(["business", "team"])}
<div id="chart-completion" style="width:100%;height:320px;"></div>
{_issue_detail(["business", "team"], include_dim=True)}"""
    append_tab("业务完成情况", biz_completion_html)

    # ===== Tab 3: Business Connection =====
    arch_score = float(dimension_scores.get("architecture", 0))
    td_score = float(dimension_scores.get("tech-debt", 0))
    arch_issues = dim_issue_map.get("architecture", [])
    dep_issues = [i for i in arch_issues if any(
        kw in " ".join([str(i.get("id", "")), str(i.get("message", ""))]).lower()
        for kw in ["depend", "import", "couple", "依赖", "耦合", "循环", "cycle"]
    )]
    conn_html = f"""
{_dim_kpi(["architecture", "tech-debt"])}
<div class="panel-kpi">
  <div class="kpi-box"><span>架构评分</span><strong>{arch_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>技术债评分</span><strong>{td_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>依赖相关问题</span><strong>{len(dep_issues)}</strong></div>
</div>
{_expert_card(["architecture", "tech-debt"])}
<div id="chart-topology" style="width:100%;height:360px;"></div>
{_issue_detail(["architecture"], include_dim=False)}"""
    append_tab("业务连接情况", conn_html)

    # ===== Tab 4: Business Connectivity =====
    business_issues = dim_issue_map.get("business", [])
    devops_issues = dim_issue_map.get("devops", [])
    business_keywords_bug = ["bug", "错误", "异常", "失败", "缺陷", "fault", "defect"]
    business_keywords_logic = ["逻辑", "流程", "链路", "状态", "规则", "不一致",
                               "workflow", "state", "rule", "condition"]
    bug_count = sum(1 for i in business_issues if any(
        kw in " ".join([str(i.get("id", "")), str(i.get("message", ""))]).lower()
        for kw in business_keywords_bug
    ))
    logic_count = sum(1 for i in business_issues if any(
        kw in " ".join([str(i.get("id", "")), str(i.get("message", ""))]).lower()
        for kw in business_keywords_logic
    ))
    biz_critical = dim_severity_counts.get("business", {}).get("critical", 0)
    biz_high = dim_severity_counts.get("business", {}).get("high", 0)
    flow_score = int(max(0, min(100, round(
        biz_score - biz_critical * 12 - biz_high * 6 - bug_count * 3 - logic_count * 2
    ))))
    flow_status = "通畅" if flow_score >= 85 else ("基本通畅" if flow_score >= 65 else ("存在断点" if flow_score >= 45 else "阻塞明显"))

    chain_signals = [
        ("需求到规则一致性", logic_count + bug_count),
        ("服务编排链路", sum(1 for i in all_issues if str(i.get("dimension", "")) in {"architecture", "devops"} and _is_business_related_issue(i))),
        ("数据状态一致性", sum(1 for i in business_issues if any(
            t in " ".join([str(i.get("id", "")), str(i.get("message", ""))]).lower()
            for t in ["state", "status", "db", "database", "transaction", "状态", "一致", "数据"]
        ))),
        ("异常与回滚闭环", biz_critical + biz_high),
    ]
    chain_rows = ""
    for cn, sc in chain_signals:
        cs = "通畅" if sc <= 0 else ("需关注" if sc <= 2 else "存在断点")
        chain_rows += f"<tr><td>{escape(cn)}</td><td>{sc}</td><td>{cs}</td></tr>"

    connectivity_html = f"""
{_dim_kpi(["business", "devops"])}
<div class="panel-kpi">
  <div class="kpi-box"><span>链路通畅度</span><strong>{flow_score}/100 ({flow_status})</strong></div>
  <div class="kpi-box"><span>Bug 信号</span><strong>{bug_count}</strong></div>
  <div class="kpi-box"><span>逻辑不通顺</span><strong>{logic_count}</strong></div>
</div>
{_expert_card(["business", "devops"])}
<div id="chart-sankey" style="width:100%;height:320px;"></div>
<h3>业务链路分段评估</h3>
<table class="issues-table">
  <thead><tr><th>链路段</th><th>信号数</th><th>状态</th></tr></thead>
  <tbody>{chain_rows}</tbody>
</table>
{_issue_detail(["business"], include_dim=False)}"""
    append_tab("业务连通率", connectivity_html)

    # ===== Tab 5: Business Logic Fluency =====
    cq_score = float(dimension_scores.get("code-quality", 0))
    logic_issues = [i for i in business_issues if any(
        kw in " ".join([str(i.get("id", "")), str(i.get("message", ""))]).lower()
        for kw in business_keywords_logic
    )]
    fluency_html = f"""
{_dim_kpi(["business", "code-quality"])}
<div class="panel-kpi">
  <div class="kpi-box"><span>业务逻辑问题</span><strong>{logic_count}</strong></div>
  <div class="kpi-box"><span>代码质量评分</span><strong>{cq_score:.0f}/100</strong></div>
</div>
{_expert_card(["business", "code-quality"])}
<div id="chart-funnel" style="width:100%;height:320px;"></div>
{_issue_detail(["business", "code-quality"], include_dim=True)}"""
    append_tab("业务逻辑通顺性", fluency_html)

    # ===== Tab 6: Architecture Health =====
    arch_dims = ["architecture", "code-quality", "tech-debt"]
    arch_health_html = f"""
{_dim_kpi(arch_dims)}
<div class="panel-kpi">
  <div class="kpi-box"><span>架构</span><strong>{arch_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>代码质量</span><strong>{cq_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>技术债</span><strong>{td_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>依赖健康度</span><strong>{dependency_health:.1f}/10</strong></div>
</div>
{_expert_card(arch_dims)}
<div class="charts-row">
  <div class="chart-box"><div id="chart-dep-matrix" style="width:100%;height:320px;"></div></div>
  <div class="chart-box"><div id="chart-polar" style="width:100%;height:320px;"></div></div>
</div>
{_issue_detail(arch_dims, include_dim=True)}"""
    append_tab("架构健康度", arch_health_html)

    # ===== Tab 7: Performance & Stability =====
    devops_score = float(dimension_scores.get("devops", 0))
    perf_html = f"""
{_dim_kpi(["devops", "code-quality"])}
<div class="panel-kpi">
  <div class="kpi-box"><span>运维交付</span><strong>{devops_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>代码质量</span><strong>{cq_score:.0f}/100</strong></div>
</div>
{_expert_card(["devops", "code-quality"])}
<div id="chart-perf" style="width:100%;height:320px;"></div>
{_issue_detail(["devops", "code-quality"], include_dim=True)}"""
    append_tab("性能与稳定性", perf_html)

    # ===== Tab 8: Security & Governance =====
    sec_score = float(dimension_scores.get("security", 0))
    sec_issues = dim_issue_map.get("security", [])
    sec_cats: dict[str, int] = {}
    for i in sec_issues:
        cat = str(i.get("id", "other")).split("-")[0] if i.get("id") else "other"
        sec_cats[cat] = sec_cats.get(cat, 0) + 1
    sec_cat_data_js = json.dumps(
        [{"name": k, "value": v} for k, v in sorted(sec_cats.items(), key=lambda x: x[1], reverse=True)],
        ensure_ascii=False,
    )
    sec_html = f"""
{_dim_kpi(["security", "architecture"])}
<div class="panel-kpi">
  <div class="kpi-box"><span>安全评分</span><strong>{sec_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>架构评分</span><strong>{arch_score:.0f}/100</strong></div>
</div>
{_expert_card(["security", "architecture"])}
<div id="chart-rose" style="width:100%;height:360px;"></div>
{_issue_detail(["security"], include_dim=False)}"""
    append_tab("安全治理与合规", sec_html)

    # ===== Tab 9: Resource & FinOps =====
    res_dims = ["devops", "tech-debt", "team"]
    res_treemap_data: list[dict] = []
    for d in res_dims:
        children = []
        for sev in ["critical", "high", "medium", "low"]:
            c = dim_severity_counts.get(d, {}).get(sev, 0)
            if c > 0:
                children.append({"name": sev, "value": c})
        if children:
            res_treemap_data.append({"name": _dimension_label(d), "children": children})
    res_treemap_js = json.dumps(res_treemap_data, ensure_ascii=False)

    res_html = f"""
{_dim_kpi(res_dims)}
<div class="panel-kpi">
  <div class="kpi-box"><span>运维</span><strong>{devops_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>技术债</span><strong>{td_score:.0f}/100</strong></div>
  <div class="kpi-box"><span>团队</span><strong>{float(dimension_scores.get('team', 0)):.0f}/100</strong></div>
</div>
{_expert_card(res_dims)}
<div id="chart-treemap" style="width:100%;height:360px;"></div>
{_issue_detail(res_dims, include_dim=True)}"""
    append_tab("资源利用与成本", res_html)

    # ===== Assemble HTML =====
    gradient_style, severity_legend = _severity_style(by_severity)
    bar_rows = ""
    for d in dimension_order:
        s = float(dimension_scores.get(d, 0))
        bar_rows += f"""
<div class="bar-row">
  <div class="bar-label">{escape(_dimension_label(d))}</div>
  <div class="bar-track"><div class="bar-fill" style="width:{max(0, min(s, 100)):.1f}%;"></div></div>
  <div class="bar-value">{s:.0f}</div>
</div>"""

    # Sankey data from chain signals
    sankey_nodes = json.dumps([{"name": cn} for cn, _ in chain_signals] + [{"name": "业务总链路"}], ensure_ascii=False)
    sankey_links = json.dumps([{"source": cn, "target": "业务总链路", "value": max(sc, 1)} for cn, sc in chain_signals], ensure_ascii=False)

    # Funnel data
    funnel_steps = [
        {"name": "需求输入", "value": 100},
        {"name": "规则匹配", "value": max(10, 100 - logic_count * 8)},
        {"name": "状态流转", "value": max(10, 100 - logic_count * 8 - bug_count * 5)},
        {"name": "业务闭环", "value": flow_score},
    ]
    funnel_js = json.dumps(funnel_steps, ensure_ascii=False)

    # Polar bar data for architecture health
    polar_data_js = json.dumps([
        {"name": _dimension_label(d), "value": float(dimension_scores.get(d, 0))}
        for d in arch_dims
    ], ensure_ascii=False)

    # Performance bar data
    perf_data_js = json.dumps([
        {"name": _dimension_label(d), "value": float(dimension_scores.get(d, 0))}
        for d in ["devops", "code-quality"]
    ], ensure_ascii=False)

    # Completion bar data
    completion_data_js = json.dumps([
        {"name": _dimension_label(d), "value": float(dimension_scores.get(d, 0))}
        for d in ["business", "team"]
    ] + [{"name": "业务成熟度(×10)", "value": business_maturity * 10}], ensure_ascii=False)

    return f"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>SPA 项目评估大盘</title>
  <script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
  <style>
    :root {{
      --bg: #f8fafc; --panel: #ffffff; --text: #0f172a; --muted: #475569;
      --primary: #3b82f6; --border: #e2e8f0;
      --good: #52C41A; --warn: #FAAD14; --bad: #F5222D; --offline: #BFBFBF;
      --chip: #f1f5f9; --chip-active: #dbeafe;
      --chip-active-border: #93c5fd; --chip-active-text: #1d4ed8;
    }}
    [data-theme="dark"] {{
      --bg: #0b1220; --panel: #111827; --text: #e5e7eb; --muted: #9ca3af;
      --border: #2b3548; --primary: #60a5fa;
      --good: #34d399; --warn: #fbbf24; --bad: #f87171; --offline: #6b7280;
      --chip: #1f2937; --chip-active: #1e3a8a;
      --chip-active-border: #3b82f6; --chip-active-text: #dbeafe;
    }}
    body {{ margin:0; padding:24px; font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif; color:var(--text); background:var(--bg); }}
    .container {{ max-width:1400px; margin:0 auto; }}
    .header,.panel {{ background:var(--panel); border:1px solid var(--border); border-radius:12px; padding:18px; margin-bottom:16px; box-shadow:0 4px 16px rgba(15,23,42,.04); }}
    h1,h2 {{ margin:0 0 10px; font-weight:700; }}
    h3 {{ margin:14px 0 8px; font-size:15px; font-weight:700; }}
    .header-top {{ display:flex; justify-content:space-between; align-items:center; gap:10px; flex-wrap:wrap; }}
    .toolbar {{ display:flex; gap:8px; flex-wrap:wrap; }}
    .tool-btn {{ border:1px solid var(--border); background:var(--chip); color:var(--text); border-radius:8px; padding:8px 10px; font-size:12px; cursor:pointer; }}
    .tool-btn:hover {{ border-color:var(--primary); }}
    .sub-text {{ color:var(--muted); font-size:14px; }}
    .summary-grid {{ display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:12px; margin-top:12px; }}
    .summary-card {{ border:1px solid var(--border); border-radius:10px; padding:12px; background:color-mix(in srgb,var(--panel) 80%,var(--bg) 20%); }}
    .summary-card span {{ display:block; color:var(--muted); font-size:12px; margin-bottom:6px; }}
    .summary-card strong {{ font-size:20px; }}
    .charts-grid {{ display:grid; grid-template-columns:2fr 1fr; gap:14px; }}
    .charts-row {{ display:grid; grid-template-columns:1fr 1fr; gap:14px; margin:12px 0; }}
    .chart-box {{ min-height:200px; }}
    .bar-row {{ display:grid; grid-template-columns:110px 1fr 36px; align-items:center; gap:10px; margin:10px 0; }}
    .bar-label {{ font-size:13px; color:var(--muted); }}
    .bar-track {{ height:10px; border-radius:999px; background:color-mix(in srgb,var(--border) 60%,var(--panel) 40%); overflow:hidden; }}
    .bar-fill {{ height:100%; border-radius:999px; background:linear-gradient(90deg,#60a5fa 0%,#2563eb 100%); }}
    .bar-value {{ text-align:right; font-weight:600; font-size:13px; }}
    .donut-wrap {{ display:flex; align-items:center; gap:12px; margin-top:10px; }}
    .donut {{ width:120px; height:120px; border-radius:50%; background:{gradient_style}; position:relative; }}
    .donut::after {{ content:""; position:absolute; inset:22px; background:var(--panel); border-radius:50%; }}
    .legend {{ margin:0; padding-left:18px; color:var(--muted); font-size:13px; }}
    .tabs {{ display:flex; flex-wrap:wrap; gap:8px; margin-bottom:12px; }}
    .tab-btn {{ border:1px solid var(--border); background:var(--chip); color:var(--text); border-radius:8px; padding:8px 12px; cursor:pointer; font-size:13px; }}
    .tab-btn.active {{ background:var(--chip-active); border-color:var(--chip-active-border); color:var(--chip-active-text); font-weight:600; }}
    .tab-panel {{ display:none; }}
    .tab-panel.active {{ display:block; }}
    .panel-kpi {{ display:grid; grid-template-columns:repeat(auto-fit,minmax(120px,1fr)); gap:10px; margin-bottom:12px; }}
    .kpi-box {{ border:1px solid var(--border); border-radius:8px; padding:10px; background:color-mix(in srgb,var(--panel) 82%,var(--bg) 18%); }}
    .kpi-box span {{ display:block; color:var(--muted); font-size:12px; margin-bottom:4px; }}
    .kpi-box strong {{ font-size:18px; }}
    .issues-table {{ width:100%; border-collapse:collapse; font-size:13px; }}
    .issues-table th,.issues-table td {{ border:1px solid var(--border); padding:8px; text-align:left; vertical-align:top; }}
    .issues-table th {{ background:color-mix(in srgb,var(--panel) 76%,var(--bg) 24%); }}
    .empty-cell {{ text-align:center; color:var(--muted); font-style:italic; }}
    @media (max-width:920px) {{ .charts-grid,.charts-row {{ grid-template-columns:1fr; }} }}
    @media print {{ body {{ padding:0; }} .toolbar,.tabs {{ display:none !important; }} .header,.panel {{ box-shadow:none; break-inside:avoid; }} .tab-panel {{ display:block !important; margin-bottom:18px; }} }}
  </style>
</head>
<body data-theme="{selected_theme}">
  <div class="container">
    <section class="header">
      <div class="header-top">
        <h1>SPA 项目评估大盘</h1>
        <div class="toolbar">
          <button id="export-png" class="tool-btn">导出 PNG</button>
          <button id="export-pdf" class="tool-btn">导出 PDF</button>
        </div>
      </div>
      <div class="sub-text">项目：{escape(str(review_input.get('project_path', '-')))}</div>
      <div class="sub-text">扫描时间：{escape(str(review_input.get('scan_timestamp', '-')))}</div>
      <div class="sub-text">主题：{escape(theme_mode_label)} | 9 Tab（1 全局 + 4 业务 + 4 技术）</div>
      <div class="summary-grid">
        <div class="summary-card"><span>综合分</span><strong>{overall_score:.0f}/100</strong></div>
        <div class="summary-card"><span>评级</span><strong>{escape(overall_grade)}</strong></div>
        <div class="summary-card"><span>业务成熟度</span><strong>{business_maturity:.1f}/10</strong></div>
        <div class="summary-card"><span>依赖健康度</span><strong>{dependency_health:.1f}/10</strong></div>
      </div>
    </section>

    <section class="panel charts-grid">
      <div>
        <h2>维度评分图</h2>
        {bar_rows}
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
      <h2>多维度评估（9 Tab：1 全局总览 + 4 业务深度剖析 + 4 技术效能维度）</h2>
      <div class="tabs">
        {''.join(tab_buttons)}
      </div>
      {''.join(tab_panels)}
    </section>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/html2canvas@1.4.1/dist/html2canvas.min.js"></script>
  <script>
    // Theme detection for ECharts
    const isDark = document.body.getAttribute('data-theme') === 'dark';
    const textColor = isDark ? '#e5e7eb' : '#0f172a';
    const splitColor = isDark ? '#2b3548' : '#e2e8f0';

    // Tab switching
    const tabButtons = document.querySelectorAll('.tab-btn');
    const tabPanels = document.querySelectorAll('.tab-panel');
    tabButtons.forEach(btn => {{
      btn.addEventListener('click', () => {{
        const t = btn.getAttribute('data-tab');
        tabButtons.forEach(b => {{ b.classList.remove('active'); b.setAttribute('aria-selected','false'); }});
        tabPanels.forEach(p => {{ p.classList.remove('active'); p.setAttribute('hidden',''); }});
        btn.classList.add('active');
        btn.setAttribute('aria-selected','true');
        const tp = document.getElementById('panel-'+t);
        if(tp) {{ tp.classList.add('active'); tp.removeAttribute('hidden'); }}
        // Resize charts in active panel
        tp && tp.querySelectorAll('[id^="chart-"]').forEach(el => {{
          const c = echarts.getInstanceByDom(el);
          if(c) c.resize();
        }});
      }});
    }});

    // Export
    document.getElementById('export-pdf').addEventListener('click', () => window.print());
    document.getElementById('export-png').addEventListener('click', async () => {{
      if(typeof window.html2canvas !== 'function') {{ alert('html2canvas 加载失败'); return; }}
      const btn = document.getElementById('export-png');
      btn.disabled = true; btn.textContent = '生成中...';
      try {{
        const canvas = await window.html2canvas(document.querySelector('.container'), {{
          backgroundColor: getComputedStyle(document.body).backgroundColor, scale:2, useCORS:true
        }});
        const a = document.createElement('a');
        a.download = 'spa-dashboard-'+new Date().toISOString().replace(/[:.]/g,'-')+'.png';
        a.href = canvas.toDataURL('image/png'); a.click();
      }} catch(e) {{ console.error(e); alert('导出失败'); }}
      finally {{ btn.disabled = false; btn.textContent = '导出 PNG'; }}
    }});

    // ===== ECharts Initialization =====
    function initChart(id, opt) {{
      const el = document.getElementById(id);
      if(!el) return null;
      const c = echarts.init(el, isDark ? 'dark' : null);
      c.setOption(opt);
      return c;
    }}

    // Tab 1: Radar Chart
    initChart('chart-radar', {{
      tooltip: {{}},
      radar: {{
        indicator: {radar_labels_js}.map(n => ({{name:n, max:100}})),
        shape: 'polygon',
        splitArea: {{ areaStyle: {{ color: isDark ? ['rgba(30,58,138,0.1)','rgba(30,58,138,0.2)'] : ['rgba(219,234,254,0.2)','rgba(219,234,254,0.4)'] }} }}
      }},
      series: [{{
        type: 'radar',
        data: [{{ value: {radar_values_js}, name: '七维评分', areaStyle: {{ opacity: 0.3 }} }}],
        lineStyle: {{ width: 2 }},
        itemStyle: {{ color: '#3b82f6' }}
      }}]
    }});

    // Tab 1: Gauge
    initChart('chart-gauge', {{
      series: [{{
        type: 'gauge',
        startAngle: 200, endAngle: -20,
        min: 0, max: 100,
        detail: {{ formatter: '{{value}}', fontSize: 28, offsetCenter: [0, '60%'], color: textColor }},
        data: [{{ value: {overall_score:.0f}, name: '综合分' }}],
        axisLine: {{ lineStyle: {{ width: 20, color: [[0.3,'#F5222D'],[0.5,'#FAAD14'],[0.7,'#3b82f6'],[1,'#52C41A']] }} }},
        axisTick: {{ show: false }},
        splitLine: {{ show: false }},
        axisLabel: {{ show: false }},
        pointer: {{ width: 4 }},
        title: {{ offsetCenter: [0, '80%'], color: textColor }}
      }}]
    }});

    // Tab 2: Business Completion Bar
    initChart('chart-completion', {{
      tooltip: {{ trigger: 'axis' }},
      xAxis: {{ type: 'category', data: {completion_data_js}.map(d => d.name), axisLabel: {{ color: textColor }} }},
      yAxis: {{ type: 'value', max: 100, axisLabel: {{ color: textColor }}, splitLine: {{ lineStyle: {{ color: splitColor }} }} }},
      series: [{{
        type: 'bar', data: {completion_data_js}.map(d => ({{
          value: d.value,
          itemStyle: {{ color: d.value >= 70 ? '#52C41A' : d.value >= 50 ? '#FAAD14' : '#F5222D' }}
        }})),
        barWidth: '40%',
        label: {{ show: true, position: 'top', color: textColor }}
      }}]
    }});

    // Tab 3: Topology placeholder (force-directed)
    initChart('chart-topology', {{
      tooltip: {{}},
      series: [{{
        type: 'graph', layout: 'force', roam: true, draggable: true,
        label: {{ show: true, color: textColor }},
        force: {{ repulsion: 200, edgeLength: 120 }},
        categories: [{{"name":"架构"}},{{"name":"业务"}},{{"name":"基础设施"}}],
        data: [
          {{name:'网关层',category:0,symbolSize:40}},
          {{name:'业务逻辑层',category:1,symbolSize:50}},
          {{name:'数据持久层',category:2,symbolSize:35}},
          {{name:'认证服务',category:0,symbolSize:30}},
          {{name:'消息队列',category:2,symbolSize:25}}
        ],
        links: [
          {{source:'网关层',target:'业务逻辑层'}},
          {{source:'业务逻辑层',target:'数据持久层'}},
          {{source:'网关层',target:'认证服务'}},
          {{source:'业务逻辑层',target:'消息队列'}}
        ],
        lineStyle: {{ curveness: 0.2 }}
      }}]
    }});

    // Tab 4: Sankey Diagram
    initChart('chart-sankey', {{
      tooltip: {{ trigger: 'item' }},
      series: [{{
        type: 'sankey', layout: 'none', emphasis: {{ focus: 'adjacency' }},
        data: {sankey_nodes},
        links: {sankey_links},
        lineStyle: {{ color: 'gradient', curveness: 0.5 }},
        label: {{ color: textColor }}
      }}]
    }});

    // Tab 5: Funnel
    initChart('chart-funnel', {{
      tooltip: {{ trigger: 'item', formatter: '{{b}}: {{c}}%' }},
      series: [{{
        type: 'funnel', left: '10%', width: '80%', sort: 'descending',
        label: {{ show: true, position: 'inside', color: '#fff' }},
        data: {funnel_js},
        itemStyle: {{ borderWidth: 1, borderColor: '#fff' }}
      }}]
    }});

    // Tab 6: Polar Bar (Architecture Health)
    initChart('chart-polar', {{
      angleAxis: {{ type: 'category', data: {polar_data_js}.map(d => d.name), axisLabel: {{ color: textColor }} }},
      radiusAxis: {{ max: 100, axisLabel: {{ color: textColor }}, splitLine: {{ lineStyle: {{ color: splitColor }} }} }},
      polar: {{}},
      tooltip: {{ trigger: 'axis' }},
      series: [{{
        type: 'bar', coordinateSystem: 'polar',
        data: {polar_data_js}.map(d => ({{
          value: d.value,
          itemStyle: {{ color: d.value >= 70 ? '#52C41A' : d.value >= 50 ? '#FAAD14' : '#F5222D' }}
        }}))
      }}]
    }});

    // Tab 6: Dependency Matrix Heatmap
    (function() {{
      const dims = {json.dumps([_dimension_label(d) for d in arch_dims], ensure_ascii=False)};
      const vals = {json.dumps([float(dimension_scores.get(d, 0)) for d in arch_dims])};
      const data = [];
      for(let i=0;i<dims.length;i++) for(let j=0;j<dims.length;j++) {{
        data.push([i, j, i===j ? vals[i] : Math.round((vals[i]+vals[j])/2)]);
      }}
      initChart('chart-dep-matrix', {{
        tooltip: {{ formatter: p => dims[p.data[0]]+' × '+dims[p.data[1]]+': '+p.data[2] }},
        xAxis: {{ type:'category', data:dims, axisLabel:{{color:textColor}} }},
        yAxis: {{ type:'category', data:dims, axisLabel:{{color:textColor}} }},
        visualMap: {{ min:0, max:100, calculable:true, orient:'horizontal', left:'center',
          inRange: {{ color: ['#F5222D','#FAAD14','#52C41A'] }},
          textStyle: {{ color: textColor }}
        }},
        series: [{{ type:'heatmap', data:data, label:{{show:true,color:textColor}} }}]
      }});
    }})();

    // Tab 7: Performance Bar
    initChart('chart-perf', {{
      tooltip: {{ trigger: 'axis' }},
      xAxis: {{ type: 'category', data: {perf_data_js}.map(d => d.name), axisLabel: {{ color: textColor }} }},
      yAxis: {{ type: 'value', max: 100, axisLabel: {{ color: textColor }}, splitLine: {{ lineStyle: {{ color: splitColor }} }} }},
      series: [{{
        type: 'bar', data: {perf_data_js}.map(d => ({{
          value: d.value,
          itemStyle: {{ color: d.value >= 70 ? '#52C41A' : d.value >= 50 ? '#FAAD14' : '#F5222D' }}
        }})),
        barWidth: '40%',
        label: {{ show: true, position: 'top', color: textColor }}
      }}]
    }});

    // Tab 8: Nightingale Rose Chart
    initChart('chart-rose', {{
      tooltip: {{ trigger: 'item' }},
      series: [{{
        type: 'pie', roseType: 'area', radius: ['20%','65%'],
        data: {sec_cat_data_js}.length > 0 ? {sec_cat_data_js} : [{{name:'暂无数据',value:1}}],
        label: {{ color: textColor }},
        itemStyle: {{ borderRadius: 4 }}
      }}]
    }});

    // Tab 9: Treemap
    initChart('chart-treemap', {{
      tooltip: {{ formatter: p => p.name + ': ' + (p.value||'') }},
      series: [{{
        type: 'treemap', roam: false,
        data: {res_treemap_js}.length > 0 ? {res_treemap_js} : [{{name:'暂无数据',value:1}}],
        label: {{ show: true, color: '#fff' }},
        levels: [
          {{ itemStyle: {{ borderColor: '#555', borderWidth: 2, gapWidth: 2 }} }},
          {{ colorSaturation: [0.3, 0.6], itemStyle: {{ borderColorSaturation: 0.7, gapWidth: 1, borderWidth: 1 }} }}
        ],
        visualMin: 0,
        visualDimension: 0
      }}]
    }});

    // Responsive resize
    window.addEventListener('resize', () => {{
      document.querySelectorAll('[id^="chart-"]').forEach(el => {{
        const c = echarts.getInstanceByDom(el);
        if(c) c.resize();
      }});
    }});
  </script>
</body>
</html>
"""


def _generate_html_dashboard_v2(review_input: dict, theme: str | None = None) -> str:
    dimension_order = [
        "architecture",
        "security",
        "code-quality",
        "business",
        "devops",
        "team",
        "tech-debt",
    ]
    dimension_scores_raw = review_input.get("dimension_scores", {})
    dimension_issues = review_input.get("dimension_issues", {})
    smell_summary = review_input.get("smell_summary", {})
    by_severity = smell_summary.get("by_severity", {})
    review_ctx = _build_review_context(review_input)
    scorecard = review_ctx.get("scorecard", {})
    recommendations = review_ctx.get("recommendations", [])
    dimension_sections = review_ctx.get("dimension_sections", {})

    scorecard_dim_scores = _scorecard_dimension_scores(scorecard)
    dimension_scores = {
        dim: float(scorecard_dim_scores.get(dim, dimension_scores_raw.get(dim, 0)))
        for dim in dimension_order
    }

    special_scores_raw = review_input.get("special_scores", {})
    scorecard_special = _scorecard_special_scores(scorecard)
    business_maturity = float(scorecard_special.get("business_maturity", special_scores_raw.get("business_maturity", 0)))
    dependency_health = float(scorecard_special.get("dependency_health", special_scores_raw.get("dependency_health", 0)))

    overall_score = float(scorecard.get("overall_score_100") or review_input.get("quantitative_score", 0))
    overall_grade = str(review_input.get("quantitative_grade", "-"))
    selected_theme = theme if theme in {"light", "dark"} else _theme_from_now()
    theme_mode_label = "浅色" if selected_theme == "light" else "深色"

    all_issues: list[dict] = []
    dim_issue_map: dict[str, list[dict]] = {}
    dim_severity_counts: dict[str, dict[str, int]] = {}
    for dk in dimension_order:
        raw = dimension_issues.get(dk, [])
        normalized = [{**i, "dimension": dk} for i in raw if isinstance(i, dict)]
        dim_issue_map[dk] = normalized
        all_issues.extend(normalized)
        counts = {"critical": 0, "high": 0, "medium": 0, "low": 0}
        for issue in normalized:
            severity = str(issue.get("severity", "")).lower()
            if severity in counts:
                counts[severity] += 1
        dim_severity_counts[dk] = counts

    total_issues = len(all_issues)
    total_critical = sum(c["critical"] for c in dim_severity_counts.values())
    total_high = sum(c["high"] for c in dim_severity_counts.values())
    overall_verdict, overall_gate, overall_risk, overall_sla = _judge_tab_status(
        overall_score, total_critical, total_high
    )
    overall_evidence = _top_evidence(all_issues, limit=5)

    business_provenance = "observed" if review_ctx.get("has_business_maturity_artifact") or scorecard_special.get("business_maturity") else "derived"
    dependency_provenance = "observed" if review_ctx.get("has_dependency_health_artifact") or scorecard_special.get("dependency_health") else "derived"

    def kpi_card(title: str, value: str, note: str = "", provenance: str = "observed", score_for_class: float | None = None) -> str:
        metric_class = _score_css(score_for_class) if score_for_class is not None else ""
        return (
            '<div class="kpi-box">'
            f'<span>{escape(title)} <em class="prov-badge prov-{escape(provenance)}">{escape(_provenance_label(provenance))}</em></span>'
            f'<strong class="{metric_class}">{escape(value)}</strong>'
            f'<div class="kpi-note">{escape(note)}</div>'
            '</div>'
        )

    def note_card(title: str, body: str, kind: str = "info") -> str:
        return (
            f'<div class="note-card note-{escape(kind)}">'
            f'<div class="note-title">{escape(title)}</div>'
            f'<div class="note-body">{escape(body)}</div>'
            '</div>'
        )

    def metric_table(rows: list[tuple[str, str, str]]) -> str:
        html_rows = "".join(
            f"<tr><td>{escape(label)}</td><td>{value}</td><td>{escape(note)}</td></tr>"
            for label, value, note in rows
        )
        return (
            '<table class="issues-table"><thead><tr><th>指标</th><th>值</th><th>说明</th></tr></thead>'
            f'<tbody>{html_rows}</tbody></table>'
        )

    def issue_detail_table(issues: list[dict], title: str, include_dimension: bool = False, max_rows: int = 12) -> str:
        headers = "<th>来源维度</th>" if include_dimension else ""
        if not issues:
            colspan = 6 if include_dimension else 5
            rows = f'<tr><td colspan="{colspan}" class="empty-cell">暂无可展示问题</td></tr>'
        else:
            rows_list: list[str] = []
            for issue in sorted(issues, key=lambda i: _severity_rank(str(i.get("severity", ""))), reverse=True)[:max_rows]:
                cells: list[str] = []
                if include_dimension:
                    cells.append(f'<td>{escape(_dimension_label(str(issue.get("dimension", "-"))))}</td>')
                severity = str(issue.get("severity", "-")).lower() or "-"
                cells.extend(
                    [
                        f'<td>{escape(str(issue.get("id", "-")))}</td>',
                        f'<td><span class="pill {_priority_class("P0" if severity == "critical" else "P1" if severity == "high" else "P2")}">{escape(severity)}</span></td>',
                        f'<td><code>{escape(str(issue.get("file", "-")))}</code></td>',
                        f'<td>{escape(str(issue.get("line", "-")))}</td>',
                        f'<td>{escape(str(issue.get("message", "-")))}</td>',
                    ]
                )
                rows_list.append("<tr>" + "".join(cells) + "</tr>")
            rows = "".join(rows_list)
        return (
            f'<h3>{escape(title)}</h3>'
            '<table class="issues-table">'
            f'<thead><tr>{headers}<th>ID</th><th>严重级别</th><th>文件</th><th>行</th><th>描述</th></tr></thead>'
            f'<tbody>{rows}</tbody></table>'
        )

    def finding_table(items: list[str], title: str, provenance: str = "observed") -> str:
        if not items:
            body = '<tr><td class="empty-cell" colspan="3">暂无补充发现</td></tr>'
        else:
            rows = []
            for item in items[:8]:
                refs = _extract_code_refs(item)
                summary = _strip_markdown_inline(item)
                evidence = "<br>".join(f"<code>{escape(ref)}</code>" for ref in refs[:3]) or "-"
                rows.append(
                    "<tr>"
                    f"<td>{escape(summary[:180])}</td>"
                    f"<td>{evidence}</td>"
                    f"<td><span class=\"prov-badge prov-{escape(provenance)}\">{escape(_provenance_label(provenance))}</span></td>"
                    "</tr>"
                )
            body = "".join(rows)
        return (
            f'<h3>{escape(title)}</h3>'
            '<table class="issues-table"><thead><tr><th>发现/说明</th><th>证据</th><th>来源</th></tr></thead>'
            f'<tbody>{body}</tbody></table>'
        )

    def recommendation_table(items: list[dict], title: str, max_rows: int = 10) -> str:
        if not items:
            rows = '<tr><td colspan="5" class="empty-cell">暂无整改建议</td></tr>'
        else:
            ranked = sorted(items, key=lambda item: _priority_rank(item.get("priority", "P2")), reverse=True)[:max_rows]
            rows = "".join(
                "<tr>"
                f'<td><span class="pill {_priority_class(item.get("priority", "P2"))}">{escape(item.get("priority", "P2"))}</span></td>'
                f'<td>{escape(item.get("task", "-"))}</td>'
                f'<td>{escape(_display_dimension_name(item.get("dimension", "未知")))}</td>'
                f'<td>{escape(_translate_priority_label(item.get("priority_label", "-")))}</td>'
                f'<td>{"<code>" + escape(item.get("evidence", "-")) + "</code>" if item.get("evidence") and item.get("evidence") != "-" else "-"}</td>'
                "</tr>"
                for item in ranked
            )
        return (
            f'<h3>{escape(title)}</h3>'
            '<table class="issues-table"><thead><tr><th>优先级</th><th>动作</th><th>维度</th><th>阶段</th><th>证据</th></tr></thead>'
            f'<tbody>{rows}</tbody></table>'
        )

    def expert_card(title: str, dims: list[str], provenance_note: str) -> str:
        issues: list[dict] = []
        for dim in dims:
            issues.extend(dim_issue_map.get(dim, []))
        score_avg = sum(float(dimension_scores.get(dim, 0)) for dim in dims) / max(len(dims), 1)
        critical = sum(dim_severity_counts.get(dim, {}).get("critical", 0) for dim in dims)
        high = sum(dim_severity_counts.get(dim, {}).get("high", 0) for dim in dims)
        verdict, gate, risk, sla = _judge_tab_status(score_avg, critical, high)
        evidence_refs = _top_evidence(issues, limit=4)
        return f"""
<div class="expert-card">
  <h3>{escape(title)}</h3>
  <div class="expert-grid">
    <div class="field"><div class="field-label">审核结论</div><div class="field-value">{escape(_display_verdict(verdict))}</div></div>
    <div class="field"><div class="field-label">Gate 建议</div><div class="field-value">{escape(_display_gate(gate))}</div></div>
    <div class="field"><div class="field-label">风险等级</div><div class="field-value">{escape(risk)}</div></div>
    <div class="field"><div class="field-label">整改时限</div><div class="field-value">{escape(sla)}</div></div>
  </div>
  <div class="expert-foot">
    <div><span class="field-label">关键证据</span><div class="field-value small">{escape(evidence_refs)}</div></div>
    <div><span class="field-label">说明</span><div class="field-value small">{escape(provenance_note)}</div></div>
  </div>
</div>"""

    dim_rows_from_scorecard = scorecard.get("dimension_rows", [])
    dim_summary_rows = "".join(
        "<tr>"
        f'<td>{escape(_display_dimension_name(row.get("Dimension", "-")))}</td>'
        f'<td>{escape(row.get("Architecture", "-"))}</td>'
        f'<td>{escape(row.get("Deep", "-"))}</td>'
        f'<td>{escape(row.get("Deep(Business)", "-"))}</td>'
        f'<td>{escape(row.get("Final", "-"))}</td>'
        f'<td>{escape(_display_rating(row.get("Rating", "-")))}</td>'
        "</tr>"
        for row in dim_rows_from_scorecard
    ) or "".join(
        f"<tr><td>{escape(_dimension_label(dim))}</td><td>-</td><td>-</td><td>-</td><td>{dimension_scores.get(dim, 0)/10:.1f}</td><td>{escape(_score_level(dimension_scores.get(dim, 0)))}</td></tr>"
        for dim in dimension_order
    )

    severity_chart_data = [int(by_severity.get(level, 0)) for level in ["critical", "high", "medium", "low"]]
    priority_counts = {priority: 0 for priority in ["P0", "P1", "P2", "P3"]}
    for item in recommendations:
        priority_counts[item.get("priority", "P2")] = priority_counts.get(item.get("priority", "P2"), 0) + 1

    business_issues = dim_issue_map.get("business", [])
    architecture_issues = dim_issue_map.get("architecture", [])
    security_issues = dim_issue_map.get("security", [])
    code_quality_issues = dim_issue_map.get("code-quality", [])
    devops_issues = dim_issue_map.get("devops", [])
    team_issues = dim_issue_map.get("team", [])
    debt_issues = dim_issue_map.get("tech-debt", [])

    connection_rows = []
    contract_rows = []
    for item in review_ctx.get("architecture_observations", [])[:6]:
        refs = _select_contract_refs(item)
        summary = _strip_markdown_inline(item)
        contract_rows.append(_infer_skill_contract_row(item))
        connection_rows.append(
            {
                "summary": summary,
                "source": refs[0] if refs else "架构观察",
                "target": refs[1] if len(refs) > 1 else ("上下游模块" if len(refs) == 1 else "关联模块"),
                "status": "需关注" if refs else "说明项",
                "evidence": "；".join(refs[:3]) or "-",
            }
        )

    if not connection_rows:
        connection_rows.append({"summary": "暂无足够模块连接证据，当前以架构发现表替代。", "source": "N/A", "target": "N/A", "status": "N/A", "evidence": "-"})
        contract_rows.append({"domain": "N/A", "contract": "-", "validator": "-", "consumer": "-", "summary": "暂无足够契约证据"})

    contract_table_rows = "".join(
        "<tr>"
        f'<td>{escape(row["domain"])}</td>'
        f'<td>{("<code>" + escape(row["contract"]) + "</code>") if row["contract"] != "观测文本" else escape(row["contract"])}</td>'
        f'<td>{("<code>" + escape(row["validator"]) + "</code>") if row["validator"] != "-" else "-"}</td>'
        f'<td>{("<code>" + escape(row["consumer"]) + "</code>") if row["consumer"] not in {"下游技能", "-"} else escape(row["consumer"])}</td>'
        f'<td>{escape(row["summary"][:110])}</td>'
        "</tr>"
        for row in contract_rows
    )

    connection_table_rows = "".join(
        "<tr>"
        f'<td>{escape(row["summary"][:120])}</td>'
        f'<td><code>{escape(row["source"])}</code></td>'
        f'<td><code>{escape(row["target"])}</code></td>'
        f'<td>{escape(row["status"])}</td>'
        f'<td>{("<code>" + escape(row["evidence"]) + "</code>") if row["evidence"] != "-" else "-"}</td>'
        "</tr>"
        for row in connection_rows
    )

    business_signal_rows = []
    for idx, item in enumerate(review_ctx.get("business_observations", [])[:6], start=1):
        refs = _meaningful_code_refs(item)
        status = "已覆盖" if refs else "说明项"
        capability_name = _business_capability_name(item, idx)
        business_signal_rows.append(
            f"<tr><td>{escape(capability_name)}</td><td>{escape(_strip_markdown_inline(item)[:110])}</td><td>{escape(status)}</td><td>{('<code>' + '</code><br><code>'.join(escape(ref) for ref in refs[:3]) + '</code>') if refs else '-'}</td></tr>"
        )
    if not business_signal_rows:
        business_signal_rows.append('<tr><td colspan="4" class="empty-cell">暂无业务能力说明</td></tr>')

    logic_keywords = ["logic", "rule", "state", "流程", "规则", "状态", "workflow"]
    logic_issues = [issue for issue in business_issues + code_quality_issues if any(keyword in (str(issue.get("message", "")) + str(issue.get("id", ""))).lower() for keyword in logic_keywords)]
    if not logic_issues:
        logic_issues = business_issues[:6] + code_quality_issues[:6]

    recommendation_backlog = recommendation_table(recommendations, "整改路线与收益-成本代理视图")

    executive_summary_table = metric_table(
        [
            ("综合评分", f"{overall_score:.1f}/100", "优先使用 scorecard 汇总结果，回退到 quantitative-input.json"),
            ("量化评级", escape(overall_grade), "当前仍沿用量化评级字段"),
            ("Business Maturity", f"{business_maturity:.1f}/10", f"{_provenance_label(business_provenance)} — {'专项产物或 scorecard 可见' if business_provenance == 'observed' else '由现有审计结论派生'}"),
            ("Dependency Health", f"{dependency_health:.1f}/10", f"{_provenance_label(dependency_provenance)} — {'专项产物或 scorecard 可见' if dependency_provenance == 'observed' else '由现有审计结论派生'}"),
        ]
    )

    risk_table = recommendation_table(recommendations, "按优先级分组的关键整改项", max_rows=8)

    tab0 = f"""
{expert_card('专家总评卡', dimension_order, '总览优先展示最终审计分与高优先级风险；专项分无专项工件时降级为 Derived。')}
<div class="panel-kpi">
  {kpi_card('综合评分', f'{overall_score:.1f}/100', '优先来自 scorecard', 'observed', overall_score)}
  {kpi_card('总问题数', str(total_issues), '聚合 7 维 issue', 'observed')}
  {kpi_card('Business Maturity', f'{business_maturity:.1f}/10', '专项成熟度', business_provenance, business_maturity * 10)}
  {kpi_card('Dependency Health', f'{dependency_health:.1f}/10', '依赖健康度', dependency_provenance, dependency_health * 10)}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub">{executive_summary_table}</div>
  <div class="card-sub">{note_card('总览说明', '雷达图、仪表盘、风险分布和能力画像已提升到页面顶部固定概览区；本 Tab 聚焦结论、评分与路线图。', 'info')}</div>
</div>
<h3>七维评分汇总</h3>
<table class="issues-table"><thead><tr><th>维度</th><th>Architecture</th><th>Deep</th><th>Deep(Business)</th><th>Final</th><th>Rating</th></tr></thead><tbody>{dim_summary_rows}</tbody></table>
{risk_table}
{finding_table(review_ctx.get('executive_summary', []), '执行摘要摘录', 'observed')}
"""

    tab1 = f"""
{expert_card('业务完成度专家卡', ['business', 'team'], '业务完成度主要基于 deep-business 审计文本、业务 issue 与建议列表，不伪造真实转化漏斗。')}
<div class="panel-kpi">
  {kpi_card('业务评分', f"{dimension_scores.get('business', 0):.0f}/100", '取最终维度分', 'observed', dimension_scores.get('business', 0))}
  {kpi_card('团队协作', f"{dimension_scores.get('team', 0):.0f}/100", '协作与知识流动', 'observed', dimension_scores.get('team', 0))}
  {kpi_card('Business Maturity', f"{business_maturity:.1f}/10", '业务链完整度', business_provenance, business_maturity * 10)}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>业务能力覆盖图 <em class="prov-badge prov-observed">Observed</em></h3><div id="chart-completion" class="chart"></div></div>
  <div class="card-sub">{note_card('边界说明', '当前仓库审计没有真实用户转化或运行时业务漏斗，因此本 Tab 使用业务能力覆盖与 evidence table 替代。', 'warn')}</div>
</div>
<h3>业务能力/流程证据表</h3>
<table class="issues-table"><thead><tr><th>能力项</th><th>说明</th><th>状态</th><th>证据</th></tr></thead><tbody>{''.join(business_signal_rows)}</tbody></table>
{finding_table(review_ctx.get('business_observations', []), '业务价值观测摘要', 'observed')}
{issue_detail_table(business_issues + team_issues, '业务 / 团队相关问题', include_dimension=True)}
"""

    tab2 = f"""
{expert_card('业务连接专家卡', ['architecture', 'tech-debt'], '针对 Skills 项目，业务连接优先展示技能 → 契约 → 校验 → 消费方矩阵，而不是通用服务拓扑。')}
<div class="panel-kpi">
  {kpi_card('架构评分', f"{dimension_scores.get('architecture', 0):.0f}/100", '来自最终评分卡', 'observed', dimension_scores.get('architecture', 0))}
  {kpi_card('技术债评分', f"{dimension_scores.get('tech-debt', 0):.0f}/100", '来自最终评分卡', 'observed', dimension_scores.get('tech-debt', 0))}
  {kpi_card('契约连接项', str(len(contract_rows)), '来自架构观察抽取', 'derived')}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>连接热区图 <em class="prov-badge prov-derived">Derived</em></h3><div id="chart-connection" class="chart"></div></div>
  <div class="card-sub">{note_card('真实性说明', '该 Tab 以技能系统的契约连接矩阵为中心，不再模拟普通业务系统服务拓扑。', 'info')}</div>
</div>
<h3>技能契约连接矩阵</h3>
<table class="issues-table"><thead><tr><th>能力域</th><th>契约/入口</th><th>校验/验证</th><th>消费方</th><th>说明</th></tr></thead><tbody>{contract_table_rows}</tbody></table>
<h3>连接关系详表</h3>
<table class="issues-table"><thead><tr><th>摘要</th><th>来源</th><th>目标</th><th>状态</th><th>证据</th></tr></thead><tbody>{connection_table_rows}</tbody></table>
{finding_table(review_ctx.get('architecture_observations', []), '架构连接相关观察', 'observed')}
{issue_detail_table(architecture_issues + debt_issues, '架构 / 技术债问题明细', include_dimension=True)}
"""

    heuristic_chain_rows = []
    flow_through_metric = _extract_special_metric(review_ctx.get("business_analysis", "") or "", "flow_through_rate") or _extract_special_metric(_safe_read_text(Path(str(review_input.get("review_dir", ""))) / "deep-business" / "dim-4-business-value.md") or "", "flow_through_rate")
    broken_flow_metric = _extract_special_metric(_safe_read_text(Path(str(review_input.get("review_dir", ""))) / "deep-business" / "dim-4-business-value.md") or "", "broken_flow_rate")
    observed_flow_through = _flow_metric_value(flow_through_metric)
    observed_broken_flow = _flow_metric_value(broken_flow_metric)
    chain_buckets = [
        ('需求到规则一致性', len(logic_issues)),
        ('业务链断点信号', len(business_issues)),
        ('交付与运行准备度', len(devops_issues)),
        ('技术债拖累', len(debt_issues)),
    ]
    if observed_flow_through is not None:
        chain_buckets.insert(0, ('flow_through_rate', int(round(observed_flow_through))))
    if observed_broken_flow is not None:
        chain_buckets.insert(1, ('broken_flow_rate', int(round(observed_broken_flow))))
    for name, value in chain_buckets:
        if name == 'flow_through_rate':
            status = '已观测' if value >= 80 else ('需关注' if value >= 50 else '偏低')
        elif name == 'broken_flow_rate':
            status = '已观测' if value <= 5 else ('需关注' if value <= 20 else '偏高')
        else:
            status = '通畅' if value <= 1 else ('需关注' if value <= 4 else '断点明显')
        heuristic_chain_rows.append(f'<tr><td>{escape(name)}</td><td>{value}</td><td>{escape(status)}</td></tr>')

    tab3 = f"""
{expert_card('业务连通率专家卡', ['business', 'devops'], '优先展示审计产物中的 flow_through_rate / broken_flow_rate；缺失时再回退到 Heuristic 代理信号。')}
<div class="panel-kpi">
  {kpi_card('业务评分', f"{dimension_scores.get('business', 0):.0f}/100", '最终业务维度评分', 'observed', dimension_scores.get('business', 0))}
  {kpi_card('DevOps 评分', f"{dimension_scores.get('devops', 0):.0f}/100", '交付与运行准备度', 'observed', dimension_scores.get('devops', 0))}
  {kpi_card('flow_through_rate', f"{observed_flow_through:.0f}%" if observed_flow_through is not None else str(sum(value for _, value in chain_buckets)), '优先使用专项观测值，缺失时回退到代理信号', 'observed' if observed_flow_through is not None else 'heuristic')}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>链路连通代理图 <em class="prov-badge prov-{'observed' if observed_flow_through is not None else 'heuristic'}">{_provenance_label('observed' if observed_flow_through is not None else 'heuristic')}</em></h3><div id="chart-sankey" class="chart"></div></div>
  <div class="card-sub"><h3>断点分段统计 <em class="prov-badge prov-{'observed' if observed_flow_through is not None else 'heuristic'}">{_provenance_label('observed' if observed_flow_through is not None else 'heuristic')}</em></h3><table class="issues-table"><thead><tr><th>链路段</th><th>信号数</th><th>状态</th></tr></thead><tbody>{''.join(heuristic_chain_rows)}</tbody></table></div>
</div>
{issue_detail_table(business_issues + devops_issues, '业务连通相关问题', include_dimension=True)}
"""

    logic_rows = []
    for issue in logic_issues[:10]:
        logic_rows.append(
            "<tr>"
            f'<td>{escape(_dimension_label(str(issue.get("dimension", "-"))))}</td>'
            f'<td>{escape(str(issue.get("message", "-")))}</td>'
            f'<td><code>{escape(str(issue.get("file", "-")))}</code>:{escape(str(issue.get("line", "-")))}</td>'
            f'<td><span class="pill {_priority_class("P1" if str(issue.get("severity", "")).lower() in {"critical", "high"} else "P2")}">{escape(str(issue.get("severity", "-")))}</span></td>'
            "</tr>"
        )
    if not logic_rows:
        logic_rows.append('<tr><td colspan="4" class="empty-cell">暂无逻辑问题明细</td></tr>')

    tab4 = f"""
{expert_card('业务逻辑通顺性专家卡', ['business', 'code-quality'], '无真实流程阶段数据时，本 Tab 用逻辑问题分类与受影响流程证据表替代 funnel。')}
<div class="panel-kpi">
  {kpi_card('业务逻辑问题', str(len(logic_issues)), '基于业务/代码质量 issue 抽取', 'derived')}
  {kpi_card('代码质量评分', f"{dimension_scores.get('code-quality', 0):.0f}/100", '最终维度分', 'observed', dimension_scores.get('code-quality', 0))}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>逻辑问题分类图 <em class="prov-badge prov-derived">Derived</em></h3><div id="chart-logic" class="chart"></div></div>
  <div class="card-sub">{note_card('替代说明', '旧版 funnel 容易被误解为真实业务转化漏斗，因此改成逻辑问题类别分布图。', 'warn')}</div>
</div>
<h3>受影响流程 / 规则问题表</h3>
<table class="issues-table"><thead><tr><th>来源维度</th><th>问题描述</th><th>证据</th><th>严重级别</th></tr></thead><tbody>{''.join(logic_rows)}</tbody></table>
{finding_table(dimension_sections.get('business', {}).get('observations', []), '业务逻辑相关观察', 'observed')}
"""

    tab5 = f"""
{expert_card('架构健康度专家卡', ['architecture', 'code-quality', 'tech-debt'], '热力图改为真实 issue-count 类别矩阵，不再使用维度均值伪矩阵。')}
<div class="panel-kpi">
  {kpi_card('架构', f"{dimension_scores.get('architecture', 0):.0f}/100", '架构演进性', 'observed', dimension_scores.get('architecture', 0))}
  {kpi_card('代码质量', f"{dimension_scores.get('code-quality', 0):.0f}/100", '工程纪律', 'observed', dimension_scores.get('code-quality', 0))}
  {kpi_card('技术债', f"{dimension_scores.get('tech-debt', 0):.0f}/100", '未来变更成本', 'observed', dimension_scores.get('tech-debt', 0))}
  {kpi_card('Dependency Health', f'{dependency_health:.1f}/10', '依赖专项分', dependency_provenance, dependency_health * 10)}
</div>
<div class="charts-grid dense-grid">
  <div class="card-sub"><h3>架构健康度极坐标图 <em class="prov-badge prov-observed">Observed</em></h3><div id="chart-polar" class="chart"></div></div>
  <div class="card-sub"><h3>架构问题热力图 <em class="prov-badge prov-derived">Derived</em></h3><div id="chart-dep-matrix" class="chart"></div></div>
</div>
{finding_table(review_ctx.get('architecture_observations', []), '架构发现摘要', 'observed')}
{issue_detail_table(architecture_issues + code_quality_issues + debt_issues, '架构 / 代码质量 / 技术债问题', include_dimension=True)}
"""

    stability_rows = [
        ('自动化测试', '弱', '仅从仓库证据推断，无覆盖率工件时视为代理项'),
        ('CI/CD', '待建设', '无 workflow/流水线证据时标为代理项'),
        ('构建稳定性', '需人工验证', '依赖审计发现与建议'),
        ('回滚准备度', 'N/A', '仓库审计未观察到可执行回滚工件'),
    ]
    tab6 = f"""
{expert_card('性能与稳定性专家卡', ['devops', 'code-quality'], '当前无运行时 telemetry，因此该 Tab 明确降级为工程稳定性代理视图。')}
<div class="panel-kpi">
  {kpi_card('DevOps', f"{dimension_scores.get('devops', 0):.0f}/100", '最终维度分', 'observed', dimension_scores.get('devops', 0))}
  {kpi_card('代码质量', f"{dimension_scores.get('code-quality', 0):.0f}/100", '用于稳定性代理', 'observed', dimension_scores.get('code-quality', 0))}
  {kpi_card('运行时遥测', 'N/A', '未发现 P95/P99/SLO 工件', 'na')}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>工程稳定性代理图 <em class="prov-badge prov-derived">Derived</em></h3><div id="chart-perf" class="chart"></div></div>
  <div class="card-sub">{metric_table(stability_rows)}</div>
</div>
{issue_detail_table(devops_issues + code_quality_issues, '稳定性 / 交付风险问题', include_dimension=True)}
"""

    governance_rows = [
        ('身份与访问控制', 'Fail' if security_issues else 'Pass', '基于 security 维度问题聚合'),
        ('依赖与供应链', 'Concern' if debt_issues else 'Pass', '依赖健康与未声明依赖检查'),
        ('日志与敏感信息', 'Pass' if not any('token' in str(issue.get('message', '')).lower() for issue in security_issues) else 'Concern', '基于审计发现'),
        ('发布门禁', 'Escalate', '正式 Go/No-Go 仍应交给 arc:gate'),
    ]
    governance_html_rows = ''.join(f'<tr><td>{escape(name)}</td><td>{escape(status)}</td><td>{escape(note)}</td></tr>' for name, status, note in governance_rows)
    tab7 = f"""
{expert_card('安全治理与合规专家卡', ['security', 'architecture'], '该 Tab 以 security issue 与治理检查为主，适合做成强表格 + 强分类图。')}
<div class="panel-kpi">
  {kpi_card('安全评分', f"{dimension_scores.get('security', 0):.0f}/100", '最终维度分', 'observed', dimension_scores.get('security', 0))}
  {kpi_card('安全问题数', str(len(security_issues)), 'security issue 聚合', 'observed')}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>安全问题分类图 <em class="prov-badge prov-derived">Derived taxonomy</em></h3><div id="chart-rose" class="chart"></div></div>
  <div class="card-sub"><h3>治理检查表 <em class="prov-badge prov-observed">Observed</em></h3><table class="issues-table"><thead><tr><th>检查项</th><th>结果</th><th>说明</th></tr></thead><tbody>{governance_html_rows}</tbody></table></div>
</div>
{issue_detail_table(security_issues, '安全问题明细', include_dimension=False)}
"""

    tab8 = f"""
{expert_card('资源与成本代理视图专家卡', ['devops', 'tech-debt', 'team'], '无账单/资源遥测时，本 Tab 只展示工程资源与技术债整改代理视图。')}
<div class="panel-kpi">
  {kpi_card('技术债', f"{dimension_scores.get('tech-debt', 0):.0f}/100", '技术债维度分', 'observed', dimension_scores.get('tech-debt', 0))}
  {kpi_card('团队协作', f"{dimension_scores.get('team', 0):.0f}/100", '知识流动与协作', 'observed', dimension_scores.get('team', 0))}
  {kpi_card('FinOps 遥测', 'N/A', '仓库审计未发现成本账单/利用率工件', 'na')}
</div>
<div class="charts-grid dense-grid-two">
  <div class="card-sub"><h3>整改工作量树图 <em class="prov-badge prov-observed">Observed</em></h3><div id="chart-treemap" class="chart"></div></div>
  <div class="card-sub">{note_card('说明', '树图按 recommendations 的优先级和维度聚合，表达整改工作量分布，而不是云资源账单。', 'info')}</div>
</div>
{recommendation_backlog}
{issue_detail_table(devops_issues + debt_issues + team_issues, '资源 / 技术债 / 团队相关问题', include_dimension=True)}
"""

    tab_buttons = []
    tab_panels = []
    tab_titles = [
        '全局总览', '业务完成度', '业务连接', '业务连通率', '业务逻辑通顺性', '架构健康度', '性能与稳定性', '安全治理与合规', '资源与成本'
    ]
    tab_contents = [tab0, tab1, tab2, tab3, tab4, tab5, tab6, tab7, tab8]
    for idx, (title, content) in enumerate(zip(tab_titles, tab_contents)):
        active = idx == 0
        active_class = 'active' if active else ''
        selected = 'true' if active else 'false'
        hidden_attr = '' if active else 'hidden'
        tab_buttons.append(
            f'<button type="button" class="tab-btn {active_class}" role="tab" id="tab-{idx}" aria-selected="{selected}" aria-controls="panel-{idx}" data-tab="{idx}">{escape(title)}</button>'
        )
        tab_panels.append(
            f'<section class="tab-panel {active_class}" role="tabpanel" id="panel-{idx}" aria-labelledby="tab-{idx}" {hidden_attr}>{content}</section>'
        )

    recommendation_priority_js = json.dumps([priority_counts.get(priority, 0) for priority in ['P0', 'P1', 'P2', 'P3']])
    radar_labels_js = json.dumps([_dimension_label(dim) for dim in dimension_order], ensure_ascii=False)
    radar_values_js = json.dumps([round(dimension_scores.get(dim, 0), 1) for dim in dimension_order])
    severity_data_js = json.dumps(severity_chart_data)
    capability_data_js = json.dumps([
        {"name": "业务交付", "value": round(dimension_scores.get('business', 0) / 10, 1)},
        {"name": "架构演进", "value": round(dimension_scores.get('architecture', 0) / 10, 1)},
        {"name": "质量保障", "value": round(dimension_scores.get('code-quality', 0) / 10, 1)},
        {"name": "安全治理", "value": round(dimension_scores.get('security', 0) / 10, 1)},
        {"name": "交付运维", "value": round(dimension_scores.get('devops', 0) / 10, 1)},
        {"name": "协作效率", "value": round(dimension_scores.get('team', 0) / 10, 1)},
    ], ensure_ascii=False)
    completion_data_js = json.dumps([
        {"name": "业务", "value": round(dimension_scores.get('business', 0), 1)},
        {"name": "团队", "value": round(dimension_scores.get('team', 0), 1)},
        {"name": "成熟度×10", "value": round(business_maturity * 10, 1)},
    ], ensure_ascii=False)
    connection_chart_js = json.dumps([
        {"name": row['summary'][:24], "value": len(_extract_code_refs(row['evidence'])) if row['evidence'] != '-' else 1}
        for row in connection_rows
    ], ensure_ascii=False)
    sankey_nodes_js = json.dumps([{"name": name} for name, _ in chain_buckets] + [{"name": "业务总链路"}], ensure_ascii=False)
    sankey_links_js = json.dumps([{"source": name, "target": "业务总链路", "value": max(value, 1)} for name, value in chain_buckets], ensure_ascii=False)
    logic_chart_js = json.dumps([
        {"name": "规则/状态问题", "value": len(logic_issues)},
        {"name": "业务问题", "value": len(business_issues)},
        {"name": "代码质量牵连", "value": len(code_quality_issues)},
    ], ensure_ascii=False)
    polar_data_js = json.dumps([
        {"name": _dimension_label(dim), "value": round(dimension_scores.get(dim, 0), 1)}
        for dim in ['architecture', 'code-quality', 'tech-debt']
    ], ensure_ascii=False)
    heatmap_dims = ['architecture', 'code-quality', 'tech-debt']
    heatmap_categories = ['critical', 'high', 'medium', 'low']
    heatmap_data_js = json.dumps([
        [x_idx, y_idx, dim_severity_counts.get(dim, {}).get(category, 0)]
        for x_idx, dim in enumerate(heatmap_dims)
        for y_idx, category in enumerate(heatmap_categories)
    ])
    perf_data_js = json.dumps([
        {"name": "DevOps", "value": round(dimension_scores.get('devops', 0), 1)},
        {"name": "Code Quality", "value": round(dimension_scores.get('code-quality', 0), 1)},
        {"name": "测试准备度(代理)", "value": max(5, 100 - len(devops_issues) * 7)},
    ], ensure_ascii=False)
    security_category_counts: dict[str, int] = {}
    for issue in security_issues:
        key = str(issue.get('id', 'other')).split('-')[0] if issue.get('id') else 'other'
        security_category_counts[key] = security_category_counts.get(key, 0) + 1
    rose_data_js = json.dumps(
        [{"name": key, "value": value} for key, value in security_category_counts.items()] or [{"name": "暂无数据", "value": 1}],
        ensure_ascii=False,
    )
    treemap_map: dict[str, dict[str, int]] = {}
    for item in recommendations:
        dim = item.get('dimension', '未知')
        treemap_map.setdefault(dim, {})
        priority = item.get('priority', 'P2')
        treemap_map[dim][priority] = treemap_map[dim].get(priority, 0) + 1
    treemap_data_js = json.dumps([
        {"name": _dimension_label(dim) if dim in dimension_order else dim, "children": [{"name": priority, "value": count} for priority, count in buckets.items()]}
        for dim, buckets in treemap_map.items()
    ] or [{"name": "暂无数据", "value": 1}], ensure_ascii=False)

    return f"""<!doctype html>
<html lang=\"zh-CN\">
<head>
  <meta charset=\"UTF-8\" />
  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />
  <title>{escape(Path(str(review_input.get('project_path', '-'))).name)} - 项目评审可视化报告</title>
  <script src=\"https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js\"></script>
  <style>
    :root {{
      --bg:#f4f6fb; --card:#ffffff; --panel:#ffffff; --text:#1f2d3d; --muted:#6b7280;
      --danger:#ef4444; --warn:#f59e0b; --ok:#10b981; --primary:#3b82f6; --border:#e5e7eb;
      --observed:#0f766e; --derived:#2563eb; --heuristic:#b45309; --na:#6b7280;
    }}
    [data-theme=\"dark\"] {{
      --bg:#0f172a; --card:#1e293b; --panel:#111827; --text:#e2e8f0; --muted:#94a3b8;
      --danger:#f87171; --warn:#fbbf24; --ok:#34d399; --primary:#60a5fa; --border:#334155;
      --observed:#5eead4; --derived:#93c5fd; --heuristic:#fcd34d; --na:#94a3b8;
    }}
    * {{ box-sizing:border-box; }}
    body {{ margin:0; font-family:-apple-system,BlinkMacSystemFont,\"Segoe UI\",Roboto,\"PingFang SC\",\"Microsoft YaHei\",sans-serif; background:var(--bg); color:var(--text); line-height:1.5; }}
    .container {{ max-width:1440px; margin:0 auto; padding:24px; }}
    .header {{ background:linear-gradient(135deg,#1d4ed8,#2563eb); color:#fff; border-radius:16px; padding:24px; margin-bottom:20px; }}
    .header h1 {{ margin:0 0 8px; font-size:28px; }}
    .meta {{ opacity:.95; font-size:14px; }}
    .summary-grid,.panel-kpi {{ display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:14px; }}
    .panel-kpi {{ margin:14px 0; }}
    .overview-grid {{ display:grid; grid-template-columns:1fr 1fr; gap:14px; margin:20px 0; }}
    .overview-grid .card-sub {{ min-height:380px; }}
    .kpi-box,.card-sub,.note-card,.expert-card,.summary-card {{ background:var(--card); border:1px solid var(--border); border-radius:12px; padding:16px; box-shadow:0 2px 8px rgba(15,23,42,.04); }}
    .summary-card .metric-title,.kpi-box span,.field-label,.kpi-note {{ color:var(--muted); font-size:12px; }}
    .summary-card .metric-value,.kpi-box strong {{ display:block; font-size:26px; margin-top:6px; font-weight:700; }}
    .summary-card .metric-note,.kpi-note {{ margin-top:6px; line-height:1.4; color:var(--muted); font-size:13px; }}
    .ok {{ color:var(--ok); }} .warn {{ color:var(--warn); }} .danger {{ color:var(--danger); }}
    .tabs {{ display:flex; flex-wrap:wrap; gap:8px; margin-bottom:16px; border-bottom:2px solid var(--border); padding-bottom:12px; }}
    .tab-btn {{ border:1px solid var(--border); background:var(--card); color:var(--muted); border-radius:8px; padding:8px 14px; cursor:pointer; font-size:13px; font-weight:500; }}
    .tab-btn.active {{ background:color-mix(in srgb, var(--primary) 15%, var(--card) 85%); border-color:var(--primary); color:var(--primary); font-weight:600; }}
    .tab-panel {{ display:none; }} .tab-panel.active {{ display:block; }}
    .charts-grid {{ display:grid; gap:14px; margin:14px 0; }}
    .dense-grid {{ grid-template-columns:1fr 1fr; }}
    .dense-grid-two {{ grid-template-columns:1.3fr .7fr; }}
    .chart {{ width:100%; height:320px; }} .chart-sm {{ height:260px; }} .chart-lg {{ height:360px; }}
    h2 {{ margin:4px 0 12px; font-size:20px; }} h3 {{ margin:0 0 10px; font-size:16px; }}
    table {{ width:100%; border-collapse:collapse; background:var(--card); border:1px solid var(--border); border-radius:12px; overflow:hidden; font-size:13px; }}
    thead {{ background:color-mix(in srgb, var(--card) 85%, var(--bg) 15%); }}
    th,td {{ border-bottom:1px solid var(--border); padding:10px; text-align:left; vertical-align:top; }}
    tr:last-child td {{ border-bottom:none; }}
    code {{ font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace; background:color-mix(in srgb, var(--card) 75%, var(--primary) 25%); border-radius:4px; padding:1px 4px; word-break:break-all; }}
    .pill {{ display:inline-block; padding:2px 8px; border-radius:999px; font-size:12px; font-weight:600; }}
    .p0 {{ background:#fee2e2; color:#b91c1c; }} .p1 {{ background:#fef3c7; color:#92400e; }} .p2 {{ background:#dbeafe; color:#1d4ed8; }}
    .prov-badge {{ display:inline-block; margin-left:6px; padding:2px 8px; border-radius:999px; font-size:11px; font-style:normal; font-weight:600; }}
    .prov-observed {{ background:color-mix(in srgb, var(--observed) 18%, transparent); color:var(--observed); }}
    .prov-derived {{ background:color-mix(in srgb, var(--derived) 18%, transparent); color:var(--derived); }}
    .prov-heuristic {{ background:color-mix(in srgb, var(--heuristic) 18%, transparent); color:var(--heuristic); }}
    .prov-na {{ background:color-mix(in srgb, var(--na) 18%, transparent); color:var(--na); }}
    .expert-card {{ background:linear-gradient(135deg,color-mix(in srgb, var(--primary) 8%, var(--card) 92%), var(--card)); }}
    .expert-grid {{ display:grid; grid-template-columns:repeat(auto-fit,minmax(160px,1fr)); gap:14px; }}
    .field-value {{ font-size:15px; font-weight:700; margin-top:3px; }}
    .field-value.small {{ font-size:13px; font-weight:500; line-height:1.5; }}
    .expert-foot {{ display:grid; grid-template-columns:1fr 1fr; gap:14px; margin-top:12px; }}
    .note-card {{ min-height:100%; }}
    .note-title {{ font-size:14px; font-weight:700; margin-bottom:8px; }}
    .note-body {{ color:var(--muted); font-size:13px; }}
    .note-warn {{ border-color:color-mix(in srgb, var(--warn) 35%, var(--border)); }}
    .note-info {{ border-color:color-mix(in srgb, var(--primary) 28%, var(--border)); }}
    .section-note {{ margin-top:10px; color:var(--muted); font-size:12px; }}
    .empty-cell {{ text-align:center; color:var(--muted); font-style:italic; }}
    @media (max-width: 1000px) {{ .dense-grid,.dense-grid-two,.expert-foot,.overview-grid {{ grid-template-columns:1fr; }} }}
    @media print {{ .tabs {{ display:none !important; }} .tab-panel {{ display:block !important; page-break-inside:avoid; margin-bottom:24px; }} .header {{ print-color-adjust:exact; -webkit-print-color-adjust:exact; }} }}
  </style>
</head>
<body data-theme=\"{selected_theme}\">
  <div class=\"container\">
    <div class=\"header\">
      <h1>{escape(Path(str(review_input.get('project_path', '-'))).name)} 项目评审可视化报告</h1>
      <div class=\"meta\">评审对象：{escape(str(review_input.get('project_path', '-')))}｜评审日期：{escape(str(review_input.get('scan_timestamp', '-')))}｜主题：{escape(theme_mode_label)}｜结论：<b>{escape(_display_gate(overall_gate))}</b></div>
    </div>
    <div class="summary-grid">
      <div class="summary-card"><div class="metric-title">综合评分（七维度）</div><div class="metric-value {_score_css(overall_score)}">{overall_score:.1f} / 100</div><div class="metric-note">优先取 scorecard 汇总，反映最终 audit 结论</div></div>
      <div class="summary-card"><div class="metric-title">推进建议</div><div class="metric-value {'danger' if 'No-Go' in overall_gate else 'warn' if 'Conditional' in overall_gate else 'ok'}">{escape(_display_gate(overall_gate))}</div><div class="metric-note">用于报告摘要，不替代 arc:gate 正式门禁</div></div>
      <div class="summary-card"><div class="metric-title">Business Maturity</div><div class="metric-value ok">{business_maturity:.1f} / 10</div><div class="metric-note">核心业务/能力链成熟度</div></div>
      <div class="summary-card"><div class="metric-title">Dependency Health</div><div class="metric-value ok">{dependency_health:.1f} / 10</div><div class="metric-note">依赖健康与供应链信号</div></div>
      <div class="summary-card"><div class="metric-title">高优先级问题</div><div class="metric-value {'danger' if priority_counts.get('P0', 0) else 'warn'}">P0: {priority_counts.get('P0', 0)} / P1: {priority_counts.get('P1', 0)}</div><div class="metric-note">来自 recommendations.md 的整改优先级</div></div>
      <div class="summary-card"><div class="metric-title">工程能力画像</div><div class="metric-value {'warn' if dimension_scores.get('devops', 0) < 50 else 'ok'}">业务强，运维弱</div><div class="metric-note">从七维度结果抽象出的当前能力特征</div></div>
    </div>
    <div class="overview-grid">
      <div class="card-sub"><h2>七维度评分雷达图</h2><div id="chart-radar" class="chart-lg"></div></div>
      <div class="card-sub"><h2>综合评分 + 能力画像</h2><div class="charts-grid dense-grid"><div><div id="chart-gauge" class="chart-sm"></div></div><div><div id="chart-capability" class="chart-sm"></div></div><div><div id="chart-severity" class="chart-sm"></div></div><div><div id="chart-priority" class="chart-sm"></div></div></div><div class="section-note">总览区固定展示全局摘要图；Tab 内聚焦专题图与明细表，避免重复堆图。</div></div>
    </div>
    <div class=\"card-sub\" style=\"margin:20px 0 14px;\">
      <h2>多维度评估（1 全局 + 4 业务 + 4 技术）</h2>
      <div class=\"tabs\">{''.join(tab_buttons)}</div>
      {''.join(tab_panels)}
    </div>
  </div>
  <script>
    const isDark = document.body.getAttribute('data-theme') === 'dark';
    const textColor = isDark ? '#e2e8f0' : '#1f2d3d';
    const splitColor = isDark ? '#334155' : '#e5e7eb';
    document.querySelectorAll('.tab-btn').forEach(btn => {{
      btn.addEventListener('click', () => {{
        document.querySelectorAll('.tab-btn').forEach(b => {{ b.classList.remove('active'); b.setAttribute('aria-selected','false'); }});
        document.querySelectorAll('.tab-panel').forEach(p => {{ p.classList.remove('active'); p.setAttribute('hidden',''); }});
        btn.classList.add('active');
        btn.setAttribute('aria-selected','true');
        const panel = document.getElementById('panel-' + btn.dataset.tab);
        if (panel) {{ panel.classList.add('active'); panel.removeAttribute('hidden'); }}
        document.querySelectorAll('#panel-' + btn.dataset.tab + ' .chart').forEach(el => {{ const c = echarts.getInstanceByDom(el); if (c) c.resize(); }});
      }});
    }});
    function initChart(id, option) {{ const el = document.getElementById(id); if (!el) return; const chart = echarts.init(el, isDark ? 'dark' : null); chart.setOption(option); return chart; }}
    initChart('chart-radar', {{ tooltip: {{}}, radar: {{ indicator: {radar_labels_js}.map(name => ({{name, max:100}})), axisName: {{ color: textColor }}, splitArea: {{ areaStyle: {{ color: isDark ? ['rgba(30,41,59,0.35)','rgba(15,23,42,0.3)'] : ['#f8fafc','#eef2ff'] }} }} }}, series: [{{ type:'radar', data:[{{ value:{radar_values_js}, name:'七维评分', areaStyle:{{opacity:.24}}, lineStyle:{{width:2,color:'#2563eb'}}, itemStyle:{{color:'#1d4ed8'}} }}] }}] }});
    initChart('chart-gauge', {{ series:[{{ type:'gauge', min:0, max:100, progress:{{show:true,width:18}}, axisLine:{{lineStyle:{{width:18}}}}, detail:{{valueAnimation:true, formatter:'{{value}} / 100', fontSize:24, offsetCenter:[0,'65%']}}, title:{{offsetCenter:[0,'92%'], fontSize:14}}, data:[{{ value:{overall_score:.1f}, name:'综合评分' }}] }}] }});
    initChart('chart-capability', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'value', max:10, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, yAxis: {{ type:'category', data:{capability_data_js}.map(d=>d.name), axisLabel:{{color:textColor}} }}, series:[{{ type:'bar', data:{capability_data_js}.map(d=>({{ value:d.value, itemStyle:{{ color:d.value>=7?'#10b981':d.value>=5?'#3b82f6':d.value>=3?'#f59e0b':'#ef4444' }} }})), label:{{show:true, position:'right', color:textColor}} }}] }});
    initChart('chart-severity', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'category', data:['critical','high','medium','low'], axisLabel:{{color:textColor}} }}, yAxis: {{ type:'value', minInterval:1, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, series:[{{ type:'bar', data:{severity_data_js}, itemStyle:{{ color:(p)=>['#ef4444','#f97316','#f59e0b','#22c55e'][p.dataIndex] }}, label:{{show:true, position:'top', color:textColor}} }}] }});
    initChart('chart-priority', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'category', data:['P0','P1','P2','P3'], axisLabel:{{color:textColor}} }}, yAxis: {{ type:'value', minInterval:1, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, series:[{{ type:'bar', data:{recommendation_priority_js}, itemStyle:{{ color:(p)=>['#ef4444','#f59e0b','#3b82f6','#94a3b8'][p.dataIndex] }}, label:{{show:true, position:'top', color:textColor}} }}] }});
    initChart('chart-completion', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'category', data:{completion_data_js}.map(d=>d.name), axisLabel:{{color:textColor}} }}, yAxis: {{ type:'value', max:100, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, series:[{{ type:'bar', data:{completion_data_js}.map(d=>({{ value:d.value, itemStyle:{{ color:d.value>=75?'#10b981':d.value>=50?'#f59e0b':'#ef4444' }} }})), label:{{show:true, position:'top', color:textColor}} }}] }});
    initChart('chart-connection', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'category', data:{connection_chart_js}.map(d=>d.name), axisLabel:{{color:textColor, rotate:25}} }}, yAxis: {{ type:'value', minInterval:1, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, series:[{{ type:'bar', data:{connection_chart_js}.map(d=>d.value), itemStyle:{{color:'#3b82f6'}}, label:{{show:true, position:'top', color:textColor}} }}] }});
    initChart('chart-sankey', {{ tooltip: {{ trigger:'item' }}, series:[{{ type:'sankey', data:{sankey_nodes_js}, links:{sankey_links_js}, lineStyle:{{ color:'gradient', curveness:.5 }}, label:{{ color:textColor }} }}] }});
    initChart('chart-logic', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'category', data:{logic_chart_js}.map(d=>d.name), axisLabel:{{color:textColor}} }}, yAxis: {{ type:'value', minInterval:1, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, series:[{{ type:'bar', data:{logic_chart_js}.map(d=>d.value), itemStyle:{{color:'#8b5cf6'}}, label:{{show:true, position:'top', color:textColor}} }}] }});
    initChart('chart-polar', {{ angleAxis: {{ type:'category', data:{polar_data_js}.map(d=>d.name), axisLabel:{{color:textColor}} }}, radiusAxis: {{ max:100, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, polar: {{}}, tooltip: {{ trigger:'axis' }}, series:[{{ type:'bar', coordinateSystem:'polar', data:{polar_data_js}.map(d=>({{ value:d.value, itemStyle:{{ color:d.value>=75?'#10b981':d.value>=50?'#f59e0b':'#ef4444' }} }})) }}] }});
    initChart('chart-dep-matrix', {{ tooltip: {{ formatter:(p)=>['架构','代码质量','技术债'][p.data[0]] + ' × ' + ['critical','high','medium','low'][p.data[1]] + ': ' + p.data[2] }}, xAxis: {{ type:'category', data:{json.dumps([_dimension_label(dim) for dim in heatmap_dims], ensure_ascii=False)}, axisLabel:{{color:textColor}} }}, yAxis: {{ type:'category', data:{json.dumps(heatmap_categories, ensure_ascii=False)}, axisLabel:{{color:textColor}} }}, visualMap: {{ min:0, max:Math.max(...{heatmap_data_js}.map(d=>d[2]), 1), calculable:true, orient:'horizontal', left:'center', inRange:{{ color:['#dbeafe','#60a5fa','#1d4ed8'] }}, textStyle:{{color:textColor}} }}, series:[{{ type:'heatmap', data:{heatmap_data_js}, label:{{show:true, color:textColor}} }}] }});
    initChart('chart-perf', {{ tooltip: {{ trigger:'axis' }}, xAxis: {{ type:'category', data:{perf_data_js}.map(d=>d.name), axisLabel:{{color:textColor}} }}, yAxis: {{ type:'value', max:100, axisLabel:{{color:textColor}}, splitLine:{{lineStyle:{{color:splitColor}}}} }}, series:[{{ type:'bar', data:{perf_data_js}.map(d=>({{ value:d.value, itemStyle:{{ color:d.value>=75?'#10b981':d.value>=50?'#f59e0b':'#ef4444' }} }})), label:{{show:true, position:'top', color:textColor}} }}] }});
    initChart('chart-rose', {{ tooltip: {{ trigger:'item' }}, series:[{{ type:'pie', roseType:'area', radius:['20%','65%'], data:{rose_data_js}, label:{{color:textColor}} }}] }});
    initChart('chart-treemap', {{ tooltip: {{ formatter:(p)=>p.name + ': ' + (p.value || '') }}, series:[{{ type:'treemap', roam:false, data:{treemap_data_js}, label:{{show:true, color:'#fff'}} }}] }});
    window.addEventListener('resize', () => {{ document.querySelectorAll('.chart').forEach(el => {{ const c = echarts.getInstanceByDom(el); if (c) c.resize(); }}); }});
  </script>
</body>
</html>"""


def generate_quantitative_section(review_input: dict) -> str:
    """生成量化数据章节（可插入评审报告）"""

    display = _resolve_display_summary(review_input)
    dimension_scores = display["dimension_scores"]
    summary_source = "scorecard.md" if display["uses_scorecard"] else "quantitative-input.json"

    content = f"""## 量化评分数据（来自 arc:score / arc:audit 汇总）

### 综合摘要

| 指标 | 值 |
|------|-----|
| **综合评分** | {display['overall_score']:.1f} / 100 |
| **综合评级** | {_display_rating(display['overall_rating'])} |
| **推进建议** | {_display_gate(display['gate'])} |
| **风险等级** | {display['risk']} |
| **数据优先源** | {summary_source} |

### 维度评分

| 维度 | 最终分数 | 问题数 |
|------|---------|--------|
"""

    for dim in ["architecture", "security", "code-quality", "business", "devops", "team", "tech-debt"]:
        issues_count = len(review_input.get("dimension_issues", {}).get(dim, []))
        content += f"| {_dimension_label(dim)} | {dimension_scores.get(dim, 0):.1f} | {issues_count} |\n"

    content += f"""
### 专项评分

| 指标 | 值 |
|------|-----|
| **Business Maturity** | {display['business_maturity']:.1f} / 10 |
| **Dependency Health** | {display['dependency_health']:.1f} / 10 |

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
    review_input["review_dir"] = str(review_path)

    # 写入 JSON
    output_path = review_path / "quantitative-input.json"
    output_path.write_text(
        json.dumps(review_input, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    # 写入 Markdown 章节
    display_summary = _resolve_display_summary(review_input)
    md_path = review_path / "quantitative-section.md"
    md_content = generate_quantitative_section(review_input)
    md_path.write_text(md_content, encoding="utf-8")

    # 写入 HTML 可视化看板（图表 + 多维度 Tab）
    # 仅输出一份：按执行时 time now() 自动选择浅/深主题
    html_auto_path = review_path / "quantitative-dashboard.html"
    html_auto_content = generate_html_dashboard(review_input)
    html_auto_path.write_text(html_auto_content, encoding="utf-8")

    print("✓ 量化数据已集成")
    print(f"  JSON: {output_path}")
    print(f"  Markdown: {md_path}")
    print(f"  HTML Dashboard: {html_auto_path}")

    # 打印摘要
    print("\n评分摘要:")
    print(
        f"  综合评分: {display_summary['overall_score']:.1f} / 100（{_display_rating(display_summary['overall_rating'])}）"
    )
    print(f"  推进建议: {_display_gate(display_summary['gate'])}")
    print(f"  风险等级: {display_summary['risk']} | 整改时限: {display_summary['sla']}")
    print("  维度评分:")
    for dim in ["architecture", "security", "code-quality", "business", "devops", "team", "tech-debt"]:
        score = display_summary["dimension_scores"].get(dim, 0)
        issues = len(review_input.get("dimension_issues", {}).get(dim, []))
        print(f"    - {_dimension_label(dim)}: {score:.1f} ({issues} issues)")


if __name__ == "__main__":
    main()
