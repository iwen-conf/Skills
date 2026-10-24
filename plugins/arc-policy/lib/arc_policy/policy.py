from __future__ import annotations

from copy import deepcopy
from pathlib import Path
from typing import Any

import yaml

from arc_policy.paths import plugin_root, policies_dir, repo_root

Policy = dict[str, Any]


def _read_yaml(path: Path) -> Policy:
    text = path.read_text(encoding="utf-8")
    data = yaml.safe_load(text)
    if data is None:
        return {}
    if not isinstance(data, dict):
        raise ValueError(f"{path}: policy must be a mapping, got {type(data).__name__}")
    return data


def merge(base: Policy, overlay: Policy) -> Policy:
    out = deepcopy(base)
    for key, value in overlay.items():
        if key == "extends":
            continue
        if isinstance(value, dict) and isinstance(out.get(key), dict):
            out[key] = merge(out[key], value)
        else:
            out[key] = deepcopy(value)
    return out


def load_named(name: str, *, plugin: Path | None = None) -> Policy:
    path = policies_dir(plugin) / f"{name}.yaml"
    if not path.is_file():
        raise FileNotFoundError(f"policy not found: {path}")
    data = _read_yaml(path)
    parent = data.get("extends")
    if isinstance(parent, str) and parent and parent != name:
        return merge(load_named(parent, plugin=plugin), data)
    return data


def load_for_repo(
    cwd: Path | None = None,
    *,
    plugin: Path | None = None,
    languages: list[str] | None = None,
) -> Policy:
    plugin = plugin or plugin_root()
    policy = load_named("default", plugin=plugin)
    for language in languages or []:
        overlay = policies_dir(plugin) / f"{language}.yaml"
        if overlay.is_file():
            policy = merge(policy, load_named(language, plugin=plugin))
    project = repo_root(cwd) / ".arc" / "policy.yaml"
    if project.is_file():
        policy = merge(policy, _read_yaml(project))
    return policy
