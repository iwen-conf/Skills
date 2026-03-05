#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import socket
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


@dataclass(frozen=True)
class ReadyCheck:
    type: str
    url: str | None = None
    expect_status: int = 200
    expect_text: str | None = None
    host: str | None = None
    port: int | None = None
    command: str | None = None
    timeout_s: int | None = None


@dataclass(frozen=True)
class Service:
    name: str
    command: str
    cwd: str | None = None
    env: dict[str, str] | None = None
    ready_check: ReadyCheck | None = None


def _fail(msg: str, *, code: int = 2) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(code)


def _tmux(args: list[str], *, capture_output: bool = True, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["tmux", *args],
        text=True,
        capture_output=capture_output,
        check=check,
    )


def _tmux_exists() -> bool:
    try:
        _tmux(["-V"], capture_output=True, check=True)
    except FileNotFoundError:
        return False
    except subprocess.CalledProcessError:
        return False
    return True


def _has_session(session_name: str) -> bool:
    result = subprocess.run(
        ["tmux", "has-session", "-t", session_name],
        text=True,
        capture_output=True,
    )
    return result.returncode == 0


def _list_windows(session_name: str) -> list[str]:
    result = _tmux(["list-windows", "-t", session_name, "-F", "#{window_name}"], capture_output=True, check=True)
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def _list_panes(window_target: str) -> list[tuple[int, str]]:
    result = _tmux(["list-panes", "-t", window_target, "-F", "#{pane_index} #{pane_id}"], capture_output=True, check=True)
    panes: list[tuple[int, str]] = []
    for line in result.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        idx_s, pane_id = line.split(maxsplit=1)
        panes.append((int(idx_s), pane_id))
    panes.sort(key=lambda x: x[0])
    return panes


def _ensure_session(session_name: str, window_name: str) -> None:
    if _has_session(session_name):
        return
    _tmux(["new-session", "-d", "-s", session_name, "-n", window_name], capture_output=True, check=True)


def _ensure_window(session_name: str, window_name: str) -> None:
    if window_name in _list_windows(session_name):
        return
    _tmux(["new-window", "-t", session_name, "-n", window_name], capture_output=True, check=True)


def _reset_window(session_name: str, window_name: str) -> None:
    windows = _list_windows(session_name)
    if window_name not in windows:
        _ensure_window(session_name, window_name)
        return
    if len(windows) == 1:
        _tmux(["kill-session", "-t", session_name], capture_output=True, check=True)
        _tmux(["new-session", "-d", "-s", session_name, "-n", window_name], capture_output=True, check=True)
        return

    _tmux(["kill-window", "-t", f"{session_name}:{window_name}"], capture_output=True, check=True)
    _tmux(["new-window", "-t", session_name, "-n", window_name], capture_output=True, check=True)


def _ensure_panes(window_target: str, count: int) -> list[str]:
    panes = _list_panes(window_target)
    while len(panes) < count:
        _tmux(["split-window", "-t", window_target, "-v"], capture_output=True, check=True)
        panes = _list_panes(window_target)
    _tmux(["select-layout", "-t", window_target, "tiled"], capture_output=True, check=True)
    panes = _list_panes(window_target)
    return [pane_id for _, pane_id in panes[:count]]


def _validate_env(env: dict[str, str]) -> None:
    for k in env:
        if not re.match(r"^[A-Za-z_][A-Za-z0-9_]*$", k):
            _fail(f"Invalid env var name: {k!r}")


def _service_command(service: Service) -> str:
    cmd = service.command
    if service.env:
        _validate_env(service.env)
        env_prefix = " ".join([f"{k}={shlex.quote(v)}" for k, v in service.env.items()])
        cmd = f"{env_prefix} {cmd}"
    if service.cwd:
        return f"cd {shlex.quote(service.cwd)} && {cmd}"
    return cmd


def _pipe_pane(pane_id: str, log_file: Path) -> None:
    log_file.parent.mkdir(parents=True, exist_ok=True)
    pipe_cmd = f"cat >> {shlex.quote(str(log_file))}"
    _tmux(["pipe-pane", "-t", pane_id, pipe_cmd], capture_output=True, check=True)


def _start_service_in_pane(pane_id: str, service: Service, *, restart: bool) -> None:
    _tmux(["select-pane", "-t", pane_id, "-T", service.name], capture_output=True, check=True)
    if restart:
        _tmux(["send-keys", "-t", pane_id, "C-c"], capture_output=True, check=True)
        _tmux(["send-keys", "-t", pane_id, "C-u"], capture_output=True, check=True)
    _tmux(["send-keys", "-t", pane_id, _service_command(service), "C-m"], capture_output=True, check=True)


