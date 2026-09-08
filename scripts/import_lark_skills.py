#!/usr/bin/env python3
"""Import named Markdown skill packages embedded in the installed lark-cli.

Only offline `skills list/read` commands are used. Existing machine resources
and legacy references are retained because other skills can still link to them.
"""
from __future__ import annotations

import argparse
from collections import deque
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def cli_json(*args: str) -> dict:
    result = subprocess.run(["lark-cli", "skills", *args], check=True, capture_output=True, text=True)
    return json.loads(result.stdout)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("names", nargs="+")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    version = subprocess.run(["lark-cli", "--version"], check=True, capture_output=True, text=True).stdout.strip()
    catalog = {entry["name"] for entry in cli_json("list")["skills"]}
    for name in args.names:
        if not re.fullmatch(r"lark-[a-z0-9-]+", name) or name not in catalog:
            raise ValueError(f"unknown embedded skill: {name}")

    pending = deque(args.names)
    files: dict[str, str] = {}
    while pending:
        directory = pending.popleft()
        for entry in cli_json("list", directory)["entries"]:
            path = PurePosixPath(entry["path"])
            if path.is_absolute() or ".." in path.parts or path.parts[0] not in args.names:
                raise ValueError(f"unexpected embedded path: {path}")
            if entry["is_dir"]:
                pending.append(str(path))
            else:
                document = cli_json("read", str(path), "--json")
                content = document["content"]
                if path.name == "SKILL.md" and not content.startswith("---\n"):
                    raise ValueError(f"{path}: embedded entry has no frontmatter")
                files[str(path)] = content

    if args.dry_run:
        print(f"Verified {len(files)} embedded files from {version}; source tree unchanged")
        return 0
    lock_path = ROOT / "Lark" / ".lark-skills-source.json"
    lock = json.loads(lock_path.read_text(encoding="utf-8")) if lock_path.exists() else {"files": {}}
    for path, content in sorted(files.items()):
        target = ROOT / "Lark" / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")
        lock["files"][path] = {"cli_version": version, "sha256": hashlib.sha256(content.encode()).hexdigest()}
    lock["source"] = "https://github.com/larksuite/cli (embedded skills)"
    lock_path.write_text(json.dumps(lock, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"Imported {len(files)} files from {version}: {', '.join(args.names)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
