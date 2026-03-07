#!/usr/bin/env python3
"""Compatibility CLI wrapper for arc_core eval runner."""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from arc_core.eval_runner import (  # noqa: E402
    AssertionResult,
    AssertionRunner,
    EvalResult,
    EvalRunner,
    RunResult,
    build_parser,
    main,
)

__all__ = [
    "AssertionResult",
    "AssertionRunner",
    "EvalResult",
    "EvalRunner",
    "RunResult",
    "build_parser",
    "main",
]


if __name__ == "__main__":
    raise SystemExit(main())
