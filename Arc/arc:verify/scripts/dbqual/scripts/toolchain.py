#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import time
from pathlib import Path

from sqlverify_common import run, which, redact_dsn, evidence_base, finalize_evidence, json_dump


def version(cmd: list[str]) -> str | None:
    try:
        cp = run(cmd, timeout=5)
        text = (cp.stdout or cp.stderr).strip()
        return text.splitlines()[0] if cp.returncode == 0 and text else None
    except Exception:
        return None


def doctor(dsn_env: str | None) -> dict:
    checks = []
    for name, cmd in [("pgbench", ["pgbench", "--version"]), ("psql", ["psql", "--version"])]:
        path = which(name)
        checks.append({"name": name, "required": True, "ok": bool(path), "path": path, "version": version(cmd) if path else None})
    try:
        import psycopg  # type: ignore
        checks.append({"name": "psycopg", "required": True, "ok": True, "version": getattr(psycopg, "__version__", None)})
    except Exception as e:
        checks.append({"name": "psycopg", "required": True, "ok": False, "error": str(e), "install": "python -m pip install 'psycopg[binary]>=3.2,<4'"})
    optional = [
        ("pgbadger", ["pgbadger", "--version"]),
        ("docker", ["docker", "--version"]),
        ("toxiproxy-cli", ["toxiproxy-cli", "-version"]),
        ("sqlancer", ["sqlancer", "--help"]),
        ("java", ["java", "-version"]),
        ("clojure", ["clojure", "-Sdescribe"]),
        ("pg_amcheck", ["pg_amcheck", "--version"]),
        ("pg_verifybackup", ["pg_verifybackup", "--version"]),
        ("pg_isready", ["pg_isready", "--version"]),
    ]
    for name, cmd in optional:
        path=which(name); checks.append({"name":name,"required":False,"ok":bool(path),"path":path,"version":version(cmd) if path else None})
    checks.append({"name": "python", "required": True, "ok": sys.version_info >= (3, 11), "version": sys.version.split()[0]})

    db = None
    if dsn_env:
        dsn = os.environ.get(dsn_env)
        if not dsn:
            db = {"ok": False, "error": f"environment variable {dsn_env} is not set"}
        else:
            env = {"PGDATABASE": dsn}
            cp = run(["psql", "-X", "-At", "-c", "select current_database(), current_setting('server_version_num'), version()"], env=env, timeout=10) if which("psql") else None
            db = {"ok": bool(cp and cp.returncode == 0), "dsn": redact_dsn(dsn), "stdout": cp.stdout.strip() if cp else None, "stderr": cp.stderr.strip() if cp else None}

    ok = all(c["ok"] for c in checks if c["required"]) and (db is None or db["ok"])
    return {"status": "PASS" if ok else "FAIL", "checks": checks, "database": db}


def main() -> int:
    ap = argparse.ArgumentParser(description="SQL verification toolchain doctor")
    ap.add_argument("--dsn-env")
    ap.add_argument("--out")
    ns = ap.parse_args()
    started = time.monotonic()
    observed = doctor(ns.dsn_env)
    ev = evidence_base("toolchain", "toolchain.doctor")
    ev["observed"] = observed
    if observed["status"] != "PASS":
        ev["violations"].append("one or more required toolchain/database preflight checks failed")
    finalize_evidence(ev, started, observed["status"])
    if ns.out:
        json_dump(Path(ns.out), ev)
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if observed["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
