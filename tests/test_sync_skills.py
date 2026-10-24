from __future__ import annotations

import importlib.util
from pathlib import Path
import subprocess
import sys

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[1]


def _load_sync_module():
    spec = importlib.util.spec_from_file_location("sync_skills", ROOT / "scripts" / "sync_skills.py")
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _source(root: Path) -> None:
    for name in ("arc:one", "arc:two"):
        skill = root / "Arc" / name
        skill.mkdir(parents=True)
        requires = "metadata:\n  requires:\n    skills: [arc:two]\n" if name == "arc:one" else ""
        link = "[other](../arc:two/SKILL.md)\n" if name == "arc:one" else ""
        (skill / "SKILL.md").write_text(
            f"---\nname: {name}\ndescription: Test skill\n{requires}---\n"
            + link + "[gate](../../docs/gate.md#scope)\n", encoding="utf-8",
        )
        (skill / "agents").mkdir()
        (skill / "agents" / "openai.yaml").write_text(
            'interface:\n  default_prompt: "Use $' + name + ' for this task."\n'
            'policy:\n  allow_implicit_invocation: false\n', encoding="utf-8",
        )
    (root / "docs").mkdir()
    (root / "docs" / "gate.md").write_text("# Scope\n[details](detail.md)\n", encoding="utf-8")
    (root / "docs" / "detail.md").write_text("# Details\nShared rule.\n", encoding="utf-8")


@pytest.mark.parametrize("naming, first, second", [("dash", "arc-one", "arc-two"), ("colon", "arc:one", "arc:two")])
def test_selected_installation_contains_transitive_resources_and_matching_metadata(tmp_path: Path, naming: str, first: str, second: str) -> None:
    mod = _load_sync_module()
    source, stage = tmp_path / "source", tmp_path / "stage"
    _source(source)
    included = mod.build_installation(source, stage, ["arc:one"], naming)

    assert included == ["arc:one", "arc:two"]
    assert mod.validate_installation(stage) == []
    assert (stage / first / "references/_shared/docs/detail.md").read_text() == "# Details\nShared rule.\n"
    assert f"../{second}/SKILL.md" in (stage / first / "SKILL.md").read_text()
    assert "#scope" in (stage / first / "SKILL.md").read_text()
    ui = yaml.safe_load((stage / first / "agents/openai.yaml").read_text())
    assert "$" + first in ui["interface"]["default_prompt"]
    assert ui["policy"]["allow_implicit_invocation"] is False


def test_missing_resource_fails_before_changing_destination(tmp_path: Path, monkeypatch) -> None:
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    _source(source)
    (source / "docs/detail.md").unlink()
    destination.mkdir()
    (destination / "unchanged").write_text("keep")
    monkeypatch.setattr(mod, "ROOT", source)

    assert mod.main(["--only", "arc:one", "--dest", str(destination)]) == 1
    assert [p.name for p in destination.iterdir()] == ["unchanged"]


def test_publish_archives_old_aliases_without_following_links(tmp_path: Path) -> None:
    mod = _load_sync_module()
    source, stage, destination = tmp_path / "source", tmp_path / "stage", tmp_path / "installed"
    _source(source)
    included = mod.build_installation(source, stage, ["arc:one"])
    destination.mkdir()
    (destination / "arc:one").symlink_to(source / "Arc/arc:one", target_is_directory=True)
    (destination / "arc-one").mkdir()
    (destination / "arc-one/local-note").write_text("preserve in backup")
    (destination / "unrelated").mkdir()

    backup = mod.publish_installation(stage, destination, included, "dash")

    assert mod.validate_installation(destination) == []
    assert (backup / "arc:one").is_symlink()
    assert (backup / "arc-one/local-note").read_text() == "preserve in backup"
    assert (source / "Arc/arc:one/SKILL.md").is_file()
    assert not (destination / "arc:one").exists()
    assert (destination / "unrelated").is_dir()


def test_cross_skill_resource_is_shared_without_installing_that_skill(tmp_path: Path) -> None:
    mod = _load_sync_module()
    source, stage = tmp_path / "source", tmp_path / "stage"
    one = source / "Arc" / "arc:one"
    two = source / "Arc" / "arc:two"
    one.mkdir(parents=True)
    two.mkdir(parents=True)
    (one / "SKILL.md").write_text(
        "---\nname: arc:one\ndescription: Test skill\n---\n[note](../arc:two/references/note.md)\n",
        encoding="utf-8",
    )
    (two / "SKILL.md").write_text(
        "---\nname: arc:two\ndescription: Test skill\n---\n[missing](missing.md)\n",
        encoding="utf-8",
    )
    (two / "references").mkdir()
    (two / "references" / "note.md").write_text("# Note\n", encoding="utf-8")

    included = mod.build_installation(source, stage, ["arc:one"])

    assert included == ["arc:one"]
    assert (stage / "arc-one" / "references/_shared/Arc/arc-two/references/note.md").is_file()
    assert not (stage / "arc-two").exists()


