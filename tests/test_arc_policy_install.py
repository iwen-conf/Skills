from __future__ import annotations

import json
import shutil
import subprocess
import sys

import pytest
from arc_policy import __version__
from arc_policy.cli import main
from arc_policy.githook import hook_command, install_git_hooks
from arc_policy.install import install_runtime, persistent_cli, runtime_home
from arc_policy.session import last_stop_notice, record_stop_notice
from arc_policy.store import active_task
from test_arc_policy import ENV, PLUGIN, git, make_repo


def test_installed_cli_survives_deleted_host_cache_and_provisioning_venv(
    tmp_path, monkeypatch
):
    cache = tmp_path / "host cache" / "version-0.3.0"
    shutil.copytree(
        PLUGIN, cache, ignore=shutil.ignore_patterns("__pycache__", ".venv")
    )
    bin_dir = tmp_path / "local bin"
    launcher = install_runtime(cache, bin_dir=bin_dir)
    command = hook_command("pre-commit", launcher)
    shutil.rmtree(cache)
    # -S proves startup is not borrowing PyYAML from any site-packages directory.
    current = runtime_home() / "current"
    result = subprocess.run(
        [sys.executable, "-S", str(current / "bin/arc-policy"), "version"],
        env={**ENV, "ARC_POLICY_HOME": str(runtime_home())},
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert f"arc-policy {__version__}" in result.stdout
    assert list((current / "vendor").rglob("LICENSE"))
    assert not list((current / "vendor").rglob("*.so"))
    launched = subprocess.run(
        [str(bin_dir / "arc-policy"), "version"],
        capture_output=True,
        text=True,
        check=False,
    )
    assert launched.returncode == 0, launched.stderr
    assert str(cache) not in command
    repo = make_repo(tmp_path)
    blocked = subprocess.run(
        ["sh", "-c", command],
        cwd=repo,
        env={**ENV, "GROK_AGENT": "1"},
        capture_output=True,
        text=True,
        check=False,
    )
    assert blocked.returncode == 0, blocked.stderr


def test_reinstall_switches_same_launcher_without_deleting_old_snapshot(tmp_path):
    source = tmp_path / "plugin"
    shutil.copytree(
        PLUGIN, source, ignore=shutil.ignore_patterns("__pycache__", ".venv")
    )
    first = install_runtime(source)
    previous = (runtime_home() / "current").resolve()
    (source / "release-note.txt").write_text("next snapshot")
    assert install_runtime(source) == first
    assert (runtime_home() / "current").resolve() != previous
    assert previous.is_dir()
    assert (runtime_home() / "current/release-note.txt").read_text() == "next snapshot"


def test_snapshot_identity_frames_paths_contents_and_modes(tmp_path):
    source = tmp_path / "plugin"
    shutil.copytree(
        PLUGIN, source, ignore=shutil.ignore_patterns("__pycache__", ".venv")
    )
    (source / "a").write_text("bc")
    install_runtime(source)
    first = (runtime_home() / "current").resolve()
    (source / "a").unlink()
    (source / "ab").write_text("c")
    install_runtime(source)
    second = (runtime_home() / "current").resolve()
    assert first != second
    assert (second / "ab").read_text() == "c"
    assert not (second / "a").exists()
    (source / "ab").chmod(0o755)
    install_runtime(source)
    third = (runtime_home() / "current").resolve()
    assert second != third
    assert (third / "ab").stat().st_mode & 0o111


def test_missing_persistent_runtime_does_not_block_human_git(tmp_path):
    command = hook_command("pre-commit", tmp_path / "missing cli")
    human = subprocess.run(
        ["sh", "-c", command],
        env=ENV,
        capture_output=True,
        text=True,
        check=False,
    )
    assert human.returncode == 0, human.stderr
    agent = subprocess.run(
        ["sh", "-c", command],
        env={**ENV, "CLAUDECODE": "1"},
        capture_output=True,
        text=True,
        check=False,
    )
    assert agent.returncode == 0, agent.stderr


def test_legacy_arc_git_hooks_are_removed_without_touching_user_hooks(tmp_path):
    from arc_policy.githook import FILE_HOOK_MARKER

    repo = make_repo(tmp_path)
    hooks = repo / ".git/hooks"
    owned = hooks / "pre-commit"
    previous = hooks / "pre-commit.arc-policy-original"
    previous_body = "#!/bin/sh\nprintf 'user hook\\n' > hook-ran\n"
    previous.write_text(previous_body)
    previous.chmod(0o755)
    owned.write_text(f"#!/bin/sh\n{FILE_HOOK_MARKER}\nexit 1\n")
    owned.chmod(0o755)
    other = hooks / "pre-push"
    other_body = "#!/bin/sh\nexit 0\n"
    other.write_text(other_body)
    other.chmod(0o755)
    for event in ("pre-commit", "pre-push"):
        git(repo, "config", "--local", f"hook.arc-policy-{event}.event", event)
        git(repo, "config", "--local", f"hook.arc-policy-{event}.command", "exit 1")

    assert install_git_hooks(repo) == "disabled"
    assert install_git_hooks(repo) == "disabled"
    assert owned.read_text() == previous_body
    assert not previous.exists()
    assert other.read_text() == other_body
    for event in ("pre-commit", "pre-push"):
        for key in ("event", "command"):
            result = subprocess.run(
                ["git", "config", "--local", "--get-all", f"hook.arc-policy-{event}.{key}"],
                cwd=repo, capture_output=True, check=False,
            )
            assert result.returncode != 0

    (repo / "example.txt").write_text("ready\n")
    git(repo, "add", "example.txt")
    result = subprocess.run(
        ["git", "commit", "-q", "-m", "without Arc verify"],
        cwd=repo, env={**ENV, "GROK_AGENT": "1"},
        capture_output=True, text=True, check=False,
    )
    assert result.returncode == 0, result.stderr
    assert (repo / "hook-ran").read_text() == "user hook\n"


def test_custom_git_hooks_path_does_not_block_task_begin(tmp_path):
    repo = make_repo(tmp_path)
    git(repo, "config", "core.hooksPath", ".husky")
    assert main(["--cwd", str(repo), "task", "begin", "--skill", "arc-code"]) == 0
    assert active_task(repo) is not None
    assert not (repo / ".husky").exists()


def test_retired_hook_command_does_not_require_a_cli(tmp_path):
    missing = tmp_path / "missing executable"
    for marker in ({}, {"GROK_AGENT": "1"}):
        result = subprocess.run(
            ["sh", "-c", hook_command("pre-commit", missing)],
            env={**ENV, **marker}, capture_output=True, text=True, check=False,
        )
        assert result.returncode == 0, result.stderr


def test_failed_hook_cleanup_aborts_task_begin_without_damaging_configuration(
    tmp_path, monkeypatch
):
    from arc_policy import githook

    repo = make_repo(tmp_path)
    git(repo, "config", "--local", "hook.arc-policy-pre-commit.event", "pre-commit")
    original_git = githook.git

    def fail_cleanup(root, *args):
        if args[:3] == ("config", "--local", "--unset-all"):
            return subprocess.CompletedProcess(args, 1, "", "configuration locked")
        return original_git(root, *args)

    monkeypatch.setattr(githook, "git", fail_cleanup)
    assert main(["--cwd", str(repo), "task", "begin", "--skill", "arc-code"]) == 2
    assert active_task(repo) is None
    assert (
        original_git(repo, "config", "--local", "--get", "hook.arc-policy-pre-commit.event").stdout.strip()
        == "pre-commit"
    )


@pytest.mark.parametrize(
    "first,second", [("tenant:17", "tenant/17"), ("x" * 128 + "a", "x" * 128 + "b")]
)
def test_distinct_session_ids_do_not_share_notice_state(
    tmp_path, monkeypatch, first, second
):
    monkeypatch.setenv("ARC_POLICY_STATE_DIR", str(tmp_path / "state"))
    record_stop_notice(first, "first receipt")
    record_stop_notice(second, "second receipt")
    assert last_stop_notice(first) == "first receipt"
    assert last_stop_notice(second) == "second receipt"


def test_doctor_reports_missing_skills_and_unknown_host_status(
    tmp_path, monkeypatch, capsys
):
    repo = make_repo(tmp_path)
    plugin = tmp_path / "incomplete plugin"
    shutil.copytree(PLUGIN / "policies", plugin / "policies")
    monkeypatch.setenv("ARC_POLICY_ROOT", str(plugin))
    assert main(["--cwd", str(repo), "doctor"]) == 1
    result = json.loads(capsys.readouterr().out)
    assert result["engine_ready"] is True
    assert result["skills_ready"] is False
    assert len(result["missing_skills"]) == 6
    assert result["workflow_ready"] is False
    assert result["host_hooks_ready"] is None
    assert "NOT_RUN" in result["host_hooks_status"]
