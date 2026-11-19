#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import operator
import os
import threading
import time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    require_test_database_name,
    redact_dsn,
    sha256_file,
    scalar_value,
    SessionOutcome,
)
from sqlverify_config import load_config

OPS = {"==": operator.eq, "!=": operator.ne, ">=": operator.ge, ">": operator.gt, "<=": operator.le, "<": operator.lt}


def _psycopg():
    try:
        import psycopg  # type: ignore

        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def main() -> int:
    ap = argparse.ArgumentParser(description="Deterministic multi-session PostgreSQL transaction/barrier runner")
    ap.add_argument("--config", required=True)
    ap.add_argument("--scenario", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    sc = next((x for x in cfg.get("transaction_scenario", []) if x.get("id") == ns.scenario), None)
    if not sc:
        raise SystemExit(f"scenario not found: {ns.scenario}")
    started = time.monotonic()
    ev = evidence_base("transaction_scenario", f"txn.{ns.scenario}")
    blockers: list[str] = []
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    ev["reproduction"] = {
        "dsn_env": dsn_env,
        "scenario": ns.scenario,
        "config_sha256": sha256_file(Path(ns.config)),
        "timeout_s": sc.get("timeout_s", 15),
        "observer_sample_ms": sc.get("observer_sample_ms", 50),
        "statement_timeout_ms": sc.get("statement_timeout_ms", max(1000, int(float(sc.get("timeout_s", 15)) * 1000))),
        "lock_timeout_ms": sc.get("lock_timeout_ms", 0),
        "require_observer": sc.get("require_observer", True),
        "require_lock_wait": sc.get("require_lock_wait", False),
        "sessions": sc.get("session", []),
        "assertions": sc.get("assertion", []),
    }
    if dsn:
        ev["reproduction"]["dsn_redacted"] = redact_dsn(dsn)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2

    try:
        psycopg = _psycopg()
        with psycopg.connect(dsn, autocommit=True) as c:
            with c.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
        require_test_database_name(
            dbname, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False))
        )
    except Exception as e:
        ev["violations"].append(repr(e))
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
        return 2

    session_count = len(sc.get("session", []))
    if session_count < 2 or not sc.get("assertion"):
        ev["warnings"].append("at least two sessions and an independent assertion are required")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2
    barrier_names = {
        step["name"] for sess in sc["session"] for step in sess.get("step", []) if step.get("op") == "barrier"
    }
    barriers = {name: threading.Barrier(session_count) for name in barrier_names}
    results: dict[str, Any] = {}
    results_lock = threading.Lock()
    connections: dict[str, Any] = {}
    stop = threading.Event()
    wait_samples: list[dict[str, Any]] = []

    def observer():
        sample_ms = int(sc.get("observer_sample_ms", 50))
        try:
            with psycopg.connect(
                dsn, autocommit=True, connect_timeout=max(1, math.ceil(float(sc.get("timeout_s", 15))))
            ) as conn:
                with results_lock:
                    connections["_observer"] = conn
                with conn.cursor() as cur:
                    cur.execute(
                        "select set_config('statement_timeout', %s, false)",
                        (f"{max(1, int(float(sc.get('timeout_s', 15)) * 1000))}ms",),
                    )
                while not stop.is_set():
                    with conn.cursor() as cur:
                        cur.execute(
                            """
                        select pid, application_name, wait_event_type, wait_event, pg_blocking_pids(pid)
                        from pg_stat_activity
                        where application_name like %s
                        order by pid
                        """,
                            (f"sqlverify:{ns.scenario}:%",),
                        )
                        rows = cur.fetchall()
                        ts = time.monotonic()
                        for r in rows:
                            wait_samples.append(
                                {
                                    "t": ts,
                                    "pid": r[0],
                                    "app": r[1],
                                    "wait_event_type": r[2],
                                    "wait_event": r[3],
                                    "blocking_pids": list(r[4] or []),
                                }
                            )
                    stop.wait(sample_ms / 1000.0)
        except Exception as e:
            with results_lock:
                results["_observer_error"] = repr(e)
        finally:
            with results_lock:
                connections.pop("_observer", None)

    def worker(sess: dict[str, Any]):
        name = sess["name"]
        rec: SessionOutcome = {"sqlstates": [], "steps": [], "error": None}
        try:
            with psycopg.connect(
                dsn, autocommit=True, connect_timeout=max(1, math.ceil(float(sc.get("timeout_s", 15))))
            ) as conn:
                with results_lock:
                    connections[name] = conn
                with conn.cursor() as cur:
                    cur.execute(
                        "select set_config('application_name', %s, false)", (f"sqlverify:{ns.scenario}:{name}",)
                    )
                    stmt_ms = int(sc.get("statement_timeout_ms", max(1000, int(float(sc.get("timeout_s", 15)) * 1000))))
                    cur.execute("select set_config('statement_timeout', %s, false)", (f"{stmt_ms}ms",))
                    lock_ms = int(sc.get("lock_timeout_ms", 0))
                    if lock_ms > 0:
                        cur.execute("select set_config('lock_timeout', %s, false)", (f"{lock_ms}ms",))
                for idx, step in enumerate(sess.get("step", [])):
                    op = step["op"]
                    t0 = time.monotonic()
                    try:
                        if op == "barrier":
                            barriers[step["name"]].wait(timeout=float(sc.get("timeout_s", 15)))
                        elif op == "sleep_ms":
                            time.sleep(float(step["value"]) / 1000.0)
                        elif op == "sql":
                            with conn.cursor() as cur:
                                cur.execute(step["sql"])
                                row = cur.fetchone() if cur.description else None
                                rec["steps"].append(
                                    {
                                        "index": idx,
                                        "op": op,
                                        "row": row,
                                        "elapsed_ms": round((time.monotonic() - t0) * 1000, 3),
                                    }
                                )
                                continue
                        else:
                            raise ValueError(f"unknown op {op}")
                        rec["steps"].append(
                            {"index": idx, "op": op, "elapsed_ms": round((time.monotonic() - t0) * 1000, 3)}
                        )
                    except Exception as e:
                        state = getattr(e, "sqlstate", None)
                        if state:
                            rec["sqlstates"].append(state)
                        else:
                            rec["error"] = repr(e)
                        rec["steps"].append(
                            {
                                "index": idx,
                                "op": op,
                                "error": repr(e),
                                "sqlstate": state,
                                "elapsed_ms": round((time.monotonic() - t0) * 1000, 3),
                            }
                        )
                        try:
                            with conn.cursor() as cur:
                                cur.execute("ROLLBACK")
                        except Exception as cleanup_error:
                            rec["cleanup_error"] = repr(cleanup_error)
                        if not state or not step.get("continue_on_error", False):
                            break
        except Exception as e:
            rec["error"] = repr(e)
        with results_lock:
            connections.pop(name, None)
            results[name] = rec

    obs_thread = threading.Thread(target=observer, daemon=True)
    obs_thread.start()
    threads = [threading.Thread(target=worker, args=(s,), daemon=True) for s in sc["session"]]
    timeout = float(sc.get("timeout_s", 15))
    deadline = time.monotonic() + timeout
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=max(0, deadline - time.monotonic()))
    hung = [sc["session"][i]["name"] for i, t in enumerate(threads) if t.is_alive()]
    stop.set()
    obs_thread.join(timeout=2)
    if hung or obs_thread.is_alive():
        blockers.append(
            f"execution did not complete before deadline: sessions={hung}, observer={obs_thread.is_alive()}"
        )
        cleanup_deadline = time.monotonic() + 5
        with results_lock:
            pending_connections = list(connections.items())
        for name, conn in pending_connections:
            try:
                remaining = cleanup_deadline - time.monotonic()
                if remaining <= 0:
                    raise TimeoutError("connection cancellation budget exhausted")
                conn.cancel_safe(timeout=remaining)
            except Exception as exc:
                blockers.append(f"cancel {name}: {exc!r}")
        for t in [*threads, obs_thread]:
            t.join(timeout=max(0, cleanup_deadline - time.monotonic()))
        if any(t.is_alive() for t in [*threads, obs_thread]):
            blockers.append("runner threads remain active; fixture reconstruction is required")
    for name, rec in results.items():
        if isinstance(rec, dict) and (rec.get("error") or rec.get("cleanup_error")):
            blockers.append(f"session {name} harness/cleanup error: {rec}")
    if sc.get("require_observer", True) and not wait_samples:
        blockers.append("lock observer captured no target-session samples; required observer evidence is missing")
    if sc.get("require_lock_wait", False) and not any(
        x.get("wait_event_type") == "Lock" or x.get("blocking_pids") for x in wait_samples
    ):
        blockers.append("required lock contention was not observed; scenario did not prove the intended interleaving")

    all_states = sorted({st for k, v in results.items() if isinstance(v, dict) for st in v.get("sqlstates", [])})
    required_states = set(sc.get("required_sqlstates", []))
    allowed_states = set(sc.get("allowed_sqlstates", [])) | required_states
    if not blockers and required_states - set(all_states):
        ev["violations"].append(f"required SQLSTATEs not observed: {sorted(required_states - set(all_states))}")
    unexpected = set(all_states) - allowed_states
    if not hung and unexpected:
        ev["violations"].append(f"unexpected SQLSTATEs observed: {sorted(unexpected)}")
    if sc.get("require_observer", True) and results.get("_observer_error"):
        blockers.append(f"lock observer failed: {results['_observer_error']}")

    assertions = []
    try:
        if hung:
            raise RuntimeError("assertions cannot prove the terminal state of an unfinished scenario")
        with psycopg.connect(dsn, autocommit=True) as conn:
            with conn.cursor() as cur:
                assertion_timeout_ms = int(sc.get("assertion_timeout_ms", 5000))
                cur.execute("select set_config('statement_timeout', %s, false)", (f"{assertion_timeout_ms}ms",))
                for a in sc.get("assertion", []):
                    cur.execute(a["sql"])
                    actual = scalar_value(cur)
                    op = a.get("op", "==")
                    expected = a["expected"]
                    passed = OPS[op](actual, expected)
                    assertions.append(
                        {"id": a["id"], "actual": actual, "op": op, "expected": expected, "passed": passed}
                    )
                    if not passed:
                        ev["violations"].append(f"assertion {a['id']} failed: {actual!r} {op} {expected!r}")
    except Exception as e:
        blockers.append(f"assertion execution failed: {e!r}")

    waits = [x for x in wait_samples if x.get("wait_event_type")]
    lock_waits = [x for x in wait_samples if x.get("wait_event_type") == "Lock" or x.get("blocking_pids")]
    # Approximate longest continuous lock-wait interval, splitting gaps larger than 2.5 samples.
    by_app: dict[str, list[float]] = {}
    for x in lock_waits:
        by_app.setdefault(x["app"], []).append(float(x["t"]))
    max_wait_ms = 0.0
    sample_s = int(sc.get("observer_sample_ms", 50)) / 1000.0
    for times in by_app.values():
        times = sorted(times)
        if not times:
            continue
        start_t = prev = times[0]
        for sample_time in times[1:]:
            if sample_time - prev > sample_s * 2.5:
                max_wait_ms = max(max_wait_ms, (prev - start_t + sample_s) * 1000)
                start_t = sample_time
            prev = sample_time
        max_wait_ms = max(max_wait_ms, (prev - start_t + sample_s) * 1000)

    ev["observed"] = {
        "database": dbname,
        "sqlstates": all_states,
        "sessions": results,
        "assertions": assertions,
        "assertions_passed": sum(1 for a in assertions if a["passed"]),
        "assertions_total": len(assertions),
        "wait_sample_count": len(wait_samples),
        "waiting_sample_count": len(waits),
        "lock_waiting_sample_count": len(lock_waits),
        "max_lock_wait_ms": round(max_wait_ms, 3),
        "wait_samples": wait_samples[-5000:],
        "blockers": blockers,
    }
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
