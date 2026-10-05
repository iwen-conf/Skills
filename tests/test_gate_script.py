"""Behavior tests for Arc/arc:code/scripts/gate.sh against throwaway git repositories."""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
GATE = ROOT / "Arc" / "arc:code" / "scripts" / "gate.sh"
BASH = "/bin/bash" if Path("/bin/bash").exists() else (shutil.which("bash") or "bash")

# Marker and suppression literals are assembled at runtime so the gate does not flag this file
# when the file itself is part of a change set.
MARK = "TO" + "DO"
NOQA = "# no" + "qa"
TS_IGNORE = "@ts" + "-ignore"
RAISE_NIE = "raise " + "NotImplementedError"

ENV = {
    key: value
    for key, value in os.environ.items()
    if key not in {"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY"}
}
ENV.update(
    {
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_AUTHOR_NAME": "Gate Test",
        "GIT_AUTHOR_EMAIL": "gate@example.invalid",
        "GIT_COMMITTER_NAME": "Gate Test",
        "GIT_COMMITTER_EMAIL": "gate@example.invalid",
    }
)


def git(repo: Path, *args: str) -> str:
    result = subprocess.run(["git", *args], cwd=repo, env=ENV, check=True, capture_output=True, text=True)
    return result.stdout


def make_repo(tmp_path: Path, name: str = "repo") -> Path:
    repo = tmp_path / name
    repo.mkdir(parents=True)
    git(repo, "-c", "init.defaultBranch=main", "init", "-q")
    (repo / "README.txt").write_text("hello\n", encoding="utf-8")
    git(repo, "add", "README.txt")
    git(repo, "commit", "-q", "-m", "init")
    return repo


def commit_all(repo: Path) -> None:
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "-m", "fixture")


def gate(
    repo: Path, *args: str, cwd: Path | None = None, env: dict[str, str] | None = None
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [BASH, str(GATE), *args],
        cwd=cwd or repo,
        env=env or ENV,
        capture_output=True,
        text=True,
        timeout=120,
    )


def checks(output: str) -> dict[str, str]:
    result = {}
    for line in output.splitlines():
        if line and not line.startswith((" ", "#")) and ": " in line:
            name, status = line.split(": ", 1)
            result[name] = status
    return result


