"""A self-contained, read-only review surface for an existing Arc diagram.

The pinned renderer's output is embedded unchanged in a sandboxed ``srcdoc``.
This module only formats the facts already present in ``spec.arc``; it does not
open files, run commands, confirm rules, or calculate a new business verdict.
"""

from __future__ import annotations

import base64
import hashlib
import html
import json
from collections import Counter
from typing import Any

_STATUS = {
    "PASS": ("通过", "pass"),
    "FAIL": ("失败", "danger"),
    "BLOCKED": ("阻塞", "warn"),
    "NOT_RUN": ("未验证", "muted"),
    "STALE": ("证据过期", "warn"),
    "FLAKY": ("结果波动", "warn"),
    "PARTIAL": ("部分完成", "warn"),
    "EXCLUDED": ("已排除", "muted"),
}
_KIND = {
    "system": "系统",
    "domain": "业务域",
    "capability": "业务能力",
    "role": "参与角色",
    "action": "业务操作",
    "condition": "前置条件",
    "state": "业务状态",
    "rule": "业务规则",
}
_RELATION = {
    "contains": "包含",
    "serves": "参与",
    "implements": "实现业务",
    "requires": "前置条件",
    "forbids": "禁止条件",
    "transitions": "产生状态",
    "transitions_to": "状态迁移",
    "recovers": "恢复条件",
    "next": "后续操作",
}
_CSP = (
    "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; "
    "img-src data:; frame-src 'self' about:; connect-src 'none'; font-src 'none'; "
    "object-src 'none'; base-uri 'none'; form-action 'none'"
)
_STYLE = """
:root{color-scheme:dark;--bg:#0b0f14;--panel:#0e141b;--text:#c9d1d9;--head:#e6edf3;
--dim:#97a1ad;--line:#303b47;--cyan:#5ad1e6;--blue:#79a8ff;--amber:#e8b25d;--red:#ff7b72;--green:#6fdc8c}
*{box-sizing:border-box}html,body{margin:0;background:var(--bg);color:var(--text)}
body{font:15px/1.7 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif}
main{max-width:1440px;margin:auto;padding:38px 40px 64px}h1,h2,h3,p{margin:0}
h1{font-size:30px;line-height:1.35;font-weight:650;letter-spacing:.01em;color:var(--head)}
h2{font-size:19px;color:var(--head)}h3{font-size:16px;margin:22px 0 10px;color:var(--head)}
.muted,.hint{color:var(--dim)}.hint{font-size:13px}
.status-strip{display:flex;flex-wrap:wrap;align-items:center;gap:8px 20px;margin-top:13px;color:var(--dim);font-size:13px}
.status-strip .exported{font-size:12px}.boundary{font-size:13px;margin-top:8px;color:var(--dim)}
.badge{display:inline-block;border:1px solid currentColor;border-radius:5px;font:600 12px/1.55 system-ui,sans-serif;padding:2px 7px;white-space:nowrap}
.badge.pass{color:var(--green)}.badge.danger{color:var(--red)}.badge.warn{color:var(--amber)}.badge.muted{color:var(--dim)}
.attention{font-size:13px;margin-top:10px;overflow-wrap:anywhere}
.attention p+p{margin-top:4px}.attention ul{margin:4px 0 0;padding-left:20px}
.alert{color:var(--amber)}.error{color:var(--red)}.reason{white-space:pre-wrap;overflow-wrap:anywhere}
.diagram{margin:18px 0 12px;border:1px solid var(--line);border-radius:8px;overflow:hidden;background:var(--panel)}
.diagram-viewport{overflow-x:auto}iframe{width:100%;display:block;border:0;background:var(--bg)}
.readiness{margin-top:10px}.readiness-items{display:flex;flex-wrap:wrap;gap:4px 18px;margin-bottom:4px}.readiness-item{font-size:12px;color:var(--dim)}.readiness-item code{color:var(--head)}
.evidence>h2{margin:26px 0 12px}.disclosure{border:1px solid var(--line);border-radius:9px;background:var(--panel);margin-top:12px;overflow:hidden}
summary{overflow-wrap:anywhere;cursor:pointer;list-style:none;padding:17px 20px;color:var(--head);font-weight:550;display:flex;align-items:center;gap:12px}
summary::-webkit-details-marker{display:none}summary:before{content:"＋";color:var(--cyan);font-size:18px;line-height:1;flex:none}
details[open]>summary:before{content:"−"}summary:focus-visible,a:focus-visible{outline:2px solid var(--cyan);outline-offset:-3px}
summary .count{margin-left:auto;color:var(--dim);font-size:12px;font-weight:400}.detail-body{padding:0 20px 20px}
details[open]>summary{border-bottom:1px solid var(--line);margin-bottom:16px}
.table-wrap{overflow-x:auto;border:1px solid var(--line);border-radius:7px}table{width:100%;border-collapse:collapse;font-size:13px;table-layout:fixed}
th,td{text-align:left;vertical-align:top;padding:12px 14px;border-bottom:1px solid var(--line);overflow-wrap:anywhere}
th{font-weight:550;color:var(--dim);background:#111a23}tr:last-child td{border-bottom:0}td p+p{margin-top:7px}
code,pre{font-family:"SFMono-Regular",Consolas,"Liberation Mono",monospace;font-size:12px}code{color:#b9c9dc;overflow-wrap:anywhere;white-space:pre-wrap}
pre{margin:10px 0 0;padding:16px;border:1px solid var(--line);border-radius:6px;background:var(--bg);white-space:pre-wrap;overflow-wrap:anywhere;tab-size:2;line-height:1.65}
.cell-label{display:block;font-size:11px;color:var(--dim);margin:8px 0 2px}.mini{margin-top:10px;border:1px solid var(--line);border-radius:6px}
.mini>summary{font-size:12px;padding:9px 12px}.mini .detail-body{padding:0 12px 12px}.mini pre{margin:0}
.empty{padding:10px 0;color:var(--dim)}.download{display:inline-block;color:var(--cyan);text-decoration:none;border:1px solid var(--line);padding:7px 12px;border-radius:6px;margin:10px 0}
.source-grid{display:grid;grid-template-columns:170px minmax(0,1fr);gap:9px 20px;margin:0}.table-wrap .source-grid{grid-template-columns:1fr;gap:2px}.table-wrap .source-grid dd{margin-bottom:7px}
.source-grid dt{color:var(--dim);font-size:13px}.source-grid dd{margin:0;overflow-wrap:anywhere;white-space:pre-wrap}
footer{margin-top:26px;font-size:12px;color:var(--dim);border-top:1px solid var(--line);padding-top:18px}
@media(max-width:900px){main{padding:24px 18px 40px}table{min-width:740px}}
@media(max-width:520px){h1{font-size:25px}.status-strip{gap:7px 14px}.source-grid{grid-template-columns:1fr;gap:3px}.source-grid dd{margin-bottom:9px}.detail-body{padding:0 12px 16px}summary{padding:15px 12px}}
@media print{main{padding:12px}.disclosure{break-inside:avoid}.download{display:none}}
"""


