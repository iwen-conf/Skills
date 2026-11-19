#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import os
import re
import time
from pathlib import Path
from typing import Any

from plan_analyzer import analyze_explain
from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    json_load,
    require_test_database_name,
    sha256_file,
    scalar_value,
)
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore
        from psycopg import sql  # type: ignore

        return psycopg, sql
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def plan_signature(raw: Any) -> str:
    if isinstance(raw, list) and raw:
        raw = raw[0]
    root = raw.get("Plan", raw) if isinstance(raw, dict) else {}
    nodes: list[str] = []

    def walk(n: dict[str, Any]) -> None:
        nodes.append(str(n.get("Node Type", "?")))
        if n.get("Join Type"):
            nodes.append(f"join:{n['Join Type']}")
        if n.get("Index Name"):
            nodes.append(f"idx:{n['Index Name']}")
        for c in n.get("Plans", []) or []:
            walk(c)

    if isinstance(root, dict):
        walk(root)
    return " > ".join(nodes)


_SAFE_TYPE = re.compile(r"^(?:[A-Za-z_][A-Za-z0-9_$]*\.)?[A-Za-z_][A-Za-z0-9_$]*(?:\[\])?$")


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
    ap = argparse.ArgumentParser(
        description="Compare forced custom vs generic PostgreSQL plans across parameter classes"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    case = next((x for x in cfg.get("plan_sensitivity", []) if x.get("id") == ns.case), None)
    if not case:
        raise SystemExit(f"plan_sensitivity not found: {ns.case}")
    q = next((x for x in cfg.get("query", []) if x.get("id") == case.get("query")), None)
    if not q:
        raise SystemExit(f"query not found: {case.get('query')}")
    start = time.monotonic()
    ev = evidence_base("plan_sensitivity", f"plan-sensitivity.{ns.case}")
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    sql_path = root / q["sql_file"]
    rows: list[dict[str, object]] = []
    ev["observed"] = {"query": q["id"], "cases": rows}
    ev["reproduction"] = {"query_file": q["sql_file"], "dsn_env": dsn_env, "case": case}
    try:
        sql_text = convert_pg_dollar_to_prepare(sql_path.read_text(encoding="utf-8"))
        ev["reproduction"]["query_sha256"] = sha256_file(sql_path)
        param_types = [validate_param_type_name(str(t)) for t in case.get("param_types", [])]
        if not param_types:
            raise ValueError("plan_sensitivity.param_types is required for PREPARE")
        max_ratio = case.get("max_generic_custom_cost_ratio")
        if type(max_ratio) not in (int, float) or not math.isfinite(max_ratio) or max_ratio <= 0:
            raise ValueError("an explicit finite positive max_generic_custom_cost_ratio is required")
        selected_names = case.get("parameter_sets") or [p.get("name") for p in q.get("parameter_set", [])]
        parameter_sets = []
        for name in selected_names:
            ps = next((x for x in q.get("parameter_set", []) if x.get("name") == name), None)
            if not ps:
                raise ValueError(f"parameter set not found: {name}")
            pdata = json_load(root / ps["params_file"])
            params = pdata["params"] if isinstance(pdata, dict) else pdata
            parameter_sets.append((name, list(params), ps))
        if not parameter_sets:
            raise ValueError("at least one parameter set is required")
        psycopg, sql = _psycopg()
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
                ev["observed"]["database"] = dbname
                require_test_database_name(
                    dbname,
                    cfg["database"]["safe_database_pattern"],
                    bool(cfg["database"].get("allow_destructive", False)),
                )
                stmt_name = "sqlverify_plan_sensitivity"
                cur.execute(
                    sql.SQL("PREPARE {} ({}) AS {}").format(
                        sql.Identifier(stmt_name),
                        sql.SQL(", ").join(sql.SQL(t) for t in param_types),
                        sql.SQL(sql_text),
                    )
                )
                for name, params, ps in parameter_sets:
                    args = sql.SQL(", ").join(sql.Literal(v) for v in params)
                    row: dict[str, Any] = {"parameter_set": name, "params_file": ps["params_file"]}
                    rows.append(row)
                    for mode in ("force_custom_plan", "force_generic_plan"):
                        cur.execute(sql.SQL("SET LOCAL plan_cache_mode = {}").format(sql.SQL(mode)))
                        cur.execute(
                            sql.SQL("EXPLAIN (FORMAT JSON, SETTINGS TRUE) EXECUTE {} ({})").format(
                                sql.Identifier(stmt_name), args
                            )
                        )
                        raw = scalar_value(cur)
                        if (
                            not isinstance(raw, list)
                            or len(raw) != 1
                            or not isinstance(raw[0], dict)
                            or not isinstance(raw[0].get("Plan"), dict)
                        ):
                            raise ValueError("EXPLAIN did not provide a plan observation")
                        ana = analyze_explain(raw)
                        cost = ana.get("total_cost")
                        if (
                            isinstance(cost, bool)
                            or not isinstance(cost, (int, float))
                            or not math.isfinite(cost)
                            or cost < 0
                        ):
                            raise ValueError("EXPLAIN did not provide a finite estimated cost")
                        row["custom" if mode == "force_custom_plan" else "generic"] = {
                            "analysis": ana,
                            "signature": plan_signature(raw),
                            "raw_explain": raw,
                        }
                    gcost = float(row["generic"]["analysis"]["total_cost"])
                    ccost = float(row["custom"]["analysis"]["total_cost"])
                    if ccost <= 0:
                        raise ValueError("generic/custom estimated cost ratio is undefined for zero custom cost")
                    row["generic_custom_cost_ratio"] = gcost / ccost
                    row["plan_changed"] = row["generic"]["signature"] != row["custom"]["signature"]
                    if row["generic_custom_cost_ratio"] > max_ratio:
                        ev["violations"].append(
                            f"{name}: generic/custom estimated cost ratio {row['generic_custom_cost_ratio']:.3f} > {max_ratio}"
                        )
                    if case.get("warn_on_plan_change", True) and row["plan_changed"]:
                        ev["warnings"].append(f"{name}: generic and custom plan shapes differ")
                cur.execute(sql.SQL("DEALLOCATE {}").format(sql.Identifier(stmt_name)))
                conn.rollback()
                status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["warnings"].append(repr(e))
        status = "FAIL" if ev["violations"] else "BLOCKED"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