def test_unreferenced_broken_markdown_is_not_packaged(tmp_path: Path) -> None:
    mod = _load_sync_module()
    source, stage = tmp_path / "source", tmp_path / "stage"
    _source(source)
    leftover = source / "Arc" / "arc:one" / "references" / "leftover.md"
    leftover.parent.mkdir()
    leftover.write_text("[missing](../../missing.md)\n", encoding="utf-8")

    included = mod.build_installation(source, stage, ["arc:one"])

    assert "arc:one" in included
    assert not (stage / "arc-one" / "references" / "leftover.md").exists()
    assert mod.validate_installation(stage) == []


def test_declared_skill_dependency_is_installed_without_a_file_link(tmp_path: Path) -> None:
    mod = _load_sync_module()
    source, stage = tmp_path / "source", tmp_path / "stage"
    for name, extra in (
        ("arc:need", "metadata:\n  requires:\n    skills: [arc:dep]\n"),
        ("arc:dep", ""),
    ):
        skill = source / "Arc" / name
        skill.mkdir(parents=True)
        (skill / "SKILL.md").write_text(
            f"---\nname: {name}\ndescription: Test skill\n{extra}---\n# {name}\n",
            encoding="utf-8",
        )

    included = mod.build_installation(source, stage, ["arc:need"])

    assert included == ["arc:dep", "arc:need"]
    assert (stage / "arc-need" / "SKILL.md").is_file()
    assert (stage / "arc-dep" / "SKILL.md").is_file()
    assert mod.validate_installation(stage) == []


def test_missing_declared_skill_leaves_destination_unchanged(tmp_path: Path, monkeypatch) -> None:
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    skill = source / "Arc" / "arc:need"
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        "---\nname: arc:need\ndescription: Test skill\n"
        "metadata:\n  requires:\n    skills: [arc:missing]\n---\n# need\n",
        encoding="utf-8",
    )
    destination.mkdir()
    (destination / "unchanged").write_text("keep")
    monkeypatch.setattr(mod, "ROOT", source)

    assert mod.main(["--only", "arc:need", "--dest", str(destination)]) == 1
    assert [path.name for path in destination.iterdir()] == ["unchanged"]


def test_publish_failure_restores_previous_installation(tmp_path: Path, monkeypatch) -> None:
    mod = _load_sync_module()
    source, stage, destination = tmp_path / "source", tmp_path / "stage", tmp_path / "installed"
    _source(source)
    included = mod.build_installation(source, stage, ["arc:one"])
    destination.mkdir()
    (destination / "arc-one").mkdir()
    (destination / "arc-one" / "old").write_text("keep-me")
    real_replace = Path.replace
    calls = {"n": 0}

    def boom(self, target):
        calls["n"] += 1
        if calls["n"] == 3:
            raise OSError("simulated failure")
        return real_replace(self, target)

    monkeypatch.setattr(Path, "replace", boom)

    with pytest.raises(OSError, match="simulated failure"):
        mod.publish_installation(stage, destination, included, "dash")

    assert (destination / "arc-one" / "old").read_text() == "keep-me"
    assert not (destination / "arc-two").exists()


def test_real_arc_packages_are_portable(tmp_path: Path) -> None:
    mod = _load_sync_module()
    names = [mod.read_metadata(p / "SKILL.md")["name"] for p in mod.list_skill_dirs() if p.parent.name == "Arc"]
    included = mod.build_installation(ROOT, tmp_path / "stage", names)
    assert included == sorted(names)
    assert set(included) == {"arc:explore", "arc:plan", "arc:code", "arc:verify", "arc:audit", "arc:vet"}
    assert mod.validate_installation(tmp_path / "stage") == []
    verify = tmp_path / "stage" / "arc-verify"
    assert (verify / "scripts" / "dbqual" / "scripts" / "run_suite.py").is_file()
    assert (verify / "references" / "_shared" / "Arc" / "_shared" / "contract.md").is_file()
    assert not list((tmp_path / "stage").rglob("__pycache__"))


