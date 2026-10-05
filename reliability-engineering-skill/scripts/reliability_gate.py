#!/usr/bin/env python3
"""Structural gate for a reliability.toml claim-to-evidence manifest.

Stdlib-only (Python 3.11+ for tomllib). This tool does NOT judge whether tests are
semantically sufficient; it detects missing traceability and policy gaps.
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import sys
from pathlib import Path

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover
    print("Python 3.11+ is required (tomllib missing).", file=sys.stderr)
    raise SystemExit(2)

VALID_RISKS = {"critical", "high", "medium", "low"}


def by_id(items, kind, errors):
    out = {}
    for i, item in enumerate(items):
        ident = item.get("id")
        if not ident:
            errors.append(f"{kind}[{i}] is missing id")
            continue
        if ident in out:
            errors.append(f"duplicate {kind} id: {ident}")
        out[ident] = item
    return out


def parse_date(value, label, errors):
    try:
        return dt.date.fromisoformat(value)
    except Exception:
        errors.append(f"{label} must be YYYY-MM-DD, got {value!r}")
        return None


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("manifest", nargs="?", default="reliability.toml")
    ap.add_argument("--json", action="store_true", help="emit JSON report")
    ap.add_argument("--today", help="override current date (YYYY-MM-DD) for reproducible CI tests")
    args = ap.parse_args()

    path = Path(args.manifest)
    if not path.exists():
        print(f"manifest not found: {path}", file=sys.stderr)
        return 2

    data = tomllib.loads(path.read_text(encoding="utf-8"))
    policy = data.get("policy", {})
    errors, warnings = [], []

    invariants = by_id(data.get("invariants", []), "invariant", errors)
    failures = by_id(data.get("failure_modes", []), "failure_mode", errors)
    evidence = by_id(data.get("evidence", []), "evidence", errors)
    controls = by_id(data.get("controls", []), "control", errors)
    incidents = by_id(data.get("incidents", []), "incident", errors)
    waivers = by_id(data.get("waivers", []), "waiver", errors)

    today = parse_date(args.today, "--today", errors) if args.today else dt.date.today()
    active_waivers = {}
    max_days = int(policy.get("waiver_max_days", 90))
    for wid, w in waivers.items():
        subject = w.get("subject")
        reason = w.get("reason")
        owner = w.get("owner")
        compensation = w.get("compensating_control")
        expires_raw = w.get("expires")
        if not all([subject, reason, owner, compensation, expires_raw]):
            errors.append(f"waiver {wid} requires subject, reason, owner, compensating_control, expires")
            continue
        expires = parse_date(expires_raw, f"waiver {wid}.expires", errors)
        if not expires:
            continue
        if expires < today:
            errors.append(f"waiver {wid} expired on {expires}")
            continue
        if (expires - today).days > max_days:
            warnings.append(f"waiver {wid} expires in {(expires-today).days} days (> policy {max_days})")
        active_waivers.setdefault(subject, []).append(wid)

    def waived(subject):
        return subject in active_waivers

    # Evidence sanity.
    for eid, e in evidence.items():
        if not e.get("kind"):
            errors.append(f"evidence {eid} missing kind")
        if not e.get("title"):
            errors.append(f"evidence {eid} missing title")
        if not e.get("independent_group"):
            warnings.append(f"evidence {eid} has no independent_group; independence cannot be assessed")
        if e.get("reproducible") is False:
            warnings.append(f"evidence {eid} is marked non-reproducible")

    # Failure-mode references.
    for fid, f in failures.items():
        invs = f.get("invariants", [])
        evs = f.get("evidence", [])
        if not invs:
            errors.append(f"failure_mode {fid} threatens no invariant")
        if not evs and not waived(fid):
            errors.append(f"failure_mode {fid} has no evidence and no active waiver")
        for iid in invs:
            if iid not in invariants:
                errors.append(f"failure_mode {fid} references missing invariant {iid}")
        for eid in evs:
            if eid not in evidence:
                errors.append(f"failure_mode {fid} references missing evidence {eid}")

    critical_min = int(policy.get("critical_min_independent_evidence", 2))
    high_min = int(policy.get("high_min_independent_evidence", 1))
    require_runtime = bool(policy.get("critical_require_runtime_detection", True))
    require_release = bool(policy.get("critical_require_release_gate", True))

    for iid, inv in invariants.items():
        risk = inv.get("risk", "").lower()
        if risk not in VALID_RISKS:
            errors.append(f"invariant {iid} has invalid risk {risk!r}; expected one of {sorted(VALID_RISKS)}")
        for field in ("title", "statement", "oracle"):
            if not inv.get(field):
                errors.append(f"invariant {iid} missing {field}")

        ev_ids = inv.get("evidence", [])
        groups = set()
        for eid in ev_ids:
            if eid not in evidence:
                errors.append(f"invariant {iid} references missing evidence {eid}")
                continue
            groups.add(evidence[eid].get("independent_group") or f"__ungrouped__:{eid}")

        for fid in inv.get("failure_modes", []):
            if fid not in failures:
                errors.append(f"invariant {iid} references missing failure_mode {fid}")

        minimum = critical_min if risk == "critical" else high_min if risk == "high" else 0
        if len(groups) < minimum and not waived(iid):
            errors.append(
                f"invariant {iid} ({risk}) has {len(groups)} independent evidence group(s); policy requires {minimum}"
            )

        if risk == "critical" and require_runtime:
            control_id = inv.get("runtime_detection")
            if not control_id and not waived(iid):
                errors.append(f"critical invariant {iid} has no runtime_detection and no active waiver")
            elif control_id and control_id not in controls:
                errors.append(f"invariant {iid} references missing runtime control {control_id}")

        if risk == "critical" and require_release and inv.get("release_gate") is not True and not waived(iid):
            errors.append(f"critical invariant {iid} is not marked release_gate=true and has no active waiver")

    # Incident learning loop.
    require_incident_guard = bool(policy.get("incident_require_durable_guard", True))
    all_guards = set(evidence) | set(controls)
    for inc_id, inc in incidents.items():
        guards = inc.get("guards", [])
        if require_incident_guard and not guards and not waived(inc_id):
            errors.append(f"incident {inc_id} has no durable guard and no active waiver")
        for gid in guards:
            if gid not in all_guards:
                errors.append(f"incident {inc_id} references missing guard {gid}")

    report = {
        "manifest": str(path),
        "counts": {
            "invariants": len(invariants),
            "failure_modes": len(failures),
            "evidence": len(evidence),
            "controls": len(controls),
            "incidents": len(incidents),
            "waivers": len(waivers),
        },
        "errors": errors,
        "warnings": warnings,
        "status": "FAIL" if errors else "PASS",
    }

    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print(f"Reliability manifest: {path}")
        print("Counts:", ", ".join(f"{k}={v}" for k, v in report["counts"].items()))
        for w in warnings:
            print(f"WARN: {w}")
        for e in errors:
            print(f"ERROR: {e}")
        print(f"STATUS: {report['status']}")

    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
