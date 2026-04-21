"""Scaffold test file skeletons from language-specific templates.

Usage:
    python3 Arc/arc:test/scripts/scaffold_tests.py \\
        --project-path <path> \\
        --target <file_or_dir> \\
        --test-types unit,boundary,benchmark \\
        [--lang auto] [--output-dir <dir>] [--force] [--dry-run]
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
TEMPLATES_DIR = SCRIPT_DIR.parent / "templates"

# Import detect_lang from sibling script
sys.path.insert(0, str(SCRIPT_DIR))
from detect_lang import detect_language  # noqa: E402

TEMPLATE_MAP: dict[str, dict[str, str]] = {
    "go": {
        "unit": "go/unit_test.go.tpl",
        "boundary": "go/table_test.go.tpl",
        "table": "go/table_test.go.tpl",
        "benchmark": "go/benchmark_test.go.tpl",
        "fuzz": "go/fuzz_test.go.tpl",
        "integration": "go/integration_test.go.tpl",
    },
    "rust": {
        "unit": "rust/unit_test.rs.tpl",
        "boundary": "rust/unit_test.rs.tpl",
        "benchmark": "rust/bench_test.rs.tpl",
        "integration": "rust/integration_test.rs.tpl",
    },
    "python": {
        "unit": "python/test_unit.py.tpl",
        "boundary": "python/test_parametrize.py.tpl",
        "table": "python/test_parametrize.py.tpl",
        "benchmark": "python/test_parametrize.py.tpl",
        "integration": "python/test_integration.py.tpl",
    },
    "javascript": {
        "unit": "javascript/unit.test.js.tpl",
        "boundary": "javascript/unit.test.js.tpl",
        "benchmark": "javascript/unit.test.js.tpl",
        "integration": "javascript/integration.test.js.tpl",
    },
    "swift": {
        "unit": "swift/UnitTests.swift.tpl",
        "boundary": "swift/UnitTests.swift.tpl",
        "benchmark": "swift/UnitTests.swift.tpl",
        "integration": "swift/IntegrationTests.swift.tpl",
    },
}

OUTPUT_NAMING: dict[str, dict[str, str]] = {
    "go": {
        "unit": "{stem}_test.go",
        "boundary": "{stem}_boundary_test.go",
        "table": "{stem}_table_test.go",
        "benchmark": "{stem}_benchmark_test.go",
        "fuzz": "{stem}_fuzz_test.go",
        "integration": "{stem}_integration_test.go",
    },
    "rust": {
        "unit": "{stem}_test.rs",
        "boundary": "{stem}_boundary_test.rs",
        "benchmark": "bench_{stem}.rs",
        "integration": "integration_{stem}.rs",
    },
    "python": {
        "unit": "test_{stem}.py",
        "boundary": "test_{stem}_parametrize.py",
        "table": "test_{stem}_parametrize.py",
        "benchmark": "test_{stem}_benchmark.py",
        "integration": "test_{stem}_integration.py",
    },
    "javascript": {
        "unit": "{stem}.test.js",
        "boundary": "{stem}.boundary.test.js",
        "benchmark": "{stem}.bench.js",
        "integration": "{stem}.integration.test.js",
    },
    "swift": {
        "unit": "{stem}Tests.swift",
        "boundary": "{stem}BoundaryTests.swift",
        "benchmark": "{stem}BenchmarkTests.swift",
        "integration": "{stem}IntegrationTests.swift",
    },
}


def _infer_placeholders(target: Path, lang: str) -> dict[str, str]:
    """Build placeholder dict from target file metadata."""
    stem = target.stem
    return {
        "PACKAGE_NAME": stem,
        "MODULE_NAME": stem,
        "MODULE_PATH": stem,
        "FUNCTION_NAME": f"Example{stem.capitalize()}" if lang != "python" else f"example_{stem}",
        "FUNCTION_NAME_PASCAL": stem.capitalize(),
        "CLASS_NAME": stem.capitalize(),
        "CRATE_NAME": stem,
        "SAMPLE_INPUT": '""' if lang == "go" else '""',
        "EXPECTED_OUTPUT": '""',
        "INPUT_TYPE": "string" if lang == "go" else "str",
        "OUTPUT_TYPE": "string" if lang == "go" else "str",
        "ZERO_VALUE": '""',
        "ZERO_EXPECTED": '""',
        "NOMINAL_VALUE": '"hello"',
        "NOMINAL_EXPECTED": '"hello"',
        "MIN_VALUE": '""',
        "MIN_EXPECTED": '""',
        "MAX_VALUE": '"x" * 1000' if lang == "python" else '"long_string"',
        "MAX_EXPECTED": '""',
        "INVALID_INPUT": '""',
        "SEED_CORPUS": '""',
        "FUZZ_PARAMS": "s string",
        "FUZZ_ARGS": "s",
    }


def _render_template(template_path: Path, placeholders: dict[str, str]) -> str:
    """Read template and replace {{KEY}} placeholders."""
    content = template_path.read_text(encoding="utf-8")
    for key, value in placeholders.items():
        content = content.replace(f"{{{{{key}}}}}", value)
    return content


def _resolve_output_dir(project_path: Path, lang: str, output_dir: str | None, target: Path) -> Path:
    """Determine where test files should be placed."""
    if output_dir:
        return Path(output_dir)
    if lang == "go":
        return target.parent
    if lang == "rust":
        return project_path / "tests"
    if lang == "python":
        return project_path / "tests"
    if lang == "javascript":
        return target.parent / "__tests__"
    if lang == "swift":
        return project_path / "Tests"
    return project_path


def scaffold(
    project_path: Path,
    target: Path,
    test_types: list[str],
    lang: str | None = None,
    output_dir: str | None = None,
    force: bool = False,
    dry_run: bool = False,
) -> list[str]:
    """Generate test file skeletons. Returns list of created file paths."""
    root = Path(project_path).resolve()

    if lang and lang != "auto":
        detected_lang = lang
    else:
        info = detect_language(root)
        detected_lang = str(info["lang"])

    if detected_lang == "unknown":
        print("ERROR: Could not detect project language", file=sys.stderr)
        return []

    lang_templates = TEMPLATE_MAP.get(detected_lang, {})
    lang_naming = OUTPUT_NAMING.get(detected_lang, {})
    placeholders = _infer_placeholders(target, detected_lang)
    out_dir = _resolve_output_dir(root, detected_lang, output_dir, target)
    created: list[str] = []

    for test_type in test_types:
        tpl_rel = lang_templates.get(test_type)
        if not tpl_rel:
            print(f"WARN: No template for {detected_lang}/{test_type}, skipping", file=sys.stderr)
            continue

        tpl_path = TEMPLATES_DIR / tpl_rel
        if not tpl_path.exists():
            print(f"WARN: Template not found: {tpl_path}", file=sys.stderr)
            continue

        name_pattern = lang_naming.get(test_type, f"{target.stem}_{test_type}_test")
        out_name = name_pattern.format(stem=target.stem)
        out_path = out_dir / out_name

        if dry_run:
            print(f"[DRY-RUN] Would create: {out_path}")
            created.append(str(out_path))
            continue

        if out_path.exists() and not force:
            print(f"SKIP: {out_path} already exists (use --force to overwrite)", file=sys.stderr)
            continue

        out_path.parent.mkdir(parents=True, exist_ok=True)
        content = _render_template(tpl_path, placeholders)
        out_path.write_text(content, encoding="utf-8")
        print(f"CREATED: {out_path}")
        created.append(str(out_path))

    return created


def main() -> None:
    parser = argparse.ArgumentParser(description="Scaffold test file skeletons")
    parser.add_argument("--project-path", required=True, help="Root path of the target project")
    parser.add_argument("--target", required=True, help="File or directory to generate tests for")
    parser.add_argument("--test-types", default="unit,boundary", help="Comma-separated: unit,boundary,table,benchmark,fuzz,integration")
    parser.add_argument("--lang", default="auto", help="Language override (default: auto-detect)")
    parser.add_argument("--output-dir", default=None, help="Output directory for test files")
    parser.add_argument("--force", action="store_true", help="Overwrite existing test files")
    parser.add_argument("--dry-run", action="store_true", help="Print planned files without writing")
    args = parser.parse_args()

    target = Path(args.target)
    test_types = [t.strip() for t in args.test_types.split(",") if t.strip()]

    created = scaffold(
        project_path=Path(args.project_path),
        target=target,
        test_types=test_types,
        lang=args.lang if args.lang != "auto" else None,
        output_dir=args.output_dir,
        force=args.force,
        dry_run=args.dry_run,
    )

    if not created:
        print("No test files were created.", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
