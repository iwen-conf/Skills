#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import os
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    json_load,
    output_tail,
    sha256_file,
    wait_process,
)


def run_cmd(argv: list[str], timeout: float) -> dict[str, Any]:
    started = time.monotonic()
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        if not math.isfinite(timeout) or timeout <= 0:
            result = {"returncode": None, "blocked": True, "timeout": True, "error": "execution budget exhausted"}
        else:
            try:
                proc = subprocess.Popen(argv, stdout=out, stderr=err, start_new_session=os.name != "nt")
            except OSError as exc:
                result = {"returncode": None, "blocked": True, "error": repr(exc)}
            else:
                result = wait_process(proc, timeout)
        return {
            "argv": argv,
            **result,
            "elapsed_ms": round((time.monotonic() - started) * 1000, 3),
            "stdout_tail": output_tail(out),
            "stderr_tail": output_tail(err),
        }


def _argv(value: Any) -> bool:
    return isinstance(value, list) and bool(value) and all(isinstance(x, str) and x.strip() for x in value)


def main() -> int:
    ap = argparse.ArgumentParser(description="Orchestrate an isolated, observer-synchronized fault scenario; no shell")
    ap.add_argument("--scenario-file", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ack-isolated-environment", action="store_true")
    ns = ap.parse_args()
    started = time.monotonic()
    ev = evidence_base("fault_scenario", "fault.scenario")
    path = Path(ns.scenario_file)
    ev["reproduction"] = {"scenario_file": str(path)}
    records: list[dict[str, Any]] = []
    blockers: list[str] = []
    proc = None
    spec: dict[str, Any] = {}
    timeout = 0.0
    execution_started = False
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        try:
            loaded = json_load(path)
            if not isinstance(loaded, dict):
                raise ValueError("scenario must be a JSON object")
            spec = loaded
            ev["reproduction"]["sha256"] = sha256_file(path)
            ev["id"] = "fault." + str(spec.get("id", "scenario"))
            if not ns.ack_isolated_environment or spec.get("isolated_environment_required") is not True:
                raise ValueError("fault injection requires acknowledgement and isolated_environment_required=true")
            budget = float(spec["timeout_s"])
            timeout = float(spec["command_timeout_s"])
            target_timeout = float(spec["target_timeout_s"])
            if any(not math.isfinite(x) or x <= 0 for x in (budget, timeout, target_timeout)):
                raise ValueError("scenario, command and target budgets must be finite and positive")
            if any(spec.get(k, 0) != 0 for k in ("fault_after_s", "fault_hold_s")):
                raise ValueError("fixed fault sleeps do not prove an interleaving; use ready/fault/recovery oracles")
            if not _argv(spec.get("target")):
                raise ValueError("target argv is required")
            for phase in ("ready_oracle", "inject", "fault_oracle", "recover", "recovery_oracle", "oracle", "cleanup"):
                if not spec.get(phase):
                    raise ValueError(f"{phase} argv commands are required")
            for phase in (
                "prepare",
                "before_oracle",
                "ready_oracle",
                "inject",
                "fault_oracle",
                "recover",
                "recovery_oracle",
                "oracle",
                "cleanup",
            ):
                commands = spec.get(phase, [])
                if not isinstance(commands, list) or any(not _argv(c) for c in commands):
                    raise ValueError(f"invalid {phase} argv commands")
            oracle_failures = spec.get("oracle_failure_returncodes", [])
            allowed_target = spec.get("allowed_target_returncodes", [0])
            if not isinstance(oracle_failures, list) or any(type(c) is not int or c <= 0 for c in oracle_failures):
                raise ValueError("oracle_failure_returncodes must explicitly name positive assertion exit codes")
            if (
                not isinstance(allowed_target, list)
                or not allowed_target
                or any(type(c) is not int or c < 0 for c in allowed_target)
            ):
                raise ValueError("allowed_target_returncodes must name completed target exits")
            deadline = started + budget
            execution_started = True

            def phase_commands(phase: str) -> None:
                for cmd in spec.get(phase, []):
                    result = run_cmd(cmd, min(timeout, deadline - time.monotonic()))
                    records.append({"phase": phase, **result})
                    if result.get("blocked") or result["returncode"] != 0:
                        raise RuntimeError(f"{phase} did not complete successfully")

            phase_commands("prepare")
            phase_commands("before_oracle")
            if deadline <= time.monotonic():
                raise RuntimeError("execution budget exhausted before target start")
            proc = subprocess.Popen(spec["target"], stdout=out, stderr=err, start_new_session=os.name != "nt")
            records.append({"phase": "target_start", "argv": spec["target"], "pid": proc.pid})
            phase_commands("ready_oracle")
            if proc.poll() is not None:
                raise RuntimeError("target exited before the observed fault boundary")
            for phase in ("inject", "fault_oracle", "recover", "recovery_oracle"):
                phase_commands(phase)
            result = wait_process(proc, min(target_timeout, deadline - time.monotonic()))
            records.append({"phase": "target_end", **result})
            if result["blocked"]:
                raise RuntimeError("target completion or process cleanup evidence is missing")
            if result["returncode"] not in allowed_target:
                ev["violations"].append(f"target returncode {result['returncode']} not in {allowed_target}")
            for cmd in spec["oracle"]:
                result = run_cmd(cmd, min(timeout, deadline - time.monotonic()))
                records.append({"phase": "oracle", **result})
                if result.get("blocked"):
                    blockers.append("oracle could not run to completion")
                elif result["returncode"] in oracle_failures:
                    ev["violations"].append(f"independent oracle asserted a violation (exit {result['returncode']})")
                elif result["returncode"] != 0:
                    blockers.append(f"oracle exit {result['returncode']} is not a declared assertion result")
        except Exception as exc:
            blockers.append(repr(exc))
        finally:
            if proc is not None:
                result = wait_process(proc, 0)
                if result["blocked"]:
                    records.append({"phase": "target_stop", **result})
                    blockers.append(
                        "target needed interruption or descendant cleanup; fixture state requires reconstruction"
                    )
                records.append(
                    {"phase": "target_output", "stdout_tail": output_tail(out), "stderr_tail": output_tail(err)}
                )
            # A separate bounded cleanup is still required after an exhausted action budget.
            if execution_started:
                cleanup_deadline = time.monotonic() + timeout
                for cmd in spec["cleanup"]:
                    result = run_cmd(cmd, cleanup_deadline - time.monotonic())
                    records.append({"phase": "cleanup", **result})
                    if result.get("blocked") or result["returncode"] != 0:
                        blockers.append("cleanup failed; actual fixture state requires reconstruction")
    ev["observed"] = {"records": records, "blockers": blockers}
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
