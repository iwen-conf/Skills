from __future__ import annotations

import importlib.util
import os
from pathlib import Path

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[1]


def _load_link_module():
    spec = importlib.util.spec_from_file_location(
        "link_hosts", ROOT / "scripts" / "link_hosts.py"
    )
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _tree(root: Path) -> dict[str, tuple[str, str]]:
    """Snapshot without following links: path -> (kind, link target or file content)."""
    entries: dict[str, tuple[str, str]] = {}
    for current, dirs, files in os.walk(root, followlinks=False):
        for name in dirs + files:
            path = Path(current) / name
            key = str(path.relative_to(root))
            if path.is_symlink():
                entries[key] = ("link", os.readlink(path))
            elif path.is_dir():
                entries[key] = ("dir", "")
            else:
                entries[key] = ("file", path.read_text(encoding="utf-8"))
    return entries


@pytest.fixture
def layout(tmp_path: Path, monkeypatch) -> dict[str, Path]:
    home, source, host = tmp_path / "home", tmp_path / "agents", tmp_path / "host"
    for name in ("alpha", "beta"):
        (source / name).mkdir(parents=True)
        (source / name / "SKILL.md").write_text(
            f"---\nname: {name}\n---\n", encoding="utf-8"
        )
    home.mkdir()
    host.mkdir()
    profiles = tmp_path / "profiles.yaml"
    profiles.write_text(
        yaml.safe_dump(
            {
                "source_root": str(source),
                "profiles": {},
                "hosts": {"h": {"root": str(host), "extra": ["alpha", "beta"]}},
            }
        ),
        encoding="utf-8",
    )
    monkeypatch.setenv("HOME", str(home))
    return {
        "home": home,
        "source": source,
        "host": host,
        "profiles": profiles,
        "base": tmp_path,
    }


def _drift(layout: dict[str, Path]) -> None:
    host, source = layout["host"], layout["source"]
    (host / "stray").mkdir()
    (host / "stray" / "note").write_text("user data", encoding="utf-8")
    (host / "ghost").symlink_to(layout["base"] / "absent", target_is_directory=True)
    (host / "alpha").mkdir()
    (host / "alpha" / "copy").write_text("stale copy", encoding="utf-8")
    (host / "beta").symlink_to(source / "alpha", target_is_directory=True)


def test_aligned_host_reports_ok_and_changes_nothing(layout, capsys) -> None:
    mod = _load_link_module()
    for name in ("alpha", "beta"):
        (layout["host"] / name).symlink_to(
            layout["source"] / name, target_is_directory=True
        )
    before = _tree(layout["base"])

    assert mod.main(["--check", "--profiles", str(layout["profiles"])]) == 0

    assert "— OK" in capsys.readouterr().out
    assert _tree(layout["base"]) == before


def test_check_lists_every_drift_kind_without_touching_the_host(layout, capsys) -> None:
    mod = _load_link_module()
    _drift(layout)
    before = _tree(layout["base"])

    assert mod.main(["--check", "--profiles", str(layout["profiles"])]) == 1

    out = capsys.readouterr().out
    for problem in (
        "not-a-link:alpha",
        "wrong-target:beta",
        "dangling:ghost",
        "unmanaged:stray",
    ):
        assert problem in out
    assert _tree(layout["base"]) == before


def test_apply_moves_drift_into_backup_and_links_the_declared_entries(
    layout, capsys
) -> None:
    mod = _load_link_module()
    _drift(layout)
    host, source = layout["host"], layout["source"]

    assert mod.main(["--apply", "--profiles", str(layout["profiles"])]) == 0

    [backup] = (layout["home"] / ".skills-backup").glob("*-link-hosts")
    moved = _tree(backup / "h")
    assert moved["stray/note"] == ("file", "user data")
    assert moved["alpha/copy"] == ("file", "stale copy")
    assert moved["ghost"] == ("link", str(layout["base"] / "absent"))
    assert moved["beta"] == ("link", str(source / "alpha"))
    assert _tree(host) == {
        "alpha": ("link", str(source / "alpha")),
        "beta": ("link", str(source / "beta")),
    }
    assert f"Moved entries saved in {backup}" in capsys.readouterr().out

    assert mod.main(["--apply", "--profiles", str(layout["profiles"])]) == 0
    assert len(list((layout["home"] / ".skills-backup").glob("*-link-hosts"))) == 1


def test_missing_source_is_reported_and_never_linked(layout, capsys) -> None:
    mod = _load_link_module()
    config = yaml.safe_load(layout["profiles"].read_text(encoding="utf-8"))
    config["hosts"]["h"]["extra"].append("gamma")
    layout["profiles"].write_text(yaml.safe_dump(config), encoding="utf-8")

    assert mod.main(["--apply", "--profiles", str(layout["profiles"])]) == 1

    assert "missing-source:gamma" in capsys.readouterr().out
    assert (
        not (layout["host"] / "gamma").exists()
        and not (layout["host"] / "gamma").is_symlink()
    )
    assert (layout["host"] / "alpha").is_symlink() and (
        layout["host"] / "beta"
    ).is_symlink()


def test_apply_failure_stops_and_names_the_backup_holding_moved_entries(
    layout, monkeypatch, capsys
) -> None:
    mod = _load_link_module()
    _drift(layout)
    real_rename = Path.rename
    calls = {"n": 0}

    def flaky(self, target):
        calls["n"] += 1
        if calls["n"] == 2:
            raise OSError("simulated failure")
        return real_rename(self, target)

    monkeypatch.setattr(Path, "rename", flaky)

    assert mod.main(["--apply", "--profiles", str(layout["profiles"])]) == 1

    out = capsys.readouterr().out
    assert "simulated failure" in out
    [backup] = (layout["home"] / ".skills-backup").glob("*-link-hosts")
    assert f"entries moved so far are in {backup}" in out
    moved = _tree(backup / "h")
    assert len([key for key in moved if "/" not in key]) == 1
