#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import operator
import os
import threading
import time
from collections import defaultdict
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name, redact_dsn, sha256_file
from sqlverify_config import load_config

OPS = {"==": operator.eq, "!=": operator.ne, ">=": operator.ge, ">": operator.gt, "<=": operator.le, "<": operator.lt}


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _graph_cycles(edges: set[tuple[int, int]]) -> list[list[int]]:
    g: dict[int, set[int]] = defaultdict(set)
    for a, b in edges:
        if a != b:
            g[a].add(b)
    found: set[tuple[int, ...]] = set()
    nodes = sorted(set(g) | {x for vs in g.values() for x in vs})
    for start in nodes:
        stack = [(start, [start], {start})]
        while stack:
            cur, path, seen = stack.pop()
            for nxt in g.get(cur, ()):
                if nxt == start and len(path) > 1:
                    body = path[:]
                    rots = [tuple(body[i:] + body[:i]) for i in range(len(body))]
                    found.add(min(rots))
                elif nxt not in seen and len(path) < len(nodes):
                    stack.append((nxt, path + [nxt], seen | {nxt}))
    return [list(c) + [c[0]] for c in sorted(found)]


def main() -> int:
    ap = argparse.ArgumentParser(description="Industrial deterministic PostgreSQL concurrency/deadlock scenario runner")
    ap.add_argument("--config", required=True)
    ap.add_argument("--scenario", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    scenarios = cfg.get("concurrency_scenario", []) or cfg.get("transaction_scenario", [])
    sc = next((x for x in scenarios if x.get("id") == ns.scenario), None)
    if not sc:
        raise SystemExit(f"scenario not found: {ns.scenario}")

    started = time.monotonic()
    ev = evidence_base("concurrency_scenario", f"concurrency.{ns.scenario}")
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    ev["reproduction"] = {
        "dsn_env": dsn_env,
        "scenario": ns.scenario,
        "config_sha256": sha256_file(Path(ns.config)),
        "timeout_s": sc.get("timeout_s", 15),
        "observer_sample_ms": sc.get("observer_sample_ms", 20),
        "sessions": sc.get("session", []),
        "assertions": sc.get("assertion", []),
    }
    if dsn:
        ev["reproduction"]["dsn_redacted"] = redact_dsn(dsn)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED")); return 2

    try:
        psycopg = _psycopg()
        with psycopg.connect(dsn, autocommit=True) as c:
            with c.cursor() as cur:
                cur.execute("select current_database()")
                dbname = cur.fetchone()[0]
        require_test_database_name(dbname, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False)))
    except Exception as e:
        ev["violations"].append(repr(e))
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED")); return 2

    sessions = sc.get("session", [])
    session_count = len(sessions)
    if session_count < 2:
        ev["violations"].append("scenario needs >=2 sessions")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "FAIL")); return 2

    barrier_names = {step["name"] for sess in sessions for step in sess.get("step", []) if step.get("op") == "barrier"}
    barriers = {name: threading.Barrier(session_count) for name in barrier_names}
    results: dict[str, Any] = {}
    results_lock = threading.Lock()
    stop = threading.Event()
    samples: list[dict[str, Any]] = []
    observed_edges: set[tuple[int, int]] = set()
    observed_cycles: list[dict[str, Any]] = []

    def observer() -> None:
        sample_ms = int(sc.get("observer_sample_ms", 20))
        try:
            with psycopg.connect(dsn, autocommit=True) as conn:
                with conn.cursor() as cur:
                    cur.execute("select set_config('application_name', %s, false)", (f"sqlverify:{ns.scenario}:observer",))
                while not stop.is_set():
                    with conn.cursor() as cur:
                        cur.execute("""
                            select pid, application_name, state, wait_event_type, wait_event,
                                   pg_blocking_pids(pid), now()-xact_start as xact_age,
                                   now()-query_start as query_age, left(query, 500)
                            from pg_stat_activity
                            where application_name like %s and application_name <> %s
                            order by pid
                        """, (f"sqlverify:{ns.scenario}:%", f"sqlverify:{ns.scenario}:observer"))
                        rows = cur.fetchall()
                    ts = time.monotonic()
                    current_edges: set[tuple[int, int]] = set()
                    for r in rows:
                        blockers = list(r[5] or [])
                        rec = {
                            "t": ts, "pid": r[0], "app": r[1], "state": r[2],
                            "wait_event_type": r[3], "wait_event": r[4], "blocking_pids": blockers,
                            "xact_age_ms": None if r[6] is None else round(r[6].total_seconds()*1000, 3),
                            "query_age_ms": None if r[7] is None else round(r[7].total_seconds()*1000, 3),
                            "query": r[8],
                        }
                        samples.append(rec)
                        for b in blockers:
                            edge = (int(r[0]), int(b))
                            current_edges.add(edge)
                            observed_edges.add(edge)
                    # Detect a cycle only in the same observer snapshot. Unioning edges across
                    # time can manufacture a cycle that never existed concurrently.
                    cyc = _graph_cycles(current_edges)
                    if cyc:
                        observed_cycles.append({"t": ts, "cycles": cyc, "edges": [list(x) for x in sorted(current_edges)]})
                    stop.wait(sample_ms / 1000.0)
        except Exception as e:
            with results_lock:
                results["_observer_error"] = repr(e)

    def worker(sess: dict[str, Any]) -> None:
        name = sess["name"]
        rec: dict[str, Any] = {"sqlstates": [], "steps": [], "error": None}
        try:
            with psycopg.connect(dsn, autocommit=True) as conn:
                with conn.cursor() as cur:
                    cur.execute("select set_config('application_name', %s, false)", (f"sqlverify:{ns.scenario}:{name}",))
                    cur.execute("select set_config('statement_timeout', %s, false)", (f"{int(sc.get('statement_timeout_ms', 15000))}ms",))
                    if int(sc.get("lock_timeout_ms", 0)) > 0:
                        cur.execute("select set_config('lock_timeout', %s, false)", (f"{int(sc['lock_timeout_ms'])}ms",))
                    if int(sc.get("deadlock_timeout_ms", 0)) > 0:
                        # Requires suitable privilege; if unavailable, keep server default and record warning via step.
                        try:
                            cur.execute("select set_config('deadlock_timeout', %s, false)", (f"{int(sc['deadlock_timeout_ms'])}ms",))
                        except Exception:
                            conn.rollback()
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
                                cur.execute(step["sql"], step.get("params"))
                                row = cur.fetchone() if cur.description else None
                            rec["steps"].append({"index": idx, "op": op, "row": row, "elapsed_ms": round((time.monotonic()-t0)*1000,3)})
                            continue
                        else:
                            raise ValueError(f"unknown op {op}")
                        rec["steps"].append({"index": idx, "op": op, "elapsed_ms": round((time.monotonic()-t0)*1000,3)})
                    except Exception as e:
                        state = getattr(e, "sqlstate", None)
                        if state:
                            rec["sqlstates"].append(state)
                        rec["steps"].append({"index": idx, "op": op, "error": repr(e), "sqlstate": state, "elapsed_ms": round((time.monotonic()-t0)*1000,3)})
                        try:
                            conn.rollback()
                        except Exception:
                            pass
                        if not step.get("continue_on_error", False):
                            break
        except Exception as e:
            rec["error"] = repr(e)
        with results_lock:
            results[name] = rec

    obs = threading.Thread(target=observer, daemon=True); obs.start()
    threads = [threading.Thread(target=worker, args=(s,), daemon=True) for s in sessions]
    for t in threads: t.start()
    deadline = time.monotonic() + float(sc.get("timeout_s", 15))
    for t in threads:
        remaining = max(0.0, deadline-time.monotonic())
        t.join(timeout=remaining)
    hung = [sessions[i]["name"] for i,t in enumerate(threads) if t.is_alive()]
    stop.set(); obs.join(timeout=2)

    all_states = [st for k,v in results.items() if isinstance(v, dict) for st in v.get("sqlstates", [])]
    counts = {"40P01": 0, "40001": 0}
    for state in all_states:
        counts[state] = counts.get(state, 0) + 1
    if hung:
        ev["violations"].append(f"sessions did not finish before timeout: {hung}")
    if results.get("_observer_error") and sc.get("require_observer", True):
        ev["violations"].append(f"observer failed: {results['_observer_error']}")

    # Global SQLSTATE expectations.
    required = set(sc.get("required_sqlstates", []))
    allowed = set(sc.get("allowed_sqlstates", [])) | required
    missing = required - set(all_states)
    unexpected = set(all_states) - allowed
    if missing:
        ev["violations"].append(f"required SQLSTATEs not observed: {sorted(missing)}")
    if unexpected:
        ev["violations"].append(f"unexpected SQLSTATEs observed: {sorted(unexpected)}")

    # Exact/range expectations for concurrency outcomes.
    for sqlstate, field in [("40P01", "expected_deadlock_count"), ("40001", "expected_serialization_failure_count")]:
        if field in sc and counts.get(sqlstate, 0) != int(sc[field]):
            ev["violations"].append(f"{field}={sc[field]} but observed {counts.get(sqlstate,0)} ({sqlstate})")
    if "max_deadlock_count" in sc and counts.get("40P01", 0) > int(sc["max_deadlock_count"]):
        ev["violations"].append(f"deadlock count {counts.get('40P01',0)} exceeds max_deadlock_count={sc['max_deadlock_count']}")
    if "max_serialization_failure_count" in sc and counts.get("40001", 0) > int(sc["max_serialization_failure_count"]):
        ev["violations"].append(f"serialization failure count {counts.get('40001',0)} exceeds max={sc['max_serialization_failure_count']}")

    # Per-session SQLSTATE expectations.
    by_name = {s["name"]: s for s in sessions}
    for name, rec in results.items():
        if name.startswith("_") or name not in by_name or not isinstance(rec, dict):
            continue
        ss = by_name[name]
        got = set(rec.get("sqlstates", []))
        req = set(ss.get("required_sqlstates", []))
        allow = set(ss.get("allowed_sqlstates", [])) | req
        if req - got:
            ev["violations"].append(f"session {name}: required SQLSTATEs missing: {sorted(req-got)}")
        if got - allow:
            ev["violations"].append(f"session {name}: unexpected SQLSTATEs: {sorted(got-allow)}")

    lock_waits = [x for x in samples if x.get("wait_event_type") == "Lock" or x.get("blocking_pids")]
    if sc.get("require_observer", True) and not samples:
        ev["violations"].append("observer captured no target session samples")
    if sc.get("require_lock_wait", False) and not lock_waits:
        ev["violations"].append("required lock contention was not observed")
    if sc.get("require_wait_cycle", False) and not observed_cycles:
        ev["violations"].append("required wait-for graph cycle was not observed before resolution")

    # Approximate continuous wait tails by application name.
    by_app: dict[str, list[float]] = defaultdict(list)
    for x in lock_waits:
        by_app[str(x["app"])].append(float(x["t"]))
    max_wait_ms = 0.0
    sample_s = int(sc.get("observer_sample_ms", 20)) / 1000.0
    for times in by_app.values():
        times.sort(); start = prev = times[0]
        for ts in times[1:]:
            if ts-prev > sample_s*2.5:
                max_wait_ms = max(max_wait_ms, (prev-start+sample_s)*1000); start = ts
            prev = ts
        max_wait_ms = max(max_wait_ms, (prev-start+sample_s)*1000)
    if "max_lock_wait_ms" in sc and max_wait_ms > float(sc["max_lock_wait_ms"]):
        ev["violations"].append(f"max lock wait {max_wait_ms:.3f}ms exceeds {sc['max_lock_wait_ms']}ms")

    assertions = []
    try:
        with psycopg.connect(dsn, autocommit=True) as conn:
            with conn.cursor() as cur:
                cur.execute("select set_config('statement_timeout', %s, false)", (f"{int(sc.get('assertion_timeout_ms',5000))}ms",))
                for a in sc.get("assertion", []):
                    cur.execute(a["sql"], a.get("params"))
                    row = cur.fetchone(); actual = row[0] if row else None
                    op = a.get("op", "=="); expected = a.get("expected")
                    passed = OPS[op](actual, expected)
                    assertions.append({"id": a["id"], "actual": actual, "op": op, "expected": expected, "passed": passed})
                    if not passed:
                        ev["violations"].append(f"assertion {a['id']} failed: {actual!r} {op} {expected!r}")
    except Exception as e:
        ev["violations"].append(f"assertion execution failed: {e!r}")

    ev["observed"] = {
        "database": dbname,
        "sqlstate_counts": counts,
        "sessions": results,
        "assertions": assertions,
        "assertions_passed": sum(1 for a in assertions if a["passed"]),
        "assertions_total": len(assertions),
        "sample_count": len(samples),
        "lock_wait_sample_count": len(lock_waits),
        "max_lock_wait_ms": round(max_wait_ms, 3),
        "wait_for_edges": [list(x) for x in sorted(observed_edges)],
        "wait_for_cycles": observed_cycles[-50:],
        "samples": samples[-5000:],
    }
    status = "PASS" if not ev["violations"] else "FAIL"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
