#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


def _fail(msg: str, *, code: int = 2) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(code)


def _strip_jsonc(text: str) -> str:
    out: list[str] = []
    i = 0
    in_string = False
    escape = False
    in_line_comment = False
    in_block_comment = False

    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""

        if in_line_comment:
            if ch in "\r\n":
                in_line_comment = False
                out.append(ch)
            i += 1
            continue

        if in_block_comment:
            if ch == "*" and nxt == "/":
                in_block_comment = False
                i += 2
            else:
                i += 1
            continue

        if in_string:
            out.append(ch)
            if escape:
                escape = False
            else:
                if ch == "\\":
                    escape = True
                elif ch == '"':
                    in_string = False
            i += 1
            continue

        if ch == '"':
            in_string = True
            out.append(ch)
            i += 1
            continue

        if ch == "/" and nxt == "/":
            in_line_comment = True
            i += 2
            continue

        if ch == "/" and nxt == "*":
            in_block_comment = True
            i += 2
            continue

        out.append(ch)
        i += 1

    return "".join(out)


def _load_accounts(path: Path) -> dict[str, Any]:
    try:
        text = path.read_text(encoding="utf-8")
    except FileNotFoundError:
        _fail(f"accounts file not found: {path}")
    try:
        raw = json.loads(_strip_jsonc(text))
    except json.JSONDecodeError as e:
        _fail(f"invalid JSONC in {path}: {e}")
    if not isinstance(raw, dict):
        _fail("accounts file root must be an object")
    return raw


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="accounts_to_personas.py",
        description="Convert ui-ux-simulation accounts.jsonc into personas JSON (role/user/pass/token).",
    )
    parser.add_argument("--accounts-file", required=True, help="Path to accounts.jsonc (JSONC allowed).")
    parser.add_argument("--role", action="append", default=[], help="Filter to a role (repeatable).")
    parser.add_argument("--out", help="Write personas JSON to this file (default: stdout).")
    args = parser.parse_args()

    accounts_path = Path(args.accounts_file).expanduser()
    raw = _load_accounts(accounts_path)
    accounts = raw.get("accounts")
    if not isinstance(accounts, list):
        _fail("accounts file must contain an 'accounts' array")

    role_filter = {r for r in args.role if r} if args.role else None

    personas: list[dict[str, str]] = []
    for a in accounts:
        if not isinstance(a, dict):
            continue
        role = str(a.get("role", "")).strip()
        if role_filter and role not in role_filter:
            continue
        personas.append(
            {
                "role": role or "role",
                "user": str(a.get("username", "")),
                "pass": str(a.get("password", "")),
                "token": str(a.get("token", "")),
            }
        )

    out_text = json.dumps(personas, indent=2, ensure_ascii=False) + "\n"
    if args.out:
        out_path = Path(args.out).expanduser()
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(out_text, encoding="utf-8")
    else:
        print(out_text, end="")


if __name__ == "__main__":
    main()