def _dict(value: Any) -> dict:
    return value if isinstance(value, dict) else {}


def _list(value: Any) -> list:
    return value if isinstance(value, list) else []


def _records(value: Any) -> list[dict]:
    return [item for item in _list(value) if isinstance(item, dict)]


def _text(value: Any, default: str = "未提供") -> str:
    if value is None or value == "":
        return default
    if isinstance(value, (dict, list)):
        return json.dumps(value, ensure_ascii=False, sort_keys=True)
    if isinstance(value, bool):
        return "是" if value else "否"
    return str(value)


def _e(value: Any, default: str = "未提供") -> str:
    return html.escape(_text(value, default), quote=True)


def _code(value: Any, default: str = "未提供") -> str:
    return f"<code>{_e(value, default)}</code>"


def _json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, indent=2, sort_keys=True, allow_nan=False
    )


def _pre(value: Any) -> str:
    return f"<pre>{html.escape(_json(value), quote=True)}</pre>"


def _status(value: Any) -> str:
    raw = _text(value, "UNKNOWN")
    label, tone = _STATUS.get(
        raw, ("未提供状态" if raw == "UNKNOWN" else "原始状态", "muted")
    )
    return f'<span class="badge {tone}">{label} · {_e(raw)}</span>'


def _details(title: str, content: str, count: str = "", *, mini: bool = False) -> str:
    cls = "mini" if mini else "disclosure"
    count_html = f'<span class="count">{_e(count)}</span>' if count else ""
    return (
        f'<details class="{cls}"><summary>{_e(title)}{count_html}</summary>'
        f'<div class="detail-body">{content}</div></details>'
    )


