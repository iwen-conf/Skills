#!/usr/bin/env python3
"""Business transaction reliability lab.

Two jobs:

1) Static transaction-catalog analysis
   Model business operations as ordered resource acquisitions and detect pairwise
   lock-order inversions, risky external calls inside transactions, and missing
   retry/idempotency/latency contracts.

2) Deterministic PostgreSQL transaction scenarios
   Run multiple real database sessions with explicit wait/signal barriers so a
   test can force important interleavings (including deadlocks), while sampling
   pg_stat_activity + pg_blocking_pids() to capture the wait-for graph.

The runtime path intentionally requires Psycopg 3. The verification toolchain
bootstrapper can install the pinned test-only adapter when this capability is
selected.
"""
from __future__ import annotations

import argparse
import json
import sys
import threading
import time
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

RETRYABLE_PG_STATES = {"40001", "40P01"}


def load_toml(path: Path) -> dict[str, Any]:
    with path.open("rb") as f:
        return tomllib.load(f)


def pairs(seq: list[str]) -> set[tuple[str, str]]:
    out: set[tuple[str, str]] = set()
    for i, left in enumerate(seq):
        for right in seq[i + 1 :]:
            if left != right:
                out.add((left, right))
    return out


def analyze_catalog(data: dict[str, Any]) -> dict[str, Any]:
    ops = data.get("operation", [])
    findings: list[dict[str, Any]] = []
    matrix: list[dict[str, Any]] = []

    normalized: dict[str, dict[str, Any]] = {}
    for raw in ops:
        op_id = str(raw.get("id", "")).strip()
        if not op_id:
            findings.append({"severity": "error", "code": "TXN-CATALOG-ID", "message": "operation missing id"})
            continue
        resources: list[str] = []
        lock_resources: list[str] = []
        lock_modes = {"write", "update", "for_update", "for_share", "exclusive", "lock", "insert", "delete"}
        for r in raw.get("resources", []):
            if isinstance(r, str):
                name, mode, acquire = r, "write", True
            elif isinstance(r, dict) and r.get("name"):
                name = str(r["name"])
                mode = str(r.get("mode", "write")).lower()
                acquire = bool(r.get("acquire", mode in lock_modes))
            else:
                continue
            resources.append(name)
            if acquire:
                lock_resources.append(name)
                if isinstance(r, dict):
                    multiplicity = str(r.get("multiplicity", "one")).lower()
                    key_order = str(r.get("key_order", "")).lower()
                    if multiplicity in {"many", "multiple", "dynamic"} and key_order not in {"canonical", "sorted", "ascending", "stable"}:
                        findings.append({
                            "severity": "critical" if bool(raw.get("critical", False)) else "high",
                            "code": "TXN-DYNAMIC-KEY-ORDER",
                            "operation": op_id,
                            "resource": name,
                            "message": "operation can lock multiple instances of the same resource without a documented canonical key order; test mirrored inputs (A→B vs B→A) and sort lock acquisition deterministically",
                        })
        normalized[op_id] = {
            "id": op_id,
            "resources": resources,
            "lock_resources": lock_resources,
            "order_pairs": pairs(lock_resources),
            "isolation": str(raw.get("isolation", "")).lower(),
            "retry_sqlstates": {str(x) for x in raw.get("retry_sqlstates", [])},
            "external_calls": list(raw.get("external_calls", [])),
            "idempotency": bool(raw.get("idempotency", False)),
            "max_duration_ms": raw.get("max_duration_ms"),
            "critical": bool(raw.get("critical", False)),
        }

        if not raw.get("isolation"):
            findings.append({"severity": "warning", "code": "TXN-ISOLATION-UNSPECIFIED", "operation": op_id,
                             "message": "transaction isolation is not documented"})
        if raw.get("max_duration_ms") is None:
            findings.append({"severity": "warning", "code": "TXN-DURATION-UNBOUNDED", "operation": op_id,
                             "message": "no transaction-duration budget is documented"})
        if raw.get("external_calls"):
            findings.append({"severity": "high", "code": "TXN-EXTERNAL-CALL", "operation": op_id,
                             "message": "external/network call is modeled inside the transaction; verify locks are not held across remote latency"})
            if not raw.get("idempotency", False):
                findings.append({"severity": "high", "code": "TXN-NONIDEMPOTENT-EXTERNAL", "operation": op_id,
                                 "message": "transaction includes an external call but no idempotency contract is recorded"})
        if len(lock_resources) >= 2 and "40P01" not in {str(x) for x in raw.get("retry_sqlstates", [])}:
            findings.append({"severity": "warning", "code": "TXN-DEADLOCK-RETRY-UNSPECIFIED", "operation": op_id,
                             "message": "operation acquires multiple modeled resources but deadlock retry policy (40P01) is not documented"})
        if str(raw.get("isolation", "")).lower() in {"serializable", "repeatable read", "repeatable_read"}:
            if "40001" not in {str(x) for x in raw.get("retry_sqlstates", [])}:
                findings.append({"severity": "high", "code": "TXN-SERIALIZATION-RETRY-UNSPECIFIED", "operation": op_id,
                                 "message": "isolation can produce serialization failures but retry policy (40001) is not documented"})

    ids = sorted(normalized)
    for i, a_id in enumerate(ids):
        a = normalized[a_id]
        for b_id in ids[i + 1 :]:
            b = normalized[b_id]
            shared = sorted(set(a["lock_resources"]) & set(b["lock_resources"]))
            inversions: list[tuple[str, str]] = []
            if len(shared) >= 2:
                for x in shared:
                    for y in shared:
                        if x == y:
                            continue
                        if (x, y) in a["order_pairs"] and (y, x) in b["order_pairs"]:
                            item = (x, y)
                            if item not in inversions and (y, x) not in inversions:
                                inversions.append(item)
            risk = "none"
            if shared:
                risk = "contention"
            if inversions:
                risk = "deadlock-order-inversion"
                findings.append({
                    "severity": "critical" if a["critical"] or b["critical"] else "high",
                    "code": "TXN-LOCK-ORDER-INVERSION",
                    "operations": [a_id, b_id],
                    "shared_resources": shared,
                    "inversions": [list(x) for x in inversions],
                    "message": "modeled business transactions acquire shared resources in opposite orders; construct a deterministic interleaving test",
                })
            matrix.append({"a": a_id, "b": b_id, "shared_resources": shared, "risk": risk,
                           "inversions": [list(x) for x in inversions]})

    return {
        "operations": len(normalized),
        "matrix": matrix,
        "findings": findings,
        "summary": {
            "critical": sum(1 for x in findings if x["severity"] == "critical"),
            "high": sum(1 for x in findings if x["severity"] == "high"),
            "warning": sum(1 for x in findings if x["severity"] == "warning"),
            "error": sum(1 for x in findings if x["severity"] == "error"),
        },
    }


