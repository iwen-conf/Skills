from __future__ import annotations

import json
import shlex
import shutil
import subprocess
from pathlib import Path
from typing import Any

from arc_policy.classify import (
    classify,
    command_text,
    git_command,
    looks_like_completion,
    shell_commands,
    tool_kind,
)
from arc_policy.detect import detect_languages, recommended_skill
from arc_policy.fallback import deny as _deny
from arc_policy.fallback import fallback_output
from arc_policy.gitutil import dirty_paths, git, tree_fingerprint
from arc_policy.host import (
    edit_targets,
    event_name,
    last_assistant_message,
    payload_cwd,
    session_id,
    stop_reason,
    tool_input,
    tool_name,
)
from arc_policy.paths import git_root, repo_root
from arc_policy.policy import load_for_repo
from arc_policy.session import (
    changed_since,
    prune,
    session_baselines,
    shell_roots,
    touch,
)
from arc_policy.store import (
    active_task,
    child_tasks,
    current_receipt,
    delivered_receipt,
    receipt_matches_tree,
)

SKILLS = "<arc-code|arc-explore|arc-plan|arc-verify|arc-audit|arc-vet>"


def _session_context(text: str) -> dict[str, Any]:
    return {
        "systemMessage": text,
        "additional_context": text,
        "additionalContext": text,
        "hookSpecificOutput": {
            "hookEventName": "SessionStart",
            "additionalContext": text,
        },
    }


def _stop_block(reason: str) -> dict[str, Any]:
    return {
        "decision": "block",
        "reason": reason,
        "followup_message": reason,
    }


def _stop_remind(text: str) -> dict[str, Any]:
    return {"systemMessage": text}


def _cli_ok() -> bool:
    return shutil.which("arc-policy") is not None


def _cli(root: Path, command: str) -> str:
    return f"arc-policy --cwd {shlex.quote(str(root))} {command}"


def _policy_for(root: Path) -> dict[str, Any]:
    return load_for_repo(root, languages=detect_languages(root))


def handle_session_start(cwd: Path, policy: dict[str, Any]) -> dict[str, Any]:
    languages = detect_languages(cwd, policy)
    skill = recommended_skill(languages, policy)
    task = active_task(cwd)
    delivered = delivered_receipt(cwd, task) if task else None
    cli = (
        "arc-policy is on PATH"
        if _cli_ok()
        else "arc-policy is NOT on PATH; install with plugins/arc-policy/README.md"
    )
    if git_root(cwd) is None:
        tasks = [
            f"{root}: {child['slug']}"
            + (" (delivered)" if delivered_receipt(root, child) else "")
            for root, child in child_tasks(cwd).items()
        ]
        task_line = (
            f"{cwd} is not a Git repository; Arc tasks belong to the repository that owns the edited files. "
            f"Before editing source in a repository: arc-policy --cwd <repo> task begin --skill {skill}. "
            f"Active tasks: {', '.join(tasks) or 'none'}"
        )
    elif not task:
        task_line = (
            "Active task: none. Before editing source: arc-policy task begin --skill "
            + skill
        )
    elif delivered:
        task_line = (
            f"Active task: {task['slug']} was delivered at {delivered['head'][:12]} and does not cover new edits. "
            f"Before editing source: arc-policy task begin --skill {skill} --slug <slug>"
        )
    else:
        task_line = f"Active task: {task['slug']} skill={task['skill']}"
    text = "\n".join(
        [
            "Arc Policy Runtime is active.",
            f"Detected stack: {', '.join(languages) or 'unknown'}",
            f"Recommended skill: {skill}",
            cli,
            task_line,
            "Write/Edit are allowed after begin. git commit/push and schema migrations need `arc-policy task verify`.",
            "Delivery: arc-policy task verify",
        ]
    )
    return _session_context(text)


def _edit_roots(payload: dict[str, Any], directory: Path) -> list[Path]:
    # A path outside every Git work tree has no task scope; `task begin` needs Git.
    return sorted(
        {
            root
            for target in edit_targets(tool_input(payload))
            for root in [git_root(directory / Path(target).expanduser())]
            if root
        }
    )


def _guard_edit(
    cwd: Path, policy: dict[str, Any], payload: dict[str, Any], directory: Path
) -> dict[str, Any] | None:
    """Tasks are per repository: check the one that owns every edited path, not the session's."""
    if edit_targets(tool_input(payload)):
        roots = _edit_roots(payload, directory)
    elif git_root(cwd) is None:
        return _deny(
            f"cannot find the edited path in {tool_name(payload)} input, and {cwd} is not a Git "
            "repository; Arc tasks are per repository. Edit with a path-bearing host tool."
        )
    else:
        roots = [cwd]
    for root in roots:
        task = active_task(root)
        delivered = delivered_receipt(root, task) if task else None
        decision = classify(
            tool_name=tool_name(payload),
            tool_input=tool_input(payload),
            policy=policy if root == cwd else _policy_for(root),
            has_task=task is not None and delivered is None,
            receipt_ok=False,
        )
        if decision["action"] != "deny":
            continue
        if task and delivered:
            return _deny(
                f"Arc task {task['slug']} in {root} was delivered at {delivered['head'][:12]}. "
                f"Run `{_cli(root, f'task begin --skill {SKILLS} --slug <slug>')}` before editing source. "
                "Do not route the edit through the shell instead."
            )
        return _deny(
            f"no active Arc task in {root}. Run `{_cli(root, f'task begin --skill {SKILLS}')}` "
            "before editing source. Do not route the edit through the shell instead."
        )
    return None


