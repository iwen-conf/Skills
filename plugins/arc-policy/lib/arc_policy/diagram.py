"""Read an Arc snapshot and render it with the bundled arch-diagram skill.

This is a presentation adapter. It never opens the ledger, runs a business
operation, confirms a rule, or recomputes a business verdict.
"""

from __future__ import annotations

import hashlib
import json
import re
import shutil
import subprocess
import tomllib
import unicodedata
from copy import deepcopy
from pathlib import Path
from typing import Any

from arc_policy.capabilities import check_pinned_skill
from arc_policy.paths import plugin_root

MAX_NODES = 16
MAX_EDGES = 32
MAX_BYTES = 64 << 20
VIEWS = {
    "capabilities": (
        {"system", "domain", "capability", "role"},
        {"contains", "serves"},
        "业务能力树",
    ),
    "flows": (
        {"capability", "action", "condition", "state", "rule"},
        {
            "implements",
            "requires",
            "forbids",
            "transitions",
            "transitions_to",
            "recovers",
            "next",
        },
        "业务流程图",
    ),
}
KINDS = {
    "system": ("系统", "blue"),
    "domain": ("业务域", "cyan"),
    "capability": ("业务能力", "purple"),
    "role": ("参与角色", "orange"),
    "action": ("业务操作", "blue"),
    "condition": ("前置条件", "amber"),
    "state": ("业务状态", "green"),
    "rule": ("业务规则", "purple"),
}
RELATIONS = {
    "contains": ("包含", "cyan"),
    "serves": ("参与", "orange"),
    "implements": ("实现业务", "blue"),
    "requires": ("前置条件", "amber"),
    "forbids": ("禁止条件", "red"),
    "transitions": ("产生状态", "green"),
    "transitions_to": ("状态迁移", "cyan"),
    "recovers": ("恢复条件", "purple"),
    "next": ("后续操作", "blue"),
}
OUTCOMES = {
    "success": "成功",
    "rejection": "拒绝",
    "retry": "重试",
    "compensation": "补偿",
    "error": "错误",
    "parallel": "并行",
    "join": "汇合",
}
STATUSES = {
    "PASS": ("done", "通过"),
    "FAIL": ("warn", "失败"),
    "BLOCKED": ("warn", "阻塞"),
    "NOT_RUN": ("none", "未验证"),
    "STALE": ("warn", "证据过期"),
    "FLAKY": ("warn", "结果波动"),
    "PARTIAL": ("dev", "部分完成"),
    "EXCLUDED": ("off", "已排除"),
}


class DiagramError(ValueError):
    """The supplied data cannot be rendered without losing its meaning."""


