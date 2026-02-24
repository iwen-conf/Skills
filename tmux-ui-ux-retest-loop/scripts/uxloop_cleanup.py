#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path
from typing import Any


def _fail(msg: str, *, code: int = 2) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(code)


def _tmux(args: list[str], *, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["tmux", *args], text=True, capture_output=True, check=check)


def _tmux_exists() -> bool:
    try:
        _tmux(["-V"], check=True)
    except FileNotFoundError:
        return False
    except subprocess.CalledProcessError:
        return False
    return True


def _has_session(session_name: str) -> bool:
    result = subprocess.run(["tmux", "has-session", "-t", session_name], text=True, capture_output=True)
    return result.returncode == 0


def _list_panes(window_target: str) -> list[str]:
    try:
        result = _tmux(["list-panes", "-t", window_target, "-F", "#{pane_id}"], check=True)
    except subprocess.CalledProcessError:
        return []
    panes = [ln.strip() for ln in result.stdout.splitlines() if ln.strip()]
    return panes


def _stop_piping(pane_id: str) -> None:
    # Without a command, tmux stops any existing pipe.
    subprocess.run(["tmux", "pipe-pane", "-t", pane_id], text=True, capture_output=True)


def _send_ctrl_c(pane_id: str) -> None:
    subprocess.run(["tmux", "send-keys", "-t", pane_id, "C-c"], text=True, capture_output=True)


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="uxloop_cleanup.py",
        description="Stop log piping and gracefully interrupt services in a tmux UX loop session.",
    )
    parser.add_argument("--session", default="uxloop", help="tmux session name (default: uxloop)")
    parser.add_argument("--window", default="svc", help="tmux window name (default: svc)")
    parser.add_argument("--graceful-timeout-s", type=int, default=5, help="Seconds to wait before optional kill-session.")
    parser.add_argument("--kill-session", action="store_true", help="Kill the tmux session after graceful interrupt.")
    parser.add_argument("--json", action="store_true", help="Print machine-readable JSON summary.")
    args = parser.parse_args()

    if not _tmux_exists():
        _fail("tmux not found or not working. Install tmux first.")

    session_name = args.session
    window_name = args.window

    if not _has_session(session_name):
        msg = f"UXLOOP_CLEANUP: session not found: {session_name}"
        if args.json:
            print(json.dumps({"session_name": session_name, "status": "not_found"}, ensure_ascii=False))
        else:
            print(msg)
        return

    window_target = f"{session_name}:{window_name}"
    panes = _list_panes(window_target)

    for pane_id in panes:
        _stop_piping(pane_id)
        _send_ctrl_c(pane_id)

    if args.graceful_timeout_s > 0:
        time.sleep(args.graceful_timeout_s)

    killed = False
    if args.kill_session:
        _tmux(["kill-session", "-t", session_name], check=True)
        killed = True

    summary: dict[str, Any] = {
        "session_name": session_name,
        "window_name": window_name,
        "window_target": window_target,
        "panes": panes,
        "kill_session": bool(args.kill_session),
        "killed": killed,
    }

    if args.json:
        print(json.dumps(summary, indent=2, ensure_ascii=False))
    else:
        print(f"UXLOOP_CLEANUP: session={session_name} window={window_name} panes={len(panes)} killed={killed}")


if __name__ == "__main__":
    main()