def validate_scenario(data: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    sessions = data.get("session", [])
    if not sessions:
        errors.append("scenario must define at least one [[session]]")
        return errors
    names: set[str] = set()
    signals: set[str] = set()
    waits: set[str] = set()
    for s in sessions:
        name = str(s.get("name", "")).strip()
        if not name:
            errors.append("session missing name")
        elif name in names:
            errors.append(f"duplicate session name: {name}")
        names.add(name)
        if not s.get("steps"):
            errors.append(f"session {name or '<unnamed>'} has no steps")
        for step in s.get("steps", []):
            if not any(k in step for k in ("sql", "sleep_ms")):
                errors.append(f"session {name}: step must contain sql or sleep_ms")
            sig = step.get("signal")
            if sig:
                signals.add(str(sig))
            for w in step.get("wait_for", []):
                waits.add(str(w))
    missing = sorted(waits - signals)
    for m in missing:
        errors.append(f"wait_for references signal never emitted: {m}")
    return errors


@dataclass
class StepResult:
    index: int
    sql: str | None
    elapsed_ms: float
    ok: bool
    sqlstate: str | None = None
    error: str | None = None


@dataclass
class SessionResult:
    name: str
    pid: int | None = None
    started_at: float = 0.0
    elapsed_ms: float = 0.0
    committed: bool = False
    sqlstate: str | None = None
    error: str | None = None
    steps: list[StepResult] = field(default_factory=list)


def run_postgres_scenario(data: dict[str, Any], dsn: str) -> tuple[int, dict[str, Any]]:
    try:
        import psycopg  # type: ignore
    except Exception as e:  # pragma: no cover - runtime dependency
        return 2, {"error": "Psycopg 3 is required for pg-run", "detail": str(e),
                   "hint": "Use verification_toolchain.py with capability business-transaction-postgres."}

    validation = validate_scenario(data)
    if validation:
        return 2, {"error": "invalid scenario", "details": validation}

    scenario = data.get("scenario", {})
    expect = data.get("expect", {})
    barrier_timeout_s = float(scenario.get("barrier_timeout_s", 10.0))
    observe_interval_ms = int(scenario.get("observe_interval_ms", 50))
    statement_timeout_ms = int(scenario.get("statement_timeout_ms", 10000))
    lock_timeout_ms = int(scenario.get("lock_timeout_ms", 0))
    required_sqlstates = {str(x) for x in expect.get("required_sqlstates", [])}
    allowed_sqlstates = {str(x) for x in expect.get("allowed_sqlstates", [])} | required_sqlstates
    forbid_sqlstates = {str(x) for x in expect.get("forbid_sqlstates", [])}
    max_transaction_ms = float(expect.get("max_transaction_ms", 0) or 0)
    max_step_ms = float(expect.get("max_step_ms", 0) or 0)
    require_observer = bool(expect.get("require_observer", False))

    events: dict[str, threading.Event] = {}
    for s in data["session"]:
        for step in s.get("steps", []):
            if step.get("signal"):
                events.setdefault(str(step["signal"]), threading.Event())
            for w in step.get("wait_for", []):
                events.setdefault(str(w), threading.Event())

    lock = threading.Lock()
    session_results: dict[str, SessionResult] = {str(s["name"]): SessionResult(str(s["name"])) for s in data["session"]}
    stop_observer = threading.Event()
    snapshots: list[dict[str, Any]] = []
    observed_edges: set[tuple[int, int]] = set()
    observer_status = {"ok": False, "error": None}

    def worker(s: dict[str, Any]) -> None:
        name = str(s["name"])
        result = session_results[name]
        t0 = time.monotonic()
        result.started_at = time.time()
        conn = None
        try:
            conn = psycopg.connect(dsn, autocommit=False)
            with conn.cursor() as cur:
                isolation = str(s.get("isolation", scenario.get("isolation", "read committed"))).upper().replace("_", " ")
                if isolation not in {"READ COMMITTED", "REPEATABLE READ", "SERIALIZABLE", "READ UNCOMMITTED"}:
                    raise ValueError(f"unsupported isolation level: {isolation}")
                # SET TRANSACTION must run before the first query/data-modification command.
                cur.execute(f"SET TRANSACTION ISOLATION LEVEL {isolation}")
                cur.execute("SELECT set_config('statement_timeout', %s, true)", (f"{statement_timeout_ms}ms",))
                if lock_timeout_ms > 0:
                    cur.execute("SELECT set_config('lock_timeout', %s, true)", (f"{lock_timeout_ms}ms",))
                cur.execute("SELECT pg_backend_pid()")
                result.pid = int(cur.fetchone()[0])
                for idx, step in enumerate(s.get("steps", [])):
                    for w in step.get("wait_for", []):
                        if not events[str(w)].wait(timeout=barrier_timeout_s):
                            raise TimeoutError(f"barrier timeout waiting for {w}")
                    if step.get("sleep_ms") is not None:
                        time.sleep(float(step["sleep_ms"]) / 1000.0)
                    sql = step.get("sql")
                    if sql:
                        q0 = time.monotonic()
                        try:
                            cur.execute(str(sql))
                            elapsed = (time.monotonic() - q0) * 1000
                            result.steps.append(StepResult(idx, str(sql), elapsed, True))
                        except Exception as e:
                            elapsed = (time.monotonic() - q0) * 1000
                            state = getattr(e, "sqlstate", None)
                            result.steps.append(StepResult(idx, str(sql), elapsed, False, state, str(e)))
                            result.sqlstate = state
                            result.error = str(e)
                            raise
                    if step.get("signal"):
                        events[str(step["signal"])].set()
                conn.commit()
                result.committed = True
        except Exception as e:
            if conn is not None:
                try:
                    conn.rollback()
                except Exception:
                    pass
            if result.error is None:
                result.error = str(e)
                result.sqlstate = getattr(e, "sqlstate", None)
        finally:
            result.elapsed_ms = (time.monotonic() - t0) * 1000
            if conn is not None:
                try:
                    conn.close()
                except Exception:
                    pass

    def observer() -> None:
        conn = None
        try:
            conn = psycopg.connect(dsn, autocommit=True)
            observer_status["ok"] = True
            while not stop_observer.is_set():
                with lock:
                    pids = [r.pid for r in session_results.values() if r.pid is not None]
                if pids:
                    with conn.cursor() as cur:
                        cur.execute(
                            """
                            SELECT pid, state, wait_event_type, wait_event,
                                   EXTRACT(EPOCH FROM (clock_timestamp() - xact_start))*1000 AS xact_ms,
                                   pg_blocking_pids(pid)
                              FROM pg_stat_activity
                             WHERE pid = ANY(%s)
                            """,
                            (pids,),
                        )
                        rows = cur.fetchall()
                    snap_rows = []
                    now = time.time()
                    for pid, state, wt, we, xact_ms, blockers in rows:
                        bs = [int(x) for x in (blockers or []) if int(x) != 0]
                        for b in bs:
                            observed_edges.add((int(pid), b))
                        snap_rows.append({
                            "pid": int(pid), "state": state, "wait_event_type": wt,
                            "wait_event": we, "xact_ms": float(xact_ms) if xact_ms is not None else None,
                            "blocking_pids": bs,
                        })
                    if any(r["blocking_pids"] for r in snap_rows):
                        snapshots.append({"at": now, "rows": snap_rows})
                stop_observer.wait(observe_interval_ms / 1000.0)
        except Exception as e:
            observer_status["ok"] = False
            observer_status["error"] = str(e)
            snapshots.append({"observer_error": str(e)})
        finally:
            if conn is not None:
                try:
                    conn.close()
                except Exception:
                    pass

    observer_thread = threading.Thread(target=observer, name="pg-lock-observer", daemon=True)
    observer_thread.start()
    threads = [threading.Thread(target=worker, args=(s,), name=f"txn-{s['name']}") for s in data["session"]]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=barrier_timeout_s + statement_timeout_ms / 1000 + 5)
    alive = [t.name for t in threads if t.is_alive()]
    stop_observer.set()
    observer_thread.join(timeout=2)

    results = list(session_results.values())
    observed_states = {r.sqlstate for r in results if r.sqlstate}
    failures: list[str] = []
    non_sql_errors = [(r.name, r.error) for r in results if r.error and not r.sqlstate]
    if non_sql_errors:
        failures.append(f"non-SQLSTATE session errors: {non_sql_errors}")
    if alive:
        failures.append(f"worker threads did not finish: {alive}")
    missing_required = sorted(required_sqlstates - observed_states)
    if missing_required:
        failures.append(f"required SQLSTATE(s) not observed: {missing_required}")
    forbidden_seen = sorted(forbid_sqlstates & observed_states)
    if forbidden_seen:
        failures.append(f"forbidden SQLSTATE(s) observed: {forbidden_seen}")
    unexpected = sorted(x for x in observed_states if x not in allowed_sqlstates)
    if unexpected:
        failures.append(f"unexpected SQLSTATE(s): {unexpected}")
    if max_transaction_ms:
        over = [(r.name, r.elapsed_ms) for r in results if r.elapsed_ms > max_transaction_ms]
        if over:
            failures.append(f"transaction duration budget exceeded: {over}")
    if max_step_ms:
        over_steps = [(r.name, s.index, s.elapsed_ms) for r in results for s in r.steps if s.elapsed_ms > max_step_ms]
        if over_steps:
            failures.append(f"step duration budget exceeded: {over_steps}")
    if require_observer and not observer_status["ok"]:
        failures.append(f"lock observer required but unavailable: {observer_status.get('error') or 'unknown error'}")

    report = {
        "scenario": scenario.get("name", "unnamed"),
        "ok": not failures,
        "failures": failures,
        "observed_sqlstates": sorted(observed_states),
        "wait_for_edges": sorted([list(x) for x in observed_edges]),
        "observer": observer_status,
        "blocking_snapshots": snapshots[-100:],
        "sessions": [
            {
                "name": r.name,
                "pid": r.pid,
                "elapsed_ms": round(r.elapsed_ms, 3),
                "committed": r.committed,
                "sqlstate": r.sqlstate,
                "error": r.error,
                "steps": [vars(x) for x in r.steps],
            }
            for r in results
        ],
    }
    return (0 if not failures else 1), report


