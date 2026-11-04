"""Fallback decisions without the policy engine or third-party dependencies."""

from __future__ import annotations

import json
from typing import Any

from arc_policy.classify import classify
from arc_policy.host import event_name, tool_input, tool_name

FALLBACK_POLICY = {
    "workflow": {"require_begin": True},
    "execution": {
        "deny_direct": ["unverified_commit", "skip_hooks", "schema_migration"]
    },
}


def deny(reason: str) -> dict[str, Any]:
    return {
        "permission": "deny",
        "user_message": reason,
        "agent_message": reason,
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        },
    }


def fallback_output(
    payload: dict[str, Any], message: str, *, host: str = "portable"
) -> str:
    """Deny edits and controlled operations; otherwise report the runtime error."""
    response = {"systemMessage": message}
    if event_name(payload) == "PreToolUse":
        try:
            decision = classify(
                tool_name=tool_name(payload),
                tool_input=tool_input(payload),
                policy=FALLBACK_POLICY,
                has_task=False,
                receipt_ok=False,
            )
        except ValueError:
            response = deny(message)
        else:
            if decision["action"] == "deny":
                response = deny(message)
    if host == "portable":
        return json.dumps(response)
    from arc_policy.protocol import encode_response

    return encode_response(payload, response, host=host)
