#!/usr/bin/env python3
from __future__ import annotations

import argparse
import glob
import json
import os
import re
import statistics
import time
from pathlib import Path
from typing import Any

from pgbench_log import parse_files
from sqlverify_common import evidence_base, finalize_evidence, json_dump, median, redact_dsn, run, which, host_metadata, sha256_file
from sqlverify_config import load_config



def capture_pgss(dsn: str, include_query_text: bool = False) -> dict[str, Any] | None:
    try:
        import psycopg  # type: ignore
    except ImportError:
        return None
    try:
        with psycopg.connect(dsn, autocommit=True) as conn:
            with conn.cursor() as cur:
                cur.execute("select exists(select 1 from pg_extension where extname='pg_stat_statements')")
                if not cur.fetchone()[0]:
                    return None
                cols = "queryid::text, calls, total_exec_time, rows, shared_blks_hit, shared_blks_read, temp_blks_read, temp_blks_written, wal_bytes"
                if include_query_text:
                    cols += ", query"
                cur.execute(f"select {cols} from pg_stat_statements where dbid=(select oid from pg_database where datname=current_database())")
                names = [d.name for d in cur.description]
                return {str(r[0]): dict(zip(names, r)) for r in cur.fetchall()}
    except Exception:
        return None


def diff_pgss(before: dict[str, Any] | None, after: dict[str, Any] | None) -> list[dict[str, Any]] | None:
    if before is None or after is None:
        return None
    rows = []
    numeric = ["calls", "total_exec_time", "rows", "shared_blks_hit", "shared_blks_read", "temp_blks_read", "temp_blks_written", "wal_bytes"]
    for qid, a in after.items():
        b = before.get(qid, {})
        row = {"queryid": qid}
        if "query" in a:
            row["query"] = a.get("query")
        changed = False
        for k in numeric:
            av = a.get(k, 0) or 0; bv = b.get(k, 0) or 0
            try:
                dv = float(av) - float(bv)
            except Exception:
                dv = 0.0
            row[k] = dv
            changed = changed or abs(dv) > 0
        if changed:
            calls = row.get("calls", 0)
            row["mean_exec_time_ms_delta"] = (row["total_exec_time"] / calls) if calls else None
            rows.append(row)
    rows.sort(key=lambda x: x.get("total_exec_time", 0), reverse=True)
    return rows

TPS_RE = re.compile(r"^tps\s*=\s*([0-9.]+)", re.M)
LAT_RE = re.compile(r"^latency average\s*=\s*([0-9.]+)\s*ms", re.M)
FAIL_RE = re.compile(r"^number of failed transactions:\s*([0-9]+)", re.M)
RETRIED_RE = re.compile(r"^number of transactions retried:\s*([0-9]+)", re.M)
RETRIES_RE = re.compile(r"^total number of retries:\s*([0-9]+)", re.M)
SKIPPED_RE = re.compile(r"^number of transactions skipped:\s*([0-9]+)", re.M)
TX_RE = re.compile(r"^number of transactions actually processed:\s*([0-9]+)(?:/([0-9]+))?", re.M)


def safe_db_check(dsn: str, safe_pattern: str) -> tuple[bool, str]:
    cp = run(["psql", "-X", "-At", "-c", "select current_database()"], env={"PGDATABASE": dsn}, timeout=10)
    if cp.returncode != 0:
        return False, cp.stderr.strip()
    db = cp.stdout.strip().splitlines()[0] if cp.stdout.strip() else ""
    return bool(re.search(safe_pattern, db)), db


