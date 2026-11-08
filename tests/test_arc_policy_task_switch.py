from __future__ import annotations

import json
from pathlib import Path

import pytest
from arc_policy.cli import main
from arc_policy.detect import (
    normalize_skill_name,
    recommended_skill,
    resolve_skill_name,
)
from arc_policy.policy import load_named
from arc_policy.store import (
    active_task,
    begin_task,
    current_receipt,
    delivered_receipt,
    load_state,
    receipt_covers_commits,
    receipt_matches_tree,
    save_state,
    switch_task,
)
from arc_policy.verify import (
    authorize_task,
    check_architecture,
    requires_business_graph,
    verify_task,
)
from test_arc_policy import PLUGIN, git, make_repo, policy_for, write_overlay


def state_bytes(repo: Path) -> bytes:
    return (repo / ".arc/policy/state.json").read_bytes()


def verified_phase(repo: Path) -> dict:
    write_overlay(repo, {"verification": {"required": ["cli_receipt"]}})
    begin_task(
        repo,
        skill="arc-code",
        slug="repair",
        extra={"evidence": ["docs/progress.md#repair"]},
    )
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    task = active_task(repo)
    assert task is not None
    return task


@pytest.mark.parametrize("phase", ["explore", "plan", "code", "verify", "audit", "vet"])
def test_arc_source_names_work_at_cli_entry_and_stack_checks(tmp_path, capsys, phase):
    repo = make_repo(tmp_path)
    (repo / "go.mod").write_text("module example\n")
    source = f"arc:{phase}"
    packaged = f"arc-{phase}"
    policy = load_named("default", plugin=PLUGIN)
    assert resolve_skill_name(source, policy) == packaged
    assert resolve_skill_name(packaged, policy) == packaged
    assert main(["--cwd", str(repo), "task", "begin", "--skill", source]) == 0
    task = json.loads(capsys.readouterr().out)
    assert task["skill"] == task["slug"] == packaged
    assert check_architecture(repo, policy, {"skill": source})["verdict"] == "PASS"


@pytest.mark.parametrize("name", ["external:review", "external-review", "arc:custom"])
def test_unknown_skill_names_keep_their_identity(tmp_path, name):
    repo = make_repo(tmp_path)
    assert normalize_skill_name(name) == name
    assert resolve_skill_name(name, {}) == name
    task = begin_task(repo, skill=name, slug="external-task")
    assert task["skill"] == name


def test_policy_alias_and_stack_recommendation_accept_arc_source_names():
    policy = {
        "skill_aliases": {"arc:audit": ["legacy-review"]},
        "languages": {"go": {"skill": "arc:code"}},
    }
    assert resolve_skill_name("legacy-review", policy) == "arc-audit"
    assert resolve_skill_name("arc-audit", policy) == "arc-audit"
    assert recommended_skill(["go"], policy) == "arc-code"


@pytest.mark.parametrize(
    ("skill", "extra", "required"),
    [
        ("arc:verify", [], True),
        ("arc-verify", [], True),
        ("arc:code", ["arc-code"], True),
        ("arc-code", ["arc:code"], True),
        ("arc:code", [], False),
        ("external:review", ["external:review"], True),
        ("external-review", ["external:review"], False),
    ],
)
def test_graph_requirements_use_known_arc_identities_only(skill, extra, required):
    policy = {"verification": {"business_graph": {"require_for_skills": extra}}}
    assert requires_business_graph(skill, policy) is required


@pytest.mark.parametrize("boundary", ["current", "push", "delivered"])
@pytest.mark.parametrize("source_task", [False, True])
def test_historical_arc_names_match_at_receipt_boundaries(
    tmp_path, boundary, source_task
):
    repo = make_repo(tmp_path)
    verified_phase(repo)
    (repo / "feature.txt").write_text("fixed\n")
    git(repo, "add", "feature.txt")
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    state = load_state(repo)
    task = state["active"]
    task["skill"] = "arc:code" if source_task else "arc-code"
    task["receipt"]["skill"] = "arc-code" if source_task else "arc:code"
    save_state(repo, state)
    before = state_bytes(repo)
    if boundary == "current":
        assert receipt_matches_tree(repo, current_receipt(repo))
    else:
        git(repo, "commit", "-qm", "fix")
        if boundary == "push":
            assert receipt_covers_commits(repo, ["HEAD"])
        else:
            assert delivered_receipt(repo, active_task(repo)) is not None
    assert state_bytes(repo) == before


