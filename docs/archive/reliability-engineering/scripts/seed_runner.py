#!/usr/bin/env python3
"""Repeat a generated/randomized test command with deterministic seeds.

Example:
  python scripts/seed_runner.py --runs 200 --timeout 60 -- cargo test model_prop -- --nocapture

The child receives RELIABILITY_SEED and TEST_SEED. If an argument contains
"{seed}", it is substituted. On failure a repro JSON file is written.
"""
from __future__ import annotations

import argparse
import json
import os
import shlex
import subprocess
import sys
import time
from pathlib import Path


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--runs", type=int, default=100)
    ap.add_argument("--start-seed", type=int, default=1)
    ap.add_argument("--timeout", type=float, default=120.0)
    ap.add_argument("--log-dir", default=".reliability-runs")
    ap.add_argument("command", nargs=argparse.REMAINDER)
    args = ap.parse_args()

    cmd_template = args.command
    if cmd_template and cmd_template[0] == "--":
        cmd_template = cmd_template[1:]
    if not cmd_template:
        ap.error("provide command after --")
    if args.runs < 1:
        ap.error("--runs must be >= 1")

    log_dir = Path(args.log_dir)
    log_dir.mkdir(parents=True, exist_ok=True)

    started = time.time()
    for offset in range(args.runs):
        seed = args.start_seed + offset
        cmd = [part.replace("{seed}", str(seed)) for part in cmd_template]
        env = os.environ.copy()
        env["RELIABILITY_SEED"] = str(seed)
        env["TEST_SEED"] = str(seed)
        print(f"[{offset+1}/{args.runs}] seed={seed} :: {shlex.join(cmd)}", flush=True)
        try:
            p = subprocess.run(
                cmd,
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                timeout=args.timeout,
            )
        except subprocess.TimeoutExpired as exc:
            output = exc.stdout or ""
            if isinstance(output, bytes):
                output = output.decode(errors="replace")
            return fail(log_dir, seed, cmd, "timeout", output, args.timeout)

        if p.returncode != 0:
            return fail(log_dir, seed, cmd, f"exit-{p.returncode}", p.stdout, args.timeout)

    elapsed = time.time() - started
    print(f"PASS: {args.runs} seeded runs in {elapsed:.2f}s")
    return 0


def fail(log_dir: Path, seed: int, cmd: list[str], reason: str, output: str, timeout: float) -> int:
    stamp = int(time.time())
    out_path = log_dir / f"failure-seed-{seed}-{stamp}.log"
    repro_path = log_dir / f"repro-seed-{seed}-{stamp}.json"
    out_path.write_text(output or "", encoding="utf-8", errors="replace")
    repro = {
        "seed": seed,
        "reason": reason,
        "command": cmd,
        "shell_command": shlex.join(cmd),
        "environment": {"RELIABILITY_SEED": str(seed), "TEST_SEED": str(seed)},
        "timeout_seconds": timeout,
        "output_log": str(out_path),
    }
    repro_path.write_text(json.dumps(repro, indent=2), encoding="utf-8")
    print(f"FAIL: seed={seed} reason={reason}", file=sys.stderr)
    print(f"repro metadata: {repro_path}", file=sys.stderr)
    print(f"output: {out_path}", file=sys.stderr)
    print(
        "reproduce: "
        + f"RELIABILITY_SEED={seed} TEST_SEED={seed} "
        + shlex.join(cmd),
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
