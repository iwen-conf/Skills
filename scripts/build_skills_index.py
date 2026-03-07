#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from arc_core.skill_registry import write_registry_and_context  # noqa: E402


def main() -> int:
    registry_path, index_path, manifest_path = write_registry_and_context(ROOT)
    print(str(registry_path))
    print(str(index_path))
    print(str(manifest_path))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
