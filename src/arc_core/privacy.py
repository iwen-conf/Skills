from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Sequence, cast

PRIVATE_PATTERN = re.compile(
    r'<private(?:\s+reason="([^"]*)")?\s*>(.*?)</private>', re.DOTALL
)
PRIVATE_OPEN = re.compile(r'<private(?:\s+reason="([^"]*)")?\s*>')
PRIVATE_CLOSE = re.compile(r"</private>")


def has_private(text: str) -> bool:
    return bool(PRIVATE_OPEN.search(text))


def _redact_with_stack(text: str, mode: str = "mask") -> str:
    result: list[str] = []
    i = 0
    stack: list[tuple[int, str]] = []

    while i < len(text):
        open_match = PRIVATE_OPEN.match(text, i)
        if open_match:
            reason = open_match.group(1) or "redacted"
            stack.append((len(result), reason))
            i = open_match.end()
            continue

        close_match = PRIVATE_CLOSE.match(text, i)
        if close_match:
            if stack:
                start_pos, reason = stack.pop()
                if mode == "mask":
                    placeholder = f"[PRIVATE: {reason}]"
                    result = result[:start_pos]
                    result.append(placeholder)
            i = close_match.end()
            continue

        if not stack:
            result.append(text[i])
        i += 1

    return "".join(result)


def redact_private(text: str, mode: str = "mask") -> str:
    return _redact_with_stack(text, mode)


def strip_private(text: str) -> str:
    return redact_private(text, mode="strip")


def extract_private(text: str) -> tuple[str, list[dict[str, str]]]:
    segments: list[dict[str, str]] = []

    def extract_replacer(match: re.Match[str]) -> str:
        reason = match.group(1) or "redacted"
        content = match.group(2)
        segments.append({"reason": reason, "content": content})
        return f"[PRIVATE: {reason}]"

    redacted = PRIVATE_PATTERN.sub(extract_replacer, text)
    while PRIVATE_OPEN.search(redacted):
        redacted = PRIVATE_PATTERN.sub(extract_replacer, redacted)

    return redacted, segments


def process_file(input_path: Path, output_path: Path | None, mode: str) -> None:
    content = input_path.read_text(encoding="utf-8")
    redacted = redact_private(content, mode)

    if output_path:
        output_path.write_text(redacted, encoding="utf-8")
    else:
        print(redacted, end="")


def load_config(config_path: Path | None = None) -> dict[str, Any]:
    if config_path is None:
        config_path = Path.cwd() / ".arc" / "privacy.json"

    if config_path.exists():
        return json.loads(config_path.read_text(encoding="utf-8"))

    return {"enabled": True, "mode": "mask"}


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Filter <private> tags from content")
    parser.add_argument("--stdin", action="store_true", help="Read from stdin")
    parser.add_argument("--input", "-i", type=Path, help="Input file")
    parser.add_argument("--output", "-o", type=Path, help="Output file")
    parser.add_argument(
        "--mode",
        "-m",
        choices=["mask", "strip"],
        default="mask",
        help="mask: replace with [PRIVATE], strip: remove entirely",
    )
    parser.add_argument("--config", "-c", type=Path, help="Config file path")
    parser.add_argument(
        "--check", action="store_true", help="Exit 0 if no private content, 1 if found"
    )
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    config = load_config(args.config)
    mode = cast(str, args.mode or config.get("mode", "mask"))

    if args.check:
        if args.stdin:
            text = sys.stdin.read()
        elif args.input:
            text = args.input.read_text(encoding="utf-8")
        else:
            parser.error("Need --stdin or --input for --check")

        if has_private(text):
            print("Private content detected", file=sys.stderr)
            return 1

        print("No private content", file=sys.stderr)
        return 0

    if args.stdin:
        text = sys.stdin.read()
        result = redact_private(text, mode)
        if args.output:
            args.output.write_text(result, encoding="utf-8")
        else:
            print(result, end="")
        return 0

    if args.input:
        process_file(args.input, args.output, mode)
        return 0

    parser.error("Need --stdin or --input")
    return 2