def sha256(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def tree_state(repo: Path) -> tuple[object, ...]:
    status = [
        entry
        for entry in git(repo, "status", "--porcelain=v1", "-z", "--untracked-files=all").split("\0")
        if entry and not entry[3:].startswith(".arc/")
    ]
    hashes = {
        path.relative_to(repo).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sorted(repo.rglob("*"))
        if path.is_file() and path.relative_to(repo).parts[0] not in {".git", ".arc"}
    }
    return (
        status,
        hashes,
        git(repo, "ls-files", "-s"),
        git(repo, "rev-parse", "HEAD"),
        git(repo, "stash", "list"),
    )


def report_path(repo: Path, output: str) -> Path:
    line = next(item for item in output.splitlines() if item.startswith("# report: "))
    return repo / line.removeprefix("# report: ")


def test_gate_is_executable_and_parses() -> None:
    assert os.access(GATE, os.X_OK)
    subprocess.run([BASH, "-n", str(GATE)], check=True)
    result = subprocess.run([str(GATE), "--help"], env=ENV, capture_output=True, text=True, timeout=30)
    assert result.returncode == 0
    assert "gate.sh check" in result.stdout


def test_start_writes_baseline_and_refuses_overwrite(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "README.txt").write_text("hello\nuser edit\n", encoding="utf-8")
    (repo / "notes.txt").write_text("draft\n", encoding="utf-8")

    started = gate(repo, "start", "--task", "t1")
    assert started.returncode == 0, started.stderr
    baseline_file = repo / ".arc/artifacts/t1/baseline.json"
    baseline = json.loads(baseline_file.read_text(encoding="utf-8"))
    assert baseline["head"] == git(repo, "rev-parse", "HEAD").strip()
    assert {entry["path"]: entry["xy"] for entry in baseline["status"]} == {"README.txt": " M", "notes.txt": "??"}
    assert baseline["files"]["notes.txt"]["hash"] == sha256(b"draft\n")
    assert baseline["files"]["README.txt"]["hash"] == sha256(b"hello\nuser edit\n")

    recorded = baseline_file.read_bytes()
    (repo / "notes.txt").write_text("changed\n", encoding="utf-8")
    refused = gate(repo, "start", "--task", "t1")
    assert refused.returncode == 1
    assert "already exists" in refused.stderr
    assert baseline_file.read_bytes() == recorded

    forced = gate(repo, "start", "--task", "t1", "--force")
    assert forced.returncode == 0, forced.stderr
    assert json.loads(baseline_file.read_text(encoding="utf-8"))["files"]["notes.txt"]["hash"] == sha256(b"changed\n")


def test_default_slug_comes_from_sanitized_branch(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    git(repo, "checkout", "-q", "-b", "feature/login")
    assert gate(repo, "start").returncode == 0
    assert (repo / ".arc/artifacts/feature-login/baseline.json").is_file()


def test_repo_without_commits_uses_none_head(tmp_path: Path) -> None:
    repo = tmp_path / "fresh"
    repo.mkdir()
    git(repo, "-c", "init.defaultBranch=main", "init", "-q")
    (repo / "existing.txt").write_text(f"{MARK} from the user\n", encoding="utf-8")

    assert gate(repo, "start").returncode == 0
    assert json.loads((repo / ".arc/artifacts/main/baseline.json").read_text(encoding="utf-8"))["head"] == "none"

    (repo / "new.txt").write_text(f"{MARK} from the agent\n", encoding="utf-8")
    result = gate(repo, "check")
    assert result.returncode == 1
    assert checks(result.stdout)["placeholders"] == "FAIL(1 hit)"
    assert f"  new.txt:1: {MARK}" in result.stdout.splitlines()
    assert "existing.txt" not in result.stdout


def test_preexisting_dirty_markers_are_excluded_but_agent_markers_fail(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "README.txt").write_text(f"hello\nuser {MARK} keep\n", encoding="utf-8")
    (repo / "notes.txt").write_text(f"{MARK} user note\n", encoding="utf-8")
    assert gate(repo, "start").returncode == 0

    untouched = gate(repo, "check")
    assert untouched.returncode == 0, untouched.stdout
    assert "changed=0" in untouched.stdout
    assert checks(untouched.stdout)["placeholders"] == "PASS"

    (repo / "README.txt").write_text(f"agent line\nhello\nuser {MARK} keep\nagent {MARK} too\n", encoding="utf-8")
    (repo / "src").mkdir()
    (repo / "src" / "new.txt").write_text(f"ok\nagent {MARK} left\n{RAISE_NIE}\n", encoding="utf-8")
    result = gate(repo, "check")
    lines = result.stdout.splitlines()

    assert result.returncode == 1
    assert "changed=2" in result.stdout
    assert checks(result.stdout)["placeholders"] == "FAIL(3 hits)"
    assert f"  README.txt:4: {MARK}" in lines
    assert f"  src/new.txt:2: {MARK}" in lines
    assert f"  src/new.txt:3: {RAISE_NIE}" in lines
    assert not any(line.startswith("  README.txt:3:") for line in lines)
    assert "notes.txt" not in result.stdout


def test_suppressions_only_count_added_lines(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "config.txt").write_text(f"x = 1  {NOQA}\n", encoding="utf-8")
    commit_all(repo)
    assert gate(repo, "start").returncode == 0

    (repo / "config.txt").write_text(f"x = 1  {NOQA}\ny = 2\n", encoding="utf-8")
    edited = gate(repo, "check")
    assert checks(edited.stdout)["suppressions"] == "PASS"

    (repo / "config.txt").write_text(f"x = 1  {NOQA}\ny = 2\nz = 3  {NOQA}\n", encoding="utf-8")
    (repo / "app.ts").write_text(f"const v: any = 1;\n// {TS_IGNORE}\n", encoding="utf-8")
    (repo / "plain.txt").write_text("const v: any = 1;\n", encoding="utf-8")
    result = gate(repo, "check")
    lines = result.stdout.splitlines()

    assert result.returncode == 1
    assert checks(result.stdout)["suppressions"] == "FAIL(3 hits)"
    assert f"  config.txt:3: {NOQA}" in lines
    assert "  app.ts:1: : any" in lines
    assert f"  app.ts:2: {TS_IGNORE}" in lines
    assert "plain.txt:" not in result.stdout


def test_scope_file_fails_outside_and_passes_inside(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    assert gate(repo, "start").returncode == 0
    (repo / "src").mkdir()
    (repo / "src" / "a.txt").write_text("a\n", encoding="utf-8")
    (repo / "docs").mkdir()
    (repo / "docs" / "b.txt").write_text("b\n", encoding="utf-8")
    scope = tmp_path / "scope.txt"
    scope.write_text("# allowed paths\nsrc/\n\n  docs/*.txt  \n", encoding="utf-8")

    inside = gate(repo, "check", "--scope", str(scope))
    assert inside.returncode == 0, inside.stdout
    assert checks(inside.stdout)["scope"] == "PASS"

    (repo / "other.txt").write_text("x\n", encoding="utf-8")
    relative = os.path.relpath(scope, repo / "src")
    outside = gate(repo, "check", "--scope", relative, cwd=repo / "src")
    assert outside.returncode == 1
    assert checks(outside.stdout)["scope"] == "FAIL(outside scope: other.txt)"


def test_max_files_warns_without_failing(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    assert gate(repo, "start").returncode == 0
    for index in range(3):
        (repo / f"f{index}.txt").write_text("x\n", encoding="utf-8")

    result = gate(repo, "check", "--max-files", "2")
    lines = result.stdout.splitlines()

    assert result.returncode == 0, result.stdout
    assert checks(result.stdout)["scope"] == "WARN(3 files > 2)"
    assert all(f"  f{index}.txt" in lines for index in range(3))


def test_missing_baseline_prints_not_run_and_diffs_against_head(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "README.txt").write_text(f"hello\n{MARK} dirty\n", encoding="utf-8")

    result = gate(repo, "check")
    lines = result.stdout.splitlines()

    assert "baseline: NOT_RUN(no baseline; using HEAD)" in lines
    assert f"  README.txt:2: {MARK}" in lines
    assert result.returncode == 1


def test_paths_with_spaces_and_colons_from_a_subdirectory(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, "repo with space:colon")
    assert gate(repo, "start").returncode == 0
    nested = repo / "my dir"
    nested.mkdir()
    (nested / "a:b file.txt").write_text(f"fine\n{MARK} here\n", encoding="utf-8")

    result = gate(repo, "check", cwd=nested)

    assert result.returncode == 1
    assert f"  my dir/a:b file.txt:2: {MARK}" in result.stdout.splitlines()
    assert report_path(repo, result.stdout).is_file()


def test_check_never_modifies_the_working_tree(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "staged.txt").write_text("staged\n", encoding="utf-8")
    git(repo, "add", "staged.txt")
    (repo / "README.txt").write_text("hello\ndirty\n", encoding="utf-8")
    (repo / "untracked.txt").write_text("untracked\n", encoding="utf-8")
    assert gate(repo, "start").returncode == 0
    (repo / "agent.txt").write_text(f"{MARK}\n", encoding="utf-8")
    (repo / "bad.go").write_text("package main\nfunc  main( ) {}\n", encoding="utf-8")
    (repo / "mod.py").write_text("x=1\n", encoding="utf-8")
    env = {**ENV, "PATH": f"{Path(sys.executable).parent}{os.pathsep}{ENV.get('PATH', '')}"}

    before = tree_state(repo)
    result = gate(repo, "check", "--strict", env=env)

    assert result.returncode == 1
    assert tree_state(repo) == before


def test_exit_codes_and_usage_errors(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    assert gate(repo, "bogus").returncode == 2
    assert gate(repo).returncode == 2
    assert gate(repo, "check", "--max-files", "many").returncode == 2
    assert gate(repo, "start", "--strict").returncode == 2
    assert gate(repo, "check", "--scope", str(tmp_path / "missing.txt")).returncode == 2

    outside = tmp_path / "not-a-repo"
    outside.mkdir()
    no_repo = gate(outside, "check", env={**ENV, "GIT_CEILING_DIRECTORIES": str(tmp_path)})
    assert no_repo.returncode == 2
    assert "not inside a git work tree" in no_repo.stderr

    assert gate(repo, "start").returncode == 0
    assert gate(repo, "check").returncode == 0


def test_hit_listing_is_capped_and_report_keeps_everything(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    assert gate(repo, "start").returncode == 0
    (repo / "many.txt").write_text("".join(f"{MARK} {index}\n" for index in range(25)), encoding="utf-8")

    result = gate(repo, "check")
    lines = result.stdout.splitlines()

    assert checks(result.stdout)["placeholders"] == "FAIL(25 hits)"
    assert len([line for line in lines if line.startswith("  many.txt:")]) == 20
    assert "  +5 more" in lines
    report = report_path(repo, result.stdout)
    assert report.parent == repo / ".arc/artifacts/main/reports"
    assert report.read_text(encoding="utf-8").count("  many.txt:") == 25


def test_gate_does_not_flag_its_own_sources(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    assert gate(repo, "start").returncode == 0
    shutil.copy(GATE, repo / "gate.sh")
    shutil.copy(Path(__file__), repo / "test_gate_script.py.txt")

    result = gate(repo, "check")

    assert checks(result.stdout)["placeholders"] == "PASS", result.stdout
    assert checks(result.stdout)["suppressions"] == "PASS", result.stdout


@pytest.mark.skipif(shutil.which("gofmt") is None, reason="gofmt not installed")
def test_gofmt_reports_unformatted_go_without_rewriting(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    assert gate(repo, "start").returncode == 0
    source = "package main\nfunc  main( ) {}\n"
    (repo / "main.go").write_text(source, encoding="utf-8")

    result = gate(repo, "check")

    assert checks(result.stdout)["format(go)"] == "FAIL(1 file(s) not gofmt-formatted)"
    assert "  main.go" in result.stdout.splitlines()
    assert (repo / "main.go").read_text(encoding="utf-8") == source


@pytest.mark.skipif(shutil.which("go") is None, reason="go not installed")
def test_go_module_vets_and_tests_affected_packages(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "go.mod").write_text("module example.com/gate\n\ngo 1.21\n", encoding="utf-8")
    (repo / "calc").mkdir()
    (repo / "calc" / "calc_test.go").write_text(
        'package calc\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n'
        '\tif Add(1, 2) != 3 {\n\t\tt.Fatal("Add(1, 2) != 3")\n\t}\n}\n',
        encoding="utf-8",
    )
    (repo / "calc" / "calc.go").write_text("package calc\n\nfunc Add(a, b int) int { return a + b }\n", encoding="utf-8")
    commit_all(repo)
    assert gate(repo, "start").returncode == 0
    (repo / "calc" / "calc.go").write_text("package calc\n\nfunc Add(a, b int) int { return a - b }\n", encoding="utf-8")

    result = gate(repo, "check", env={**ENV, "GOTOOLCHAIN": "local", "GOFLAGS": ""})
    status = checks(result.stdout)

    assert status["format(go)"] == "PASS"
    assert status["lint(go)"] == "PASS"
    assert status["tests(go)"] == "FAIL(go test exit 1)"
    assert result.returncode == 1


def test_python_runs_tests_named_after_changed_module(tmp_path: Path) -> None:
    bindir = str(Path(sys.executable).parent)
    if shutil.which("pytest", path=bindir) is None:
        pytest.skip("pytest not installed next to the running interpreter")
    env = {**ENV, "PATH": f"{bindir}{os.pathsep}{ENV.get('PATH', '')}"}
    repo = make_repo(tmp_path)
    (repo / "pyproject.toml").write_text('[tool.pytest.ini_options]\npythonpath = ["."]\n', encoding="utf-8")
    (repo / "calc.py").write_text("def add(a, b):\n    return a + b\n", encoding="utf-8")
    (repo / "tests").mkdir()
    (repo / "tests" / "test_calc.py").write_text(
        "from calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n", encoding="utf-8"
    )
    commit_all(repo)
    assert gate(repo, "start", env=env).returncode == 0
    (repo / "calc.py").write_text("def add(a, b):\n    return a - b\n", encoding="utf-8")

    before = tree_state(repo)
    result = gate(repo, "check", env=env)

    assert checks(result.stdout)["tests(python)"] == "FAIL(pytest exit 1)", result.stdout
    assert tree_state(repo) == before