@pytest.mark.parametrize("boundary", ["current", "push", "delivered"])
def test_legacy_arc_verify_cannot_skip_its_graph_gate(tmp_path, boundary):
    repo = make_repo(tmp_path)
    write_overlay(
        repo,
        {
            "verification": {
                "required": ["cli_receipt"],
                "business_graph": {"graph": "unverified-candidate"},
            }
        },
    )
    begin_task(repo, skill="arc-code", slug="legacy")
    (repo / "feature.txt").write_text("fixed\n")
    git(repo, "add", "feature.txt")
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    state = load_state(repo)
    # An old arc:verify receipt could have omitted its mandatory graph check.
    state["active"]["skill"] = "arc:verify"
    state["active"]["receipt"]["skill"] = "arc-verify"
    save_state(repo, state)
    if boundary == "current":
        assert not receipt_matches_tree(repo, current_receipt(repo))
    else:
        git(repo, "commit", "-qm", "fix")
        if boundary == "push":
            assert not receipt_covers_commits(repo, ["HEAD"])
        else:
            assert delivered_receipt(repo, active_task(repo)) is None


@pytest.mark.parametrize("skill", ["arc-code", "arc-verify"])
def test_begin_keeps_an_unfinished_phase_by_default(tmp_path, skill):
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="repair")
    before = state_bytes(repo)
    with pytest.raises(RuntimeError, match="task status") as error:
        begin_task(repo, skill=skill, slug="next")
    assert "--force" not in str(error.value)
    assert "task end" in str(error.value) or "task switch" in str(error.value)
    assert state_bytes(repo) == before


def test_explicit_legacy_force_archives_receipt_and_evidence(tmp_path):
    repo = make_repo(tmp_path)
    previous = verified_phase(repo)
    (repo / "README.txt").write_text("unverified additional work\n")
    task = begin_task(repo, skill="arc:verify", slug="proof", force=True)
    archived = load_state(repo)["history"][-1]
    assert {key: archived[key] for key in previous} == previous
    assert archived["handoff"]["status"] == "replaced"
    assert "--force" in archived["handoff"]["reason"]
    assert task["previous_task_id"] == previous["id"]
    assert task["receipt"] is None


def test_verified_switch_preserves_phase_and_starts_without_receipt(tmp_path):
    repo = make_repo(tmp_path)
    previous = verified_phase(repo)
    task = switch_task(repo, skill="arc:verify", slug="proof")
    state = load_state(repo)
    assert state["active"] == task
    assert task["skill"] == "arc-verify"
    assert task["id"] != previous["id"]
    assert task["previous_task_id"] == previous["id"]
    assert task["receipt"] is None
    archived = state["history"][-1]
    assert {key: archived[key] for key in previous} == previous
    assert archived["handoff"] == {
        "to_task_id": task["id"],
        "to_skill": "arc-verify",
        "status": "verified",
    }
    assert archived["ended_at"]
    assert not receipt_matches_tree(repo, previous["receipt"])


@pytest.mark.parametrize("stale_receipt", [False, True])
def test_unfinished_or_stale_phase_cannot_switch_as_verified(tmp_path, stale_receipt):
    repo = make_repo(tmp_path)
    if stale_receipt:
        verified_phase(repo)
        (repo / "README.txt").write_text("unverified\n")
    else:
        begin_task(repo, skill="arc-code", slug="repair")
    before = state_bytes(repo)
    with pytest.raises(RuntimeError, match="no fresh verification"):
        switch_task(repo, skill="arc-verify", slug="proof")
    assert state_bytes(repo) == before


@pytest.mark.parametrize(
    ("incomplete", "reason"),
    [(True, None), (True, ""), (True, " \n "), (False, "not marked incomplete")],
)
def test_incomplete_switch_requires_an_explicit_reason(tmp_path, incomplete, reason):
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="repair")
    before = state_bytes(repo)
    with pytest.raises(ValueError, match="reason"):
        switch_task(
            repo, skill="arc-verify", slug="proof", incomplete=incomplete, reason=reason
        )
    assert state_bytes(repo) == before


def test_incomplete_switch_keeps_stale_receipt_and_records_reason(tmp_path):
    repo = make_repo(tmp_path)
    previous = verified_phase(repo)
    (repo / "README.txt").write_text("known incomplete work\n")
    task = switch_task(
        repo,
        skill="arc-plan",
        slug="rule-decision",
        incomplete=True,
        reason="  The unresolved rule is recorded in docs/progress.md.  ",
    )
    archived = load_state(repo)["history"][-1]
    assert archived["receipt"] == previous["receipt"]
    assert archived["handoff"]["status"] == "incomplete"
    assert archived["handoff"]["reason"] == (
        "The unresolved rule is recorded in docs/progress.md."
    )
    assert task["previous_task_id"] == previous["id"]
    assert task["receipt"] is None


