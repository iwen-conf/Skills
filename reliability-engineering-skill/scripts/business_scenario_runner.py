#!/usr/bin/env python3
"""Execute business scenarios against real project boundaries and emit evidence.

The business plan describes *what* must be true. A binding file describes *how*
that business scenario is exercised in this repository/environment. Keeping these
separate prevents infrastructure details from becoming the specification.

Supported step kinds:
  command  - argv execution without a shell
  http     - HTTP request using urllib
  sql      - PostgreSQL query via Psycopg 3 (optional dependency)
  sleep    - controlled delay

Contiguous steps with the same non-empty parallel_group run concurrently.
Cleanup steps (phase="cleanup") always run after the main execution.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import threading
import time
import tomllib
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass
from pathlib import Path
from string import Template
from typing import Any


def load_toml(path: Path) -> dict[str, Any]:
    with path.open("rb") as f:
        return tomllib.load(f)


def expand(value: Any, variables: dict[str, str]) -> Any:
    if isinstance(value, str):
        return Template(value).safe_substitute(variables)
    if isinstance(value, list):
        return [expand(x, variables) for x in value]
    if isinstance(value, dict):
        return {k: expand(v, variables) for k, v in value.items()}
    return value


def json_path(value: Any, path: str) -> Any:
    cur = value
    if not path:
        return cur
    for part in path.split("."):
        if isinstance(cur, list):
            cur = cur[int(part)]
        elif isinstance(cur, dict):
            cur = cur[part]
        else:
            raise KeyError(f"cannot descend into {type(cur).__name__} at {part}")
    return cur


@dataclass
class StepEvidence:
    name: str
    kind: str
    ok: bool
    elapsed_ms: float
    detail: dict[str, Any]
    error: str | None = None


def run_command(step: dict[str, Any], variables: dict[str, str], default_timeout: float) -> StepEvidence:
    started = time.monotonic()
    argv = expand(step.get("argv", []), variables)
    if not isinstance(argv, list) or not argv:
        return StepEvidence(str(step.get("name", "command")), "command", False, 0, {}, "argv is required")
    env = os.environ.copy()
    env.update({str(k): str(v) for k, v in expand(step.get("env", {}), variables).items()})
    timeout = float(step.get("timeout_seconds", default_timeout))
    cwd = expand(step.get("cwd"), variables) if step.get("cwd") else None
    try:
        p = subprocess.run([str(x) for x in argv], cwd=cwd, env=env, text=True,
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
        expected = int(step.get("expect_exit", 0))
        ok = p.returncode == expected
        contains = expand(step.get("expect_stdout_contains"), variables) if step.get("expect_stdout_contains") else None
        if contains is not None:
            ok = ok and str(contains) in (p.stdout or "")
        return StepEvidence(
            str(step.get("name", argv[0])), "command", ok,
            (time.monotonic() - started) * 1000,
            {"argv": argv, "exit": p.returncode, "stdout": (p.stdout or "")[-12000:]},
            None if ok else f"command expectation failed: expected exit={expected}",
        )
    except Exception as e:
        return StepEvidence(str(step.get("name", argv[0])), "command", False,
                            (time.monotonic() - started) * 1000, {"argv": argv}, str(e))


def run_http(step: dict[str, Any], variables: dict[str, str], default_timeout: float, base_url: str) -> StepEvidence:
    started = time.monotonic()
    method = str(step.get("method", "GET")).upper()
    url = str(expand(step.get("url", ""), variables))
    if url.startswith("/"):
        url = base_url.rstrip("/") + url
    headers = {str(k): str(v) for k, v in expand(step.get("headers", {}), variables).items()}
    payload = step.get("body_json")
    body = None
    if payload is not None:
        body = json.dumps(expand(payload, variables), ensure_ascii=False).encode("utf-8")
        headers.setdefault("Content-Type", "application/json")
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    timeout = float(step.get("timeout_seconds", default_timeout))
    status = None
    raw = b""
    err_text = None
    try:
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                status = resp.status
                raw = resp.read()
        except urllib.error.HTTPError as e:
            status = e.code
            raw = e.read()
        expected_status = int(step.get("expect_status", 200))
        ok = status == expected_status
        parsed: Any = None
        text = raw.decode("utf-8", errors="replace")
        if raw:
            try:
                parsed = json.loads(text)
            except Exception:
                parsed = None
        if step.get("expect_body_contains") is not None:
            ok = ok and str(expand(step["expect_body_contains"], variables)) in text
        if step.get("expect_json_path") is not None:
            if parsed is None:
                ok = False
                err_text = "response is not JSON"
            else:
                try:
                    actual = json_path(parsed, str(step["expect_json_path"]))
                    expected = expand(step.get("expect_json_equals"), variables)
                    ok = ok and actual == expected
                    if actual != expected:
                        err_text = f"json assertion failed: {actual!r} != {expected!r}"
                except Exception as e:
                    ok = False
                    err_text = f"json assertion failed: {e}"
        return StepEvidence(
            str(step.get("name", f"{method} {url}")), "http", ok,
            (time.monotonic() - started) * 1000,
            {"method": method, "url": url, "status": status, "body": text[-12000:]},
            err_text if not ok else None,
        )
    except Exception as e:
        return StepEvidence(str(step.get("name", f"{method} {url}")), "http", False,
                            (time.monotonic() - started) * 1000, {"method": method, "url": url}, str(e))


def run_sql(step: dict[str, Any], variables: dict[str, str], default_timeout: float, runner: dict[str, Any]) -> StepEvidence:
    started = time.monotonic()
    try:
        import psycopg  # type: ignore
    except Exception as e:
        return StepEvidence(str(step.get("name", "sql")), "sql", False, 0, {}, f"Psycopg 3 required: {e}")
    dsn = expand(step.get("dsn") or runner.get("dsn") or "", variables)
    if not dsn:
        dsn_env = str(step.get("dsn_env") or runner.get("dsn_env") or "TEST_DATABASE_URL")
        dsn = os.environ.get(dsn_env, "")
    if not dsn:
        return StepEvidence(str(step.get("name", "sql")), "sql", False, 0, {}, "no PostgreSQL DSN configured")
    query = str(expand(step.get("query", ""), variables))
    if not query:
        return StepEvidence(str(step.get("name", "sql")), "sql", False, 0, {}, "query is required")
    params = expand(step.get("params", []), variables)
    timeout = float(step.get("timeout_seconds", default_timeout))
    try:
        # connect_timeout is integral seconds; statement_timeout constrains query execution.
        with psycopg.connect(str(dsn), connect_timeout=max(1, int(timeout))) as conn:
            with conn.cursor() as cur:
                cur.execute(f"SET LOCAL statement_timeout = {max(1, int(timeout * 1000))}")
                cur.execute(query, params if isinstance(params, list) else None)
                rows = cur.fetchall() if cur.description is not None else []
                row_count = len(rows) if cur.description is not None else cur.rowcount
        ok = True
        error = None
        if step.get("expect_row_count") is not None:
            expected_count = int(step["expect_row_count"])
            if row_count != expected_count:
                ok = False
                error = f"row_count {row_count} != {expected_count}"
        if step.get("expect_scalar") is not None:
            actual = rows[0][0] if rows and rows[0] else None
            expected = expand(step.get("expect_scalar"), variables)
            # TOML preserves scalar types; compare directly, but permit explicit string expectation.
            if actual != expected and str(actual) != str(expected):
                ok = False
                error = f"scalar {actual!r} != {expected!r}"
        return StepEvidence(
            str(step.get("name", "sql")), "sql", ok,
            (time.monotonic() - started) * 1000,
            {"query": query, "row_count": row_count, "rows": [list(r) for r in rows[:50]]},
            error,
        )
    except Exception as e:
        sqlstate = getattr(e, "sqlstate", None)
        return StepEvidence(str(step.get("name", "sql")), "sql", False,
                            (time.monotonic() - started) * 1000,
                            {"query": query, "sqlstate": sqlstate}, str(e))


def run_step(step: dict[str, Any], variables: dict[str, str], runner: dict[str, Any]) -> StepEvidence:
    kind = str(step.get("kind", "")).lower()
    timeout = float(runner.get("timeout_seconds", 30))
    if kind == "command":
        return run_command(step, variables, timeout)
    if kind == "http":
        return run_http(step, variables, timeout, str(expand(runner.get("base_url", ""), variables)))
    if kind == "sql":
        return run_sql(step, variables, timeout, runner)
    if kind == "sleep":
        started = time.monotonic()
        seconds = float(step.get("seconds", 0))
        time.sleep(seconds)
        return StepEvidence(str(step.get("name", "sleep")), "sleep", True,
                            (time.monotonic() - started) * 1000, {"seconds": seconds})
    return StepEvidence(str(step.get("name", kind or "unknown")), kind or "unknown", False, 0, {}, f"unsupported step kind {kind!r}")


def run_parallel(steps: list[dict[str, Any]], variables: dict[str, str], runner: dict[str, Any]) -> list[StepEvidence]:
    results: list[StepEvidence | None] = [None] * len(steps)
    threads: list[threading.Thread] = []

    def one(i: int, step: dict[str, Any]):
        results[i] = run_step(step, variables, runner)

    for i, step in enumerate(steps):
        t = threading.Thread(target=one, args=(i, step), daemon=True)
        threads.append(t)
        t.start()
    for t in threads:
        t.join()
    return [x for x in results if x is not None]


def execute(binding: dict[str, Any], scenario_id: str) -> tuple[int, dict[str, Any]]:
    runner = binding.get("runner") or {}
    variables = {k: str(v) for k, v in os.environ.items()}
    variables.update({str(k): str(v) for k, v in (binding.get("variables") or {}).items()})
    executions = [x for x in binding.get("execution", []) if str(x.get("scenario_id")) == scenario_id or str(x.get("id")) == scenario_id]
    if not executions:
        return 2, {"status": "BLOCKED", "error": f"no execution binding for scenario {scenario_id}"}
    if len(executions) > 1:
        return 2, {"status": "BLOCKED", "error": f"multiple execution bindings match {scenario_id}"}
    execution = executions[0]
    steps = list(execution.get("steps", []))
    cleanup = [s for s in steps if str(s.get("phase", "")).lower() == "cleanup"]
    main = [s for s in steps if str(s.get("phase", "")).lower() != "cleanup"]
    evidence: list[StepEvidence] = []
    failed = False
    started = time.time()
    try:
        i = 0
        while i < len(main):
            group = str(main[i].get("parallel_group", ""))
            if group:
                batch = [main[i]]
                i += 1
                while i < len(main) and str(main[i].get("parallel_group", "")) == group:
                    batch.append(main[i])
                    i += 1
                batch_results = run_parallel(batch, variables, runner)
                evidence.extend(batch_results)
                if any(not x.ok for x in batch_results):
                    failed = True
                    if bool(execution.get("fail_fast", True)):
                        break
            else:
                result = run_step(main[i], variables, runner)
                evidence.append(result)
                i += 1
                if not result.ok:
                    failed = True
                    if bool(execution.get("fail_fast", True)):
                        break
    finally:
        for step in cleanup:
            result = run_step(step, variables, runner)
            evidence.append(result)
            if not result.ok:
                failed = True

    result = {
        "execution_id": execution.get("id"),
        "scenario_id": execution.get("scenario_id"),
        "status": "FAIL" if failed else "PASS",
        "started_at_epoch": started,
        "elapsed_ms": (time.time() - started) * 1000,
        "steps": [asdict(x) for x in evidence],
    }
    return (1 if failed else 0), result


def validate_binding(binding: dict[str, Any], plan: dict[str, Any] | None = None) -> list[str]:
    errors: list[str] = []
    scenario_ids = {str(x.get("id")) for x in (plan or {}).get("scenarios", []) if x.get("id")}
    seen: set[str] = set()
    for ex in binding.get("execution", []):
        eid = str(ex.get("id", "")).strip()
        sid = str(ex.get("scenario_id", "")).strip()
        if not eid:
            errors.append("execution missing id")
        elif eid in seen:
            errors.append(f"duplicate execution id {eid}")
        seen.add(eid)
        if not sid:
            errors.append(f"execution {eid or '<unknown>'} missing scenario_id")
        elif plan is not None and sid not in scenario_ids:
            errors.append(f"execution {eid} references unknown plan scenario {sid}")
        steps = ex.get("steps", [])
        if not steps:
            errors.append(f"execution {eid} has no steps")
        for step in steps:
            kind = str(step.get("kind", ""))
            if kind not in {"command", "http", "sql", "sleep"}:
                errors.append(f"execution {eid} has unsupported step kind {kind!r}")
            if kind == "command" and not step.get("argv"):
                errors.append(f"execution {eid} command step missing argv")
            if kind == "http" and not step.get("url"):
                errors.append(f"execution {eid} http step missing url")
            if kind == "sql" and not step.get("query"):
                errors.append(f"execution {eid} sql step missing query")
    return errors


def main() -> int:
    ap = argparse.ArgumentParser(description="Execute business verification scenarios and emit evidence")
    sub = ap.add_subparsers(dest="cmd", required=True)

    val = sub.add_parser("validate")
    val.add_argument("--binding", required=True, type=Path)
    val.add_argument("--plan", type=Path)

    run = sub.add_parser("run")
    run.add_argument("--binding", required=True, type=Path)
    run.add_argument("--scenario", required=True)
    run.add_argument("--plan", type=Path)
    run.add_argument("--evidence-out", type=Path)

    args = ap.parse_args()
    binding = load_toml(args.binding)
    plan = load_toml(args.plan) if args.plan else None
    errors = validate_binding(binding, plan)
    if errors:
        for e in errors:
            print(f"ERROR {e}")
        return 2
    if args.cmd == "validate":
        print("PASS business scenario binding")
        return 0
    rc, result = execute(binding, args.scenario)
    text = json.dumps(result, ensure_ascii=False, indent=2)
    if args.evidence_out:
        args.evidence_out.parent.mkdir(parents=True, exist_ok=True)
        args.evidence_out.write_text(text + "\n", encoding="utf-8")
    print(text)
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
