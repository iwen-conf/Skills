from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

import pytest
import yaml

from arc_policy.classify import classify
from arc_policy.cli import main
from arc_policy.detect import detect_languages, recommended_skill, resolve_skill_name
from arc_policy.githook import AGENT_MARKERS
from arc_policy.hook import run_hook
from arc_policy.policy import load_for_repo, load_named, merge
from arc_policy.store import begin_task, current_receipt, receipt_matches_tree
from arc_policy.verify import summarize, verify_task

PLUGIN = Path(__file__).resolve().parents[1] / "plugins" / "arc-policy"
LIB = PLUGIN / "lib"
# Subprocess Git runs as a human: the suite itself often runs inside an agent shell.
ENV = {
    key: value
    for key, value in os.environ.items()
    if key not in {"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY"}
    and key not in AGENT_MARKERS
}
ENV.update(
    {
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_AUTHOR_NAME": "Policy Test",
        "GIT_AUTHOR_EMAIL": "policy@example.invalid",
        "GIT_COMMITTER_NAME": "Policy Test",
        "GIT_COMMITTER_EMAIL": "policy@example.invalid",
        "GROK_PLUGIN_ROOT": str(PLUGIN),
        "CLAUDE_PLUGIN_ROOT": str(PLUGIN),
    }
)


def git(repo: Path, *args: str) -> None:
    subprocess.run(
        ["git", *args], cwd=repo, env=ENV, check=True, capture_output=True, text=True
    )


def make_repo(tmp_path: Path) -> Path:
    repo = tmp_path / "repo"
    repo.mkdir()
    git(repo, "-c", "init.defaultBranch=main", "init", "-q")
    (repo / "README.txt").write_text("hello\n", encoding="utf-8")
    git(repo, "add", "README.txt")
    git(repo, "commit", "-q", "-m", "init")
    return repo


def write_overlay(repo: Path, data: dict) -> None:
    path = repo / ".arc" / "policy.yaml"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(yaml.safe_dump(data), encoding="utf-8")


def policy_for(repo: Path) -> dict:
    return load_for_repo(repo, plugin=PLUGIN, languages=detect_languages(repo))


def test_merge_overlay_replaces_lists() -> None:
    merged = merge(
        {"verification": {"required": ["a", "b"]}},
        {"verification": {"required": ["c"]}},
    )
    assert merged["verification"]["required"] == ["c"]


def test_default_policy_loads() -> None:
    policy = load_named("default", plugin=PLUGIN)
    assert policy["mode"] == "free_write_forced_verify"
    assert policy["workflow"]["require_begin"] is True


def test_detect_go(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "go.mod").write_text("module example\n", encoding="utf-8")
    policy = load_named("default", plugin=PLUGIN)
    languages = detect_languages(repo, policy)
    assert languages == ["go"]
    assert recommended_skill(languages, policy) == "arc-code"


def test_skill_alias() -> None:
    policy = load_named("default", plugin=PLUGIN)
    assert resolve_skill_name("go-backend", policy) == "arc-code"
    assert resolve_skill_name("arc-verify", policy) == "arc-verify"
    assert resolve_skill_name("arc-vet", policy) == "arc-vet"
    assert resolve_skill_name("open-source-vetting", policy) == "arc-vet"


def test_vet_task_on_detected_stack_passes_architecture_check(tmp_path: Path) -> None:
    from arc_policy.verify import check_architecture

    repo = make_repo(tmp_path)
    (repo / "go.mod").write_text("module example\n", encoding="utf-8")
    policy = load_named("default", plugin=PLUGIN)
    assert check_architecture(repo, policy, {"skill": "arc-vet"})["verdict"] != "FAIL"
    assert check_architecture(repo, policy, {"skill": "android"})["verdict"] == "FAIL"


def test_classify_write_without_task_is_denied() -> None:
    policy = load_named("default", plugin=PLUGIN)
    decision = classify(
        tool_name="Write",
        tool_input={"path": "main.go", "contents": "package main\n"},
        policy=policy,
        has_task=False,
        receipt_ok=False,
    )
    assert decision["action"] == "deny"
    assert decision["operation"] == "source_edit"


def test_classify_write_with_task_is_allowed() -> None:
    policy = load_named("default", plugin=PLUGIN)
    decision = classify(
        tool_name="search_replace",
        tool_input={"path": "main.go"},
        policy=policy,
        has_task=True,
        receipt_ok=False,
    )
    assert decision["action"] == "allow"


