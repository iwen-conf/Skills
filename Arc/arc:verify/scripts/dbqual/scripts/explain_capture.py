#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import time
from pathlib import Path
from typing import Any

from plan_analyzer import analyze_explain
from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load, require_test_database_name, sha256_file
from sqlverify_config import load_config

DML_WRITE = {"INSERT", "UPDATE", "DELETE", "MERGE", "CREATE", "ALTER", "DROP", "TRUNCATE", "CALL", "DO", "COPY", "REFRESH", "GRANT", "REVOKE"}


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def first_keyword(sql: str) -> str:
    s = re.sub(r"/\*.*?\*/", " ", sql, flags=re.S)
    s = re.sub(r"--[^\n]*", " ", s).lstrip()
    m = re.match(r"([A-Za-z]+)", s)
    return m.group(1).upper() if m else ""


def potential_write(sql: str) -> str | None:
    # Conservative safety classifier. It intentionally favors false positives over executing
    # an unapproved write through EXPLAIN ANALYZE. Strip comments and common quoted literals first.
    s = re.sub(r"/\*.*?\*/", " ", sql, flags=re.S)
    s = re.sub(r"--[^\n]*", " ", s)
    s = re.sub(r"'(?:''|[^'])*'", "''", s, flags=re.S)
    s = re.sub(r'"(?:""|[^"])*"', '""', s, flags=re.S)
    for kw in DML_WRITE:
        if re.search(rf"\b{kw}\b", s, flags=re.I):
            return kw
    return None


def convert_pg_dollar(sql: str, params: list[Any]) -> tuple[str, list[Any]]:
    out_params: list[Any] = []
    def repl(m: re.Match[str]) -> str:
        idx = int(m.group(1)) - 1
        if idx < 0 or idx >= len(params):
            raise ValueError(f"placeholder ${idx+1} has no value")
        out_params.append(params[idx])
        return "%s"
    converted = re.sub(r"\$(\d+)", repl, sql)
    return converted, out_params


def main() -> int:
    ap = argparse.ArgumentParser(description="Capture EXPLAIN ANALYZE JSON for declared business SQL parameter sets")
    ap.add_argument("--config", required=True)
    ap.add_argument("--query", required=True)
    ap.add_argument("--parameter-set", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    q = next((x for x in cfg.get("query", []) if x.get("id") == ns.query), None)
    if not q:
        raise SystemExit(f"query not found: {ns.query}")
    ps = next((x for x in q.get("parameter_set", []) if x.get("name") == ns.parameter_set), None)
    if not ps:
        raise SystemExit(f"parameter set not found: {ns.parameter_set}")
    evidence_id = f"explain.{ns.query}.{ns.parameter_set}"
    start = time.monotonic()
    ev = evidence_base("explain", evidence_id)
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    sql_path = root / q["sql_file"]
    params_path = root / ps["params_file"]
    sql = sql_path.read_text(encoding="utf-8").strip().rstrip(";")
    pdata = json_load(params_path)
    params = pdata["params"] if isinstance(pdata, dict) else pdata
    if q.get("param_style", "pg_dollar") == "pg_dollar":
        sql_exec, bind = convert_pg_dollar(sql, list(params))
    elif q.get("param_style") == "psycopg":
        sql_exec, bind = sql, list(params)
    else:
        raise SystemExit("param_style must be pg_dollar or psycopg")
    kw = first_keyword(sql)
    ev["reproduction"] = {"query_file": q["sql_file"], "query_sha256": sha256_file(sql_path), "params_file": ps["params_file"], "params_sha256": sha256_file(params_path), "dsn_env": dsn_env}

    try:
        psycopg = _psycopg()
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database(), current_setting('server_version_num')::int")
                dbname, version_num = cur.fetchone()
                require_test_database_name(dbname, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False)))
                write_kw = potential_write(sql)
                if write_kw and not q.get("allow_write_explain", False):
                    raise RuntimeError(f"refusing EXPLAIN ANALYZE for potential write statement {write_kw}; set query.allow_write_explain=true only in isolated DB")
                statement_timeout_ms = int(q.get("statement_timeout_ms", 30000))
                cur.execute("select set_config('statement_timeout', %s, true)", (f"{statement_timeout_ms}ms",))
                ev["reproduction"]["statement_timeout_ms"] = statement_timeout_ms
                options = ["ANALYZE", "BUFFERS", "WAL", "SETTINGS", "FORMAT JSON", "TIMING FALSE", "SUMMARY TRUE"]
                if version_num >= 180000 and q.get("explain_memory", True):
                    options.append("MEMORY TRUE")
                explain_sql = f"EXPLAIN ({', '.join(options)}) {sql_exec}"
                cur.execute(explain_sql, bind)
                raw = cur.fetchone()[0]
                conn.rollback()  # even SELECT is rolled back to keep a consistent safety rule
                analysis = analyze_explain(raw)
                ev["observed"] = {"database": dbname, "server_version_num": version_num, "analysis": analysis, "raw_explain": raw}
                gates = q.get("gates", {})
                if gates.get("max_exec_ms") is not None and (analysis.get("execution_time_ms") or 0) > float(gates["max_exec_ms"]):
                    ev["violations"].append(f"execution_time_ms {analysis.get('execution_time_ms')} > {gates['max_exec_ms']}")
                if gates.get("max_estimate_error_ratio") is not None and analysis.get("max_estimate_error_ratio", 1) > float(gates["max_estimate_error_ratio"]):
                    ev["violations"].append(f"max_estimate_error_ratio {analysis.get('max_estimate_error_ratio')} > {gates['max_estimate_error_ratio']}")
                if gates.get("max_temp_written_blocks") is not None and analysis.get("temp_written_blocks_sum_nodes", 0) > int(gates["max_temp_written_blocks"]):
                    ev["violations"].append(f"temp_written_blocks {analysis.get('temp_written_blocks_sum_nodes')} > {gates['max_temp_written_blocks']}")
                forbidden = set(gates.get("forbid_node_types", []))
                used = set(analysis.get("node_types", {}))
                bad = sorted(forbidden & used)
                if bad:
                    ev["violations"].append(f"forbidden plan node types used: {bad}")
                status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e))
        status = "BLOCKED" if isinstance(e, (ImportError, RuntimeError)) else "FAIL"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
