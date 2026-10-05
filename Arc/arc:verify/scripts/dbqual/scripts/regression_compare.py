#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import time
import random
import statistics
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load, sha256_file


def load_dir(d: Path) -> dict[str, dict[str, Any]]:
    out = {}
    for p in d.glob("*.json"):
        try:
            x = json_load(p)
            if isinstance(x, dict) and x.get("id"):
                out[x["id"]] = x
        except Exception:
            pass
    return out


def verify_baseline_manifest(d: Path) -> dict[str, Any]:
    p = d / "baseline-manifest.json"
    if not p.exists():
        return {"present": False, "ok": None, "errors": ["baseline-manifest.json is absent"]}
    try:
        m = json_load(p)
        errors = []
        for item in m.get("files", []):
            fp = d / item["name"]
            if not fp.exists():
                errors.append(f"missing {item['name']}")
            elif sha256_file(fp) != item.get("sha256"):
                errors.append(f"sha256 mismatch: {item['name']}")
        if m.get("gate_status") != "PASS" and not m.get("override_nonpass"):
            errors.append(f"baseline manifest gate_status is {m.get('gate_status')!r}")
        return {"present": True, "ok": not errors, "errors": errors, "created_at": m.get("created_at"), "git": m.get("git")}
    except Exception as e:
        return {"present": True, "ok": False, "errors": [repr(e)]}


def ratio_delta(cur: Any, base: Any) -> float | None:
    if not isinstance(cur, (int, float)) or not isinstance(base, (int, float)) or base == 0:
        return None
    return (cur - base) / base



def bootstrap_delta_ci(cur: list[float], base: list[float], *, seed: int = 20260920, samples: int = 4000) -> dict[str, float] | None:
    if len(cur) < 3 or len(base) < 3:
        return None
    rng = random.Random(seed)
    vals: list[float] = []
    for _ in range(samples):
        cs = [cur[rng.randrange(len(cur))] for _ in cur]
        bs = [base[rng.randrange(len(base))] for _ in base]
        cm = statistics.median(cs); bm = statistics.median(bs)
        if bm != 0:
            vals.append((cm-bm)/bm)
    if not vals:
        return None
    vals.sort()
    def q(p: float) -> float:
        i = min(len(vals)-1, max(0, round((len(vals)-1)*p)))
        return vals[i]
    return {"low": q(0.025), "median": q(0.5), "high": q(0.975), "samples": len(vals)}

