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
