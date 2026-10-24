"""Behavior tests for Arc/arc:verify/scripts/with_server.py against real local processes."""

from __future__ import annotations

import socket
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HELPER = ROOT / "Arc" / "arc:verify" / "scripts" / "with_server.py"
PY = sys.executable


def free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def listening(port: int) -> bool:
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.5):
            return True
    except OSError:
        return False


def http_server(port: int, root: Path) -> str:
    return f"{PY} -m http.server {port} --bind 127.0.0.1 --directory {root}"


def fetch(port: int, path: str = "/") -> list[str]:
    return [
        PY,
        "-c",
        f"import urllib.request; urllib.request.urlopen('http://127.0.0.1:{port}{path}').read()",
    ]


def run(
    tmp_path: Path, *args: str, timeout: float = 60
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [PY, str(HELPER), "--log-dir", str(tmp_path / "logs"), *args],
        capture_output=True,
        text=True,
        timeout=timeout,
    )


def test_runs_command_against_a_ready_server_and_stops_it(tmp_path: Path) -> None:
    port = free_port()
    (tmp_path / "health.txt").write_text("ok", encoding="utf-8")

    result = run(
        tmp_path,
        "--server",
        http_server(port, tmp_path),
        "--port",
        str(port),
        "--ready-url",
        f"http://127.0.0.1:{port}/health.txt",
        "--",
        *fetch(port, "/health.txt"),
    )

    assert result.returncode == 0, result.stderr
    assert "ready after" in result.stderr
    assert not listening(port)
    assert (tmp_path / "logs" / f"server-1-{port}.log").is_file()


def test_command_exit_status_is_passed_through(tmp_path: Path) -> None:
    port = free_port()

    result = run(
        tmp_path,
        "--server",
        http_server(port, tmp_path),
        "--port",
        str(port),
        "--",
        PY,
        "-c",
        "raise SystemExit(3)",
    )

    assert result.returncode == 3
    assert not listening(port)


def test_busy_port_is_a_harness_failure_unless_reuse_is_explicit(
    tmp_path: Path,
) -> None:
    port = free_port()
    existing = subprocess.Popen(
        http_server(port, tmp_path).split(),
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        deadline = time.monotonic() + 10
        while not listening(port) and time.monotonic() < deadline:
            time.sleep(0.05)  # poll the listening condition under a deadline
        refused = run(
            tmp_path, "--server", "exit 0", "--port", str(port), "--", *fetch(port)
        )
        reused = run(
            tmp_path,
            "--server",
            "exit 0",
            "--port",
            str(port),
            "--reuse-running",
            "--",
            *fetch(port),
        )
    finally:
        existing.terminate()
        existing.wait(timeout=10)

    assert refused.returncode == 125
    assert "already in use" in refused.stderr
    assert reused.returncode == 0, reused.stderr
    assert "unverified" in reused.stderr


def test_server_that_dies_before_ready_reports_its_log(tmp_path: Path) -> None:
    port = free_port()

    result = run(
        tmp_path,
        "--server",
        "echo boot failed: missing DATABASE_URL; exit 4",
        "--port",
        str(port),
        "--",
        PY,
        "-c",
        "print('must not run')",
    )

    assert result.returncode == 125
    assert "exited with 4" in result.stderr
    assert "boot failed: missing DATABASE_URL" in result.stderr
    assert "must not run" not in result.stdout


def test_ready_url_that_never_succeeds_times_out(tmp_path: Path) -> None:
    port = free_port()

    result = run(
        tmp_path,
        "--server",
        http_server(port, tmp_path),
        "--port",
        str(port),
        "--ready-url",
        f"http://127.0.0.1:{port}/missing",
        "--timeout",
        "2",
        "--",
        *fetch(port),
    )

    assert result.returncode == 125
    assert "not ready after 2s" in result.stderr
    assert not listening(port)


def test_missing_command_is_a_usage_error(tmp_path: Path) -> None:
    result = run(tmp_path, "--server", "true", "--port", "1")

    assert result.returncode == 2
    assert "put the test command after --" in result.stderr
