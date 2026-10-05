#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import operator
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load, require_test_database_name, redact_dsn, sha256_file

OPS = {"==": operator.eq, "!=": operator.ne, ">=": operator.ge, ">": operator.gt, "<=": operator.le, "<": operator.lt}


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def main() -> int:
    ap = argparse.ArgumentParser(description="Validate benchmark dataset shape using explicit SQL oracles")
    ap.add_argument("--dsn-env", required=True)
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--id", default="dataset.audit")
    ap.add_argument("--safe-database-pattern", default=r"(_test|_bench|sqlverify)$")
    ap.add_argument("--allow-destructive", action="store_true")
    ap.add_argument("--statement-timeout-ms", type=int, default=10000)
    ns = ap.parse_args()
    start = time.monotonic()
    ev = evidence_base("dataset_audit", ns.id)
    dsn = os.environ.get(ns.dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {ns.dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    manifest_path = Path(ns.manifest)
    manifest = json_load(manifest_path)
    ev["reproduction"] = {"dsn_env": ns.dsn_env, "dsn_redacted": redact_dsn(dsn), "manifest": str(manifest_path), "manifest_sha256": sha256_file(manifest_path), "statement_timeout_ms": ns.statement_timeout_ms}
    ev["observed"] = {"profile": manifest.get("profile"), "seed": manifest.get("seed"), "checks": []}
    try:
        psycopg = _psycopg()
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
                require_test_database_name(dbname, ns.safe_database_pattern, ns.allow_destructive)
                cur.execute("set transaction read only")
                cur.execute("select set_config('statement_timeout', %s, true)", (f"{ns.statement_timeout_ms}ms",))
                ev["observed"]["database"] = dbname
                for check in manifest.get("checks", []):
                    cur.execute(check["sql"])
                    row = cur.fetchone()
                    actual = row[0] if row else None
                    op = check.get("op", "==")
                    expected = check.get("expected")
                    passed = OPS[op](actual, expected)
                    item = {"id": check["id"], "actual": actual, "op": op, "expected": expected, "passed": passed, "description": check.get("description")}
                    ev["observed"]["checks"].append(item)
                    if not passed:
                        ev["violations"].append(f"dataset check {check['id']} failed: {actual!r} {op} {expected!r}")
                conn.rollback()
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e))
        status = "BLOCKED"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
