from __future__ import annotations

import errno
import json
import os
import re
import sys
import tempfile
import uuid
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

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
    return _read(policy_state_dir(root) / STATE_NAME)


def save_state(root: Path, state: dict[str, Any]) -> None:
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
    for state in sorted(workspace.glob(f"*/.arc/policy/{STATE_NAME}")):
        root = state.parents[2]
        task = active_task(root) if (root / ".git").exists() else None
        if task:
            tasks[root] = task
    return tasks


def begin_task(
    root: Path,
    *,
    skill: str,
    slug: str,
    extra: dict[str, Any] | None = None,
    force: bool = True,
) -> dict[str, Any]:
    validate_slug(slug)
    if not skill:
        raise ValueError("task skill must not be empty")
    with task_lock(root):
        existing = active_task(root)
        delivered = existing is not None and delivered_receipt(root, existing) is not None
        if existing and not force and not delivered:
            raise RuntimeError(
                f"active task already exists ({existing['slug']}, skill={existing['skill']}); pass --force to replace"
            )
        fingerprint = tree_fingerprint(root)
        task = {
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
        state = load_state(root)
        if delivered:
            history = state.setdefault("history", [])
            if isinstance(history, list):
                history.append(state["active"])
        state["active"] = task
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
            history = state.setdefault("history", [])
            if isinstance(history, list):
                history.append(ended)
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
    _write(path, receipt)
    state = load_state(root)
    active = state.get("active")
    if (
        not isinstance(active, dict)
        or active.get("slug") != slug
        or active.get("id") != receipt.get("task_id")
    ):
        raise RuntimeError("active task changed during verification; receipt not saved")
    active["receipt"] = receipt
    save_state(root, state)
    return path


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


def receipt_matches_tree(root: Path, receipt: dict[str, Any] | None) -> bool:
    if (
        not receipt
        or receipt.get("verdict") not in RECEIPT_VERDICTS
        or not receipt.get("digest")
    ):
        return False
    task = active_task(root)
    if (
        not task
        or not task.get("id")
        or receipt.get("task_id") != task["id"]
        or receipt.get("slug") != task["slug"]
        or receipt.get("skill") != task["skill"]
    ):
        return False
    fingerprint = tree_fingerprint(root)
    if not all(
        receipt.get(key) == fingerprint[key] for key in ("head", "dirty", "digest")
    ):
        return False
    # Graph evidence is local/ignored: verify its RAW references again when
    # checking a receipt, not just the database file fingerprint. This also
    # rejects removed runner evidence and policy/binary failures.
    if any(
        check.get("id") == "business_graph" and check.get("verdict") == "PASS"
        for check in receipt.get("checks", [])
        if isinstance(check, dict)
    ):
        from arc_policy.policy import load_for_repo
        from arc_policy.verify import check_business_graph
        try:
            check = check_business_graph(root, load_for_repo(root))
        except (OSError, RuntimeError, ValueError):
            return False
        if check is None or check.get("verdict") != "PASS":
            return False
    return True


def delivered_receipt(root: Path, task: dict[str, Any]) -> dict[str, Any] | None:
    """Return the task's receipt once its verified work is committed and HEAD has moved on.

    A delivered task no longer covers new edits; they need their own `task begin`.
    """
    receipt = task.get("receipt")
    if not isinstance(receipt, dict) or receipt.get("verdict") not in RECEIPT_VERDICTS:
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
    receipt = task.get("receipt") if task else None
    if (
        not revisions
        or not isinstance(receipt, dict)
        or receipt.get("verdict") not in RECEIPT_VERDICTS
        or receipt.get("task_id") != task.get("id")
        or not isinstance(receipt.get("tree"), str)
        or not isinstance(receipt.get("head"), str)
    ):
        return False
    for revision in revisions:
        if commit_tree(root, revision) != receipt["tree"]:
            return False
        if receipt["head"] != "UNBORN" and git(
            root, "merge-base", "--is-ancestor", receipt["head"], revision
        ).returncode:
            return False
    return True
