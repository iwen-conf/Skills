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


def _psycopg():
    try:
        import psycopg  # type: ignore
        from psycopg import sql  # type: ignore
        return psycopg, sql
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def plan_signature(raw: Any) -> str:
    if isinstance(raw, list) and raw: raw = raw[0]
    root = raw.get("Plan", raw) if isinstance(raw, dict) else {}
    nodes: list[str] = []
    def walk(n: dict[str, Any]) -> None:
        nodes.append(str(n.get("Node Type", "?")))
        if n.get("Join Type"): nodes.append(f"join:{n['Join Type']}")
        if n.get("Index Name"): nodes.append(f"idx:{n['Index Name']}")
        for c in n.get("Plans", []) or []:
            walk(c)
    if isinstance(root, dict): walk(root)
    return " > ".join(nodes)


_SAFE_TYPE = re.compile(r'^(?:[A-Za-z_][A-Za-z0-9_$]*\.)?[A-Za-z_][A-Za-z0-9_$]*(?:\[\])?$')

def validate_param_type_name(value: str) -> str:
    # Keep PREPARE type declarations deliberately narrow. Use PostgreSQL aliases
    # such as int8/timestamptz/float8 rather than arbitrary type expressions.
    # This prevents a trusted-but-mistyped config from becoming SQL text.
    if not _SAFE_TYPE.fullmatch(value.strip()):
        raise ValueError(
            f"unsafe/unsupported parameter type {value!r}; use a simple PostgreSQL type or schema-qualified type (arrays allowed)"
        )
    return value.strip()


def convert_pg_dollar_to_prepare(sql_text: str) -> str:
    # PREPARE syntax already accepts $1, $2 placeholders, so only strip terminal semicolon.
    return sql_text.strip().rstrip(";")


def main() -> int:
    ap = argparse.ArgumentParser(description="Compare forced custom vs generic PostgreSQL plans across parameter classes")
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    root = Path(ns.root).resolve(); cfg = load_config(Path(ns.config))
    case = next((x for x in cfg.get("plan_sensitivity", []) if x.get("id") == ns.case), None)
    if not case: raise SystemExit(f"plan_sensitivity not found: {ns.case}")
    q = next((x for x in cfg.get("query", []) if x.get("id") == case.get("query")), None)
    if not q: raise SystemExit(f"query not found: {case.get('query')}")
    start = time.monotonic(); ev = evidence_base("plan_sensitivity", f"plan-sensitivity.{ns.case}")
    dsn_env = cfg["database"]["dsn_env"]; dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}"); json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    sql_path = root / q["sql_file"]; sql_text = convert_pg_dollar_to_prepare(sql_path.read_text(encoding="utf-8"))
    param_types = [validate_param_type_name(str(t)) for t in case.get("param_types", [])]
    if not param_types:
        ev["violations"].append("plan_sensitivity.param_types is required for PREPARE")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "FAIL")); return 2
    selected_names = case.get("parameter_sets") or [p.get("name") for p in q.get("parameter_set", [])]
    parameter_sets = []
    for name in selected_names:
        ps = next((x for x in q.get("parameter_set", []) if x.get("name") == name), None)
        if not ps: raise SystemExit(f"parameter set not found: {name}")
        pdata = json_load(root / ps["params_file"]); params = pdata["params"] if isinstance(pdata, dict) else pdata
        parameter_sets.append((name, list(params), ps))
    try:
        psycopg, sql = _psycopg()
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
                require_test_database_name(dbname, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False)))
                stmt_name = "sqlverify_plan_sensitivity"
                cur.execute(sql.SQL("PREPARE {} ({}) AS {}").format(
                    sql.Identifier(stmt_name),
                    sql.SQL(", ").join(sql.SQL(t) for t in param_types),
                    sql.SQL(sql_text),
                ))
                rows = []
                for name, params, ps in parameter_sets:
                    args = sql.SQL(", ").join(sql.Literal(v) for v in params)
                    row: dict[str, Any] = {"parameter_set": name, "params_file": ps["params_file"]}
                    for mode in ("force_custom_plan", "force_generic_plan"):
                        cur.execute(sql.SQL("SET LOCAL plan_cache_mode = {}").format(sql.SQL(mode)))
                        cur.execute(sql.SQL("EXPLAIN (FORMAT JSON, SETTINGS TRUE) EXECUTE {} ({})").format(sql.Identifier(stmt_name), args))
                        raw = cur.fetchone()[0]
                        ana = analyze_explain(raw)
                        row["custom" if mode == "force_custom_plan" else "generic"] = {
                            "analysis": ana,
                            "signature": plan_signature(raw),
                            "raw_explain": raw,
                        }
                    gcost = float((row["generic"]["analysis"] or {}).get("total_cost") or 0)
                    ccost = float((row["custom"]["analysis"] or {}).get("total_cost") or 0)
                    row["generic_custom_cost_ratio"] = (gcost / ccost) if ccost > 0 else None
                    row["plan_changed"] = row["generic"]["signature"] != row["custom"]["signature"]
                    rows.append(row)
                cur.execute(sql.SQL("DEALLOCATE {}").format(sql.Identifier(stmt_name)))
                conn.rollback()
                ev["observed"] = {"database": dbname, "query": q["id"], "cases": rows}
                max_ratio = float(case.get("max_generic_custom_cost_ratio", 10.0))
                for r in rows:
                    ratio = r.get("generic_custom_cost_ratio")
                    if ratio is not None and ratio > max_ratio:
                        ev["violations"].append(f"{r['parameter_set']}: generic/custom estimated cost ratio {ratio:.3f} > {max_ratio}")
                if case.get("warn_on_plan_change", True):
                    for r in rows:
                        if r["plan_changed"]:
                            ev["warnings"].append(f"{r['parameter_set']}: generic and custom plan shapes differ")
                status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    ev["reproduction"] = {"query_file": q["sql_file"], "query_sha256": sha256_file(sql_path), "dsn_env": dsn_env, "case": case}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
