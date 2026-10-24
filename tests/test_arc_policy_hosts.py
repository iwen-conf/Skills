from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
from pathlib import Path

import pytest

from arc_policy.classify import MUTATE_TOOLS, SHELL_TOOLS

PLUGIN = Path(__file__).resolve().parents[1] / "plugins" / "arc-policy"


def run_adapter(
    tmp_path: Path, events: list[dict], replies: dict, hook_source: str | None = None
) -> dict:
    node = shutil.which("node")
    if node is None:
        pytest.skip(
            "Node.js with TypeScript stripping is required for host adapter tests"
        )
    plugin = tmp_path / "plugin"
    adapter = plugin / "hosts" / "omp" / "arc-policy.ts"
    adapter.parent.mkdir(parents=True)
    shutil.copyfile(PLUGIN / "hosts" / "omp" / "arc-policy.ts", adapter)
    hook = plugin / "hooks" / "hook.py"
    hook.parent.mkdir()
    hook.write_text(
        hook_source
        or "import json, os, sys\n"
        "payload = json.load(sys.stdin)\n"
        "with open(os.environ['ARC_POLICY_HOST_CALLS'], 'a') as stream:\n"
        "    stream.write(json.dumps({'payload': payload, 'cwd': os.getcwd()}) + '\\n')\n"
        "reply = json.loads(os.environ['ARC_POLICY_HOST_REPLIES']).get(payload['hook_event_name'], {})\n"
        "json.dump({} if payload.get('stop_hook_active') else reply, sys.stdout)\n",
        encoding="utf-8",
    )
    session = tmp_path / "session"
    session.mkdir()
    calls = tmp_path / "calls.jsonl"
    result = subprocess.run(
        [
            node,
            "--experimental-strip-types",
            "--input-type=module",
            "-e",
            """
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const data = JSON.parse(readFileSync(0, 'utf8'));
const { default: install } = await import(pathToFileURL(data.adapter));
const handlers = new Map();
const notices = [];
install({ on: (name, handler) => handlers.set(name, handler) });
const ctx = { cwd: data.cwd, ui: { notify: (...args) => notices.push(args) } };
const results = [];
for (const item of data.events) {
  const handler = handlers.get(item.name);
  const event = item.event ?? {};
  const result = handler ? await handler(event, ctx) : undefined;
  results.push({ registered: Boolean(handler), result: result ?? null, event });
}
console.log(JSON.stringify({ handlers: [...handlers.keys()], results, notices }));
""",
        ],
        input=json.dumps(
            {"adapter": str(adapter), "cwd": str(session), "events": events}
        ),
        text=True,
        capture_output=True,
        cwd=tmp_path,
        env={
            **os.environ,
            "ARC_POLICY_HOST_CALLS": str(calls),
            "ARC_POLICY_HOST_REPLIES": json.dumps(replies),
        },
        timeout=20,
        check=True,
    )
    output = json.loads(result.stdout)
    output["calls"] = (
        [json.loads(line) for line in calls.read_text().splitlines()]
        if calls.exists()
        else []
    )
    output["cwd"] = str(session)
    return output


def test_session_context_reaches_prompt_and_tools_use_context_cwd(
    tmp_path: Path,
) -> None:
    out = run_adapter(
        tmp_path,
        [
            {"name": "session_start"},
            {
                "name": "before_agent_start",
                "event": {"systemPromptOptions": {"promptGuidelines": ["existing"]}},
            },
            {
                "name": "tool_call",
                "event": {"toolName": "bash", "input": {"command": "git push"}},
            },
        ],
        {
            "SessionStart": {"systemMessage": "Detected stack: fixture-rust"},
            "PreToolUse": {"permission": "deny", "user_message": "verify first"},
        },
    )
    assert out["calls"]
    assert all(
        call["cwd"] == out["cwd"] == call["payload"]["cwd"] for call in out["calls"]
    )
    guidelines = out["results"][1]["event"]["systemPromptOptions"]["promptGuidelines"]
    assert "existing" in guidelines
    assert "Detected stack: fixture-rust" in guidelines
    assert out["results"][2]["result"] == {"block": True, "reason": "verify first"}