@pytest.mark.parametrize("naming", ["dash", "colon"])
def test_closure_package_runs_after_installation(tmp_path: Path, naming: str) -> None:
    mod = _load_sync_module()
    stage, project = tmp_path / "installed", tmp_path / "project"
    included = mod.build_installation(ROOT, stage, ["arc:plan"], naming)
    assert "arc:plan" in included
    tools = stage / mod.published_name("arc:plan", naming) / "scripts" / "closure"
    initialized = subprocess.run(
        [sys.executable, str(tools / "init_docs.py"), str(project)],
        capture_output=True, text=True, check=False,
    )
    assert initialized.returncode == 0, initialized.stdout + initialized.stderr
    model_dir = project / "docs" / "01-系统设计" / "业务闭环"
    assert (model_dir / "03-closure-spec.yaml").is_file()
    assert (model_dir / "01-requirements.md").is_file()
    assert (model_dir / "README.md").is_file()
    assert not (project / "docs" / "arc").exists()
    assert (model_dir / "tools/check_closure.py").read_bytes() == (tools / "check_closure.py").read_bytes()
    assert not (project / "CLAUDE.md").exists()
    assert not (project / "AGENTS.md").exists()
    spec = model_dir / "03-closure-spec.yaml"
    spec.write_bytes((tools / "assets/examples/example-closed.yaml").read_bytes())
    checker = model_dir / "tools/check_closure.py"
    for command in ("check", "gate"):
        checked = subprocess.run(
            [sys.executable, str(checker), command, str(spec)],
            capture_output=True, text=True, check=False,
        )
        assert checked.returncode == 0, checked.stdout + checked.stderr
    exercised = subprocess.run(
        [sys.executable, str(tools / "selftest.py")],
        capture_output=True, text=True, check=False,
    )
    assert exercised.returncode == 0, exercised.stdout + exercised.stderr


def test_lark_router_packages_meeting_entries_as_library(tmp_path: Path) -> None:
    mod = _load_sync_module()
    stage = tmp_path / "stage"
    assert mod.build_installation(ROOT, stage, ["lark"]) == ["lark"]
    library = stage / "lark" / "library"
    for entry in ("lark-meeting", "lark-minutes", "lark-note", "lark-vc", "lark-vc-agent", "lark-workflow-meeting-summary"):
        assert (library / entry / "ENTRY.md").is_file(), entry
    assert sorted(path.relative_to(stage).as_posix() for path in stage.rglob("SKILL.md")) == ["lark/SKILL.md"]
    assert mod.validate_installation(stage) == []


def test_discovery_rejects_independent_copies_and_arc_aliases(tmp_path: Path) -> None:
    mod = _load_sync_module()
    for root, name in (("shared", "arc-one"), ("host", "arc:one")):
        folder = tmp_path / root / name
        folder.mkdir(parents=True)
        (folder / "SKILL.md").write_text(f"---\nname: {name}\ndescription: Test\n---\n")
    errors = mod.validate_discovery([tmp_path / "shared", tmp_path / "host"])
    assert len(errors) == 1
    assert "duplicate discovery name arc-one" in errors[0]


def test_discovery_allows_shared_links_but_reports_broken_links(tmp_path: Path) -> None:
    mod = _load_sync_module()
    shared, host = tmp_path / "shared", tmp_path / "host"
    skill = shared / "sample"
    skill.mkdir(parents=True)
    host.mkdir()
    (skill / "SKILL.md").write_text("---\nname: sample\ndescription: Test\n---\n")
    (host / "sample").symlink_to(skill, target_is_directory=True)
    assert mod.validate_discovery([shared, host]) == []
    (host / "missing").symlink_to(tmp_path / "absent", target_is_directory=True)
    assert mod.validate_discovery([shared, host]) == [f"broken skill link: {host / 'missing'}"]


def test_check_rejects_empty_installation_and_cross_root_duplicates(tmp_path: Path, monkeypatch, capsys) -> None:
    mod = _load_sync_module()
    source, installed, other = tmp_path / "source", tmp_path / "installed", tmp_path / "other"
    _source(source)
    installed.mkdir()
    monkeypatch.setattr(mod, "ROOT", source)
    assert mod.main(["--check", "--dest", str(installed)]) == 1
    assert "missing expected installed skill" in capsys.readouterr().err
    mod.build_installation(source, installed, ["arc:one"])
    assert mod.main(["--check", "--dest", str(installed)]) == 0
    mod.build_installation(source, other, ["arc:one"], "colon")
    assert mod.main(["--check", "--dest", str(installed), "--also-root", str(other)]) == 1
    assert "duplicate discovery name arc-one" in capsys.readouterr().err


