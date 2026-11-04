#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name, scalar_value
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore
        from psycopg.rows import dict_row  # type: ignore

        return psycopg, dict_row
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Audit PostgreSQL logical-replication apply state, conflicts and freshness"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    case = next((x for x in cfg.get("logical_replication_audit", []) if x.get("id") == ns.case), None)
    if not case:
        raise SystemExit(f"logical_replication_audit not found: {ns.case}")
    start = time.monotonic()
    ev = evidence_base("logical_replication", f"logical-replication.{ns.case}")
    envname = case.get("subscriber_dsn_env")
    dsn = os.environ.get(str(envname or ""))
    if not dsn:
        ev["violations"].append(f"missing env {envname}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    obs: dict[str, Any] = {}
    try:
        psycopg, dict_row = _psycopg()
        with psycopg.connect(dsn, autocommit=True, row_factory=dict_row) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "select current_database() as db, current_setting('server_version_num')::int as version_num"
                )
                meta = cur.fetchone()
                require_test_database_name(
                    meta["db"],
                    case.get("safe_database_pattern", cfg["database"]["safe_database_pattern"]),
                    bool(case.get("allow_destructive", False)),
                )
                obs.update(meta)
                cur.execute(
                    "select subid, subname, worker_type, pid, relid, received_lsn, latest_end_lsn, last_msg_send_time, last_msg_receipt_time, latest_end_time from pg_stat_subscription order by subname, worker_type, pid"
                )
                workers = cur.fetchall()
                obs["workers"] = workers
                cur.execute("select * from pg_stat_subscription_stats order by subname")
                stats = cur.fetchall()
                if not stats:
                    raise RuntimeError("subscription stats contained no observed subscription")
                obs["subscription_stats"] = stats
                if any(type(r.get("apply_error_count")) is not int or r["apply_error_count"] < 0 for r in stats):
                    raise ValueError("subscription apply_error_count is missing or invalid")
                apply_errors = sum(r["apply_error_count"] for r in stats)
                conflict_total = 0
                for r in stats:
                    for k, v in r.items():
                        if str(k).startswith("confl_") and isinstance(v, (int, float)):
                            conflict_total += int(v or 0)
                obs["apply_error_count"] = apply_errors
                obs["conflict_count"] = conflict_total
                min_workers = int(case.get("min_active_workers", 1))
                active = sum(1 for r in workers if r.get("pid"))
                obs["active_workers"] = active
                if active < min_workers:
                    ev["violations"].append(f"active subscription workers {active} < {min_workers}")
                if int(case.get("max_apply_errors", 0)) < apply_errors:
                    ev["violations"].append(f"apply_error_count {apply_errors} > {case.get('max_apply_errors', 0)}")
                if int(case.get("max_conflicts", 0)) < conflict_total:
                    ev["violations"].append(f"conflict_count {conflict_total} > {case.get('max_conflicts', 0)}")
                # Freshness based on latest reported apply/end time. This is not byte lag; it is an observable time-age signal.
                ages = []
                for r in workers:
                    t = r.get("latest_end_time") or r.get("last_msg_receipt_time")
                    if t is not None:
                        cur.execute(
                            "select extract(epoch from (clock_timestamp() - %s::timestamptz))*1000 as apply_age_ms",
                            (t,),
                        )
                        age = scalar_value(cur)
                        if age is None or isinstance(age, bool) or not math.isfinite(float(age)):
                            raise ValueError("apply age observation is missing or invalid")
                        ages.append(float(age))
                obs["max_apply_age_ms"] = max(ages, default=None)
                if case.get("max_apply_age_ms") is not None and not ages:
                    raise RuntimeError("required apply age observation is missing")
                if (
                    case.get("max_apply_age_ms") is not None
                    and obs["max_apply_age_ms"] is not None
                    and obs["max_apply_age_ms"] > float(case["max_apply_age_ms"])
                ):
                    ev["violations"].append(
                        f"max_apply_age_ms {obs['max_apply_age_ms']:.3f} > {case['max_apply_age_ms']}"
                    )
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["warnings"].append(repr(e))
        status = "FAIL" if ev["violations"] else "BLOCKED"
    ev["observed"] = obs
    ev["reproduction"] = {"subscriber_dsn_env": envname, "case": case}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