def _table(headings: list[str], rows: list[list[str]]) -> str:
    if not rows:
        return '<p class="empty">本快照未提供此类记录。</p>'
    head = "".join(f'<th scope="col">{_e(item)}</th>' for item in headings)
    body = "".join(
        "<tr>" + "".join(f"<td>{cell}</td>" for cell in row) + "</tr>" for row in rows
    )
    return f'<div class="table-wrap"><table><thead><tr>{head}</tr></thead><tbody>{body}</tbody></table></div>'


def _fields(items: list[tuple[str, Any]]) -> str:
    return (
        '<dl class="source-grid">'
        + "".join(
            f"<dt>{_e(label)}</dt><dd>{_code(value)}</dd>" for label, value in items
        )
        + "</dl>"
    )


def _approval(value: Any) -> str:
    return (
        "已确认" if value is True else "待确认" if value is False else "未提供确认记录"
    )


def _inverse(value: Any) -> dict[str, list[str]]:
    result: dict[str, list[str]] = {}
    for renderer, source in _dict(value).items():
        if isinstance(source, str):
            result.setdefault(source, []).append(str(renderer))
    return result


def _nodes(arc: dict, spec: dict) -> str:
    statuses, packages = (
        _dict(arc.get("node_status")),
        _dict(arc.get("package_evidence")),
    )
    mapped = _inverse(arc.get("node_ids"))
    rendered = {
        item.get("id"): item
        for item in _records(spec.get("nodes"))
        if isinstance(item.get("id"), str)
    }
    candidates = _list(arc.get("candidate_nodes"))
    names = _dict(_dict(arc.get("presentation")).get("names"))
    rows = []
    for node in _records(arc.get("source_nodes")):
        identity = _text(node.get("id"))
        state = _dict(statuses.get(identity))
        render_ids = mapped.get(identity, [])
        title = names.get(identity) or next(
            (rendered[item].get("title") for item in render_ids if item in rendered),
            node.get("name"),
        )
        business = f'<p>{_e(title)}</p><p class="hint">{_e(_KIND.get(node.get("kind"), node.get("kind")))}</p>'
        if identity in candidates:
            business += '<p class="alert">候选／待确认</p>'
        status = (
            _status(state.get("status"))
            + f'<p class="hint">历史 FAIL：{_e(state.get("history_failures"))}<br>通过义务：{_e(state.get("passed"))} / {_e(state.get("required"))}</p>'
        )
        evidence = []
        for package in _records(packages.get(identity)):
            evidence.append(
                _fields(
                    [
                        ("实际业务包", package.get("name")),
                        (
                            "来源位置",
                            f"{_text(package.get('file'))}:{_text(package.get('line'))}",
                        ),
                        ("定位依据", package.get("kind")),
                        ("文件 SHA-256", package.get("file_sha256")),
                    ]
                )
            )
        source = (
            _code(identity)
            + '<span class="cell-label">渲染 ID</span>'
            + _code("、".join(render_ids))
        )
        source += _details("节点原始记录", _pre(node), mini=True)
        rows.append(
            [
                business,
                status,
                "<br>".join(evidence) or '<span class="muted">业务包待核实</span>',
                source,
            ]
        )
    return _table(
        ["业务节点", "快照状态", "包名与来源证据", "来源 ID 与原始记录"], rows
    )


