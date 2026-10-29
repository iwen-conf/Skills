from __future__ import annotations

import json
import re
import shlex
import subprocess
from pathlib import Path
from typing import Any

SHELL_TOOLS = frozenset(
    {
        "bash",
        "run_terminal_command",
        "run_terminal_cmd",
        "run_shell_command",
        "shell",
        "powershell",
        "local_shell",
        "terminal",
        "terminal_exec",
        "execute_command",
        "exec_command",
    }
)
MUTATE_TOOLS = frozenset(
    {
        "write",
        "edit",
        "multiedit",
        "notebookedit",
        "apply_patch",
        "applypatch",
        "search_replace",
        "hashline_edit",
        "editnotebook",
        "str_replace",
        "strreplace",
        "delete",
    }
)

ASSIGNMENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")


def tool_kind(name: str) -> str:
    key = (name or "").strip().casefold()
    if key in SHELL_TOOLS:
        return "shell"
    if key in MUTATE_TOOLS:
        return "mutate"
    return "other"


def command_text(tool_input: dict[str, Any] | None) -> str:
    if not isinstance(tool_input, dict):
        return ""
    for key in ("command", "cmd", "command_line"):
        value = tool_input.get(key)
        if isinstance(value, str):
            return value
    return ""


def looks_like_completion(message: str, markers: list[str]) -> bool:
    if not message.strip():
        return False
    request = {
        "state": {"assistant_message": message, "completion_examples": markers},
        "questions": {
            "completion": {
                "type": "noul",
                "instructions": "Does assistant_message positively assert that the requested work is completed, fixed, or ready for delivery/commit? Judge the speaker's actual assertion, not quoted examples or individual keywords.",
                "criteria": {
                    "true": "The speaker claims the requested work is finished or ready for delivery.",
                    "false": "The speaker denies readiness, explains a gate, describes remaining work, or asks for missing information.",
                },
            }
        },
    }
    result = subprocess.run(
        ["jev", "ask"],
        input=json.dumps(request),
        capture_output=True,
        text=True,
        timeout=5,
        check=False,
    )
    if result.returncode:
        raise RuntimeError(
            f"TypeSafe completion judgment failed (jev exit {result.returncode}); verification remains pending"
        )
    answer = json.loads(result.stdout)["answers"]["completion"]
    if not isinstance(answer, dict):
        raise ValueError("TypeSafe returned an invalid completion answer")
    probability = answer.get("noul")
    if (
        answer.get("type") != "noul"
        or not isinstance(probability, (int, float))
        or isinstance(probability, bool)
        or not 0 <= probability <= 1
    ):
        raise ValueError("TypeSafe returned an invalid completion Noul")
    if 0.1 < probability < 0.9:
        raise RuntimeError(
            f"TypeSafe completion judgment is uncertain (noul={probability}); verification remains pending"
        )
    return probability >= 0.9


def mentions_completion(message: str, markers: list[str]) -> bool:
    """Marker match used when TypeSafe cannot judge; a negated marker still counts."""
    text = message.casefold()
    for marker in markers:
        word = marker.strip().casefold()
        if not word:
            continue
        if word.isascii():
            if re.search(rf"(?<![\w-]){re.escape(word)}(?![\w-])", text):
                return True
        elif word in text:
            return True
    return False


_MAX_NESTING = 8
SHELLS = frozenset({"sh", "bash", "zsh", "dash", "ksh"})
# Prefix commands that run the rest of their argv: options taking a value,
# operands before the command, and options that change directory.
_WRAPPERS: dict[str, tuple[frozenset[str], int, frozenset[str]]] = {
    "command": (frozenset(), 0, frozenset()),
    "env": (
        frozenset({"-u", "--unset", "-C", "--chdir"}),
        0,
        frozenset({"-C", "--chdir"}),
    ),
    "exec": (frozenset({"-a"}), 0, frozenset()),
    "nice": (frozenset({"-n"}), 0, frozenset()),
    "nohup": (frozenset(), 0, frozenset()),
    "sudo": (
        frozenset(
            {"-u", "-g", "-h", "-p", "-U", "-r", "-t", "-C", "-T", "-D", "--chdir"}
        ),
        0,
        frozenset({"-D", "--chdir"}),
    ),
    "time": (frozenset({"-f", "-o"}), 0, frozenset()),
    "timeout": (frozenset({"-s", "-k", "--signal", "--kill-after"}), 1, frozenset()),
    "xargs": (
        frozenset({"-I", "-L", "-n", "-P", "-s", "-E", "-d", "-a"}),
        0,
        frozenset(),
    ),
}


