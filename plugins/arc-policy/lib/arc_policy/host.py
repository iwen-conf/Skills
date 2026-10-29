from __future__ import annotations

import hashlib
import json
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
    "post_tool_use": "PostToolUse",
    "posttooluse": "PostToolUse",
    "posttoolusefailure": "PostToolUse",
    "aftershellexecution": "PostToolUse",
    "after_shell_execution": "PostToolUse",
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
            if value in {
                "Stop",
                "PreToolUse",
                "PostToolUse",
                "SessionStart",
                "UserPromptSubmit",
            }:
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


def call_key(payload: dict[str, Any]) -> str:
    """Pairs one tool call's pre and post events: the host's call id, else the call's input."""
    value = payload.get("tool_use_id") or payload.get("toolUseId")
    if isinstance(value, str) and value:
        return "id:" + value
    text = json.dumps(tool_input(payload), sort_keys=True, ensure_ascii=True)
    return "input:" + hashlib.sha256(text.encode()).hexdigest()


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


def transcript_path(payload: dict[str, Any]) -> Path | None:
    value = payload.get("transcript_path") or payload.get("transcriptPath")
    return Path(value).expanduser() if isinstance(value, str) and value else None


# Host-injected wrappers and pasted material are not words the user typed.
_NOT_TYPED = re.compile(
    r"<(system-reminder|local-command-caveat|local-command-stdout|command-name|command-message"
    r"|command-args|pasted_content)\b[^>]*>.*?</\1\b[^>]*>",
    re.DOTALL,
)


def _typed_text(entry: dict[str, Any]) -> str:
    # Claude Code: {"type": "user", "message": {"content": str | [blocks]}}; tool results are blocks too.
    if entry.get("type") == "user" and not entry.get("isMeta") and not entry.get("isSidechain"):
        content = (entry.get("message") or {}).get("content")
        if isinstance(content, str):
            parts = [content]
        elif isinstance(content, list):
            parts = [
                block["text"]
                for block in content
                if isinstance(block, dict)
                and block.get("type") == "text"
                and isinstance(block.get("text"), str)
            ]
        else:
            parts = []
        text = _NOT_TYPED.sub("", "\n".join(parts)).strip()
        return "" if text.startswith("[Request interrupted") else text
    # Codex: {"type": "event_msg", "payload": {"type": "user_message", "message": str}}
    payload = entry.get("payload")
    if (
        entry.get("type") == "event_msg"
        and isinstance(payload, dict)
        and payload.get("type") == "user_message"
        and isinstance(payload.get("message"), str)
    ):
        return _NOT_TYPED.sub("", payload["message"]).strip()
    return ""


def recent_user_messages(path: Path, limit: int) -> list[str]:
    """The last `limit` messages the user typed, read from a Claude Code or Codex transcript."""
    messages = []
    with path.open(encoding="utf-8", errors="replace") as stream:
        for line in stream:
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            text = _typed_text(entry) if isinstance(entry, dict) else ""
            if text:
                messages.append(text)
    return messages[-limit:]


def stop_reason(payload: dict[str, Any]) -> str:
    value = payload.get("reason") or ""
    return value if isinstance(value, str) else ""


def session_id(payload: dict[str, Any]) -> str:
    value = (
        payload.get("session_id")
        or payload.get("sessionId")
        or payload.get("conversation_id")
        or ""
    )
    return value if isinstance(value, str) else ""
