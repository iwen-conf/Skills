from __future__ import annotations

import re
from pathlib import Path
from typing import Any

_EDIT_PATH_KEYS = (
    "file_path",
    "filePath",
    "path",
    "target_file",
    "notebook_path",
    "target_notebook",
)
_PATCH_PATH = re.compile(
    r"^\*\*\* (?:Add File|Update File|Delete File|Move to): (.+?)\s*$", re.MULTILINE
)

_EVENT_ALIASES = {
    "stop": "Stop",
    "pre_tool_use": "PreToolUse",
    "pretooluse": "PreToolUse",
    "session_start": "SessionStart",
    "sessionstart": "SessionStart",
    "user_prompt_submit": "UserPromptSubmit",
    "beforeshellexecution": "PreToolUse",
    "beforemcpexecution": "PreToolUse",
    "before_shell_execution": "PreToolUse",
    "before_mcp_execution": "PreToolUse",
}


def event_name(payload: dict[str, Any]) -> str:
    for key in ("hook_event_name", "hookEventName", "event"):
        value = payload.get(key)
        if isinstance(value, str) and value:
            alias = _EVENT_ALIASES.get(value) or _EVENT_ALIASES.get(value.casefold())
            if alias:
                return alias
            if value in {"Stop", "PreToolUse", "SessionStart", "UserPromptSubmit"}:
                return value
            folded = value.casefold().replace("-", "").replace("_", "")
            alias = _EVENT_ALIASES.get(folded)
            if alias:
                return alias
            return value
    if tool_name(payload) or isinstance(payload.get("command"), str):
        return "PreToolUse"
    return ""


def tool_name(payload: dict[str, Any]) -> str:
    value = payload.get("tool_name") or payload.get("toolName")
    if isinstance(value, str) and value:
        return value
    if isinstance(payload.get("command"), str):
        return "Shell"
    return ""


def tool_input(payload: dict[str, Any]) -> dict[str, Any]:
    value = payload.get("tool_input") or payload.get("toolInput")
    if isinstance(value, dict):
        return value
    command = payload.get("command")
    if isinstance(command, str):
        return {"command": command}
    return {}


def payload_cwd(payload: dict[str, Any]) -> Path | None:
    nested = tool_input(payload)
    value = (
        payload.get("cwd")
        or payload.get("working_directory")
        or payload.get("workingDirectory")
    )
    roots = payload.get("workspace_roots")
    if not value and isinstance(roots, list) and roots and isinstance(roots[0], str):
        value = roots[0]
    base = Path(value) if isinstance(value, str) and value else None
    directory = (
        nested.get("workdir") or nested.get("working_directory") or nested.get("cwd")
    )
    if isinstance(directory, str) and directory:
        return (base or Path.cwd()) / directory
    if isinstance(value, str) and value:
        return base
    return None


def edit_targets(value: dict[str, Any]) -> list[str]:
    """Paths an edit tool writes: host path keys plus apply_patch file headers."""
    targets = [
        value[key]
        for key in _EDIT_PATH_KEYS
        if isinstance(value.get(key), str) and value[key]
    ]
    for key in ("patch", "input", "command"):
        text = value.get(key)
        if isinstance(text, str):
            targets.extend(_PATCH_PATH.findall(text))
    return targets


def last_assistant_message(payload: dict[str, Any]) -> str:
    value = (
        payload.get("last_assistant_message")
        or payload.get("lastAssistantMessage")
        or ""
    )
    return value if isinstance(value, str) else ""


def stop_reason(payload: dict[str, Any]) -> str:
    value = payload.get("reason") or ""
    return value if isinstance(value, str) else ""


def session_id(payload: dict[str, Any]) -> str:
    value = payload.get("session_id") or payload.get("sessionId") or ""
    return value if isinstance(value, str) else ""