def shell_commands(
    command: str, *, require_known_cwd: bool = False, _depth: int = 0
) -> list[list[str]]:
    """Return every argv the command runs, including nested scripts.

    Heredoc bodies are data. Command substitutions, unquoted heredoc substitutions,
    `sh -c` and `eval` scripts are parsed as commands. Scripts run by other programs
    (make, npm run, python) stay opaque; the Git hook and CI cover those.
    """
    if "\0" in command:
        raise ValueError("shell command contains a NUL byte")
    if _depth > _MAX_NESTING:
        raise ValueError("shell command nests too deeply to verify")
    separated: list[str] = []
    nested: list[str] = []
    heredocs: list[tuple[str, bool, bool]] = []
    quote = ""
    escaped = False
    comment = False
    word_start = True
    index = 0
    while index < len(command):
        char = command[index]
        if comment:
            if char == "\n":
                comment = False
                separated.append(" \0 ")
                word_start = True
                index = _heredoc_bodies(command, index + 1, heredocs, nested)
                continue
        elif escaped:
            if char == "\n":
                separated.pop()
            else:
                separated.append(char)
            escaped = False
        elif char == "\\" and quote != "'":
            separated.append(char)
            escaped = True
            word_start = False
        elif quote != "'" and (char == "`" or command.startswith("$(", index)):
            end = _substitution_end(command, index)
            if not command.startswith("$((", index):
                nested.append(command[index + (1 if char == "`" else 2) : end])
            index = end + 1
            word_start = False
            continue
        elif quote:
            separated.append(char)
            if char == quote:
                quote = ""
        elif char in {"'", '"'}:
            separated.append(char)
            quote = char
            word_start = False
        elif char == "#" and word_start:
            comment = True
        elif command.startswith("<<", index) and not command.startswith("<<<", index):
            delimiter, strip, expands, index = _heredoc_delimiter(command, index + 2)
            heredocs.append((delimiter, strip, expands))
            word_start = False
            continue
        elif char in ";&|()\n":
            if require_known_cwd and char in "()":
                raise ValueError(
                    "cannot verify repository scope for grouped shell commands; run the controlled operation separately with git -C"
                )
            separated.append(" \0 ")
            word_start = True
            if char == "\n":
                index = _heredoc_bodies(command, index + 1, heredocs, nested)
                continue
        else:
            separated.append(char)
            word_start = char.isspace()
        index += 1
    commands: list[list[str]] = []
    current: list[str] = []
    for token in shlex.split("".join(separated)):
        if token == "\0":
            if current:
                commands.append(current)
                current = []
        else:
            current.append(token)
    if current:
        commands.append(current)
    result: list[list[str]] = []
    for argv in commands:
        argv = _unwrap(argv, require_known_cwd=require_known_cwd)
        result.append(argv)
        script = _shell_script(argv)
        if script is not None:
            nested.append(script)
    for script in nested:
        result.extend(
            shell_commands(
                script, require_known_cwd=require_known_cwd, _depth=_depth + 1
            )
        )
    return result


def _substitution_end(text: str, index: int) -> int:
    """Index of the character closing the `$(` or backtick substitution at index."""
    if text[index] == "`":
        cursor = index + 1
        while cursor < len(text):
            if text[cursor] == "\\":
                cursor += 2
                continue
            if text[cursor] == "`":
                return cursor
            cursor += 1
        raise ValueError("command substitution has no closing backtick")
    depth = 0
    quote = ""
    cursor = index + 1
    while cursor < len(text):
        char = text[cursor]
        if char == "\\" and quote != "'":
            cursor += 2
            continue
        if quote:
            if char == quote:
                quote = ""
        elif char in {"'", '"'}:
            quote = char
        elif char == "(":
            depth += 1
        elif char == ")":
            depth -= 1
            if depth == 0:
                return cursor
        cursor += 1
    raise ValueError("command substitution has no closing parenthesis")


