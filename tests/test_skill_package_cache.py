from pathlib import Path

import yaml

from test_sync_skills import _load_sync_module, _source


def test_installed_skill_retains_scripts_without_local_tool_caches(
    tmp_path: Path, monkeypatch
):
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    _source(source)
    scripts = source / "Arc/arc:one/scripts/dbqual"
    caches = (".ruff_cache", ".pytest_cache", ".mypy_cache", "__pycache__")
    for name in caches:
        (scripts / name).mkdir(parents=True)
        (scripts / name / "local-cache").write_text("local data")
    script = scripts / "check.py"
    script.write_text("raise SystemExit(0)\n")
    script.chmod(0o755)
    monkeypatch.setattr(mod, "ROOT", source)

    assert mod.main(["--only", "arc:one", "--dest", str(destination)]) == 0
    installed = destination / "arc-one/scripts/dbqual"
    assert (installed / "check.py").read_bytes() == script.read_bytes()
    assert (installed / "check.py").stat().st_mode & 0o111 == 0o111
    assert all(not (installed / name).exists() for name in caches)


def test_published_version_uses_metadata_without_losing_source_or_dependencies(
    tmp_path: Path, monkeypatch
):
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    _source(source)
    skill = source / "Arc/arc:one/SKILL.md"
    original = skill.read_text().replace(
        "name: arc:one\n", "name: arc:one\nversion: 3.5.0\n"
    )
    skill.write_text(original)
    monkeypatch.setattr(mod, "ROOT", source)

    assert mod.main(["--only", "arc:one", "--dest", str(destination)]) == 0
    installed = (destination / "arc-one/SKILL.md").read_text()
    metadata = yaml.safe_load(installed.split("---", 2)[1])
    assert "version" not in metadata
    assert metadata["metadata"]["version"] == "3.5.0"
    assert metadata["metadata"]["requires"]["skills"] == ["arc-two"]
    assert skill.read_text() == original
    assert mod.validate_installation(destination) == []


def test_conflicting_source_versions_fail_before_replacing_installation(
    tmp_path: Path, monkeypatch, capsys
):
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    _source(source)
    skill = source / "Arc/arc:one/SKILL.md"
    skill.write_text(
        skill.read_text().replace(
            "metadata:\n", "version: 3.5.0\nmetadata:\n  version: 2.0.0\n"
        )
    )
    destination.mkdir()
    (destination / "existing").write_text("keep")
    monkeypatch.setattr(mod, "ROOT", source)

    assert mod.main(["--only", "arc:one", "--dest", str(destination)]) == 1
    assert "conflicting version" in capsys.readouterr().err
    assert [item.name for item in destination.iterdir()] == ["existing"]
    assert (destination / "existing").read_text() == "keep"
