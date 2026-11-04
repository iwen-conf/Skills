#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, redact_dsn, sha256_bytes, host_metadata


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required: python -m pip install 'psycopg[binary]>=3.2,<4'") from e


def snapshot(dsn: str) -> dict[str, Any]:
    psycopg = _psycopg()
    out: dict[str, Any] = {}
    with psycopg.connect(dsn, autocommit=True) as conn:
        with conn.cursor() as cur:
            cur.execute("select current_database(), current_user, current_setting('server_version_num')::int, version()")
            db, user, version_num, version_text = cur.fetchone()
            out.update({"database": db, "user": user, "server_version_num": version_num, "version": version_text})
            keys = [
                "shared_buffers", "work_mem", "maintenance_work_mem", "effective_cache_size",
                "random_page_cost", "effective_io_concurrency", "max_connections", "max_worker_processes",
                "max_parallel_workers", "max_parallel_workers_per_gather", "jit", "track_io_timing",
                "default_transaction_isolation", "synchronous_commit", "wal_compression", "shared_preload_libraries",
                "plan_cache_mode", "autovacuum", "autovacuum_max_workers", "autovacuum_naptime",
                "autovacuum_vacuum_scale_factor", "autovacuum_analyze_scale_factor", "autovacuum_freeze_max_age",
                "max_locks_per_transaction", "deadlock_timeout", "lock_timeout", "statement_timeout",
                "idle_in_transaction_session_timeout", "wal_level", "max_wal_size", "min_wal_size",
                "checkpoint_timeout", "checkpoint_completion_target", "max_slot_wal_keep_size",
                "hot_standby", "synchronous_standby_names", "track_wal_io_timing", "data_checksums",
                "archive_mode", "archive_command", "max_replication_slots", "max_wal_senders"
            ]
            cur.execute("select name, setting, unit, source from pg_settings where name = any(%s) or name like 'pg_stat_statements.%' order by name", (keys,))
            settings = [{"name": r[0], "setting": r[1], "unit": r[2], "source": r[3]} for r in cur.fetchall()]
            out["settings"] = settings
            out["settings_sha256"] = sha256_bytes(json.dumps(settings, sort_keys=True).encode())
            cur.execute("select extname, extversion from pg_extension order by extname")
            out["extensions"] = [{"name": r[0], "version": r[1]} for r in cur.fetchall()]
            cur.execute("select datname, xact_commit, xact_rollback, blks_read, blks_hit, temp_files, temp_bytes, deadlocks from pg_stat_database where datname=current_database()")
            row = cur.fetchone()
            out["pg_stat_database"] = dict(zip([d.name for d in cur.description], row)) if row else None
            cur.execute("select wal_records, wal_fpi, wal_bytes, wal_buffers_full, stats_reset from pg_stat_wal")
            row = cur.fetchone()
            out["pg_stat_wal"] = dict(zip([d.name for d in cur.description], row)) if row else None
            try:
                cur.execute("select * from pg_stat_checkpointer")
                cols = [d.name for d in cur.description]
                row = cur.fetchone()
                out["pg_stat_checkpointer"] = dict(zip(cols, row)) if row else None
            except Exception as exc:
                conn.rollback()
                out["pg_stat_checkpointer_unavailable"] = type(exc).__name__
            try:
                cur.execute("select * from pg_stat_archiver")
                cols = [d.name for d in cur.description]
                row = cur.fetchone()
                out["pg_stat_archiver"] = dict(zip(cols, row)) if row else None
            except Exception as exc:
                conn.rollback()
                out["pg_stat_archiver_unavailable"] = type(exc).__name__
            if version_num >= 160000:
                cur.execute("select * from pg_stat_io order by backend_type, object, context")
                cols = [d.name for d in cur.description]
                out["pg_stat_io"] = [dict(zip(cols, r)) for r in cur.fetchall()]
            try:
                cur.execute("""
                    select slot_name, slot_type, active, wal_status, safe_wal_size,
                           pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn) as retained_wal_bytes
                    from pg_replication_slots order by slot_name
                """)
                cols = [d.name for d in cur.description]
                out["replication_slots"] = [dict(zip(cols, r)) for r in cur.fetchall()]
            except Exception as exc:
                conn.rollback()
                out["replication_slots_unavailable"] = type(exc).__name__
            cur.execute("select count(*) from pg_stat_activity where datname=current_database()")
            out["active_sessions_snapshot"] = cur.fetchone()[0]
            cur.execute("""
                select table_schema, table_name, ordinal_position, column_name, data_type, is_nullable, coalesce(column_default,'')
                from information_schema.columns
                where table_schema not in ('pg_catalog','information_schema')
                order by table_schema, table_name, ordinal_position
            """)
            columns = cur.fetchall()
            cur.execute("select schemaname, tablename, indexname, indexdef from pg_indexes where schemaname not in ('pg_catalog','information_schema') order by schemaname, tablename, indexname")
            indexes = cur.fetchall()
            cur.execute("""
                select n.nspname, c.relname, con.conname, con.contype, con.convalidated, pg_get_constraintdef(con.oid, true)
                from pg_constraint con
                join pg_class c on c.oid=con.conrelid
                join pg_namespace n on n.oid=c.relnamespace
                where n.nspname not in ('pg_catalog','information_schema')
                order by n.nspname,c.relname,con.conname
            """)
            constraints = cur.fetchall()
            cur.execute("""
                select n.nspname, c.relname, t.tgname, pg_get_triggerdef(t.oid, true)
                from pg_trigger t
                join pg_class c on c.oid=t.tgrelid
                join pg_namespace n on n.oid=c.relnamespace
                where not t.tgisinternal and n.nspname not in ('pg_catalog','information_schema')
                order by n.nspname,c.relname,t.tgname
            """)
            triggers = cur.fetchall()
            cur.execute("""
                select n.nspname, c.relname, c.relkind, pg_get_expr(c.relpartbound, c.oid, true)
                from pg_class c join pg_namespace n on n.oid=c.relnamespace
                where c.relispartition and n.nspname not in ('pg_catalog','information_schema')
                order by n.nspname,c.relname
            """)
            partitions = cur.fetchall()
            cur.execute("""
                select n.nspname, p.proname, pg_get_function_identity_arguments(p.oid), p.prokind,
                       p.provolatile, p.prosecdef, pg_get_functiondef(p.oid)
                from pg_proc p join pg_namespace n on n.oid=p.pronamespace
                where n.nspname not in ('pg_catalog','information_schema')
                order by n.nspname,p.proname,pg_get_function_identity_arguments(p.oid)
            """)
            functions = cur.fetchall()
            schema_material = {
                "columns": columns, "indexes": indexes, "constraints": constraints,
                "triggers": triggers, "partitions": partitions, "functions": functions,
            }
            out["schema_fingerprint_sha256"] = sha256_bytes(json.dumps(schema_material, default=str, sort_keys=True).encode())
            out["schema_object_counts"] = {
                "columns": len(columns), "indexes": len(indexes), "constraints": len(constraints),
                "triggers": len(triggers), "partitions": len(partitions), "functions": len(functions),
            }
            out["host"] = host_metadata()
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dsn-env", required=True)
    ap.add_argument("--id", default="postgres.snapshot")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    dsn = os.environ.get(ns.dsn_env)
    start = time.monotonic()
    ev = evidence_base("postgres_snapshot", ns.id)
    if not dsn:
        ev["violations"].append(f"missing env {ns.dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    ev["reproduction"] = {"dsn_env": ns.dsn_env, "dsn_redacted": redact_dsn(dsn)}
    try:
        ev["observed"] = snapshot(dsn)
        status = "PASS"
    except Exception as e:
        ev["violations"].append(repr(e))
        status = "BLOCKED"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
