#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, math, os, subprocess, tempfile, time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    require_test_database_name,
    redact_dsn,
    output_tail,
    stop_process_tree,
    wait_process,
)


def _psycopg():
    try:
        import psycopg  # type: ignore

        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def snap(conn) -> dict[str, Any]:
    out = {}
    with conn.cursor() as cur:
        cur.execute("""select count(*), count(*) filter(where state='active'), count(*) filter(where state='idle in transaction'),
                    count(*) filter(where wait_event_type='Lock'),
                    coalesce(max(extract(epoch from (now()-xact_start))*1000) filter(where xact_start is not null),0)
                    from pg_stat_activity where datname=current_database()""")
        r = cur.fetchone()
        out.update(
            {
                "connections": r[0],
                "active": r[1],
                "idle_in_tx": r[2],
                "lock_waiters": r[3],
                "max_xact_age_ms": float(r[4] or 0),
            }
        )
        cur.execute(
            "select xact_commit,xact_rollback,temp_files,temp_bytes,deadlocks,blks_read,blks_hit from pg_stat_database where datname=current_database()"
        )
        r = cur.fetchone()
        out["db"] = {
            "xact_commit": r[0],
            "xact_rollback": r[1],
            "temp_files": r[2],
            "temp_bytes": r[3],
            "deadlocks": r[4],
            "blks_read": r[5],
            "blks_hit": r[6],
        }
        try:
            cur.execute("select wal_records,wal_fpi,wal_bytes,wal_buffers_full from pg_stat_wal")
            r = cur.fetchone()
            out["wal"] = {"wal_records": r[0], "wal_fpi": r[1], "wal_bytes": float(r[2]), "wal_buffers_full": r[3]}
        except Exception:
            conn.rollback()
            out["wal"] = None
    return out


def delta(a: dict[str, Any] | None, b: dict[str, Any] | None) -> dict[str, Any] | None:
    if not a or not b:
        return None
    out = {}
    for k in set(a) | set(b):
        try:
            out[k] = float(b.get(k, 0)) - float(a.get(k, 0))
        except Exception:
            pass
    return out


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Long-running DB resource/lock observer; optionally wraps a workload command"
    )
    ap.add_argument("--dsn-env", default="DATABASE_URL")
    ap.add_argument("--safe-database-pattern", default="(_test|_bench|sqlverify)$")
    ap.add_argument("--duration-s", type=int, required=True)
    ap.add_argument("--sample-s", type=float, default=5)
    ap.add_argument("--out", required=True)
    ap.add_argument("--max-idle-in-tx", type=int, default=0)
    ap.add_argument("--max-lock-waiters", type=int)
    ap.add_argument("--max-xact-age-ms", type=float)
    ap.add_argument("command", nargs=argparse.REMAINDER)
    ns = ap.parse_args()
    started = time.monotonic()
    ev = evidence_base("longrun_monitor", "longrun.resource")
    dsn = os.environ.get(ns.dsn_env)
    ev["reproduction"] = {
        "dsn_env": ns.dsn_env,
        "duration_s": ns.duration_s,
        "sample_s": ns.sample_s,
        "command": ns.command,
    }
    if ns.duration_s <= 0 or not math.isfinite(ns.sample_s) or ns.sample_s <= 0:
        ev["warnings"].append("duration and sampling interval must be finite and positive")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2
    if dsn:
        ev["reproduction"]["dsn_redacted"] = redact_dsn(dsn)
    if not dsn:
        ev["violations"].append(f"missing env {ns.dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2
    try:
        psycopg = _psycopg()
        conn = psycopg.connect(dsn, autocommit=True)
        with conn.cursor() as cur:
            cur.execute("select current_database()")
            db = cur.fetchone()[0]
        require_test_database_name(db, ns.safe_database_pattern, False)
    except Exception as e:
        ev["violations"].append(repr(e))
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2
    proc = None
    samples = []
    blockers = []
    peaks: dict[str, float] = {}
    violated = set()
    out_file = tempfile.TemporaryFile()
    err_file = tempfile.TemporaryFile()
    try:
        if ns.command:
            cmd = ns.command[1:] if ns.command and ns.command[0] == "--" else ns.command
            proc = subprocess.Popen(cmd, stdout=out_file, stderr=err_file, start_new_session=os.name != "nt")
        deadline = time.monotonic() + ns.duration_s
        while time.monotonic() < deadline:
            x = snap(conn)
            x["t_ms"] = round((time.monotonic() - started) * 1000, 3)
            samples.append(x)
            for key, limit in (
                ("connections", None),
                ("idle_in_tx", ns.max_idle_in_tx),
                ("lock_waiters", ns.max_lock_waiters),
                ("max_xact_age_ms", ns.max_xact_age_ms),
            ):
                value = x[key]
                if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
                    raise ValueError(f"resource observation {key} is missing or invalid")
                peaks[key] = max(peaks.get(key, value), value)
                if limit is not None and value > limit and key not in violated:
                    violated.add(key)
                    ev["violations"].append(f"{key} observed {value} > configured {limit}")
                    ev["observed"].setdefault("first_violation_sample", dict(x))
            ev["observed"]["peaks"] = dict(peaks)
            time.sleep(min(ns.sample_s, max(0, deadline - time.monotonic())))
        if proc:
            result = wait_process(proc, 0)
            if result["blocked"] or result["returncode"] != 0:
                blockers.append("wrapped workload did not complete successfully within the observation window")
            ev["observed"]["command"] = result
        idle_peak = max((x["idle_in_tx"] for x in samples), default=0)
        lock_peak = max((x["lock_waiters"] for x in samples), default=0)
        age_peak = max((x["max_xact_age_ms"] for x in samples), default=0)
        conn_peak = max((x["connections"] for x in samples), default=0)
        ev["observed"].update(
            {
                "database": db,
                "samples": samples[-5000:],
                "peaks": {
                    "connections": conn_peak,
                    "idle_in_tx": idle_peak,
                    "lock_waiters": lock_peak,
                    "max_xact_age_ms": age_peak,
                },
                "db_delta": delta(
                    samples[0].get("db") if samples else None, samples[-1].get("db") if samples else None
                ),
                "wal_delta": delta(
                    samples[0].get("wal") if samples else None, samples[-1].get("wal") if samples else None
                ),
            }
        )
    except Exception as e:
        blockers.append(repr(e))
    finally:
        if proc is not None and proc.poll() is None:
            cleanup_error = stop_process_tree(proc)
            if cleanup_error:
                blockers.append(cleanup_error)
        if proc is not None:
            ev["observed"].setdefault("command", {}).update(
                {
                    "returncode": proc.returncode,
                    "stdout_tail": output_tail(out_file),
                    "stderr_tail": output_tail(err_file),
                }
            )
        out_file.close()
        err_file.close()
        try:
            conn.close()
        except Exception as e:
            blockers.append(f"connection close failed: {e!r}")
    if not samples:
        blockers.append("observer collected no samples")
    ev["observed"].update({"samples": samples[-5000:], "blockers": blockers})
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