def _relationships(arc: dict) -> str:
    names = {
        _text(item.get("id")): item.get("name")
        for item in _records(arc.get("source_nodes"))
    }
    names.update(_dict(_dict(arc.get("presentation")).get("names")))
    mapped = _inverse(arc.get("edge_ids"))
    representations: dict[str, list[tuple[str, dict]]] = {}
    for renderer, record in _dict(arc.get("edge_rendering")).items():
        if isinstance(record, dict):
            representations.setdefault(_text(record.get("source_id")), []).append(
                (str(renderer), record)
            )
    transitions: dict[str, list[dict]] = {}
    for item in _records(arc.get("transitions")):
        transitions.setdefault(_text(item.get("edge")), []).append(item)
    rows = []
    for edge in _records(arc.get("source_edges")):
        identity, start, end = (_text(edge.get(key)) for key in ("id", "from", "to"))
        relation = (
            _e(_RELATION.get(edge.get("kind"), edge.get("kind")))
            + "<br>"
            + _code(edge.get("kind"))
        )
        endpoints = (
            f"<p>{_e(names.get(start), start)} → {_e(names.get(end), end)}</p>"
            f"<p>{_code(start)} → {_code(end)}</p>"
        )
        source = _code(identity) + '<span class="cell-label">渲染映射</span>'
        if identity in representations:
            source += "<br>".join(
                _code(renderer)
                + " · "
                + _e(
                    {"containment": "包含区域", "wire": "连线"}.get(
                        record.get("representation"), record.get("representation")
                    )
                )
                for renderer, record in representations[identity]
            )
        else:
            source += _code("、".join(mapped.get(identity, [])), "未提供")
        logic = (
            _pre(transitions[identity])
            if identity in transitions
            else '<span class="muted">未声明迁移分支</span>'
        )
        rows.append([relation, endpoints, source, logic])
    result = _table(
        ["关系含义", "业务起点 → 终点", "源关系 ID", "迁移、条件与结果"], rows
    )
    presentation = _dict(arc.get("presentation"))
    if presentation.get("relationships"):
        result += _details(
            "完整业务关系说明", _pre(presentation["relationships"]), mini=True
        )
    mapping = {
        key: arc[key]
        for key in ("node_ids", "group_ids", "edge_ids", "edge_rendering")
        if key in arc
    }
    mapping.update(
        {
            key: presentation[key]
            for key in ("layout", "containment")
            if key in presentation
        }
    )
    if mapping:
        result += _details("展示映射与包含关系", _pre(mapping), mini=True)
    return result


def _literal(value: Any) -> str:
    item = _dict(value)
    state = (
        "满足"
        if item.get("value") is True
        else "不满足"
        if item.get("value") is False
        else "值未声明"
    )
    return f"{_text(item.get('condition'))}={state}"


def _guard(value: Any) -> str:
    clauses = _dict(value).get("any_of")
    if (
        not isinstance(clauses, list)
        or not clauses
        or any(not isinstance(clause, list) for clause in clauses)
    ):
        return "未提供有效的 any_of 条件表达式；请核对原始记录"
    return " 或 ".join(
        "（" + (" 且 ".join(_literal(item) for item in clause) or "恒真") + "）"
        for clause in clauses
    )


