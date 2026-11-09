#!/usr/bin/env python3
"""Host-agnostic hook entry.

If the policy engine cannot load or crashes, edits and controlled operations are denied
(fail closed) and every other event gets the error as a message.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

# Events before a tool runs, compared case- and separator-insensitively.
_PRE_TOOL_USE = {"pretooluse", "beforeshellexecution", "beforemcpexecution"}


def _plugin_root(*, host: str = "portable") -> Path:
    keys = (
        ("PLUGIN_ROOT", "CLAUDE_PLUGIN_ROOT")
        if host == "codex"
        else ("GROK_PLUGIN_ROOT", "CLAUDE_PLUGIN_ROOT", "PLUGIN_ROOT")
    )
    for key in keys:
        value = os.environ.get(key)
        if value:
            return Path(value)
    return Path(__file__).resolve().parent.parent


def _unavailable(raw: str, exc: BaseException, *, host: str = "portable") -> str:
    message = f"arc-policy engine unavailable: {exc}; verification remains pending"
    try:
        payload = json.loads(raw)
    except ValueError:
        return ""
    if not isinstance(payload, dict):
        return ""
    try:
        from arc_policy.fallback import fallback_output

        return fallback_output(payload, message, host=host)
    except Exception as nested:
        message = f"{message}; fallback failed: {nested}"
    event = payload.get("hook_event_name") or payload.get("hookEventName")
    if isinstance(event, str) and event:
        before_tool = (
            event.casefold().replace("_", "").replace("-", "") in _PRE_TOOL_USE
        )
    else:
        before_tool = bool(payload.get("tool_name") or payload.get("toolName"))
    if before_tool:
        specific = {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": message,
        }
        # Keep this path dependency-free: the protocol module itself may be missing.
        if host == "codex":
            return json.dumps({"hookSpecificOutput": specific})
        return json.dumps(
            {
                "permission": "deny",
                "user_message": message,
                "agent_message": message,
                "hookSpecificOutput": specific,
            }
        )
    return json.dumps({"systemMessage": message})


def _native_cursor_hook_owns(raw: str) -> bool:
    """Cursor also runs hooks of imported Claude Code plugins; skip that second copy.

    Only a Cursor payload defers, and only when a native arc-policy Cursor hook is
    configured, so plain Claude Code and Cursor without native hooks stay enforced.
    """
    if os.environ.get("ARC_POLICY_CURSOR_NATIVE") == "1":
        return False
    try:
        payload = json.loads(raw)
    except ValueError:
        return False
    if not isinstance(payload, dict) or "cursor_version" not in payload:
        return False
    configs = [Path.home() / ".cursor" / "hooks.json"]
    roots = payload.get("workspace_roots")
    if isinstance(roots, list):
        configs += [
            Path(root) / ".cursor" / "hooks.json"
            for root in roots
            if isinstance(root, str)
        ]
    for config in configs:
        try:
            if "hosts/cursor/run.sh" in config.read_text(encoding="utf-8"):
                return True
        except OSError:
            continue
    return False


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", choices=("portable", "codex"), default="portable")
    # Existing host launchers may supply extra arguments; their behavior is unchanged.
    args, _ = parser.parse_known_args()
    root = _plugin_root(host=args.host).resolve()
    # Code and policy must come from the same host-selected installation, even
    # when a nested CLI inherited another host's plugin environment.
    os.environ["ARC_POLICY_ROOT"] = str(root)
    sys.path.insert(0, str(root / "lib"))
    raw = ""
    try:
        from arc_policy.runtime import ensure_runtime

        ensure_runtime()
        raw = sys.stdin.read()
        if _native_cursor_hook_owns(raw):
            return 0
        from arc_policy.hook import run_hook

        output = run_hook(raw)
        if args.host == "codex":
            from arc_policy.protocol import adapt_output

            output = adapt_output(raw, output, host=args.host)
    except Exception as exc:
        output = _unavailable(raw or sys.stdin.read(), exc, host=args.host)
    sys.stdout.write(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
