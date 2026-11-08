from __future__ import annotations

import copy
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path

import arc_policy.hook as engine
import pytest
from arc_policy.fallback import deny, fallback_output
from arc_policy.host import call_key, edit_targets, payload_cwd, tool_input
from arc_policy.protocol import adapt_output, encode_response
from arc_policy.store import begin_task
from jsonschema import Draft7Validator, ValidationError
from test_arc_policy import ENV, PLUGIN, make_repo, write_overlay
from test_arc_policy_runtime import copy_plugin

FIXTURES = Path(__file__).parent / "fixtures" / "codex-hooks"
EVENT_FILES = {
    "SessionStart": "session-start",
    "PreToolUse": "pre-tool-use",
    "PostToolUse": "post-tool-use",
    "Stop": "stop",
}


@pytest.fixture(autouse=True)
def state_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ARC_POLICY_STATE_DIR", str(tmp_path / "state"))


def event_payload(event: str, repo: Path | None = None) -> dict:
    fixtures = json.loads((FIXTURES / "events.json").read_text())
    payload = copy.deepcopy(fixtures[event])
    if repo is not None:
        payload["cwd"] = str(repo)
    return payload


def validator(event: str, direction: str = "output") -> Draft7Validator:
    name = f"{EVENT_FILES[event]}.command.{direction}.schema.json"
    return Draft7Validator(json.loads((FIXTURES / name).read_text()))


def valid_response(event: str, raw: str) -> dict:
    result = json.loads(raw)
    validator(event).validate(result)
    return result