def handle_pre_tool_use(
    cwd: Path,
    policy: dict[str, Any],
    payload: dict[str, Any],
    *,
    working_directory: Path | None = None,
) -> dict[str, Any] | None:
    directory = working_directory or cwd
    kind = tool_kind(tool_name(payload))
    session = session_id(payload)
    if kind == "mutate":
        fallback = git_root(cwd)
        touch(
            session, _edit_roots(payload, directory) or ([fallback] if fallback else [])
        )
        return _guard_edit(cwd, policy, payload, directory)
    if kind == "shell":
        if payload.get("toolInputTruncated") is True:
            return _deny(
                "the host truncated this shell command (input over 128 KiB), so its hidden "
                "part cannot be checked. Split it into smaller commands."
            )
        touch(session, shell_roots(command_text(tool_input(payload)), directory))
    has_task = active_task(cwd) is not None
    decision = classify(
        tool_name=tool_name(payload),
        tool_input=tool_input(payload),
        policy=policy,
        has_task=has_task,
        receipt_ok=False,
    )
    if decision["action"] == "deny" and decision["operation"] in {
        "commit",
        "schema_migration",
    }:
        targets: set[Path] = set()
        try:
            commands = shell_commands(
                command_text(tool_input(payload)), require_known_cwd=True
            )
        except ValueError as exc:
            return _deny(str(exc))
        for argv in commands:
            parsed = git_command(argv)
            if parsed and parsed[1] and parsed[1][0] in {"commit", "push"}:
                result = git(directory, *parsed[0], "rev-parse", "--show-toplevel")
                if result.returncode:
                    return _deny(
                        "cannot resolve Git target repository; verification remains pending"
                    )
                targets.add(Path(result.stdout.strip()).resolve())
            elif (
                classify(
                    tool_name="shell",
                    tool_input={"cmd": shlex.join(argv)},
                    policy=policy,
                    has_task=True,
                    receipt_ok=False,
                )["operation"]
                == "schema_migration"
            ):
                targets.add(repo_root(directory))
        if decision["operation"] == "schema_migration" or not targets:
            targets.add(repo_root(directory))
        if all(
            receipt_matches_tree(target, current_receipt(target)) for target in targets
        ):
            decision = classify(
                tool_name=tool_name(payload),
                tool_input=tool_input(payload),
                policy=policy,
                has_task=has_task,
                receipt_ok=True,
            )
    if decision["action"] == "deny":
        return _deny(decision["reason"])
    return None


_STOP_STRICTNESS = {"remind": 0, "block_on_claim": 1, "block": 2}


def _task_reminder(
    root: Path, task: dict[str, Any], delivered: dict[str, Any] | None
) -> str | None:
    begin_dirty = set(task.get("begin_dirty") or [])
    if delivered:
        covered = begin_dirty | set(delivered.get("dirty") or [])
        new_changes = [path for path in dirty_paths(root) if path not in covered]
        if not new_changes:
            return None
        return (
            f"Arc task {task['slug']} was delivered at {delivered['head'][:12]} and does not cover these changes in {root}. "
            f"Run `{_cli(root, 'task begin --skill <skill> --slug <slug>')}`, then `{_cli(root, 'task verify')}` "
            f"before claiming delivery or committing. Changed paths: {', '.join(new_changes[:8])}."
        )
    fingerprint = tree_fingerprint(root)
    if (
        task.get("begin_digest") == fingerprint["digest"]
        and task.get("begin_head") == fingerprint["head"]
    ):
        return None
    changed = dirty_paths(root)
    new_changes = [path for path in changed if path not in begin_dirty]
    if not new_changes and not changed:
        return None
    return (
        f"Arc task {task['slug']} is unverified in {root}. Run `{_cli(root, 'task verify')}` before claiming delivery "
        f"or committing. Changed paths: {', '.join(new_changes[:8]) or 'worktree dirty'}."
    )


