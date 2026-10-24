from __future__ import annotations

import json
import shutil
import subprocess
from pathlib import Path

import pytest

from arc_policy.classify import classify
from arc_policy.cli import main
from arc_policy.hook import run_hook
from arc_policy.store import begin_task
from arc_policy.verify import verify_task
from test_arc_policy import ENV, PLUGIN, git, make_repo, policy_for, write_overlay

NO_RECEIPT = {
    "execution": {
        "deny_direct": ["unverified_commit", "skip_hooks", "schema_migration"]
    }
}


@pytest.fixture(autouse=True)
def state_dir(tmp_path_factory: pytest.TempPathFactory, monkeypatch) -> Path:
    path = tmp_path_factory.mktemp("arc-state")
    monkeypatch.setenv("ARC_POLICY_STATE_DIR", str(path))
    return path


def shell(command: str) -> dict:
    return classify(
        tool_name="Bash",
        tool_input={"command": command},
        policy=NO_RECEIPT,
        has_task=True,
        receipt_ok=False,
    )


def hook(payload: dict) -> dict:
    raw = run_hook(json.dumps(payload))
    return json.loads(raw) if raw else {}


def named_repo(parent: Path, name: str) -> Path:
    holder = parent / f"{name}-holder"
    holder.mkdir()
    return make_repo(holder).rename(parent / name)


@pytest.mark.parametrize(
    "command",
    [
        "cat > notes.md <<'MD'\nDon't commit yet\ngit commit -m draft\n(cd elsewhere)\nMD",
        "python3 - <<'PY'\nprint('it\\'s fine')\nPY\necho done",
        "cat <<-EOF\n\tit's literal, `date` and $(date) run\n\tEOF",
        'cat <<"END" | wc -l\ngit push\nEND',
    ],
)
def test_heredoc_bodies_are_data(command: str) -> None:
    assert shell(command)["action"] == "allow"


@pytest.mark.parametrize(
    "command, operation",
    [
        ("cat <<EOF\n$(git commit -m x)\nEOF", "commit"),
        ('echo "$(git push)"', "commit"),
        ("echo `git commit -m x`", "commit"),
        ("version=$(alembic upgrade head)", "schema_migration"),
        ("bash -lc 'git commit -m x'", "commit"),
        ("bash -o pipefail -c 'git push'", "commit"),
        ('sh -c "alembic upgrade head"', "schema_migration"),
        ("eval git push", "commit"),
        ("timeout 30 git commit -m x", "commit"),
        ("nohup git push", "commit"),
        ("sudo -u me git commit -m x", "commit"),
        ("xargs -n1 git commit -m", "commit"),
        ("time git push", "commit"),
    ],
)
def test_nested_and_wrapped_controlled_commands_are_found(
    command: str, operation: str
) -> None:
    decision = shell(command)
    assert decision["action"] == "deny"
    assert decision["operation"] == operation