def _http_ready(check: ReadyCheck) -> tuple[bool, str]:
    if not check.url:
        return False, "ready_check.url is required for type=http"
    try:
        request = Request(check.url, headers={"User-Agent": "uxloop_tmux/1.0"})
        with urlopen(request, timeout=5) as response:
            status = getattr(response, "status", None) or response.getcode()
            body = response.read(4096) if check.expect_text else b""
    except HTTPError as e:
        return False, f"http status={e.code}"
    except URLError as e:
        return False, f"http error={e.reason}"
    except Exception as e:  # noqa: BLE001
        return False, f"http error={e}"

    if status != check.expect_status:
        return False, f"http status={status} expected={check.expect_status}"
    if check.expect_text and check.expect_text.encode("utf-8") not in body:
        return False, "http body missing expected text"
    return True, "ok"


def _tcp_ready(check: ReadyCheck) -> tuple[bool, str]:
    if not check.host or check.port is None:
        return False, "ready_check.host and ready_check.port are required for type=tcp"
    try:
        with socket.create_connection((check.host, check.port), timeout=2):
            return True, "ok"
    except OSError as e:
        return False, f"tcp error={e.strerror or e}"


def _cmd_ready(check: ReadyCheck) -> tuple[bool, str]:
    if not check.command:
        return False, "ready_check.command is required for type=cmd"
    result = subprocess.run(check.command, shell=True, text=True, capture_output=True)
    if result.returncode == 0:
        return True, "ok"
    msg = (result.stderr or result.stdout).strip().splitlines()[-1:] or [f"exit={result.returncode}"]
    return False, msg[0]


def _wait_ready(name: str, check: ReadyCheck, *, default_timeout_s: int) -> None:
    timeout_s = check.timeout_s if check.timeout_s is not None else default_timeout_s
    deadline = time.monotonic() + timeout_s
    last_msg = "not checked"
    while time.monotonic() < deadline:
        if check.type == "http":
            ok, last_msg = _http_ready(check)
        elif check.type == "tcp":
            ok, last_msg = _tcp_ready(check)
        elif check.type == "cmd":
            ok, last_msg = _cmd_ready(check)
        else:
            _fail(f"Unsupported ready_check.type: {check.type!r} (expected http|tcp|cmd)")
        if ok:
            print(f"READY: {name}: ok", file=sys.stderr)
            return
        time.sleep(1)
    _fail(f"Service not ready: {name}: {last_msg}", code=3)


def _parse_ready_check(raw: Any) -> ReadyCheck | None:
    if raw is None:
        return None
    if not isinstance(raw, dict):
        _fail("services[].ready_check must be an object")
    check_type = raw.get("type")
    if not isinstance(check_type, str) or not check_type:
        _fail("services[].ready_check.type must be a non-empty string")

    if check_type == "http":
        url = raw.get("url")
        if not isinstance(url, str) or not url:
            _fail("services[].ready_check.url must be a non-empty string for type=http")
        expect_status = raw.get("expect_status", 200)
        if not isinstance(expect_status, int):
            _fail("services[].ready_check.expect_status must be an int")
        expect_text = raw.get("expect_text")
        if expect_text is not None and (not isinstance(expect_text, str) or not expect_text):
            _fail("services[].ready_check.expect_text must be a non-empty string when set")
        timeout_s = raw.get("timeout_s")
        if timeout_s is not None and not isinstance(timeout_s, int):
            _fail("services[].ready_check.timeout_s must be an int when set")
        return ReadyCheck(type="http", url=url, expect_status=expect_status, expect_text=expect_text, timeout_s=timeout_s)

    if check_type == "tcp":
        host = raw.get("host")
        port = raw.get("port")
        if not isinstance(host, str) or not host:
            _fail("services[].ready_check.host must be a non-empty string for type=tcp")
        if not isinstance(port, int):
            _fail("services[].ready_check.port must be an int for type=tcp")
        timeout_s = raw.get("timeout_s")
        if timeout_s is not None and not isinstance(timeout_s, int):
            _fail("services[].ready_check.timeout_s must be an int when set")
        return ReadyCheck(type="tcp", host=host, port=port, timeout_s=timeout_s)

    if check_type == "cmd":
        command = raw.get("command")
        if not isinstance(command, str) or not command:
            _fail("services[].ready_check.command must be a non-empty string for type=cmd")
        timeout_s = raw.get("timeout_s")
        if timeout_s is not None and not isinstance(timeout_s, int):
            _fail("services[].ready_check.timeout_s must be an int when set")
        return ReadyCheck(type="cmd", command=command, timeout_s=timeout_s)

    _fail(f"Unsupported services[].ready_check.type: {check_type!r} (expected http|tcp|cmd)")