def test_classify_commit_without_receipt_is_denied() -> None:
    policy = load_named("default", plugin=PLUGIN)
    decision = classify(
        tool_name="Bash",
        tool_input={"command": "git commit -m 'wip'"},
        policy=policy,
        has_task=True,
        receipt_ok=False,
    )
    assert decision["action"] == "deny"
    assert decision["operation"] == "commit"


def test_classify_commit_no_verify_is_denied_even_with_receipt() -> None:
    policy = load_named("default", plugin=PLUGIN)
    decision = classify(
        tool_name="run_terminal_command",
        tool_input={"command": "git commit --no-verify -m 'wip'"},
        policy=policy,
        has_task=True,
        receipt_ok=True,
    )
    assert decision["action"] == "deny"
    assert decision["operation"] == "skip_hooks"


def test_classify_cli_passthrough() -> None:
    policy = load_named("default", plugin=PLUGIN)
    decision = classify(
        tool_name="Bash",
        tool_input={"command": "arc-policy task verify"},
        policy=policy,
        has_task=False,
        receipt_ok=False,
    )
    assert decision["action"] == "allow"
    assert decision["operation"] == "cli"


def test_classify_migration_denied() -> None:
    policy = load_named("default", plugin=PLUGIN)
    decision = classify(
        tool_name="Bash",
        tool_input={"command": "alembic upgrade head"},
        policy=policy,
        has_task=True,
        receipt_ok=False,
    )
    assert decision["action"] == "deny"
    assert decision["operation"] == "schema_migration"


def test_hook_fail_open_on_invalid_json() -> None:
    assert run_hook("not-json") == ""
    assert run_hook("") == ""


def test_hook_session_start_injects_skill(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "go.mod").write_text("module example\n", encoding="utf-8")
    raw = run_hook(json.dumps({"hook_event_name": "SessionStart"}), cwd=repo)
    payload = json.loads(raw)
    assert "arc-code" in payload["systemMessage"]
    assert payload["hookSpecificOutput"]["hookEventName"] == "SessionStart"


def test_hook_pretooluse_deny_write_without_task(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    raw = run_hook(
        json.dumps(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": "Write",
                "tool_input": {"path": "a.go", "contents": "package a\n"},
            }
        ),
        cwd=repo,
    )
    payload = json.loads(raw)
    assert payload["hookSpecificOutput"]["permissionDecision"] == "deny"
    assert "task begin" in payload["hookSpecificOutput"]["permissionDecisionReason"]


def test_hook_pretooluse_silent_allow_does_not_emit_allow(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    raw = run_hook(
        json.dumps(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": "Write",
                "tool_input": {"path": "a.go", "contents": "package a\n"},
            }
        ),
        cwd=repo,
    )
    assert raw == ""