def run_one(root: Path, workload: dict[str, Any], clients: int, rep: int, dsn: str, out_dir: Path) -> dict[str, Any]:
    wid = workload["id"]
    rid = f"{wid}.c{clients}.r{rep}"
    run_dir = out_dir / rid
    run_dir.mkdir(parents=True, exist_ok=True)
    prefix = f"pgbench_{rid.replace('.', '_')}"
    base_seed = int(workload.get("random_seed", 1))
    run_seed = base_seed + int(rep)
    cmd = [
        "pgbench", "-n", "-c", str(clients), "-j", str(min(int(workload.get("threads", clients)), clients)),
        "-T", str(int(workload["duration_s"])), "-M", workload.get("protocol", "prepared"),
        "-r", "-P", str(int(workload.get("progress_s", 10))), "-l", f"--log-prefix={prefix}",
        "--failures-detailed", "--verbose-errors", f"--random-seed={run_seed}",
    ]
    if workload.get("rate_tps"):
        cmd += ["--rate", str(workload["rate_tps"])]
    if workload.get("latency_limit_ms"):
        cmd += ["--latency-limit", str(workload["latency_limit_ms"])]
    if workload.get("max_tries") is not None:
        cmd += ["--max-tries", str(workload["max_tries"])]
    sampling = workload.get("log_sampling_rate")
    if sampling is not None:
        cmd += ["--sampling-rate", str(sampling)]
    for s in workload["scripts"]:
        cmd += ["-f", f"{(root / s['path']).resolve()}@{int(s.get('weight',1))}"]
    env = {"PGDATABASE": dsn}
    # pgbench names logs by pid, so logs of an earlier run in the same directory survive; parse only this run's.
    earlier_logs = set(glob.glob(str(run_dir / f"{prefix}.*")))
    started = time.monotonic()
    cp = run(cmd, cwd=run_dir, env=env, timeout=int(workload["duration_s"]) + int(workload.get("command_timeout_extra_s", 120)))
    log_paths = [Path(p) for p in glob.glob(str(run_dir / f"{prefix}.*")) if p not in earlier_logs]
    parsed = parse_files(log_paths, has_rate=bool(workload.get("rate_tps")), has_retries=int(workload.get("max_tries", 1)) != 1)
    tx_match = TX_RE.search(cp.stdout)
    failed_match = FAIL_RE.search(cp.stdout)
    retried_match = RETRIED_RE.search(cp.stdout)
    retries_match = RETRIES_RE.search(cp.stdout)
    skipped_match = SKIPPED_RE.search(cp.stdout)
    tps_match = TPS_RE.search(cp.stdout)
    avg_match = LAT_RE.search(cp.stdout)
    processed = int(tx_match.group(1)) if tx_match else None
    intended = int(tx_match.group(2)) if tx_match and tx_match.group(2) else None
    failed = int(failed_match.group(1)) if failed_match else None
    retried = int(retried_match.group(1)) if retried_match else 0
    retries = int(retries_match.group(1)) if retries_match else 0
    skipped = int(skipped_match.group(1)) if skipped_match else None
    return {
        "id": rid,
        "returncode": cp.returncode,
        "random_seed": run_seed,
        "command": cmd,
        "elapsed_ms": round((time.monotonic() - started) * 1000, 3),
        "tps": float(tps_match.group(1)) if tps_match else None,
        "latency_average_ms": float(avg_match.group(1)) if avg_match else None,
        "processed": processed,
        "intended": intended,
        "failed": failed,
        "retried_transactions": retried,
        "total_retries": retries,
        "skipped": skipped,
        "stdout": cp.stdout,
        "stderr": cp.stderr,
        "log_files": [p.name for p in log_paths],
        "parsed": parsed,
    }


