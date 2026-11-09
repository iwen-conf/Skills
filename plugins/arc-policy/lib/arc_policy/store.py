from __future__ import annotations

import errno
import json
import os
import re
import shlex
import sys
import tempfile
import uuid
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from arc_policy.dbstore import DBState, load_db_state, save_db_state
from arc_policy.detect import normalize_skill_name
from arc_policy.gitutil import commit_tree, dirty_paths, git, head, tree_fingerprint
from arc_policy.paths import policy_state_dir

STATE_NAME = "state.json"
# PASS: `task verify` ran. AUTHORIZED: the user explicitly confirmed delivery without it.
RECEIPT_VERDICTS = frozenset({"PASS", "AUTHORIZED"})


@contextmanager
def task_lock(root: Path) -> Iterator[None]:
    path = policy_state_dir(root) / "task.lock"
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a+b") as stream:
        stream.seek(0)
        try:
            if sys.platform == "win32":
                import msvcrt

                msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl

                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as exc:
            if exc.errno in {errno.EACCES, errno.EAGAIN}:
                raise RuntimeError(
                    "Arc task is locked by another task operation or verification; retry after it finishes"
                ) from exc
            raise
        try:
            yield
        finally:
            if sys.platform == "win32":
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(stream, fcntl.LOCK_UN)


def _now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def _read(path: Path) -> dict[str, Any]:
    if not path.is_file():
        return {}
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise ValueError(f"{path}: expected object")
    return data


def _write(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary: str | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", dir=path.parent, delete=False
        ) as stream:
            temporary = stream.name
            json.dump(data, stream, indent=2, ensure_ascii=True)
            stream.write("\n")
        os.replace(temporary, path)
    finally:
        if temporary and os.path.exists(temporary):
            os.unlink(temporary)


def validate_slug(slug: object) -> None:
    if (
        not isinstance(slug, str)
        or re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}", slug) is None
    ):
        raise ValueError(
            "task slug must be 1–128 letters, digits, dots, underscores or hyphens, starting with a letter or digit"
        )


def load_state(root: Path) -> dict[str, Any]:
    current = load_db_state(root)
    return current if current is not None else _read(policy_state_dir(root) / STATE_NAME)


def save_state(root: Path, state: dict[str, Any]) -> None:
    if isinstance(state, DBState):
        save_db_state(root, state)
        return
    # A caller holding an old dict is never allowed to overwrite a migrated
    # DB record by falling back to a legacy JSON write.
    if load_db_state(root) is not None:
        raise RuntimeError("Policy state migrated to arc.db; reload before saving")
    _write(policy_state_dir(root) / STATE_NAME, state)


def active_task(root: Path) -> dict[str, Any] | None:
    active = load_state(root).get("active")
    if active is None:
        return None
    if (
        not isinstance(active, dict)
        or not isinstance(active.get("skill"), str)
        or not active["skill"]
    ):
        raise ValueError("invalid active Arc task; inspect .arc/policy/state.json")
    validate_slug(active.get("slug"))
    return active


def child_tasks(workspace: Path) -> dict[Path, dict[str, Any]]:
    """Active Arc tasks of the Git repositories directly under a non-Git workspace."""
    tasks = {}
    # Migrated Policy tasks no longer have a state.json file. Discover both
    # project layouts without recursing into unrelated repositories.
    roots = {
        file.parents[2]
        for pattern in (f"*/.arc/policy/{STATE_NAME}", "*/.arc/arc.json")
        for file in workspace.glob(pattern)
    }
    for root in sorted(roots):
        task = active_task(root) if (root / ".git").exists() else None
        if task:
            tasks[root] = task
    return tasks


