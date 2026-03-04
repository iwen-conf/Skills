#!/usr/bin/env python3
"""
Evaluation Runner for Arc Skills

Runs evals.json test suites for arc: skills.

Usage:
    # Run all evals for a skill
    python run_evals.py --skill arc:simulate

    # Run specific eval
    python run_evals.py --skill arc:simulate --id compile_then_gate

    # Run with specific trigger
    python run_evals.py --skill arc:simulate --trigger ci

    # JSON output
    python run_evals.py --skill arc:simulate --json

Exit codes:
    0 = All tests PASS
    1 = One or more tests FAIL
    2 = Configuration error
"""

import sys
import json
import subprocess
import argparse
import tempfile
import shutil
from pathlib import Path
from typing import Dict, List, Any, Optional
from dataclasses import dataclass, field
from datetime import datetime
import re
import glob as glob_module

# Schema validation (optional, requires jsonschema package)
try:
    from jsonschema import validate, ValidationError

    HAS_JSONSCHEMA = True
except ImportError:
    HAS_JSONSCHEMA = False


@dataclass
class AssertionResult:
    """Result of a single assertion."""

    type: str
    passed: bool
    message: str
    details: Optional[Dict[str, Any]] = None


@dataclass
class EvalResult:
    """Result of a single evaluation."""

    id: str
    title: Optional[str]
    passed: bool
    duration_ms: int
    assertions: List[AssertionResult] = field(default_factory=list)
    error: Optional[str] = None


@dataclass
class RunResult:
    """Result of a complete eval run."""

    skill: str
    timestamp: str
    total: int
    passed: int
    failed: int
    duration_ms: int
    results: List[EvalResult] = field(default_factory=list)


