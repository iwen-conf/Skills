"""Run generated tests and produce a test-report.md with evidence.

Usage:
    python3 Arc/arc-test/scripts/validate_tests.py \\
        --project-path <path> \\
        --test-files <file1,file2> \\
        [--report-out test-report.md] [--timeout 120]
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
from detect_lang import detect_language  # noqa: E402

LANG_RUN_COMMANDS: dict[str, str] = {
    "go": "go test -v -count=1 {files}",
    "rust": "cargo test -- --nocapture",
    "python": "pytest -v {files}",
    "typescript": "npx vitest run {files}",
    "swift": "swift test",
}


def _run_tests(
    project_path: Path, lang: str, test_files: list[str], timeout: int
) -> tuple[int, str, str]:
    """Execute tests and return (exit_code, stdout, stderr)."""
    cmd_template = LANG_RUN_COMMANDS.get(lang, "")
    if not cmd_template:
        return 1, "", f"No test command for language: {lang}"

    files_str = " ".join(test_files)
    # Go uses package paths, not file paths directly for multiple files
    if lang == "go":
        # Derive unique package dirs from test files
        pkg_dirs: set[str] = set()
        for f in test_files:
            p = Path(f)
            pkg_dir = str(p.parent) if p.parent != Path(".") else "."
            pkg_dirs.add(f"./{pkg_dir}/...")
        files_str = " ".join(sorted(pkg_dirs)) if pkg_dirs else "./..."

    cmd = cmd_template.format(files=files_str)

    try:
        result = subprocess.run(
            cmd,
            shell=True,
            cwd=str(project_path),
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return 124, "", f"Test execution timed out after {timeout}s"
    except FileNotFoundError as exc:
        return 1, "", f"Command not found: {exc}"


def _generate_report(
    lang: str,
    framework: str,
    test_files: list[str],
    exit_code: int,
    stdout: str,
    stderr: str,
    duration_hint: str,
) -> str:
    """Generate test-report.md content."""
    status = "PASS" if exit_code == 0 else "FAIL"
    now = datetime.now(tz=timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

    lines = [
        "# Test Report",
        "",
        "## Metadata",
        "",
        f"- **Generated**: {now}",
        f"- **Language**: {lang}",
        f"- **Framework**: {framework}",
        f"- **Result**: {status}",
        f"- **Exit Code**: {exit_code}",
        f"- **Duration**: {duration_hint}",
        "",
        "## Files Tested",
        "",
    ]
    for f in test_files:
        lines.append(f"- `{f}`")
    lines.extend([
        "",
        "## Output",
        "",
        "```",
        stdout[:4000] if stdout else "(no stdout)",
        "```",
        "",
    ])
    if stderr.strip():
        lines.extend([
            "## Errors",
            "",
            "```",
            stderr[:2000],
            "```",
            "",
        ])
    return "\n".join(lines)


def validate(
    project_path: Path,
    test_files: list[str],
    report_out: str = "test-report.md",
    timeout: int = 120,
) -> int:
    """Run tests and write report. Returns exit code."""
    root = Path(project_path).resolve()
    info = detect_language(root)
    lang = str(info["lang"])
    framework = str(info["framework"])

    if lang == "unknown":
        print("ERROR: Could not detect project language", file=sys.stderr)
        return 1

    import time

    start = time.monotonic()
    exit_code, stdout, stderr = _run_tests(root, lang, test_files, timeout)
    elapsed = time.monotonic() - start
    duration_hint = f"{elapsed:.1f}s"

    report = _generate_report(lang, framework, test_files, exit_code, stdout, stderr, duration_hint)

    report_path = root / report_out
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(report, encoding="utf-8")
    print(f"Report written to: {report_path}")

    if exit_code == 0:
        print(f"ALL TESTS PASSED ({len(test_files)} file(s))")
    else:
        print(f"TESTS FAILED (exit code {exit_code})", file=sys.stderr)

    return exit_code


def main() -> None:
    parser = argparse.ArgumentParser(description="Validate generated tests and produce report")
    parser.add_argument("--project-path", required=True, help="Root path of the target project")
    parser.add_argument("--test-files", required=True, help="Comma-separated test file paths (relative to project)")
    parser.add_argument("--report-out", default="test-report.md", help="Report output path (default: test-report.md)")
    parser.add_argument("--timeout", type=int, default=120, help="Test execution timeout in seconds (default: 120)")
    args = parser.parse_args()

    test_files = [f.strip() for f in args.test_files.split(",") if f.strip()]
    exit_code = validate(
        project_path=Path(args.project_path),
        test_files=test_files,
        report_out=args.report_out,
        timeout=args.timeout,
    )
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