def main() -> int:
    ap = argparse.ArgumentParser(description="Compare SQL verification evidence against a reproducible baseline")
    ap.add_argument("--current-dir", required=True)
    ap.add_argument("--baseline-dir", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--max-p95-regression", type=float, default=0.20)
    ap.add_argument("--max-p99-regression", type=float, default=0.25)
    ap.add_argument("--max-tps-drop", type=float, default=0.15)
    ap.add_argument("--max-explain-time-regression", type=float, default=0.50)
    ap.add_argument("--allow-environment-drift", action="store_true")
    ns = ap.parse_args()
    started = time.monotonic()
    ev = evidence_base("regression_compare", "regression.baseline")
    cur = load_dir(Path(ns.current_dir)); base = load_dir(Path(ns.baseline_dir))
    observed: dict[str, Any] = {"comparisons": [], "environment": {}}
    manifest_check = verify_baseline_manifest(Path(ns.baseline_dir))
    observed["baseline_manifest"] = manifest_check
    if manifest_check["present"] and not manifest_check["ok"]:
        ev["violations"].append("baseline manifest integrity check failed: " + "; ".join(manifest_check["errors"]))
    elif not manifest_check["present"]:
        ev["warnings"].append("baseline-manifest.json is absent; baseline provenance/integrity is not cryptographically checked")

    cenv = cur.get("postgres.snapshot.pre") or cur.get("postgres.snapshot")
    benv = base.get("postgres.snapshot.pre") or base.get("postgres.snapshot")
    if cenv and benv:
        co, bo = cenv.get("observed", {}), benv.get("observed", {})
        same_major = int(co.get("server_version_num", 0)) // 10000 == int(bo.get("server_version_num", 0)) // 10000
        same_settings = co.get("settings_sha256") == bo.get("settings_sha256")
        same_schema = co.get("schema_fingerprint_sha256") == bo.get("schema_fingerprint_sha256")
        same_machine = (co.get("host") or {}).get("machine") == (bo.get("host") or {}).get("machine")
        observed["environment"] = {
            "same_postgres_major": same_major, "same_settings_hash": same_settings, "same_schema_hash": same_schema,
            "same_machine_arch": same_machine, "current_version": co.get("server_version_num"), "baseline_version": bo.get("server_version_num")
        }
        if not ns.allow_environment_drift and (not same_major or not same_settings or not same_schema):
            ev["violations"].append("baseline environment is not comparable: PostgreSQL major/settings/schema differ; use a matching baseline or explicitly allow drift")


    cd = cur.get("dataset.audit"); bd = base.get("dataset.audit")
    if cd and bd:
        cdo, bdo = cd.get("observed", {}), bd.get("observed", {})
        observed["dataset_comparable"] = (cdo.get("profile") == bdo.get("profile") and cdo.get("seed") == bdo.get("seed"))
        if not ns.allow_environment_drift and not observed["dataset_comparable"]:
            ev["violations"].append("baseline dataset profile/seed differs; performance regression is not directly comparable")

    for eid, ce in sorted(cur.items()):
        be = base.get(eid)
        if not be:
            continue
        if ce.get("kind") == "pgbench" and be.get("kind") == "pgbench":
            cr, br = ce.get("reproduction", {}) or {}, be.get("reproduction", {}) or {}
            same_workload = cr.get("workload_config") == br.get("workload_config") and cr.get("scripts") == br.get("scripts")
            if not same_workload and not ns.allow_environment_drift:
                ev["violations"].append(f"{eid}: workload definition/script hashes differ from baseline; performance numbers are not directly comparable")
            ch, bh = cr.get("host") or {}, br.get("host") or {}
            if ch and bh and (ch.get("machine") != bh.get("machine") or ch.get("cpu_count") != bh.get("cpu_count")):
                ev["warnings"].append(f"{eid}: benchmark host architecture/CPU-count differs from baseline; interpret performance deltas cautiously")
            co, bo = ce.get("observed", {}), be.get("observed", {})
            cp95 = (co.get("latency_ms") or {}).get("p95"); bp95 = (bo.get("latency_ms") or {}).get("p95")
            cp99 = (co.get("latency_ms") or {}).get("p99"); bp99 = (bo.get("latency_ms") or {}).get("p99")
            ctps, btps = co.get("tps"), bo.get("tps")
            d95, d99, dtps = ratio_delta(cp95, bp95), ratio_delta(cp99, bp99), ratio_delta(ctps, btps)
            cp95_runs=[float(x) for x in (co.get("latency_ms") or {}).get("p95_runs",[]) if isinstance(x,(int,float))]
            bp95_runs=[float(x) for x in (bo.get("latency_ms") or {}).get("p95_runs",[]) if isinstance(x,(int,float))]
            cp99_runs=[float(x) for x in (co.get("latency_ms") or {}).get("p99_runs",[]) if isinstance(x,(int,float))]
            bp99_runs=[float(x) for x in (bo.get("latency_ms") or {}).get("p99_runs",[]) if isinstance(x,(int,float))]
            ctps_runs=[float(x) for x in co.get("tps_runs",[]) if isinstance(x,(int,float))]
            btps_runs=[float(x) for x in bo.get("tps_runs",[]) if isinstance(x,(int,float))]
            ci95=bootstrap_delta_ci(cp95_runs,bp95_runs,seed=20260920)
            ci99=bootstrap_delta_ci(cp99_runs,bp99_runs,seed=20260921)
            citps=bootstrap_delta_ci(ctps_runs,btps_runs,seed=20260922)
            item = {"id": eid, "kind": "pgbench", "p95_delta": d95, "p99_delta": d99, "tps_delta": dtps, "p95_delta_ci95":ci95, "p99_delta_ci95":ci99, "tps_delta_ci95":citps, "current": {"p95": cp95, "p99": cp99, "tps": ctps}, "baseline": {"p95": bp95, "p99": bp99, "tps": btps}}
            observed["comparisons"].append(item)
            if ci95 and ci95["low"] > ns.max_p95_regression:
                ev["violations"].append(f"{eid}: P95 regression CI [{ci95['low']:.1%},{ci95['high']:.1%}] exceeds {ns.max_p95_regression:.1%}")
            elif d95 is not None and d95 > ns.max_p95_regression:
                ev["warnings"].append(f"{eid}: P95 point regression {d95:.1%} exceeds threshold but bootstrap CI is inconclusive")
            if ci99 and ci99["low"] > ns.max_p99_regression:
                ev["violations"].append(f"{eid}: P99 regression CI [{ci99['low']:.1%},{ci99['high']:.1%}] exceeds {ns.max_p99_regression:.1%}")
            elif d99 is not None and d99 > ns.max_p99_regression:
                ev["warnings"].append(f"{eid}: P99 point regression {d99:.1%} exceeds threshold but bootstrap CI is inconclusive")
            if citps and citps["high"] < -ns.max_tps_drop:
                ev["violations"].append(f"{eid}: TPS drop CI [{citps['low']:.1%},{citps['high']:.1%}] worse than {-ns.max_tps_drop:.1%}")
            elif dtps is not None and dtps < -ns.max_tps_drop:
                ev["warnings"].append(f"{eid}: TPS point drop {-dtps:.1%} exceeds threshold but bootstrap CI is inconclusive")
        elif ce.get("kind") == "explain" and be.get("kind") == "explain":
            ca = (ce.get("observed", {}).get("analysis") or {}); ba = (be.get("observed", {}).get("analysis") or {})
            de = ratio_delta(ca.get("execution_time_ms"), ba.get("execution_time_ms"))
            added = sorted(set((ca.get("node_types") or {}).keys()) - set((ba.get("node_types") or {}).keys()))
            item = {"id": eid, "kind": "explain", "execution_time_delta": de, "node_types_added": added}
            observed["comparisons"].append(item)
            # EXPLAIN timing is noisy and instrumented; only a large configured drift is blocking.
            if de is not None and de > ns.max_explain_time_regression:
                ev["violations"].append(f"{eid}: EXPLAIN execution-time regression {de:.1%} > {ns.max_explain_time_regression:.1%}; confirm with workload latency before tuning")

    ev["observed"] = observed
    status = "PASS" if not ev["violations"] else "FAIL"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
