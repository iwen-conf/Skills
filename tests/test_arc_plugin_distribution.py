from __future__ import annotations

import importlib.util
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

from scripts import sync_skills

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT / "plugins/arc-policy"
WORKFLOW_SKILLS = {
    "arc-audit",
    "arc-code",
    "arc-explore",
    "arc-plan",
    "arc-verify",
    "arc-vet",
}


def _builder():
    spec = importlib.util.spec_from_file_location(
        "build_arc_plugin", ROOT / "scripts/build_arc_plugin.py"
    )
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.fixture
def source(tmp_path: Path) -> Path:
    root = tmp_path / "source checkout"
    plugin = root / "plugins/arc-policy"
    plugin.mkdir(parents=True)
    shutil.copyfile(PLUGIN / "plugin.json", plugin / "plugin.json")
    bridge = plugin / "skills/arc-policy/SKILL.md"
    bridge.parent.mkdir(parents=True)
    bridge.write_text("hand-maintained bridge\n", encoding="utf-8")
    rules = root / "docs/rules.md"
    rules.parent.mkdir()
    rules.write_text("# Packaged rules\n", encoding="utf-8")
    for name in sorted(WORKFLOW_SKILLS):
        canonical = name.replace("-", ":", 1)
        skill = root / "Arc" / canonical
        skill.mkdir(parents=True)
        (skill / "SKILL.md").write_text(
            f"---\nname: {canonical}\nversion: 1.0.0\ndescription: Fixture\n---\n"
            "[rules](../../docs/rules.md)\n",
            encoding="utf-8",
        )
        script = skill / "scripts/probe.py"
        script.parent.mkdir()
        script.write_text("#!/usr/bin/env python3\nprint('fixture')\n")
        script.chmod(0o755)
    return root


def test_checked_in_distribution_matches_canonical_sources(capsys) -> None:
    assert _builder().main(["--check"]) == 0
    assert "6 bundled workflow skills" in capsys.readouterr().out


def test_build_uses_one_version_and_native_marketplace_locations(
    source: Path, monkeypatch
) -> None:
    builder = _builder()
    monkeypatch.setattr(builder, "ROOT", source)
    portable_path = source / builder.PLUGIN / "plugin.json"
    portable = json.loads(portable_path.read_text())
    portable["version"] = "1.2.3"
    portable_path.write_text(json.dumps(portable))
    legacy = source / "plugins/marketplace.json"
    legacy.write_text('{"name":"obsolete"}')

    assert builder.main([]) == 0

    for path in (
        ".claude-plugin/plugin.json",
        ".codex-plugin/plugin.json",
    ):
        manifest = json.loads((source / builder.PLUGIN / path).read_text())
        assert manifest["name"] == "arc-policy"
        assert manifest["version"] == "1.2.3"
        assert manifest["skills"] == "./skills/"
    assert (
        '__version__ = "1.2.3"'
        in (source / builder.PLUGIN / "lib/arc_policy/__init__.py").read_text()
    )
    claude = json.loads((source / ".claude-plugin/marketplace.json").read_text())
    grok = json.loads((source / ".grok-plugin/marketplace.json").read_text())
    codex = json.loads((source / ".agents/plugins/marketplace.json").read_text())
    assert claude["plugins"][0]["version"] == "1.2.3"
    assert grok["plugins"][0]["version"] == "1.2.3"
    assert claude["plugins"][0]["source"] == "./plugins/arc-policy"
    assert grok["plugins"][0]["source"] == {
        "type": "local",
        "path": "./plugins/arc-policy",
    }
    assert codex["plugins"][0]["source"] == {
        "source": "local",
        "path": "./plugins/arc-policy",
    }
    assert not legacy.exists()
    assert (source / builder.PLUGIN / "skills/arc-policy/SKILL.md").read_text() == (
        "hand-maintained bridge\n"
    )
    assert builder.main(["--check"]) == 0


def test_check_detects_source_and_version_drift_without_rewriting(
    source: Path, monkeypatch, capsys
) -> None:
    builder = _builder()
    monkeypatch.setattr(builder, "ROOT", source)
    assert builder.main([]) == 0
    before = builder._managed_files(source)
    (source / "docs/rules.md").write_text("# Updated canonical rule\n")
    portable_path = source / builder.PLUGIN / "plugin.json"
    portable = json.loads(portable_path.read_text())
    portable["version"] = "0.4.0"
    portable_path.write_text(json.dumps(portable))

    assert builder.main(["--check"]) == 1

    error = capsys.readouterr().err
    assert "references/_shared/docs/rules.md" in error
    assert ".claude-plugin/plugin.json" in error
    assert builder._managed_files(source) == before
    assert builder.main([]) == 0
    assert builder.main(["--check"]) == 0


