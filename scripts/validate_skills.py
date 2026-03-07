#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from arc_core.skill_validation import run_validation  # noqa: E402


def main() -> int:
    errors, warnings, count = run_validation(ROOT)
    if errors:
        print("Skill validation failed:")
        for item in errors:
            print(f"- {item}")
        if warnings:
            print("\nSkill validation warnings:")
            for item in warnings:
                print(f"- {item}")
        return 1
    if warnings:
        print("Skill validation warnings:")
        for item in warnings:
            print(f"- {item}")
    print(f"Skill validation passed for {count} files.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
