#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import operator
import os
import time
import threading
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    run,
    which,
    require_test_database_name,
    scalar_value,
    command_verdict,
)
from sqlverify_config import load_config

OPS = {"==": operator.eq, "!=": operator.ne, ">=": operator.ge, ">": operator.gt, "<=": operator.le, "<": operator.lt}


def _psycopg():
    try:
        import psycopg  # type: ignore

        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _cmd(x: Any) -> list[str]:
    if not isinstance(x, list) or not x or not all(isinstance(i, str) and i for i in x):
        raise ValueError("commands must be non-empty argv arrays; shell strings are intentionally unsupported")
    return list(x)


def main() -> int:
    ap = argparse.ArgumentParser(description="Verify backup integrity and rehearse an isolated restore/PITR workflow")
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ap.add_argument("--ack-isolated-environment", action="store_true")
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    root = Path(ns.root).resolve()
    case = next((x for x in cfg.get("backup_restore_rehearsal", []) if x.get("id") == ns.case), None)
    if not case:
        raise SystemExit(f"backup_restore_rehearsal not found: {ns.case}")
    start = time.monotonic()
    ev = evidence_base("backup_restore_rehearsal", f"backup-restore.{ns.case}")
    if not ns.ack_isolated_environment:
        ev["violations"].append("restore rehearsal requires --ack-isolated-environment")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    obs: dict[str, Any] = {"backup_verification": None, "restore": None, "assertions": [], "blockers": []}
    blockers = obs["blockers"]
    cleanup_cmd = case.get("cleanup_command")
    try:
        backup_path = root / case.get("backup_path", "") if case.get("backup_path") else None
        if case.get("verify_backup", True):
            if not backup_path or not backup_path.exists():
                raise RuntimeError(f"backup_path not found: {backup_path}")
            if not which("pg_verifybackup"):
                raise RuntimeError("pg_verifybackup not found")
            cmd = ["pg_verifybackup", str(backup_path)]
            if case.get("pg_verifybackup_args"):
                cmd[1:1] = list(case["pg_verifybackup_args"])
            cp = run(cmd, timeout=float(case.get("verify_timeout_s", 900)))
            obs["backup_verification"] = {
                "returncode": cp.returncode,
                "stdout_tail": cp.stdout[-4000:],
                "stderr_tail": cp.stderr[-4000:],
            }
            verdict = command_verdict(cp.returncode, case.get("oracle_failure_returncodes", []))
            obs["backup_verification"]["verdict"] = verdict
            if verdict == "FAIL":
                ev["violations"].append(f"pg_verifybackup assertion failed rc={cp.returncode}")
            if verdict != "PASS":
                raise RuntimeError("backup integrity was not verified; restore was not started")
        restore_cmd = _cmd(case["restore_command"])
        t0 = time.monotonic()
        cp = run(restore_cmd, cwd=root, timeout=float(case.get("restore_command_timeout_s", 1800)))
        if cp.returncode != 0:
            raise RuntimeError(f"restore command failed rc={cp.returncode}; stderr={cp.stderr[-2000:]}")
        obs["restore"] = {"command_rc": cp.returncode, "healthchecks": []}
        # Healthcheck can be a command (preferred for container/cluster bring-up) or direct DB connect.
        hc_timeout = float(case.get("healthcheck_timeout_s", 180))
        hc_poll = float(case.get("healthcheck_poll_s", 2))
        if not all(math.isfinite(value) and value > 0 for value in (hc_timeout, hc_poll)):
            raise ValueError("healthcheck deadline and poll interval must be finite and positive")
        if not case.get("healthcheck_command"):
            envname = case.get("restored_dsn_env")
            dsn = os.environ.get(envname or "")
            if not dsn:
                raise RuntimeError(f"missing restored DSN env {envname}")
            psycopg = _psycopg()
        deadline = time.monotonic() + hc_timeout
        healthy = False
        hc_attempts = 0
        hc_last = ""
        while time.monotonic() < deadline:
            hc_attempts += 1
            if case.get("healthcheck_command"):
                hp = run(
                    _cmd(case["healthcheck_command"]),
                    cwd=root,
                    timeout=min(deadline - time.monotonic(), float(case.get("healthcheck_command_timeout_s", 15))),
                )
                verdict = command_verdict(hp.returncode, case.get("oracle_failure_returncodes", []))
                hc_last = (hp.stdout + "\n" + hp.stderr)[-2000:]
                obs["restore"]["healthchecks"].append(
                    {"returncode": hp.returncode, "verdict": verdict, "output_tail": hc_last}
                )
                if verdict == "BLOCKED":
                    raise RuntimeError(f"healthcheck did not yield a declared observation: rc={hp.returncode}")
                healthy = verdict == "PASS"
            else:
                with psycopg.connect(dsn, connect_timeout=int(max(1, min(10, hc_poll + 1)))) as conn:
                    with conn.cursor() as cur:
                        cur.execute("select 1")
                        actual = scalar_value(cur)
                        healthy = actual == 1
                obs["restore"]["healthchecks"].append({"actual": actual, "healthy": healthy})
            if healthy:
                break
            threading.Event().wait(max(0, min(hc_poll, deadline - time.monotonic())))
        rto_s = time.monotonic() - t0
        obs["restore"].update(
            {"rto_s": rto_s, "healthy": healthy, "healthcheck_attempts": hc_attempts, "healthcheck_last": hc_last}
        )
        if not healthy:
            ev["violations"].append("restored target did not become healthy")
        if case.get("max_rto_s") is not None and rto_s > float(case["max_rto_s"]):
            ev["violations"].append(f"RTO {rto_s:.3f}s > {case['max_rto_s']}s")
        if healthy and case.get("restored_dsn_env"):
            envname = case["restored_dsn_env"]
            dsn = os.environ.get(envname)
            if not dsn:
                raise RuntimeError(f"missing restored DSN env {envname}")
            psycopg = _psycopg()
            with psycopg.connect(dsn, autocommit=False) as conn:
                with conn.cursor() as cur:
                    cur.execute("select current_database()")
                    dbname = cur.fetchone()[0]
                    require_test_database_name(
                        dbname,
                        case.get("safe_database_pattern", cfg["database"]["safe_database_pattern"]),
                        bool(case.get("allow_destructive", False)),
                    )
                    cur.execute("set transaction read only")
                    for a in case.get("assertion", []):
                        cur.execute(
                            "select set_config('statement_timeout', %s, true)",
                            (f"{int(a.get('statement_timeout_ms', 10000))}ms",),
                        )
                        cur.execute(a["sql"])
                        actual = scalar_value(cur)
                        op = a.get("op", "==")
                        expected = a["expected"]
                        passed = OPS[op](actual, expected)
                        obs["assertions"].append(
                            {"id": a["id"], "actual": actual, "op": op, "expected": expected, "passed": passed}
                        )
                        if not passed:
                            ev["violations"].append(f"restore assertion {a['id']} failed: {actual!r} {op} {expected!r}")
                    conn.rollback()
    except Exception as e:
        blockers.append(repr(e))
    finally:
        if cleanup_cmd:
            try:
                cp = run(_cmd(cleanup_cmd), cwd=root, timeout=float(case.get("cleanup_timeout_s", 300)))
                obs["cleanup"] = {
                    "returncode": cp.returncode,
                    "stdout_tail": cp.stdout[-2000:],
                    "stderr_tail": cp.stderr[-2000:],
                }
                if cp.returncode != 0:
                    blockers.append(f"cleanup command failed rc={cp.returncode}; fixture reconstruction is required")
            except Exception as e:
                blockers.append(f"cleanup failed: {e!r}; fixture reconstruction is required")
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    ev["observed"] = obs
    ev["reproduction"] = {"case": case}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