def test_delivered_phase_can_switch_without_rechecking_old_tree(tmp_path):
    repo = make_repo(tmp_path)
    verified_phase(repo)
    (repo / "feature.txt").write_text("fixed\n")
    git(repo, "add", "feature.txt")
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    previous = active_task(repo)
    git(repo, "commit", "-qm", "fix")
    assert not receipt_matches_tree(repo, previous["receipt"])
    task = switch_task(repo, skill="arc-verify", slug="proof")
    archived = load_state(repo)["history"][-1]
    assert archived["handoff"]["status"] == "delivered"
    assert archived["receipt"] == previous["receipt"]
    assert task["receipt"] is None


def test_authorized_switch_never_labels_the_old_phase_verified(tmp_path):
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="repair")
    receipt = authorize_task(
        repo, "I explicitly authorize delivery without verification."
    )
    task = switch_task(repo, skill="arc-verify", slug="proof")
    archived = load_state(repo)["history"][-1]
    assert archived["handoff"]["status"] == "authorized"
    assert archived["receipt"] == receipt
    assert task["receipt"] is None


@pytest.mark.parametrize("skill", ["arc-code", "arc:code"])
@pytest.mark.parametrize("incomplete", [False, True])
def test_same_skill_switch_does_not_clear_the_task(tmp_path, skill, incomplete):
    repo = make_repo(tmp_path)
    verified_phase(repo)
    before = state_bytes(repo)
    with pytest.raises(RuntimeError, match="already uses"):
        switch_task(
            repo,
            skill=skill,
            slug="another-name",
            incomplete=incomplete,
            reason="requested" if incomplete else None,
        )
    assert state_bytes(repo) == before


def test_switch_without_active_task_does_not_create_one(tmp_path):
    repo = make_repo(tmp_path)
    with pytest.raises(RuntimeError, match="task begin"):
        switch_task(repo, skill="arc-verify", slug="proof")
    assert active_task(repo) is None


def test_switch_write_failure_leaves_the_old_phase_active(tmp_path, monkeypatch):
    repo = make_repo(tmp_path)
    verified_phase(repo)
    before = state_bytes(repo)

    def fail_replace(*args):
        raise OSError("simulated atomic state write failure")

    monkeypatch.setattr("arc_policy.store.os.replace", fail_replace)
    with pytest.raises(OSError, match="state write failure"):
        switch_task(repo, skill="arc-verify", slug="proof")
    assert state_bytes(repo) == before


@pytest.mark.parametrize("operation", ["switch", "force_begin"])
def test_invalid_history_cannot_drop_previous_phase(tmp_path, operation):
    repo = make_repo(tmp_path)
    verified_phase(repo)
    state = load_state(repo)
    state["history"] = {"unexpected": "object"}
    save_state(repo, state)
    before = state_bytes(repo)
    with pytest.raises(RuntimeError, match="invalid Arc task history"):
        if operation == "switch":
            switch_task(repo, skill="arc-verify", slug="proof")
        else:
            begin_task(repo, skill="arc-verify", slug="proof", force=True)
    assert state_bytes(repo) == before


def test_cli_switch_no_longer_requires_installing_commit_hooks(tmp_path, monkeypatch):
    repo = make_repo(tmp_path)
    verified_phase(repo)
    git(repo, "config", "core.hooksPath", ".husky")
    monkeypatch.setattr("arc_policy.githook.config_hooks_supported", lambda: False)
    assert main(["--cwd", str(repo), "task", "switch", "--skill", "arc-verify"]) == 0
    assert active_task(repo)["skill"] == "arc-verify"
    assert not (repo / ".husky").exists()


def test_cli_switch_records_incomplete_handoff(tmp_path, capsys):
    repo = make_repo(tmp_path)
    previous = begin_task(repo, skill="arc:code", slug="repair")
    assert (
        main(
            [
                "--cwd",
                str(repo),
                "task",
                "switch",
                "--skill",
                "arc:plan",
                "--incomplete",
                "--reason",
                "Rule decision pending in docs/progress.md.",
            ]
        )
        == 0
    )
    task = json.loads(capsys.readouterr().out)
    assert task["skill"] == task["slug"] == "arc-plan"
    assert task["previous_task_id"] == previous["id"]
    assert task["receipt"] is None
    archived = load_state(repo)["history"][-1]
    assert archived["handoff"]["reason"] == "Rule decision pending in docs/progress.md."