def _new_task(
    root: Path,
    *,
    skill: str,
    slug: str,
    extra: dict[str, Any] | None = None,
) -> dict[str, Any]:
    fingerprint = tree_fingerprint(root)
    return {
        **(extra or {}),
        "id": uuid.uuid4().hex,
        "slug": slug,
        "skill": skill,
        "started_at": _now(),
        "begin_head": fingerprint["head"],
        "begin_dirty": fingerprint["dirty"],
        "begin_digest": fingerprint["digest"],
        "receipt": None,
    }


def _append_history(state: dict[str, Any], task: dict[str, Any]) -> None:
    history = state.setdefault("history", [])
    if not isinstance(history, list):
        raise RuntimeError(  # noqa: TRY004 - corrupt persisted state, not a caller type error
            "invalid Arc task history; inspect .arc/policy/state.json"
        )
    history.append(task)


def _handoff(
    state: dict[str, Any],
    previous: dict[str, Any],
    task: dict[str, Any],
    *,
    status: str,
    reason: str | None = None,
) -> None:
    archived = {**previous, "ended_at": _now()}
    # Older state may lack an instance ID. Give the archived record an identity
    # for this link without treating that bookkeeping as verification evidence.
    if not archived.get("id"):
        archived["id"] = uuid.uuid4().hex
    archived["handoff"] = {
        "to_task_id": task["id"],
        "to_skill": task["skill"],
        "status": status,
    }
    if reason is not None:
        archived["handoff"]["reason"] = reason
    _append_history(state, archived)
    task["previous_task_id"] = archived["id"]
    state["active"] = task


def begin_task(
    root: Path,
    *,
    skill: str,
    slug: str,
    extra: dict[str, Any] | None = None,
    force: bool = False,
) -> dict[str, Any]:
    validate_slug(slug)
    if not isinstance(skill, str) or not skill.strip():
        raise ValueError("task skill must not be empty")
    skill = normalize_skill_name(skill)
    with task_lock(root):
        existing = active_task(root)
        delivered = (
            existing is not None and delivered_receipt(root, existing) is not None
        )
        if existing and not force and not delivered:
            recovery = (
                "continue it, or use `arc-policy task end` before beginning new work; "
                "unfinished work uses `task end --incomplete` after recording its reason"
                if normalize_skill_name(existing["skill"]) == skill
                else f"use `arc-policy task switch --skill {shlex.quote(skill)}`; "
                "unfinished work requires `--incomplete --reason '<why>'`"
            )
            raise RuntimeError(
                f"active task already exists ({existing['slug']}, skill={existing['skill']}); "
                f"inspect `arc-policy task status`, then {recovery}"
            )
        task = _new_task(root, skill=skill, slug=slug, extra=extra)
        state = load_state(root)
        if existing:
            _handoff(
                state,
                existing,
                task,
                status="delivered" if delivered else "replaced",
                reason=None if delivered else "replaced by explicit task begin --force",
            )
        else:
            state["active"] = task
        save_state(root, state)
        return task


def switch_task(
    root: Path,
    *,
    skill: str,
    slug: str,
    incomplete: bool = False,
    reason: str | None = None,
) -> dict[str, Any]:
    """Archive the old phase and start the next in one locked state update."""
    validate_slug(slug)
    if not isinstance(skill, str) or not skill.strip():
        raise ValueError("task skill must not be empty")
    skill = normalize_skill_name(skill)
    if incomplete:
        if not isinstance(reason, str) or not reason.strip():
            raise ValueError("an incomplete phase switch requires a nonempty --reason")
        reason = reason.strip()
    elif reason is not None:
        raise ValueError("--reason requires --incomplete")
    with task_lock(root):
        previous = active_task(root)
        if previous is None:
            raise RuntimeError(
                "no active task to switch; run "
                f"`arc-policy task begin --skill {shlex.quote(skill)}` first"
            )
        if normalize_skill_name(previous["skill"]) == skill:
            raise RuntimeError(
                f"active task {previous['slug']} already uses {skill}; continue it. "
                "For new work in the same skill, end this task first or begin after delivery"
            )
        delivered = delivered_receipt(root, previous)
        receipt = previous.get("receipt")
        fresh = (
            not incomplete
            and isinstance(receipt, dict)
            and receipt_matches_tree(root, receipt)
        )
        if not incomplete and not fresh and delivered is None:
            raise RuntimeError(
                "the current phase has no fresh verification or delivery receipt; run "
                "`arc-policy task verify`, or switch with `--incomplete --reason '<why>'`"
            )
        task = _new_task(root, skill=skill, slug=slug)
        if (
            fresh
            and delivered is None
            and any(
                task[f"begin_{key}"] != receipt[key]
                for key in ("head", "dirty", "digest")
            )
        ):
            raise RuntimeError(
                "tree changed during phase switch; verify the current phase again"
            )
        status = "incomplete"
        if not incomplete:
            status = (
                "delivered"
                if delivered is not None
                else "authorized"
                if receipt["verdict"] == "AUTHORIZED"
                else "verified"
            )
        state = load_state(root)
        _handoff(state, previous, task, status=status, reason=reason)
        save_state(root, state)
        return task


