#!/usr/bin/env python3
"""Start the app's servers, wait until each one answers, run a test command, then stop them.

    with_server.py --server "npm run dev" --port 5173 \
        [--server "cd api && go run ./cmd/api" --port 8080] [--ready-url http://localhost:8080/healthz] \
        [--timeout 90] [--log-dir .arc/artifacts/<task-slug>/logs] -- npx playwright test

The exit status is the command's own. 125 means the environment never became ready (port already
taken, server exited, readiness deadline passed): a harness failure, never a test verdict.
A port that is already listening is refused unless --reuse-running is given, because a server this
run did not start may serve another build. POSIX only (process groups).
"""

from __future__ import annotations

import argparse
import os
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

HARNESS_FAILURE = 125
STOP_GRACE_S = 10.0
LOG_TAIL_LINES = 30


def port_open(port: int) -> bool:
    try:
        with socket.create_connection(("localhost", port), timeout=0.5):
            return True
    except OSError:
        return False


def url_ready(url: str) -> bool:
    try:
        with urllib.request.urlopen(url, timeout=2) as resp:
            return 200 <= resp.status < 400
    except (urllib.error.URLError, OSError, ValueError):
        return False


def log_tail(path: Path) -> str:
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    return "\n".join(lines[-LOG_TAIL_LINES:])


def say(message: str) -> None:
    print(f"with_server: {message}", file=sys.stderr, flush=True)


def stop(proc: subprocess.Popen) -> None:
    for sig, wait in ((signal.SIGTERM, STOP_GRACE_S), (signal.SIGKILL, None)):
        try:
            os.killpg(proc.pid, sig)
        except ProcessLookupError:
            return
        try:
            proc.wait(timeout=wait)
            return
        except subprocess.TimeoutExpired:
            continue


def parse(argv: list[str]) -> tuple[argparse.Namespace, list[str]]:
    split = argv.index("--") if "--" in argv else len(argv)
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument(
        "--server",
        action="append",
        default=[],
        help="shell command that starts one server",
    )
    ap.add_argument(
        "--port",
        action="append",
        type=int,
        default=[],
        help="port of the matching --server, same order",
    )
    ap.add_argument(
        "--ready-url",
        action="append",
        default=[],
        help="URL that must answer 2xx/3xx before the command runs",
    )
    ap.add_argument(
        "--timeout",
        type=float,
        default=60.0,
        help="readiness deadline in seconds (default 60)",
    )
    ap.add_argument(
        "--log-dir",
        default=".arc/artifacts/with-server/logs",
        help="server log directory",
    )
    ap.add_argument(
        "--reuse-running",
        action="store_true",
        help="test a server already listening on its port",
    )
    ns = ap.parse_args(argv[:split])
    command = argv[split + 1 :]
    if not ns.server or len(ns.server) != len(ns.port):
        ap.error("give one --port for every --server, and at least one --server")
    if not command:
        ap.error("put the test command after --")
    return ns, command


def main(argv: list[str] | None = None) -> int:
    ns, command = parse(sys.argv[1:] if argv is None else argv)
    log_dir = Path(ns.log_dir)
    log_dir.mkdir(parents=True, exist_ok=True)
    started: list[tuple[subprocess.Popen, int, Path]] = []
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(128 + signal.SIGTERM))
    try:
        for i, (cmd, port) in enumerate(zip(ns.server, ns.port), 1):
            if port_open(port):
                if not ns.reuse_running:
                    say(
                        f"port {port} is already in use; stop that server or pass --reuse-running"
                    )
                    return HARNESS_FAILURE
                say(
                    f"port {port}: reusing a server this run did not start; its build is unverified"
                )
                continue
            log = log_dir / f"server-{i}-{port}.log"
            with log.open("ab") as fh:
                proc = subprocess.Popen(
                    cmd,
                    shell=True,
                    stdout=fh,
                    stderr=subprocess.STDOUT,
                    stdin=subprocess.DEVNULL,
                    start_new_session=True,
                )
            started.append((proc, port, log))
            say(f"server {i} pid {proc.pid}: {cmd} (log {log})")

        t0 = time.monotonic()
        while True:
            for proc, port, log in started:
                if proc.poll() is not None:
                    say(
                        f"server on port {port} exited with {proc.returncode} before it was ready:\n{log_tail(log)}"
                    )
                    return HARNESS_FAILURE
            waiting = [f"port {p}" for p in ns.port if not port_open(p)] + [
                u for u in ns.ready_url if not url_ready(u)
            ]
            if not waiting:
                break
            if time.monotonic() - t0 > ns.timeout:
                say(f"not ready after {ns.timeout:.0f}s: {', '.join(waiting)}")
                for _, _, log in started:
                    say(f"{log}:\n{log_tail(log)}")
                return HARNESS_FAILURE
            time.sleep(0.2)  # polling a readiness condition under a deadline
        say(f"ready after {time.monotonic() - t0:.1f}s; running: {' '.join(command)}")
        try:
            return subprocess.call(command)
        except FileNotFoundError:
            say(f"command not found: {command[0]}")
            return 127
    finally:
        for proc, _, _ in started:
            stop(proc)


if __name__ == "__main__":
    raise SystemExit(main())
