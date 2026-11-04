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
    assert blocked.returncode != 0
    assert "no Arc task" in blocked.stderr


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
    assert agent.returncode == 1
    assert "CLI missing" in agent.stderr


def test_old_git_chains_existing_hooks_with_arguments_and_stdin(tmp_path, monkeypatch):
    monkeypatch.setattr("arc_policy.githook.config_hooks_supported", lambda: False)
    repo = make_repo(tmp_path)
    hooks = repo / ".git/hooks"
    previous = hooks / "pre-push"
    previous.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > old-args\ncat > old-input\n')
    previous.chmod(0o755)
    cli = tmp_path / "fake arc-policy"
    cli.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > new-args\ncat > new-input\n')
    cli.chmod(0o755)
    assert install_git_hooks(repo, cli) == "file"
    assert install_git_hooks(repo, cli) == "file"
    updates = "refs/heads/main aaaa refs/heads/main bbbb\n"
    result = subprocess.run(
        [str(previous), "origin", "remote with spaces"],
        cwd=repo,
        input=updates,
        env={**ENV, "GROK_AGENT": "1"},
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert (repo / "old-args").read_text().splitlines() == [
        "origin",
        "remote with spaces",
    ]
    assert (repo / "new-args").read_text().splitlines() == [
        "git-hook",
        "pre-push",
        "origin",
        "remote with spaces",
    ]
    assert (repo / "old-input").read_text() == updates
    assert (repo / "new-input").read_text() == updates
    assert (hooks / "pre-push.arc-policy-original").is_file()


def test_old_git_custom_hooks_path_fails_before_task_or_runtime_creation(
    tmp_path, monkeypatch
):
    monkeypatch.setattr("arc_policy.githook.config_hooks_supported", lambda: False)
    repo = make_repo(tmp_path)
    git(repo, "config", "core.hooksPath", ".husky")
    assert main(["--cwd", str(repo), "task", "begin", "--skill", "arc-code"]) == 2
    assert active_task(repo) is None
    assert not persistent_cli().exists()
    assert not (repo / ".husky").exists()


def test_git_upgrade_restores_original_file_and_uses_one_gate(tmp_path, monkeypatch):
    repo = make_repo(tmp_path)
    original = repo / ".git/hooks/pre-commit"
    original.write_text("#!/bin/sh\nexit 0\n")
    original.chmod(0o755)
    cli = tmp_path / "cli"
    cli.write_text("#!/bin/sh\nexit 0\n")
    cli.chmod(0o755)
    monkeypatch.setattr("arc_policy.githook.config_hooks_supported", lambda: False)
    install_git_hooks(repo, cli)
    monkeypatch.setattr("arc_policy.githook.config_hooks_supported", lambda: True)
    assert install_git_hooks(repo, cli) == "config"
    assert original.read_text() == "#!/bin/sh\nexit 0\n"
    assert not original.with_name("pre-commit.arc-policy-original").exists()
    assert not (repo / ".git/hooks/pre-push").exists()


def test_failed_git_upgrade_keeps_existing_gate(tmp_path, monkeypatch):
    from arc_policy import githook

    repo = make_repo(tmp_path)
    cli = tmp_path / "cli"
    cli.write_text("#!/bin/sh\nexit 1\n")
    cli.chmod(0o755)
    monkeypatch.setattr(githook, "config_hooks_supported", lambda: False)
    install_git_hooks(repo, cli)
    real_git = githook.git

    def fail_write(root, *args):
        if args[:3] == ("config", "--local", "--replace-all"):
            return subprocess.CompletedProcess(args, 1, "", "configuration locked")
        return real_git(root, *args)

    monkeypatch.setattr(githook, "git", fail_write)
    monkeypatch.setattr(githook, "config_hooks_supported", lambda: True)
    with pytest.raises(RuntimeError, match="configuration locked"):
        install_git_hooks(repo, cli)
    for event in ("pre-commit", "pre-push"):
        path = repo / ".git/hooks" / event
        assert path.is_file()
        check = subprocess.run(
            [str(path)],
            cwd=repo,
            input="",
            env={**ENV, "GROK_AGENT": "1"},
            capture_output=True,
            text=True,
            check=False,
        )
        assert check.returncode == 1


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