def end_task(root: Path, *, require_verified: bool = False) -> dict[str, Any] | None:
    with task_lock(root):
        task = active_task(root)
        if (
            require_verified
            and task
            and not receipt_matches_tree(root, current_receipt(root))
            and not delivered_receipt(root, task)
        ):
            raise RuntimeError(
                "task is unverified; run `arc-policy task verify` or pass --incomplete"
            )
        state = load_state(root)
        ended = state.pop("active", None)
        if ended is not None:
            _append_history(state, ended)
            save_state(root, state)
        return ended if isinstance(ended, dict) else None


def save_receipt(root: Path, receipt: dict[str, Any]) -> Path:
    slug = str(receipt.get("slug") or "default")
    validate_slug(slug)
    active = active_task(root)
    if (
        not active
        or active.get("id") != receipt.get("task_id")
        or active["slug"] != slug
    ):
        raise RuntimeError("active task changed during verification; receipt not saved")
    path = policy_state_dir(root) / "receipts" / f"{slug}.json"
    state = load_state(root)
    # Once migrated, receipts are inserted transactionally by save_state into
    # arc_policy_receipts; never create/overwrite legacy receipt JSON files.
    if not isinstance(state, DBState):
        _write(path, receipt)
    active = state.get("active")
    if (
        not isinstance(active, dict)
        or active.get("slug") != slug
        or active.get("id") != receipt.get("task_id")
    ):
        raise RuntimeError("active task changed during verification; receipt not saved")
    active["receipt"] = receipt
    save_state(root, state)
    return (root / ".arc" / "arc.db") if isinstance(state, DBState) else path


def invalidate_receipt(root: Path) -> dict[str, Any]:
    task = active_task(root)
    if not task:
        raise RuntimeError(
            "no active task; run `arc-policy task begin --skill …` first"
        )
    task.setdefault("id", uuid.uuid4().hex)
    task["receipt"] = None
    state = load_state(root)
    state["active"] = task
    save_state(root, state)
    return task


def current_receipt(root: Path) -> dict[str, Any] | None:
    task = active_task(root)
    if not task:
        return None
    receipt = task.get("receipt")
    return receipt if isinstance(receipt, dict) else None


def _receipt_graph_valid(
    root: Path, receipt: dict[str, Any], task: dict[str, Any]
) -> bool:
    if (
        receipt.get("task_id") != task.get("id")
        or receipt.get("slug") != task.get("slug")
        or not isinstance(receipt.get("skill"), str)
        or normalize_skill_name(receipt["skill"]) != normalize_skill_name(task["skill"])
    ):
        return False
    if receipt.get("verdict") == "AUTHORIZED":
        return True
    checks = receipt.get("checks", [])
    if not isinstance(checks, list):
        return False
    graph_checks = [
        check
        for check in checks
        if isinstance(check, dict) and check.get("id") == "business_graph"
    ]
    if graph_checks:
        return len(graph_checks) == 1 and graph_checks[0].get("verdict") == "PASS"

    from arc_policy.policy import load_for_repo
    from arc_policy.verify import requires_business_graph

    try:
        policy = load_for_repo(root)
        configured = (root / ".arc/verify/config.jsonc").is_file() or (
            policy.get("verification") or {}
        ).get("business_graph") is not None
        return not (configured and requires_business_graph(task["skill"], policy))
    except (OSError, RuntimeError, ValueError):
        return False