def test_all_runtime_families_have_a_complete_package(tmp_path: Path) -> None:
    mod = _load_sync_module()
    families = ["Arc", *mod.VERTICAL_SKILL_ROOTS]
    names = mod.selectable_names(ROOT, families)
    stage = tmp_path / 'stage'
    assert mod.build_installation(ROOT, stage, names) == sorted(names)
    # All runtime families are present, so no unavailable optional sibling is excused.
    assert mod.validate_installation(stage) == []
    assert not set(mod.library_owners(ROOT)) & {path.name for path in stage.iterdir()}


def test_perfetto_analysis_ships_with_its_sql_contract_inside_android(tmp_path: Path) -> None:
    mod = _load_sync_module()
    stage = tmp_path / 'stage'
    assert mod.build_installation(ROOT, stage, ['android']) == ['android']
    library = stage / 'android' / 'library'
    assert (library / 'perfetto-sql' / 'ENTRY.md').is_file()
    assert (library / 'perfetto-trace-analysis' / 'references' / 'sql.md').is_file()
    with pytest.raises(ValueError, match='bundled in android'):
        mod.build_installation(ROOT, tmp_path / 'alone', ['perfetto-trace-analysis'])


def test_curated_waza_packages_keep_resources_without_duplicate_workflows(tmp_path: Path) -> None:
    mod = _load_sync_module()
    stage = tmp_path / 'stage'
    selected = ['check', 'health', 'research-write']
    included = mod.build_installation(ROOT, stage, selected)
    assert set(selected) == set(included)
    assert not {'think', 'hunt', 'ui', 'read', 'write', 'learn'} & set(included)
    assert mod.validate_installation(stage) == []
    for name in selected:
        assert (stage / name / 'references/_shared/Waza/LICENSE').is_file()
    for name in ('check', 'health'):
        assert not (stage / name / 'scripts').exists()
    package = stage / 'research-write'
    assert (package / 'scripts/fetch.sh').is_file()
    assert (package / 'scripts/check_punctuation.py').is_file()
    assert (package / 'scripts/anysearch/anysearch_cli.py').is_file()
    assert (package / 'scripts/anysearch/LICENSE').is_file()
    for mode in ('fetch', 'search', 'research', 'edit'):
        assert (package / 'references' / f'{mode}.md').is_file()


def test_research_write_is_one_package_without_skill_dependencies(tmp_path: Path) -> None:
    mod = _load_sync_module()
    stage = tmp_path / 'stage'
    included = mod.build_installation(ROOT, stage, ['research-write'])
    assert included == ['research-write']
    assert mod.validate_installation(stage) == []


def _library_source(root: Path) -> None:
    router = root / "Lark" / "lark"
    router.mkdir(parents=True)
    (router / "SKILL.md").write_text(
        "---\nname: lark\ndescription: Router\nmetadata:\n  library: [lark-doc, lark-shared]\n---\n"
        "[doc](../lark-doc/SKILL.md)\n", encoding="utf-8",
    )
    for name in ("lark-doc", "lark-shared"):
        entry = root / "Lark" / name
        (entry / "references").mkdir(parents=True)
        link = "[auth](../lark-shared/SKILL.md) [ref](references/a.md) [gate](../../docs/gate.md)\n" if name == "lark-doc" else ""
        (entry / "SKILL.md").write_text(f"---\nname: {name}\ndescription: Entry\n---\n{link}", encoding="utf-8")
        (entry / "references" / "a.md").write_text("# A\n", encoding="utf-8")
    (root / "docs").mkdir()
    (root / "docs" / "gate.md").write_text("# Gate\n", encoding="utf-8")


def test_router_bundles_library_entries_as_undiscoverable_files(tmp_path: Path) -> None:
    mod = _load_sync_module()
    source, stage = tmp_path / "src", tmp_path / "stage"
    _library_source(source)
    assert mod.library_owners(source) == {"lark-doc": "lark", "lark-shared": "lark"}
    assert mod.selectable_names(source, ["Lark"]) == ["lark"]
    assert mod.build_installation(source, stage, ["lark"]) == ["lark"]
    entry = stage / "lark" / "library" / "lark-doc" / "ENTRY.md"
    text = entry.read_text(encoding="utf-8")
    assert "](../lark-shared/ENTRY.md)" in text
    assert (stage / "lark/library/lark-doc/references/a.md").is_file()
    assert (stage / "lark/references/_shared/docs/gate.md").is_file()
    assert "](library/lark-doc/ENTRY.md)" in (stage / "lark" / "SKILL.md").read_text(encoding="utf-8")
    assert sorted(path.relative_to(stage).as_posix() for path in stage.rglob("SKILL.md")) == ["lark/SKILL.md"]
    assert mod.validate_installation(stage) == []