class AssertionRunner:
    """Runs individual assertions."""

    def __init__(self, work_dir: Path):
        self.work_dir = work_dir

    def run(self, assertion: Dict[str, Any]) -> AssertionResult:
        """Run a single assertion."""
        atype = assertion.get("type")

        runners = {
            "file_exists": self._assert_file_exists,
            "file_not_exists": self._assert_file_not_exists,
            "glob_count": self._assert_glob_count,
            "file_contains": self._assert_file_contains,
            "file_not_contains": self._assert_file_not_contains,
            "json_parse": self._assert_json_parse,
            "jsonl_parse": self._assert_jsonl_parse,
            "json_schema": self._assert_json_schema,
            "markdown_tables_valid": self._assert_markdown_tables,
            "backticked_paths_exist": self._assert_backticked_paths,
            "run_validator": self._assert_run_validator,
        }

        runner = runners.get(atype)
        if not runner:
            return AssertionResult(
                type=atype, passed=False, message=f"Unknown assertion type: {atype}"
            )

        try:
            return runner(assertion)
        except Exception as e:
            return AssertionResult(
                type=atype, passed=False, message=f"Assertion error: {e}"
            )

    def _resolve_path(self, path: str) -> Path:
        """Resolve path relative to work_dir."""
        p = Path(path)
        if p.is_absolute():
            return p
        return self.work_dir / path

    def _assert_file_exists(self, a: Dict) -> AssertionResult:
        path = self._resolve_path(a["path"])
        exists = path.exists()
        return AssertionResult(
            type="file_exists",
            passed=exists,
            message=f"File {'exists' if exists else 'not found'}: {a['path']}",
            details={"path": str(path)},
        )

    def _assert_file_not_exists(self, a: Dict) -> AssertionResult:
        path = self._resolve_path(a["path"])
        exists = path.exists()
        return AssertionResult(
            type="file_not_exists",
            passed=not exists,
            message=f"File {'found' if exists else 'does not exist'}: {a['path']}",
        )

    def _assert_glob_count(self, a: Dict) -> AssertionResult:
        pattern = a["glob"]
        matches = list(self.work_dir.glob(pattern))
        count = len(matches)
        min_count = a.get("min", 0)
        max_count = a.get("max", float("inf"))

        passed = min_count <= count <= max_count
        return AssertionResult(
            type="glob_count",
            passed=passed,
            message=f"Glob '{pattern}': found {count} matches (expected {min_count}-{max_count})",
            details={"count": count, "matches": [str(m) for m in matches]},
        )

    def _assert_file_contains(self, a: Dict) -> AssertionResult:
        path = self._resolve_path(a["path"])
        pattern = a["pattern"]

        if not path.exists():
            return AssertionResult(
                type="file_contains",
                passed=False,
                message=f"File not found: {a['path']}",
            )

        content = path.read_text(encoding="utf-8")
        found = bool(re.search(pattern, content, re.MULTILINE))

        return AssertionResult(
            type="file_contains",
            passed=found,
            message=f"Pattern {'found' if found else 'not found'} in {a['path']}",
        )

    def _assert_file_not_contains(self, a: Dict) -> AssertionResult:
        path = self._resolve_path(a["path"])
        pattern = a["pattern"]

        if not path.exists():
            return AssertionResult(
                type="file_not_contains",
                passed=True,
                message=f"File not found (vacuously true): {a['path']}",
            )

        content = path.read_text(encoding="utf-8")
        found = bool(re.search(pattern, content, re.MULTILINE))

        return AssertionResult(
            type="file_not_contains",
            passed=not found,
            message=f"Pattern {'found (fail)' if found else 'not found (pass)'} in {a['path']}",
        )

    def _assert_json_parse(self, a: Dict) -> AssertionResult:
        path = self._resolve_path(a["path"])

        if not path.exists():
            return AssertionResult(
                type="json_parse", passed=False, message=f"File not found: {a['path']}"
            )

        try:
            data = json.loads(path.read_text(encoding="utf-8"))
            return AssertionResult(
                type="json_parse",
                passed=True,
                message=f"Valid JSON in {a['path']}",
                details={"keys": list(data.keys()) if isinstance(data, dict) else None},
            )
        except json.JSONDecodeError as e:
            return AssertionResult(
                type="json_parse", passed=False, message=f"Invalid JSON: {e}"
            )

    def _assert_jsonl_parse(self, a: Dict) -> AssertionResult:
        path = self._resolve_path(a["path"])

        if not path.exists():
            return AssertionResult(
                type="jsonl_parse", passed=False, message=f"File not found: {a['path']}"
            )

        errors = []
        lines = path.read_text(encoding="utf-8").strip().split("\n")

        for i, line in enumerate(lines, 1):
            try:
                json.loads(line)
            except json.JSONDecodeError as e:
                errors.append(f"Line {i}: {e}")

        passed = len(errors) == 0
        return AssertionResult(
            type="jsonl_parse",
            passed=passed,
            message=f"{'Valid' if passed else 'Invalid'} JSONL ({len(lines)} lines)",
            details={"errors": errors[:5] if errors else None},
        )

    def _assert_json_schema(self, a: Dict) -> AssertionResult:
        if not HAS_JSONSCHEMA:
            return AssertionResult(
                type="json_schema",
                passed=False,
                message="jsonschema package required for json_schema assertion",
            )

        path = self._resolve_path(a["path"])
        schema = a["schema"]

        if not path.exists():
            return AssertionResult(
                type="json_schema", passed=False, message=f"File not found: {a['path']}"
            )

        try:
            data = json.loads(path.read_text(encoding="utf-8"))
            validate(instance=data, schema=schema)
            return AssertionResult(
                type="json_schema", passed=True, message=f"JSON matches schema"
            )
        except json.JSONDecodeError as e:
            return AssertionResult(
                type="json_schema", passed=False, message=f"Invalid JSON: {e}"
            )
        except ValidationError as e:
            return AssertionResult(
                type="json_schema",
                passed=False,
                message=f"Schema validation failed: {e.message}",
            )

    def _assert_markdown_tables(self, a: Dict) -> AssertionResult:
        """Validate markdown tables have consistent column counts."""
        path = self._resolve_path(a["file"])

        if not path.exists():
            return AssertionResult(
                type="markdown_tables_valid",
                passed=False,
                message=f"File not found: {a['file']}",
            )

        content = path.read_text(encoding="utf-8")
        lines = content.split("\n")

        errors = []
        in_table = False
        table_start = 0
        expected_cols = 0

        for i, line in enumerate(lines, 1):
            if "|" in line:
                cols = len([c for c in line.split("|") if c.strip()])

                if not in_table:
                    in_table = True
                    table_start = i
                    expected_cols = cols
                elif cols != expected_cols:
                    errors.append(
                        f"Line {i}: expected {expected_cols} cols, got {cols}"
                    )
            else:
                in_table = False

        passed = len(errors) == 0
        return AssertionResult(
            type="markdown_tables_valid",
            passed=passed,
            message=f"{'Valid' if passed else 'Invalid'} markdown tables",
            details={"errors": errors[:5] if errors else None},
        )

    def _assert_backticked_paths(self, a: Dict) -> AssertionResult:
        """Check that backticked file paths exist."""
        path = self._resolve_path(a["file"])

        if not path.exists():
            return AssertionResult(
                type="backticked_paths_exist",
                passed=False,
                message=f"File not found: {a['file']}",
            )

        content = path.read_text(encoding="utf-8")
        # Find backticked paths
        backticks = re.findall(r"`([^`]+)`", content)

        missing = []
        for bt in backticks:
            # Skip if it looks like code, not a path
            if any(c in bt for c in "(){}[]<>|&;"):
                continue
            bt_path = self._resolve_path(bt)
            if not bt_path.exists():
                missing.append(bt)

        passed = len(missing) == 0
        return AssertionResult(
            type="backticked_paths_exist",
            passed=passed,
            message=f"{'All' if passed else 'Some'} backticked paths exist",
            details={"missing": missing[:10] if missing else None},
        )

    def _assert_run_validator(self, a: Dict) -> AssertionResult:
        """Run an external validator command."""
        cmd = a["command"]
        expect_code = a.get("expect_exit_code", 0)

        try:
            result = subprocess.run(
                cmd,
                shell=True,
                cwd=self.work_dir,
                capture_output=True,
                text=True,
                timeout=60,
            )

            passed = result.returncode == expect_code
            return AssertionResult(
                type="run_validator",
                passed=passed,
                message=f"Validator {'passed' if passed else 'failed'} (exit {result.returncode})",
                details={
                    "stdout": result.stdout[:500] if result.stdout else None,
                    "stderr": result.stderr[:500] if result.stderr else None,
                },
            )
        except subprocess.TimeoutExpired:
            return AssertionResult(
                type="run_validator", passed=False, message="Validator timed out"
            )
        except Exception as e:
            return AssertionResult(
                type="run_validator", passed=False, message=f"Validator error: {e}"
            )