def _logic(arc: dict) -> str:
    logic, approvals = (
        _dict(arc.get("business_logic")),
        _dict(arc.get("rule_approvals")),
    )
    guards = {_text(item.get("rule")): item for item in _records(logic.get("guards"))}
    rows = []
    for rule in _records(arc.get("rules")):
        identity = _text(rule.get("id"))
        condition = (
            _guard(guards[identity])
            if identity in guards
            else " 且 ".join(
                f"{_text(item)}=满足" for item in _list(rule.get("conditions"))
            )
            or "未声明前置条件"
        )
        rows.append(
            [
                _code(identity)
                + f'<p class="hint">{_approval(approvals.get(identity))}</p>',
                _code(rule.get("action")),
                _e(condition),
                _fields(
                    [("允许后", rule.get("expected")), ("拒绝后", rule.get("rejected"))]
                )
                + _details("RULE 原始记录", _pre(rule), mini=True),
            ]
        )
    result = _table(["RULE 与确认状态", "作用操作", "允许条件", "期望结果"], rows)
    if guards:
        result += "<h3>Guard 条件</h3>" + _table(
            ["RULE", "条件表达式", "原始记录"],
            [
                [_code(item.get("rule")), _e(_guard(item)), _pre(item)]
                for item in _records(logic.get("guards"))
            ],
        )
    constraints = _records(logic.get("constraints"))
    if constraints:
        result += '<h3>约束条件</h3><p class="hint">各约束须同时成立；每项内部保留原始「或」关系。</p>'
        result += _table(
            ["约束 ID", "条件表达式", "原始记录"],
            [
                [
                    _code(item.get("id")),
                    _e(
                        " 或 ".join(
                            _literal(literal) for literal in _list(item.get("literals"))
                        )
                        or "未提供条件"
                    ),
                    _pre(item),
                ]
                for item in constraints
            ],
        )
    remaining = {
        key: value
        for key, value in logic.items()
        if key not in {"guards", "constraints"}
    }
    if remaining:
        result += _details("契约、不变量及其他业务逻辑", _pre(remaining), mini=True)
    if arc.get("flows"):
        result += _details("业务流程定义", _pre(arc["flows"]), mini=True)
    presentation = _dict(arc.get("presentation"))
    if presentation.get("logic"):
        result += _details("完整规则与条件说明", _pre(presentation["logic"]), mini=True)
    return result


def _runs(arc: dict) -> str:
    result = '<p class="hint">业务结果、首次偏离与 RAW 完整性分别保留；后续 PASS 不覆盖历史 FAIL。</p>'
    for run in _records(arc.get("runs")):
        metadata = {
            key: value for key, value in run.items() if key not in {"steps", "evidence"}
        }
        content = _pre(metadata)
        content += "<h3>执行步骤</h3>" + _table(
            ["步骤与业务 ID", "业务结果", "期望与实际", "RAW 来源与完整性"],
            [
                [
                    _fields(
                        [
                            ("步骤", step.get("step")),
                            ("节点", step.get("node")),
                            ("迁移", step.get("transition")),
                        ]
                    ),
                    _status(step.get("verdict"))
                    + '<span class="cell-label">继续决策</span>'
                    + _code(step.get("decision")),
                    _fields(
                        [("期望", step.get("expected")), ("实际", step.get("observed"))]
                    ),
                    _fields(
                        [
                            ("RAW 路径", step.get("raw")),
                            ("SHA-256", step.get("sha256")),
                            ("完整性", step.get("evidence_status")),
                        ]
                    ),
                ]
                for step in _records(run.get("steps"))
            ],
        )
        content += "<h3>RAW 校验记录</h3>" + _pre(run.get("evidence", []))
        summary = f"运行 {_text(run.get('run'))} · {_text(run.get('status'))} · 首次偏离：{_text(run.get('first_divergence'), '未记录')}"
        result += _details(summary, content, mini=True)
    if not _records(arc.get("runs")):
        result += '<p class="empty">本快照未提供运行记录；不代表业务已通过。</p>'
    if _dict(arc.get("business_logic")).get("history") is not None:
        result += _details(
            "历史映射", _pre(arc["business_logic"]["history"]), mini=True
        )
    if arc.get("exploration"):
        result += _details("探索、恢复与证据缺口", _pre(arc["exploration"]), mini=True)
    return result


