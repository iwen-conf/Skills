#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    json_load,
    load_evidence_dir,
    load_run_manifest,
)
from release_gate import _matches, policy_problems

_GATE_RE = re.compile(r"^G(?:[0-9]|[12][0-9]|3[01])$")


def evaluate_coverage(evidence: list[dict[str, Any]], manifest: dict[str, Any]) -> dict[str, Any]:
    failures: list[str] = []
    warnings: list[str] = []
    observed_claims: list[dict[str, Any]] = []
    if not isinstance(manifest, dict):
        return {"claims": [], "failures": ["qualification manifest must be an object"], "warnings": []}
    if "strict_complete" in manifest and type(manifest["strict_complete"]) is not bool:
        failures.append("strict_complete must be boolean")
    ids = [ev.get("id") for ev in evidence]
    if len(ids) != len(set(ids)):
        failures.append("duplicate evidence ids cannot satisfy multiple observations")
    if any(ev.get("status") == "PASS" and ev.get("violations") for ev in evidence):
        failures.append("PASS evidence contains violations")
    claims = manifest.get("claims", [])
    if not isinstance(claims, list) or not claims:
        failures.append("qualification manifest declares no claims: nothing would be checked")
        claims = []

    seen: set[str] = set()
    for claim in claims:
        if not isinstance(claim, dict):
            failures.append("claim must be an object")
            continue
        gate = str(claim.get("gate", ""))
        if not _GATE_RE.fullmatch(gate):
            failures.append(f"invalid gate id: {gate!r}")
            continue
        if gate in seen:
            failures.append(f"duplicate gate declaration: {gate}")
            continue
        seen.add(gate)
        if "applicable" not in claim or not isinstance(claim.get("applicable"), bool):
            failures.append(f"{gate}: applicable must be explicit boolean")
            continue

        applicable = bool(claim["applicable"])
        row: dict[str, Any] = {"gate": gate, "applicable": applicable, "title": claim.get("title")}
        if not applicable:
            rationale = claim.get("rationale", "")
            rationale = rationale.strip() if isinstance(rationale, str) else ""
            if not rationale:
                failures.append(f"{gate}: non-applicable claim requires rationale")
            row["rationale"] = rationale
            observed_claims.append(row)
            continue

        requirements = claim.get("require", [])
        if not isinstance(requirements, list) or not requirements:
            failures.append(f"{gate}: applicable claim has no evidence requirements")
            observed_claims.append(row)
            continue

        req_rows: list[dict[str, Any]] = []
        for req in requirements:
            if not isinstance(req, dict) or set(req) - {
                "kind",
                "id",
                "id_prefix",
                "allowed_status",
                "min_pass",
                "label",
            }:
                failures.append(f"{gate}: invalid evidence requirement")
                continue
            selector = {k: v for k, v in req.items() if k not in {"min_pass", "label"}}
            selector["min_count"] = req.get("min_pass", 1)
            problems = policy_problems({"required_evidence": [selector]})
            if problems:
                failures.append(f"{gate}: invalid evidence requirement: {problems}")
                continue
            matches = [ev for ev in evidence if _matches(ev, req)]
            allowed = set(req.get("allowed_status", ["PASS"]))
            good = [ev for ev in matches if ev.get("status") in allowed]
            min_pass = int(req.get("min_pass", 1))
            label = req.get("label") or req.get("id") or req.get("id_prefix") or req.get("kind") or "selector"
            req_row = {
                "label": label,
                "selector": {k: req.get(k) for k in ("kind", "id", "id_prefix") if req.get(k) is not None},
                "allowed_status": sorted(allowed),
                "matched": len(matches),
                "satisfied": len(good),
                "required": min_pass,
                "matching_evidence": [
                    {"id": e.get("id"), "kind": e.get("kind"), "status": e.get("status")} for e in matches
                ],
            }
            req_rows.append(req_row)
            if any(ev.get("status") != "PASS" for ev in matches):
                failures.append(f"{gate}: selected evidence for {label} contains a non-PASS result")
            if len(good) < min_pass:
                if not matches:
                    failures.append(f"{gate}: missing evidence for {label}")
                else:
                    states = ", ".join(f"{e.get('id')}={e.get('status')}" for e in matches)
                    failures.append(f"{gate}: evidence for {label} does not satisfy allowed status/count ({states})")
        row["requirements"] = req_rows
        observed_claims.append(row)

    if manifest.get("strict_complete", False):
        expected = {f"G{i}" for i in range(32)}
        missing = sorted(expected - seen, key=lambda x: int(x[1:]))
        if missing:
            failures.append("qualification manifest omits gates: " + ", ".join(missing))

    return {
        "risk_class": manifest.get("risk_class"),
        "profile": manifest.get("profile"),
        "claims": observed_claims,
        "failures": failures,
        "warnings": warnings,
        "applicable_claims": sum(1 for claim in observed_claims if claim.get("applicable")),
        "proof_scope": "declared_qualification_claims",
    }


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Fail-closed applicability/evidence completeness gate for G0-G31 database qualification claims"
    )
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    start = time.monotonic()
    ev = evidence_base("qualification_coverage", "qualification.coverage")
    manifest = json_load(Path(ns.manifest))
    # Evidence of another run, unreadable files and duplicate ids are failures, never skipped.
    run_manifest, issues = load_run_manifest(Path(ns.evidence_dir))
    evidence, load_issues = load_evidence_dir(
        Path(ns.evidence_dir), run_id=run_manifest["run_id"] if run_manifest else None, skip_ids=[ev["id"]]
    )
    result = evaluate_coverage(evidence, manifest)
    result["evidence_issues"] = issues + load_issues
    ev["observed"] = result
    ev["violations"].extend(json.dumps(x, ensure_ascii=False) for x in issues + load_issues)
    ev["violations"].extend(result["failures"])
    ev["warnings"].extend(result["warnings"])
    ev["reproduction"] = {
        "manifest": str(Path(ns.manifest).resolve()),
        "evidence_dir": str(Path(ns.evidence_dir).resolve()),
    }
    if issues or load_issues:
        status = "BLOCKED"
    elif ev["violations"]:
        status = "FAIL"
    elif result["applicable_claims"] == 0:
        status = "NOT_RUN"
        ev["warnings"].append(
            "all claims are non-applicable; applicability was checked but no runtime claim was executed"
        )
    else:
        status = "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
