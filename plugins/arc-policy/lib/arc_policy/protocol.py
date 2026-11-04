"""Project policy responses onto the explicitly selected host's wire protocol.

The engine retains its portable response envelope. Codex rejects unknown output
fields, so its command entry emits only the fields used by our four registered
events. This module deliberately depends only on the standard library so that
the same projection is available when the policy runtime cannot load.
"""

from __future__ import annotations

import json
from typing import Any

HOSTS = ("portable", "codex")

_CODEX_FIELDS = {
    "SessionStart": {"systemMessage", "hookSpecificOutput"},
    "PreToolUse": {"systemMessage", "decision", "reason", "hookSpecificOutput"},
    "PostToolUse": {"systemMessage", "hookSpecificOutput"},
    "Stop": {"systemMessage", "decision", "reason"},
}
_CODEX_SPECIFIC_FIELDS = {
    "SessionStart": {"hookEventName", "additionalContext"},
    "PreToolUse": {
        "hookEventName",
        "permissionDecision",
        "permissionDecisionReason",
        "additionalContext",
    },
    "PostToolUse": {"hookEventName", "additionalContext"},
}


def encode_response(
    payload: dict[str, Any], response: dict[str, Any], *, host: str = "portable"
) -> str:
    """Serialize a policy response without exporting another host's aliases."""
    if host not in HOSTS:
        raise ValueError(f"unsupported hook host: {host}")
    if host == "portable":
        return json.dumps(response)
    event = payload.get("hook_event_name")
    allowed = _CODEX_FIELDS.get(event, {"systemMessage"})
    output = {key: value for key, value in response.items() if key in allowed}
    if "hookSpecificOutput" in output:
        specific = output["hookSpecificOutput"]
        if not isinstance(specific, dict) or specific.get("hookEventName") != event:
            raise ValueError("hook response event does not match the Codex input")
        output["hookSpecificOutput"] = {
            key: value
            for key, value in specific.items()
            if key in _CODEX_SPECIFIC_FIELDS[event]
        }
    return json.dumps(output, ensure_ascii=False) if output else ""


def adapt_output(raw: str, output: str, *, host: str = "portable") -> str:
    """Adapt engine stdout; a successful no-op stays empty with exit status zero."""
    if host == "portable" or not output.strip():
        return output
    payload = json.loads(raw)
    response = json.loads(output)
    if not isinstance(payload, dict) or not isinstance(response, dict):
        # The arguments are strings of the correct type; their JSON value is invalid.
        raise ValueError("hook input and output must be JSON objects")  # noqa: TRY004
    return encode_response(payload, response, host=host)
