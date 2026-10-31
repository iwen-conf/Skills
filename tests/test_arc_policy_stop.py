from __future__ import annotations

import json
import os
import subprocess
import time
from pathlib import Path

import pytest

import arc_policy.classify as classifier
from arc_policy.hook import run_hook
from arc_policy.session import RETENTION_SECONDS, prune
from arc_policy.store import begin_task
from test_arc_policy import make_repo


@pytest.fixture(autouse=True)
def state_dir(tmp_path_factory: pytest.TempPathFactory, monkeypatch) -> Path:
    path = tmp_path_factory.mktemp("arc-state")
    monkeypatch.setenv("ARC_POLICY_STATE_DIR", str(path))
    return path


@pytest.fixture
def no_jev(monkeypatch) -> None:
    original = subprocess.run

    def run(argv, *args, **kwargs):
        if argv[0] == "jev":
            raise FileNotFoundError(2, "No such file or directory", "jev")
        return original(argv, *args, **kwargs)

    monkeypatch.setattr(classifier.subprocess, "run", run)


def judged(monkeypatch, probability: float) -> None:
    monkeypatch.setattr(
        classifier.subprocess,
        "run",
        lambda argv, *args, **kwargs: subprocess.CompletedProcess(
            argv,
            0,
            json.dumps(
                {"answers": {"completion": {"type": "noul", "noul": probability}}}
            ),
            "",
        ),
    )


def hook(payload: dict, cwd: Path | None = None) -> dict:
    raw = run_hook(json.dumps(payload), cwd=cwd)
    return json.loads(raw) if raw else {}


def grok_stop(repo: Path, session: str, message: str, active: bool = False) -> dict:
    """The Stop envelope Grok sends: camelCase keys plus Claude's snake_case aliases."""
    return {
        "hookEventName": "stop",
        "hook_event_name": "Stop",
        "sessionId": session,
        "session_id": session,
        "cwd": str(repo),
        "reason": "end_turn",
        "stopHookActive": active,
        "stop_hook_active": active,
        "lastAssistantMessage": message,
        "last_assistant_message": message,
    }


def edit(repo: Path, session: str, name: str, text: str) -> None:
    assert (
        hook(
            {
                "hook_event_name": "PreToolUse",
                "session_id": session,
                "cwd": str(repo),
                "tool_name": "Write",
                "tool_input": {"file_path": str(repo / name)},
            }
        )
        == {}
    )
    (repo / name).write_text(text)


def shell(repo: Path, session: str, call: str, command: str, write=None) -> None:
    """One shell tool call: PreToolUse, its side effect, then PostToolUse."""
    base = {
        "session_id": session,
        "cwd": str(repo),
        "tool_name": "Bash",
        "tool_use_id": call,
        "tool_input": {"command": command},
    }
    assert hook({**base, "hook_event_name": "PreToolUse"}) == {}
    if write:
        write()
    assert hook({**base, "hook_event_name": "PostToolUse"}) == {}


def started(tmp_path: Path, session: str) -> Path:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    hook({"hook_event_name": "SessionStart", "session_id": session, "cwd": str(repo)})
    return repo


def test_unavailable_judgment_still_blocks_a_completion_claim(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "claim")
    edit(repo, "claim", "a.txt", "x\n")
    result = hook(grok_stop(repo, "claim", "已修复，可以提交。"))
    assert result["decision"] == "block"
    assert "task verify" in result["reason"]


def test_unavailable_judgment_is_silent_without_a_claim(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "progress")
    edit(repo, "progress", "a.txt", "x\n")
    assert hook(grok_stop(repo, "progress", "Reading the router next.")) == {}


def test_judged_non_claim_is_silent(tmp_path: Path, monkeypatch) -> None:
    repo = started(tmp_path, "question")
    edit(repo, "question", "a.txt", "x\n")
    judged(monkeypatch, 0.02)
    assert hook(grok_stop(repo, "question", "Which route should own this?")) == {}


def test_session_is_reminded_once_per_tree_state(tmp_path: Path, no_jev: None) -> None:
    repo = started(tmp_path, "once")
    edit(repo, "once", "a.txt", "x\n")
    assert hook(grok_stop(repo, "once", "Done."))["decision"] == "block"
    assert hook(grok_stop(repo, "once", "Done.", active=True)) == {}
    assert hook(grok_stop(repo, "once", "Done.")) == {}
    edit(repo, "once", "a.txt", "y\n")
    assert hook(grok_stop(repo, "once", "Done."))["decision"] == "block"


