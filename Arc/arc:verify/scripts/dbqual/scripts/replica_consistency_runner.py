#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
import uuid
from pathlib import Path

from sqlverify_common import evidence_base, finalize_evidence, json_dump, percentile, require_test_database_name
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore
        from psycopg import sql  # type: ignore
        return psycopg, sql
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def main() -> int:
    ap = argparse.ArgumentParser(description="Measure PostgreSQL replica visibility and monotonic-read behavior")
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config)); case = next((x for x in cfg.get("replica_consistency", []) if x.get("id") == ns.case), None)
    if not case: raise SystemExit(f"replica_consistency not found: {ns.case}")
    start = time.monotonic(); ev = evidence_base("replica_consistency", f"replica.{ns.case}")
    penv = case.get("primary_dsn_env", cfg["database"]["dsn_env"]); renv = case.get("replica_dsn_env")
    pdsn = os.environ.get(penv); rdsn = os.environ.get(renv or "")
    if not pdsn or not rdsn:
        ev["violations"].append(f"missing primary/replica DSN env ({penv}, {renv})"); json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    runs = int(case.get("runs", 5)); poll_ms = int(case.get("poll_ms", 25)); max_visibility_ms = float(case.get("max_visibility_ms", 5000))
    schema = case.get("probe_schema", "sqlverify_probe"); table = case.get("probe_table", "replica_visibility")
    safe_pattern = cfg["database"]["safe_database_pattern"]
    latencies: list[float] = []; monotonic_failures = 0; samples = []
    try:
        psycopg, sql = _psycopg()
        with psycopg.connect(pdsn, autocommit=True) as pconn, psycopg.connect(rdsn, autocommit=True) as rconn:
            with pconn.cursor() as pc, rconn.cursor() as rc:
                pc.execute("select current_database()"); pdb = pc.fetchone()[0]
                rc.execute("select current_database()"); rdb = rc.fetchone()[0]
                require_test_database_name(pdb, safe_pattern, bool(cfg["database"].get("allow_destructive", False)))
                require_test_database_name(rdb, safe_pattern, bool(cfg["database"].get("allow_destructive", False)))
                if case.get("create_probe_table", True):
                    pc.execute(sql.SQL("create schema if not exists {}").format(sql.Identifier(schema)))
                    pc.execute(sql.SQL("create table if not exists {}.{} (token text primary key, created_at timestamptz not null default clock_timestamp())").format(sql.Identifier(schema), sql.Identifier(table)))
                for _ in range(runs):
                    token = str(uuid.uuid4())
                    t0 = time.monotonic()
                    pc.execute(sql.SQL("insert into {}.{}(token) values (%s)").format(sql.Identifier(schema), sql.Identifier(table)), (token,))
                    pc.execute("select pg_current_wal_lsn()::text"); target_lsn = pc.fetchone()[0]
                    visible = False; replay_lsn = None
                    deadline = t0 + max_visibility_ms/1000.0
                    while time.monotonic() < deadline:
                        try:
                            rc.execute(sql.SQL("select count(*) from {}.{} where token=%s").format(sql.Identifier(schema), sql.Identifier(table)), (token,))
                            visible = int(rc.fetchone()[0]) == 1
                            try:
                                rc.execute("select pg_last_wal_replay_lsn()::text")
                                replay_lsn = rc.fetchone()[0]
                            except Exception:
                                replay_lsn = None
                            if visible: break
                        except Exception:
                            visible = False
                        time.sleep(poll_ms/1000.0)
                    elapsed = (time.monotonic()-t0)*1000.0
                    if visible:
                        latencies.append(elapsed)
                        # Once visible, repeated reads should not move backwards for this marker.
                        for _j in range(3):
                            rc.execute(sql.SQL("select count(*) from {}.{} where token=%s").format(sql.Identifier(schema), sql.Identifier(table)), (token,))
                            if int(rc.fetchone()[0]) != 1: monotonic_failures += 1
                    samples.append({"token": token, "visible": visible, "visibility_ms": elapsed, "target_lsn": target_lsn, "replica_replay_lsn": replay_lsn})
                # cleanup on primary; replication of cleanup is intentionally not in the timing sample
                pc.execute(sql.SQL("delete from {}.{} where token = any(%s)").format(sql.Identifier(schema), sql.Identifier(table)), ([s["token"] for s in samples],))
        ev["observed"] = {
            "runs": runs,
            "visible_runs": sum(1 for s in samples if s["visible"]),
            "visibility_ms": {"p50": percentile(latencies, .50), "p95": percentile(latencies, .95), "p99": percentile(latencies, .99), "max": max(latencies) if latencies else None},
            "monotonic_failures": monotonic_failures,
            "samples": samples,
        }
        if len(latencies) != runs: ev["violations"].append(f"only {len(latencies)}/{runs} markers became visible within {max_visibility_ms} ms")
        if latencies and max(latencies) > max_visibility_ms: ev["violations"].append("replica visibility exceeded configured budget")
        if case.get("require_monotonic_reads", True) and monotonic_failures: ev["violations"].append(f"monotonic read failures={monotonic_failures}")
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    ev["reproduction"] = {"primary_dsn_env": penv, "replica_dsn_env": renv, "case": case}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
