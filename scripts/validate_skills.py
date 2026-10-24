#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from arc_core.skill_validation import run_validation  # noqa: E402
from arc_core.usecase.validate_skills import (  # noqa: E402
    collect_library_members,
    collect_source_skill_files,
    collect_vertical_skill_files,
)
from arc_core.infrastructure.markdown import parse_frontmatter  # noqa: E402


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
    members = collect_library_members(ROOT)
    bundled = sum(
        1 for path in collect_vertical_skill_files(ROOT)
        if parse_frontmatter(path.read_text(encoding="utf-8"))[0].get("name") in members
    )
    runtime = count - source_count - bundled
    print(f"Skill validation passed for {count} skill files: {runtime} discoverable entrypoints, "
          f"{bundled} in-repo library entries, {source_count} source distribution.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
