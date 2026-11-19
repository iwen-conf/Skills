#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
from typing import Any

from jsonschema import Draft202012Validator

from sqlverify_common import (
    evidence_hashes,
    json_load,
    load_evidence_dir,
    load_run_manifest,
    sha256_file,
    verify_candidate,
)

POLICY_SCHEMA = Path(__file__).resolve().parents[1] / "schemas" / "gate-policy.schema.json"
POLICY_VALIDATOR = Draft202012Validator(json_load(POLICY_SCHEMA))


def _get(d: dict[str, Any], path: str):
    cur: Any = d
    for part in path.split("."):
        if not isinstance(cur, dict) or part not in cur:
            return None
        cur = cur[part]
    return cur


def _cmp(value: Any, op: str, threshold: float) -> bool:
    if not finite_number(value):
        return False
    return {
        "<=": value <= threshold,
        "<": value < threshold,
        ">=": value >= threshold,
        ">": value > threshold,
        "==": value == threshold,
    }[op]


def finite_number(value: Any) -> bool:
    return type(value) is int or (type(value) is float and math.isfinite(value))


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
    problems = [
        {"type": "invalid_policy", "key": ".".join(map(str, error.path)), "detail": error.message}
        for error in POLICY_VALIDATOR.iter_errors(policy)
    ]
    if problems:
        return problems
    for rule in policy.get("metric_rule", []):
        if not finite_number(rule["threshold"]):
            problems.append({"type": "invalid_policy", "detail": "metric threshold must be finite"})
    if not any(policy.get(key) for key in POLICY_VALIDATOR.schema["properties"]):
        problems.append(
            {"type": "empty_policy", "detail": "no required_evidence or metric_rule: nothing would be checked"}
        )
    return problems


def evaluate(
    evidence: list[dict[str, Any]], policy: dict[str, Any], issues: list[dict[str, Any]] | None = None
) -> dict[str, Any]:
    """issues: run-binding and evidence-file problems found while loading; each one fails the gate."""
    by_id: dict[str, dict[str, Any]] = {}
    failures: list[dict[str, Any]] = list(issues or [])
    warnings: list[dict[str, Any]] = []
    problems = policy_problems(policy)
    if problems:
        return {"status": "FAIL", "failures": failures + problems, "warnings": warnings, "evaluated_evidence": []}
    for ev in evidence:
        eid = ev.get("id")
        if not isinstance(eid, str) or not eid.strip():
            failures.append({"type": "invalid_evidence_id"})
        elif eid in by_id:
            failures.append({"type": "duplicate_evidence_id", "id": eid})
        else:
            by_id[eid] = ev

    for req in policy.get("required_evidence", []):
        if not any(req.get(k) is not None for k in ("id", "kind", "id_prefix")):
            failures.append({"type": "invalid_evidence_selector", "selector": req})
            continue
        matches = [ev for ev in evidence if _matches(ev, req)]
        allowed = set(req.get("allowed_status", ["PASS"]))
        good = [ev for ev in matches if ev.get("status") in allowed]
        min_count = int(req.get("min_count", 1))
        label = _selector_label(req)
        if any(ev.get("status") != "PASS" for ev in matches):
            failures.append({"type": "nonpass_required_evidence", "selector": label})
        if len(good) < min_count:
            if not matches:
                failures.append({"type": "missing_evidence", "selector": label, "required": min_count})
            else:
                failures.append(
                    {
                        "type": "bad_status_or_count",
                        "selector": label,
                        "required": min_count,
                        "allowed": sorted(allowed),
                        "matching": [
                            {"id": e.get("id"), "kind": e.get("kind"), "status": e.get("status")} for e in matches
                        ],
                    }
                )

    for rule in policy.get("metric_rule", []):
        if rule.get("evidence_id") == "*":
            candidates = list(evidence)
        elif rule.get("evidence_id") is not None:
            candidate = by_id.get(rule["evidence_id"])
            candidates = [candidate] if candidate is not None else []
        else:
            selector = {k: rule.get(k) for k in ("id", "kind", "id_prefix") if rule.get(k) is not None}
            candidates = [ev for ev in evidence if _matches(ev, selector)] if selector else []
        candidates = [x for x in candidates if x]
        if not candidates:
            failures.append({"type": "metric_evidence_missing", "rule": rule})
            continue
        min_count = int(rule.get("min_count", 1))
        if len(candidates) < min_count:
            failures.append(
                {"type": "metric_evidence_count", "rule": rule, "matched": len(candidates), "required": min_count}
            )
            continue
        for ev in candidates:
            if ev.get("status") != "PASS":
                failures.append({"type": "metric_evidence_status", "id": ev.get("id"), "status": ev.get("status")})
                continue
            val = _get(ev, rule["path"])
            if not finite_number(val):
                failures.append({"type": "invalid_metric", "id": ev.get("id"), "path": rule["path"], "value": val})
                continue
            ok = _cmp(val, rule.get("op", "<="), rule["threshold"])
            if not ok:
                item = {
                    "type": "metric_gate",
                    "id": ev.get("id"),
                    "path": rule["path"],
                    "value": val,
                    "op": rule.get("op", "<="),
                    "threshold": rule["threshold"],
                }
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


