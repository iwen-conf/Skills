---
name: "arc:test"
description: "代码级测试生成：为函数/模块自动生成单测、集成、边界、表驱动、benchmark 和 fuzz 测试。"
---
# arc:test — code-level test generation

## Overview

`arc:test` generates function-level tests for Go, Rust, Python, TypeScript and Swift projects. It detects the language ecosystem, scaffolds test files from templates, fills test logic guided by ISTQB techniques (boundary value analysis, equivalence partitioning, error path), and validates that every generated test compiles and passes.

## Quick Contract

- **Trigger**: Source code needs test coverage — new modules, refactored functions, coverage gaps.
- **Inputs**: `project_path`, `target` (file or directory), `test_types` (unit/boundary/table/benchmark/fuzz/integration).
- **Outputs**: Generated test files + `test-report.md` with compile/pass evidence.
- **Quality Gate**: All generated tests must compile and pass; `test-report.md` must contain evidence.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:test`, which will analyze source code and generate code-level tests with compile-and-pass evidence."

## The Iron Law

```
NO TEST CLAIM WITHOUT COMPILE AND PASS EVIDENCE
```

No test may be declared generated unless it compiles and passes in the target project.

## Workflow

1. **Phase 0 — Detect ecosystem**: Run `detect_lang.py --project-path <path> --json` to identify language, framework, test command, and existing test files.
2. **Phase 1 — Analyze source**: Read target files, extract function/method/type signatures, identify public API surface and edge-case-prone logic.
3. **Phase 2 — Design test cases**: Apply ISTQB techniques — boundary value analysis for numeric/string limits, equivalence partitioning for input domains, error path coverage for failure modes. For integration test types, identify module boundaries, interface contracts, and dependency wiring points. Produce a case list before writing code.
4. **Phase 3 — Scaffold and fill**: Run `scaffold_tests.py` to generate test file skeletons from templates, then fill in assertions and test logic using the case list.
5. **Phase 4 — Validate**: Run `validate_tests.py` to compile and execute all generated tests. Output `test-report.md` with pass/fail evidence per file.

## Quality Gates

- Every generated test file must compile without errors.
- Every generated test must pass (or be explicitly marked as expected-failure with justification).
- `test-report.md` must contain: language detected, files generated, test command used, exit code, and stdout/stderr excerpt.
- Benchmark tests must include at least one `b.ReportAllocs()` (Go) or equivalent metric collection.
- Fuzz tests must define a valid corpus entry.
- If the chat handoff includes a compact matrix such as generated files, test types, or pass/fail status, prefer `terminal-table-output`; keep `test-report.md` as the canonical file artifact.

## Expert Standards

- Test case design follows `ISTQB` terminology and techniques at the component testing level.
- Numeric inputs must include `boundary value analysis` (边界值分析): min, min+1, nominal, max-1, max, and off-boundary values.
- Input domains must apply `equivalence partitioning` (等价类划分): valid classes, invalid classes, and empty/nil/zero classes.
- Generated tests should improve `code coverage` (代码覆盖率) — target statement coverage for new code, branch coverage for complex logic.
- Test strategy respects the `test pyramid` (测试金字塔): unit tests are the foundation, integration tests are selective, E2E is delegated to `arc:e2e`.
- `Integration tests` (集成测试) verify interactions across module boundaries — focus on interface contracts, dependency wiring, error propagation, and data flow between collaborating components. Use build tags (Go: `//go:build integration`), markers (Python: `@pytest.mark.integration`), or separate directories (Rust: `tests/`) to isolate them from fast unit tests.
- Integration tests must have explicit setup/teardown: initialize shared dependencies (DB, HTTP client, service stubs) in `TestMain`/`beforeAll`/`setUp`/fixtures, and clean state in teardown.
- Error paths must be tested: invalid input, permission denied, timeout, resource exhaustion.
- Table-driven / parameterized tests are preferred over duplicated test functions.

## Scripts & Commands

- Language detection: `python3 Arc/arc:test/scripts/detect_lang.py --project-path <path> --json`
- Test scaffolding: `python3 Arc/arc:test/scripts/scaffold_tests.py --project-path <path> --target <file_or_dir> --test-types unit,boundary,benchmark,integration [--lang auto] [--output-dir <dir>] [--force] [--dry-run]`
- Test validation: `python3 Arc/arc:test/scripts/validate_tests.py --project-path <path> --test-files <file1,file2> [--report-out test-report.md] [--timeout 120]`

## Red Flags

- Generating tests that import non-existent modules or reference undefined symbols.
- Claiming test coverage without running the tests.
- Writing only happy-path tests and ignoring error/boundary cases.
- Integration tests without setup/teardown — leaking state across test runs.
- Integration tests that depend on external services without stubs or build-tag isolation.
- Generating benchmark tests without metric collection calls.
- Overwriting existing test files without `--force` flag.

## When to Use

- **Primary Trigger**: Source code needs test coverage — new modules, refactored functions, coverage gaps identified by `arc:gate` or `arc:audit`.
- **Typical Scenario**: After `arc:build` delivers code, generate unit/boundary/benchmark tests; before `arc:gate`, raise coverage to meet thresholds.
- **Boundary Note**: Browser E2E testing uses `arc:e2e`; fixing failing tests uses `arc:fix`; `arc:test` focuses exclusively on generating new code-level tests.

## Input Arguments

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_path` | `string` | required | Root path of the target project (must contain language marker file) |
| `target` | `string` | required | File or directory to generate tests for |
| `test_types` | `string` | optional | Comma-separated test types: unit, boundary, table, benchmark, fuzz, integration (default: unit,boundary) |
| `lang` | `string` | optional | Language override: go, rust, python, typescript, swift (default: auto-detect) |
| `output_dir` | `string` | optional | Output directory for generated test files (default: language convention) |
| `force` | `boolean` | optional | Overwrite existing test files (default: false) |
| `dry_run` | `boolean` | optional | Print planned files without writing (default: false) |

## Outputs

```text
arc:test/
  test-report.md          # Compile/pass evidence for all generated tests
  <generated test files>  # Placed according to language convention
```
