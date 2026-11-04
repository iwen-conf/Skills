#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import os
import threading
import time
from collections import defaultdict
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    percentile,
    require_test_database_name,
    redact_dsn,
    sha256_file,
)
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore

        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _canonical_digest(rows: list[Any]) -> str:
    payload = json.dumps(rows, ensure_ascii=False, separators=(",", ":"), default=str).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def load_trace(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    ids: set[str] = set()
    for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        obj = json.loads(line)
        if not isinstance(obj, dict):
            raise ValueError(f"line {lineno}: object required")
        for key in ("session", "offset_ms"):
            if key not in obj:
                raise ValueError(f"line {lineno}: missing {key}")
        op = str(obj.get("op", "sql")).lower()
        if op not in {"sql", "begin", "commit", "rollback"}:
            raise ValueError(f"line {lineno}: unsupported op={op!r}")
        if op == "sql" and "sql" not in obj:
            raise ValueError(f"line {lineno}: missing sql for op=sql")
        event_id = str(obj.get("id") or f"line:{lineno}")
        if event_id in ids:
            raise ValueError(f"line {lineno}: duplicate event id {event_id!r}")
        ids.add(event_id)
        item = dict(obj)
        item["op"] = op
        item["_event_id"] = event_id
        deps = item.get("after", [])
        if isinstance(deps, str):
            deps = [deps]
        if not isinstance(deps, list) or any(not isinstance(x, str) for x in deps):
            raise ValueError(f"line {lineno}: after must be a string or array of event IDs")
        item["after"] = deps
        rows.append(item)
    for item in rows:
        unknown = [x for x in item["after"] if x not in ids]
        if unknown:
            raise ValueError(f"event {item['_event_id']}: unknown dependency IDs {unknown}")
    return rows


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Replay sanitized SQL workload preserving session order, timing, transaction boundaries and declared dependencies"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--replay", required=True, help="replay id from [[workload_replay]]")
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    root = Path(ns.root)
    rep = next((x for x in cfg.get("workload_replay", []) if x.get("id") == ns.replay), None)
    if not rep:
        raise SystemExit(f"workload_replay not found: {ns.replay}")
    started = time.monotonic()
    ev = evidence_base("workload_replay", f"replay.{ns.replay}")
    trace_path = (root / rep["trace_file"]).resolve()
    ev["reproduction"] = {
        "replay": ns.replay,
        "trace_file": str(trace_path),
        "trace_sha256": sha256_file(trace_path),
        "time_scale": rep.get("time_scale", 1.0),
    }
    dsn_env = cfg["database"]["dsn_env"]
    dsn = os.environ.get(dsn_env)
    ev["reproduction"]["dsn_env"] = dsn_env
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
        trace = load_trace(trace_path)
    except Exception as e:
        ev["violations"].append(repr(e))
        json_dump(Path(ns.out), finalize_evidence(ev, started, "BLOCKED"))
        return 2

    by_session: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for row in trace:
        by_session[str(row["session"])].append(row)
    for rows in by_session.values():
        rows.sort(key=lambda x: float(x["offset_ms"]))

    t0 = time.monotonic()
    lock = threading.Lock()
    observations: list[dict[str, Any]] = []
    scale = float(rep.get("time_scale", 1.0))
    timeout_ms = int(rep.get("statement_timeout_ms", 30000))
    dependency_timeout_s = float(rep.get("dependency_timeout_s", 30.0))
    max_result_rows = int(rep.get("max_result_rows", 10000))
    allowed_states = set(rep.get("allowed_sqlstates", []))
    hard_stop = threading.Event()
    completion = {str(r["_event_id"]): threading.Event() for r in trace}

    def run_session(name: str, rows: list[dict[str, Any]]) -> None:
        try:
            with psycopg.connect(dsn, autocommit=True) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        "select set_config('application_name', %s, false)", (f"sqlverify:replay:{ns.replay}:{name}",)
                    )
                    cur.execute("select set_config('statement_timeout', %s, false)", (f"{timeout_ms}ms",))
                for idx, row in enumerate(rows):
                    eid = str(row["_event_id"])
                    if hard_stop.is_set():
                        completion[eid].set()
                        continue
                    dep_failure = None
                    for dep in row.get("after", []):
                        if not completion[dep].wait(dependency_timeout_s):
                            dep_failure = f"dependency timeout waiting for {dep}"
                            break
                    if dep_failure:
                        with lock:
                            observations.append(
                                {"event_id": eid, "session": name, "index": idx, "fatal_error": dep_failure}
                            )
                        completion[eid].set()
                        hard_stop.set()
                        continue

                    due = t0 + (float(row["offset_ms"]) / 1000.0) * scale
                    delay = due - time.monotonic()
                    if delay > 0:
                        time.sleep(delay)
                    st = time.monotonic()
                    state = None
                    err = None
                    rowcount = None
                    result_digest = None
                    scalar = None
                    oracle_failures: list[str] = []
                    try:
                        op = row["op"]
                        sql = row.get("sql")
                        if op == "begin":
                            sql = "BEGIN"
                        elif op == "commit":
                            sql = "COMMIT"
                        elif op == "rollback":
                            sql = "ROLLBACK"
                        with conn.cursor() as cur:
                            cur.execute(sql, row.get("params"))
                            rowcount = cur.rowcount
                            if cur.description:
                                wants_rows = row.get("fetch") in {"one", "all"} or any(
                                    k in row for k in ("expected_result_sha256", "expected_scalar")
                                )
                                if wants_rows:
                                    fetched = cur.fetchmany(max_result_rows + 1)
                                    if len(fetched) > max_result_rows:
                                        oracle_failures.append(f"result exceeds max_result_rows={max_result_rows}")
                                        fetched = fetched[:max_result_rows]
                                    result_digest = _canonical_digest(fetched)
                                    if fetched and fetched[0]:
                                        scalar = fetched[0][0]
                            if "expected_rowcount" in row and rowcount != int(row["expected_rowcount"]):
                                oracle_failures.append(f"rowcount={rowcount} expected={row['expected_rowcount']}")
                            if "expected_result_sha256" in row and result_digest != row["expected_result_sha256"]:
                                oracle_failures.append(
                                    f"result_sha256={result_digest} expected={row['expected_result_sha256']}"
                                )
                            if "expected_scalar" in row and scalar != row["expected_scalar"]:
                                oracle_failures.append(f"scalar={scalar!r} expected={row['expected_scalar']!r}")
                    except Exception as e:
                        state = getattr(e, "sqlstate", None)
                        err = repr(e)
                        try:
                            conn.rollback()
                        except Exception as cleanup_error:
                            err += f"; rollback failed: {cleanup_error!r}"
                            hard_stop.set()
                        if not state:
                            hard_stop.set()
                    obs = {
                        "event_id": eid,
                        "session": name,
                        "index": idx,
                        "op": row["op"],
                        "after": row.get("after", []),
                        "scheduled_offset_ms": float(row["offset_ms"]),
                        "start_offset_ms": round((st - t0) * 1000, 3),
                        "elapsed_ms": round((time.monotonic() - st) * 1000, 3),
                        "sqlstate": state,
                        "error": err,
                        "rowcount": rowcount,
                        "result_sha256": result_digest,
                        "scalar": scalar,
                        "oracle_failures": oracle_failures,
                        "tag": row.get("tag"),
                    }
                    with lock:
                        observations.append(obs)
                    completion[eid].set()
        except Exception as e:
            with lock:
                observations.append({"session": name, "fatal_error": repr(e)})
            hard_stop.set()
            # Unblock dependants so the run can terminate and report the root failure.
            for row in rows:
                completion[str(row["_event_id"])].set()

    threads = [threading.Thread(target=run_session, args=(n, r), daemon=True) for n, r in sorted(by_session.items())]
    for t in threads:
        t.start()
    max_trace_offset = max((float(x["offset_ms"]) for x in trace), default=0)
    max_runtime = float(rep.get("max_runtime_s", max(30.0, (max_trace_offset / 1000.0) * scale + 30)))
    deadline = time.monotonic() + max_runtime
    for t in threads:
        t.join(max(0, deadline - time.monotonic()))
    blockers = []
    if any(t.is_alive() for t in threads):
        hard_stop.set()
        blockers.append("replay exceeded max_runtime_s; fixture reconstruction is required")

    states = [state for x in observations if isinstance(state := x.get("sqlstate"), str) and state]
    unexpected = sorted(set(states) - allowed_states)
    if unexpected:
        ev["violations"].append(f"unexpected SQLSTATEs: {unexpected}")
    fatal = [x for x in observations if x.get("fatal_error")]
    if fatal:
        blockers.append(f"fatal replay session/dependency errors: {len(fatal)}")
    if any(x.get("error") and not x.get("sqlstate") for x in observations):
        blockers.append("replay event did not yield an observable SQL outcome")
    if len([x for x in observations if "index" in x]) != len(trace) or not trace:
        blockers.append("replay did not execute every scheduled event")
    oracle_failures = [(x.get("event_id"), y) for x in observations for y in (x.get("oracle_failures") or [])]
    for eid, msg in oracle_failures[:100]:
        ev["violations"].append(f"event {eid}: {msg}")
    if len(oracle_failures) > 100:
        ev["violations"].append(f"additional replay oracle failures omitted: {len(oracle_failures) - 100}")

    lat = [float(x["elapsed_ms"]) for x in observations if isinstance(x.get("elapsed_ms"), (int, float))]
    schedule_lag = [
        max(0.0, float(x["start_offset_ms"]) - float(x["scheduled_offset_ms"]) * scale)
        for x in observations
        if "start_offset_ms" in x
    ]
    counts = {"40P01": 0, "40001": 0}
    for s in states:
        counts[s] = counts.get(s, 0) + 1
    ev["observed"] = {
        "database": dbname,
        "sessions": len(by_session),
        "events": len(trace),
        "executed_events": len(
            [
                x
                for x in observations
                if "index" in x and not x.get("fatal_error") and (not x.get("error") or x.get("sqlstate"))
            ]
        ),
        "transaction_control_events": sum(1 for x in trace if x["op"] != "sql"),
        "dependency_edges": sum(len(x.get("after", [])) for x in trace),
        "oracle_failure_count": len(oracle_failures),
        "sqlstate_counts": counts,
        "latency_ms": {
            "p50": percentile(lat, 0.5),
            "p95": percentile(lat, 0.95),
            "p99": percentile(lat, 0.99),
            "max": max(lat) if lat else None,
        },
        "schedule_lag_ms": {
            "p95": percentile(schedule_lag, 0.95),
            "p99": percentile(schedule_lag, 0.99),
            "max": max(schedule_lag) if schedule_lag else None,
        },
        "observations": observations[-10000:],
        "blockers": blockers,
    }
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
