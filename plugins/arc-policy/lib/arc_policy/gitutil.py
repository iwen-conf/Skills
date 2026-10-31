from __future__ import annotations

import hashlib
import json
import os
import subprocess
from pathlib import Path

import yaml


def _env() -> dict[str, str]:
    env = dict(os.environ)
    env.setdefault("GIT_OPTIONAL_LOCKS", "0")
    return env


def git(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=root,
        env=_env(),
        capture_output=True,
        text=True,
        errors="surrogateescape",
        timeout=30,
        check=False,
    )


def head(root: Path) -> str:
    _output(root, "rev-parse", "--is-inside-work-tree")
    result = git(root, "rev-parse", "--verify", "--quiet", "HEAD")
    if result.returncode == 1:
        return "UNBORN"
    if result.returncode:
        raise RuntimeError(f"git rev-parse failed: {result.stderr.strip()}")
    return result.stdout.strip()


def _output(root: Path, *args: str) -> str:
    result = git(root, *args)
    if result.returncode:
        raise RuntimeError(f"git {args[0]} failed: {result.stderr.strip()}")
    return result.stdout


def _runtime_path(path: str) -> bool:
    return path.startswith((".arc/policy/", ".arc/artifacts/")) or path in {
        ".ai-code-index/tags.json",
        ".ai-code-index/daemon.pid",
        ".ai-code-index/daemon.log",
    }


def dirty_paths(root: Path) -> list[str]:
    status = _output(
        root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames"
    )
    return sorted(
        record[3:]
        for record in status.split("\0")
        if len(record) >= 4 and not _runtime_path(record[3:])
    )


def _file_digest(path: Path) -> str:
    if path.is_symlink():
        return "link:" + os.readlink(path)
    if not path.exists():
        return "missing"
    if path.is_dir():
        if (path / ".git").exists():
            return json.dumps(tree_fingerprint(path), sort_keys=True)
        raise RuntimeError(f"cannot fingerprint directory: {path}")
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b""):
            digest.update(chunk)
    return f"{path.stat().st_mode}:{digest.hexdigest()}"


def dirty_digests(root: Path) -> dict[str, str]:
    return {path: _file_digest(root / path) for path in dirty_paths(root)}


def dirty_stats(root: Path) -> dict[str, str]:
    """Cheap per-call change signature of dirty paths: mode, size and mtime, no content read."""
    stats = {}
    for path in dirty_paths(root):
        try:
            info = (root / path).lstat()
        except FileNotFoundError:
            stats[path] = "missing"
            continue
        stats[path] = f"{info.st_mode}:{info.st_size}:{info.st_mtime_ns}"
    return stats


def index_tree(root: Path) -> str | None:
    """Tree object the index would commit, or None (e.g. unmerged entries)."""
    result = git(root, "write-tree")
    return result.stdout.strip() if result.returncode == 0 else None


def commit_tree(root: Path, revision: str) -> str | None:
    result = git(root, "rev-parse", "--verify", "--quiet", f"{revision}^{{tree}}")
    return result.stdout.strip() if result.returncode == 0 else None


def tree_fingerprint(root: Path) -> dict[str, object]:
    commit = head(root)
    dirty = dirty_paths(root)
    index = [
        entry
        for entry in _output(root, "ls-files", "--stage", "-z").split("\0")
        if entry and not _runtime_path(entry.split("\t", 1)[-1])
    ]
    content = {path: _file_digest(root / path) for path in dirty}
    # Project policy is often gitignored alongside runtime state; it still changes verification.
    content[".arc/policy.yaml"] = _file_digest(root / ".arc/policy.yaml")
    # The graph ledger is gitignored but affects whether the business claim is
    # verified. Bind it to the same receipt as the source tree and policy.
    # A changed local run, graph or recorded evidence invalidates old receipts.
    graph_config = root / ".arc/verify/config.jsonc"
    policy_overlay = root / ".arc/policy.yaml"
    policy_text = policy_overlay.read_text(encoding="utf-8") if policy_overlay.is_file() else ""
    overlay = yaml.safe_load(policy_text) if policy_text else {}
    graph_policy = ((overlay or {}).get("verification") or {}).get("business_graph")
    graph_enabled = graph_config.is_file() or graph_policy is not None
    if graph_enabled:
        location = (
            graph_policy.get("db", ".arc/verify/data.db")
            if isinstance(graph_policy, dict) else ".arc/verify/data.db"
        )
        if not isinstance(location, str) or not location:
            raise RuntimeError("invalid verification.business_graph.db path")
        path = (root / location).resolve()
        if not path.is_relative_to(root):
            raise RuntimeError("verification.business_graph.db escapes the repository")
        rel_db = path.relative_to(root).as_posix()
        for rel in (
            ".arc/verify/config.jsonc",
            rel_db, rel_db + "-wal", rel_db + "-shm",
        ):
            content[rel] = _file_digest(root / rel)

    digest = hashlib.sha256(
        json.dumps([index, content], sort_keys=True).encode()
    ).hexdigest()
    return {"head": commit, "dirty": dirty, "digest": digest}