def _substitutions(text: str) -> list[str]:
    """Scripts of the command substitutions in an expanding heredoc body."""
    scripts: list[str] = []
    index = 0
    while index < len(text):
        char = text[index]
        if char == "\\":
            index += 2
            continue
        if char == "`" or text.startswith("$(", index):
            end = _substitution_end(text, index)
            if not text.startswith("$((", index):
                scripts.append(text[index + (1 if char == "`" else 2) : end])
            index = end + 1
            continue
        index += 1
    return scripts


def _heredoc_delimiter(text: str, index: int) -> tuple[str, bool, bool, int]:
    """Parse the word after `<<`: (delimiter, strip tabs, body expands, next index)."""
    strip = text.startswith("-", index)
    index += strip
    while index < len(text) and text[index] in " \t":
        index += 1
    word: list[str] = []
    quoted = False
    while (
        index < len(text) and not text[index].isspace() and text[index] not in ";&|()<>"
    ):
        char = text[index]
        if char in {"'", '"'}:
            end = text.find(char, index + 1)
            if end < 0:
                raise ValueError("heredoc delimiter has an unclosed quote")
            word.append(text[index + 1 : end])
            quoted = True
            index = end + 1
        elif char == "\\" and index + 1 < len(text):
            word.append(text[index + 1])
            quoted = True
            index += 2
        else:
            word.append(char)
            index += 1
    if not word:
        raise ValueError("heredoc has no delimiter")
    return "".join(word), strip, not quoted, index


def _heredoc_bodies(
    text: str,
    index: int,
    heredocs: list[tuple[str, bool, bool]],
    nested: list[str],
) -> int:
    """Skip the bodies of pending heredocs starting at index; return the next index."""
    for delimiter, strip, expands in heredocs:
        lines: list[str] = []
        while index < len(text):
            end = text.find("\n", index)
            end = len(text) if end < 0 else end
            line = text[index:end]
            index = end + 1
            if (line.lstrip("\t") if strip else line) == delimiter:
                break
            lines.append(line)
        if expands:
            nested.extend(_substitutions("\n".join(lines)))
    heredocs.clear()
    return min(index, len(text))


def _shell_script(argv: list[str]) -> str | None:
    """The script a shell runs via -c, or the words eval runs."""
    if not argv:
        return None
    name = Path(argv[0]).name
    if name == "eval":
        return " ".join(argv[1:])
    if name not in SHELLS:
        return None
    has_c = False
    index = 1
    while index < len(argv):
        arg = argv[index]
        if arg in {"-o", "+o", "-O", "+O", "--rcfile", "--init-file"}:
            index += 2
            continue
        if arg == "--":
            index += 1
            break
        if not arg.startswith(("-", "+")) or arg in {"-", "+"}:
            break
        if not arg.startswith("--") and "c" in arg[1:]:
            has_c = True
        index += 1
    return argv[index] if has_c and index < len(argv) else None


def _changes_directory(option: str, chdir: frozenset[str]) -> bool:
    return option in chdir or any(
        option.startswith(name + "=" if name.startswith("--") else name)
        for name in chdir
    )


def _unwrap(argv: list[str], *, require_known_cwd: bool = False) -> list[str]:
    argv = list(argv)
    while argv:
        if ASSIGNMENT.match(argv[0]):
            argv.pop(0)
            continue
        name = Path(argv[0]).name
        spec = _WRAPPERS.get(name)
        if spec is None:
            break
        values, operands, chdir = spec
        argv.pop(0)
        while argv and argv[0].startswith("-") and argv[0] != "-":
            option = argv.pop(0)
            if option == "--":
                break
            if require_known_cwd and _changes_directory(option, chdir):
                raise ValueError(
                    f"cannot verify repository scope for {name} directory changes; run the controlled operation separately with git -C"
                )
            if option in values and argv:
                argv.pop(0)
        del argv[:operands]
    if require_known_cwd and argv and argv[0] == "cd":
        raise ValueError(
            "cannot verify repository scope after shell cd; run the controlled operation separately with git -C"
        )
    return argv