@pytest.mark.parametrize(
    "change", ["extra-file", "hidden-file", "executable-bit", "symlink"]
)
def test_check_rejects_untracked_generated_drift(
    source: Path, monkeypatch, change: str
) -> None:
    builder = _builder()
    monkeypatch.setattr(builder, "ROOT", source)
    assert builder.main([]) == 0
    skill = source / builder.PLUGIN / "skills/arc-code"
    if change == "extra-file":
        (skill / "stale.md").write_text("outdated generated resource")
    elif change == "hidden-file":
        (skill / ".untracked-resource").write_text("unexpected hidden resource")
    elif change == "executable-bit":
        (skill / "scripts/probe.py").chmod(0o644)
    else:
        target = skill / "references/_shared/docs/rules.md"
        target.unlink()
        target.symlink_to(source / "docs/rules.md")

    assert builder.main(["--check"]) == 1


def test_missing_source_resource_does_not_replace_distribution(
    source: Path, monkeypatch
) -> None:
    builder = _builder()
    monkeypatch.setattr(builder, "ROOT", source)
    assert builder.main([]) == 0
    before = builder._managed_files(source)
    (source / "docs/rules.md").unlink()

    assert builder.main([]) == 1
    assert builder._managed_files(source) == before


def test_packaged_python_import_does_not_dirty_generated_check(tmp_path: Path) -> None:
    builder = _builder()
    expected = tmp_path / "expected distribution"
    cached = tmp_path / "executed plugin copy"
    builder.build_generated(ROOT, expected)
    shutil.copytree(expected, cached)
    scripts = cached / builder.PLUGIN / "skills/arc-verify/scripts/dbqual/scripts"
    result = subprocess.run(
        [
            sys.executable,
            "-c",
            "import sys; sys.path.insert(0, sys.argv[1]); import sqlverify_common",
            str(scripts),
        ],
        cwd=tmp_path,
        env={
            key: value
            for key, value in os.environ.items()
            if key != "PYTHONDONTWRITEBYTECODE"
        },
        text=True,
        capture_output=True,
        timeout=15,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert list((scripts / "__pycache__").glob("sqlverify_common.*.pyc"))
    assert builder.drift(cached, expected) == []


def test_detached_cache_copy_has_closed_skills_and_runs_with_provisioned_python(
    tmp_path: Path,
) -> None:
    # Only the plugin is copied. There is no adjacent Arc/, docs/, scripts/,
    # repository pyproject.toml, or source .venv to satisfy missing references.
    cached = tmp_path / "host plugin cache" / "arc-policy 0.3.0"
    shutil.copytree(
        PLUGIN,
        cached,
        ignore=shutil.ignore_patterns(".venv", "__pycache__", "*.pyc"),
    )
    project = tmp_path / "unrelated project"
    project.mkdir()
    assert not cached.resolve().is_relative_to(ROOT)
    assert {path.parent.name for path in cached.glob("skills/*/SKILL.md")} == (
        WORKFLOW_SKILLS | {"arc-policy"}
    )
    assert not any(path.is_symlink() for path in cached.rglob("*"))
    assert sync_skills.validate_installation(cached / "skills") == []
    for name in WORKFLOW_SKILLS:
        source_skill = ROOT / "Arc" / name.replace("-", ":", 1)
        for script in (source_skill / "scripts").rglob("*"):
            if script.is_file() and script.stat().st_mode & 0o111:
                packaged = cached / "skills" / name / script.relative_to(source_skill)
                assert packaged.stat().st_mode & 0o111 == script.stat().st_mode & 0o111
    portable = json.loads((cached / "plugin.json").read_text())
    for relative in (".claude-plugin/plugin.json", ".codex-plugin/plugin.json"):
        manifest = json.loads((cached / relative).read_text())
        assert manifest["version"] == portable["version"]
        assert (cached / manifest["skills"]).is_dir()
        assert (cached / manifest["hooks"]).is_file()
    assert (cached / portable["extensions"]["com.openai"]["hooks"]).is_file()

    # This test supplies the installed Python dependencies. It does not claim
    # arc-idx, arc-verify, or a Python environment are bundled in the plugin.
    clean_env = {
        key: value
        for key, value in os.environ.items()
        if key
        not in {"PYTHONPATH", "GROK_PLUGIN_ROOT", "CLAUDE_PLUGIN_ROOT", "PLUGIN_ROOT"}
    }
    result = subprocess.run(
        [sys.executable, "-I", str(cached / "bin/arc-policy"), "detect"],
        cwd=project,
        env=clean_env,
        text=True,
        capture_output=True,
        timeout=15,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout)["plugin_root"] == str(cached)
    assert json.loads(result.stdout)["repo_root"] == str(project)
