from __future__ import annotations

import os
from pathlib import Path


def plugin_root() -> Path:
    for key in (
        "GROK_PLUGIN_ROOT",
        "CLAUDE_PLUGIN_ROOT",
        "PLUGIN_ROOT",
    ):
        value = os.environ.get(key)
        if value:
            return Path(value)
    return Path(__file__).resolve().parents[2]


def policies_dir(root: Path | None = None) -> Path:
    return (root or plugin_root()) / "policies"


def git_root(path: Path) -> Path | None:
    """Nearest enclosing Git work tree; the path itself need not exist yet."""
    start = path.resolve()
    for candidate in (start, *start.parents):
        if (candidate / ".git").exists():
            return candidate
    return None


def repo_root(cwd: Path | None = None) -> Path:
    start = (cwd or Path.cwd()).resolve()
    return git_root(start) or start


def policy_state_dir(root: Path) -> Path:
    return root / ".arc" / "policy"
