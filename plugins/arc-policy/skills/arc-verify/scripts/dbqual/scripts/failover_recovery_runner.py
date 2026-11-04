#!/usr/bin/env python3
from __future__ import annotations

import argparse
from contextlib import ExitStack
import json
import math
import os
import subprocess
import tempfile
import threading
import time
from pathlib import Path
from typing import Any

from sqlverify_common import (
    evidence_base,
    finalize_evidence,
    json_dump,
    run,
    command_verdict,
    stop_process_tree,
    wait_process,
    output_tail,
)
from sqlverify_config import load_config


def _cmd(x: Any) -> list[str]:
    if not isinstance(x, list) or not x or not all(isinstance(i, str) and i for i in x):
        raise ValueError("commands must be non-empty argv arrays; shell strings are unsupported")
    return list(x)


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Orchestrate isolated failover/failback qualification with explicit RTO/business oracles"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ap.add_argument("--ack-isolated-environment", action="store_true")
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config))
    root = Path(ns.root).resolve()
    case = next((x for x in cfg.get("failover_rehearsal", []) if x.get("id") == ns.case), None)
    if not case:
        raise SystemExit(f"failover_rehearsal not found: {ns.case}")
    start = time.monotonic()
    ev = evidence_base("failover_rehearsal", f"failover.{ns.case}")
    if not ns.ack_isolated_environment:
        ev["violations"].append("failover rehearsal requires --ack-isolated-environment")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED"))
        return 2
    workload: subprocess.Popen[bytes] | None = None
    obs: dict[str, Any] = {"steps": [], "blockers": []}
    blockers = obs["blockers"]
    streams = ExitStack()
    try:
        for label in ("prepare_command",):
            if case.get(label):
                cp = run(_cmd(case[label]), cwd=root, timeout=float(case.get("step_timeout_s", 300)))
                obs["steps"].append(
                    {
                        "step": label,
                        "returncode": cp.returncode,
                        "stdout_tail": cp.stdout[-2000:],
                        "stderr_tail": cp.stderr[-2000:],
                    }
                )
                if cp.returncode != 0:
                    raise RuntimeError(f"{label} failed rc={cp.returncode}")
        if case.get("workload_command"):
            if not case.get("ready_oracle"):
                raise RuntimeError("workload needs independent ready_oracle evidence before failover injection")
            out_log = streams.enter_context(tempfile.TemporaryFile())
            err_log = streams.enter_context(tempfile.TemporaryFile())
            workload = subprocess.Popen(
                _cmd(case["workload_command"]),
                cwd=str(root),
                stdout=out_log,
                stderr=err_log,
                start_new_session=os.name != "nt",
            )
            for cmdx in case["ready_oracle"]:
                cp = run(_cmd(cmdx), cwd=root, timeout=float(case.get("step_timeout_s", 300)))
                obs["steps"].append(
                    {
                        "step": "ready_oracle",
                        "returncode": cp.returncode,
                        "stdout_tail": cp.stdout[-2000:],
                        "stderr_tail": cp.stderr[-2000:],
                    }
                )
                if cp.returncode != 0:
                    raise RuntimeError(f"workload readiness not established: rc={cp.returncode}")
            if workload.poll() is not None:
                raise RuntimeError("workload exited before failover injection")
        if not case.get("oracle_command"):
            raise RuntimeError("failover needs an independent business oracle")
        t0 = time.monotonic()
        cp = run(
            _cmd(case["inject_failover_command"]), cwd=root, timeout=float(case.get("failover_command_timeout_s", 300))
        )
        obs["steps"].append(
            {
                "step": "inject_failover",
                "returncode": cp.returncode,
                "stdout_tail": cp.stdout[-4000:],
                "stderr_tail": cp.stderr[-4000:],
            }
        )
        if cp.returncode != 0:
            raise RuntimeError(f"failover command failed rc={cp.returncode}")
        timeout_s = float(case.get("healthcheck_timeout_s", 180))
        poll_s = float(case.get("healthcheck_poll_s", 1))
        deadline = time.monotonic() + timeout_s
        if not all(math.isfinite(value) and value > 0 for value in (timeout_s, poll_s)):
            raise ValueError("healthcheck deadline and poll interval must be finite and positive")
        healthy = False
        attempts = 0
        last = ""
        while time.monotonic() < deadline:
            attempts += 1
            hc = run(
                _cmd(case["healthcheck_command"]),
                cwd=root,
                timeout=min(deadline - time.monotonic(), float(case.get("healthcheck_command_timeout_s", 10))),
            )
            last = (hc.stdout + "\n" + hc.stderr)[-2000:]
            verdict = command_verdict(hc.returncode, case.get("oracle_failure_returncodes", []))
            obs["steps"].append(
                {"step": "healthcheck", "returncode": hc.returncode, "verdict": verdict, "output_tail": last}
            )
            if verdict == "BLOCKED":
                raise RuntimeError(f"healthcheck did not yield a declared observation: rc={hc.returncode}")
            if verdict == "PASS":
                healthy = True
                break
            threading.Event().wait(max(0, min(poll_s, deadline - time.monotonic())))
        rto_s = time.monotonic() - t0
        obs["rto_s"] = rto_s
        obs["healthcheck"] = {"healthy": healthy, "attempts": attempts, "last": last}
        if not healthy:
            ev["violations"].append("service/database did not become healthy after failover")
        if case.get("max_rto_s") is not None and rto_s > float(case["max_rto_s"]):
            ev["violations"].append(f"RTO {rto_s:.3f}s > {case['max_rto_s']}s")
        for idx, cmdx in enumerate(case.get("oracle_command", [])):
            cp = run(_cmd(cmdx), cwd=root, timeout=float(case.get("oracle_timeout_s", 120)))
            obs["steps"].append(
                {
                    "step": f"oracle[{idx}]",
                    "returncode": cp.returncode,
                    "stdout_tail": cp.stdout[-4000:],
                    "stderr_tail": cp.stderr[-4000:],
                }
            )
            verdict = command_verdict(cp.returncode, case.get("oracle_failure_returncodes", []))
            obs["steps"][-1]["verdict"] = verdict
            if verdict == "FAIL":
                ev["violations"].append(f"oracle[{idx}] assertion failed rc={cp.returncode}")
            elif verdict == "BLOCKED":
                raise RuntimeError(f"oracle[{idx}] did not yield a declared assertion result: rc={cp.returncode}")
        if case.get("failback_command"):
            cp = run(_cmd(case["failback_command"]), cwd=root, timeout=float(case.get("failback_timeout_s", 300)))
            obs["steps"].append(
                {
                    "step": "failback",
                    "returncode": cp.returncode,
                    "stdout_tail": cp.stdout[-4000:],
                    "stderr_tail": cp.stderr[-4000:],
                }
            )
            if cp.returncode != 0:
                raise RuntimeError(f"failback did not complete: rc={cp.returncode}")
    except Exception as e:
        blockers.append(repr(e))
    finally:
        if workload is not None:
            try:
                if workload.poll() is None:
                    cleanup_error = stop_process_tree(workload)
                    if cleanup_error:
                        blockers.append(cleanup_error)
                else:
                    result = wait_process(workload, 0)
                    if result["blocked"]:
                        blockers.append(f"workload lifecycle incomplete: {result}")
                    if workload.returncode != 0:
                        blockers.append(
                            f"workload exited abnormally: rc={workload.returncode}; independent workload evidence is incomplete"
                        )
                obs["workload"] = {
                    "returncode": workload.returncode,
                    "stdout_tail": output_tail(out_log),
                    "stderr_tail": output_tail(err_log),
                }
            except Exception as e:
                blockers.append(f"workload cleanup could not be verified: {e!r}")
        if case.get("cleanup_command"):
            try:
                cp = run(_cmd(case["cleanup_command"]), cwd=root, timeout=float(case.get("cleanup_timeout_s", 300)))
                obs["steps"].append(
                    {
                        "step": "cleanup",
                        "returncode": cp.returncode,
                        "stdout_tail": cp.stdout[-2000:],
                        "stderr_tail": cp.stderr[-2000:],
                    }
                )
                if cp.returncode != 0:
                    blockers.append(f"cleanup failed rc={cp.returncode}; fixture reconstruction is required")
            except Exception as e:
                blockers.append(f"cleanup exception: {e!r}; fixture reconstruction is required")
        streams.close()
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    ev["observed"] = obs
    ev["reproduction"] = {"case": case}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