def test_begin_and_receipt_bind_to_tree(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    write_overlay(
        repo, {"verification": {"required": ["cli_receipt"], "fail_closed": True}}
    )
    begin_task(repo, skill="arc-code", slug="demo")
    receipt = verify_task(repo, policy_for(repo), run_tests=False)
    assert receipt["verdict"] == "PASS"
    assert receipt_matches_tree(repo, current_receipt(repo))
    (repo / "extra.txt").write_text("x\n", encoding="utf-8")
    assert not receipt_matches_tree(repo, current_receipt(repo))


def test_verify_fail_closed_when_tests_missing(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    receipt = verify_task(repo, policy_for(repo), run_tests=True)
    assert receipt["verdict"] == "FAIL"
    tests = next(item for item in receipt["checks"] if item["id"] == "tests")
    assert tests["verdict"] == "NOT_RUN"
    assert current_receipt(repo) is None


def test_summarize_fail_closed() -> None:
    policy = {"verification": {"required": ["tests"], "fail_closed": True}}
    assert summarize([{"id": "tests", "verdict": "NOT_RUN"}], policy) == "FAIL"
    assert summarize([{"id": "tests", "verdict": "PASS"}], policy) == "PASS"


def test_stop_blocks_completion_claim(tmp_path: Path, monkeypatch) -> None:
    monkeypatch.setattr(
        "arc_policy.hook.looks_like_completion", lambda message, markers: True
    )
    repo = make_repo(tmp_path)
    write_overlay(
        repo, {"verification": {"required": ["cli_receipt"], "fail_closed": True}}
    )
    begin_task(repo, skill="arc-code", slug="demo")
    (repo / "changed.txt").write_text("x\n", encoding="utf-8")
    raw = run_hook(
        json.dumps(
            {
                "hook_event_name": "Stop",
                "reason": "end_turn",
                "last_assistant_message": "已完成，可以提交。",
            }
        ),
        cwd=repo,
    )
    payload = json.loads(raw)
    assert payload["decision"] == "block"
    assert "task verify" in payload["reason"]


def test_stop_ignores_session_end_reason(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    raw = run_hook(
        json.dumps({"hook_event_name": "Stop", "reason": "channel_closed"}),
        cwd=repo,
    )
    assert raw == ""


def test_cli_task_begin(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    repo = make_repo(tmp_path)
    assert main(["--cwd", str(repo), "task", "begin", "--skill", "go-backend"]) == 0
    out = json.loads(capsys.readouterr().out)
    assert out["skill"] == "arc-code"


def test_plugin_layout_exists() -> None:
    assert (PLUGIN / "plugin.json").is_file()
    assert (PLUGIN / ".claude-plugin" / "plugin.json").is_file()
    assert (PLUGIN / ".codex-plugin" / "plugin.json").is_file()
    assert (PLUGIN / ".omp-plugin" / "plugin.json").is_file()
    assert (PLUGIN / "package.json").is_file()
    assert (PLUGIN / "hooks" / "hooks.json").is_file()
    assert (PLUGIN / "skills" / "arc-policy" / "SKILL.md").is_file()
    assert (PLUGIN / "hosts" / "omp" / "arc-policy.ts").is_file()
    assert (PLUGIN / "hosts" / "cursor" / "hooks.json").is_file()
    assert (PLUGIN / "hosts" / "cursor" / "run.sh").is_file()
    assert (PLUGIN / "hosts" / "dsh" / "package.json").is_file()
    assert (PLUGIN / "hosts" / "dsh" / "lib" / "index.js").is_file()
    hooks = json.loads((PLUGIN / "hooks" / "hooks.json").read_text(encoding="utf-8"))
    assert "SessionStart" in hooks["hooks"]
    assert "PreToolUse" in hooks["hooks"]
    assert "Stop" in hooks["hooks"]


def test_classify_cursor_shell_and_omp_bash_casefold() -> None:
    policy = load_named("default", plugin=PLUGIN)
    commit = classify(
        tool_name="Shell",
        tool_input={"command": "git commit -m wip"},
        policy=policy,
        has_task=True,
        receipt_ok=False,
    )
    assert commit["action"] == "deny"
    assert commit["operation"] == "commit"
    write = classify(
        tool_name="write",
        tool_input={"path": "main.go"},
        policy=policy,
        has_task=False,
        receipt_ok=False,
    )
    assert write["action"] == "deny"
    assert write["operation"] == "source_edit"
    bash = classify(
        tool_name="bash",
        tool_input={"command": "git push origin main"},
        policy=policy,
        has_task=True,
        receipt_ok=False,
    )
    assert bash["action"] == "deny"


def test_hook_cursor_pretooluse_without_event_name(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    raw = run_hook(
        json.dumps(
            {
                "tool_name": "Write",
                "tool_input": {"path": "a.go", "contents": "package a\n"},
                "cwd": str(repo),
            }
        ),
        cwd=repo,
    )
    payload = json.loads(raw)
    assert payload["permission"] == "deny"
    assert payload["hookSpecificOutput"]["permissionDecision"] == "deny"
    assert "task begin" in payload["user_message"]


def test_hook_cursor_before_shell_execution_commit(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    raw = run_hook(
        json.dumps(
            {
                "hook_event_name": "beforeShellExecution",
                "command": "git commit -m wip",
                "cwd": str(repo),
            }
        ),
        cwd=repo,
    )
    payload = json.loads(raw)
    assert payload["permission"] == "deny"
    assert payload["hookSpecificOutput"]["permissionDecision"] == "deny"


def test_hook_session_start_emits_cursor_additional_context(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    raw = run_hook(json.dumps({"hook_event_name": "sessionStart"}), cwd=repo)
    payload = json.loads(raw)
    assert "Arc Policy Runtime" in payload["systemMessage"]
    assert payload["additional_context"] == payload["systemMessage"]
