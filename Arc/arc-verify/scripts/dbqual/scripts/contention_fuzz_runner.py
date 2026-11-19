#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, operator, os, random, threading, time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    require_test_database_name,
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
    ap = argparse.ArgumentParser(
        description="Seeded contention/schedule exploration for PostgreSQL business transactions"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--fuzz", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    fc = next((x for x in cfg.get("contention_fuzz", []) if x.get("id") == ns.fuzz), None)
    if not fc:
        raise SystemExit(f"contention_fuzz not found: {ns.fuzz}")
    started = time.monotonic()
    ev = evidence_base("contention_fuzz", f"fuzz.{ns.fuzz}")
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    seed = int(fc.get("seed", 1))
    runs = int(fc.get("runs", 100))
    jitter_ms = int(fc.get("jitter_ms", 20))
    rng = random.Random(seed)
    ev["reproduction"] = {
        "dsn_env": dsn_env,
        "fuzz": ns.fuzz,
        "config_sha256": sha256_file(Path(ns.config)),
        "seed": seed,
        "runs": runs,
        "jitter_ms": jitter_ms,
    }
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}")
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2
    try:
        psycopg = _psycopg()
        with psycopg.connect(dsn, autocommit=True) as c:
            with c.cursor() as cur:
                cur.execute("select current_database()")
                db = cur.fetchone()[0]
        require_test_database_name(
            db, cfg["database"]["safe_database_pattern"], bool(cfg["database"].get("allow_destructive", False))
        )
    except Exception as e:
        ev["violations"].append(repr(e))
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2

    sessions = fc.get("session", [])
    sqlstate_counts: dict[str, int] = {"40P01": 0, "40001": 0}
    failure_examples: list[dict[str, object]] = []
    elapsed = []
    blockers = []
    failed_iterations = 0
    max_examples = int(fc.get("max_failure_examples", 20))
    if len(sessions) < 2 or runs <= 0 or max_examples < 0 or not fc.get("assertion"):
        ev["warnings"].append(
            "at least two sessions, positive runs, a nonnegative example budget and independent assertions are required"
        )
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2

    def exec_sql(sql: str):
        with psycopg.connect(dsn, autocommit=True) as c:
            with c.cursor() as cur:
                cur.execute(sql)

    try:
        if fc.get("setup_sql"):
            exec_sql(fc["setup_sql"])
        for iteration in range(runs):
            if fc.get("reset_sql"):
                exec_sql(fc["reset_sql"])
            start_barrier = threading.Barrier(len(sessions))
            recs: list[SessionOutcome] = []
            lock = threading.Lock()
            schedule = {}
            # Precompute every delay from the seed, so a failing schedule can be replayed approximately.
            for s in sessions:
                schedule[s["name"]] = [rng.randint(0, jitter_ms) for _ in s.get("sql", [])]

            def worker(s: dict[str, Any]):
                name = s["name"]
                states: list[str] = []
                steps: list[dict[str, object]] = []
                try:
                    with psycopg.connect(dsn, autocommit=False) as conn:
                        with conn.cursor() as cur:
                            cur.execute(
                                "select set_config('application_name', %s, false)",
                                (f"sqlverify:fuzz:{ns.fuzz}:{name}",),
                            )
                            cur.execute(
                                "select set_config('statement_timeout', %s, false)",
                                (f"{int(fc.get('statement_timeout_ms', 5000))}ms",),
                            )
                        start_barrier.wait(timeout=float(fc.get("iteration_timeout_s", 10)))
                        for idx, sql in enumerate(s.get("sql", [])):
                            d = schedule[name][idx]
                            time.sleep(d / 1000.0)
                            st = time.monotonic()
                            try:
                                with conn.cursor() as cur:
                                    cur.execute(sql)
                                steps.append(
                                    {
                                        "index": idx,
                                        "delay_ms": d,
                                        "elapsed_ms": round((time.monotonic() - st) * 1000, 3),
                                    }
                                )
                            except Exception as e:
                                state = getattr(e, "sqlstate", None)
                                if isinstance(state, str) and state:
                                    states.append(state)
                                steps.append(
                                    {
                                        "index": idx,
                                        "delay_ms": d,
                                        "sqlstate": state,
                                        "error": repr(e),
                                        "elapsed_ms": round((time.monotonic() - st) * 1000, 3),
                                    }
                                )
                                conn.rollback()
                                break
                        else:
                            conn.commit()
                except Exception as e:
                    steps.append({"fatal": repr(e)})
                with lock:
                    recs.append({"session": name, "sqlstates": [x for x in states if x], "steps": steps})

            ts = [threading.Thread(target=worker, args=(s,), daemon=True) for s in sessions]
            it0 = time.monotonic()
            deadline = time.monotonic() + float(fc.get("iteration_timeout_s", 10))
            for t in ts:
                t.start()
            for t in ts:
                t.join(max(0, deadline - time.monotonic()))
            hung = any(t.is_alive() for t in ts)
            elapsed.append((time.monotonic() - it0) * 1000)
            if hung:
                blockers.append(f"iteration {iteration}: sessions unfinished; fixture reconstruction is required")
                break
            harness_errors = [
                step
                for r in recs
                for step in r["steps"]
                if step.get("fatal") or (step.get("error") and not step.get("sqlstate"))
            ]
            if harness_errors:
                blockers.append(f"iteration {iteration}: worker harness errors: {harness_errors}")
                break
            states = [st for r in recs for st in r["sqlstates"]]
            for st in states:
                sqlstate_counts[st] = sqlstate_counts.get(st, 0) + 1
            assertions: list[dict[str, object]] = []
            deadlock = "40P01" in states
            serialization = "40001" in states
            unexpected = sorted(set(states) - set(fc.get("allowed_sqlstates", [])))
            failed = (
                bool(unexpected)
                or (deadlock and fc.get("fail_on_deadlock", True))
                or (serialization and fc.get("fail_on_serialization", False))
            )
            example: dict[str, object] = {
                "iteration": iteration,
                "schedule_ms": schedule,
                "sessions": recs,
                "assertions": assertions,
                "hung": hung,
                "unexpected_sqlstates": unexpected,
            }
            if failed:
                failed_iterations += 1
                ev["violations"].append(f"iteration {iteration}: forbidden transaction outcomes {states}")
                ev["observed"].setdefault("first_failure", example)
                if len(failure_examples) < max_examples:
                    failure_examples.append(example)
            with psycopg.connect(dsn, autocommit=True) as conn:
                with conn.cursor() as cur:
                    for a in fc.get("assertion", []):
                        cur.execute(a["sql"])
                        actual = scalar_value(cur)
                        op = a.get("op", "==")
                        passed = OPS[op](actual, a["expected"])
                        assertions.append(
                            {"id": a["id"], "actual": actual, "expected": a["expected"], "op": op, "passed": passed}
                        )
                        if not passed:
                            ev["violations"].append(
                                f"iteration {iteration} assertion {a['id']} failed: {actual!r} {op} {a['expected']!r}"
                            )
                            ev["observed"].setdefault("first_failure", example)
                            if not failed:
                                failed = True
                                failed_iterations += 1
                                if len(failure_examples) < max_examples:
                                    failure_examples.append(example)
        if failed_iterations:
            ev["violations"].append(
                f"contention fuzz found {failed_iterations} failing schedules ({len(failure_examples)} captured)"
            )
        max_deadlocks = int(fc.get("max_deadlocks", 0))
        if sqlstate_counts.get("40P01", 0) > max_deadlocks:
            ev["violations"].append(f"deadlocks={sqlstate_counts.get('40P01', 0)} > max_deadlocks={max_deadlocks}")
        ev["observed"].update(
            {
                "database": db,
                "runs": runs,
                "completed_iterations": len(elapsed) if not blockers else max(0, len(elapsed) - 1),
                "failed_iterations": failed_iterations,
                "sqlstate_counts": sqlstate_counts,
                "failure_examples": failure_examples,
                "blockers": blockers,
                "iteration_elapsed_ms": {
                    "p95": sorted(elapsed)[int(0.95 * (len(elapsed) - 1))] if elapsed else None,
                    "max": max(elapsed) if elapsed else None,
                },
            }
        )
        status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    except Exception as e:
        blockers.append(repr(e))
        ev["observed"].update(
            {"blockers": blockers, "failed_iterations": failed_iterations, "failure_examples": failure_examples}
        )
        if failed_iterations:
            ev["violations"].append(f"contention fuzz found {failed_iterations} failing schedules before interruption")
        status = "FAIL" if ev["violations"] else "BLOCKED"
    ev["warnings"].extend(blockers)
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
