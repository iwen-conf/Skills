#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from arc_core.skill_validation import run_validation  # noqa: E402
from arc_core.usecase.validate_skills import collect_source_skill_files  # noqa: E402


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
    source_count = len(collect_source_skill_files(ROOT))
    print(f"Skill validation passed for {count} skill entrypoints "
          f"({count - source_count} runtime, {source_count} source distribution).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