def git_command(argv: list[str]) -> tuple[list[str], list[str]] | None:
    if not argv or Path(argv[0]).name != "git":
        return None
    index = 1
    while index < len(argv) and argv[index].startswith("-"):
        option = argv[index]
        index += 1
        if option in {
            "-C",
            "-c",
            "--git-dir",
            "--work-tree",
            "--namespace",
            "--config-env",
        }:
            index += 1
    return argv[1:index], argv[index:]


def _skips_hooks(args: list[str]) -> bool:
    index = 1
    while index < len(args):
        arg = args[index]
        if arg == "--":
            break
        if arg in {
            "-m",
            "--message",
            "-F",
            "--file",
            "--author",
            "--date",
            "--fixup",
            "--squash",
            "--trailer",
        }:
            index += 2
            continue
        if arg == "--no-verify" or (
            args[0] == "commit"
            and arg.startswith("-")
            and not arg.startswith("--")
            and "n" in arg[1:].split("m", 1)[0].split("F", 1)[0]
        ):
            return True
        index += 1
    return False


def classify(
    *,
    tool_name: str,
    tool_input: dict[str, Any] | None,
    policy: dict[str, Any],
    has_task: bool,
    receipt_ok: bool,
) -> dict[str, Any]:
    """Return {action: allow|deny, reason, operation}."""
    kind = tool_kind(tool_name)
    command = command_text(tool_input)
    deny_direct = set((policy.get("execution") or {}).get("deny_direct") or [])
    workflow = policy.get("workflow") or {}
    require_begin = bool(workflow.get("require_begin"))

    if kind == "shell":
        commands = shell_commands(command)
        for argv in commands:
            if not argv:
                continue
            parsed = git_command(argv)
            args = parsed[1] if parsed else []
            operation = args[0] if args else ""
            if operation in {"commit", "push"}:
                skip = _skips_hooks(args)
                if "skip_hooks" in deny_direct and skip:
                    return {
                        "action": "deny",
                        "reason": "git --no-verify/-n is denied; run arc-policy task verify without skipping hooks.",
                        "operation": "skip_hooks",
                    }
                if "unverified_commit" in deny_direct and not receipt_ok:
                    return {
                        "action": "deny",
                        "reason": "git commit/push is a controlled operation. Stage the commit, then run "
                        "`arc-policy task verify` in its own call. If the user explicitly confirmed committing "
                        "without verification, run `arc-policy task authorize --quote '<their words>'` instead.",
                        "operation": "commit",
                    }
            executable = Path(argv[0]).name
            migration = (
                executable in {"alembic", "goose", "sql-migrate", "flyway", "liquibase"}
                or (
                    executable in {"diesel", "prisma"}
                    and len(argv) > 1
                    and argv[1] in {"migration", "migrate"}
                )
                or (
                    executable == "psql"
                    and any(
                        arg in {"-f", "--file"} or arg.startswith("--file=")
                        for arg in argv[1:]
                    )
                )
            )
            if "schema_migration" in deny_direct and migration and not receipt_ok:
                return {
                    "action": "deny",
                    "reason": "schema migration must go through `arc-policy task verify` (or a project overlay that allows it).",
                    "operation": "schema_migration",
                }
        if commands and all(
            argv
            and (
                Path(argv[0]).name == "arc-policy"
                or argv[:3]
                in [["python", "-m", "arc_policy"], ["python3", "-m", "arc_policy"]]
            )
            for argv in commands
        ):
            return {"action": "allow", "reason": "arc-policy CLI", "operation": "cli"}
        return {
            "action": "allow",
            "reason": "shell not controlled",
            "operation": "shell",
        }

    if kind == "mutate" and require_begin and not has_task:
        return {
            "action": "deny",
            "reason": "no active Arc task. Run `arc-policy task begin --skill <arc-code|arc-explore|arc-plan|arc-verify|arc-audit|arc-vet>` before editing source.",
            "operation": "source_edit",
        }

    return {"action": "allow", "reason": "not controlled", "operation": kind}
