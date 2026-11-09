"""Turso-backed Arc Policy document, accessed through the Go arc CLI.

The Python runtime deliberately never opens arc.db with sqlite3. Local Turso
is owned by the official Go driver and a single repository-wide Arc lock.
No fallback to the old JSON is allowed once the DB state row is active.
"""

from __future__ import annotations

import json
import shutil
import subprocess
from pathlib import Path
from typing import Any


class DBState(dict):
    """A dict carrying a CAS version, without embedding metadata in user state."""

    def __init__(self, state: dict[str, Any], version: int) -> None:
        super().__init__(state)
        self.version = version


def _arc_command(root: Path, *args: str, data: str | None = None) -> dict[str, Any]:
    binary = shutil.which("arc")
    if binary is None:
        raise RuntimeError(
            "arc.db is active but the unified arc CLI is missing; refusing a "
            "silent fallback to .arc/policy/state.json"
        )
    try:
        result = subprocess.run(
            [binary, "policy", "state", *args, "--repo", str(root)],
            cwd=root,
            capture_output=True,
            input=data,
            text=True,
            timeout=25,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise RuntimeError(f"arc.db Policy backend unavailable: {exc}") from exc
    if result.returncode:
        raise RuntimeError(
            f"arc.db Policy state command failed (exit {result.returncode}): "
            + result.stderr[-1200:]
        )
    try:
        parsed = json.loads(result.stdout)
    except (TypeError, ValueError) as exc:
        raise RuntimeError("arc.db Policy backend emitted invalid JSON") from exc
    if not isinstance(parsed, dict):
        raise RuntimeError("arc.db Policy backend response must be an object")
    return parsed


def load_db_state(root: Path) -> DBState | None:
    # A project must explicitly activate arc.db. No manifest means old format.
    if not (root / ".arc" / "arc.json").exists():
        return None
    result = _arc_command(root, "get")
    mode = result.get("mode")
    if mode == "legacy":
        return None  # active Arc DB, but Policy has not opted in to migration yet
    if mode != "arc.db":
        raise RuntimeError(f"unexpected Arc Policy storage mode: {mode!r}")
    version, state = result.get("version"), result.get("state")
    if (
        type(version) is not int
        or version < 1
        or not isinstance(state, dict)
    ):
        raise RuntimeError("arc.db Policy state response is incomplete or corrupt")
    return DBState(state, version)


def save_db_state(root: Path, state: DBState) -> None:
    if type(state.version) is not int or state.version < 1:
        raise RuntimeError("arc.db Policy state lacks its CAS version")
    result = _arc_command(
        root,
        "put",
        "--expected-version",
        str(state.version),
        data=json.dumps(dict(state), separators=(",", ":"), ensure_ascii=True),
    )
    next_version = result.get("version")
    if result.get("mode") != "arc.db" or next_version != state.version + 1:
        raise RuntimeError("arc.db Policy CAS update returned an invalid version")
    state.version = next_version