def receipt_matches_tree(root: Path, receipt: dict[str, Any] | None) -> bool:
    if (
        not receipt
        or receipt.get("verdict") not in RECEIPT_VERDICTS
        or not receipt.get("digest")
    ):
        return False
    task = active_task(root)
    if not task or not task.get("id") or not _receipt_graph_valid(root, receipt, task):
        return False
    fingerprint = tree_fingerprint(root)
    if not all(
        receipt.get(key) == fingerprint[key] for key in ("head", "dirty", "digest")
    ):
        return False
    # Graph evidence is local/ignored: verify its RAW references again when
    # checking a receipt, not just the database file fingerprint. This also
    # rejects removed runner evidence and policy/binary failures.
    graph_checks = [
        check
        for check in receipt.get("checks", [])
        if isinstance(check, dict) and check.get("id") == "business_graph"
    ]
    if graph_checks:
        from arc_policy.policy import load_for_repo
        from arc_policy.verify import check_business_graph

        try:
            check = check_business_graph(root, load_for_repo(root))
        except (OSError, RuntimeError, ValueError):
            return False
        if check is None or check.get("verdict") != "PASS":
            return False
        if len(graph_checks) != 1 or any(
            graph_checks[0].get(key) != check.get(key) or not check.get(key)
            for key in ("proof_scope", "context")
        ):
            return False
    return True


def delivered_receipt(root: Path, task: dict[str, Any]) -> dict[str, Any] | None:
    """Return the task's receipt once its verified work is committed and HEAD has moved on.

    A delivered task no longer covers new edits; they need their own `task begin`.
    """
    receipt = task.get("receipt")
    if (
        not isinstance(receipt, dict)
        or receipt.get("verdict") not in RECEIPT_VERDICTS
        or not _receipt_graph_valid(root, receipt, task)
    ):
        return None
    verified_head = receipt.get("head")
    if not isinstance(verified_head, str) or verified_head == head(root):
        return None
    # Unknown or unrelated verified commits (unborn HEAD, rebase, reset) cannot prove delivery.
    if git(root, "merge-base", "--is-ancestor", verified_head, "HEAD").returncode:
        return None
    # Verified task changes that are still uncommitted mean the task is still in progress.
    own = set(receipt.get("dirty") or []) - set(task.get("begin_dirty") or [])
    if own & set(dirty_paths(root)):
        return None
    return receipt


def receipt_covers_commits(root: Path, revisions: list[str]) -> bool:
    """Whether every revision commits exactly the index the active task's receipt bound.

    A commit made from a matching receipt is already verified or authorized, so pushing
    it needs no second verification; whoever pushes, the content is the receipt's.
    """
    task = active_task(root)
    if task is None:
        return False
    receipt = task.get("receipt")
    if (
        not revisions
        or not isinstance(receipt, dict)
        or receipt.get("verdict") not in RECEIPT_VERDICTS
        or not _receipt_graph_valid(root, receipt, task)
        or not isinstance(receipt.get("tree"), str)
        or not isinstance(receipt.get("head"), str)
    ):
        return False
    for revision in revisions:
        if commit_tree(root, revision) != receipt["tree"]:
            return False
        if (
            receipt["head"] != "UNBORN"
            and git(
                root, "merge-base", "--is-ancestor", receipt["head"], revision
            ).returncode
        ):
            return False
    return True