def test_index_daemon_does_not_rearm_an_unchanged_source_notice(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "index-refresh")
    edit(repo, "index-refresh", "mine.txt", "unverified source\n")
    assert hook(grok_stop(repo, "index-refresh", "Done."))["decision"] == "block"

    def reindex() -> None:
        folder = repo / ".ai-code-index"
        folder.mkdir()
        for name in ("tags.json", "daemon.pid", "daemon.log"):
            (folder / name).write_text("generated\n")

    shell(repo, "index-refresh", "refresh", "arc-idx index --quiet", reindex)
    assert hook(grok_stop(repo, "index-refresh", "Done.")) == {}
    edit(repo, "index-refresh", "mine.txt", "new source change\n")
    assert hook(grok_stop(repo, "index-refresh", "Done."))["decision"] == "block"


def test_session_is_not_charged_for_changes_it_never_saw(
    tmp_path: Path, no_jev: None
) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="other-session")
    (repo / "theirs.txt").write_text("another session's work\n")
    hook({"hook_event_name": "SessionStart", "session_id": "reader", "cwd": str(repo)})
    assert hook(grok_stop(repo, "reader", "Done.")) == {}


def test_reader_is_not_charged_for_concurrent_writes(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "reader")
    shell(repo, "reader", "t1", "git status --short")
    (repo / "theirs.txt").write_text("another session\n")
    assert hook(grok_stop(repo, "reader", "Done.")) == {}


@pytest.mark.parametrize(
    "command",
    [
        "git status --short",
        "git diff --stat",
        "arc-policy task status",
        "arc-idx doctor --format json",
        "cat README.txt",
        "nl -ba README.txt | sed -n '1,20p'",
    ],
)
def test_reader_is_not_charged_for_writes_during_a_read_only_call(
    tmp_path: Path, no_jev: None, command: str
) -> None:
    repo = started(tmp_path, "reader-window")
    shell(
        repo,
        "reader-window",
        "read",
        command,
        lambda: (repo / "theirs.txt").write_text("concurrent writer\n"),
    )
    assert hook(grok_stop(repo, "reader-window", "Done.")) == {}


def test_unfinished_read_only_call_does_not_charge_another_repository(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "cross-reader")
    other_dir = tmp_path / "other"
    other_dir.mkdir()
    other = make_repo(other_dir)
    begin_task(other, skill="arc-code", slug="other-writer")
    assert (
        hook(
            {
                "hook_event_name": "PreToolUse",
                "session_id": "cross-reader",
                "cwd": str(repo),
                "tool_name": "Bash",
                "tool_use_id": "read-other",
                "tool_input": {"command": f"git -C {other} status --short"},
            }
        )
        == {}
    )
    (other / "theirs.txt").write_text("other session's source\n")
    assert hook(grok_stop(repo, "cross-reader", "Done.")) == {}


def test_explicit_edit_without_post_charges_only_its_target(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "known-editor")
    edit(repo, "known-editor", "mine.txt", "own source\n")
    (repo / "theirs.txt").write_text("other session's source\n")
    result = hook(grok_stop(repo, "known-editor", "Done."))
    assert result["decision"] == "block"
    assert "mine.txt" in result["reason"]
    assert "theirs.txt" not in result["reason"]


@pytest.mark.parametrize(
    "command",
    [
        "git status --short > mine.txt",
        "git diff --output=mine.txt",
        "sed -n '1p;w mine.txt' README.txt",
        "sed -n '1p' -e 'w mine.txt' README.txt",
        "time -o mine.txt git status --short",
        "nohup git status --short",
        "python3 build.py",
    ],
)
def test_commands_that_can_write_keep_their_verification_gate(
    tmp_path: Path, no_jev: None, command: str
) -> None:
    repo = started(tmp_path, "possible-writer")
    shell(
        repo,
        "possible-writer",
        "write",
        command,
        lambda: (repo / "mine.txt").write_text("own source\n"),
    )
    result = hook(grok_stop(repo, "possible-writer", "Done."))
    assert result["decision"] == "block"
    assert "mine.txt" in result["reason"]


@pytest.mark.parametrize(
    "message",
    [
        "The work is not done; tests are not completed.",
        "本轮尚未交付，不能提交。",
        "示例状态：\n```text\n已修复，可以提交。\n```\n实际测试尚未运行。",
        "> 已完成，可以提交。\n这是引用，实际工作仍在进行。",
    ],
)
def test_fallback_does_not_treat_denials_or_examples_as_completion(
    tmp_path: Path, no_jev: None, message: str
) -> None:
    repo = started(tmp_path, "not-finished")
    edit(repo, "not-finished", "mine.txt", "unfinished source\n")
    assert hook(grok_stop(repo, "not-finished", message)) == {}