def _object(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise DiagramError(f"{name} must be an object")
    return value


def _items(value: Any, name: str) -> list[dict[str, Any]]:
    if not isinstance(value, list) or any(not isinstance(item, dict) for item in value):
        raise DiagramError(f"{name} must be an array of objects")
    return value


def _optional_items(value: Any, name: str) -> list[dict[str, Any]]:
    # Go serializes nil slices as null, including a candidate's empty RULE set.
    return _items([] if value is None else value, name)


def _text(value: Any, name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise DiagramError(f"{name} must be a nonempty string")
    return value


def _indexed(items: list[dict[str, Any]], name: str) -> dict[str, dict[str, Any]]:
    result = {}
    for item in items:
        identity = _text(item.get("id"), f"{name}.id")
        if identity in result:
            raise DiagramError(f"duplicate {name} id: {identity}")
        result[identity] = item
    return result


def _chinese(text: str) -> bool:
    return bool(re.search(r"[\u3400-\u9fff]", text))


def _display(text: Any) -> str:
    # Text remains data, including arch-diagram's own inline-markup syntax.
    return str(text).replace("{", "｛").replace("}", "｝")


def _wrap(text: Any, width: int = 48) -> str:
    lines, line, size = [], "", 0
    for char in _display(text):
        cells = 2 if unicodedata.east_asian_width(char) in "WF" else 1
        if char == "\n" or size + cells > width:
            lines.append(line)
            line, size = "", 0
        if char != "\n":
            line += char
            size += cells
    lines.append(line)
    return "\n".join(lines)


def _presentation(value: Any, graph: dict[str, Any], graph_hash: str) -> dict[str, Any]:
    if value is None:
        return {}
    value = _object(value, "presentation")
    if set(value) - {"graph", "graph_sha256", "title_zh", "node_ids", "nodes", "edges"}:
        raise DiagramError(
            "presentation accepts labels, package evidence and node selection only"
        )
    if value.get("graph") != graph["id"] or value.get("graph_sha256") != graph_hash:
        raise DiagramError("presentation graph/hash does not match the snapshot")
    for key in ("nodes", "edges"):
        _object(value.get(key, {}), f"presentation.{key}")
    for identity, item in value.get("nodes", {}).items():
        _object(item, f"presentation.nodes.{identity}")
        if set(item) - {"name_zh", "description_zh", "packages"}:
            raise DiagramError(
                f"presentation node {identity} may not change source facts or verdicts"
            )
        for field in ("name_zh", "description_zh"):
            if field in item and not _chinese(_text(item[field], field)):
                raise DiagramError(
                    f"{identity}.{field} must contain a Chinese business label"
                )
        if any(char in item.get("name_zh", "") for char in "【】"):
            raise DiagramError(
                "put package identifiers in packages, not in the Chinese name"
            )
        for package in _items(item.get("packages", []), f"{identity}.packages"):
            if set(package) != {"name", "file", "line"}:
                raise DiagramError("package evidence needs exactly name, file and line")
            if not re.fullmatch(
                r"[A-Za-z0-9_./:@+-]+", str(package["name"])
            ) or not re.search(r"[A-Za-z]", str(package["name"])):
                raise DiagramError(
                    "package name must be an actual English code identifier"
                )
            _text(package["file"], "package.file")
            if type(package["line"]) is not int or package["line"] < 1:
                raise DiagramError("package.line must be a positive integer")
    for identity, item in value.get("edges", {}).items():
        if (
            not isinstance(item, dict)
            or set(item) != {"label_zh"}
            or not _chinese(_text(item.get("label_zh"), "label_zh"))
        ):
            raise DiagramError(
                f"presentation edge {identity} accepts a Chinese label_zh only"
            )
    if "title_zh" in value and not _chinese(_text(value["title_zh"], "title_zh")):
        raise DiagramError("title_zh must contain Chinese")
    return value


def _package_facts(
    presentation: dict[str, Any], root: Path | None
) -> tuple[dict[str, list[dict[str, Any]]], list[str]]:
    """Check local package evidence; never trust a claimed 'verified' map field."""
    facts, warnings = {}, []
    base = root.resolve() if root is not None else None
    for identity, node in presentation.get("nodes", {}).items():
        verified = []
        for item in node.get("packages", []):
            try:
                if base is None:
                    raise ValueError("未提供源码根目录")
                relative = Path(item["file"])
                if relative.is_absolute() or ".." in relative.parts:
                    raise ValueError("证据必须是源码目录内的相对路径")
                path = (base / relative).resolve()
                if (
                    not path.is_relative_to(base)
                    or not path.is_file()
                    or path.stat().st_size > 4 << 20
                ):
                    raise ValueError("证据文件缺失、越界或超过 4 MiB")
                body = path.read_bytes()
                lines = body.decode("utf-8").splitlines()
                if item["line"] > len(lines):
                    raise ValueError("证据行不存在")
                line = lines[item["line"] - 1]
                name = item["name"]
                kind = (
                    "directory"
                    if name in relative.parts[:-1]
                    else _package_declaration(path, body, line, name)
                )
                if not kind:
                    raise ValueError(
                        "无法从实际目录、包声明或包配置确认该名称；普通 API／表名不算包名"
                    )
                # Keep only the cited location and hash, never a whole manifest
                # line that may also contain unrelated credentials or settings.
                verified.append(
                    {
                        **item,
                        "file_sha256": hashlib.sha256(body).hexdigest(),
                        "kind": kind,
                    }
                )
            except (OSError, UnicodeError, ValueError) as error:
                warnings.append(f"{identity}：业务包待核实（{error}）")
        # A partially checked multi-package claim stays pending as a whole.
        if verified and len(verified) == len(node.get("packages", [])):
            facts[identity] = verified
    return facts, warnings


def _package_declaration(path: Path, body: bytes, line: str, name: str) -> str:
    token = re.escape(name)
    if re.match(rf"^\s*(?:package|namespace)\s+{token}(?:\s*[;{{]|\s*$)", line):
        return "declaration"
    if path.name == "go.mod" and re.match(rf"^\s*module\s+{token}\s*(?://.*)?$", line):
        return "manifest"
    if path.name in {"package.json", "composer.json"}:
        document = json.loads(body)
        if (
            isinstance(document, dict)
            and document.get("name") == name
            and re.search(rf'"name"\s*:\s*"{token}"', line)
        ):
            return "manifest"
    if path.name in {"pyproject.toml", "Cargo.toml"}:
        document = tomllib.loads(body.decode("utf-8"))
        candidates = [
            document.get("project", {}).get("name"),
            document.get("package", {}).get("name"),
            document.get("tool", {}).get("poetry", {}).get("name"),
        ]
        if name in candidates and re.match(rf"^\s*name\s*=\s*['\"]{token}['\"]", line):
            return "manifest"
    return ""


def _logic_notes(
    graph: dict[str, Any], names: dict[str, str], selected: set[str]
) -> tuple[list[str], list[str]]:
    guards = {
        item.get("rule"): item
        for item in _optional_items(graph.get("guards"), "guards")
    }

    def literal(item: dict[str, Any]) -> str:
        if type(item.get("value")) is not bool:
            raise DiagramError("condition literal requires an explicit boolean")
        identity = _text(item.get("condition"), "literal.condition")
        return (
            f"{names.get(identity, identity)}={'满足' if item['value'] else '不满足'}"
        )

    candidates = []
    for rule in _optional_items(graph.get("rules"), "rules"):
        if rule["id"] not in selected and rule.get("action") not in selected:
            continue
        guard = guards.get(rule["id"])
        if guard is not None:
            clauses = guard.get("any_of")
            if not isinstance(clauses, list) or not clauses:
                raise DiagramError(
                    "guard.any_of must contain explicit condition clauses"
                )
            parts = []
            for clause in clauses:
                values = _items(clause, "guard clause")
                parts.append(
                    "（"
                    + (
                        " 且 ".join(literal(item) for item in values)
                        if values
                        else "恒真"
                    )
                    + "）"
                )
            formula = " 或 ".join(parts)
        else:
            conditions = rule.get("conditions") or []
            formula = (
                " 且 ".join(
                    f"{names.get(identity, identity)}=满足" for identity in conditions
                )
                or "未声明前置条件"
            )
        candidates.append((rule["id"], f"规则 {rule['id']} 的允许条件：{formula}"))
    for clause in _optional_items(graph.get("constraints"), "constraints"):
        values = _items(clause.get("literals"), "constraint.literals")
        if any(item.get("condition") in selected for item in values):
            candidates.append(
                (
                    clause["id"],
                    f"约束 {clause['id']}（各约束须同时成立）："
                    + " 或 ".join(literal(item) for item in values),
                )
            )
    notes, omitted = [], []
    for identity, text in candidates:
        if len(notes) >= 16 or len(text) > 600:
            omitted.append(identity)
        else:
            notes.append(text)
    if omitted:
        notes.append(
            f"还有 {len(omitted)} 条较长或超出当前范围的逻辑说明，完整 guards／constraints 及源 ID 保留在 spec；没有简化 AND／OR。"
        )
    return notes, omitted


def _exploration_notes(value: Any) -> list[str]:
    if value is None:
        return []
    result = []
    for identity, raw in _object(value, "exploration").items():
        coverage = _object(raw, f"exploration.{identity}")
        result.append(
            f"探索 {identity}：{coverage.get('business_verdict') or 'NOT_RUN'}；"
            f"证据信任：{coverage.get('evidence_trust') or '未声明'}；可信：{'是' if coverage.get('trusted') is True else '未证实'}。"
        )
        result.append(
            f"人工确认 {len(coverage.get('human_confirmed') or [])} 项；AI 推断 {len(coverage.get('ai_inferred') or [])} 项；"
            f"AI 候选 {len(coverage.get('ai_candidates') or [])} 项；"
            f"UNSPECIFIED 未定义 {len(coverage.get('unspecified') or [])} 项；"
            f"确认失败 {len(coverage.get('confirmed_fails') or [])} 项。"
        )
        if coverage.get("first_divergence"):
            result.append("探索首次偏离：" + str(coverage["first_divergence"]))
        if (
            coverage.get("budget_exhausted")
            or coverage.get("pending_recovery")
            or coverage.get("evidence_missing")
        ):
            result.append("探索仍有预算、恢复或证据缺口；不是完整业务通过。")
    return result


def _rows(
    nodes: list[dict[str, Any]], edges: list[dict[str, Any]], view: str
) -> list[list[str]]:
    """Small topological rows; cycles retain stable IDs and use outside routes."""
    if view == "flows":
        rows = []
        # These are semantic bands, not an invented execution sequence.
        for kinds in ({"capability", "rule"}, {"condition"}, {"action"}, {"state"}):
            band = sorted(node["id"] for node in nodes if node["kind"] in kinds)
            rows.extend(band[start : start + 2] for start in range(0, len(band), 2))
        return rows
    remaining = {node["id"] for node in nodes}
    predecessors = {identity: set() for identity in remaining}
    for edge in edges:
        if edge["from"] != edge["to"]:
            predecessors[edge["to"]].add(edge["from"])
    rows = []
    while remaining:
        ready = sorted(
            identity for identity in remaining if not predecessors[identity] & remaining
        )
        if not ready:
            ready = [min(remaining)]
        rows.extend(ready[start : start + 2] for start in range(0, len(ready), 2))
        remaining.difference_update(ready)
    return rows


def _run_summaries(runs: list[dict[str, Any]]) -> list[dict[str, Any]]:
    result = []
    for run in runs:
        trace = _object(run.get("trace"), "runs.trace")
        evidence = _items(run.get("evidence", []), "runs.evidence")
        summary = {
            key: deepcopy(trace[key])
            for key in (
                "run",
                "task",
                "status",
                "round",
                "first_divergence",
                "latest_observed",
                "continue_decision",
                "evidence_status",
                "recovery",
            )
            if key in trace
        }
        summary["steps"] = [
            {
                key: deepcopy(step[key])
                for key in (
                    "step",
                    "node",
                    "transition",
                    "verdict",
                    "expected",
                    "observed",
                    "decision",
                    "raw",
                    "sha256",
                    "evidence_status",
                )
                if key in step
            }
            for step in _items(trace.get("steps", []), "trace.steps")
        ]
        summary["evidence"] = []
        for item in evidence:
            record = {
                key: deepcopy(item[key])
                for key in ("run", "phase", "step", "sha256", "error")
                if key in item
            }
            if isinstance(item.get("data"), dict):
                record["identity"] = deepcopy(item["data"].get("identity", {}))
            summary["evidence"].append(record)
        result.append(summary)
    return result


def snapshot_to_spec(
    snapshot: dict[str, Any],
    presentation: dict[str, Any] | None = None,
    view: str = "flows",
    *,
    package_facts: dict[str, list[dict[str, Any]]] | None = None,
) -> dict[str, Any]:
    """Pure conversion; package_facts must come from local evidence verification.

    With no verified package facts, every business title keeps 业务包待核实.
    The original snapshot and presentation dictionaries are never modified.
    """
    if view not in VIEWS:
        raise DiagramError("view must be capabilities or flows")
    snapshot = _object(snapshot, "snapshot")
    meta = _object(snapshot.get("meta"), "meta")
    model = _object(snapshot.get("model"), "model")
    graph = _object(model.get("data"), "model.data")
    graph_id = _text(graph.get("id"), "graph.id")
    graph_hash = _text(meta.get("graph_sha256"), "meta.graph_sha256")
    if (
        graph.get("schema_version") != 2
        or meta.get("graph") != graph_id
        or model.get("sha256") != graph_hash
    ):
        raise DiagramError(
            "snapshot graph identity, v2 model and fingerprints must agree"
        )
    if not re.fullmatch(r"[0-9a-f]{64}", graph_hash):
        raise DiagramError("snapshot graph fingerprint must be a SHA-256")
    _text(meta.get("generated"), "meta.generated")
    if type(model.get("approved")) is not bool:
        raise DiagramError("model.approved must be an explicit boolean")
    approvals = _object(model.get("rules"), "model.rules")
    if any(type(value) is not bool for value in approvals.values()):
        raise DiagramError("rule approvals must be explicit booleans")
    projection = _object(
        _object(snapshot.get("views"), "views").get(view), f"views.{view}"
    )
    verdict = projection.get("verdict")
    if (
        projection.get("graph") != graph_id
        or projection.get("view") != view
        or verdict not in STATUSES
    ):
        raise DiagramError("view graph, name or business verdict is invalid")
    if verdict == "PASS" and not model["approved"]:
        raise DiagramError("an unapproved candidate cannot carry a business PASS")
    for field in ("environment", "round"):
        if (projection.get(field) or "") != (meta.get(field) or ""):
            raise DiagramError(
                f"snapshot meta and selected view have different {field}"
            )
    context = _object(projection.get("context", {}), "view.context")
    for field, expected in (
        ("graph", graph_id),
        ("environment", meta.get("environment") or ""),
    ):
        if context.get(field) and context[field] != expected:
            raise DiagramError(f"selected view proof context has a different {field}")
    stale_context = bool(
        context.get("graph_sha256") and context["graph_sha256"] != graph_hash
    )
    labels = _presentation(presentation, graph, graph_hash)
    source_nodes = _indexed(_items(graph.get("nodes"), "model.nodes"), "node")
    rules = _indexed(_optional_items(graph.get("rules"), "model.rules"), "rule")
    if set(source_nodes) & set(rules):
        raise DiagramError("rule and node identities overlap")
    source_nodes.update(
        {
            identity: {
                "id": identity,
                "kind": "rule",
                "name": rule.get("description", identity),
            }
            for identity, rule in rules.items()
        }
    )
    source_edges = _indexed(_optional_items(graph.get("edges"), "model.edges"), "edge")
    if set(labels.get("nodes", {})) - set(source_nodes) or set(
        labels.get("edges", {})
    ) - set(source_edges):
        raise DiagramError("presentation references an unknown canonical node or edge")
    kinds, relations, heading = VIEWS[view]
    eligible = {
        identity: node
        for identity, node in source_nodes.items()
        if node.get("kind") in kinds
    }
    selection = labels.get("node_ids")
    if selection is None:
        selected = sorted(eligible)
    else:
        if (
            not isinstance(selection, list)
            or any(not isinstance(identity, str) for identity in selection)
            or len(selection) != len(set(selection))
        ):
            raise DiagramError(
                "presentation.node_ids must be a unique list of canonical IDs"
            )
        selected = selection[:]
        if set(selected) - set(eligible):
            raise DiagramError(
                "node selection includes an identity outside this business view"
            )
    if not selected or len(selected) > MAX_NODES:
        raise DiagramError(
            f"view has {len(selected)} nodes; select 1–{MAX_NODES} explicit presentation.node_ids and retain omitted scope"
        )
    selected_set = set(selected)
    eligible_edges = []
    for edge in source_edges.values():
        if (
            edge.get("kind") in relations
            and edge.get("from") in eligible
            and edge.get("to") in eligible
        ):
            eligible_edges.append(edge)
    edges = sorted(
        (
            edge
            for edge in eligible_edges
            if edge["from"] in selected_set and edge["to"] in selected_set
        ),
        key=lambda edge: edge["id"],
    )
    if len(edges) > MAX_EDGES:
        raise DiagramError(
            f"view has {len(edges)} edges; narrow presentation.node_ids to at most {MAX_EDGES} relationships"
        )
    projected = _indexed(
        _items(projection.get("nodes"), "view.nodes"), "projected node"
    )
    if set(projected) - set(source_nodes):
        raise DiagramError("view contains a node absent from the canonical model")
    if projection.get("truncated") and selection is None:
        raise DiagramError(
            "snapshot view is truncated; select explicit presentation.node_ids and retain the missing scope"
        )
    contracts = {
        item.get("subject"): item
        for item in _optional_items(graph.get("contracts"), "contracts")
    }
    transitions = {
        item.get("edge"): item
        for item in _optional_items(graph.get("transitions"), "transitions")
    }
    warnings, names, render_nodes, render_ids = [], {}, {}, {}
    package_facts = package_facts or {}
    candidate_nodes = []
    for index, identity in enumerate(selected):
        node = eligible[identity]
        label = labels.get("nodes", {}).get(identity, {})
        source_name = _text(node.get("name"), f"{identity}.name")
        name = label.get("name_zh", source_name.split("【", 1)[0])
        if not _chinese(name):
            name = f"{KINDS[node['kind']][0]}名称待核实"
            warnings.append(f"{identity}：缺少中文业务名称；原名保存在来源映射中")
        names[identity] = name
        packages = package_facts.get(identity, [])
        package_label = (
            "、".join(item["name"] for item in packages) if packages else "业务包待核实"
        )
        state = projected.get(identity, {})
        status = state.get("status", "NOT_RUN")
        if status not in STATUSES:
            raise DiagramError(f"unknown node status for {identity}: {status}")
        unconfirmed = (
            not model["approved"]
            or bool(contracts.get(identity, {}).get("unconfirmed"))
            or identity in rules
            and approvals.get(identity) is not True
        )
        if status == "PASS" and unconfirmed:
            raise DiagramError(
                f"unconfirmed node {identity} cannot carry a business PASS"
            )
        if stale_context and (status == "PASS" or verdict == "PASS"):
            raise DiagramError(
                "a frozen proof context with a different graph fingerprint cannot carry PASS"
            )
        if unconfirmed:
            candidate_nodes.append(identity)
        badge, status_zh = STATUSES[status]
        description = label.get("description_zh", "")
        lines = [
            f"历史 FAIL：{state.get('history_failures', 0)}　通过义务：{state.get('passed', 0)}/{state.get('required', 0)}"
        ]
        if description:
            lines.append(description)
        if identity in rules:
            lines.append(
                f"RULE：{identity} · {'已确认' if approvals.get(identity) else '待确认'}"
            )
        elif node["kind"] == "action":
            own_rules = [
                rule_id
                for rule_id, rule in rules.items()
                if rule.get("action") == identity
            ]
            if own_rules:
                lines.append("关联 RULE：" + "、".join(own_rules))
        if packages:
            lines.append(
                "包名来源："
                + "；".join(f"{item['file']}:{item['line']}" for item in packages)
            )
        if identity not in projected:
            lines.append("快照未导出此节点的执行状态")
        renderer_id = f"n{index + 1:03d}"
        render_ids[identity] = renderer_id
        render_nodes[identity] = {
            "id": renderer_id,
            "layer": node["kind"],
            "w": 420,
            "title": _wrap(f"{name}【{package_label}】"),
            "sub": KINDS[node["kind"]][0] + (" · 候选／待确认" if unconfirmed else ""),
            "lines": [_wrap(line) for line in lines],
            "footer": _wrap(f"源 ID：{identity}"),
            "status": badge,
            "statusText": f"{status_zh} · {status}",
            "sourceId": identity,
        }
    for identity, rule in rules.items():
        if identity in render_nodes:
            expected = names.get(rule.get("expected"), rule.get("expected", "待核实"))
            rejected = (
                "保持原状"
                if rule.get("rejected") == "UNCHANGED"
                else names.get(rule.get("rejected"), rule.get("rejected", "待核实"))
            )
            render_nodes[identity]["lines"].append(
                _wrap(f"允许后：{expected}；拒绝后：{rejected}")
            )
    for flow in _optional_items(graph.get("flows"), "flows"):
        if flow.get("initial") in render_nodes:
            render_nodes[flow["initial"]]["sub"] += " · 初始状态"
    rows = _rows([eligible[identity] for identity in selected], edges, view)
    header = f"快照结论：{STATUSES[verdict][1]} · {verdict}（导出时）｜历史 FAIL：{projection.get('history_failures', 0)}"
    scope = str(graph.get("scope", ""))
    fixture = bool(
        re.search(
            r"(?:reference|synthetic|isolated).*fixture|not .*product contract|tool acceptance",
            scope,
            re.IGNORECASE,
        )
    )
    provenance_label = (
        "工具示例／合成夹具；不代表真实产品验证"
        if fixture
        else "快照记录；未重新执行业务验证"
    )
    subtitle = _wrap(
        f"{graph_id} · {'候选业务模型，尚未确认' if not model['approved'] else '业务目录已确认'}\n{header}\n{provenance_label}\n只读关系图；图形检查不提供业务执行证明。",
        132,
    )
    y = 205 + 21 * subtitle.count("\n")
    row_bounds, positions = [], {}
    for row_index, row in enumerate(rows):
        heights = []
        for column, identity in enumerate(row):
            node = render_nodes[identity]
            node["x"], node["y"] = 36 + column * 476, y
            text_lines = (
                1
                + node["title"].count("\n")
                + 1
                + sum(1 + line.count("\n") for line in node["lines"])
                + 1
                + node["footer"].count("\n")
            )
            node["h"] = 52 + text_lines * 22
            heights.append(node["h"])
            positions[identity] = row_index
        height = max(heights)
        # Equal row heights make all explicit edge routing stay in open gutters.
        for identity in row:
            render_nodes[identity]["h"] = height
        row_bounds.append((y, y + height))
        outgoing = sum(edge["from"] in row for edge in edges)
        y += height + max(132, 52 + outgoing * 22)
    render_edges, relation_notes, edge_ids = [], [], {}
    row_slots: dict[int, int] = {}
    for index, edge in enumerate(edges):
        start, end = render_nodes[edge["from"]], render_nodes[edge["to"]]
        start_row, end_row = positions[edge["from"]], positions[edge["to"]]
        lane = 1100 + index * 3
        slot = row_slots.get(start_row, 0)
        row_slots[start_row] = slot + 1
        offset = 28 + slot * 22
        source_y = row_bounds[start_row][1] + offset
        same_row = start_row == end_row
        target_y = (
            row_bounds[end_row][1] + offset if same_row else row_bounds[end_row][0] - 24
        )
        source_x, target_x = start["x"] + 210, end["x"] + 210
        transition = transitions.get(edge["id"], {})
        label = (
            labels.get("edges", {})
            .get(edge["id"], {})
            .get("label_zh", RELATIONS[edge["kind"]][0])
        )
        details = []
        if transition:
            details.append(
                OUTCOMES.get(
                    transition.get("outcome"), str(transition.get("outcome", ""))
                )
            )
            if transition.get("rule"):
                details.append("RULE " + transition["rule"])
            conditions = transition.get("preconditions", {})
            details.extend(
                f"{names.get(condition, condition)}={'满足' if value else '不满足'}"
                for condition, value in conditions.items()
            )
        renderer_id = f"e{index + 1:03d}"
        edge_ids[renderer_id] = edge["id"]
        from_anchor, to_anchor = (
            start["id"] + ".b",
            end["id"] + (".b" if same_row else ".t"),
        )
        if edge["from"] == edge["to"]:
            from_anchor, to_anchor = start["id"] + ".b@0.25", end["id"] + ".b@0.75"
            source_x, target_x = start["x"] + 105, end["x"] + 315
        if same_row or end_row == start_row + 1:
            via = [[source_x, source_y], [target_x, source_y]]
            label_position = [
                (source_x + target_x) / 2 + (70 if source_x == target_x else 0),
                source_y - 5,
            ]
        else:
            via = [
                [source_x, source_y],
                [lane, source_y],
                [lane, target_y],
                [target_x, target_y],
            ]
            label_position = [1035, source_y - 5]
        render_edges.append(
            {
                "id": renderer_id,
                "sourceId": edge["id"],
                "from": from_anchor,
                "to": to_anchor,
                "flow": edge["kind"],
                "packet": False,
                "via": via,
                "label": f"{index + 1} {RELATIONS[edge['kind']][0]}",
                "labelPos": label_position,
            }
        )
        relation_notes.append(
            f"{index + 1}. {names[edge['from']]} → {names[edge['to']]}：{label}"
            + ("；" + "；".join(details) if details else "")
            + f"\n   源边：{edge['id']}"
        )
    runs = _run_summaries(_items(snapshot.get("runs", []), "runs"))
    risks = deepcopy(_optional_items(graph.get("risks"), "risks"))
    omitted_nodes = sorted(set(eligible) - selected_set)
    omitted_edges = [edge["id"] for edge in eligible_edges if edge not in edges]
    if omitted_nodes or omitted_edges:
        warnings.append(
            f"显式局部范围：省略 {len(omitted_nodes)} 个节点、{len(omitted_edges)} 条关系；源 ID 见 spec"
        )
    if projection.get("truncated"):
        warnings.append("来源投影已截断；未导出的节点状态仍为未验证")
    if stale_context:
        warnings.append(
            "轮次证据绑定旧图指纹：上下文已过期；未声明当前通过，原始上下文继续保留"
        )
    evidence_errors = [
        item for run in runs for item in run["evidence"] if item.get("error")
    ]
    summary = [
        f"图版本：{graph_id}；导出时间：{meta['generated']}",
        f"原始业务范围：{scope}",
        f"图指纹：{graph_hash}",
        f"环境：{meta.get('environment') or '未设置'}；轮次：{meta.get('round') or '未设置'}",
        f"显示节点 {len(selected)}/{len(eligible)}，关系 {len(edges)}/{len(eligible_edges)}；模型 RISK {len(risks)} 项；候选节点 {len(candidate_nodes)} 个。",
        f"已收录 RAW 的运行 {len(runs)} 个；RAW 校验错误 {len(evidence_errors)} 项。证据完整性 PASS 不等于业务 PASS。",
    ]
    for run in runs:
        summary.append(
            f"运行 {run.get('run', '未知')}：{run.get('status', '未知')}；首次偏离：{run.get('first_divergence') or '未记录'}"
        )
    if risks:
        summary.append(
            "风险 ID：" + "、".join(str(risk.get("id", "未知")) for risk in risks)
        )
    if context.get("graph_sha256"):
        summary.append("轮次证据图指纹：" + str(context["graph_sha256"]))
    logic_notes, omitted_logic = _logic_notes(graph, names, selected_set)
    summary.extend(_exploration_notes(snapshot.get("exploration")))
    summary.extend(warnings)
    notes = []
    for title, lines in (
        ("业务关系与分支", relation_notes),
        ("规则与条件逻辑", logic_notes),
        ("快照来源与验证边界", summary),
    ):
        if not lines:
            continue
        content = _wrap(title + "\n" + "\n".join(lines), 142)
        notes.append({"x": 36, "y": y, "w": 1140, "text": content, "align": "left"})
        y += (content.count("\n") + 1) * 20 + 28
    return {
        "lang": "zh-CN",
        "width": 1240,
        "title": _wrap(labels.get("title_zh", heading), 132),
        "pageTitle": heading + " · " + graph_id,
        "subtitle": subtitle,
        "layers": [
            {"id": kind, "label": KINDS[kind][0], "color": KINDS[kind][1]}
            for kind in KINDS
            if any(eligible[identity]["kind"] == kind for identity in selected)
        ],
        "flows": [
            {"id": kind, "label": RELATIONS[kind][0], "color": RELATIONS[kind][1]}
            for kind in RELATIONS
            if any(edge["kind"] == kind for edge in edges)
        ],
        "nodes": list(render_nodes.values()),
        "edges": render_edges,
        "notes": notes,
        "wireLegend": False,
        "statusLabels": {
            "done": "快照记录通过",
            "warn": "失败／阻塞／过期／波动",
            "none": "尚未验证",
            "dev": "部分完成",
            "off": "已排除",
        },
        "footer": {
            "note": "静态只读；不执行、批准或重试。业务结论仅转述快照导出时的记录；包名定位不构成测试证明。"
        },
        "arc": {
            "read_only": True,
            "view": view,
            "meta": deepcopy(meta),
            "scope": scope,
            "fixture": fixture,
            "previous": graph.get("previous", ""),
            "model_approved": model["approved"],
            "rule_approvals": deepcopy(approvals),
            "candidate_nodes": candidate_nodes,
            "business_verdict": verdict,
            "business_reason": projection.get("reason", ""),
            "context": deepcopy(projection.get("context", {})),
            "history_failures": projection.get("history_failures", 0),
            "coverage": deepcopy(projection.get("coverage", {})),
            "diagnostics": deepcopy(projection.get("diagnostics", [])),
            "node_ids": {
                renderer: identity for identity, renderer in render_ids.items()
            },
            "edge_ids": edge_ids,
            "source_nodes": [deepcopy(source_nodes[identity]) for identity in selected],
            "node_status": {
                identity: deepcopy(projected.get(identity, {})) for identity in selected
            },
            "source_edges": deepcopy(edges),
            "rules": deepcopy(list(rules.values())),
            "transitions": deepcopy(graph.get("transitions", [])),
            "flows": deepcopy(graph.get("flows", [])),
            "business_logic": {
                key: deepcopy(graph.get(key))
                for key in (
                    "guards",
                    "constraints",
                    "invariants",
                    "applicability",
                    "tests",
                    "contracts",
                    "shared_owners",
                    "history",
                    "equivalence",
                    "reference_bindings",
                )
                if key in graph
            },
            "risks": risks,
            "runs": runs,
            "exploration": deepcopy(snapshot.get("exploration", {})),
            "package_evidence": deepcopy(package_facts),
            "omitted": {
                "nodes": omitted_nodes,
                "edges": omitted_edges,
                "logic": omitted_logic,
                "projection_truncated": bool(projection.get("truncated")),
                "projection_total": projection.get("total"),
            },
            "warnings": warnings,
        },
    }


def _load_json(path: Path) -> tuple[dict[str, Any], str]:
    def unique(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        value = {}
        for key, item in pairs:
            if key in value:
                raise DiagramError(f"duplicate JSON field: {key}")
            value[key] = item
        return value

    if not path.is_file() or not 0 < path.stat().st_size <= MAX_BYTES:
        raise DiagramError("JSON input must be a nonempty file no larger than 64 MiB")
    body = path.read_bytes()
    if not 0 < len(body) <= MAX_BYTES:
        raise DiagramError("JSON input changed size while reading; retry its export")
    return (
        _object(json.loads(body, object_pairs_hook=unique), str(path)),
        hashlib.sha256(body).hexdigest(),
    )


def _run(argv: list[str]) -> dict[str, Any]:
    try:
        completed = subprocess.run(
            argv, capture_output=True, text=True, timeout=60, check=False
        )
        return {
            "argv": argv,
            "exit_code": completed.returncode,
            "stdout": completed.stdout[-12000:],
            "stderr": completed.stderr[-12000:],
        }
    except (OSError, subprocess.TimeoutExpired) as error:
        return {"argv": argv, "exit_code": None, "stderr": str(error)}


def build_diagram(
    snapshot: Path,
    out: Path,
    presentation: Path | None = None,
    view: str = "flows",
    root: Path | None = None,
    check: bool = True,
    *,
    skill_dir: Path | None = None,
) -> dict[str, Any]:
    """Write a reviewable spec/HTML, retaining them when browser checks fail.

    out may be an HTML path or an output directory. Existing output artifacts
    are never overwritten. READY describes the diagram only, never the business.
    """
    result: dict[str, Any] = {
        "status": "BLOCKED",
        "read_only": True,
        "artifacts": {},
        "readiness": {
            "spec": "NOT_RUN",
            "build": "NOT_RUN",
            "layout_check": "NOT_RUN",
            "screenshot": "NOT_RUN",
            "visual_review": "NOT_RUN",
        },
    }
    try:
        snapshot = Path(snapshot).resolve()
        data, snapshot_digest = _load_json(snapshot)
        labels = _load_json(Path(presentation))[0] if presentation is not None else None
        # Validate presentation before consulting its filesystem references.
        spec = snapshot_to_spec(data, labels, view)
        packages, warnings = _package_facts(labels or {}, root)
        if packages:
            spec = snapshot_to_spec(data, labels, view, package_facts=packages)
        spec["arc"]["warnings"].extend(warnings)
        spec["arc"]["snapshot"] = {
            "path": str(snapshot),
            "sha256": snapshot_digest,
        }
        if warnings:
            spec["footer"]["note"] += "\n" + _wrap("；".join(warnings), 142)
        result["business_verdict"] = {
            "recorded": spec["arc"]["business_verdict"],
            "as_of": data["meta"]["generated"],
            "source": str(snapshot),
            "reverified": False,
        }
        result["source"] = deepcopy(spec["arc"]["meta"])
        result["warnings"] = spec["arc"]["warnings"]
        out = Path(out).resolve()
        html = out if out.suffix.lower() in {".html", ".htm"} else out / f"{view}.html"
        spec_path = html.with_suffix(".spec.json")
        screenshot = html.with_suffix(".check.png")
        if any(path.exists() for path in (html, spec_path, screenshot)):
            raise DiagramError(
                "diagram outputs already exist; choose a new output path"
            )
        html.parent.mkdir(parents=True, exist_ok=True)
        spec_path.write_text(
            json.dumps(spec, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        result["artifacts"]["spec"] = str(spec_path)
        result["readiness"]["spec"] = "READY"
        directory = (skill_dir or plugin_root() / "skills" / "arch-diagram").resolve()
        manifest = check_pinned_skill(directory)
        result["renderer"] = {
            "directory": str(directory),
            **{key: manifest.get(key) for key in ("repository", "version", "commit")},
        }
        # The official build embeds this spec, so standalone HTML retains the
        # reviewed renderer identity and its complete original license notice.
        spec["arc"]["renderer"] = {
            **{key: manifest[key] for key in ("repository", "version", "commit")},
            "license": "MIT",
            "license_text": (directory / "LICENSE").read_text(encoding="utf-8"),
        }
        spec_path.write_text(
            json.dumps(spec, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        runtime = shutil.which("node") or shutil.which("bun")
        if runtime is None:
            raise DiagramError("arch-diagram build requires Bun or Node.js")
        build = _run(
            [runtime, str(directory / "scripts/build.mjs"), str(spec_path), str(html)]
        )
        result["build"] = build
        if build["exit_code"] != 0 or not html.is_file():
            result["readiness"]["build"] = "BLOCKED"
            raise DiagramError("upstream arch-diagram build failed; see build output")
        result["artifacts"]["html"] = str(html)
        result["readiness"]["build"] = "READY"
        if not check:
            result["status"] = "NOT_RUN"
            result["reason"] = (
                "HTML built; layout and screenshot checks were explicitly not run"
            )
            return result
        checked = _run(
            [runtime, str(directory / "scripts/check.mjs"), str(html), str(screenshot)]
        )
        result["check"] = checked
        png_ok = (
            screenshot.is_file() and screenshot.read_bytes()[:8] == b"\x89PNG\r\n\x1a\n"
        )
        if png_ok:
            result["artifacts"]["screenshot"] = str(screenshot)
            result["readiness"]["screenshot"] = "READY"
        if checked["exit_code"] == 0 and png_ok:
            result["status"] = "READY"
            result["readiness"]["layout_check"] = "PASS"
            result["reason"] = (
                "upstream layout check passed; screenshot still needs visual inspection"
            )
        else:
            result["readiness"]["layout_check"] = (
                "FAIL" if checked["exit_code"] == 1 else "BLOCKED"
            )
            result["reason"] = (
                "upstream layout check could not complete cleanly; retain spec/HTML and inspect check output"
            )
        return result
    except (OSError, UnicodeError, ValueError, TypeError, KeyError) as error:
        result["reason"] = str(error)
        return result