def revalidate_gate(gate: dict, evidence_dir: Path, root: Path) -> tuple[list[dict], list[dict]]:
    manifest, issues = load_run_manifest(evidence_dir)
    if not manifest:
        return [], issues
    if gate.get("run_id") != manifest["run_id"] or gate.get("candidate") != manifest.get("candidate"):
        issues.append({"type": "gate_run_or_candidate_mismatch"})
    issues += verify_candidate(manifest, root)
    evidence, load_issues = load_evidence_dir(evidence_dir, run_id=manifest["run_id"])
    issues += load_issues
    binding = gate.get("input_binding")
    if not isinstance(binding, dict) or not isinstance(binding.get("policy"), dict):
        return evidence, issues + [{"type": "missing_gate_input_binding"}]
    policy_ref = binding["policy"]
    policy_path = Path(policy_ref.get("path", ""))
    if not policy_path.is_file() or sha256_file(policy_path) != policy_ref.get("sha256"):
        return evidence, issues + [{"type": "gate_policy_changed"}]
    hashes = evidence_hashes(evidence_dir)
    if not hashes or hashes != binding.get("evidence"):
        issues.append({"type": "gated_evidence_changed"})
    current = evaluate(evidence, json_load(policy_path), load_issues)
    if current["status"] != gate.get("status"):
        issues.append({"type": "stored_gate_verdict_mismatch"})
    if hashes != evidence_hashes(evidence_dir) or sha256_file(policy_path) != policy_ref.get("sha256"):
        issues.append({"type": "inputs_changed_during_report"})
    issues += verify_candidate(manifest, root)
    return evidence, issues


def main() -> int:
    ap = argparse.ArgumentParser(description="Claim/metric based database release gate")
    ap.add_argument("--policy", required=True, help="JSON policy file")
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument(
        "--root",
        default=".",
        help="candidate root used by run_suite; its fingerprint must still match the run manifest",
    )
    ap.add_argument("--out")
    ns = ap.parse_args()
    policy_path = Path(ns.policy).resolve()
    evdir = Path(ns.evidence_dir)
    binding = {
        "policy": {"path": str(policy_path), "sha256": sha256_file(policy_path)},
        "evidence": evidence_hashes(evdir),
    }
    policy = json_load(policy_path)
    manifest, issues = load_run_manifest(evdir)
    if manifest:
        issues += verify_candidate(manifest, Path(ns.root))
    evidence, load_issues = load_evidence_dir(evdir, run_id=manifest["run_id"] if manifest else None)
    out = evaluate(evidence, policy, issues + load_issues)
    out["run_id"] = manifest["run_id"] if manifest else None
    out["candidate"] = manifest.get("candidate") if manifest else None
    if binding["policy"]["sha256"] != sha256_file(policy_path) or binding["evidence"] != evidence_hashes(evdir):
        out["failures"].append({"type": "inputs_changed_during_gate"})
    if manifest:
        out["failures"].extend(verify_candidate(manifest, Path(ns.root)))
    if out["failures"]:
        out["status"] = "FAIL"
    out["input_binding"] = binding
    text = json.dumps(out, ensure_ascii=False, indent=2)
    if ns.out:
        Path(ns.out).write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0 if out["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
