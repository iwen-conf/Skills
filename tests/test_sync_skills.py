from __future__ import annotations

import importlib.util
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def _load_sync_module():
    path = ROOT / "scripts" / "sync_skills.py"
    spec = importlib.util.spec_from_file_location("sync_skills", path)
    assert spec and spec.loader
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def test_dash_transform_rewrites_name_and_refs() -> None:
    mod = _load_sync_module()
    src = """---
name: arc:build
version: 0.2.0
description: short
---

# arc:build

Use `arc:arch` and `arc:sdlc` before edits.
"""
    out = mod.transform_skill_md_for_dash(src, "arc:build")
    assert "name: arc-build" in out
    assert "name: arc:build" not in out
    assert "`arc-arch`" in out
    assert "`arc-sdlc`" in out
    assert "Reasonix-compatible" in out


def test_list_skill_dirs_finds_arc_namespace() -> None:
    mod = _load_sync_module()
    names = {p.name for p in mod.list_skill_dirs()}
    assert "arc:build" in names
    assert "arc:sdlc" in names
    assert "arc:arch" in names
