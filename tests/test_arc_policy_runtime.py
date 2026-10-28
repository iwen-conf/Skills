from __future__ import annotations

import json
import shlex
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

from test_arc_policy import ENV, PLUGIN, git, make_repo, write_overlay


def copy_plugin(tmp_path: Path) -> Path:
    plugin = tmp_path / "source" / "plugins" / "arc-policy"
    shutil.copytree(
        PLUGIN, plugin, ignore=shutil.ignore_patterns("__pycache__", ".venv")
    )
    (plugin.parents[1] / "pyproject.toml").write_text('[project]\nname = "fixture"\n')
    return plugin


def entry_env(plugin: Path) -> dict[str, str]:
    return {
        **ENV,
        "GROK_PLUGIN_ROOT": str(plugin),
        "CLAUDE_PLUGIN_ROOT": str(plugin),
        "PLUGIN_ROOT": str(plugin),
    }


@pytest.mark.parametrize("runtime", ["plugin", "source"])
@pytest.mark.parametrize("entry", ["cli", "hook"])
def test_entries_use_installed_runtime_before_loading_dependencies(
    tmp_path: Path, runtime: str, entry: str
) -> None:
    plugin = copy_plugin(tmp_path)
    owner = plugin if runtime == "plugin" else plugin.parents[1]
    (owner / ".venv").symlink_to(sys.prefix, target_is_directory=True)
    if runtime == "plugin":
        wrong = plugin.parents[1] / ".venv" / "bin" / "python"
        wrong.parent.mkdir(parents=True)
        wrong.write_text("#!/bin/sh\nexit 77\n")
        wrong.chmod(0o755)
    repo = make_repo(tmp_path)
    if entry == "cli":
        launcher = tmp_path / "arc-policy"
        launcher.symlink_to(plugin / "bin" / "arc-policy")
        args = [str(launcher), "--cwd", str(repo), "detect"]
    else:
        args = [str(plugin / "hooks" / "hook.py")]
    result = subprocess.run(
        [sys.executable, "-S", *args],
        input=json.dumps({"hook_event_name": "SessionStart", "cwd": str(repo)}),
        capture_output=True,
        text=True,
        env=entry_env(plugin),
        cwd=tmp_path,
        timeout=15,
    )
    assert result.returncode == 0, result.stderr
    output = json.loads(result.stdout)
    if entry == "cli":
        assert output["plugin_root"] == str(plugin)
        assert output["repo_root"] == str(repo)
    else:
        assert "Arc Policy Runtime is active." in output["systemMessage"]


@pytest.mark.parametrize("entry", ["cli", "hook"])
def test_missing_runtime_explains_repair_and_keeps_edits_denied(
    tmp_path: Path, entry: str
) -> None:
    plugin = copy_plugin(tmp_path)
    (tmp_path / ".venv").symlink_to(sys.prefix, target_is_directory=True)
    script = plugin / ("bin/arc-policy" if entry == "cli" else "hooks/hook.py")
    result = subprocess.run(
        [sys.executable, "-S", str(script), "detect"],
        input=json.dumps(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": "search_replace",
                "tool_input": {"file_path": "a.py"},
                "cwd": str(tmp_path),
            }
        ),
        capture_output=True,
        text=True,
        env=entry_env(plugin),
        cwd=tmp_path,
        timeout=15,
    )
    if entry == "cli":
        assert result.returncode != 0
        message = result.stderr
        assert "Traceback" not in message
    else:
        assert result.returncode == 0, result.stderr
        output = json.loads(result.stdout)
        assert output["permission"] == "deny"
        message = output["agent_message"]
    assert "PyYAML" in message
    assert "uv pip install" in message


@pytest.mark.parametrize("delivered", [False, True])
def test_denied_edit_provides_executable_agent_recovery(
    tmp_path: Path, delivered: bool
) -> None:
    from arc_policy.cli import main
    from arc_policy.hook import run_hook
    from arc_policy.store import active_task, load_state

    repo = make_repo(tmp_path).rename(tmp_path / "repo with spaces")
    if delivered:
        write_overlay(repo, {"verification": {"required": ["cli_receipt"]}})
        assert (
            main(
                [
                    "--cwd",
                    str(repo),
                    "task",
                    "begin",
                    "--skill",
                    "arc-code",
                    "--slug",
                    "delivered",
                ]
            )
            == 0
        )
        (repo / "feature.txt").write_text("done\n")
        assert main(["--cwd", str(repo), "task", "verify", "--no-tests"]) == 0
        git(repo, "add", "feature.txt")
        git(repo, "commit", "-q", "-m", "feature")
    payload = json.dumps(
        {
            "hook_event_name": "PreToolUse",
            "tool_name": "search_replace",
            "tool_input": {"file_path": str(repo / "next.py")},
        }
    )
    denied = json.loads(run_hook(payload, cwd=tmp_path))
    assert denied["permission"] == "deny"
    task = active_task(repo)
    if delivered:
        assert task is not None and task["slug"] == "delivered"
    else:
        assert task is None
    message = denied["agent_message"]
    command = message.split("`")[1]
    argv = shlex.split(command)
    assert argv == [
        "arc-policy",
        "--cwd",
        str(repo),
        "task",
        "begin",
        "--skill",
        "arc-code",
    ]
    assert "without asking the user" in message
    assert main(argv[1:]) == 0
    assert run_hook(payload, cwd=tmp_path) == ""
    if delivered:
        assert load_state(repo)["history"][-1]["slug"] == "delivered"
