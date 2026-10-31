"""Per-session record of the repositories an agent touched and the paths its tool calls wrote.

Stop checks every recorded repository, so work done in another repository or through the
shell is still held to verification. A shell call's writes are the dirty-file changes between
its PreToolUse and PostToolUse (a tool-call window); edit tools record their target paths.
State lives outside repositories (ARC_POLICY_STATE_DIR, default ~/.cache/arc-policy): an
append-only JSON-lines log per session plus its open windows.
"""

from __future__ import annotations

import json
import os
import re
import sys
import tempfile
import time
from collections.abc import Iterable, Iterator
from contextlib import contextmanager
from pathlib import Path

from arc_policy.classify import git_command, shell_commands
from arc_policy.gitutil import dirty_digests, dirty_stats
from arc_policy.paths import git_root

STATE_ENV = "ARC_POLICY_STATE_DIR"
RETENTION_SECONDS = 14 * 24 * 3600
# Absolute or home-relative paths anywhere in a command, including script bodies.
_PATH = re.compile(r"(?<![\w.$-])(~?/[^\s'\"`;|&()<>]+)")
_MAX_PATHS = 64
_STATE_SUFFIXES = (".jsonl", ".stop", ".windows.json", ".lock", ".tmp")


def _sessions_dir() -> Path:
    value = os.environ.get(STATE_ENV)
    base = Path(value) if value else Path.home() / ".cache" / "arc-policy"
    return base / "sessions"


def _file(session: str, suffix: str) -> Path:
    return _sessions_dir() / (re.sub(r"[^A-Za-z0-9_.-]", "_", session)[:128] + suffix)


def _log(session: str) -> Path:
    return _file(session, ".jsonl")


def last_stop_notice(session: str) -> str:
    """Key of the pending-verification state this session was last told about."""
    path = _file(session, ".stop")
    return path.read_text(encoding="utf-8").strip() if path.is_file() else ""


def record_stop_notice(session: str, key: str) -> None:
    path = _file(session, ".stop")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(key + "\n", encoding="utf-8")


def _entries(session: str) -> list[dict]:
    path = _log(session)
    if not path.is_file():
        return []
    return [
        json.loads(line)
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]


def _append(session: str, entries: list[dict]) -> None:
    if not entries:
        return
    path = _log(session)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as stream:
        for entry in entries:
            stream.write(json.dumps(entry, ensure_ascii=True) + "\n")


def session_baselines(session: str) -> dict[Path, dict[str, str]]:
    """Dirty-file digests of each repository when the session first touched it."""
    baselines: dict[Path, dict[str, str]] = {}
    for entry in _entries(session):
        if "digests" in entry:
            baselines.setdefault(Path(entry["root"]), entry["digests"])
    return baselines


def authored_paths(session: str) -> dict[Path, set[str]] | None:
    """Paths this session's tool calls wrote, per repository.

    Exact edit targets are known without PostToolUse. Shell windows become usable after
    PostToolUse or Stop closes them; older untracked sessions retain first-touch fallback.
    """
    if not session:
        return None
    authored: dict[Path, set[str]] = {}
    for entry in _entries(session):
        if "authored" in entry:
            authored.setdefault(Path(entry["root"]), set()).update(entry["authored"])
    return authored if authored or _read_windows(session)["post"] else None


def record_authored(session: str, root: Path, paths: Iterable[str]) -> None:
    paths = sorted(set(paths))
    if session and paths:
        _append(session, [{"root": str(root), "authored": paths}])


@contextmanager
def _locked(session: str) -> Iterator[None]:
    path = _file(session, ".lock")
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a+b") as stream:
        stream.seek(0)
        if sys.platform == "win32":
            import msvcrt

            msvcrt.locking(stream.fileno(), msvcrt.LK_LOCK, 1)
        else:
            import fcntl

            fcntl.flock(stream, fcntl.LOCK_EX)
        try:
            yield
        finally:
            if sys.platform == "win32":
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(stream, fcntl.LOCK_UN)


