#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path

from sqlverify_common import evidence_base, finalize_evidence, json_dump, run, which
from sqlverify_config import load_config


def _libpq_cli_env(dsn: str) -> tuple[list[str], dict[str, str], list[str]]:
    """Return pg_amcheck connection args/env without exposing passwords in argv.

    psycopg's libpq-compatible conninfo parser handles both URI and keyword DSNs.
    We intentionally fail closed if a password-bearing DSN cannot be parsed safely.
    """
    warnings: list[str] = []
    try:
        from psycopg.conninfo import conninfo_to_dict  # type: ignore
        info = conninfo_to_dict(dsn)
    except Exception as exc:
        lowered = dsn.lower()
        if "password=" in lowered or "://" in dsn and "@" in dsn and ":" in dsn.split("@", 1)[0]:
            raise RuntimeError(
                "refusing to pass a potentially password-bearing DSN in process argv; install psycopg 3 so conninfo can be parsed into PG* environment variables"
            ) from exc
        warnings.append("could not parse DSN with psycopg; passing non-secret DSN to pg_amcheck argv")
        return ["--dbname", dsn], {}, warnings

    env_map = {
        "host": "PGHOST", "hostaddr": "PGHOSTADDR", "port": "PGPORT",
        "user": "PGUSER", "password": "PGPASSWORD", "dbname": "PGDATABASE",
        "sslmode": "PGSSLMODE", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY",
        "sslrootcert": "PGSSLROOTCERT", "sslcrl": "PGSSLCRL",
        "connect_timeout": "PGCONNECT_TIMEOUT", "application_name": "PGAPPNAME",
        "service": "PGSERVICE", "servicefile": "PGSERVICEFILE",
        "target_session_attrs": "PGTARGETSESSIONATTRS",
    }
    env: dict[str, str] = {}
    ignored: list[str] = []
    for key, value in info.items():
        if value is None:
            continue
        target = env_map.get(key)
        if target:
            env[target] = str(value)
        elif key not in {"options", "passfile"}:
            ignored.append(key)
    if info.get("options"):
        env["PGOPTIONS"] = str(info["options"])
    if info.get("passfile"):
        env["PGPASSFILE"] = str(info["passfile"])
    if ignored:
        warnings.append("unmapped libpq options omitted from pg_amcheck environment: " + ", ".join(sorted(ignored)))
    # PGDATABASE may itself be a conninfo string in libpq, but here it is the parsed
    # database name only, so no credential is exposed in argv.
    return [], env, warnings


def main() -> int:
    ap = argparse.ArgumentParser(description="Run PostgreSQL physical/logical relation integrity verification with pg_amcheck")
    ap.add_argument("--config", required=True)
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config)); pcfg = cfg.get("physical_integrity", {})
    start = time.monotonic(); ev = evidence_base("physical_integrity", "physical.integrity")
    if not pcfg.get("enabled", False):
        ev["warnings"].append("physical integrity check disabled by config")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "NOT_RUN")); return 0
    dsn_env = pcfg.get("dsn_env", cfg["database"]["dsn_env"]); dsn = os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}"); json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    if not which("pg_amcheck"):
        ev["violations"].append("pg_amcheck not found"); json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    args = ["pg_amcheck"]
    if pcfg.get("jobs"): args += ["--jobs", str(int(pcfg["jobs"]))]
    if pcfg.get("heapallindexed", False): args.append("--heapallindexed")
    if pcfg.get("checkunique", True): args.append("--checkunique")
    if pcfg.get("parent_check", False): args.append("--parent-check")
    if pcfg.get("rootdescend", False): args.append("--rootdescend")
    for schema in pcfg.get("schema", []): args += ["--schema", schema]
    for ex in pcfg.get("exclude_schema", []): args += ["--exclude-schema", ex]
    if pcfg.get("install_missing", False): args.append("--install-missing")
    try:
        conn_args, conn_env, conn_warnings = _libpq_cli_env(dsn)
        args.extend(conn_args)
        ev["warnings"].extend(conn_warnings)
        ev["command"] = list(args)
        # Record only connection parameter names, never values that may contain secrets.
        ev["connection_environment_keys"] = sorted(conn_env)
        cp = run(args, env=conn_env, timeout=float(pcfg.get("timeout_s", 3600)))
        ev["observed"] = {"returncode": cp.returncode, "stdout_tail": cp.stdout[-12000:], "stderr_tail": cp.stderr[-12000:]}
        if cp.returncode != 0: ev["violations"].append(f"pg_amcheck failed rc={cp.returncode}")
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    ev["reproduction"] = {"dsn_env": dsn_env, "config": pcfg}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if status in {"PASS", "NOT_RUN"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
