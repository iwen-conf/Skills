"""Per-session record of the repositories an agent touched and their state at first touch.

Stop checks every recorded repository, so work done in another repository or through the
shell is still held to verification. Logs live outside repositories (ARC_POLICY_STATE_DIR,
default ~/.cache/arc-policy) and are append-only JSON lines, one per repository.
"""

from __future__ import annotations

import json
import os
import re
import time
from collections.abc import Iterable
from pathlib import Path

from arc_policy.classify import git_command, shell_commands
from arc_policy.gitutil import dirty_digests
from arc_policy.paths import git_root

STATE_ENV = "ARC_POLICY_STATE_DIR"
RETENTION_SECONDS = 14 * 24 * 3600
# Absolute or home-relative paths anywhere in a command, including script bodies.
_PATH = re.compile(r"(?<![\w.$-])(~?/[^\s'\"`;|&()<>]+)")
_MAX_PATHS = 64


def _sessions_dir() -> Path:
    value = os.environ.get(STATE_ENV)
    base = Path(value) if value else Path.home() / ".cache" / "arc-policy"
    return base / "sessions"


def _log(session: str) -> Path:
    return _sessions_dir() / (re.sub(r"[^A-Za-z0-9_.-]", "_", session)[:128] + ".jsonl")


def session_baselines(session: str) -> dict[Path, dict[str, str]]:
    """Dirty-file digests of each repository when the session first touched it."""
    path = _log(session)
    if not path.is_file():
        return {}
    baselines: dict[Path, dict[str, str]] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            entry = json.loads(line)
            baselines.setdefault(Path(entry["root"]), entry["digests"])
    return baselines


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
    for path in directory.glob("*.jsonl"):
        if path.stat().st_mtime < cutoff:
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
