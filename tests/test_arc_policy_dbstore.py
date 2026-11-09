from __future__ import annotations

import json
import subprocess
from pathlib import Path

import pytest

from arc_policy.dbstore import DBState, load_db_state
from arc_policy.store import (
    active_task,
    begin_task,
    child_tasks,
    load_state,
    save_receipt,
    save_state,
)
from test_arc_policy import make_repo


def setup_migrated_repo(tmp_path: Path, monkeypatch):
    repo = make_repo(tmp_path)
    (repo / ".arc").mkdir(exist_ok=True)
    (repo / ".arc" / "arc.json").write_text(
        json.dumps({"active": True, "schema_version": 1, "db": ".arc/arc.db"})
    )
    backend = {"version": 1, "state": {"history": []}}
    calls = []

    def fake_arc(root, *args, data=None):
        assert root == repo
        calls.append((args, data))
        if args == ("get",):
            return {"mode": "arc.db", **backend}
        if args[:1] == ("put",):
            assert args[1] == "--expected-version"
            if int(args[2]) != backend["version"]:
                raise RuntimeError("CAS conflict")
            backend["version"] += 1
            backend["state"] = json.loads(data)
            return {"mode": "arc.db", "version": backend["version"]}
        raise AssertionError(f"unexpected Arc Policy command: {args}")

    monkeypatch.setattr("arc_policy.dbstore._arc_command", fake_arc)
    return repo, backend, calls


def test_migrated_policy_uses_db_for_task_and_receipt(tmp_path: Path, monkeypatch):
    repo, backend, calls = setup_migrated_repo(tmp_path, monkeypatch)
    old_state = repo / ".arc/policy/state.json"
    old_state.parent.mkdir(parents=True, exist_ok=True)
    old_state.write_text('{"active": {"id": "stale", "skill": "arc-code", "slug": "stale"}}')

    task = begin_task(repo, skill="arc-code", slug="repair")
    assert task["id"] == backend["state"]["active"]["id"]
    assert task["slug"] == active_task(repo)["slug"]
    assert backend["version"] == 2
    # Old file remains an archival source, and is not rewritten by any update.
    assert "stale" in old_state.read_text()

    receipt = {"task_id": task["id"], "slug": "repair", "verdict": "PASS"}
    stored_path = save_receipt(repo, receipt)
    assert stored_path == repo / ".arc" / "arc.db"
    assert backend["state"]["active"]["receipt"] == receipt
    assert not (repo / ".arc/policy/receipts/repair.json").exists()
    assert backend["version"] == 3
    assert any(argv[0] == "put" for argv, _ in calls)


def test_migrated_policy_state_cas_rejects_stale_writes(tmp_path: Path, monkeypatch):
    repo, backend, _ = setup_migrated_repo(tmp_path, monkeypatch)
    first = load_state(repo)
    second = load_state(repo)
    assert isinstance(first, DBState) and first.version == 1
    first["notice"] = "first"
    save_state(repo, first)
    second["notice"] = "stale"
    with pytest.raises(RuntimeError, match="CAS conflict"):
        save_state(repo, second)
    assert backend["state"]["notice"] == "first"


def test_backend_error_never_falls_back_to_legacy_json(tmp_path: Path, monkeypatch):
    repo, _, _ = setup_migrated_repo(tmp_path, monkeypatch)
    (repo / ".arc/policy").mkdir(exist_ok=True)
    (repo / ".arc/policy/state.json").write_text('{"active": {"id": "wrong"}}')

    def unavailable(*args, **kwargs):
        raise RuntimeError("database locked")

    monkeypatch.setattr("arc_policy.dbstore._arc_command", unavailable)
    with pytest.raises(RuntimeError, match="database locked"):
        load_state(repo)


def test_migrated_task_discovered_from_workspace(tmp_path: Path, monkeypatch):
    repo, backend, _ = setup_migrated_repo(tmp_path, monkeypatch)
    begin_task(repo, skill="arc-code", slug="workspace-task")
    assert not (repo / ".arc/policy/state.json").exists()
    assert child_tasks(tmp_path)[repo]["id"] == backend["state"]["active"]["id"]


def test_unmigrated_arc_db_retains_legacy_policy_authority(tmp_path: Path, monkeypatch):
    repo = make_repo(tmp_path)
    (repo / ".arc").mkdir(exist_ok=True)
    (repo / ".arc/arc.json").write_text('{"active": true, "schema_version": 1}')
    (repo / ".arc/policy").mkdir(exist_ok=True)
    (repo / ".arc/policy/state.json").write_text('{"history": []}')
    monkeypatch.setattr(
        "arc_policy.dbstore._arc_command",
        lambda *args, **kwargs: {"mode": "legacy", "version": 0, "state": {}},
    )
    assert load_db_state(repo) is None
    assert load_state(repo) == {"history": []}


def test_real_cli_failure_does_not_trigger_legacy_fallback(tmp_path: Path, monkeypatch):
    repo = make_repo(tmp_path)
    (repo / ".arc").mkdir(exist_ok=True)
    (repo / ".arc/arc.json").write_text('{"active": true, "schema_version": 1}')
    monkeypatch.setattr("arc_policy.dbstore.shutil.which", lambda name: "/tmp/arc")

    def failed(argv, **kwargs):
        return subprocess.CompletedProcess(argv, 2, "", "corrupt arc database")

    monkeypatch.setattr("arc_policy.dbstore.subprocess.run", failed)
    with pytest.raises(RuntimeError, match="corrupt arc database"):
        load_state(repo)
