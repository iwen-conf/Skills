#!/usr/bin/env python3
"""Compatibility CLI wrapper for arc_core privacy helpers."""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from arc_core.privacy import (  # noqa: E402
    extract_private,
    has_private,
    load_config,
    main,
    process_file,
    redact_private,
    strip_private,
)

__all__ = [
    "extract_private",
    "has_private",
    "load_config",
    "main",
    "process_file",
    "redact_private",
    "strip_private",
]


if __name__ == "__main__":
    raise SystemExit(main())
