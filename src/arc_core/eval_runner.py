from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Sequence

from .skill_validation import find_skill_file, get_namespace_dir

try:
    from jsonschema import ValidationError, validate

    HAS_JSONSCHEMA = True
except ImportError:
    ValidationError = Exception
    validate = None
    HAS_JSONSCHEMA = False


@dataclass
class AssertionResult:
    type: str
    passed: bool
    message: str
    details: dict[str, Any] | None = None


@dataclass
class EvalResult:
    id: str
    title: str | None
    passed: bool
    duration_ms: int
    assertions: list[AssertionResult] = field(default_factory=list)
    error: str | None = None


@dataclass
class RunResult:
    skill: str
    timestamp: str
    total: int
    passed: int
    failed: int
    duration_ms: int
    results: list[EvalResult] = field(default_factory=list)


class AssertionRunner:
    def __init__(self, work_dir: Path):
        self.work_dir = work_dir

    def run(self, assertion: dict[str, Any]) -> AssertionResult:
        atype_value = assertion.get("type")
        atype = atype_value if isinstance(atype_value, str) else None
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
        runner = runners.get(atype or "")
        if not runner:
            return AssertionResult(
                type=str(atype),
                passed=False,
                message=f"Unknown assertion type: {atype}",
            )
        try:
            return runner(assertion)
        except Exception as exc:
            return AssertionResult(
                type=str(atype),
                passed=False,
                message=f"Assertion error: {exc}",
            )

    def _resolve_path(self, path: str) -> Path:
        candidate = Path(path)
        if candidate.is_absolute():
            return candidate
        return self.work_dir / path

    def _assert_file_exists(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["path"])
        exists = path.exists()
        return AssertionResult(
            type="file_exists",
            passed=exists,
            message=f"File {'exists' if exists else 'not found'}: {assertion['path']}",
            details={"path": str(path)},
        )

    def _assert_file_not_exists(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["path"])
        exists = path.exists()
        return AssertionResult(
            type="file_not_exists",
            passed=not exists,
            message=f"File {'found' if exists else 'does not exist'}: {assertion['path']}",
        )

    def _assert_glob_count(self, assertion: dict[str, Any]) -> AssertionResult:
        pattern = assertion["glob"]
        matches = list(self.work_dir.glob(pattern))
        count = len(matches)
        min_count = assertion.get("min", 0)
        max_count = assertion.get("max", float("inf"))
        passed = min_count <= count <= max_count
        return AssertionResult(
            type="glob_count",
            passed=passed,
            message=f"Glob '{pattern}': found {count} matches (expected {min_count}-{max_count})",
            details={"count": count, "matches": [str(match) for match in matches]},
        )

    def _assert_file_contains(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["path"])
        pattern = assertion["pattern"]
        if not path.exists():
            return AssertionResult(
                type="file_contains",
                passed=False,
                message=f"File not found: {assertion['path']}",
            )
        content = path.read_text(encoding="utf-8")
        found = bool(re.search(pattern, content, re.MULTILINE))
        return AssertionResult(
            type="file_contains",
            passed=found,
            message=f"Pattern {'found' if found else 'not found'} in {assertion['path']}",
        )

    def _assert_file_not_contains(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["path"])
        pattern = assertion["pattern"]
        if not path.exists():
            return AssertionResult(
                type="file_not_contains",
                passed=True,
                message=f"File not found (vacuously true): {assertion['path']}",
            )
        content = path.read_text(encoding="utf-8")
        found = bool(re.search(pattern, content, re.MULTILINE))
        return AssertionResult(
            type="file_not_contains",
            passed=not found,
            message=f"Pattern {'found (fail)' if found else 'not found (pass)'} in {assertion['path']}",
        )

    def _assert_json_parse(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["path"])
        if not path.exists():
            return AssertionResult(
                type="json_parse",
                passed=False,
                message=f"File not found: {assertion['path']}",
            )
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            return AssertionResult(
                type="json_parse",
                passed=False,
                message=f"Invalid JSON: {exc}",
            )
        return AssertionResult(
            type="json_parse",
            passed=True,
            message=f"Valid JSON in {assertion['path']}",
            details={"keys": list(data.keys()) if isinstance(data, dict) else None},
        )

    def _assert_jsonl_parse(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["path"])
        if not path.exists():
            return AssertionResult(
                type="jsonl_parse",
                passed=False,
                message=f"File not found: {assertion['path']}",
            )
        errors: list[str] = []
        lines = path.read_text(encoding="utf-8").strip().split("\n")
        for index, line in enumerate(lines, 1):
            try:
                json.loads(line)
            except json.JSONDecodeError as exc:
                errors.append(f"Line {index}: {exc}")
        passed = not errors
        return AssertionResult(
            type="jsonl_parse",
            passed=passed,
            message=f"{'Valid' if passed else 'Invalid'} JSONL ({len(lines)} lines)",
            details={"errors": errors[:5] if errors else None},
        )

    def _assert_json_schema(self, assertion: dict[str, Any]) -> AssertionResult:
        if not HAS_JSONSCHEMA or validate is None:
            return AssertionResult(
                type="json_schema",
                passed=False,
                message="jsonschema package required for json_schema assertion",
            )
        path = self._resolve_path(assertion["path"])
        schema = assertion["schema"]
        if not path.exists():
            return AssertionResult(
                type="json_schema",
                passed=False,
                message=f"File not found: {assertion['path']}",
            )
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
            validate(instance=data, schema=schema)
        except json.JSONDecodeError as exc:
            return AssertionResult(
                type="json_schema",
                passed=False,
                message=f"Invalid JSON: {exc}",
            )
        except ValidationError as exc:
            return AssertionResult(
                type="json_schema",
                passed=False,
                message=f"Schema validation failed: {exc.message}",
            )
        return AssertionResult(
            type="json_schema",
            passed=True,
            message="JSON matches schema",
        )

    def _assert_markdown_tables(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["file"])
        if not path.exists():
            return AssertionResult(
                type="markdown_tables_valid",
                passed=False,
                message=f"File not found: {assertion['file']}",
            )
        lines = path.read_text(encoding="utf-8").split("\n")
        errors: list[str] = []
        in_table = False
        expected_cols = 0
        for index, line in enumerate(lines, 1):
            if "|" in line:
                cols = len([column for column in line.split("|") if column.strip()])
                if not in_table:
                    in_table = True
                    expected_cols = cols
                elif cols != expected_cols:
                    errors.append(f"Line {index}: expected {expected_cols} cols, got {cols}")
            else:
                in_table = False
        passed = not errors
        return AssertionResult(
            type="markdown_tables_valid",
            passed=passed,
            message=f"{'Valid' if passed else 'Invalid'} markdown tables",
            details={"errors": errors[:5] if errors else None},
        )

    def _assert_backticked_paths(self, assertion: dict[str, Any]) -> AssertionResult:
        path = self._resolve_path(assertion["file"])
        if not path.exists():
            return AssertionResult(
                type="backticked_paths_exist",
                passed=False,
                message=f"File not found: {assertion['file']}",
            )
        content = path.read_text(encoding="utf-8")
        backticks = re.findall(r"`([^`]+)`", content)
        missing: list[str] = []
        for backtick in backticks:
            if any(char in backtick for char in "(){}[]<>|&;"):
                continue
            bt_path = self._resolve_path(backtick)
            if not bt_path.exists():
                missing.append(backtick)
        passed = not missing
        return AssertionResult(
            type="backticked_paths_exist",
            passed=passed,
            message=f"{'All' if passed else 'Some'} backticked paths exist",
            details={"missing": missing[:10] if missing else None},
        )

    def _assert_run_validator(self, assertion: dict[str, Any]) -> AssertionResult:
        command = assertion["command"]
        expect_code = assertion.get("expect_exit_code", 0)
        try:
            result = subprocess.run(
                command,
                shell=True,
                cwd=self.work_dir,
                capture_output=True,
                text=True,
                timeout=60,
            )
        except subprocess.TimeoutExpired:
            return AssertionResult(
                type="run_validator",
                passed=False,
                message="Validator timed out",
            )
        except Exception as exc:
            return AssertionResult(
                type="run_validator",
                passed=False,
                message=f"Validator error: {exc}",
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


class EvalRunner:
    def __init__(self, skills_root: Path | None = None):
        self.skills_root = (skills_root or Path(__file__).resolve().parents[2]).resolve()

    def resolve_skill_dir(self, skill: str) -> Path:
        skill_file = find_skill_file(self.skills_root, skill)
        if skill_file is not None:
            return skill_file.parent

        candidates: list[Path] = []
        namespace_dir = get_namespace_dir(skill)
        if namespace_dir:
            candidates.append(self.skills_root / namespace_dir / skill)
        candidates.extend(
            [
                self.skills_root / skill,
                self.skills_root / skill.replace("arc:", ""),
            ]
        )
        for candidate in candidates:
            if candidate.exists():
                return candidate
        return candidates[0]

    def find_evals(self, skill: str) -> Path:
        skill_dir = self.resolve_skill_dir(skill)
        evals_path = skill_dir / "evals.json"
        if not evals_path.exists():
            raise FileNotFoundError(f"No evals.json found for {skill} at {evals_path}")
        return evals_path

    def load_evals(self, path: Path) -> dict[str, Any]:
        return json.loads(path.read_text(encoding="utf-8"))

    def stage_eval_workspace(
        self,
        skill_dir: Path,
        eval_def: dict[str, Any],
        workspace_root: Path,
    ) -> Path:
        workspace_dir = workspace_root / skill_dir.name
        shutil.copytree(skill_dir, workspace_dir)
        inputs = eval_def.get("inputs", {})
        fixtures = inputs.get("fixtures", [])
        for fixture in fixtures:
            src = self.skills_root / fixture["src"]
            dest = workspace_dir / fixture.get("dest", fixture["src"])
            if not src.exists():
                raise FileNotFoundError(f"Fixture not found: {src}")
            dest.parent.mkdir(parents=True, exist_ok=True)
            if src.is_dir():
                shutil.copytree(src, dest, dirs_exist_ok=True)
            else:
                shutil.copy2(src, dest)
        return workspace_dir

    def run_eval(self, eval_def: dict[str, Any], work_dir: Path) -> EvalResult:
        start = time.time()
        eval_id = eval_def["id"]
        title = eval_def.get("title")
        workflow = eval_def.get("workflow", {})
        steps = workflow.get("steps", [])
        for step in steps:
            command = step.get("run")
            if not command:
                continue
            expect_code = step.get("expect_exit_code", 0)
            try:
                command_result = subprocess.run(
                    command,
                    shell=True,
                    cwd=work_dir,
                    capture_output=True,
                    text=True,
                    timeout=300,
                )
            except Exception as exc:
                return EvalResult(
                    id=eval_id,
                    title=title,
                    passed=False,
                    duration_ms=int((time.time() - start) * 1000),
                    error=f"Workflow error: {exc}",
                )
            if command_result.returncode != expect_code:
                stderr = command_result.stderr.strip() or command_result.stdout.strip()
                error = f"Workflow step failed: {command}"
                if stderr:
                    error = f"{error} | {stderr[:300]}"
                return EvalResult(
                    id=eval_id,
                    title=title,
                    passed=False,
                    duration_ms=int((time.time() - start) * 1000),
                    error=error,
                )
        assertion_runner = AssertionRunner(work_dir)
        assertions: list[AssertionResult] = []
        all_passed = True
        for assertion in eval_def.get("assertions", []):
            assertion_result = assertion_runner.run(assertion)
            assertions.append(assertion_result)
            if not assertion_result.passed:
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
        eval_id: str | None = None,
        trigger: str | None = None,
        tags: list[str] | None = None,
    ) -> RunResult:
        start = time.time()
        evals_path = self.find_evals(skill)
        evals_data = self.load_evals(evals_path)
        skill_dir = evals_path.parent
        evals_to_run = evals_data.get("evals", [])
        if eval_id:
            evals_to_run = [item for item in evals_to_run if item["id"] == eval_id]
        if trigger:
            evals_to_run = [
                item
                for item in evals_to_run
                if item.get("trigger") == trigger
                or trigger in item.get("triggers", [])
                or trigger in item.get("tags", [])
            ]
        if tags:
            evals_to_run = [
                item for item in evals_to_run if any(tag in item.get("tags", []) for tag in tags)
            ]
        with tempfile.TemporaryDirectory(prefix=f"eval_{skill_dir.name.replace(':', '_')}_") as tmpdir:
            workspace_root = Path(tmpdir)
            results: list[EvalResult] = []
            for eval_def in evals_to_run:
                eval_root = workspace_root / eval_def["id"]
                eval_root.mkdir(parents=True, exist_ok=True)
                work_dir = self.stage_eval_workspace(skill_dir, eval_def, eval_root)
                results.append(self.run_eval(eval_def, work_dir))
        passed = sum(1 for result in results if result.passed)
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
                            "id": item.id,
                            "title": item.title,
                            "passed": item.passed,
                            "duration_ms": item.duration_ms,
                            "error": item.error,
                            "assertions": [
                                {
                                    "type": assertion.type,
                                    "passed": assertion.passed,
                                    "message": assertion.message,
                                }
                                for assertion in item.assertions
                            ],
                        }
                        for item in result.results
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
        for item in result.results:
            status = "✓ PASS" if item.passed else "✗ FAIL"
            lines.append(f"[{status}] {item.id}")
            if item.title:
                lines.append(f"    {item.title}")
            if item.error:
                lines.append(f"    Error: {item.error}")
            for assertion in item.assertions:
                assertion_status = "✓" if assertion.passed else "✗"
                lines.append(f"    {assertion_status} {assertion.type}: {assertion.message}")
            lines.append("")
        return "\n".join(lines)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run arc: skill evaluations")
    parser.add_argument("--skill", "-s", required=True, help="Skill to evaluate (e.g., arc:e2e)")
    parser.add_argument("--id", help="Run specific eval by ID")
    parser.add_argument("--trigger", help="Filter by trigger type")
    parser.add_argument("--tag", "-t", action="append", help="Filter by tag (can repeat)")
    parser.add_argument("--json", "-j", action="store_true", help="Output as JSON")
    parser.add_argument("--skills-root", type=Path, help="Root directory of skills")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        runner = EvalRunner(args.skills_root)
        result = runner.run(skill=args.skill, eval_id=args.id, trigger=args.trigger, tags=args.tag)
        print(runner.format_result(result, json_output=args.json))
        return 1 if result.failed > 0 else 0
    except FileNotFoundError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 2
    except Exception as exc:
        print(f"Error: {exc}", file=sys.stderr)
        import traceback

        traceback.print_exc()
        return 2
