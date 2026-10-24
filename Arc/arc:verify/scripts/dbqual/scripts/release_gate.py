#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
from typing import Any

from sqlverify_common import json_load, load_evidence_dir, load_run_manifest, verify_candidate

POLICY_SCHEMA = Path(__file__).resolve().parents[1] / "schemas" / "gate-policy.schema.json"


def _get(d: dict[str, Any], path: str):
    cur: Any = d
    for part in path.split("."):
        if not isinstance(cur, dict) or part not in cur:
            return None
        cur = cur[part]
    return cur


def _cmp(value: Any, op: str, threshold: float) -> bool:
    if not isinstance(value, (int, float)) or (isinstance(value, float) and math.isnan(value)):
        return False
    return {"<=": value <= threshold, "<": value < threshold, ">=": value >= threshold, ">": value > threshold, "==": value == threshold}[op]


def _matches(ev: dict[str, Any], selector: dict[str, Any]) -> bool:
    if selector.get("id") is not None and ev.get("id") != selector["id"]:
        return False
    if selector.get("kind") is not None and ev.get("kind") != selector["kind"]:
        return False
    if selector.get("id_prefix") is not None and not str(ev.get("id", "")).startswith(str(selector["id_prefix"])):
        return False
    return True


def _selector_label(selector: dict[str, Any]) -> str:
    return str(selector.get("id") or selector.get("kind") or selector.get("id_prefix") or "unnamed selector")


def policy_problems(policy: Any) -> list[dict[str, Any]]:
    """Keys outside the schema and a policy without any assertion would let the gate check nothing."""
    sections = json_load(POLICY_SCHEMA)["properties"]
    if not isinstance(policy, dict):
        return [{"type": "invalid_policy", "detail": "policy must be a JSON object"}]
    problems = [{"type": "invalid_policy", "key": key, "detail": "not a policy section"} for key in sorted(set(policy) - set(sections))]
    for key, spec in sections.items():
        items = policy.get(key, [])
        if not isinstance(items, list):
            problems.append({"type": "invalid_policy", "key": key, "detail": "must be a list"})
            continue
        allowed = set(spec["items"]["properties"])
        for index, item in enumerate(items):
            if not isinstance(item, dict):
                problems.append({"type": "invalid_policy", "key": f"{key}[{index}]", "detail": "must be an object"})
            elif set(item) - allowed:
                problems.append({"type": "invalid_policy", "key": f"{key}[{index}]", "detail": f"unknown fields {sorted(set(item) - allowed)}"})
    if not problems and not any(policy.get(key) for key in sections):
        problems.append({"type": "empty_policy", "detail": "no required_evidence or metric_rule: nothing would be checked"})
    return problems


def evaluate(evidence: list[dict[str, Any]], policy: dict[str, Any], issues: list[dict[str, Any]] | None = None) -> dict[str, Any]:
    """issues: run-binding and evidence-file problems found while loading; each one fails the gate."""
    by_id = {e.get("id"): e for e in evidence if e.get("id")}
    failures: list[dict[str, Any]] = list(issues or [])
    warnings: list[dict[str, Any]] = []
    problems = policy_problems(policy)
    if problems:
        return {"status": "FAIL", "failures": failures + problems, "warnings": warnings, "evaluated_evidence": []}

    for req in policy.get("required_evidence", []):
        if not any(req.get(k) is not None for k in ("id", "kind", "id_prefix")):
            failures.append({"type": "invalid_evidence_selector", "selector": req})
            continue
        matches = [ev for ev in evidence if _matches(ev, req)]
        allowed = set(req.get("allowed_status", ["PASS"]))
        good = [ev for ev in matches if ev.get("status") in allowed]
        min_count = int(req.get("min_count", 1))
        label = _selector_label(req)
        if len(good) < min_count:
            if not matches:
                failures.append({"type": "missing_evidence", "selector": label, "required": min_count})
            else:
                failures.append({
                    "type": "bad_status_or_count",
                    "selector": label,
                    "required": min_count,
                    "allowed": sorted(allowed),
                    "matching": [{"id": e.get("id"), "kind": e.get("kind"), "status": e.get("status")} for e in matches],
                })

    for rule in policy.get("metric_rule", []):
        if rule.get("evidence_id") == "*":
            candidates = list(evidence)
        elif rule.get("evidence_id") is not None:
            candidates = [by_id.get(rule.get("evidence_id"))]
        else:
            selector = {k: rule.get(k) for k in ("id", "kind", "id_prefix") if rule.get(k) is not None}
            candidates = [ev for ev in evidence if _matches(ev, selector)] if selector else []
        candidates = [x for x in candidates if x]
        if not candidates:
            failures.append({"type": "metric_evidence_missing", "rule": rule})
            continue
        min_count = int(rule.get("min_count", 1))
        if len(candidates) < min_count:
            failures.append({"type": "metric_evidence_count", "rule": rule, "matched": len(candidates), "required": min_count})
            continue
        for ev in candidates:
            val = _get(ev, rule["path"])
            ok = _cmp(val, rule.get("op", "<="), float(rule["threshold"]))
            if not ok:
                item = {"type": "metric_gate", "id": ev.get("id"), "path": rule["path"], "value": val, "op": rule.get("op", "<="), "threshold": rule["threshold"]}
                (warnings if rule.get("severity") == "warn" else failures).append(item)

    # A PASS containing violations indicates a harness/data-contract bug and is always fatal.
    for ev in evidence:
        if ev.get("status") == "PASS" and ev.get("violations"):
            failures.append({"type": "inconsistent_pass", "id": ev.get("id"), "violations": ev.get("violations")})

    return {
        "status": "PASS" if not failures else "FAIL",
        "failures": failures,
        "warnings": warnings,
        "evaluated_evidence": sorted(x for x in by_id if x),
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Claim/metric based database release gate")
    ap.add_argument("--policy", required=True, help="JSON policy file")
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument("--root", default=".", help="candidate root used by run_suite; its fingerprint must still match the run manifest")
    ap.add_argument("--out")
    ns = ap.parse_args()
    policy = json_load(Path(ns.policy))
    evdir = Path(ns.evidence_dir)
    manifest, issues = load_run_manifest(evdir)
    if manifest:
        issues += verify_candidate(manifest, Path(ns.root))
    evidence, load_issues = load_evidence_dir(evdir, run_id=manifest["run_id"] if manifest else None)
    out = evaluate(evidence, policy, issues + load_issues)
    out["run_id"] = manifest["run_id"] if manifest else None
    out["candidate"] = manifest.get("candidate") if manifest else None
    text = json.dumps(out, ensure_ascii=False, indent=2)
    if ns.out:
        Path(ns.out).write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0 if out["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
