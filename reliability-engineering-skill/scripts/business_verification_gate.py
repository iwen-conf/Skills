#!/usr/bin/env python3
"""Cross-check business plan, executable bindings, and produced evidence.

This gate intentionally produces PASS/FAIL/BLOCKED style facts, not a quality
score. It prevents a plan from claiming automated coverage when no runnable
binding or execution evidence exists.
"""
from __future__ import annotations

import argparse
import json
import sys
import tomllib
from pathlib import Path
from typing import Any

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from business_test_plan import validate as validate_plan  # noqa: E402
from business_scenario_runner import validate_binding  # noqa: E402


def load_toml(path: Path) -> dict[str, Any]:
    with path.open("rb") as f:
        return tomllib.load(f)


def load_evidence_dir(path: Path | None) -> dict[str, dict[str, Any]]:
    out: dict[str, dict[str, Any]] = {}
    if path is None or not path.exists():
        return out
    for p in sorted(path.glob("*.json")):
        try:
            data = json.loads(p.read_text(encoding="utf-8"))
        except Exception:
            continue
        sid = str(data.get("scenario_id", ""))
        if sid:
            out[sid] = data
    return out


def evaluate(plan: dict[str, Any], binding: dict[str, Any] | None = None,
             evidence: dict[str, dict[str, Any]] | None = None,
             require_evidence: bool = False) -> dict[str, Any]:
    binding = binding or {"execution": []}
    evidence = evidence or {}
    issues: list[dict[str, str]] = []

    for i in validate_plan(plan):
        issues.append({"severity": i["severity"], "code": i["code"], "message": i["message"]})
    for msg in validate_binding(binding, plan):
        issues.append({"severity": "error", "code": "BINDING", "message": msg})

    bound = {str(x.get("scenario_id")) for x in binding.get("execution", []) if x.get("scenario_id")}
    scenarios = {str(x.get("id")): x for x in plan.get("scenarios", []) if x.get("id")}
    release = plan.get("release") or {}
    required = [str(x) for x in release.get("required_scenarios", [])]

    for sid in required:
        scenario = scenarios.get(sid)
        if not scenario:
            continue
        mode = str(scenario.get("automation", "")).lower()
        if mode == "automated" and sid not in bound:
            issues.append({"severity": "error", "code": "AUTOMATION-NO-BINDING",
                           "message": f"release scenario {sid} is automated but has no executable binding"})
        if mode == "manual" and not scenario.get("manual_steps"):
            issues.append({"severity": "error", "code": "MANUAL-NO-PROCEDURE",
                           "message": f"release scenario {sid} is manual but has no procedure"})
        if require_evidence:
            if mode == "automated":
                ev = evidence.get(sid)
                if not ev:
                    issues.append({"severity": "error", "code": "EVIDENCE-MISSING",
                                   "message": f"release scenario {sid} has no execution evidence"})
                elif str(ev.get("status")) != "PASS":
                    issues.append({"severity": "error", "code": "EVIDENCE-NOT-PASS",
                                   "message": f"release scenario {sid} evidence status={ev.get('status')}"})
            elif mode == "manual" and not scenario.get("evidence_artifact"):
                issues.append({"severity": "error", "code": "MANUAL-EVIDENCE-MISSING",
                               "message": f"release scenario {sid} has no evidence_artifact recorded"})

    errors = [x for x in issues if x["severity"] == "error"]
    return {
        "status": "FAIL" if errors else "PASS",
        "required_release_scenarios": required,
        "bound_scenarios": sorted(bound),
        "evidence_scenarios": sorted(evidence),
        "issues": issues,
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Gate business verification plan -> binding -> evidence traceability")
    ap.add_argument("--plan", required=True, type=Path)
    ap.add_argument("--binding", type=Path)
    ap.add_argument("--evidence-dir", type=Path)
    ap.add_argument("--require-evidence", action="store_true")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()

    plan = load_toml(args.plan)
    binding = load_toml(args.binding) if args.binding else {"execution": []}
    evidence = load_evidence_dir(args.evidence_dir)
    result = evaluate(plan, binding, evidence, args.require_evidence)
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(f"STATUS: {result['status']}")
        for x in result["issues"]:
            print(f"{x['severity'].upper():7} {x['code']}: {x['message']}")
    return 1 if result["status"] == "FAIL" else 0


if __name__ == "__main__":
    raise SystemExit(main())
