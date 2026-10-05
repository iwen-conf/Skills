#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, run
from sqlverify_config import load_config


def _cmd(x: Any) -> list[str]:
    if not isinstance(x, list) or not x or not all(isinstance(i, str) and i for i in x):
        raise ValueError("commands must be non-empty argv arrays; shell strings are unsupported")
    return list(x)


def main() -> int:
    ap = argparse.ArgumentParser(description="Orchestrate isolated failover/failback qualification with explicit RTO/business oracles")
    ap.add_argument("--config", required=True)
    ap.add_argument("--case", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ap.add_argument("--ack-isolated-environment", action="store_true")
    ns = ap.parse_args()
    cfg = load_config(Path(ns.config)); root = Path(ns.root).resolve()
    case = next((x for x in cfg.get("failover_rehearsal", []) if x.get("id") == ns.case), None)
    if not case: raise SystemExit(f"failover_rehearsal not found: {ns.case}")
    start = time.monotonic(); ev = evidence_base("failover_rehearsal", f"failover.{ns.case}")
    if not ns.ack_isolated_environment:
        ev["violations"].append("failover rehearsal requires --ack-isolated-environment")
        json_dump(Path(ns.out), finalize_evidence(ev, start, "BLOCKED")); return 2
    workload: subprocess.Popen[str] | None = None
    obs: dict[str, Any] = {"steps": []}
    try:
        for label in ("prepare_command",):
            if case.get(label):
                cp = run(_cmd(case[label]), cwd=root, timeout=float(case.get("step_timeout_s", 300)))
                obs["steps"].append({"step": label, "returncode": cp.returncode, "stdout_tail": cp.stdout[-2000:], "stderr_tail": cp.stderr[-2000:]})
                if cp.returncode != 0: raise RuntimeError(f"{label} failed rc={cp.returncode}")
        if case.get("workload_command"):
            workload = subprocess.Popen(_cmd(case["workload_command"]), cwd=str(root), text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            time.sleep(float(case.get("workload_warmup_s", 2)))
        t0 = time.monotonic()
        cp = run(_cmd(case["inject_failover_command"]), cwd=root, timeout=float(case.get("failover_command_timeout_s", 300)))
        obs["steps"].append({"step": "inject_failover", "returncode": cp.returncode, "stdout_tail": cp.stdout[-4000:], "stderr_tail": cp.stderr[-4000:]})
        if cp.returncode != 0: raise RuntimeError(f"failover command failed rc={cp.returncode}")
        timeout_s = float(case.get("healthcheck_timeout_s", 180)); poll_s = float(case.get("healthcheck_poll_s", 1)); deadline = time.monotonic()+timeout_s
        healthy = False; attempts = 0; last = ""
        while time.monotonic() < deadline:
            attempts += 1
            hc = run(_cmd(case["healthcheck_command"]), cwd=root, timeout=float(case.get("healthcheck_command_timeout_s", 10)))
            last = (hc.stdout + "\n" + hc.stderr)[-2000:]
            if hc.returncode == 0: healthy = True; break
            time.sleep(poll_s)
        rto_s = time.monotonic()-t0
        obs["rto_s"] = rto_s; obs["healthcheck"] = {"healthy": healthy, "attempts": attempts, "last": last}
        if not healthy: ev["violations"].append("service/database did not become healthy after failover")
        if case.get("max_rto_s") is not None and rto_s > float(case["max_rto_s"]): ev["violations"].append(f"RTO {rto_s:.3f}s > {case['max_rto_s']}s")
        for idx, cmdx in enumerate(case.get("oracle_command", [])):
            cp = run(_cmd(cmdx), cwd=root, timeout=float(case.get("oracle_timeout_s", 120)))
            obs["steps"].append({"step": f"oracle[{idx}]", "returncode": cp.returncode, "stdout_tail": cp.stdout[-4000:], "stderr_tail": cp.stderr[-4000:]})
            if cp.returncode != 0: ev["violations"].append(f"oracle[{idx}] failed rc={cp.returncode}")
        if case.get("failback_command"):
            cp = run(_cmd(case["failback_command"]), cwd=root, timeout=float(case.get("failback_timeout_s", 300)))
            obs["steps"].append({"step": "failback", "returncode": cp.returncode, "stdout_tail": cp.stdout[-4000:], "stderr_tail": cp.stderr[-4000:]})
            if cp.returncode != 0: ev["violations"].append(f"failback failed rc={cp.returncode}")
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "BLOCKED"
    finally:
        if workload is not None:
            try:
                workload.terminate(); out, err = workload.communicate(timeout=10)
            except Exception:
                workload.kill(); out, err = workload.communicate()
            obs["workload"] = {"returncode": workload.returncode, "stdout_tail": (out or "")[-4000:], "stderr_tail": (err or "")[-4000:]}
        if case.get("cleanup_command"):
            try:
                cp = run(_cmd(case["cleanup_command"]), cwd=root, timeout=float(case.get("cleanup_timeout_s", 300)))
                obs["steps"].append({"step": "cleanup", "returncode": cp.returncode, "stdout_tail": cp.stdout[-2000:], "stderr_tail": cp.stderr[-2000:]})
                if cp.returncode != 0: ev["warnings"].append(f"cleanup failed rc={cp.returncode}")
            except Exception as e:
                ev["warnings"].append(f"cleanup exception: {e!r}")
    ev["observed"] = obs; ev["reproduction"] = {"case": case}
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
