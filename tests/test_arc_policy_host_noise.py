"""Hook noise seen in Grok, Claude Code, Cursor and Codex logs: false "hook error" denials."""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
from pathlib import Path

import pytest
from arc_policy.classify import classify
from arc_policy.hook import run_hook
from arc_policy.store import begin_task
from test_arc_policy import ENV, PLUGIN, make_repo

POLICY = {"execution": {"deny_direct": ["skip_hooks", "schema_migration"]}}


@pytest.fixture(autouse=True)
def state_dir(tmp_path_factory: pytest.TempPathFactory, monkeypatch) -> Path:
    path = tmp_path_factory.mktemp("arc-state")
    monkeypatch.setenv("ARC_POLICY_STATE_DIR", str(path))
    return path


def shell(command: str) -> dict:
    return classify(
        tool_name="Bash",
        tool_input={"command": command},
        policy=POLICY,
        has_task=True,
        receipt_ok=False,
    )


def hook(payload: dict) -> dict:
    raw = run_hook(json.dumps(payload))
    return json.loads(raw) if raw else {}


# Shapes taken from real denied calls; bash -n rejects each of them as well.
UNPARSEABLE_INSPECTION = [
    "python3 -c '\nimport re\nprint(re.findall(r'x=\"([^\"]+)\"', s))\n'",
    'ssh -o BatchMode=yes host \'python3 - <<"PY"\nprint("it\'s here")\nPY\'',
    'echo "unterminated',
]


@pytest.mark.parametrize("command", UNPARSEABLE_INSPECTION)
def test_unparseable_shell_without_controlled_words_is_left_to_the_shell(
    command: str,
) -> None:
    decision = shell(command)
    assert decision["action"] == "allow"
    assert decision["operation"] == "shell"


@pytest.mark.parametrize(
    "command, operation",
    [
        ('git commit --no-verify -m "wip', "skip_hooks"),
        ("echo 'x\ngit push -n origin", "skip_hooks"),
        ('alembic upgrade head; echo "x', "schema_migration"),
        ("psql -f migrate.sql; echo 'x", "schema_migration"),
    ],
)
def test_unparseable_shell_that_may_hide_controlled_operations_is_denied_clearly(
    command: str, operation: str
) -> None:
    decision = shell(command)
    assert decision["action"] == "deny"
    assert decision["operation"] == operation
    assert "cannot parse" in decision["reason"]
    assert "quot" in decision["reason"]


def test_unparseable_plain_commit_is_not_controlled() -> None:
    assert shell('git commit -m "wip')["action"] == "allow"


def test_hook_does_not_report_engine_error_for_unbalanced_quotes(
    tmp_path: Path,
) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    for command in UNPARSEABLE_INSPECTION:
        result = hook(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": "Bash",
                "tool_input": {"command": command},
                "cwd": str(repo),
                "session_id": "quotes",
            }
        )
        assert result == {}, result


def test_stop_skips_touched_repository_that_was_deleted(tmp_path: Path) -> None:
    holder = tmp_path / "holder"
    holder.mkdir()
    gone = make_repo(holder)
    session = {"session_id": "deleted-worktree"}
    hook({"hook_event_name": "SessionStart", "cwd": str(gone), **session})
    shutil.rmtree(gone)
    result = hook(
        {
            "hook_event_name": "Stop",
            "cwd": str(tmp_path),
            "stop_reason": "end_turn",
            **session,
        }
    )
    assert "hook error" not in json.dumps(result)


def native_cursor_home(tmp_path: Path) -> Path:
    home = tmp_path / "home"
    (home / ".cursor").mkdir(parents=True)
    run = PLUGIN / "hosts" / "cursor" / "run.sh"
    (home / ".cursor" / "hooks.json").write_text(
        json.dumps({"version": 1, "hooks": {"preToolUse": [{"command": str(run)}]}})
    )
    return home


def entry(plugin: Path, payload: dict, home: Path, **env: str) -> str:
    result = subprocess.run(
        [sys.executable, str(plugin / "hooks" / "hook.py")],
        input=json.dumps(payload),
        env={**ENV, "HOME": str(home), **env},
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    return result.stdout


def cursor_write(repo: Path) -> dict:
    return {
        "hook_event_name": "preToolUse",
        "cursor_version": "3.0.0",
        "conversation_id": "c1",
        "tool_name": "Write",
        "tool_input": {"file_path": str(repo / "a.txt")},
        "cwd": str(repo),
        "workspace_roots": [str(repo)],
    }


def test_cursor_imported_claude_plugin_hook_defers_to_native_cursor_hook(
    tmp_path: Path,
) -> None:
    repo = make_repo(tmp_path)
    home = native_cursor_home(tmp_path)
    # Cursor imports enabled Claude Code plugins and runs their hooks too.
    assert entry(PLUGIN, cursor_write(repo), home) == ""
    native = subprocess.run(
        [str(PLUGIN / "hosts" / "cursor" / "run.sh")],
        input=json.dumps(cursor_write(repo)),
        env={**ENV, "HOME": str(home)},
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    assert json.loads(native.stdout)["permission"] == "deny"


def test_cursor_without_native_hook_still_enforces_through_claude_plugin(
    tmp_path: Path,
) -> None:
    repo = make_repo(tmp_path)
    home = tmp_path / "bare-home"
    home.mkdir()
    assert json.loads(entry(PLUGIN, cursor_write(repo), home))["permission"] == "deny"


def test_claude_code_payload_is_never_treated_as_cursor(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    home = native_cursor_home(tmp_path)
    payload = {
        "hook_event_name": "PreToolUse",
        "session_id": "s1",
        "tool_name": "Write",
        "tool_input": {"file_path": str(repo / "a.txt")},
        "cwd": str(repo),
    }
    # A Cursor-launched terminal can leak CURSOR_* variables into Claude Code.
    out = entry(PLUGIN, payload, home, CURSOR_VERSION="3.0.0")
    assert json.loads(out)["hookSpecificOutput"]["permissionDecision"] == "deny"


def test_unified_arc_reads_count_as_read_only_like_the_backend():
    from arc_policy.classify import read_only_shell

    assert read_only_shell("arc search foo --format json")
    assert read_only_shell("arc idx search foo --format json")
    assert read_only_shell("arc idx doctor --format json")
    assert not read_only_shell("arc idx index --quiet")
    assert read_only_shell("arc-idx search foo")
    assert not read_only_shell("arc task create --title x")
    assert not read_only_shell("arc index --quiet")
