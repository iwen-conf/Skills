"""Detect project language, test framework, and conventions.

Usage:
    python3 Arc/arc-test/scripts/detect_lang.py --project-path <path> [--json]
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

LANG_MARKERS: list[dict[str, str | list[str]]] = [
    {
        "lang": "go",
        "marker": "go.mod",
        "framework": "testing",
        "test_command": "go test ./... -v",
        "bench_command": "go test -bench=. -benchmem",
        "test_file_pattern": "*_test.go",
        "test_dir": ".",
    },
    {
        "lang": "rust",
        "marker": "Cargo.toml",
        "framework": "cargo-test",
        "test_command": "cargo test -- --nocapture",
        "bench_command": "cargo bench",
        "test_file_pattern": "**/tests/**/*.rs",
        "test_dir": "tests",
    },
    {
        "lang": "python",
        "marker": "pyproject.toml",
        "framework": "pytest",
        "test_command": "pytest -v",
        "bench_command": "pytest --benchmark-only",
        "test_file_pattern": "test_*.py",
        "test_dir": "tests",
    },
    {
        "lang": "swift",
        "marker": "Package.swift",
        "framework": "xctest",
        "test_command": "swift test",
        "bench_command": "swift test",
        "test_file_pattern": "*Tests.swift",
        "test_dir": "Tests",
    },
]

TS_FRAMEWORKS = ["vitest", "jest"]


def _detect_ts_framework(project_path: Path) -> str:
    """Check package.json devDependencies for vitest or jest."""
    pkg_json = project_path / "package.json"
    if not pkg_json.exists():
        return "vitest"
    try:
        data = json.loads(pkg_json.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return "vitest"
    deps = {**data.get("devDependencies", {}), **data.get("dependencies", {})}
    for fw in TS_FRAMEWORKS:
        if fw in deps:
            return fw
    return "vitest"


def _find_existing_tests(project_path: Path, pattern: str) -> list[str]:
    """Glob for existing test files, return relative paths."""
    results: list[str] = []
    for p in sorted(project_path.rglob(pattern)):
        if any(part in p.parts for part in ("node_modules", ".git", ".venv", "venv", "__pycache__")):
            continue
        try:
            results.append(str(p.relative_to(project_path)))
        except ValueError:
            results.append(str(p))
    return results


def detect_language(project_path: Path) -> dict[str, object]:
    """Detect project language and return a structured result dict."""
    root = Path(project_path).resolve()

    for marker_def in LANG_MARKERS:
        marker_file = root / str(marker_def["marker"])
        if marker_file.exists():
            pattern = str(marker_def["test_file_pattern"])
            return {
                "lang": marker_def["lang"],
                "framework": marker_def["framework"],
                "test_command": marker_def["test_command"],
                "bench_command": marker_def["bench_command"],
                "test_file_pattern": pattern,
                "test_dir": marker_def["test_dir"],
                "existing_tests": _find_existing_tests(root, pattern),
                "config_files": [str(marker_def["marker"])],
            }

    # TypeScript / JavaScript (checked after others since package.json is common)
    pkg_json = root / "package.json"
    if pkg_json.exists():
        fw = _detect_ts_framework(root)
        test_cmd = "npx vitest run" if fw == "vitest" else "npx jest"
        bench_cmd = "npx vitest bench" if fw == "vitest" else "npx jest"
        pattern = "*.test.ts" if fw == "vitest" else "*.test.ts"
        return {
            "lang": "typescript",
            "framework": fw,
            "test_command": test_cmd,
            "bench_command": bench_cmd,
            "test_file_pattern": pattern,
            "test_dir": "src",
            "existing_tests": _find_existing_tests(root, f"**/{pattern}"),
            "config_files": ["package.json"],
        }

    return {
        "lang": "unknown",
        "framework": "unknown",
        "test_command": "",
        "bench_command": "",
        "test_file_pattern": "",
        "test_dir": ".",
        "existing_tests": [],
        "config_files": [],
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Detect project language and test framework")
    parser.add_argument("--project-path", required=True, help="Root path of the target project")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    args = parser.parse_args()

    result = detect_language(Path(args.project_path))

    if args.json:
        json.dump(result, sys.stdout, indent=2, ensure_ascii=False)
        sys.stdout.write("\n")
    else:
        for key, value in result.items():
            print(f"{key}: {value}")


if __name__ == "__main__":
    main()