def run_entry(
    payload: dict,
    tmp_path: Path,
    *,
    plugin: Path = PLUGIN,
    host: str = "codex",
    no_site: bool = False,
    environment: dict[str, str | None] | None = None,
) -> subprocess.CompletedProcess[str]:
    env = {
        **ENV,
        "GROK_PLUGIN_ROOT": str(plugin),
        "CLAUDE_PLUGIN_ROOT": str(plugin),
        "PLUGIN_ROOT": str(plugin),
        "ARC_POLICY_HOME": str(tmp_path / "arc-home"),
        "ARC_POLICY_STATE_DIR": str(tmp_path / "state"),
    }
    env.pop("ARC_POLICY_ROOT", None)
    for key, value in (environment or {}).items():
        if value is None:
            env.pop(key, None)
        else:
            env[key] = value
    args = [sys.executable]
    if no_site:
        args.append("-S")
    args += [str(plugin / "hooks" / "hook.py"), "--host", host]
    result = subprocess.run(
        args,
        input=json.dumps(payload),
        cwd=tmp_path,
        env=env,
        text=True,
        capture_output=True,
        timeout=15,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    return result


def test_codex_schema_snapshots_are_unmodified_upstream_blobs() -> None:
    sources = json.loads((FIXTURES / "sources.json").read_text())
    assert sources["source_ref"] == "rust-v0.155.1"
    assert sources["compared_ref"] == "rust-v0.166.0"
    for name, provenance in sources["files"].items():
        raw = (FIXTURES / name).read_bytes()
        digest = hashlib.sha1(f"blob {len(raw)}\0".encode() + raw).hexdigest()
        assert digest == provenance["git_blob_sha"], name


@pytest.mark.parametrize("event", EVENT_FILES)
def test_codex_input_fixtures_match_official_wire_schema(event: str) -> None:
    validator(event, "input").validate(event_payload(event))


@pytest.mark.parametrize(
    "event,response",
    [
        ("SessionStart", engine._session_context("Arc workflow")),
        ("PreToolUse", deny("Begin a task first")),
        ("Stop", engine._stop_block("Run task verify")),
    ],
)
def test_codex_projection_preserves_policy_decisions_and_other_hosts(
    event: str, response: dict
) -> None:
    payload = event_payload(event)
    original = copy.deepcopy(response)
    # The old mixed envelope is invalid even though it contains a correct decision.
    with pytest.raises(ValidationError):
        validator(event).validate(response)
    raw = json.dumps(response)
    assert adapt_output(json.dumps(payload), raw) == raw
    adapted = valid_response(
        event, adapt_output(json.dumps(payload), raw, host="codex")
    )
    assert response == original
    if event == "PreToolUse":
        assert adapted["hookSpecificOutput"]["permissionDecision"] == "deny"
    elif event == "Stop":
        assert adapted["decision"] == "block"
        assert adapted["reason"] == response["reason"]
    else:
        assert adapted["hookSpecificOutput"]["additionalContext"] == "Arc workflow"


def test_codex_config_selects_protocol_and_only_registered_tool_paths() -> None:
    hooks = json.loads((PLUGIN / "hooks" / "codex.json").read_text())["hooks"]
    assert set(hooks) == set(EVENT_FILES)
    for event, groups in hooks.items():
        for group in groups:
            for command in group["hooks"]:
                assert command["type"] == "command"
                assert command["command"].endswith('hooks/hook.py" --host codex')
                assert "${PLUGIN_ROOT}" in command["command"]
                assert not command.get("async", False)
            if event in {"PreToolUse", "PostToolUse"}:
                assert re.fullmatch(group["matcher"], "Bash")
                assert re.fullmatch(group["matcher"], "apply_patch")
                assert not re.search(group["matcher"], "mcp__fs__write_file")


def test_codex_freeform_patch_and_session_fields_need_no_input_rewrite(
    tmp_path: Path,
) -> None:
    payload = event_payload("PreToolUse", tmp_path)
    assert edit_targets(tool_input(payload)) == ["feature.py"]
    assert payload_cwd(payload) == tmp_path
    assert call_key(payload) == call_key(event_payload("PostToolUse", tmp_path))
    assert payload["transcript_path"] is None
    assert payload["session_id"] == event_payload("Stop")["session_id"]


def test_codex_entry_session_denial_allow_post_and_stop(tmp_path: Path) -> None:
    """Run entry subprocesses; the test supplies the host's file side effect."""
    repo = make_repo(tmp_path)
    write_overlay(repo, {"stop": {"mode": "block"}})
    startup = valid_response(
        "SessionStart", run_entry(event_payload("SessionStart", repo), tmp_path).stdout
    )
    assert (
        "Arc Policy Runtime is active."
        in startup["hookSpecificOutput"]["additionalContext"]
    )

    pre = event_payload("PreToolUse", repo)
    rejected = valid_response("PreToolUse", run_entry(pre, tmp_path).stdout)
    assert rejected["hookSpecificOutput"]["permissionDecision"] == "deny"
    assert "task begin" in rejected["hookSpecificOutput"]["permissionDecisionReason"]

    begin_task(repo, skill="arc-code", slug="codex-contract")
    assert run_entry(pre, tmp_path).stdout == ""
    (repo / "feature.py").write_text("print('fixture')\n")
    assert run_entry(event_payload("PostToolUse", repo), tmp_path).stdout == ""

    stop = event_payload("Stop", repo)
    blocked = valid_response("Stop", run_entry(stop, tmp_path).stdout)
    assert blocked["decision"] == "block"
    assert "task verify" in blocked["reason"]
    stop["stop_hook_active"] = True
    reminder = valid_response("Stop", run_entry(stop, tmp_path).stdout)
    assert set(reminder) == {"systemMessage"}


@pytest.mark.parametrize(
    "command,denied", [("git status --short", False), ("git commit -m fixture", False)]
)
def test_codex_bash_guard_keeps_read_only_calls_silent(
    tmp_path: Path, command: str, denied: bool
) -> None:
    payload = event_payload("PreToolUse", make_repo(tmp_path))
    payload.update(tool_name="Bash", tool_input={"command": command})
    result = run_entry(payload, tmp_path)
    if denied:
        response = valid_response("PreToolUse", result.stdout)
        assert response["hookSpecificOutput"]["permissionDecision"] == "deny"
    else:
        assert result.stdout == ""


@pytest.mark.parametrize("inherited_override", [False, True])
def test_codex_entry_uses_selected_roots_policy_with_conflicting_host_environment(
    tmp_path: Path, inherited_override: bool
) -> None:
    repo = make_repo(tmp_path)
    foreign = tmp_path / "inherited-grok"
    policies = foreign / "policies"
    policies.mkdir(parents=True)
    default = (PLUGIN / "policies" / "default.yaml").read_text()
    assert "require_begin: true" in default
    (policies / "default.yaml").write_text(
        default.replace("require_begin: true", "require_begin: false")
    )
    result = run_entry(
        event_payload("PreToolUse", repo),
        tmp_path,
        environment={
            "GROK_PLUGIN_ROOT": str(foreign),
            "CLAUDE_PLUGIN_ROOT": str(foreign),
            "ARC_POLICY_ROOT": str(foreign) if inherited_override else None,
        },
    )
    assert result.stdout, "Codex must not load the inherited host's permissive policy"
    response = valid_response("PreToolUse", result.stdout)
    assert response["hookSpecificOutput"]["permissionDecision"] == "deny"
    assert "task begin" in response["hookSpecificOutput"]["permissionDecisionReason"]


def test_codex_engine_exception_uses_the_same_strict_response(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    def fail(*args, **kwargs):
        raise RuntimeError("fixture policy failure")

    monkeypatch.setattr(engine, "handle_event", fail)
    payload = event_payload("PreToolUse", tmp_path)
    raw = json.dumps(payload)
    response = valid_response(
        "PreToolUse", adapt_output(raw, engine.run_hook(raw), host="codex")
    )
    assert response["hookSpecificOutput"]["permissionDecision"] == "deny"
    assert (
        "fixture policy failure"
        in response["hookSpecificOutput"]["permissionDecisionReason"]
    )
    assert json.loads(fallback_output(payload, "fixture"))["permission"] == "deny"
    valid_response("PreToolUse", fallback_output(payload, "fixture", host="codex"))


@pytest.mark.parametrize("event", EVENT_FILES)
@pytest.mark.parametrize("missing", ["runtime", "policy", "fallback", "protocol"])
def test_codex_unavailable_and_emergency_paths_stay_valid(
    tmp_path: Path, event: str, missing: str
) -> None:
    plugin = copy_plugin(tmp_path)
    if missing != "runtime":
        (plugin / "lib" / "arc_policy" / f"{missing}.py").unlink()
    result = run_entry(
        event_payload(event, tmp_path),
        tmp_path,
        plugin=plugin,
        no_site=missing == "runtime",
    )
    response = valid_response(event, result.stdout)
    if event == "PreToolUse":
        assert response["hookSpecificOutput"]["permissionDecision"] == "deny"
        message = response["hookSpecificOutput"]["permissionDecisionReason"]
    else:
        message = response["systemMessage"]
    assert "engine unavailable" in message
    if missing in {"fallback", "protocol"}:
        assert "fallback failed" in message
    assert "Traceback" not in result.stderr


def test_codex_wrong_event_output_uses_explicit_failure() -> None:
    with pytest.raises(ValueError, match="does not match"):
        encode_response(
            event_payload("PreToolUse"),
            engine._session_context("wrong event"),
            host="codex",
        )


@pytest.mark.parametrize("without_adapter", [False, True])
def test_portable_entry_keeps_existing_aliases(
    tmp_path: Path, without_adapter: bool
) -> None:
    repo = make_repo(tmp_path)
    plugin = copy_plugin(tmp_path) if without_adapter else PLUGIN
    if without_adapter:
        (plugin / "lib" / "arc_policy" / "protocol.py").unlink()
    response = json.loads(
        run_entry(
            event_payload("PreToolUse", repo), tmp_path, plugin=plugin, host="portable"
        ).stdout
    )
    assert response["permission"] == "deny"
    assert response["agent_message"] == response["user_message"]
