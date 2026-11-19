#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    require_test_database_name,
    scalar_value,
    SCALAR_OPERATORS,
)
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
        description="Audit PostgreSQL privilege/RLS/SECURITY DEFINER boundaries with explicit policy"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    policy = cfg.get("security_audit", {})
    start = time.monotonic()
    ev = evidence_base("security_boundary", "security.boundary")
    if policy and not policy.get("enabled", True):
        ev["warnings"].append("security audit disabled")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "NOT_RUN"))
        return 0
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    obs: dict[str, Any] = {}
    try:
        psycopg, dict_row = _psycopg()
        with psycopg.connect(dsn, autocommit=True, row_factory=dict_row) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database() as db")
                db = cur.fetchone()["db"]
                require_test_database_name(
                    db, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False))
                )
                obs["database"] = db
                cur.execute(
                    "select rolname, rolsuper, rolcreaterole, rolcreatedb, rolcanlogin from pg_roles order by rolname"
                )
                roles = cur.fetchall()
                obs["roles"] = roles
                allowed_super = set(policy.get("allowed_superusers", ["postgres"]))
                extra_super = [r["rolname"] for r in roles if r["rolsuper"] and r["rolname"] not in allowed_super]
                obs["unexpected_superusers"] = extra_super
                if policy.get("require_no_unexpected_superusers", False) and extra_super:
                    ev["violations"].append("unexpected superusers: " + ", ".join(extra_super))
                allowed_createrole = set(policy.get("allowed_createrole", list(allowed_super)))
                extra_cr = [
                    r["rolname"] for r in roles if r["rolcreaterole"] and r["rolname"] not in allowed_createrole
                ]
                obs["unexpected_createrole_roles"] = extra_cr
                if policy.get("require_no_unexpected_createrole", False) and extra_cr:
                    ev["violations"].append("unexpected CREATEROLE roles: " + ", ".join(extra_cr))
                # SECURITY DEFINER without an explicit search_path setting is a common review target.
                cur.execute("""
                    select n.nspname as schema_name, p.proname, pg_get_userbyid(p.proowner) as owner,
                           coalesce(p.proconfig, array[]::text[]) as proconfig
                    from pg_proc p join pg_namespace n on n.oid=p.pronamespace
                    where p.prosecdef and n.nspname not in ('pg_catalog','information_schema')
                    order by 1,2
                """)
                sdef = cur.fetchall()
                unsafe = []
                for r in sdef:
                    has_sp = any(str(x).startswith("search_path=") for x in (r["proconfig"] or []))
                    if not has_sp:
                        unsafe.append(r)
                obs["security_definer_functions"] = sdef[:500]
                obs["security_definer_without_explicit_search_path"] = unsafe[:500]
                if policy.get("require_security_definer_search_path", False) and unsafe:
                    ev["violations"].append(f"SECURITY DEFINER functions without explicit search_path: {len(unsafe)}")
                # Explicit table list for RLS avoids cargo-culting RLS onto every table.
                rls = []
                for name in policy.get("require_rls", []):
                    if "." not in name:
                        raise ValueError(f"require_rls entry must be schema.table: {name!r}")
                    schema, table = name.split(".", 1)
                    cur.execute(
                        """
                        select c.relrowsecurity as enabled, c.relforcerowsecurity as forced
                        from pg_class c join pg_namespace n on n.oid=c.relnamespace
                        where n.nspname=%s and c.relname=%s
                    """,
                        (schema, table),
                    )
                    row = cur.fetchone()
                    item = {
                        "table": name,
                        "exists": bool(row),
                        "enabled": bool(row and row["enabled"]),
                        "forced": bool(row and row["forced"]),
                    }
                    rls.append(item)
                    if not row or not row["enabled"]:
                        ev["violations"].append(f"required RLS not enabled: {name}")
                    if policy.get("require_force_rls", False) and row and not row["forced"]:
                        ev["violations"].append(f"required FORCE ROW LEVEL SECURITY missing: {name}")
                obs["required_rls"] = rls
                cur.execute("select extname, extversion from pg_extension order by extname")
                exts = cur.fetchall()
                obs["extensions"] = exts
                allowed_ext = set(policy.get("allowed_extensions", []))
                if allowed_ext:
                    unexpected = [e["extname"] for e in exts if e["extname"] not in allowed_ext]
                    obs["unexpected_extensions"] = unexpected
                    if unexpected and policy.get("require_extension_allowlist", True):
                        ev["violations"].append("extensions outside allowlist: " + ", ".join(unexpected))
                assertions = obs.setdefault("assertions", [])
                for a in policy.get("assertion", []):
                    cur.execute("begin read only")
                    try:
                        cur.execute(
                            "select set_config('statement_timeout', %s, true)",
                            (f"{int(a.get('statement_timeout_ms', 10000))}ms",),
                        )
                        cur.execute(a["sql"])
                        actual = scalar_value(cur)
                    finally:
                        cur.execute("rollback")
                    op = a.get("op", "==")
                    expected = a["expected"]
                    ok = SCALAR_OPERATORS[op](actual, expected)
                    assertions.append({"id": a["id"], "actual": actual, "op": op, "expected": expected, "passed": ok})
                    if not ok:
                        ev["violations"].append(f"security assertion {a['id']} failed")
                obs["assertions"] = assertions
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["warnings"].append(repr(e))
        status = "FAIL" if ev["violations"] else "BLOCKED"
    ev["observed"] = obs
    ev["reproduction"] = {"dsn_env": dsn_env, "policy": policy}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status in {"PASS", "NOT_RUN"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
