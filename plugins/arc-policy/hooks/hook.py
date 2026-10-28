#!/usr/bin/env python3
"""Host-agnostic hook entry.

If the policy engine cannot load or crashes, edits and controlled operations are denied
(fail closed) and every other event gets the error as a message.
"""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

_PRE_TOOL_USE = {"PreToolUse", "pre_tool_use", "pretooluse", "beforeShellExecution"}


def _plugin_root() -> Path:
    for key in (
        "GROK_PLUGIN_ROOT",
        "CLAUDE_PLUGIN_ROOT",
        "PLUGIN_ROOT",
    ):
        value = os.environ.get(key)
        if value:
            return Path(value)
    return Path(__file__).resolve().parent.parent


def _unavailable(raw: str, exc: BaseException) -> str:
    message = f"arc-policy engine unavailable: {exc}; verification remains pending"
    try:
        payload = json.loads(raw)
    except ValueError:
        return ""
    if not isinstance(payload, dict):
        return ""
    try:
        from arc_policy.fallback import fallback_output

        return fallback_output(payload, message)
    except Exception as nested:
        message = f"{message}; fallback failed: {nested}"
    event = payload.get("hook_event_name") or payload.get("hookEventName")
    if event in _PRE_TOOL_USE or payload.get("tool_name") or payload.get("toolName"):
        return json.dumps(
            {
                "permission": "deny",
                "user_message": message,
                "agent_message": message,
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": message,
                },
            }
        )
    return json.dumps({"systemMessage": message})


def main() -> int:
    raw = sys.stdin.read()
    sys.path.insert(0, str(_plugin_root() / "lib"))
    try:
        from arc_policy.hook import run_hook

        output = run_hook(raw)
    except Exception as exc:
        output = _unavailable(raw, exc)
    sys.stdout.write(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
