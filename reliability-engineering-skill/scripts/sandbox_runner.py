#!/usr/bin/env python3
"""Project-local test sandbox lifecycle runner.

Executes explicit argv arrays from a TOML contract. It never invokes a shell,
never uses sudo, and always attempts teardown. This is an adapter around the
project's existing Docker Compose/Testcontainers/dev-stack commands, not a
replacement for them.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import tomllib
from pathlib import Path
from typing import Any


def load(path: Path) -> dict[str, Any]:
    with path.open("rb") as f:
        return tomllib.load(f)


def run_argv(argv: list[Any], cwd: Path, timeout: float, env: dict[str, str]) -> dict[str, Any]:
    started = time.monotonic()
    try:
        p = subprocess.run([str(x) for x in argv], cwd=cwd, env=env, text=True,
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
        return {
            "argv": [str(x) for x in argv],
            "exit": p.returncode,
            "ok": p.returncode == 0,
            "elapsed_ms": (time.monotonic() - started) * 1000,
            "output": (p.stdout or "")[-12000:],
        }
    except Exception as e:
        return {"argv": [str(x) for x in argv], "exit": None, "ok": False,
                "elapsed_ms": (time.monotonic() - started) * 1000, "output": "", "error": str(e)}


def commands(section: dict[str, Any], key: str) -> list[list[Any]]:
    value = section.get(key, [])
    if not isinstance(value, list):
        return []
    if value and all(not isinstance(x, list) for x in value):
        return [value]
    return [x for x in value if isinstance(x, list)]


def main() -> int:
    ap = argparse.ArgumentParser(description="Run an isolated project test sandbox lifecycle")
    ap.add_argument("--contract", required=True, type=Path)
    ap.add_argument("--root", type=Path, default=Path.cwd())
    ap.add_argument("--timeout", type=float, default=120)
    ap.add_argument("--evidence-out", type=Path)
    ap.add_argument("--", dest="sep", action="store_true")
    ap.add_argument("test_command", nargs=argparse.REMAINDER)
    args = ap.parse_args()

    data = load(args.contract)
    sb = data.get("sandbox") or {}
    if not sb.get("isolation"):
        print("ERROR sandbox.isolation must document how test data/traffic is isolated", file=sys.stderr)
        return 2
    root = args.root.resolve()
    env = os.environ.copy()
    env.update({str(k): str(v) for k, v in (data.get("env") or {}).items()})
    evidence: dict[str, Any] = {"isolation": sb.get("isolation"), "steps": []}
    failed = False

    try:
        for key in ("bootstrap", "reset", "healthcheck"):
            for argv in commands(sb, key):
                result = run_argv(argv, root, args.timeout, env)
                result["phase"] = key
                evidence["steps"].append(result)
                if not result["ok"]:
                    failed = True
                    break
            if failed:
                break
        if not failed and args.test_command:
            cmd = list(args.test_command)
            if cmd and cmd[0] == "--":
                cmd = cmd[1:]
            if cmd:
                result = run_argv(cmd, root, args.timeout, env)
                result["phase"] = "test"
                evidence["steps"].append(result)
                if not result["ok"]:
                    failed = True
    finally:
        for argv in commands(sb, "teardown"):
            result = run_argv(argv, root, args.timeout, env)
            result["phase"] = "teardown"
            evidence["steps"].append(result)
            if not result["ok"]:
                failed = True

    evidence["status"] = "FAIL" if failed else "PASS"
    text = json.dumps(evidence, ensure_ascii=False, indent=2)
    if args.evidence_out:
        args.evidence_out.parent.mkdir(parents=True, exist_ok=True)
        args.evidence_out.write_text(text + "\n", encoding="utf-8")
    print(text)
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
