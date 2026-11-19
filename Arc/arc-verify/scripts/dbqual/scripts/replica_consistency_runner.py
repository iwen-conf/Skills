#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import os
import time
import uuid
from pathlib import Path

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    percentile,
    require_test_database_name,
    scalar_value,
)
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
    cfg = load_config(Path(ns.config))
    case = next((x for x in cfg.get("replica_consistency", []) if x.get("id") == ns.case), None)
    if not case:
        raise SystemExit(f"replica_consistency not found: {ns.case}")
    start = time.monotonic()
    ev = evidence_base("replica_consistency", f"replica.{ns.case}")
    penv = case.get("primary_dsn_env", cfg["database"]["dsn_env"])
    renv = case.get("replica_dsn_env")
    pdsn = os.environ.get(penv)
    rdsn = os.environ.get(renv or "")
    if not pdsn or not rdsn:
        ev["violations"].append(f"missing primary/replica DSN env ({penv}, {renv})")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    schema = case.get("probe_schema", "sqlverify_probe")
    table = case.get("probe_table", "replica_visibility")
    safe_pattern = cfg["database"]["safe_database_pattern"]
    latencies: list[float] = []
    monotonic_failures = 0
    samples: list[dict[str, object]] = []
    blockers: list[str] = []
    ev["observed"] = {"samples": samples, "blockers": blockers}
    ev["reproduction"] = {"primary_dsn_env": penv, "replica_dsn_env": renv, "case": case}
    try:
        runs = int(case.get("runs", 5))
        poll_ms = float(case.get("poll_ms", 25))
        max_visibility_ms = float(case["max_visibility_ms"])
        if runs <= 0 or not all(math.isfinite(value) and value > 0 for value in (poll_ms, max_visibility_ms)):
            raise ValueError("positive runs, polling interval and an explicit finite visibility budget are required")
        ev["observed"]["runs"] = runs
        psycopg, sql = _psycopg()
        with psycopg.connect(pdsn, autocommit=True) as pconn, psycopg.connect(rdsn, autocommit=True) as rconn:
            with pconn.cursor() as pc, rconn.cursor() as rc:
                pc.execute("select current_database()")
                pdb = pc.fetchone()[0]
                rc.execute("select current_database()")
                rdb = rc.fetchone()[0]
                require_test_database_name(pdb, safe_pattern, bool(cfg["database"].get("allow_destructive", False)))
                require_test_database_name(rdb, safe_pattern, bool(cfg["database"].get("allow_destructive", False)))
                if case.get("create_probe_table", True):
                    pc.execute(sql.SQL("create schema if not exists {}").format(sql.Identifier(schema)))
                    pc.execute(
                        sql.SQL(
                            "create table if not exists {}.{} (token text primary key, created_at timestamptz not null default clock_timestamp())"
                        ).format(sql.Identifier(schema), sql.Identifier(table))
                    )
                try:
                    for _ in range(runs):
                        token = str(uuid.uuid4())
                        t0 = time.monotonic()
                        monotonic_reads: list[int] = []
                        sample: dict[str, object] = {
                            "token": token,
                            "visible": None,
                            "monotonic_reads": monotonic_reads,
                        }
                        samples.append(sample)
                        pc.execute(
                            sql.SQL("insert into {}.{}(token) values (%s)").format(
                                sql.Identifier(schema), sql.Identifier(table)
                            ),
                            (token,),
                        )
                        pc.execute("select pg_current_wal_lsn()::text")
                        sample["target_lsn"] = scalar_value(pc)
                        deadline = t0 + max_visibility_ms / 1000.0
                        while time.monotonic() < deadline:
                            rc.execute(
                                sql.SQL("select count(*) from {}.{} where token=%s").format(
                                    sql.Identifier(schema), sql.Identifier(table)
                                ),
                                (token,),
                            )
                            count = scalar_value(rc)
                            sample["count"] = count
                            if type(count) is not int or count not in (0, 1):
                                raise ValueError("replica probe count is not a valid observation")
                            sample["visible"] = count == 1
                            try:
                                rc.execute("select pg_last_wal_replay_lsn()::text")
                                sample["replica_replay_lsn"] = scalar_value(rc)
                            except Exception as e:
                                sample["replica_replay_lsn"] = None
                                ev["warnings"].append(f"optional replay LSN unavailable: {e!r}")
                            if sample["visible"]:
                                break
                            time.sleep(max(0, min(poll_ms / 1000.0, deadline - time.monotonic())))
                        elapsed = (time.monotonic() - t0) * 1000.0
                        sample["visibility_ms"] = elapsed
                        if sample["visible"] is None:
                            raise ValueError("visibility deadline expired before any valid observation")
                        if not sample["visible"]:
                            ev["violations"].append(f"marker {token} not visible within {max_visibility_ms} ms")
                            continue
                        latencies.append(elapsed)
                        if elapsed > max_visibility_ms:
                            ev["violations"].append("replica visibility exceeded configured budget")
                        for _j in range(3):
                            rc.execute(
                                sql.SQL("select count(*) from {}.{} where token=%s").format(
                                    sql.Identifier(schema), sql.Identifier(table)
                                ),
                                (token,),
                            )
                            count = scalar_value(rc)
                            monotonic_reads.append(count)
                            if type(count) is not int or count not in (0, 1):
                                raise ValueError("monotonic probe count is not a valid observation")
                            if count != 1:
                                monotonic_failures += 1
                                if case.get("require_monotonic_reads", True):
                                    ev["violations"].append(f"marker {token}: monotonic read lost visibility")
                finally:
                    if samples:
                        try:
                            pc.execute(
                                sql.SQL("delete from {}.{} where token = any(%s)").format(
                                    sql.Identifier(schema), sql.Identifier(table)
                                ),
                                ([s["token"] for s in samples],),
                            )
                        except Exception as e:
                            blockers.append(f"probe cleanup failed: {e!r}; fixture reconstruction is required")
    except Exception as e:
        blockers.append(repr(e))
    ev["observed"].update(
        {
            "visible_runs": sum(1 for s in samples if s["visible"] is True),
            "visibility_ms": {
                "p50": percentile(latencies, 0.50),
                "p95": percentile(latencies, 0.95),
                "p99": percentile(latencies, 0.99),
                "max": max(latencies) if latencies else None,
            },
            "monotonic_failures": monotonic_failures,
        }
    )
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
