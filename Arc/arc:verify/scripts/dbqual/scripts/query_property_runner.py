#!/usr/bin/env python3
from __future__ import annotations

import argparse
import collections
import json
import os
import re
import time
from pathlib import Path
from typing import Any

from parameter_corpus import generate
from sqlverify_common import evidence_base, finalize_evidence, json_dump, redact_dsn, sha256_file, require_test_database_name
from sqlverify_config import load_config

PG_PARAM_RE = re.compile(r"\$(\d+)")


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required: python -m pip install 'psycopg[binary]>=3.2,<4'") from e


def compile_sql(sql: str, params: list[Any], style: str) -> tuple[str, list[Any]]:
    if style == "psycopg":
        return sql, params
    if style != "pg_dollar":
        raise ValueError(f"unsupported param_style: {style}")
    order: list[int] = []
    def repl(m: re.Match[str]) -> str:
        order.append(int(m.group(1)) - 1)
        return "%s"
    compiled = PG_PARAM_RE.sub(repl, sql)
    try:
        bound = [params[i] for i in order]
    except IndexError as e:
        raise ValueError(f"parameter set has {len(params)} values but SQL references ${max(order)+1 if order else 0}") from e
    return compiled, bound


def row_key(row: Any) -> str:
    return json.dumps(list(row), ensure_ascii=False, sort_keys=True, default=str, separators=(",", ":"))


def compare(kind: str, left: list[Any], right: list[Any]) -> tuple[bool, dict[str, Any]]:
    if kind == "exact_multiset":
        lc = collections.Counter(row_key(r) for r in left)
        rc = collections.Counter(row_key(r) for r in right)
        return lc == rc, {"left_rows": len(left), "right_rows": len(right), "left_only": sum((lc-rc).values()), "right_only": sum((rc-lc).values())}
    if kind == "count_matches_rows":
        if len(right) != 1 or len(right[0]) != 1:
            return False, {"error": "oracle query must return exactly one scalar row", "left_rows": len(left), "right_rows": len(right)}
        try:
            expected = int(right[0][0])
        except Exception:
            return False, {"error": "oracle scalar is not integer-like", "oracle": str(right[0][0])}
        return len(left) == expected, {"actual_rows": len(left), "oracle_count": expected}
    if kind == "subset":
        lc = collections.Counter(row_key(r) for r in left)
        rc = collections.Counter(row_key(r) for r in right)
        missing = lc - rc
        return not missing, {"left_rows": len(left), "right_rows": len(right), "left_not_in_right": sum(missing.values())}
    raise ValueError(f"unsupported property kind: {kind}")


def main() -> int:
    ap = argparse.ArgumentParser(description="Execute deterministic generated SQL property/metamorphic tests against PostgreSQL")
    ap.add_argument("--config", required=True)
    ap.add_argument("--property", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    prop = next((x for x in cfg.get("property_test", []) if x.get("id") == ns.property), None)
    if not prop:
        raise SystemExit(f"property_test not found: {ns.property}")

    started = time.monotonic()
    ev_id = f"property.{prop['id']}"
    ev = evidence_base("query_property", ev_id)
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED")); return 2

    query_path = root / prop["query_file"]
    oracle_path = root / prop["oracle_query_file"]
    corpus_path = root / prop["corpus_spec"]
    ev["reproduction"] = {
        "dsn_env": dsn_env, "dsn_redacted": redact_dsn(dsn), "property": prop["id"], "kind": prop["kind"],
        "query_sha256": sha256_file(query_path), "oracle_query_sha256": sha256_file(oracle_path),
        "corpus_spec_sha256": sha256_file(corpus_path),
    }

    try:
        psycopg = _psycopg()
        spec = json.loads(corpus_path.read_text(encoding="utf-8"))
        corpus = generate(spec)
        max_cases = int(prop.get("max_cases", corpus["count"]))
        cases = corpus["cases"][:max_cases]
        query_sql = query_path.read_text(encoding="utf-8")
        oracle_sql = oracle_path.read_text(encoding="utf-8")
        style = prop.get("param_style", "pg_dollar")
        statement_timeout_ms = int(prop.get("statement_timeout_ms", 5000))
        failures = []
        durations = []
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
                require_test_database_name(dbname, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False)))
                cur.execute("set transaction read only")
                cur.execute("select set_config('statement_timeout', %s, true)", (f"{statement_timeout_ms}ms",))
                for case in cases:
                    qsql, qparams = compile_sql(query_sql, case["params"], style)
                    osql, oparams = compile_sql(oracle_sql, case["params"], style)
                    t0 = time.monotonic()
                    cur.execute(qsql, qparams); left = cur.fetchall()
                    cur.execute(osql, oparams); right = cur.fetchall()
                    elapsed = (time.monotonic() - t0) * 1000.0
                    durations.append(elapsed)
                    ok, details = compare(prop["kind"], left, right)
                    if not ok:
                        failures.append({"case": case, "details": details, "elapsed_ms": round(elapsed, 3)})
                        if len(failures) >= int(prop.get("max_failure_examples", 20)):
                            break
                conn.rollback()
        ev["observed"] = {
            "database": dbname, "kind": prop["kind"], "seed": corpus["seed"], "generated_case_count": corpus["count"],
            "executed_cases": min(len(cases), len(cases) if not failures else max(1, len(durations))),
            "failed_cases": len(failures), "max_case_elapsed_ms": round(max(durations), 3) if durations else None,
            "failure_examples": failures,
        }
        ev["reproduction"]["seed"] = corpus["seed"]
        ev["reproduction"]["case_count"] = len(cases)
        if failures:
            ev["violations"].append(f"{len(failures)} property case(s) violated {prop['kind']}")
        status = "PASS" if not failures else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
