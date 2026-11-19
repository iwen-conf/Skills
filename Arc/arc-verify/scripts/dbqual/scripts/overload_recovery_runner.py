#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, os, time
from pathlib import Path
from typing import Any

from pgbench_runner import run_one, safe_db_check
from sqlverify_common import evidence_base, finalize_evidence, json_dump, percentile, redact_dsn, which, command_verdict
from sqlverify_config import load_config


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Run ramp/impulse/overload/recovery pgbench phases and verify graceful recovery"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--profile", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--evidence-dir", required=True)
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    profile = next((x for x in cfg.get("overload_profile", []) if x.get("id") == ns.profile), None)
    if not profile:
        raise SystemExit(f"overload_profile not found: {ns.profile}")
    base = next((w for w in cfg.get("workload", []) if w.get("id") == profile.get("workload")), None)
    if not base:
        raise SystemExit(f"workload not found: {profile.get('workload')}")
    outdir = Path(ns.evidence_dir)
    outdir.mkdir(parents=True, exist_ok=True)
    ev_id = f"overload.{ns.profile}"
    started = time.monotonic()
    ev = evidence_base("overload_recovery", ev_id)
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    ev["reproduction"] = {
        "dsn_env": dsn_env,
        "profile": ns.profile,
        "phases": profile.get("phase", []),
        "workload": profile.get("workload"),
    }
    if dsn:
        ev["reproduction"]["dsn_redacted"] = redact_dsn(dsn)
    if not dsn or not which("pgbench") or not which("psql"):
        ev["violations"].append("DATABASE_URL/pgbench/psql unavailable")
        json_dump(outdir / f"{ev_id}.json", finalize_evidence(ev, started, "BLOCKED"))
        return 2
    safe, db = safe_db_check(dsn, cfg["database"]["safe_database_pattern"])
    if not safe and not (
        cfg["database"].get("allow_destructive", False) and os.environ.get("SQLVERIFY_ALLOW_DESTRUCTIVE") == "1"
    ):
        ev["violations"].append(f"unsafe database target {db!r}")
        json_dump(outdir / f"{ev_id}.json", finalize_evidence(ev, started, "BLOCKED"))
        return 2

    phases = []
    rawdir = outdir / "raw" / ev_id
    rawdir.mkdir(parents=True, exist_ok=True)
    try:
        if not profile.get("phase"):
            raise ValueError("overload profile contains no phases to observe")
        if profile.get("nominal_phase") or profile.get("recovery_phase"):
            if (
                not profile.get("nominal_phase")
                or not profile.get("recovery_phase")
                or any(profile.get(key) is None for key in ("max_recovery_p99_ratio", "min_recovery_tps_ratio"))
            ):
                raise ValueError("recovery comparison requires both named phases and explicit p99/TPS budgets")
        for i, p in enumerate(profile.get("phase", []), 1):
            w = dict(base)
            w["id"] = f"{base['id']}-{p.get('name', i)}"
            w["clients"] = [int(p.get("clients", profile.get("clients", max(base.get("clients", [1])))))]
            w["duration_s"] = int(p.get("duration_s", profile.get("duration_s", 30)))
            w["warmup_s"] = 0
            w["repetitions"] = 1
            if p.get("rate_tps") is None:
                w.pop("rate_tps", None)
            else:
                w["rate_tps"] = float(p["rate_tps"])
            if p.get("latency_limit_ms") is not None:
                w["latency_limit_ms"] = float(p["latency_limit_ms"])
            r = run_one(root, w, w["clients"][0], i, dsn, rawdir)
            parsed = r["parsed"]
            successes = r.get("processed")
            failures = r.get("failed")
            valid_counts = all(type(value) is int and value >= 0 for value in (successes, failures))
            if isinstance(successes, int) and isinstance(failures, int) and valid_counts:
                den = successes + failures
                failure_rate = failures / den if den else None
            else:
                den = 0
                failure_rate = None
            phase = {
                "name": p.get("name", str(i)),
                "kind": p.get("kind", "load"),
                "rate_tps": p.get("rate_tps"),
                "clients": w["clients"][0],
                "tps": r.get("tps"),
                "p95_ms": parsed.get("latency_ms", {}).get("p95"),
                "p99_ms": parsed.get("latency_ms", {}).get("p99"),
                "schedule_lag_p99_ms": (parsed.get("schedule_lag_ms") or {}).get("p99"),
                "failure_rate": failure_rate,
                "failed": failures,
                "processed": successes,
                "retries": r.get("total_retries", 0),
                "returncode": r["returncode"],
                "seed": r["random_seed"],
            }
            phases.append(phase)
            ev["observed"] = {"database": db, "phases": phases, "recovery": None}
            verdict = command_verdict(r["returncode"], profile.get("oracle_failure_returncodes", []))
            phase["verdict"] = verdict
            if verdict == "FAIL":
                ev["violations"].append(
                    f"phase {phase['name']} declared workload assertion failed: rc={r['returncode']}"
                )
            elif verdict == "BLOCKED":
                raise RuntimeError(f"phase {phase['name']} workload result is undeclared: rc={r['returncode']}")
            if phase["failure_rate"] is None:
                raise ValueError(f"phase {phase['name']} has no complete nonempty transaction counts")
            max_fail = p.get("max_failure_rate", profile.get("max_failure_rate"))
            if max_fail is not None and phase["failure_rate"] > float(max_fail):
                ev["violations"].append(f"phase {phase['name']} failure_rate={phase['failure_rate']:.6f} > {max_fail}")
            if p.get("max_p99_ms") is not None and phase["p99_ms"] is None:
                raise ValueError(f"phase {phase['name']} required p99 observation is missing")
            if p.get("max_p99_ms") is not None and phase["p99_ms"] > float(p["max_p99_ms"]):
                ev["violations"].append(f"phase {phase['name']} p99_ms={phase['p99_ms']} > {p['max_p99_ms']}")
        # Recovery gate compares a named recovery phase with named nominal phase.
        nominal_name = profile.get("nominal_phase")
        recovery_name = profile.get("recovery_phase")
        nominal = next((p for p in phases if p["name"] == nominal_name), None)
        recovery = next((p for p in phases if p["name"] == recovery_name), None)
        recovery_eval = None
        if nominal_name and recovery_name:
            if not nominal or not recovery:
                raise ValueError("nominal/recovery phase was not observed")
            else:
                p99_ratio = (
                    (recovery["p99_ms"] / nominal["p99_ms"]) if recovery["p99_ms"] and nominal["p99_ms"] else None
                )
                tps_ratio = (recovery["tps"] / nominal["tps"]) if recovery["tps"] and nominal["tps"] else None
                recovery_eval = {"p99_ratio": p99_ratio, "tps_ratio": tps_ratio}
                if p99_ratio is None or tps_ratio is None:
                    raise ValueError("recovery p99/TPS ratios have no valid observations")
                if p99_ratio > float(profile["max_recovery_p99_ratio"]):
                    ev["violations"].append(
                        f"recovery p99 ratio {p99_ratio} exceeds max {profile['max_recovery_p99_ratio']}"
                    )
                if tps_ratio < float(profile["min_recovery_tps_ratio"]):
                    ev["violations"].append(
                        f"recovery TPS ratio {tps_ratio} below min {profile['min_recovery_tps_ratio']}"
                    )
        ev["observed"] = {"database": db, "phases": phases, "recovery": recovery_eval}
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["warnings"].append(repr(e))
        status = "FAIL" if ev["violations"] else "BLOCKED"
    json_dump(outdir / f"{ev_id}.json", finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
