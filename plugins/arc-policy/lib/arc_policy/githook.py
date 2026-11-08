"""Compatibility helpers for removing the retired per-commit Arc receipt gate.

Task verification and graph evidence are still available explicitly, but ordinary
Git commits and pushes must not trigger or require `arc-policy task verify`.
Only Arc-owned hooks may be uninstalled; preserve other repository hooks.
"""

from __future__ import annotations

import os
import shlex
import subprocess
import tempfile
from collections.abc import Mapping
from functools import lru_cache
from pathlib import Path

from arc_policy.gitutil import git
# Variables each host exports to the shells its agent runs (observed in host sessions or source).
AGENT_MARKERS = (
    "ARC_POLICY_AGENT",
    "AI_AGENT",
    "CLAUDECODE",
    "GROK_AGENT",
    "CODEX_THREAD_ID",
    "CODEX_CI",
    "CURSOR_AGENT",
    "PI_CODING_AGENT",
)
HOOK_EVENTS = ("pre-commit", "pre-push")
PLUGIN = Path(__file__).resolve().parents[2]
FILE_HOOK_MARKER = "# arc-policy managed file hook v1"


def is_agent(env: Mapping[str, str]) -> bool:
    return any(
        env.get(name, "").strip().casefold() not in {"", "0", "false"}
        for name in AGENT_MARKERS
    )


def hook_command(event: str, cli: Path, *, replace_process: bool = True) -> str:
    """Retired: do not install a per-commit/pre-push Arc verifier command."""
    return ":"  # POSIX shell no-op; preserves user Git hooks.



@lru_cache(maxsize=1)
def config_hooks_supported() -> bool:
    """Unknown Git config keys are accepted, so verify actual execution in an empty repo."""
    with tempfile.TemporaryDirectory(prefix="arc-hook-probe-") as temporary:
        root = Path(temporary)
        marker = root / "executed"
        env = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith("GIT_")
        }
        env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
        init = subprocess.run(
            ["git", "init", "-q", str(root)],
            env=env,
            capture_output=True,
            timeout=15,
            check=False,
        )
        if init.returncode:
            raise RuntimeError("unable to initialize Git hook capability probe")
        subprocess.run(
            [
                "git",
                "-C",
                str(root),
                "-c",
                "hook.arc-probe.event=pre-commit",
                "-c",
                "hook.arc-probe.command="
                + "printf ready > "
                + shlex.quote(str(marker)),
                "hook",
                "run",
                "pre-commit",
            ],
            env=env,
            capture_output=True,
            timeout=15,
            check=False,
        )
        return marker.is_file()


def _file_hook(root: Path, event: str) -> Path:
    result = git(root, "rev-parse", "--git-path", f"hooks/{event}")
    if result.returncode:
        raise RuntimeError(result.stderr.strip())
    path = Path(result.stdout.strip())
    return path if path.is_absolute() else root / path


def _file_hook_script(event: str, cli: Path, original: Path) -> str:
    previous = shlex.quote(str(original))
    start = f"#!/bin/sh\n{FILE_HOOK_MARKER}\n"
    if event == "pre-push":
        # Both hooks receive the complete update list. A shell function confines the
        # human early-exit and avoids exec skipping the temporary-file cleanup trap.
        return start + (
            "arc_policy_check() {\n"
            + hook_command(event, cli, replace_process=False)
            + ' "$@"\n}\n'
            "arc_policy_input=$(mktemp) || exit 1\n"
            "trap 'rm -f \"$arc_policy_input\"' EXIT HUP INT TERM\n"
            'cat > "$arc_policy_input" || exit 1\n'
            f'if [ -x {previous} ]; then {previous} "$@" < "$arc_policy_input" || exit $?; fi\n'
            'arc_policy_check "$@" < "$arc_policy_input"\n'
        )
    return start + (
        f'if [ -x {previous} ]; then {previous} "$@" || exit $?; fi\n'
        + hook_command(event, cli)
        + ' "$@"\n'
    )


def remove_git_hooks(root: Path) -> str:
    """Remove only Arc-owned Git commit/push gates, preserving other Git hooks.

    Previous releases installed config hooks (Git 2.54+) or managed file-hook
    wrappers. Clean both layouts when a task starts or changes phase, so an
    already configured repository stops demanding a receipt on each commit.
    """
    for event in HOOK_EVENTS:
        name = f"hook.arc-policy-{event}"
        for key in ("event", "command"):
            result = git(root, "config", "--local", "--unset-all", f"{name}.{key}")
            if result.returncode not in (0, 5):
                raise RuntimeError(
                    f"cannot remove old Arc Git gate {name}.{key}: {result.stderr.strip()}"
                )

        path = _file_hook(root, event)
        # Never modify a symlink or another tool's hook, even if it contains
        # the Arc marker somewhere in its contents.
        if path.is_symlink() or not path.is_file():
            continue
        header = path.read_text(encoding="utf-8", errors="replace").splitlines()[:2]
        if FILE_HOOK_MARKER not in header:
            continue
        original = path.with_name(path.name + ".arc-policy-original")
        if original.is_file() and not original.is_symlink():
            os.replace(original, path)
        elif original.exists() or original.is_symlink():
            raise RuntimeError(f"unsafe original Git hook backup: {original}")
        else:
            path.unlink()
    return "disabled"


def install_git_hooks(root: Path, cli: Path | None = None) -> str:
    """Compatibility alias: commit/push receipt hooks were retired."""
    return remove_git_hooks(root)


ZERO_SHA = frozenset("0")


def pushed_revisions(updates: str) -> list[str] | None:
    """Local commits a pre-push update list sends; None when it deletes a ref or is unreadable."""
    revisions = []
    for line in updates.splitlines():
        fields = line.split()
        if not fields:
            continue
        if len(fields) != 4 or set(fields[1]) <= ZERO_SHA:
            return None
        revisions.append(fields[1])
    return revisions


def refusal(
    root: Path, event: str, env: Mapping[str, str], updates: str = ""
) -> str | None:
    """Legacy hook handler: commit/push no longer requires a verify receipt.

    Explicit task verification, business graph gates and schema migration
    policy remain in arc_policy.verify and arc_policy.classify.
    """
    return None