def test_fallback_keeps_a_positive_claim_beside_an_unfinished_item(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "partial-claim")
    edit(repo, "partial-claim", "mine.txt", "unverified source\n")
    result = hook(grok_stop(repo, "partial-claim", "已修复后端；客户端尚未交付。"))
    assert result["decision"] == "block"


def test_shell_write_inside_its_tool_call_is_charged(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "writer")
    (repo / "before.txt").write_text("another session, earlier\n")
    script = "python3 - <<'PY'\nopen('mine.txt', 'w').write('x')\nPY"
    shell(repo, "writer", "t1", script, lambda: (repo / "mine.txt").write_text("x\n"))
    (repo / "after.txt").write_text("another session, later\n")
    result = hook(grok_stop(repo, "writer", "Done."))
    assert result["decision"] == "block"
    assert "mine.txt" in result["reason"]
    assert "before.txt" not in result["reason"]
    assert "after.txt" not in result["reason"]


def test_edit_tool_paths_are_charged_beside_tool_windows(
    tmp_path: Path, no_jev: None
) -> None:
    repo = started(tmp_path, "editor")
    shell(repo, "editor", "t1", "ls")
    edit(repo, "editor", "edited.txt", "x\n")
    (repo / "theirs.txt").write_text("another session\n")
    result = hook(grok_stop(repo, "editor", "Done."))
    assert result["decision"] == "block"
    assert "edited.txt" in result["reason"]
    assert "theirs.txt" not in result["reason"]


def test_tool_call_without_post_is_charged_at_stop(
    tmp_path: Path, no_jev: None
) -> None:
    """A failed call can skip PostToolUse; Stop still charges what changed while it was open."""
    repo = started(tmp_path, "failer")
    shell(repo, "failer", "t1", "ls")
    base = {
        "session_id": "failer",
        "cwd": str(repo),
        "tool_name": "Bash",
        "tool_use_id": "t2",
        "tool_input": {"command": "python3 build.py"},
    }
    assert hook({**base, "hook_event_name": "PreToolUse"}) == {}
    (repo / "partial.txt").write_text("x\n")
    result = hook(grok_stop(repo, "failer", "Done."))
    assert result["decision"] == "block"
    assert "partial.txt" in result["reason"]


def test_denied_shell_call_charges_nothing(tmp_path: Path, no_jev: None) -> None:
    repo = started(tmp_path, "denied")
    shell(repo, "denied", "t1", "ls")
    denied = hook(
        {
            "hook_event_name": "PreToolUse",
            "session_id": "denied",
            "cwd": str(repo),
            "tool_name": "Bash",
            "tool_use_id": "t2",
            "tool_input": {"command": "git commit -m x"},
        }
    )
    assert denied["hookSpecificOutput"]["permissionDecision"] == "deny"
    (repo / "theirs.txt").write_text("another session\n")
    assert hook(grok_stop(repo, "denied", "Done.")) == {}


def test_prune_removes_every_expired_session_file(state_dir: Path) -> None:
    sessions = state_dir / "sessions"
    sessions.mkdir()
    expired = time.time() - RETENTION_SECONDS - 60
    for name in ("s.jsonl", "s.stop", "s.windows.json", "s.lock"):
        path = sessions / name
        path.write_text("x")
        os.utime(path, (expired, expired))
    (sessions / "fresh.jsonl").write_text("x")
    prune()
    assert [path.name for path in sessions.iterdir()] == ["fresh.jsonl"]


@pytest.mark.parametrize(
    "message, claimed",
    [
        ("Done.", True),
        ("All tests pass; ready to commit.", True),
        ("已完成，可以提交", True),
        ("The abandoned-branch cleanup is next.", False),
        ("I'll fix the router after the review.", False),
    ],
)
def test_completion_markers_match_whole_words(message: str, claimed: bool) -> None:
    markers = ["done", "fixed", "ready to commit", "已完成"]
    assert classifier.mentions_completion(message, markers) is claimed


def test_cursor_conversation_is_reminded_once_per_tree_state(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    stop = {
        "hook_event_name": "stop",
        "conversation_id": "cursor-chat",
        "status": "completed",
        "loop_count": 0,
        "workspace_roots": [str(repo)],
    }
    hook({**stop, "hook_event_name": "sessionStart"})
    (repo / "a.txt").write_text("x\n")
    assert hook(stop)["decision"] == "block"
    assert hook(stop) == {}