def test_library_entries_cannot_be_installed_alone(tmp_path: Path) -> None:
    mod = _load_sync_module()
    source = tmp_path / "src"
    _library_source(source)
    with pytest.raises(ValueError, match="bundled in lark"):
        mod.build_installation(source, tmp_path / "stage", ["lark-doc"])


def test_nested_skill_files_fail_installation_validation(tmp_path: Path) -> None:
    mod = _load_sync_module()
    package = tmp_path / "pkg"
    (package / "inner").mkdir(parents=True)
    (package / "SKILL.md").write_text("---\nname: pkg\ndescription: d\n---\n", encoding="utf-8")
    (package / "inner" / "SKILL.md").write_text("---\nname: inner\ndescription: d\n---\n", encoding="utf-8")
    assert any("nested SKILL.md" in item for item in mod.validate_installation(tmp_path))


def _tree(root: Path) -> dict[str, str]:
    return {str(p.relative_to(root)): p.read_text(encoding="utf-8") if p.is_file() else "<dir>" for p in sorted(root.rglob("*"))}


def test_also_root_without_check_is_a_usage_error(tmp_path: Path) -> None:
    mod = _load_sync_module()
    with pytest.raises(SystemExit) as exited:
        mod.main(["--dest", str(tmp_path / "installed"), "--also-root", str(tmp_path / "other")])
    assert exited.value.code == 2
    assert not (tmp_path / "installed").exists()


@pytest.mark.parametrize("where", ["inside", "parent"])
def test_destination_overlapping_the_source_is_refused_without_moving_anything(tmp_path: Path, monkeypatch, capsys, where: str) -> None:
    mod = _load_sync_module()
    source = tmp_path / "source"
    _source(source)
    monkeypatch.setattr(mod, "ROOT", source)
    destination = source / "Arc" if where == "inside" else tmp_path
    before = _tree(tmp_path)

    assert mod.main(["--only", "arc:one", "--dest", str(destination)]) == 1

    assert "must not overlap the source repository" in capsys.readouterr().err
    assert _tree(tmp_path) == before


def test_empty_selection_is_refused(tmp_path: Path, monkeypatch, capsys) -> None:
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    _source(source)
    destination.mkdir()
    (destination / "unchanged").write_text("keep")
    monkeypatch.setattr(mod, "ROOT", source)

    assert mod.main(["--family", "Android", "--dest", str(destination)]) == 1

    assert "no skills selected" in capsys.readouterr().err
    assert _tree(destination) == {"unchanged": "keep"}


def test_dry_run_builds_and_validates_without_touching_the_destination(tmp_path: Path, monkeypatch, capsys) -> None:
    mod = _load_sync_module()
    source, destination = tmp_path / "source", tmp_path / "installed"
    _source(source)
    destination.mkdir()
    (destination / "arc-one").mkdir()
    (destination / "arc-one" / "old").write_text("keep-me")
    monkeypatch.setattr(mod, "ROOT", source)
    before = _tree(tmp_path)

    assert mod.main(["--only", "arc:one", "--dest", str(destination), "--dry-run"]) == 0

    out = capsys.readouterr().out
    assert "Verified 2 packages" in out and "Dry run: destination unchanged" in out
    assert _tree(tmp_path) == before


def test_failed_rollback_names_the_backup_that_still_holds_the_previous_entries(tmp_path: Path, monkeypatch) -> None:
    mod = _load_sync_module()
    source, stage, destination = tmp_path / "source", tmp_path / "stage", tmp_path / "installed"
    _source(source)
    included = mod.build_installation(source, stage, ["arc:one"])
    destination.mkdir()
    (destination / "arc-one").mkdir()
    (destination / "arc-one" / "old").write_text("keep-me")
    real_replace = Path.replace
    calls = {"n": 0}

    def boom(self, target):
        calls["n"] += 1
        if calls["n"] >= 3:  # third move fails the publish; every later move fails the rollback too
            raise OSError("simulated failure")
        return real_replace(self, target)

    monkeypatch.setattr(Path, "replace", boom)

    with pytest.raises(OSError) as raised:
        mod.publish_installation(stage, destination, included, "dash")

    message = str(raised.value)
    assert "rollback failed" in message
    backup = Path(message.rsplit("previous entries remain in ", 1)[1])
    assert (backup / "arc-one" / "old").read_text() == "keep-me"
