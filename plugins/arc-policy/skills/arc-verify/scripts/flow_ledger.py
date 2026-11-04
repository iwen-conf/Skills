#!/usr/bin/env python3
"""Check a business-flow catalog: Mermaid flowcharts, terminal and branch ledgers, README index.

usage: flow_ledger.py check <catalog-dir> [--json]

The catalog holds README.md (inventory; modeled flows are linked) and one FLOW-NNN-<name>.md per
modeled flow. Format: references/flow-model.md §3 and references/flow-matrix.md §5.
It proves structure and traceability (every branch reaches a terminal and has a ledger row with a
legal verdict and evidence), never business correctness.

Exit status: 0 no error, 1 structural errors, 2 usage error.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import sys

FLOW_FILE = re.compile(r"^(FLOW-\d{3})-.+\.md$")
FENCE = re.compile(r"```mermaid\s*\n(.*?)```", re.S)
ARROW = re.compile(r"\s*(?:-->|-\.->|==>)\s*(?:\|([^|]*)\|)?\s*")
SEGMENT = re.compile(r"^([A-Za-z][A-Za-z0-9_]*)\s*(.*)$")
SKIP = re.compile(
    r"^(?:flowchart|graph|subgraph|end\b|direction|classDef|class |style |linkStyle|click )"
)
TERMINAL_ID = re.compile(r"^T\d{2}$")
DECISION_ID = re.compile(r"^N\d{2}$")
BRANCH_LABEL = re.compile(r"^(B\d{2})\s*(.*)$")
TERMINAL_ROW = re.compile(r"^(FLOW-\d{3})-(T\d{2})$")
BRANCH_ROW = re.compile(r"^(FLOW-\d{3})-(N\d{2})-(B\d{2})$")
README_LINK = re.compile(r"\]\((FLOW-\d{3}-[^)#\s]+\.md)\)")
README_ID = re.compile(r"^\|\s*(FLOW-\d{3})\s*\|", re.M)
CLOSURES = {
    "正常",
    "拒绝",
    "失败",
    "恢复",
    "异步",
    "缺口",
    "normal",
    "reject",
    "failure",
    "recovery",
    "async",
    "gap",
}
GAP = {"缺口", "gap"}
RISKS = {"Critical", "High", "Medium", "Low", "Info"}
VERDICT = re.compile(r"^(PASS|FAIL|FLAKY|BLOCKED|NOT_RUN)\b\s*(.*)$")
REASON = re.compile(r"^[(（:：]\s*\S")
EMPTY = {"", "—", "-", "–", "n/a"}
BARE_CONDITIONS = {
    "是",
    "否",
    "成功",
    "失败",
    "通过",
    "不通过",
    "yes",
    "no",
    "y",
    "n",
    "ok",
    "success",
    "fail",
    "failure",
    "true",
    "false",
}
BRANCH_COLUMNS = 9  # 分支 | 条件 | 去向 | 闭环 | 风险 | 用例 | 绑定 | 判定 | 证据
TERMINAL_COLUMNS = 4  # 终点 | 结果 | 闭环 | 观测点


def shape_of(rest: str) -> str | None:
    """Return the node kind for the text after an ID, '' for a bare reference, None if unsupported."""
    if not rest:
        return ""
    for opener, closer, kind in (
        ("([", "])", "stadium"),
        ("[[", "]]", "process"),
        ("{{", "}}", "process"),
        ("[(", ")]", "process"),
        ("{", "}", "decision"),
        ("[", "]", "process"),
        ("((", "))", "process"),
        ("(", ")", "process"),
        (">", "]", "process"),
    ):
        if rest.startswith(opener) and rest.endswith(closer):
            return kind
    return None


def parse_chart(
    text: str, errors: list[str]
) -> tuple[dict[str, str], list[tuple[str, str, str | None]]]:
    nodes: dict[str, str] = {}
    edges: list[tuple[str, str, str | None]] = []
    used: set[str] = set()
    for block in FENCE.findall(text):
        for raw in block.splitlines():
            line = raw.split("%%", 1)[0].strip()
            if not line or SKIP.match(line):
                continue
            parts = ARROW.split(line)
            ids: list[str] = []
            for index in range(0, len(parts), 2):
                match = SEGMENT.match(parts[index].strip())
                kind = shape_of(match.group(2).strip()) if match else None
                if match is None or kind is None:
                    errors.append(f"unsupported Mermaid syntax: {line}")
                    ids = []
                    break
                node = match.group(1)
                ids.append(node)
                used.add(node)
                if kind:
                    previous = nodes.setdefault(node, kind)
                    if previous != kind:
                        errors.append(f"node {node} is drawn with two shapes")
            labels = parts[1::2]
            for index in range(len(ids) - 1):
                edges.append(
                    (ids[index], ids[index + 1], (labels[index] or "").strip() or None)
                )
    for node in sorted(used - set(nodes)):
        errors.append(f"node {node} is used but never defined")
    return nodes, edges


def check_chart(
    nodes: dict[str, str], edges: list[tuple[str, str, str | None]], errors: list[str]
) -> dict[tuple[str, str], str]:
    """Validate shapes, branch labels and reachability; return {(decision, branch): target}."""
    if nodes.get("S") != "stadium":
        errors.append("the chart needs exactly one start node S([...])")
    out: dict[str, list[tuple[str, str | None]]] = {node: [] for node in nodes}
    for source, target, label in edges:
        out.setdefault(source, []).append((target, label))
    branches: dict[tuple[str, str], str] = {}
    for node, kind in nodes.items():
        if kind == "decision" and not DECISION_ID.match(node):
            errors.append(f"decision {node} must be named Nxx")
        if DECISION_ID.match(node) and kind != "decision":
            errors.append(f"{node} must be drawn as a decision {{...}}")
        if TERMINAL_ID.match(node):
            if kind != "stadium":
                errors.append(f"terminal {node} must be drawn as ([...])")
            if out.get(node):
                errors.append(f"terminal {node} has an outgoing edge")
    for node, targets in out.items():
        if nodes.get(node) != "decision":
            for target, label in targets:
                if label and BRANCH_LABEL.match(label):
                    errors.append(
                        f"{node} -> {target}: branch labels belong to decisions only"
                    )
            continue
        if len(targets) < 2:
            errors.append(f"decision {node} has fewer than 2 branches")
        for target, label in targets:
            match = BRANCH_LABEL.match(label or "")
            if not match:
                errors.append(
                    f"{node} -> {target}: decision edges need a |Bxx condition| label"
                )
                continue
            branch, condition = match.groups()
            if (node, branch) in branches:
                errors.append(f"{node}-{branch} is used twice")
            branches[(node, branch)] = target
            if (
                len(condition.strip()) < 2
                or condition.strip().lower() in BARE_CONDITIONS
            ):
                errors.append(
                    f"{node}-{branch} condition must name the business condition, not '{condition.strip()}'"
                )
    reach = _closure(
        {"S"} & set(nodes), {n: [t for t, _ in ts] for n, ts in out.items()}
    )
    for node in sorted(set(nodes) - reach):
        errors.append(f"node {node} is unreachable from S")
    reverse: dict[str, list[str]] = {}
    for source, target, _ in edges:
        reverse.setdefault(target, []).append(source)
    finishing = _closure({n for n in nodes if TERMINAL_ID.match(n)}, reverse)
    for node in sorted(set(nodes) - finishing):
        errors.append(f"node {node} cannot reach a terminal")
    return branches


def _closure(start: set[str], graph: dict[str, list[str]]) -> set[str]:
    seen, stack = set(start), list(start)
    while stack:
        for nxt in graph.get(stack.pop(), []):
            if nxt not in seen:
                seen.add(nxt)
                stack.append(nxt)
    return seen


def table_rows(text: str) -> list[list[str]]:
    rows = []
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("|") and line.endswith("|"):
            rows.append([cell.strip() for cell in line[1:-1].split("|")])
    return rows


def check_flow(path: Path, flow_id: str, errors: list[str]) -> dict:
    text = path.read_text(encoding="utf-8")
    local: list[str] = []
    nodes, edges = parse_chart(text, local)
    if not nodes:
        local.append("no ```mermaid flowchart block")
    branches = check_chart(nodes, edges, local) if nodes else {}
    terminals_in_chart = {node for node in nodes if TERMINAL_ID.match(node)}
    terminals: dict[str, str] = {}
    ledger: dict[tuple[str, str], list[str]] = {}
    for row in table_rows(text):
        key = row[0] if row else ""
        if match := TERMINAL_ROW.match(key):
            if match.group(1) != flow_id or len(row) != TERMINAL_COLUMNS:
                local.append(
                    f"{key}: terminal rows need {TERMINAL_COLUMNS} columns and this flow's ID"
                )
                continue
            terminal = match.group(2)
            if terminal in terminals:
                local.append(f"{key}: duplicate terminal row")
                continue
            terminals[terminal] = row[2]
            if row[2] not in CLOSURES:
                local.append(
                    f"{key}: closure {row[2]} is not one of {sorted(CLOSURES)}"
                )
            if row[3] in EMPTY:
                local.append(f"{key}: terminal needs an observable result")
        elif match := BRANCH_ROW.match(key):
            if match.group(1) != flow_id or len(row) != BRANCH_COLUMNS:
                local.append(
                    f"{key}: branch rows need {BRANCH_COLUMNS} columns and this flow's ID"
                )
                continue
            branch_key = (match.group(2), match.group(3))
            if branch_key in ledger:
                local.append(f"{key}: duplicate branch row")
                continue
            ledger[branch_key] = row
    for node in sorted(terminals_in_chart - set(terminals)):
        local.append(f"{flow_id}-{node} is in the chart but not in the terminal table")
    for node in sorted(set(terminals) - terminals_in_chart):
        local.append(f"{flow_id}-{node} is in the terminal table but not in the chart")
    for node, branch in sorted(set(branches) - set(ledger)):
        local.append(
            f"{flow_id}-{node}-{branch} is in the chart but not in the branch table"
        )
    for node, branch in sorted(set(ledger) - set(branches)):
        local.append(
            f"{flow_id}-{node}-{branch} is in the branch table but not in the chart"
        )

    verdicts: dict[str, int] = {}
    designed = bound = 0
    open_high: list[str] = []
    for (node, branch), row in sorted(ledger.items()):
        key = f"{flow_id}-{node}-{branch}"
        _, condition, target, closure, risk, case, binding, verdict_cell, evidence = row
        if condition in EMPTY:
            local.append(f"{key}: condition is empty")
        if (node, branch) in branches and target.split()[0:1] != [
            branches[(node, branch)]
        ]:
            local.append(
                f"{key}: target {target.split()[0] if target.split() else '—'} differs from the chart edge to {branches[(node, branch)]}"
            )
        if closure not in CLOSURES:
            local.append(f"{key}: closure {closure} is not one of {sorted(CLOSURES)}")
        if risk not in RISKS:
            local.append(f"{key}: risk {risk} is not one of {sorted(RISKS)}")
        match = VERDICT.match(verdict_cell)
        if not match:
            local.append(
                f"{key}: verdict {verdict_cell} is not one of PASS/FAIL/FLAKY/BLOCKED/NOT_RUN"
            )
            continue
        verdict, reason = match.groups()
        verdicts[verdict] = verdicts.get(verdict, 0) + 1
        designed += case not in EMPTY
        bound += binding not in EMPTY
        if verdict in {"PASS", "FAIL", "FLAKY"} and (
            case in EMPTY or binding in EMPTY or evidence in EMPTY
        ):
            local.append(f"{key}: {verdict} needs case, binding and evidence")
        if verdict in {"BLOCKED", "NOT_RUN"} and not REASON.match(reason):
            local.append(
                f"{key}: {verdict} needs a reason, e.g. {verdict} (no test environment)"
            )
        if risk in {"Critical", "High"} and verdict != "PASS":
            open_high.append(key)
    errors.extend(f"{path.name}: {item}" for item in local)
    return {
        "branches": len(ledger),
        "verdicts": verdicts,
        "designed": designed,
        "bound": bound,
        "gap_terminals": sorted(
            f"{flow_id}-{t}" for t, c in terminals.items() if c in GAP
        ),
        "open_high_risk": open_high,
    }


def check_catalog(catalog: Path) -> dict:
    errors: list[str] = []
    readme_text = (catalog / "README.md").read_text(encoding="utf-8")
    linked = set(README_LINK.findall(readme_text))
    files = {path.name: path for path in catalog.glob("FLOW-*.md")}
    for name in sorted(linked - set(files)):
        errors.append(f"README links {name}, which does not exist")
    for name in sorted(set(files) - linked):
        errors.append(f"{name} is not linked from README.md")
    flows: dict[str, dict] = {}
    for name, path in sorted(files.items()):
        match = FLOW_FILE.match(name)
        if not match:
            errors.append(f"{name}: file name must be FLOW-NNN-<name>.md")
            continue
        flow_id = match.group(1)
        if not re.search(rf"^#\s+{flow_id}\b", path.read_text(encoding="utf-8"), re.M):
            errors.append(f"{name}: title must start with '# {flow_id}'")
        if flow_id in flows:
            errors.append(f"{name}: duplicate flow ID {flow_id}")
        flows[flow_id] = check_flow(path, flow_id, errors)
    listed = README_ID.findall(readme_text)
    for flow_id in sorted(set(flows) - set(listed)):
        errors.append(f"{flow_id} has a file but no README inventory row")
    unmodeled = [flow_id for flow_id in dict.fromkeys(listed) if flow_id not in flows]
    return {"flows": flows, "unmodeled": unmodeled, "errors": errors}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Business-flow catalog checker")
    parser.add_argument("command", choices=["check"])
    parser.add_argument("catalog", type=Path)
    parser.add_argument(
        "--json", action="store_true", help="print the summary as one JSON line last"
    )
    args = parser.parse_args(argv)
    if not (args.catalog / "README.md").is_file():
        print(
            f"flow_ledger: catalog directory without README.md: {args.catalog}",
            file=sys.stderr,
        )
        return 2
    result = check_catalog(args.catalog)
    for item in result["errors"]:
        print(f"ERROR {item}")
    for flow_id, flow in result["flows"].items():
        verdicts = (
            " ".join(f"{k}={v}" for k, v in sorted(flow["verdicts"].items())) or "none"
        )
        print(
            f"{flow_id}: branches={flow['branches']} designed={flow['designed']} bound={flow['bound']} {verdicts}"
        )
        for key in flow["gap_terminals"]:
            print(f"  WARN gap terminal {key}")
        for key in flow["open_high_risk"]:
            print(f"  WARN high-risk branch without PASS {key}")
    if result["unmodeled"]:
        print("unmodeled: " + ", ".join(result["unmodeled"]))
    print(f"# {len(result['errors'])} error(s), {len(result['flows'])} modeled flow(s)")
    if args.json:
        print(json.dumps(result, ensure_ascii=False))
    return 1 if result["errors"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
