"""Git-level backstop: agents cannot commit or push without a receipt, however Git is invoked.

`task begin` registers config-based hooks (Git 2.54+ `hook.<name>.command`), which run beside
any hook files or core.hooksPath the repository already uses. Humans are not gated: the
check applies only when a host's agent marker is in the environment. A command the user
types into the host (Claude Code `!`) inherits that marker and is checked like the agent's.
"""

from __future__ import annotations

import shlex
from collections.abc import Mapping
from pathlib import Path

from arc_policy.gitutil import git
from arc_policy.store import (
    active_task,
    current_receipt,
    receipt_covers_commits,
    receipt_matches_tree,
)

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
CLI = Path(__file__).resolve().parents[2] / "bin" / "arc-policy"


def is_agent(env: Mapping[str, str]) -> bool:
    return any(
        env.get(name, "").strip().casefold() not in {"", "0", "false"}
        for name in AGENT_MARKERS
    )


def hook_command(event: str, cli: Path = CLI) -> str:
    # Git appends the hook's own arguments to the command, so the CLI call stays last.
    quoted = shlex.quote(str(cli))
    return (
        f"[ -x {quoted} ] || {{ echo 'arc-policy: CLI missing at '{quoted}"
        f"'; reinstall it or remove hook.arc-policy-{event}' >&2; exit 1; }}; "
        f"exec {quoted} git-hook {event}"
    )


def install_git_hooks(root: Path, cli: Path = CLI) -> None:
    for event in HOOK_EVENTS:
        name = f"hook.arc-policy-{event}"
        for key, value in (("command", hook_command(event, cli)), ("event", event)):
            result = git(
                root, "config", "--local", "--replace-all", f"{name}.{key}", value
            )
            if result.returncode:
                raise RuntimeError(
                    f"git config {name}.{key} failed: {result.stderr.strip()}"
                )


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
    """Why an agent's commit or push must stop, or None.

    A push also passes when every pushed commit is exactly the tree the receipt bound,
    so committing verified or user-authorized work needs no second verification.
    """
    if not is_agent(env) or receipt_matches_tree(root, current_receipt(root)):
        return None
    if event == "pre-push":
        revisions = pushed_revisions(updates) if updates.strip() else ["HEAD"]
        if revisions and receipt_covers_commits(root, revisions):
            return None
    action = "commit" if event == "pre-commit" else "push"
    task = active_task(root)
    if task is None:
        return (
            f"arc-policy: agent {action} refused in {root}: no Arc task. Run "
            "`arc-policy task begin --skill <skill>`, then `arc-policy task verify`."
        )
    return (
        f"arc-policy: agent {action} refused in {root}: task {task['slug']} has no receipt "
        "matching this tree. Stage the commit, then run `arc-policy task verify`; when the "
        "user explicitly confirmed committing without it, run "
        "`arc-policy task authorize --quote '<their words>'`."
    )
