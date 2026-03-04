#!/usr/bin/env python3
"""
Privacy Tag Filter for Arc Skills

Filters <private>...</private> tags from content before storage/logging.

Usage:
    # CLI mode (stdin/stdout)
    echo "Hello <private>secret</private> World" | python arc_privacy.py --stdin --mode mask
    # Output: Hello [PRIVATE] World

    # File mode
    python arc_privacy.py --input file.md --output file_redacted.md --mode strip

    # As module
    from arc_privacy import redact_private, strip_private, has_private
    redacted = redact_private("Hello <private>secret</private>")  # "Hello [PRIVATE]"
    stripped = strip_private("Hello <private>secret</private>")   # "Hello "
    if has_private(text):
        print("Contains private content")
"""

import re
import sys
import json
import argparse
from pathlib import Path
from typing import Optional, Tuple

# Pattern for <private>...</private> with optional reason attribute
# Handles nested tags by using non-greedy match and iterative processing
PRIVATE_PATTERN = re.compile(
    r'<private(?:\s+reason="([^"]*)")?\s*>(.*?)</private>', re.DOTALL
)

# Alternative pattern for stack-based nested handling
PRIVATE_OPEN = re.compile(r'<private(?:\s+reason="([^"]*)")?\s*>')
PRIVATE_CLOSE = re.compile(r"</private>")


def has_private(text: str) -> bool:
    """Check if text contains <private> tags."""
    return bool(PRIVATE_OPEN.search(text))


def _redact_with_stack(text: str, mode: str = "mask") -> str:
    """
    Redact private content using stack-based approach for nested tags.

    Args:
        text: Input text containing <private> tags
        mode: 'mask' - replace with placeholder, 'strip' - remove entirely

    Returns:
        Text with private content redacted
    """
    result = []
    i = 0
    stack = []  # Stack of (start_pos, reason)

    while i < len(text):
        # Check for opening tag
        open_match = PRIVATE_OPEN.match(text, i)
        if open_match:
            reason = open_match.group(1) or "redacted"
            stack.append((len(result), reason))
            i = open_match.end()
            continue

        # Check for closing tag
        close_match = PRIVATE_CLOSE.match(text, i)
        if close_match:
            if stack:
                start_pos, reason = stack.pop()
                if mode == "mask":
                    # Replace with placeholder
                    placeholder = f"[PRIVATE: {reason}]"
                    # Truncate result to start_pos and add placeholder
                    result = result[:start_pos]
                    result.append(placeholder)
            i = close_match.end()
            continue

        # Regular character
        if not stack:  # Only append if not inside private block
            result.append(text[i])
        i += 1

    return "".join(result)


def redact_private(text: str, mode: str = "mask") -> str:
    """
    Redact private content marked with <private> tags.

    Args:
        text: Input text containing <private> tags
        mode: 'mask' - replace with [PRIVATE: reason] placeholder
              'strip' - remove entirely including tags

    Returns:
        Text with private content redacted
    """
    return _redact_with_stack(text, mode)


def strip_private(text: str) -> str:
    """
    Remove private content entirely (convenience function).
    Equivalent to redact_private(text, mode='strip').
    """
    return redact_private(text, mode="strip")


def extract_private(text: str) -> Tuple[str, list]:
    """
    Extract private content and return redacted text + list of private segments.

    Returns:
        (redacted_text, [private_segments])
    """
    segments = []

    def extract_replacer(match):
        reason = match.group(1) or "redacted"
        content = match.group(2)
        segments.append({"reason": reason, "content": content})
        return f"[PRIVATE: {reason}]"

    redacted = PRIVATE_PATTERN.sub(extract_replacer, text)
    # Handle nested tags iteratively
    while PRIVATE_OPEN.search(redacted):
        redacted = PRIVATE_PATTERN.sub(extract_replacer, redacted)

    return redacted, segments


def process_file(input_path: Path, output_path: Optional[Path], mode: str) -> None:
    """Process a single file."""
    content = input_path.read_text(encoding="utf-8")
    redacted = redact_private(content, mode)

    if output_path:
        output_path.write_text(redacted, encoding="utf-8")
    else:
        print(redacted, end="")


def load_config(config_path: Optional[Path] = None) -> dict:
    """Load privacy configuration from .arc/privacy.json."""
    if config_path is None:
        config_path = Path.cwd() / ".arc" / "privacy.json"

    if config_path.exists():
        return json.loads(config_path.read_text(encoding="utf-8"))

    return {"enabled": True, "mode": "mask"}


def main():
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

    args = parser.parse_args()

    # Load config
    config = load_config(args.config)
    mode = args.mode or config.get("mode", "mask")

    if args.check:
        # Check mode: just report if private content exists
        if args.stdin:
            text = sys.stdin.read()
        elif args.input:
            text = args.input.read_text(encoding="utf-8")
        else:
            parser.error("Need --stdin or --input for --check")

        if has_private(text):
            print("Private content detected", file=sys.stderr)
            sys.exit(1)
        else:
            print("No private content", file=sys.stderr)
            sys.exit(0)

    if args.stdin:
        # Stdin mode
        text = sys.stdin.read()
        result = redact_private(text, mode)
        if args.output:
            args.output.write_text(result, encoding="utf-8")
        else:
            print(result, end="")
    elif args.input:
        # File mode
        process_file(args.input, args.output, mode)
    else:
        parser.error("Need --stdin or --input")


if __name__ == "__main__":
    main()