def _stop_reminder(
    root: Path, policy: dict[str, Any], baseline: dict[str, str] | None
) -> str | None:
    """What still needs verification in root; baseline is the session's first-touch state."""
    if not bool((policy.get("workflow") or {}).get("require_verification")):
        return None
    task = active_task(root)
    if task and receipt_matches_tree(root, current_receipt(root)):
        return None
    delivered = delivered_receipt(root, task) if task else None
    reminder = _task_reminder(root, task, delivered) if task else None
    if reminder or baseline is None:
        return reminder
    # Changes made through the shell, or before a late `task begin`, still need a receipt.
    changed = changed_since(root, baseline)
    if not changed:
        return None
    paths = ", ".join(changed[:8])
    # The log cannot tell this session's writes from another session's in the same repository.
    if task and not delivered:
        return (
            f"Arc task {task['slug']} is unverified in {root}: files changed since this session "
            f"first touched it. Run `{_cli(root, 'task verify')}` before claiming delivery or "
            f"committing. Changed paths: {paths}."
        )
    return (
        f"{root} has uncommitted changes since this session first touched it, without an Arc "
        f"task covering them. If they are this session's work, run "
        f"`{_cli(root, f'task begin --skill {SKILLS}')}`, then `{_cli(root, 'task verify')}`, "
        f"before claiming delivery or committing. Changed paths: {paths}."
    )


def _stop_roots(
    cwd: Path, payload: dict[str, Any], baselines: dict[Path, dict[str, str]]
) -> list[Path]:
    """The working repository (or a workspace's task repositories), Cursor roots, touched ones."""
    roots = [cwd] if git_root(cwd) is not None else list(child_tasks(cwd))
    workspace = payload.get("workspace_roots")
    if isinstance(workspace, list):
        roots += [
            root
            for value in workspace
            if isinstance(value, str)
            for root in [git_root(Path(value))]
            if root
        ]
    roots += list(baselines)
    return list(dict.fromkeys(roots))


def handle_stop(
    cwd: Path, policy: dict[str, Any], payload: dict[str, Any]
) -> dict[str, Any] | None:
    reason = stop_reason(payload)
    if reason and reason != "end_turn":
        return None
    if payload.get("status") in {"aborted", "error"}:
        return None
    session = session_id(payload)
    baselines = session_baselines(session) if session else {}
    reminders: list[str] = []
    modes: list[str] = []
    markers: list[str] = []
    for root in _stop_roots(cwd, payload, baselines):
        root_policy = policy if root == cwd else _policy_for(root)
        reminder = _stop_reminder(root, root_policy, baselines.get(root))
        if reminder:
            stop = root_policy.get("stop") or {}
            reminders.append(reminder)
            modes.append(stop.get("mode") or "block_on_claim")
            markers += list(stop.get("completion_markers") or [])
    if not reminders:
        return None
    mode = max(modes, key=lambda value: _STOP_STRICTNESS.get(value, 1))
    return _stop_decision(
        mode, list(dict.fromkeys(markers)), payload, " ".join(reminders)
    )


def _stop_decision(
    mode: str, markers: list[str], payload: dict[str, Any], reminder: str
) -> dict[str, Any]:
    if (
        payload.get("stop_hook_active")
        or payload.get("stopHookActive")
        or payload.get("loop_count", 0)
    ):
        return _stop_remind(reminder)
    if mode == "remind":
        return _stop_remind(reminder)
    if mode == "block":
        return _stop_block(reminder)
    if payload.get("status") == "completed" and not last_assistant_message(payload):
        return _stop_block(reminder)
    try:
        if looks_like_completion(last_assistant_message(payload), markers):
            return _stop_block(reminder)
    except (
        OSError,
        RuntimeError,
        ValueError,
        KeyError,
        TypeError,
        subprocess.SubprocessError,
    ) as exc:
        return _stop_remind(f"{reminder} Completion judgment unavailable: {exc}")
    return _stop_remind(reminder)


def handle_event(
    payload: dict[str, Any], *, cwd: Path | None = None
) -> dict[str, Any] | None:
    directory = (cwd or payload_cwd(payload) or Path.cwd()).resolve()
    root = repo_root(directory)
    try:
        languages = detect_languages(root)
    except OSError:
        languages = []
    policy = load_for_repo(root, languages=languages)
    name = event_name(payload)
    if name == "SessionStart":
        prune()
        touch(session_id(payload), [root] if git_root(root) is not None else [])
        return handle_session_start(root, policy)
    if name == "PreToolUse":
        return handle_pre_tool_use(root, policy, payload, working_directory=directory)
    if name == "Stop":
        return handle_stop(root, policy, payload)
    return None


def run_hook(stdin: str, *, cwd: Path | None = None) -> str:
    stdin = stdin.strip()
    if not stdin:
        return ""
    try:
        payload = json.loads(stdin)
    except json.JSONDecodeError:
        return ""
    if not isinstance(payload, dict):
        return ""
    try:
        result = handle_event(payload, cwd=cwd)
    except Exception as exc:
        return fallback_output(
            payload, f"arc-policy hook error; verification remains pending: {exc}"
        )
    if not result:
        return ""
    return json.dumps(result, ensure_ascii=False)