def _attention(arc: dict) -> str:
    states = _dict(arc.get("node_status"))
    counts = Counter(
        _text(_dict(item).get("status"), "UNKNOWN") for item in states.values()
    )
    runs = _records(arc.get("runs"))
    nonpass_runs = [run for run in runs if run.get("status") != "PASS"]
    raw_errors = [
        (run, item)
        for run in runs
        for item in _records(run.get("evidence"))
        if item.get("error")
    ]
    raw_nonpass = [run for run in runs if run.get("evidence_status") != "PASS"]
    # Keep actual exceptions above the graph; routine zero counts belong in the
    # evidence below. NOT_RUN is already explicit in the graph's node badges.
    facts = []
    status_text = "；".join(
        f"{status} {count}"
        for status, count in sorted(counts.items())
        if status not in {"PASS", "NOT_RUN", "EXCLUDED"}
    )
    if status_text:
        facts.append("节点：" + status_text)
    if arc.get("history_failures") not in (None, 0, "0", ""):
        facts.append("历史 FAIL：" + _text(arc["history_failures"]))
    if nonpass_runs:
        facts.append(f"运行非 PASS：{len(nonpass_runs)}")
    if raw_nonpass:
        facts.append(f"RAW 完整性非 PASS／未提供：{len(raw_nonpass)}")
    if raw_errors:
        facts.append(f"RAW 校验错误：{len(raw_errors)}")
    omitted = _dict(arc.get("omitted"))
    for key, label, unit in (
        ("nodes", "省略节点", "个"),
        ("edges", "省略关系", "条"),
        ("logic", "省略图内逻辑说明", "项"),
    ):
        if _list(omitted.get(key)):
            facts.append(f"{label} {len(omitted[key])} {unit}")
    if not omitted:
        facts.append("省略范围未提供")
    if omitted.get("projection_truncated") is True:
        facts.append("来源投影已截断")
    body = f'<p class="alert">{_e(" · ".join(facts))}</p>' if facts else ""
    firsts = []
    for run in runs:
        if run.get("first_divergence") or run.get("status") != "PASS":
            firsts.append(
                f"<li>{_code(run.get('run'))} · {_status(run.get('status'))} · 首次偏离：{_code(run.get('first_divergence'), '未记录')}</li>"
            )
    for identity, value in _dict(arc.get("exploration")).items():
        exploration = _dict(value)
        if exploration.get("first_divergence"):
            firsts.append(
                f"<li>探索 {_code(identity)} · 首次偏离：{_code(exploration['first_divergence'])}</li>"
            )
    if firsts:
        body += "<ul>" + "".join(firsts) + "</ul>"
    if raw_errors:
        body += (
            '<ul class="error">'
            + "".join(
                f"<li>运行 {_code(run.get('run'))} / {_code(item.get('phase'))} / 步骤 {_code(item.get('step'))}：{_e(item['error'])}</li>"
                for run, item in raw_errors
            )
            + "</ul>"
        )
    # These known presentation notices are preserved in source details. Unknown
    # warnings and genuine failure/stale evidence warnings stay visible.
    routine = (
        "业务包待核实",
        "缺少中文业务名称",
        "包含关系存在共享归属或环",
        "显式局部范围",
        "来源投影已截断",
    )
    critical = ("FAIL", "BLOCKED", "STALE", "FLAKY", "过期", "旧图", "失败")
    warnings = [
        item
        for item in _list(arc.get("warnings"))
        if any(token in _text(item) for token in critical)
        or not any(token in _text(item) for token in routine)
    ]
    if warnings:
        body += (
            '<ul class="alert">'
            + "".join(f"<li>{_e(item)}</li>" for item in warnings)
            + "</ul>"
        )
    if arc.get("diagnostics"):
        body += '<p class="reason alert">诊断：' + _e(arc["diagnostics"]) + "</p>"
    return (
        '<section class="attention" aria-label="非 PASS、首次偏离与错误摘要">'
        + body
        + "</section>"
        if body
        else ""
    )