def _load_config(config_path: Path) -> tuple[str, str, list[Service]]:
    try:
        raw = json.loads(config_path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        _fail(f"Config not found: {config_path}")
    except json.JSONDecodeError as e:
        _fail(f"Invalid JSON in {config_path}: {e}")

    if not isinstance(raw, dict):
        _fail("Config root must be an object")

    session_name = raw.get("session_name", "uxloop")
    window_name = raw.get("window_name", "svc")
    if not isinstance(session_name, str) or not session_name:
        _fail("session_name must be a non-empty string")
    if not isinstance(window_name, str) or not window_name:
        _fail("window_name must be a non-empty string")

    raw_services = raw.get("services")
    if not isinstance(raw_services, list) or not raw_services:
        _fail("services must be a non-empty array")

    services: list[Service] = []
    for i, item in enumerate(raw_services):
        if not isinstance(item, dict):
            _fail(f"services[{i}] must be an object")
        name = item.get("name")
        command = item.get("command")
        if not isinstance(name, str) or not name:
            _fail(f"services[{i}].name must be a non-empty string")
        if not isinstance(command, str) or not command:
            _fail(f"services[{i}].command must be a non-empty string")
        cwd = item.get("cwd")
        if cwd is not None and (not isinstance(cwd, str) or not cwd):
            _fail(f"services[{i}].cwd must be a non-empty string when set")
        env = item.get("env")
        if env is not None:
            if not isinstance(env, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in env.items()):
                _fail(f"services[{i}].env must be an object<string,string> when set")
        ready_check = _parse_ready_check(item.get("ready_check"))
        services.append(Service(name=name, command=command, cwd=cwd, env=env, ready_check=ready_check))

    return session_name, window_name, services


def _generate_run_id() -> str:
    ts = datetime.now(timezone.utc).astimezone().strftime("%Y-%m-%d_%H-%M-%S")
    suffix = uuid.uuid4().hex[:6]
    return f"{ts}_{suffix}"


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="uxloop_tmux.py",
        description="Bootstrap (or restart) a tmux session with service panes + log piping, for UI/UX retest loops.",
    )
    parser.add_argument("--config", required=True, help="Path to uxloop JSON config (see assets/uxloop.config.example.json).")
    parser.add_argument("--run-id", help="Run identifier for log folder naming (defaults to timestamp+random).")
    parser.add_argument("--logs-dir", help="Override log output directory (default: logs/uxloop/<run_id>/).")
    parser.add_argument("--reset-window", action="store_true", help="Recreate the tmux window (deterministic layout).")
    parser.add_argument("--no-restart", action="store_true", help="Do not send Ctrl-C before starting commands.")
    parser.add_argument("--wait-ready", action="store_true", help="Wait for each service ready_check before returning.")
    parser.add_argument("--ready-timeout-s", type=int, default=90, help="Default ready_check timeout in seconds.")
    parser.add_argument("--json", action="store_true", help="Print machine-readable JSON summary to stdout.")
    parser.add_argument("--attach", action="store_true", help="Attach to the tmux session at the end.")
    args = parser.parse_args()

    if not _tmux_exists():
        _fail("tmux not found or not working. Install tmux first.")

    config_path = Path(args.config).expanduser()
    session_name, window_name, services = _load_config(config_path)

    run_id = args.run_id or os.environ.get("UXLOOP_RUN_ID") or _generate_run_id()
    logs_dir = Path(args.logs_dir).expanduser() if args.logs_dir else Path("logs") / "uxloop" / run_id
    window_target = f"{session_name}:{window_name}"

    _ensure_session(session_name, window_name)
    if args.reset_window:
        _reset_window(session_name, window_name)
    else:
        _ensure_window(session_name, window_name)

    _tmux(["set-option", "-t", session_name, "history-limit", "20000"], capture_output=True, check=True)

    pane_ids = _ensure_panes(window_target, len(services))

    summary_services: list[dict[str, Any]] = []
    for pane_id, service in zip(pane_ids, services, strict=True):
        log_file = logs_dir / f"{service.name}.log"
        _pipe_pane(pane_id, log_file)
        _start_service_in_pane(pane_id, service, restart=not args.no_restart)
        summary_services.append(
            {
                "name": service.name,
                "pane_id": pane_id,
                "pane_target": f"{window_target}.<auto>",
                "log_file": str(log_file),
                "ready_check": None if service.ready_check is None else {"type": service.ready_check.type},
            }
        )

    if args.wait_ready:
        for service in services:
            if service.ready_check is None:
                continue
            _wait_ready(service.name, service.ready_check, default_timeout_s=args.ready_timeout_s)

    summary: dict[str, Any] = {
        "run_id": run_id,
        "session_name": session_name,
        "window_name": window_name,
        "window_target": window_target,
        "logs_dir": str(logs_dir),
        "services": summary_services,
        "attach_cmd": f"tmux attach -t {session_name}",
        "cleanup_cmd": (
            f"python tmux-ui-ux-retest-loop/scripts/uxloop_cleanup.py --session {session_name} --window {window_name} --kill-session"
        ),
    }

    if args.json:
        print(json.dumps(summary, indent=2, ensure_ascii=False))
    else:
        print(f"UXLOOP: session={session_name} window={window_name} logs_dir={logs_dir}")
        for s in summary_services:
            print(f"UXLOOP: service={s['name']} pane_id={s['pane_id']} log={s['log_file']}")
        print(f"UXLOOP: next: tmux attach -t {session_name}")
        print(f"UXLOOP: cleanup: {summary['cleanup_cmd']}")

    if args.attach:
        os.execvp("tmux", ["tmux", "attach", "-t", session_name])


if __name__ == "__main__":
    main()
