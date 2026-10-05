#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover
    print("Python 3.11+ is required (tomllib missing)", file=sys.stderr)
    raise

RISK = {"low", "medium", "high", "critical"}
ADVERSE_CLASSES = {
    "invalid", "permission", "boundary", "state", "state-transition", "duplicate", "retry",
    "duplicate/retry", "duplicate/concurrency", "concurrency", "timeout", "failure", "recovery",
    "compatibility", "migration", "expiry", "cancellation", "partial-failure", "overload",
}
NORMAL_CLASSES = {"normal", "happy", "happy-path", "success"}
APPLICABILITY_KEYS = {
    "stateful", "permissions", "concurrent", "retryable", "external_side_effects", "time_sensitive",
    "compatibility", "migration", "asynchronous", "destructive", "long_running",
}


def load_plan(path: Path) -> dict:
    with path.open("rb") as f:
        return tomllib.load(f)


def _list(v):
    return v if isinstance(v, list) else []


def _text(v) -> str:
    return str(v).strip() if v is not None else ""


def _is_true(data: dict, key: str) -> bool:
    return bool((data.get("applicability") or {}).get(key, False))


def validate(data: dict) -> list[dict]:
    """Validate a business verification contract.

    This is deliberately a *completeness/traceability* gate, not a quality score.
    It checks whether declared business characteristics have corresponding model/evidence.
    It does not decide whether the domain semantics themselves are correct.
    """
    issues: list[dict] = []

    def add(severity: str, code: str, message: str):
        issues.append({"severity": severity, "code": code, "message": message})

    plan = data.get("plan") or {}
    for key in ("id", "title", "risk", "objective"):
        if not plan.get(key):
            add("error", "PLAN-MISSING", f"plan.{key} is required")
    risk = _text(plan.get("risk")).lower()
    if risk and risk not in RISK:
        add("error", "PLAN-RISK", f"unsupported plan.risk={risk!r}")

    applicability = data.get("applicability") or {}
    unknown = sorted(set(applicability) - APPLICABILITY_KEYS)
    if unknown:
        add("warning", "APPLICABILITY-UNKNOWN", f"unknown applicability keys: {', '.join(unknown)}")
    if not applicability:
        add("warning", "APPLICABILITY-MISSING", "declare [applicability] so omitted test dimensions are explicit rather than accidental")

    actors = data.get("actors", [])
    rules = data.get("rules", [])
    journeys = data.get("journeys", [])
    scenarios = data.get("scenarios", [])
    cross = data.get("cross_business", [])
    oracles = data.get("oracles", [])
    states = data.get("states", [])
    transitions = data.get("transitions", [])
    side_effects = data.get("side_effects", [])
    compat = data.get("compatibility_dimensions", [])

    seen: dict[str, set[str]] = defaultdict(set)
    for kind, items in (
        ("actor", actors), ("rule", rules), ("journey", journeys), ("scenario", scenarios),
        ("cross_business", cross), ("oracle", oracles), ("state", states), ("transition", transitions),
        ("side_effect", side_effects), ("compatibility_dimension", compat),
    ):
        for item in items:
            ident = _text(item.get("id"))
            if not ident:
                add("error", "ID-MISSING", f"{kind} item is missing id")
            elif ident in seen[kind]:
                add("error", "ID-DUPLICATE", f"duplicate {kind} id {ident}")
            else:
                seen[kind].add(ident)

    actor_ids = seen["actor"]
    rule_ids = seen["rule"]
    journey_ids = seen["journey"]
    scenario_ids = seen["scenario"]
    oracle_ids = seen["oracle"]
    state_ids = seen["state"]
    referenced_rules: set[str] = set()
    referenced_oracles: set[str] = set()

    if not rules:
        add("error", "NO-BUSINESS-RULES", "at least one explicit business rule is required")
    if not journeys:
        add("error", "NO-JOURNEYS", "at least one primary business journey is required")
    if not scenarios:
        add("error", "NO-SCENARIOS", "at least one concrete business scenario is required")

    for a in actors:
        aid = _text(a.get("id")) or "<unknown>"
        if _is_true(data, "permissions"):
            if not _list(a.get("allowed")) and not _list(a.get("forbidden")):
                add("error", "ACTOR-AUTHORITY-MISSING", f"{aid} has no allowed/forbidden authority model while permissions=true")

    for o in oracles:
        oid = _text(o.get("id")) or "<unknown>"
        kind = _text(o.get("kind")).lower()
        if not kind:
            add("error", "ORACLE-KIND", f"{oid} has no oracle kind")
        if not _text(o.get("observe")):
            add("error", "ORACLE-OBSERVE", f"{oid} does not say what is observed")
        if not _text(o.get("assertion")):
            add("error", "ORACLE-ASSERTION", f"{oid} has no business assertion")

    for r in rules:
        rid = _text(r.get("id")) or "<unknown>"
        if not _text(r.get("statement")):
            add("error", "RULE-NO-STATEMENT", f"{rid} has no business rule statement")
        rrisk = _text(r.get("risk") or risk or "medium").lower()
        if rrisk not in RISK:
            add("error", "RULE-RISK", f"{rid} has unsupported risk {rrisk!r}")
        refs = _list(r.get("oracle_ids"))
        legacy = _text(r.get("oracle"))
        if not refs and not legacy:
            add("error", "RULE-NO-ORACLE", f"{rid} has no business oracle")
        for oid in refs:
            referenced_oracles.add(oid)
            if oid not in oracle_ids:
                add("error", "RULE-ORACLE", f"{rid} references unknown oracle {oid}")
        if rrisk in {"high", "critical"} and not refs:
            add("error", "RULE-ORACLE-UNSTRUCTURED", f"{rid} is high/critical and requires typed oracle_ids")

    for j in journeys:
        jid = _text(j.get("id")) or "<unknown>"
        unknown_actors = [x for x in _list(j.get("actors")) if x not in actor_ids]
        if unknown_actors:
            add("error", "JOURNEY-ACTOR", f"{jid} references unknown actors: {', '.join(unknown_actors)}")
        if not _text(j.get("trigger")):
            add("warning", "JOURNEY-NO-TRIGGER", f"{jid} has no explicit trigger")
        if not _list(j.get("expected")):
            add("error", "JOURNEY-NO-EXPECTED", f"{jid} has no expected business outcomes")
        if not _list(j.get("forbidden")):
            sev = "error" if risk in {"high", "critical"} else "warning"
            add(sev, "JOURNEY-NO-FORBIDDEN", f"{jid} has no forbidden business outcomes")
        for rid in _list(j.get("rules")):
            referenced_rules.add(rid)
            if rid not in rule_ids:
                add("error", "JOURNEY-RULE", f"{jid} references unknown rule {rid}")

    classes: set[str] = set()
    for s in scenarios:
        sid = _text(s.get("id")) or "<unknown>"
        jid = _text(s.get("journey"))
        if not jid or jid not in journey_ids:
            add("error", "SCENARIO-JOURNEY", f"{sid} references unknown/missing journey {jid!r}")
        klass = _text(s.get("class")).lower() or "unspecified"
        classes.add(klass)
        if not _list(s.get("given")):
            add("warning", "SCENARIO-NO-GIVEN", f"{sid} has no explicit precondition/test data")
        if not _text(s.get("when")):
            add("error", "SCENARIO-NO-WHEN", f"{sid} has no action/trigger")
        if not _list(s.get("expected")):
            add("error", "SCENARIO-NO-EXPECTED", f"{sid} has no expected business outcome")
        if not _list(s.get("forbidden")):
            add("warning", "SCENARIO-NO-FORBIDDEN", f"{sid} has no forbidden outcome")
        if not _text(s.get("evidence")):
            add("warning", "SCENARIO-NO-EVIDENCE", f"{sid} has no intended evidence/test layer")
        automation = _text(s.get("automation")).lower()
        if automation and automation not in {"automated", "manual", "analysis"}:
            add("error", "SCENARIO-AUTOMATION", f"{sid} has unsupported automation mode {automation!r}")
        if risk in {"high", "critical"} and not automation:
            add("error", "SCENARIO-AUTOMATION-MISSING", f"{sid} must declare automation=automated|manual|analysis")
        if automation == "manual" and not _list(s.get("manual_steps")):
            add("error", "SCENARIO-MANUAL-PROCEDURE", f"{sid} is manual but has no manual_steps")
        if automation == "analysis" and not _text(s.get("analysis_ref")):
            add("error", "SCENARIO-ANALYSIS-REF", f"{sid} is analysis-only but has no analysis_ref")
        for rid in _list(s.get("rules")):
            referenced_rules.add(rid)
            if rid not in rule_ids:
                add("error", "SCENARIO-RULE", f"{sid} references unknown rule {rid}")
        scenario_oracles = _list(s.get("oracle_ids"))
        for oid in scenario_oracles:
            referenced_oracles.add(oid)
            if oid not in oracle_ids:
                add("error", "SCENARIO-ORACLE", f"{sid} references unknown oracle {oid}")
        if risk in {"high", "critical"} and not scenario_oracles:
            add("error", "SCENARIO-ORACLE-UNBOUND", f"{sid} has no typed oracle_ids; high/critical expected/forbidden must be tied to explicit oracles")

    for rid in sorted(rule_ids - referenced_rules):
        add("error", "RULE-UNTESTED", f"business rule {rid} is not referenced by any journey/scenario")
    for oid in sorted(oracle_ids - referenced_oracles):
        add("warning", "ORACLE-UNUSED", f"oracle {oid} is defined but unused")

    # Explicit applicability drives required evidence. No aggregate score is produced.
    if _is_true(data, "stateful"):
        if not states:
            add("error", "STATE-MODEL-MISSING", "stateful=true requires [[states]]")
        if not transitions:
            add("error", "TRANSITIONS-MISSING", "stateful=true requires [[transitions]]")
        for t in transitions:
            tid = _text(t.get("id")) or "<unknown>"
            fr, to = _text(t.get("from")), _text(t.get("to"))
            if fr and fr not in state_ids:
                add("error", "TRANSITION-STATE", f"{tid} references unknown from-state {fr}")
            if to and to not in state_ids:
                add("error", "TRANSITION-STATE", f"{tid} references unknown to-state {to}")
            if not _text(t.get("event")):
                add("error", "TRANSITION-EVENT", f"{tid} has no triggering event")
        if not any(c in {"state", "state-transition", "invalid"} for c in classes):
            add("error", "STATE-SCENARIO-MISSING", "stateful=true requires scenario coverage of legal/illegal state transitions")

    if _is_true(data, "concurrent"):
        if not cross and not any("concurr" in c for c in classes):
            add("error", "CONCURRENCY-SCENARIO-MISSING", "concurrent=true requires cross-business or concurrency scenarios")

    if _is_true(data, "retryable") and not any(("retry" in c or "duplicate" in c) for c in classes):
        add("error", "RETRY-SCENARIO-MISSING", "retryable=true requires duplicate/retry scenario(s)")

    if _is_true(data, "time_sensitive") and not any(c in {"boundary", "expiry", "timeout"} for c in classes):
        add("error", "TIME-SCENARIO-MISSING", "time_sensitive=true requires boundary/expiry/timeout scenario(s)")

    if _is_true(data, "compatibility"):
        if not compat:
            add("error", "COMPAT-DIMENSION-MISSING", "compatibility=true requires [[compatibility_dimensions]]")
        if not any(c == "compatibility" for c in classes):
            add("error", "COMPAT-SCENARIO-MISSING", "compatibility=true requires compatibility scenario(s)")

    if _is_true(data, "migration") and not any(c == "migration" for c in classes):
        add("error", "MIGRATION-SCENARIO-MISSING", "migration=true requires migration/lifecycle scenario(s)")

    if _is_true(data, "external_side_effects"):
        if not side_effects:
            add("error", "SIDE-EFFECT-MODEL-MISSING", "external_side_effects=true requires [[side_effects]]")
        for e in side_effects:
            eid = _text(e.get("id")) or "<unknown>"
            if not _text(e.get("commit_point")):
                add("error", "SIDE-EFFECT-COMMIT-POINT", f"{eid} has no documented commit/visibility point")
            if not _text(e.get("duplicate_semantics")):
                add("error", "SIDE-EFFECT-DUPLICATE", f"{eid} has no duplicate/retry semantics")

    if _is_true(data, "destructive"):
        if not any(c in {"recovery", "cancellation", "failure", "partial-failure"} for c in classes):
            add("error", "DESTRUCTIVE-RECOVERY-MISSING", "destructive=true requires recovery/rollback/partial-failure scenario(s)")

    if _is_true(data, "asynchronous"):
        if not any(c in {"timeout", "failure", "recovery", "duplicate/retry", "retry"} for c in classes):
            add("error", "ASYNC-FAILURE-MISSING", "asynchronous=true requires delivery/timeout/retry/recovery coverage")

    if risk in {"high", "critical"}:
        normal = any(c in NORMAL_CLASSES for c in classes)
        adverse = any(c in ADVERSE_CLASSES or c not in NORMAL_CLASSES | {"unspecified"} for c in classes)
        if not normal:
            add("error", "PLAN-NO-NORMAL", "high/critical plan has no normal success scenario")
        if not adverse:
            add("error", "PLAN-NO-ADVERSE", "high/critical plan has no adverse/exception scenario")
        env = data.get("environment") or {}
        if not _text(env.get("isolation")):
            add("error", "ENV-ISOLATION-MISSING", "high/critical plan must document test data/environment isolation")
        release = data.get("release") or {}
        required = _list(release.get("required_scenarios"))
        if not required:
            add("error", "RELEASE-CRITERIA-MISSING", "high/critical plan must list release.required_scenarios")
        for sid in required:
            if sid not in scenario_ids:
                add("error", "RELEASE-SCENARIO", f"release requires unknown scenario {sid}")
        if not _text(release.get("rollback")) and risk == "critical":
            add("warning", "RELEASE-ROLLBACK-UNSPECIFIED", "critical plan has no documented rollback/containment strategy")

    for x in cross:
        xid = _text(x.get("id")) or "<unknown>"
        if len(_list(x.get("flows"))) < 2:
            add("error", "CROSS-FLOWS", f"{xid} must name at least two interacting business flows")
        if not _list(x.get("expected")):
            add("error", "CROSS-NO-EXPECTED", f"{xid} has no expected combined outcome")
        if not _list(x.get("forbidden")):
            add("warning", "CROSS-NO-FORBIDDEN", f"{xid} has no forbidden combined outcome")

    return issues