def _provenance(arc: dict, diagram_html: str) -> str:
    meta, snapshot, renderer = (
        _dict(arc.get(key)) for key in ("meta", "snapshot", "renderer")
    )
    result = _fields(
        [
            ("原始业务范围", arc.get("scope")),
            ("业务结论说明", arc.get("business_reason")),
            ("快照业务结论", arc.get("business_verdict")),
            ("历史 FAIL", arc.get("history_failures")),
            ("图版本", meta.get("graph")),
            ("导出时间", meta.get("generated")),
            ("来源快照", snapshot.get("path")),
            ("快照 SHA-256", snapshot.get("sha256")),
            ("业务图 SHA-256", meta.get("graph_sha256")),
            ("前一模型", arc.get("previous")),
            ("环境", meta.get("environment")),
            ("轮次", meta.get("round")),
            ("渲染器仓库", renderer.get("repository")),
            ("渲染器版本", renderer.get("version")),
            ("渲染器 commit", renderer.get("commit")),
            (
                "嵌入图 HTML SHA-256",
                hashlib.sha256(diagram_html.encode("utf-8")).hexdigest(),
            ),
        ]
    )
    for title, key in (
        ("原始导出元数据", "meta"),
        ("证据上下文与覆盖范围", "context"),
        ("覆盖记录", "coverage"),
        ("省略范围与截断信息", "omitted"),
        ("模型 RISK", "risks"),
        ("渲染器来源与许可", "renderer"),
        ("完整展示说明", "presentation"),
        ("来源警告", "warnings"),
        ("来源诊断", "diagnostics"),
    ):
        if key in arc:
            result += _details(title, _pre(arc[key]), mini=True)
    return result


def _readiness(value: dict | None) -> str:
    labels = {
        "spec": "规格",
        "build": "构建",
        "layout_check": "布局检查",
        "screenshot": "截图",
        "visual_review": "目视核对",
    }
    if value is None:
        body = '<p class="hint">图形检查状态未提供。</p>'
    else:
        body = (
            '<div class="readiness-items">'
            + "".join(
                f'<span class="readiness-item">{_e(label)}：{_code(value.get(key), "未提供")}</span>'
                for key, label in labels.items()
            )
            + "</div>"
        )
    return (
        '<div class="readiness" aria-label="图形检查状态">'
        + body
        + '<p class="hint">此处记录图形的生成与检查状态；业务结论见上方快照记录。</p></div>'
    )


