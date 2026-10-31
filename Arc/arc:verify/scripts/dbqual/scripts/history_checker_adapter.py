#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

from sqlverify_common import evidence_base, finalize_evidence, json_dump, run, sha256_file, command_verdict
from sqlverify_config import load_config


def _cmd(x):
    if not isinstance(x, list) or not x or not all(isinstance(i, str) and i for i in x):
        raise ValueError("checker_command must be an argv array; shell strings are unsupported")
    return list(x)


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Adapter for Jepsen/Elle or project-specific transactional history checkers"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    root = Path(ns.root).resolve()
    case = next((x for x in cfg.get("history_check", []) if x.get("id") == ns.case), None)
    if not case:
        raise SystemExit(f"history_check not found: {ns.case}")
    start = time.monotonic()
    ev = evidence_base("history_check", f"history.{ns.case}")
    history = root / case.get("history_file", "")
    if not history.exists():
        ev["violations"].append(f"history file not found: {history}")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    cmd = _cmd(case["checker_command"])
    # {history} is replaced as one argv value, never through a shell.
    cmd = [str(history) if x == "{history}" else x for x in cmd]
    try:
        cp = run(cmd, cwd=root, timeout=float(case.get("timeout_s", 900)))
        obs = {"returncode": cp.returncode, "stdout_tail": cp.stdout[-20000:], "stderr_tail": cp.stderr[-20000:]}
        ev["observed"] = obs
        expected_rc = case.get("expected_returncode", 0)
        status = command_verdict(cp.returncode, case.get("oracle_failure_returncodes", []), (expected_rc,))
        obs["verdict"] = status
        if status == "FAIL":
            ev["violations"].append(f"history assertion failed: rc={cp.returncode}, expected {expected_rc}")
        elif status == "BLOCKED":
            raise RuntimeError(f"history checker did not yield a declared assertion result: rc={cp.returncode}")
        for needle in case.get("require_stdout_contains", []):
            if needle not in cp.stdout:
                ev["violations"].append(f"checker stdout missing required marker: {needle!r}")
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["warnings"].append(repr(e))
        status = "FAIL" if ev["violations"] else "BLOCKED"
    ev["reproduction"] = {
        "history_file": str(history),
        "history_sha256": sha256_file(history),
        "model": case.get("model"),
        "checker_version": case.get("checker_version"),
        "command": cmd,
    }
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