def render_markdown(data: dict) -> str:
    p = data.get("plan", {})
    lines = [f"# Business Verification Plan — {p.get('title', 'Untitled')}", ""]
    lines += [
        f"- ID: `{p.get('id', '')}`",
        f"- Risk: **{p.get('risk', '')}**",
        f"- Owner: {p.get('owner', '')}",
        f"- Objective: {p.get('objective', '')}",
        "",
    ]

    if data.get("applicability"):
        lines += ["## Applicability", ""]
        for k, v in sorted(data["applicability"].items()):
            lines.append(f"- {k}: **{'yes' if v else 'no'}**")
        lines.append("")

    lines += ["## Business rules", "", "| ID | Risk | Rule | Oracles |", "|---|---|---|---|"]
    for r in data.get("rules", []):
        oracle_text = ", ".join(_list(r.get("oracle_ids"))) or _text(r.get("oracle"))
        lines.append(f"| {r.get('id','')} | {r.get('risk','')} | {r.get('statement','')} | {oracle_text} |")
    lines.append("")

    if data.get("oracles"):
        lines += ["## Business oracles", "", "| ID | Kind | Observe | Assertion |", "|---|---|---|---|"]
        for o in data.get("oracles", []):
            lines.append(f"| {o.get('id','')} | {o.get('kind','')} | {o.get('observe','')} | {o.get('assertion','')} |")
        lines.append("")

    if data.get("states"):
        lines += ["## State model", "", "| State | Terminal | Meaning |", "|---|---|---|"]
        for s in data.get("states", []):
            lines.append(f"| {s.get('id','')} | {bool(s.get('terminal', False))} | {s.get('meaning','')} |")
        lines.append("")
        lines += ["### Transitions", "", "| ID | From | Event | To |", "|---|---|---|---|"]
        for t in data.get("transitions", []):
            lines.append(f"| {t.get('id','')} | {t.get('from','')} | {t.get('event','')} | {t.get('to','')} |")
        lines.append("")

    lines += ["## Journeys", ""]
    for j in data.get("journeys", []):
        lines.append(f"### {j.get('id','')} — {j.get('title','')}")
        lines.append(f"- Actors: {', '.join(_list(j.get('actors')))}")
        lines.append(f"- Preconditions: {'; '.join(_list(j.get('preconditions')))}")
        lines.append(f"- Trigger: {j.get('trigger','')}")
        lines.append(f"- Expected: {'; '.join(_list(j.get('expected')))}")
        lines.append(f"- Forbidden: {'; '.join(_list(j.get('forbidden')))}")
        lines.append("")

    lines += ["## Scenario matrix", "", "| ID | Class | Given | When | Expected | Forbidden | Oracles | Evidence |", "|---|---|---|---|---|---|---|---|"]
    for s in data.get("scenarios", []):
        lines.append(
            f"| {s.get('id','')} | {s.get('class','')} | {'; '.join(_list(s.get('given')))} | {s.get('when','')} | "
            f"{'; '.join(_list(s.get('expected')))} | {'; '.join(_list(s.get('forbidden')))} | "
            f"{', '.join(_list(s.get('oracle_ids')))} | {s.get('evidence','')} |"
        )
    lines.append("")

    if data.get("cross_business"):
        lines += ["## Cross-business interactions", "", "| ID | Flows | Shared state | Expected | Forbidden |", "|---|---|---|---|---|"]
        for x in data.get("cross_business", []):
            lines.append(
                f"| {x.get('id','')} | {' + '.join(_list(x.get('flows')))} | {'; '.join(_list(x.get('shared_state')))} | "
                f"{'; '.join(_list(x.get('expected')))} | {'; '.join(_list(x.get('forbidden')))} |"
            )
        lines.append("")

    release = data.get("release") or {}
    if release:
        lines += ["## Release contract", "", f"- Required scenarios: {', '.join(_list(release.get('required_scenarios')))}", f"- Rollback/containment: {release.get('rollback','')}", ""]

    return "\n".join(lines)


def main() -> int:
    ap = argparse.ArgumentParser(description="Validate and render a business-first verification plan")
    sub = ap.add_subparsers(dest="cmd", required=True)

    v = sub.add_parser("validate")
    v.add_argument("--plan", required=True, type=Path)
    v.add_argument("--json", action="store_true")

    r = sub.add_parser("render")
    r.add_argument("--plan", required=True, type=Path)
    r.add_argument("--out", type=Path)

    args = ap.parse_args()
    data = load_plan(args.plan)

    if args.cmd == "validate":
        issues = validate(data)
        if args.json:
            print(json.dumps(issues, ensure_ascii=False, indent=2))
        else:
            if not issues:
                print("PASS business verification plan")
            for i in issues:
                print(f"{i['severity'].upper():7} {i['code']}: {i['message']}")
        return 1 if any(i["severity"] == "error" for i in issues) else 0

    md = render_markdown(data)
    if args.out:
        args.out.write_text(md, encoding="utf-8")
        print(args.out)
    else:
        print(md)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