def pi_settle(*, outcome: str = "completed", entries: list | None = None) -> dict:
    return {
        "name": "agent_before_settle",
        "event": {
            "outcome": outcome,
            "entries": entries or [],
            "continue": False,
            "context": {
                "contextMessages": [
                    {"role": "assistant", "content": "old reply"},
                    {
                        "role": "assistant",
                        "content": [
                            {"type": "thinking", "thinking": "private"},
                            {"type": "text", "text": "Done."},
                            {"type": "text", "text": "Checked."},
                        ],
                    },
                    {
                        "role": "toolResult",
                        "content": [{"type": "text", "text": "not the reply"}],
                    },
                ],
                "canContinue": True,
            },
        },
    }


def test_pi_settle_continues_once_preserves_entries_and_reads_assistant(
    tmp_path: Path,
) -> None:
    existing = {
        "type": "custom",
        "customType": "other-extension",
        "data": {"keep": True},
    }
    out = run_adapter(
        tmp_path,
        [
            {"name": "before_agent_start", "event": {"systemPromptOptions": {}}},
            pi_settle(entries=[existing]),
            pi_settle(),
        ],
        {"Stop": {"decision": "block", "reason": "verify first"}},
    )
    assert out["results"][1]["registered"]
    result = out["results"][1]["result"]
    assert result["continue"] is True
    assert result["entries"][0] == existing
    assert result["entries"][-1]["type"] == "custom_message"
    assert result["entries"][-1]["content"] == "verify first"
    assert out["calls"][0]["payload"]["last_assistant_message"] == "Done.\nChecked."
    assert out["calls"][1]["payload"]["stop_hook_active"] is True
    assert out["results"][2]["result"] is None
    assert "agent_end" not in out["handlers"]


@pytest.mark.parametrize("outcome", ["aborted", "error"])
def test_pi_settle_leaves_aborted_or_failed_runs_alone(
    tmp_path: Path, outcome: str
) -> None:
    out = run_adapter(
        tmp_path,
        [pi_settle(outcome=outcome)],
        {"Stop": {"decision": "block", "reason": "verify"}},
    )
    assert out["results"][0]["registered"]
    assert out["results"][0]["result"] is None
    assert out["calls"] == []


@pytest.mark.parametrize(
    "reply", [{}, {"systemMessage": "Verification pending: classifier unavailable"}]
)
def test_pi_settle_does_not_continue_on_allow_or_passive_notice(
    tmp_path: Path, reply: dict
) -> None:
    out = run_adapter(tmp_path, [pi_settle()], {"Stop": reply})
    assert out["results"][0]["registered"]
    assert out["results"][0]["result"] is None
    if reply:
        assert out["notices"] == [[reply["systemMessage"], "warning"]]


def test_omp_session_stop_uses_actionable_result_and_recursion_flag(
    tmp_path: Path,
) -> None:
    event = {
        "messages": [],
        "last_assistant_message": {"role": "assistant", "content": "Ready."},
        "session_id": "fixture-session",
        "stop_hook_active": False,
    }
    out = run_adapter(
        tmp_path,
        [
            {"name": "session_stop", "event": event},
            {"name": "session_stop", "event": {**event, "stop_hook_active": True}},
        ],
        {"Stop": {"decision": "block", "reason": "verify first"}},
    )
    assert out["results"][0]["registered"]
    result = out["results"][0]["result"]
    assert result == {"continue": True, "additionalContext": "verify first"}
    assert out["calls"][0]["payload"]["last_assistant_message"] == "Ready."
    assert out["calls"][0]["payload"]["session_id"].startswith("omp-")
    assert out["calls"][1]["payload"]["stop_hook_active"] is True
    assert out["results"][1]["result"] is None


