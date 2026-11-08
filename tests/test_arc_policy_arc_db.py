from __future__ import annotations

import json
import subprocess
from pathlib import Path

import pytest

from arc_policy.hook import handle_post_tool_use
from arc_policy.verify import check_business_graph
from test_arc_policy import make_repo
from test_arc_policy_business_graph import graph_proof


def enabled_repo(tmp_path: Path) -> Path:
    repo = make_repo(tmp_path)
    (repo / ".arc").mkdir()
    (repo / ".arc/arc.json").write_text(
        json.dumps({"schema_version": 1, "active": True, "db": ".arc/arc.db"})
    )
    (repo / ".arc/arc.db").write_bytes(b"test-placeholder")
    return repo


def test_hook_reconciles_active_arc_db_after_edit(tmp_path: Path, monkeypatch):
    repo = enabled_repo(tmp_path)
    target = repo / "example.go"
    target.write_text("package example")
    commands = []

    monkeypatch.setattr(
        "arc_policy.hook.shutil.which",
        lambda name: "/tmp/arc" if name == "arc" else None,
    )

    def fake_run(argv, **kwargs):
        commands.append((argv, kwargs))
        return subprocess.CompletedProcess(argv, 0, '{"index":"pending"}', "")

    monkeypatch.setattr("arc_policy.hook.subprocess.run", fake_run)
    handle_post_tool_use(
        {
            "hook_event_name": "PostToolUse",
            "tool_name": "Write",
            "tool_input": {"file_path": str(target)},
            "cwd": str(repo),
            "session_id": "arc-db-test",
            "tool_use_id": "write-1",
        }
    )
    assert commands
    assert commands[0][0] == [
        "/tmp/arc", "update", "--changed-only", "--repo", str(repo)
    ]


def test_hook_without_active_arc_db_does_not_call_arc(tmp_path: Path, monkeypatch):
    repo = make_repo(tmp_path)

    def forbidden(*args, **kwargs):
        raise AssertionError("arc should not run before explicit cutover")

    monkeypatch.setattr("arc_policy.hook.subprocess.run", forbidden)
    handle_post_tool_use(
        {
            "hook_event_name": "PostToolUse",
            "tool_name": "Write",
            "tool_input": {"file_path": str(repo / "foo.go")},
            "cwd": str(repo),
            "session_id": "arc-db-uninitialized",
            "tool_use_id": "write-2",
        }
    )


def test_hook_reports_failed_reconciliation(tmp_path: Path, monkeypatch):
    repo = enabled_repo(tmp_path)
    monkeypatch.setattr("arc_policy.hook.shutil.which", lambda _: "/tmp/arc")
    monkeypatch.setattr(
        "arc_policy.hook.subprocess.run",
        lambda argv, **kwargs: subprocess.CompletedProcess(argv, 2, "", "locked"),
    )
    with pytest.raises(RuntimeError, match="post-edit sync failed"):
        handle_post_tool_use(
            {
                "hook_event_name": "PostToolUse",
                "tool_name": "Write",
                "tool_input": {"file_path": str(repo / "file.txt")},
                "cwd": str(repo),
                "session_id": "arc-db-failed",
                "tool_use_id": "write-3",
            }
        )


def test_graph_gate_selects_new_db_only_after_cutover(tmp_path: Path, monkeypatch):
    repo = enabled_repo(tmp_path)
    commands = []
    monkeypatch.setattr("arc_policy.verify.shutil.which", lambda _: None)

    def fake_run(root, argv):
        commands.append(argv)
        return subprocess.CompletedProcess(argv, 0, json.dumps(graph_proof(root)), "")

    monkeypatch.setattr("arc_policy.verify._run", fake_run)
    proof = check_business_graph(
        repo, {"verification": {"business_graph": {"graph": "academic-v1"}}}
    )
    assert proof["verdict"] == "PASS"
    assert commands[0] == [
        "arc-verify", "gate", "--db", ".arc/arc.db",
        "--graph", "academic-v1",
    ]


def test_explicit_graph_database_is_never_silently_overridden(tmp_path: Path, monkeypatch):
    repo = enabled_repo(tmp_path)
    calls = []
    monkeypatch.setattr("arc_policy.verify.shutil.which", lambda _: None)

    def fake_run(root, argv):
        calls.append(argv)
        return subprocess.CompletedProcess(argv, 0, json.dumps(graph_proof(root)), "")

    monkeypatch.setattr("arc_policy.verify._run", fake_run)
    result = check_business_graph(
        repo, {"verification": {"business_graph": {
            "graph": "academic-v1", "db": ".arc/verify/data.db"
        }}},
    )
    assert result["verdict"] == "PASS"
    assert calls[0][3] == ".arc/verify/data.db"
