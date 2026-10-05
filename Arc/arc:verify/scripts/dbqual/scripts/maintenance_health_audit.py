#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore
        from psycopg.rows import dict_row  # type: ignore
        return psycopg, dict_row
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _ratio(a: Any, b: Any) -> float:
    a = float(a or 0); b = float(b or 0)
    return a / max(1.0, a + b)


def evaluate_health(obs: dict[str, Any], policy: dict[str, Any]) -> list[str]:
    v: list[str] = []
    scalar_rules = [
        ("connections_utilization", "max_connections_utilization", "connections utilization"),
        ("oldest_transaction_s", "max_oldest_transaction_s", "oldest transaction seconds"),
        ("oldest_idle_in_transaction_s", "max_idle_in_transaction_s", "oldest idle-in-transaction seconds"),
        ("database_xid_age", "max_database_xid_age", "database XID age"),
        ("max_table_xid_age", "max_table_xid_age", "table XID age"),
        ("max_dead_tuple_ratio", "max_dead_tuple_ratio", "dead tuple ratio"),
        ("max_inactive_slot_retained_bytes", "max_inactive_slot_retained_bytes", "inactive slot retained WAL bytes"),
        ("max_mod_since_analyze_ratio", "max_mod_since_analyze_ratio", "modified-since-analyze ratio"),
        ("oldest_statistics_age_s", "max_statistics_age_s", "statistics age seconds"),
        ("max_sequence_utilization", "max_sequence_utilization", "sequence utilization"),
    ]
    for key, pkey, label in scalar_rules:
        if policy.get(pkey) is not None and obs.get(key) is not None and float(obs[key]) > float(policy[pkey]):
            v.append(f"{label} {obs[key]} > {policy[pkey]}")
    zero_rules = [
        ("invalid_indexes", "require_no_invalid_indexes"),
        ("unvalidated_constraints", "require_no_unvalidated_constraints"),
        ("prepared_transactions", "require_no_prepared_transactions"),
        ("lost_replication_slots", "require_no_lost_replication_slots"),
        ("never_analyzed_tables", "require_no_never_analyzed_tables"),
    ]
    for key, pkey in zero_rules:
        if policy.get(pkey, False) and int(obs.get(key, 0) or 0) != 0:
            v.append(f"{key}={obs.get(key)} but {pkey}=true")
    return v