def render_review(
    spec: dict,
    diagram_html: str,
    diagram_height: int | None = None,
    *,
    readiness: dict | None = None,
) -> str:
    """Return an offline HTML companion without mutating ``spec`` or its diagram.

    ``diagram_html`` must be the already-built pinned renderer output. Its exact
    text is carried in ``srcdoc`` (including carriage returns), not rewritten.
    ``diagram_height`` selects the iframe viewport in pixels; long diagrams keep
    native scrolling. Omission uses a 960px viewport. No wrapper JavaScript is
    needed: disclosures and the JSON download are native browser features.
    Optional ``readiness`` contains the caller's actual visualization check
    results; the review never infers those results from the presence of HTML.

    The iframe can run the renderer but has an opaque sandbox origin; it cannot
    access this review page. An inherited CSP denies network, forms, external
    resources, and object loads. Missing Arc fields stay explicitly unprovided.
    """
    if not isinstance(spec, dict) or not isinstance(diagram_html, str):
        raise TypeError("spec must be an object and diagram_html must be text")
    if diagram_height is not None and (
        type(diagram_height) is not int or diagram_height < 1
    ):
        raise ValueError("diagram_height must be a positive integer or None")
    if readiness is not None and not isinstance(readiness, dict):
        raise TypeError("readiness must be an object or None")
    height = diagram_height if diagram_height is not None else 960
    width = spec.get("width", 1080)
    native_width = width + 32 if type(width) is int and width > 0 else 1112
    arc = _dict(spec.get("arc"))
    meta = _dict(arc.get("meta"))
    title = _text(spec.get("title"), "业务图")
    verdict = arc.get("business_verdict")
    approved = arc.get("model_approved")
    approval = (
        "业务目录已确认"
        if approved is True
        else "候选目录 · 待确认"
        if approved is False
        else "确认状态未提供"
    )
    candidates = len(_list(arc.get("candidate_nodes")))
    candidate_info = (
        f"<span>候选／待确认节点：{candidates}</span>" if candidates else ""
    )
    reason_text = " ".join(_text(arc.get("business_reason"), "").split())
    reason = ""
    if _text(verdict) in {"FAIL", "BLOCKED", "STALE", "FLAKY"} and reason_text:
        concise = reason_text if len(reason_text) <= 180 else reason_text[:177] + "…"
        reason = f'<p class="boundary reason">{_e(concise)}</p>'
    fixture = (
        '<p class="boundary">工具示例／合成夹具；不代表真实产品验证。</p>'
        if arc.get("fixture") is True
        else ""
    )
    no_runs = " 本快照未提供运行记录。" if not _records(arc.get("runs")) else ""
    review_spec = _json(spec) + "\n"
    download = base64.b64encode(review_spec.encode("utf-8")).decode("ascii")
    full_spec = (
        f'<p class="hint">完整 JSON 保留来源 ID、业务证据、展示映射与渲染器来源，可在本页检查或保存。</p>'
        f'<a class="download" download="arc-business-diagram.spec.json" href="data:application/json;base64,{download}">下载完整 spec JSON</a>'
        f'<pre id="source-spec">{html.escape(review_spec, quote=True)}</pre>'
    )
    evidence = "".join(
        [
            _details(
                "业务节点与来源",
                _nodes(arc, spec),
                f"{len(_records(arc.get('source_nodes')))} 个节点",
            ),
            _details(
                "实际业务关系",
                _relationships(arc),
                f"{len(_records(arc.get('source_edges')))} 条关系",
            ),
            _details(
                "RULE、Guard 与约束逻辑",
                _logic(arc),
                f"{len(_records(arc.get('rules')))} 条 RULE",
            ),
            _details(
                "运行、历史 FAIL 与 RAW",
                _runs(arc),
                f"{len(_records(arc.get('runs')))} 个运行",
            ),
            _details(
                "来源、哈希与范围",
                _provenance(arc, diagram_html)
                + (
                    _details("图形检查原始记录", _pre(readiness), mini=True)
                    if readiness is not None
                    else ""
                ),
            ),
            _details("完整 spec · 只读", full_spec),
        ]
    )
    srcdoc = html.escape(diagram_html, quote=True).replace("\r", "&#13;")
    return f'''<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="{html.escape(_CSP, quote=True)}">
<title>{_e(title)} · 业务图核对</title><style>{_STYLE}</style></head>
<body><main><header><h1>{_e(title)}</h1>
<div class="status-strip" aria-label="快照状态与模型确认">
<span>快照业务结论 {_status(verdict)}</span><span>{_e(approval)}</span>
<span>{len(_records(arc.get("source_nodes")))} 节点 · {len(_records(arc.get("source_edges")))} 关系</span>{candidate_info}
<span class="exported">导出于 {_code(meta.get("generated"))}</span></div>
{reason}{fixture}{_attention(arc)}</header>
<section class="diagram" aria-label="官方业务图"><div class="diagram-viewport"><iframe title="原始 arch-diagram 业务图" sandbox="allow-scripts" referrerpolicy="no-referrer" height="{height}" style="min-width:{native_width}px" srcdoc="{srcdoc}"></iframe></div></section>
{_readiness(readiness)}
<p class="hint">本页未重新执行业务验证；图形检查和 RAW 完整性均不替代业务判定。{no_runs}</p>
<section class="evidence" aria-label="业务证据详情"><h2>证据与来源</h2><p class="hint">展开核对状态、实际业务关系、规则与原始记录。</p>{evidence}</section>
<footer>静态快照 · 离线可读 · 仅供核对。包名定位、图形展示与页面生成均不产生新的业务通过记录。</footer>
</main></body></html>'''