def test_truncated_shell_input_is_denied(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    denied = hook(
        {
            "hookEventName": "pre_tool_use",
            "hook_event_name": "PreToolUse",
            "toolName": "run_terminal_command",
            "toolInput": {"command": "echo start [truncated]"},
            "toolInputTruncated": True,
            "cwd": str(repo),
        }
    )
    assert denied["permission"] == "deny"
    assert "truncated" in denied["agent_message"]


@pytest.mark.parametrize(
    "tool, key",
    [("hashline_edit", "file_path"), ("EditNotebook", "target_notebook")],
)
def test_grok_edit_tools_are_gated(tmp_path: Path, tool: str, key: str) -> None:
    repo = make_repo(tmp_path)
    payload = {
        "hook_event_name": "PreToolUse",
        "tool_name": tool,
        "tool_input": {key: str(repo / "a.ipynb")},
        "cwd": str(repo),
    }
    assert hook(payload)["permission"] == "deny"
    begin_task(repo, skill="arc-code", slug="demo")
    assert hook(payload) == {}


def git_run(repo: Path, *args: str, **extra: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", *args],
        cwd=repo,
        env={**ENV, **extra},
        capture_output=True,
        text=True,
        check=False,
    )


def test_task_begin_installs_agent_only_git_hooks(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    write_overlay(repo, {"verification": {"required": ["cli_receipt"]}})
    assert main(["--cwd", str(repo), "task", "begin", "--skill", "arc-code"]) == 0
    assert (
        main(["--cwd", str(repo), "task", "begin", "--skill", "arc-code", "--force"])
        == 0
    )
    for event in ("pre-commit", "pre-push"):
        commands = git_run(
            repo, "config", "--get-all", f"hook.arc-policy-{event}.command"
        )
        assert len(commands.stdout.splitlines()) == 1
        assert f"git-hook {event}" in commands.stdout

    (repo / "a.txt").write_text("agent\n")
    git(repo, "add", "a.txt")
    blocked = git_run(repo, "commit", "-q", "-m", "agent", GROK_AGENT="1")
    assert blocked.returncode != 0
    assert "arc-policy task verify" in blocked.stderr
    assert git_run(repo, "commit", "-q", "-m", "human").returncode == 0

    (repo / "b.txt").write_text("verified\n")
    git(repo, "add", "b.txt")
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    assert git_run(repo, "commit", "-q", "-am", "agent", CLAUDECODE="1").returncode == 0
    push = ("hook", "run", "pre-push", "--", "origin", "https://example.invalid/x")
    assert git_run(repo, *push, CODEX_THREAD_ID="t").returncode != 0
    assert git_run(repo, *push).returncode == 0


def test_stop_checks_repositories_the_session_edited(tmp_path: Path) -> None:
    first = named_repo(tmp_path, "first")
    second = named_repo(tmp_path, "second")
    write_overlay(second, {"stop": {"mode": "block"}})
    begin_task(first, skill="arc-code", slug="first-task")
    begin_task(second, skill="arc-code", slug="second-task")
    session = {"session_id": "edits", "cwd": str(first)}
    write = {
        "hook_event_name": "PreToolUse",
        "tool_name": "Write",
        "tool_input": {"file_path": str(second / "b.txt")},
    }
    assert hook({**write, **session}) == {}
    (second / "b.txt").write_text("x\n")
    result = hook({"hook_event_name": "Stop", **session})
    assert result["decision"] == "block"
    assert "second-task is unverified" in result["reason"]
    assert str(second) in result["reason"]


def test_stop_reports_untasked_changes_made_through_the_shell(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    write_overlay(repo, {"stop": {"mode": "block"}})
    (repo / "old.txt").write_text("before the session\n")
    session = {"session_id": "shell", "cwd": str(repo)}
    hook({"hook_event_name": "SessionStart", **session})
    script = "python3 - <<'PY'\nopen('new.txt', 'w').write('x')\nPY"
    assert (
        hook(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": "Bash",
                "tool_input": {"command": script},
                **session,
            }
        )
        == {}
    )
    (repo / "new.txt").write_text("x\n")
    result = hook({"hook_event_name": "Stop", **session})
    assert result["decision"] == "block"
    assert "without an Arc task" in result["reason"]
    assert "new.txt" in result["reason"]
    assert "old.txt" not in result["reason"]


def test_shell_named_repository_is_tracked_for_stop(tmp_path: Path) -> None:
    workspace = tmp_path / "ditie"
    workspace.mkdir()
    other = named_repo(tmp_path, "other")
    write_overlay(other, {"stop": {"mode": "block"}})
    session = {"session_id": "named", "cwd": str(workspace)}
    command = f"python3 - <<'PY'\nopen('{other}/note.txt', 'w').write('x')\nPY"
    hook(
        {
            "hook_event_name": "PreToolUse",
            "tool_name": "run_terminal_command",
            "tool_input": {"command": command},
            **session,
        }
    )
    (other / "note.txt").write_text("x\n")
    result = hook({"hook_event_name": "Stop", **session})
    assert result["decision"] == "block"
    assert "note.txt" in result["reason"]


def test_stop_reports_changes_made_before_a_late_task_begin(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    write_overlay(repo, {"stop": {"mode": "block"}})
    session = {"session_id": "late", "cwd": str(repo)}
    hook({"hook_event_name": "SessionStart", **session})
    (repo / "early.txt").write_text("x\n")
    begin_task(repo, skill="arc-code", slug="late")
    result = hook({"hook_event_name": "Stop", **session})
    assert result["decision"] == "block"
    assert "early.txt" in result["reason"]


def test_cursor_stop_checks_every_workspace_root(tmp_path: Path) -> None:
    first = named_repo(tmp_path, "first")
    second = named_repo(tmp_path, "second")
    begin_task(second, skill="arc-code", slug="second-task")
    (second / "b.txt").write_text("x\n")
    result = hook(
        {
            "hook_event_name": "stop",
            "status": "completed",
            "loop_count": 0,
            "workspace_roots": [str(first), str(second)],
        }
    )
    assert result["decision"] == "block"
    assert str(second) in result["reason"]


def broken_plugin(tmp_path: Path) -> Path:
    plugin = tmp_path / "plugin"
    shutil.copytree(PLUGIN, plugin, ignore=shutil.ignore_patterns("__pycache__"))
    (plugin / "lib" / "arc_policy" / "policy.py").write_text(
        "raise ImportError('No module named yaml')\n"
    )
    return plugin


@pytest.mark.parametrize(
    "tool, tool_input, denied",
    [
        ("Write", {"file_path": "a.txt"}, True),
        ("Bash", {"command": "git commit -m x"}, True),
        ("Bash", {"command": "ls"}, False),
    ],
)
def test_hook_entry_fails_closed_when_engine_cannot_load(
    tmp_path: Path, tool: str, tool_input: dict, denied: bool
) -> None:
    plugin = broken_plugin(tmp_path)
    result = subprocess.run(
        ["python3", str(plugin / "hooks" / "hook.py")],
        input=json.dumps(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": tool,
                "tool_input": tool_input,
                "cwd": str(tmp_path),
            }
        ),
        env={**ENV, "GROK_PLUGIN_ROOT": str(plugin), "CLAUDE_PLUGIN_ROOT": str(plugin)},
        capture_output=True,
        text=True,
        check=True,
    )
    output = json.loads(result.stdout) if result.stdout.strip() else {}
    assert (output.get("permission") == "deny") is denied
    assert "No module named yaml" in json.dumps(output)