def main() -> int:
    ap = argparse.ArgumentParser(description="Audit PostgreSQL steady-state maintenance and operability hazards")
    ap.add_argument("--config", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config)); policy = cfg.get("maintenance_audit", {})
    start = time.monotonic(); ev = evidence_base("maintenance_health", "maintenance.health")
    dsn_env = cfg["database"]["dsn_env"]; dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}"); json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    if not policy.get("enabled", True):
        ev["warnings"].append("maintenance audit disabled by config"); json_dump(Path(ns.out), finalize_evidence(ev, start, "NOT_RUN")); return 0
    try:
        psycopg, dict_row = _psycopg()
        with psycopg.connect(dsn, autocommit=True, row_factory=dict_row) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database() as db, current_setting('max_connections')::int as max_connections")
                meta = cur.fetchone(); dbname = meta["db"]
                require_test_database_name(dbname, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False)))
                cur.execute("select count(*)::int as n from pg_stat_activity where datname=current_database()")
                conn_count = cur.fetchone()["n"]
                cur.execute("""
                    select
                      coalesce(max(extract(epoch from (clock_timestamp()-xact_start))) filter (where xact_start is not null),0)::float8 as oldest_xact_s,
                      coalesce(max(extract(epoch from (clock_timestamp()-xact_start))) filter (where state='idle in transaction' and xact_start is not null),0)::float8 as oldest_idle_s
                    from pg_stat_activity where datname=current_database()
                """)
                ages = cur.fetchone()
                cur.execute("select age(datfrozenxid)::bigint as age from pg_database where datname=current_database()")
                db_xid_age = cur.fetchone()["age"]
                cur.execute("""
                    select coalesce(max(age(c.relfrozenxid)),0)::bigint as max_age
                    from pg_class c join pg_namespace n on n.oid=c.relnamespace
                    where c.relkind in ('r','m','t') and n.nspname not in ('pg_catalog','information_schema')
                """)
                table_xid = cur.fetchone()["max_age"]
                cur.execute("""
                    select coalesce(max(n_dead_tup::float8 / greatest(1,(n_live_tup+n_dead_tup))::float8),0)::float8 as max_ratio
                    from pg_stat_user_tables
                """)
                dead_ratio = cur.fetchone()["max_ratio"]
                cur.execute("select count(*)::int as n from pg_index where not indisvalid or not indisready")
                invalid_indexes = cur.fetchone()["n"]
                cur.execute("select count(*)::int as n from pg_constraint where not convalidated")
                unvalidated = cur.fetchone()["n"]
                cur.execute("select count(*)::int as n from pg_prepared_xacts where database=current_database()")
                prepared = cur.fetchone()["n"]
                cur.execute("""
                    select
                      coalesce(max(n_mod_since_analyze::float8 / greatest(1,n_live_tup)::float8),0)::float8 as max_mod_ratio,
                      count(*) filter (where n_live_tup > 0 and last_analyze is null and last_autoanalyze is null)::int as never_analyzed,
                      coalesce(max(extract(epoch from (clock_timestamp()-greatest(last_analyze,last_autoanalyze)))) filter (where greatest(last_analyze,last_autoanalyze) is not null),0)::float8 as oldest_stats_age_s
                    from pg_stat_user_tables
                """)
                stats = cur.fetchone()
                max_sequence_util = 0.0
                try:
                    cur.execute("""
                        select coalesce(max(
                          case
                            when increment_by > 0 and last_value is not null and max_value > min_value
                              then (last_value - min_value)::float8 / (max_value - min_value)::float8
                            when increment_by < 0 and last_value is not null and max_value > min_value
                              then (max_value - last_value)::float8 / (max_value - min_value)::float8
                            else 0
                          end
                        ),0)::float8 as max_util
                        from pg_sequences where schemaname not in ('pg_catalog','information_schema') and not cycle
                    """)
                    max_sequence_util = float(cur.fetchone()["max_util"] or 0)
                except Exception:
                    max_sequence_util = 0.0
                # Replication slot columns are version-sensitive; guard the query.
                max_inactive_retained = 0; lost_slots = 0; slot_rows = []
                try:
                    cur.execute("""
                        select slot_name, slot_type, active, wal_status,
                               case when restart_lsn is null then 0 else pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)::bigint end as retained_bytes
                        from pg_replication_slots
                    """)
                    slot_rows = cur.fetchall()
                    for r in slot_rows:
                        if not r["active"]: max_inactive_retained = max(max_inactive_retained, int(r["retained_bytes"] or 0))
                        if r.get("wal_status") == "lost": lost_slots += 1
                except Exception:
                    slot_rows = []
                obs = {
                    "database": dbname,
                    "connections": conn_count,
                    "max_connections": meta["max_connections"],
                    "connections_utilization": conn_count / max(1, int(meta["max_connections"])),
                    "oldest_transaction_s": ages["oldest_xact_s"],
                    "oldest_idle_in_transaction_s": ages["oldest_idle_s"],
                    "database_xid_age": db_xid_age,
                    "max_table_xid_age": table_xid,
                    "max_dead_tuple_ratio": dead_ratio,
                    "invalid_indexes": invalid_indexes,
                    "unvalidated_constraints": unvalidated,
                    "prepared_transactions": prepared,
                    "max_mod_since_analyze_ratio": stats["max_mod_ratio"],
                    "never_analyzed_tables": stats["never_analyzed"],
                    "oldest_statistics_age_s": stats["oldest_stats_age_s"],
                    "max_sequence_utilization": max_sequence_util,
                    "max_inactive_slot_retained_bytes": max_inactive_retained,
                    "lost_replication_slots": lost_slots,
                    "replication_slots": slot_rows,
                }
                ev["observed"] = obs
                ev["violations"].extend(evaluate_health(obs, policy))
                status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    ev["reproduction"] = {"dsn_env": dsn_env, "policy": policy}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status in {"PASS", "NOT_RUN"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