def test_omp_tool_calls_and_stop_share_one_session(tmp_path: Path) -> None:
    out = run_adapter(
        tmp_path,
        [
            {
                "name": "tool_call",
                "event": {"toolName": "bash", "input": {"command": "ls"}},
            },
            {"name": "session_stop", "event": {"messages": []}},
        ],
        {},
    )
    ids = {call["payload"]["session_id"] for call in out["calls"]}
    assert len(ids) == 1 and ids.pop().startswith("omp-")


@pytest.mark.parametrize(
    "hook_source",
    [
        "import sys\nsys.exit(3)\n",
        "print('not json')\n",
    ],
)
def test_omp_tool_call_blocks_when_engine_cannot_answer(
    tmp_path: Path, hook_source: str
) -> None:
    out = run_adapter(
        tmp_path,
        [{"name": "tool_call", "event": {"toolName": "write", "input": {"path": "a"}}}],
        {},
        hook_source=hook_source,
    )
    result = out["results"][0]["result"]
    assert result["block"] is True
    assert "arc-policy engine unavailable" in result["reason"]


@pytest.mark.parametrize("tool", sorted(SHELL_TOOLS | MUTATE_TOOLS))
def test_portable_hook_matcher_covers_classified_tools(tool: str) -> None:
    hooks = json.loads((PLUGIN / "hooks" / "hooks.json").read_text())["hooks"][
        "PreToolUse"
    ]
    assert any(re.fullmatch(hook["matcher"], tool) for hook in hooks), tool


def run_dsh(tmp_path: Path, hook_source: str, tool_input: dict) -> dict:
    node = shutil.which("node")
    if node is None:
        pytest.skip("Node.js is required for host adapter tests")
    plugin = tmp_path / "plugin"
    adapter = plugin / "hosts" / "dsh" / "lib" / "index.js"
    adapter.parent.mkdir(parents=True)
    shutil.copyfile(PLUGIN / "hosts" / "dsh" / "lib" / "index.js", adapter)
    (plugin / "hosts" / "dsh" / "package.json").write_text('{"type": "module"}')
    hook = plugin / "hooks" / "hook.py"
    hook.parent.mkdir()
    hook.write_text(hook_source, encoding="utf-8")
    result = subprocess.run(
        [
            node,
            "--input-type=module",
            "-e",
            """
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const data = JSON.parse(readFileSync(0, 'utf8'));
const { apply } = await import(pathToFileURL(data.adapter));
const handlers = new Map();
apply({ inject: () => {}, on: (name, handler) => handlers.set(name, handler) }, { injectPrompt: false });
const result = await handlers.get('tools/pre-execute')(
  { name: 'write', arguments: data.input }, async () => 'next');
console.log(JSON.stringify({ result, agent: process.env.ARC_POLICY_AGENT ?? null }));
""",
        ],
        input=json.dumps({"adapter": str(adapter), "input": tool_input}),
        text=True,
        capture_output=True,
        cwd=tmp_path,
        env={k: v for k, v in os.environ.items() if k != "ARC_POLICY_AGENT"},
        timeout=20,
        check=True,
    )
    return json.loads(result.stdout)


@pytest.mark.parametrize(
    "hook_source, expected",
    [
        ("import sys\nsys.exit(3)\n", "engine unavailable"),
        ("print('not json')\n", "engine unavailable"),
        (
            "import json\nprint(json.dumps({'permission': 'deny', 'user_message': 'begin first'}))\n",
            "begin first",
        ),
        ("print('{}')\n", None),
    ],
)
def test_dsh_pre_execute_fails_closed(
    tmp_path: Path, hook_source: str, expected: str | None
) -> None:
    out = run_dsh(tmp_path, hook_source, {"path": "a.txt"})
    assert out["agent"] == "1"
    if expected is None:
        assert out["result"] == "next"
    else:
        assert out["result"]["kind"] == "deny"
        assert expected in out["result"]["reason"]