def cmd_analyze(args: argparse.Namespace) -> int:
    report = analyze_catalog(load_toml(Path(args.catalog)))
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 1 if report["summary"]["critical"] or report["summary"]["error"] else 0


def cmd_validate(args: argparse.Namespace) -> int:
    errors = validate_scenario(load_toml(Path(args.scenario)))
    if errors:
        print(json.dumps({"ok": False, "errors": errors}, ensure_ascii=False, indent=2))
        return 1
    print(json.dumps({"ok": True}, indent=2))
    return 0


def cmd_pg_run(args: argparse.Namespace) -> int:
    data = load_toml(Path(args.scenario))
    code, report = run_postgres_scenario(data, args.dsn)
    text = json.dumps(report, ensure_ascii=False, indent=2)
    if args.report:
        Path(args.report).write_text(text + "\n", encoding="utf-8")
    print(text)
    return code


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest="command", required=True)

    a = sub.add_parser("analyze", help="analyze a business transaction catalog")
    a.add_argument("--catalog", required=True)
    a.set_defaults(func=cmd_analyze)

    v = sub.add_parser("validate-scenario", help="validate deterministic transaction scenario syntax")
    v.add_argument("--scenario", required=True)
    v.set_defaults(func=cmd_validate)

    r = sub.add_parser("pg-run", help="run a deterministic scenario against real PostgreSQL")
    r.add_argument("--scenario", required=True)
    r.add_argument("--dsn", required=True, help="PostgreSQL DSN; use a disposable test database")
    r.add_argument("--report")
    r.set_defaults(func=cmd_pg_run)
    return p


if __name__ == "__main__":
    # Parse once: keeping this explicit avoids hidden global state in tests.
    parser = build_parser()
    ns = parser.parse_args()
    raise SystemExit(ns.func(ns))