def summarize(runs: list[dict[str, Any]], clients: int, workload: dict[str, Any]) -> dict[str, Any]:
    good = [r for r in runs if r["returncode"] == 0]
    tps = [r["tps"] for r in good if isinstance(r.get("tps"), (int, float))]
    p50s = [r["parsed"]["latency_ms"]["p50"] for r in good if r["parsed"]["latency_ms"]["p50"] is not None]
    p95s = [r["parsed"]["latency_ms"]["p95"] for r in good if r["parsed"]["latency_ms"]["p95"] is not None]
    p99s = [r["parsed"]["latency_ms"]["p99"] for r in good if r["parsed"]["latency_ms"]["p99"] is not None]
    maxs = [r["parsed"]["latency_ms"]["max"] for r in good if r["parsed"]["latency_ms"]["max"] is not None]
    lag95s = [r["parsed"]["schedule_lag_ms"]["p95"] for r in good if r["parsed"].get("schedule_lag_ms") and r["parsed"]["schedule_lag_ms"]["p95"] is not None]
    lag99s = [r["parsed"]["schedule_lag_ms"]["p99"] for r in good if r["parsed"].get("schedule_lag_ms") and r["parsed"]["schedule_lag_ms"]["p99"] is not None]
    sched95s = [r["parsed"]["scheduled_latency_ms"]["p95"] for r in good if r["parsed"].get("scheduled_latency_ms") and r["parsed"]["scheduled_latency_ms"]["p95"] is not None]
    sched99s = [r["parsed"]["scheduled_latency_ms"]["p99"] for r in good if r["parsed"].get("scheduled_latency_ms") and r["parsed"]["scheduled_latency_ms"]["p99"] is not None]
    success_total = sum(int(r.get("processed") or 0) for r in good)
    failed_total = sum(int(r.get("failed") or 0) for r in good)
    exact_denominator = success_total + failed_total
    retried_total = sum(int(r.get("retried_transactions") or 0) for r in good)
    retries_total = sum(int(r.get("total_retries") or 0) for r in good)
    skipped_values = [r.get("skipped") for r in good if isinstance(r.get("skipped"), int)]
    sampled_failures: dict[str, int] = {}
    for r in good:
        for k, v in (r["parsed"].get("failures") or {}).items():
            sampled_failures[k] = sampled_failures.get(k, 0) + int(v)
    tps_cv = None
    if len(tps) >= 2 and statistics.mean(tps) != 0:
        tps_cv = statistics.pstdev(tps) / statistics.mean(tps)
    return {
        "clients": clients,
        "repetitions": len(runs),
        "successful_repetitions": len(good),
        "tps": median(tps),
        "tps_runs": tps,
        "tps_cv": tps_cv,
        "latency_ms": {
            "p50": median(p50s), "p95": median(p95s), "p99": median(p99s), "max": max(maxs) if maxs else None,
            "p50_runs": p50s, "p95_runs": p95s, "p99_runs": p99s,
        },
        "schedule_lag_ms": {"p95": median(lag95s), "p99": median(lag99s)} if lag95s or lag99s else None,
        "scheduled_latency_ms": {"p95": median(sched95s), "p99": median(sched99s)} if sched95s or sched99s else None,
        "successful_transactions": success_total,
        "failed_transactions": failed_total,
        "failure_rate": (failed_total / exact_denominator) if exact_denominator else 0.0,
        "retried_transactions": retried_total,
        "total_retries": retries_total,
        "skipped_transactions": sum(skipped_values) if skipped_values else None,
        "sampled_failure_breakdown": sampled_failures,
        "log_sampling_rate": workload.get("log_sampling_rate", 1.0),
        "run_ids": [r["id"] for r in runs],
        "aggregation": "median_of_per-repetition_latency_percentiles_and_tps; exact failure/retry counts from pgbench main reports",
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Run reproducible pgbench business workload matrix")
    ap.add_argument("--config", required=True)
    ap.add_argument("--workload", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--evidence-dir", required=True)
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    workload = next((w for w in cfg.get("workload", []) if w.get("id") == ns.workload), None)
    if not workload:
        raise SystemExit(f"workload not found: {ns.workload}")
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    evidence_dir = Path(ns.evidence_dir)
    evidence_dir.mkdir(parents=True, exist_ok=True)

    preflight_error = None
    db_or_err = None
    if not dsn:
        preflight_error = f"missing env {dsn_env}"
    elif not which("pgbench") or not which("psql"):
        preflight_error = "pgbench and psql are required"
    else:
        safe, db_or_err = safe_db_check(dsn, cfg["database"]["safe_database_pattern"])
        if not safe and not (cfg["database"].get("allow_destructive", False) and os.environ.get("SQLVERIFY_ALLOW_DESTRUCTIVE") == "1"):
            preflight_error = f"refusing benchmark database {db_or_err!r}: does not match safe pattern"

    if preflight_error:
        for clients in workload["clients"]:
            ev_id = f"bench.{workload['id']}.c{int(clients)}"
            ev = evidence_base("pgbench", ev_id)
            ev["violations"].append(preflight_error)
            ev["reproduction"] = {"dsn_env": dsn_env, "workload": workload["id"], "clients": int(clients)}
            json_dump(evidence_dir / f"{ev_id}.json", finalize_evidence(ev, time.monotonic(), "BLOCKED"))
        print(json.dumps({"status":"BLOCKED","error":preflight_error}, ensure_ascii=False))
        return 2

    raw_dir = evidence_dir / "raw" / workload["id"]
    raw_dir.mkdir(parents=True, exist_ok=True)
    overall_rc = 0
    for clients in workload["clients"]:
        ev_id = f"bench.{workload['id']}.c{int(clients)}"
        started = time.monotonic()
        ev = evidence_base("pgbench", ev_id)
        ev["reproduction"] = {
            "dsn_env": dsn_env, "dsn_redacted": redact_dsn(dsn), "workload": workload["id"],
            "clients": int(clients), "random_seed": int(workload.get("random_seed", 1)),
            "scripts": [{"path": s["path"], "weight": int(s.get("weight", 1)), "sha256": sha256_file(root / s["path"])} for s in workload["scripts"]],
            "workload_config": {k:v for k,v in workload.items() if k not in {"scripts"}},
            "host": host_metadata(),
        }
        runs = []
        try:
            warmup = int(workload.get("warmup_s", 0))
            if warmup > 0:
                warm = dict(workload)
                warm["duration_s"] = warmup
                warm["log_sampling_rate"] = 0.01
                warm_result = run_one(root, warm, int(clients), 0, dsn, raw_dir / "warmup")
                if warm_result["returncode"] != 0:
                    raise RuntimeError(f"warmup failed: {warm_result['stderr'][-1000:]}")
            pgss_before = capture_pgss(dsn, bool(workload.get("capture_query_text", False))) if workload.get("capture_pg_stat_statements", True) else None
            if workload.get("require_pg_stat_statements", False) and pgss_before is None:
                raise RuntimeError("pg_stat_statements is required by workload but extension/statistics are unavailable")
            for rep in range(1, int(workload["repetitions"]) + 1):
                r = run_one(root, workload, int(clients), rep, dsn, raw_dir)
                runs.append(r)
            ev["observed"] = summarize(runs, int(clients), workload)
            ev["observed"]["runs"] = [{k:v for k,v in r.items() if k not in {"stdout", "stderr"}} for r in runs]
            ev["observed"]["database"] = db_or_err
            pgss_after = capture_pgss(dsn, bool(workload.get("capture_query_text", False))) if workload.get("capture_pg_stat_statements", True) else None
            ev["observed"]["pg_stat_statements_delta"] = diff_pgss(pgss_before, pgss_after)
            if workload.get("capture_pg_stat_statements", True) and ev["observed"]["pg_stat_statements_delta"] is None:
                ev["warnings"].append("pg_stat_statements delta unavailable; enable extension for per-query total-time/IO/WAL attribution")
            for r in runs:
                if r["returncode"] != 0:
                    ev["violations"].append(f"{r['id']} pgbench exit {r['returncode']}: {r['stderr'][-1000:]}")
            if ev["observed"]["successful_repetitions"] < int(workload["repetitions"]):
                ev["violations"].append("not all repetitions completed successfully")
            status = "PASS" if not ev["violations"] else "FAIL"
        except Exception as e:
            ev["violations"].append(repr(e))
            status = "BLOCKED"
        json_dump(evidence_dir / f"{ev_id}.json", finalize_evidence(ev, started, status))
        if status != "PASS": overall_rc = 2
    return overall_rc


if __name__ == "__main__":
    raise SystemExit(main())
