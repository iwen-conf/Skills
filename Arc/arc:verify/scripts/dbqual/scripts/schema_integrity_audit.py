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


def evaluate(obs: dict[str, Any], policy: dict[str, Any]) -> list[str]:
    v: list[str] = []
    checks = [
        ("invalid_indexes", "require_no_invalid_indexes"),
        ("unvalidated_constraints", "require_no_unvalidated_constraints"),
        ("orphaned_sequences", "require_no_orphaned_sequences"),
    ]
    for key, flag in checks:
        if policy.get(flag, False) and int(obs.get(key, 0) or 0) != 0:
            v.append(f"{key}={obs.get(key)} but {flag}=true")
    if policy.get("max_tables_without_primary_key") is not None:
        actual = int(obs.get("tables_without_primary_key", 0) or 0)
        limit = int(policy["max_tables_without_primary_key"])
        if actual > limit:
            v.append(f"tables_without_primary_key={actual} > {limit}")
    if policy.get("max_unindexed_foreign_keys") is not None:
        actual = int(obs.get("unindexed_foreign_keys", 0) or 0)
        limit = int(policy["max_unindexed_foreign_keys"])
        if actual > limit:
            v.append(f"unindexed_foreign_keys={actual} > {limit}")
    if policy.get("max_default_partition_rows") is not None:
        actual = int(obs.get("max_default_partition_rows", 0) or 0)
        limit = int(policy["max_default_partition_rows"])
        if actual > limit:
            v.append(f"max_default_partition_rows={actual} > {limit}")
    return v


def main() -> int:
    ap = argparse.ArgumentParser(description="Audit PostgreSQL schema-level integrity and performance hazards")
    ap.add_argument("--config", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config)); policy = cfg.get("schema_integrity", {})
    start = time.monotonic(); ev = evidence_base("schema_integrity", "schema.integrity")
    if policy and not policy.get("enabled", True):
        ev["warnings"].append("schema integrity audit disabled")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "NOT_RUN")); return 0
    dsn_env = cfg["database"]["dsn_env"]; dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    obs: dict[str, Any] = {}
    try:
        psycopg, dict_row = _psycopg()
        with psycopg.connect(dsn, autocommit=True, row_factory=dict_row) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database() as db")
                db = cur.fetchone()["db"]
                require_test_database_name(db, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False)))
                obs["database"] = db
                cur.execute("select count(*)::int as n from pg_index where not indisvalid or not indisready")
                obs["invalid_indexes"] = cur.fetchone()["n"]
                cur.execute("select count(*)::int as n from pg_constraint where not convalidated")
                obs["unvalidated_constraints"] = cur.fetchone()["n"]
                cur.execute("""
                    select n.nspname as schema_name, c.relname as table_name
                    from pg_class c join pg_namespace n on n.oid=c.relnamespace
                    where c.relkind in ('r','p')
                      and n.nspname not in ('pg_catalog','information_schema')
                      and not exists (
                        select 1 from pg_index i where i.indrelid=c.oid and i.indisprimary
                      )
                    order by 1,2
                """)
                no_pk = cur.fetchall(); obs["tables_without_primary_key"] = len(no_pk); obs["tables_without_primary_key_detail"] = no_pk[:200]
                # Approximate FK coverage: an index whose leading columns equal FK columns in declared order.
                cur.execute("""
                    with fk as (
                      select con.oid, con.conname, con.conrelid, con.conkey,
                             n.nspname as schema_name, c.relname as table_name
                      from pg_constraint con
                      join pg_class c on c.oid=con.conrelid
                      join pg_namespace n on n.oid=c.relnamespace
                      where con.contype='f' and n.nspname not in ('pg_catalog','information_schema')
                    )
                    select fk.schema_name, fk.table_name, fk.conname,
                           not exists (
                             select 1 from pg_index i
                             where i.indrelid=fk.conrelid and i.indisvalid and i.indisready
                               and (i.indkey::smallint[])[0:cardinality(fk.conkey)-1] = fk.conkey
                           ) as missing_covering_index
                    from fk order by 1,2,3
                """)
                fks = cur.fetchall(); missing = [r for r in fks if r["missing_covering_index"]]
                obs["foreign_keys"] = len(fks); obs["unindexed_foreign_keys"] = len(missing); obs["unindexed_foreign_key_detail"] = missing[:200]
                # Sequences not owned by any table column. This is not always wrong, hence policy-controlled.
                cur.execute("""
                    select n.nspname as schema_name, c.relname as sequence_name
                    from pg_class c join pg_namespace n on n.oid=c.relnamespace
                    where c.relkind='S' and n.nspname not in ('pg_catalog','information_schema')
                      and not exists (
                        select 1 from pg_depend d
                        where d.objid=c.oid and d.deptype in ('a','i')
                      )
                    order by 1,2
                """)
                seq = cur.fetchall(); obs["orphaned_sequences"] = len(seq); obs["orphaned_sequence_detail"] = seq[:200]
                # Default partitions can silently absorb unexpected keys. Measure approximate live rows.
                cur.execute("""
                    select pn.nspname as schema_name, pc.relname as parent_name,
                           cn.nspname as child_schema, cc.relname as default_partition,
                           coalesce(st.n_live_tup,0)::bigint as estimated_rows
                    from pg_partitioned_table pt
                    join pg_class pc on pc.oid=pt.partrelid join pg_namespace pn on pn.oid=pc.relnamespace
                    join pg_inherits inh on inh.inhparent=pc.oid
                    join pg_class cc on cc.oid=inh.inhrelid join pg_namespace cn on cn.oid=cc.relnamespace
                    left join pg_stat_all_tables st on st.relid=cc.oid
                    where pg_get_expr(cc.relpartbound, cc.oid)='DEFAULT'
                    order by estimated_rows desc
                """)
                defaults = cur.fetchall(); obs["default_partitions"] = defaults[:200]
                obs["max_default_partition_rows"] = max([int(r["estimated_rows"] or 0) for r in defaults], default=0)
                # User-supplied read-only assertions make schema contracts domain-specific.
                assertions = []
                for a in policy.get("assertion", []):
                    cur.execute("begin read only")
                    try:
                        cur.execute("select set_config('statement_timeout', %s, true)", (f"{int(a.get('statement_timeout_ms',10000))}ms",))
                        cur.execute(a["sql"]); row = cur.fetchone(); actual = list(row.values())[0] if row else None
                    finally:
                        cur.execute("rollback")
                    op=a.get("op","=="); expected=a.get("expected")
                    ok={"==":actual==expected,"!=":actual!=expected,">":actual>expected,">=":actual>=expected,"<":actual<expected,"<=":actual<=expected}[op]
                    assertions.append({"id":a["id"],"actual":actual,"op":op,"expected":expected,"passed":ok})
                    if not ok: ev["violations"].append(f"schema assertion {a['id']} failed: {actual!r} {op} {expected!r}")
                obs["assertions"] = assertions
        ev["violations"].extend(evaluate(obs, policy)); status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    ev["observed"] = obs; ev["reproduction"] = {"dsn_env": dsn_env, "policy": policy}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status in {"PASS","NOT_RUN"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