def _read_windows(session: str) -> dict:
    path = _file(session, ".windows.json")
    if not path.is_file():
        return {"post": False, "open": {}}
    return json.loads(path.read_text(encoding="utf-8"))


@contextmanager
def _windows(session: str) -> Iterator[dict]:
    """Tool-call windows, changed under the session lock and replaced atomically for readers."""
    path = _file(session, ".windows.json")
    with _locked(session):
        state = _read_windows(session)
        yield state
        with tempfile.NamedTemporaryFile(
            "w",
            encoding="utf-8",
            dir=path.parent,
            prefix=path.name,
            suffix=".tmp",
            delete=False,
        ) as stream:
            json.dump(state, stream, ensure_ascii=True)
        os.replace(stream.name, path)


def open_window(session: str, call: str, roots: Iterable[Path]) -> None:
    """Snapshot dirty files before an allowed tool call runs; a repeated call keeps its first snapshot."""
    roots = sorted(set(roots))
    if not session:
        return
    snapshot = {str(root): dirty_stats(root) for root in roots}
    with _windows(session) as state:
        state["open"].setdefault(call, snapshot)


def close_window(session: str, call: str) -> None:
    """Charge the paths that changed while the tool call ran to this session."""
    if not session:
        return
    with _windows(session) as state:
        state["post"] = True
        window = state["open"].pop(call, None)
    _charge(session, window or {})


def close_open_windows(session: str) -> None:
    """At Stop no call of the turn still runs; charge the windows failed calls left open."""
    if not session or not _file(session, ".windows.json").is_file():
        return
    with _windows(session) as state:
        windows = list(state["open"].values())
        state["open"] = {}
        if windows:
            state["post"] = True
    for window in windows:
        _charge(session, window)


def _charge(session: str, window: dict[str, dict[str, str]]) -> None:
    for root, before in window.items():
        record_authored(
            session,
            Path(root),
            [
                path
                for path, stat in dirty_stats(Path(root)).items()
                if before.get(path) != stat
            ],
        )


def touch(session: str, roots: Iterable[Path]) -> None:
    """Record repositories this session reaches for the first time, before it changes them."""
    if not session:
        return
    known = session_baselines(session)
    fresh = sorted({root for root in roots if root not in known})
    if not fresh:
        return
    path = _log(session)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as stream:
        for root in fresh:
            entry = {"root": str(root), "digests": dirty_digests(root)}
            stream.write(json.dumps(entry, ensure_ascii=True) + "\n")


def changed_since(root: Path, baseline: dict[str, str]) -> list[str]:
    """Uncommitted paths whose content differs from the session's first touch."""
    return [
        path
        for path, digest in dirty_digests(root).items()
        if baseline.get(path) != digest
    ]


def prune() -> None:
    directory = _sessions_dir()
    if not directory.is_dir():
        return
    cutoff = time.time() - RETENTION_SECONDS
    for path in directory.iterdir():
        if path.name.endswith(_STATE_SUFFIXES) and path.stat().st_mtime < cutoff:
            path.unlink()


def shell_roots(command: str, directory: Path) -> set[Path]:
    """Repositories a shell command can reach: its directory, cd and -C targets, named paths."""
    candidates = [directory]
    current = directory
    try:
        commands = shell_commands(command)
    except ValueError:
        commands = []
    for argv in commands:
        if len(argv) > 1 and argv[0] == "cd":
            current = current / Path(argv[1]).expanduser()
            candidates.append(current)
        parsed = git_command(argv)
        options = parsed[0] if parsed else argv
        for index, option in enumerate(options[:-1]):
            if option in {"-C", "--cwd"}:
                candidates.append(current / Path(options[index + 1]).expanduser())
    candidates += [Path(match).expanduser() for match in _PATH.findall(command)][
        :_MAX_PATHS
    ]
    return {root for path in candidates for root in [git_root(path)] if root}
