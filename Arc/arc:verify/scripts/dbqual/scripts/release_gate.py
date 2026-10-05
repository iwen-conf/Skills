#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
from typing import Any

from sqlverify_common import json_load


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


def evaluate(evidence: list[dict[str, Any]], policy: dict[str, Any]) -> dict[str, Any]:
    by_id = {e.get("id"): e for e in evidence if e.get("id")}
    failures: list[dict[str, Any]] = []
    warnings: list[dict[str, Any]] = []

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
    ap.add_argument("--out")
    ns = ap.parse_args()
    policy = json_load(Path(ns.policy))
    evidence = []
    for p in sorted(Path(ns.evidence_dir).glob("*.json")):
        try:
            obj = json_load(p)
            if isinstance(obj, dict) and "status" in obj and "id" in obj:
                evidence.append(obj)
        except Exception:
            pass
    out = evaluate(evidence, policy)
    text = json.dumps(out, ensure_ascii=False, indent=2)
    if ns.out:
        Path(ns.out).write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0 if out["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
