#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load

_GATE_RE = re.compile(r"^G(?:[0-9]|[12][0-9]|3[01])$")


def _matches(ev: dict[str, Any], selector: dict[str, Any]) -> bool:
    if selector.get("kind") is not None and ev.get("kind") != selector["kind"]:
        return False
    if selector.get("id") is not None and ev.get("id") != selector["id"]:
        return False
    if selector.get("id_prefix") is not None and not str(ev.get("id", "")).startswith(str(selector["id_prefix"])):
        return False
    return True


def evaluate_coverage(evidence: list[dict[str, Any]], manifest: dict[str, Any]) -> dict[str, Any]:
    failures: list[str] = []
    warnings: list[str] = []
    observed_claims: list[dict[str, Any]] = []
    claims = manifest.get("claims", [])

    seen: set[str] = set()
    for claim in claims:
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
            rationale = str(claim.get("rationale", "")).strip()
            if not rationale:
                failures.append(f"{gate}: non-applicable claim requires rationale")
            row["rationale"] = rationale
            observed_claims.append(row)
            continue

        requirements = claim.get("require", [])
        if not requirements:
            failures.append(f"{gate}: applicable claim has no evidence requirements")
            observed_claims.append(row)
            continue

        req_rows: list[dict[str, Any]] = []
        for req in requirements:
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
                "matching_evidence": [{"id": e.get("id"), "kind": e.get("kind"), "status": e.get("status")} for e in matches],
            }
            req_rows.append(req_row)
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
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Fail-closed applicability/evidence completeness gate for G0-G31 database qualification claims")
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    start = time.monotonic()
    ev = evidence_base("qualification_coverage", "qualification.coverage")
    manifest = json_load(Path(ns.manifest))
    evidence: list[dict[str, Any]] = []
    for p in sorted(Path(ns.evidence_dir).glob("*.json")):
        try:
            obj = json_load(p)
            if isinstance(obj, dict) and obj.get("id") and obj.get("id") != ev["id"]:
                evidence.append(obj)
        except Exception as exc:
            ev["warnings"].append(f"ignored unreadable evidence {p.name}: {exc!r}")
    result = evaluate_coverage(evidence, manifest)
    ev["observed"] = result
    ev["violations"].extend(result["failures"])
    ev["warnings"].extend(result["warnings"])
    ev["reproduction"] = {"manifest": str(Path(ns.manifest).resolve()), "evidence_dir": str(Path(ns.evidence_dir).resolve())}
    status = "PASS" if not ev["violations"] else "FAIL"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
