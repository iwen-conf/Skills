"""Decisions when the policy engine cannot run. Imports nothing beyond the classifier."""

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


def fallback_output(payload: dict[str, Any], message: str) -> str:
    """Deny edits and controlled operations; other tool calls and events get the message."""
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
            return json.dumps(deny(message))
        if decision["action"] == "deny":
            return json.dumps(deny(message))
    return json.dumps({"systemMessage": message})