class EvalRunner:
    """Runs evaluation suites for arc: skills."""

    def __init__(self, skills_root: Path = None):
        self.skills_root = skills_root or Path(__file__).parent.parent

    def find_evals(self, skill: str) -> Path:
        """Find evals.json for a skill."""
        # Convert arc:simulate -> simulate
        skill_dir = skill.replace("arc:", "")
        evals_path = self.skills_root / skill_dir / "evals.json"

        if not evals_path.exists():
            raise FileNotFoundError(f"No evals.json found for {skill} at {evals_path}")

        return evals_path

    def load_evals(self, path: Path) -> Dict:
        """Load and parse evals.json."""
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)

    def run_eval(self, eval_def: Dict, work_dir: Path) -> EvalResult:
        """Run a single evaluation."""
        import time

        start = time.time()

        eval_id = eval_def["id"]
        title = eval_def.get("title")

        # Run workflow if present
        workflow = eval_def.get("workflow", {})
        steps = workflow.get("steps", [])

        for step in steps:
            cmd = step.get("run")
            if cmd:
                expect_code = step.get("expect_exit_code", 0)
                try:
                    result = subprocess.run(
                        cmd,
                        shell=True,
                        cwd=work_dir,
                        capture_output=True,
                        text=True,
                        timeout=300,
                    )
                    if result.returncode != expect_code:
                        return EvalResult(
                            id=eval_id,
                            title=title,
                            passed=False,
                            duration_ms=int((time.time() - start) * 1000),
                            error=f"Workflow step failed: {cmd}",
                        )
                except Exception as e:
                    return EvalResult(
                        id=eval_id,
                        title=title,
                        passed=False,
                        duration_ms=int((time.time() - start) * 1000),
                        error=f"Workflow error: {e}",
                    )

        # Run assertions
        assertion_runner = AssertionRunner(work_dir)
        assertions = []
        all_passed = True

        for a in eval_def.get("assertions", []):
            result = assertion_runner.run(a)
            assertions.append(result)
            if not result.passed:
                all_passed = False

        return EvalResult(
            id=eval_id,
            title=title,
            passed=all_passed,
            duration_ms=int((time.time() - start) * 1000),
            assertions=assertions,
        )

    def run(
        self,
        skill: str,
        eval_id: Optional[str] = None,
        trigger: Optional[str] = None,
        tags: List[str] = None,
    ) -> RunResult:
        """Run evaluations for a skill."""
        import time

        start = time.time()

        evals_path = self.find_evals(skill)
        evals_data = self.load_evals(evals_path)

        # Filter evals
        evals_to_run = evals_data.get("evals", [])

        if eval_id:
            evals_to_run = [e for e in evals_to_run if e["id"] == eval_id]

        if tags:
            evals_to_run = [
                e for e in evals_to_run if any(t in e.get("tags", []) for t in tags)
            ]

        # Create work directory
        skill_dir = skill.replace("arc:", "")
        with tempfile.TemporaryDirectory(prefix=f"eval_{skill_dir}_") as tmpdir:
            work_dir = Path(tmpdir)

            results = []
            for eval_def in evals_to_run:
                # Copy fixtures if present
                inputs = eval_def.get("inputs", {})
                fixtures = inputs.get("fixtures", [])

                for fixture in fixtures:
                    src = self.skills_root / fixture["src"]
                    dest = work_dir / fixture.get("dest", fixture["src"])
                    dest.parent.mkdir(parents=True, exist_ok=True)
                    if src.is_dir():
                        shutil.copytree(src, dest)
                    else:
                        shutil.copy(src, dest)

                # Run eval
                result = self.run_eval(eval_def, work_dir)
                results.append(result)

        passed = sum(1 for r in results if r.passed)

        return RunResult(
            skill=skill,
            timestamp=datetime.now().isoformat(),
            total=len(results),
            passed=passed,
            failed=len(results) - passed,
            duration_ms=int((time.time() - start) * 1000),
            results=results,
        )

    def format_result(self, result: RunResult, json_output: bool = False) -> str:
        """Format result for output."""
        if json_output:
            return json.dumps(
                {
                    "skill": result.skill,
                    "timestamp": result.timestamp,
                    "total": result.total,
                    "passed": result.passed,
                    "failed": result.failed,
                    "duration_ms": result.duration_ms,
                    "results": [
                        {
                            "id": r.id,
                            "title": r.title,
                            "passed": r.passed,
                            "duration_ms": r.duration_ms,
                            "error": r.error,
                            "assertions": [
                                {
                                    "type": a.type,
                                    "passed": a.passed,
                                    "message": a.message,
                                }
                                for a in r.assertions
                            ],
                        }
                        for r in result.results
                    ],
                },
                indent=2,
            )

        lines = [
            f"Eval Results for {result.skill}",
            "=" * 50,
            f"Total: {result.total} | Passed: {result.passed} | Failed: {result.failed}",
            f"Duration: {result.duration_ms}ms",
            "",
        ]

        for r in result.results:
            status = "✓ PASS" if r.passed else "✗ FAIL"
            lines.append(f"[{status}] {r.id}")
            if r.title:
                lines.append(f"    {r.title}")
            if r.error:
                lines.append(f"    Error: {r.error}")
            for a in r.assertions:
                astatus = "✓" if a.passed else "✗"
                lines.append(f"    {astatus} {a.type}: {a.message}")
            lines.append("")

        return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Run arc: skill evaluations")
    parser.add_argument(
        "--skill", "-s", required=True, help="Skill to evaluate (e.g., arc:simulate)"
    )
    parser.add_argument("--id", help="Run specific eval by ID")
    parser.add_argument("--trigger", help="Filter by trigger type")
    parser.add_argument(
        "--tag", "-t", action="append", help="Filter by tag (can repeat)"
    )
    parser.add_argument("--json", "-j", action="store_true", help="Output as JSON")
    parser.add_argument("--skills-root", type=Path, help="Root directory of skills")

    args = parser.parse_args()

    try:
        runner = EvalRunner(args.skills_root)
        result = runner.run(
            skill=args.skill, eval_id=args.id, trigger=args.trigger, tags=args.tag
        )

        print(runner.format_result(result, json_output=args.json))

        if result.failed > 0:
            sys.exit(1)
        sys.exit(0)

    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(2)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        import traceback

        traceback.print_exc()
        sys.exit(2)


if __name__ == "__main__":
    main()
