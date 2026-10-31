#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import operator
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name, scalar_value
from sqlverify_config import load_config

OPS = {"==": operator.eq, "!=": operator.ne, ">=": operator.ge, ">": operator.gt, "<=": operator.le, "<": operator.lt}


def _psycopg():
    try:
        import psycopg  # type: ignore

        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def main() -> int:
    ap = argparse.ArgumentParser(description="Run independent business/database invariants as release evidence")
    ap.add_argument("--config", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ap.add_argument("--id", default="invariant.audit")
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    checks = cfg.get("invariant", [])
    start = time.monotonic()
    ev = evidence_base("invariant_audit", ns.id)
    ev["observed"] = {"checks": []}
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    if not checks:
        ev["warnings"].append("no invariant checks configured")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "NOT_RUN"))
        return 0
    try:
        psycopg = _psycopg()
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
                require_test_database_name(
                    dbname,
                    cfg["database"]["safe_database_pattern"],
                    bool(cfg["database"].get("allow_destructive", False)),
                )
                cur.execute("set transaction read only")
                for c in checks:
                    timeout_ms = int(c.get("statement_timeout_ms", 10000))
                    cur.execute("select set_config('statement_timeout', %s, true)", (f"{timeout_ms}ms",))
                    sql = c.get("sql")
                    if not sql and c.get("sql_file"):
                        sql = (root / c["sql_file"]).read_text(encoding="utf-8")
                    cur.execute(sql)
                    actual = scalar_value(cur)
                    op = c.get("op", "==")
                    expected = c["expected"]
                    passed = OPS[op](actual, expected)
                    item = {
                        "id": c["id"],
                        "actual": actual,
                        "op": op,
                        "expected": expected,
                        "passed": passed,
                        "description": c.get("description"),
                    }
                    ev["observed"]["checks"].append(item)
                    if not passed:
                        ev["violations"].append(f"invariant {c['id']} failed: {actual!r} {op} {expected!r}")
                conn.rollback()
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["warnings"].append(repr(e))
        status = "FAIL" if ev["violations"] else "BLOCKED"
    ev["reproduction"] = {"dsn_env": dsn_env, "config": str(Path(ns.config).resolve())}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status in {"PASS", "NOT_RUN"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
