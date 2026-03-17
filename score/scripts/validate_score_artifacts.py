#!/usr/bin/env python3

import argparse
import json
from pathlib import Path
from typing import Any


def _is_number(value: Any) -> bool:
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _read_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception as e:
        raise ValueError(f"failed to read json: {path}: {e}")


def _expect(condition: bool, message: str, errors: list[str]) -> None:
    if not condition:
        errors.append(message)


def _expect_dict(obj: Any, label: str, errors: list[str]) -> dict:
    _expect(isinstance(obj, dict), f"{label}: expected object", errors)
    return obj if isinstance(obj, dict) else {}


def _expect_str(obj: Any, label: str, errors: list[str]) -> str:
    _expect(
        isinstance(obj, str) and obj.strip() != "",
        f"{label}: expected non-empty string",
        errors,
    )
    return obj if isinstance(obj, str) else ""


def _expect_list(obj: Any, label: str, errors: list[str]) -> list:
    _expect(isinstance(obj, list), f"{label}: expected array", errors)
    return obj if isinstance(obj, list) else []


def _extract_meta(obj: Any, label: str, errors: list[str]) -> tuple[str, str]:
    root = _expect_dict(obj, label, errors)
    schema_version = _expect_str(
        root.get("schema_version"), f"{label}.schema_version", errors
    )
    producer_skill = _expect_str(
        root.get("producer_skill"), f"{label}.producer_skill", errors
    )
    return schema_version, producer_skill


def validate_overall_score(obj: Any, errors: list[str]) -> None:
    root = _expect_dict(obj, "overall-score", errors)
    _expect(
        _is_number(root.get("score")), "overall-score.score: expected number", errors
    )
    if "weighted_score" in root:
        _expect(
            _is_number(root.get("weighted_score")),
            "overall-score.weighted_score: expected number",
            errors,
        )
    _expect_str(root.get("grade"), "overall-score.grade", errors)
    dim_scores = root.get("dimension_scores")
    if dim_scores is not None:
        dim_scores_obj = _expect_dict(
            dim_scores, "overall-score.dimension_scores", errors
        )
        for k, v in dim_scores_obj.items():
            if not isinstance(k, str):
                errors.append("overall-score.dimension_scores: keys must be strings")
                break
            if not _is_number(v):
                errors.append(f"overall-score.dimension_scores.{k}: expected number")
    calc_at = root.get("calculated_at")
    if calc_at is not None:
        _expect_str(calc_at, "overall-score.calculated_at", errors)


def validate_smell_report(obj: Any, errors: list[str]) -> None:
    root = _expect_dict(obj, "smell-report", errors)
    summary = _expect_dict(root.get("summary"), "smell-report.summary", errors)
    _expect(
        isinstance(summary.get("total_violations"), int),
        "smell-report.summary.total_violations: expected int",
        errors,
    )
    by_severity = summary.get("by_severity")
    if by_severity is not None:
        _expect_dict(by_severity, "smell-report.summary.by_severity", errors)
    by_category = summary.get("by_category")
    if by_category is not None:
        _expect_dict(by_category, "smell-report.summary.by_category", errors)
    violations = _expect_list(root.get("violations"), "smell-report.violations", errors)
    for i, v in enumerate(violations[:50]):
        if not isinstance(v, dict):
            errors.append(f"smell-report.violations[{i}]: expected object")
            continue
        _expect_str(v.get("id"), f"smell-report.violations[{i}].id", errors)
        _expect_str(v.get("category"), f"smell-report.violations[{i}].category", errors)
        _expect_str(v.get("severity"), f"smell-report.violations[{i}].severity", errors)


def validate_bugfix_grades(obj: Any, errors: list[str]) -> None:
    root = _expect_dict(obj, "bugfix-grades", errors)
    summary = _expect_dict(root.get("summary"), "bugfix-grades.summary", errors)
    _expect(
        isinstance(summary.get("total_commits"), int),
        "bugfix-grades.summary.total_commits: expected int",
        errors,
    )
    _expect_dict(summary.get("by_grade"), "bugfix-grades.summary.by_grade", errors)
    _expect_dict(summary.get("by_tag"), "bugfix-grades.summary.by_tag", errors)
    _expect(
        isinstance(summary.get("total_lines_changed"), int),
        "bugfix-grades.summary.total_lines_changed: expected int",
        errors,
    )
    commits = root.get("commits")
    if commits is not None:
        _expect_list(commits, "bugfix-grades.commits", errors)


def validate_review_handoff(obj: Any, errors: list[str]) -> None:
    root = _expect_dict(obj, "review-handoff", errors)
    _expect_str(root.get("generated_at"), "review-handoff.generated_at", errors)
    _expect_str(root.get("project_path"), "review-handoff.project_path", errors)
    artifacts = _expect_dict(root.get("artifacts"), "review-handoff.artifacts", errors)
    _expect(
        isinstance(artifacts.get("overall_score"), dict),
        "review-handoff.artifacts.overall_score: expected object",
        errors,
    )
    _expect(
        isinstance(artifacts.get("smell_report"), dict),
        "review-handoff.artifacts.smell_report: expected object",
        errors,
    )
    overall = _expect_dict(root.get("overall"), "review-handoff.overall", errors)
    _expect(
        _is_number(overall.get("score")),
        "review-handoff.overall.score: expected number",
        errors,
    )
    _expect_str(overall.get("grade"), "review-handoff.overall.grade", errors)


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate arc:score output artifacts")
    parser.add_argument(
        "--score-dir", required=True, help="Path to .arc/score/<project>"
    )
    parser.add_argument(
        "--require-bugfix",
        action="store_true",
        help="Fail if analysis/bugfix-grades.json is missing",
    )
    parser.add_argument(
        "--require-review-handoff",
        action="store_true",
        help="Fail if handoff/review-input.json is missing",
    )
    parser.add_argument("--output", help="Optional path to write validation JSON")

    args = parser.parse_args()
    score_dir = Path(args.score_dir).resolve()

    paths = {
        "overall_score": score_dir / "score" / "overall-score.json",
        "smell_report": score_dir / "analysis" / "smell-report.json",
        "bugfix_grades": score_dir / "analysis" / "bugfix-grades.json",
        "review_handoff": score_dir / "handoff" / "review-input.json",
    }

    errors: list[str] = []
    warnings: list[str] = []
    meta: dict[str, dict[str, str]] = {}

    if not paths["overall_score"].exists():
        errors.append(f"missing: {paths['overall_score']}")
    if not paths["smell_report"].exists():
        errors.append(f"missing: {paths['smell_report']}")
    if args.require_bugfix and not paths["bugfix_grades"].exists():
        errors.append(f"missing: {paths['bugfix_grades']}")
    if args.require_review_handoff and not paths["review_handoff"].exists():
        errors.append(f"missing: {paths['review_handoff']}")

    loaded: dict[str, Any] = {}
    for key, p in paths.items():
        if not p.exists():
            continue
        try:
            loaded[key] = _read_json(p)
        except Exception as e:
            errors.append(str(e))

    if "overall_score" in loaded:
        meta["overall_score"] = {}
        schema_version, producer_skill = _extract_meta(
            loaded["overall_score"], "overall-score", errors
        )
        meta["overall_score"]["schema_version"] = schema_version
        meta["overall_score"]["producer_skill"] = producer_skill
        validate_overall_score(loaded["overall_score"], errors)

    if "smell_report" in loaded:
        meta["smell_report"] = {}
        schema_version, producer_skill = _extract_meta(
            loaded["smell_report"], "smell-report", errors
        )
        meta["smell_report"]["schema_version"] = schema_version
        meta["smell_report"]["producer_skill"] = producer_skill
        validate_smell_report(loaded["smell_report"], errors)

    if "bugfix_grades" in loaded:
        meta["bugfix_grades"] = {}
        schema_version, producer_skill = _extract_meta(
            loaded["bugfix_grades"], "bugfix-grades", errors
        )
        meta["bugfix_grades"]["schema_version"] = schema_version
        meta["bugfix_grades"]["producer_skill"] = producer_skill
        validate_bugfix_grades(loaded["bugfix_grades"], errors)

    if "review_handoff" in loaded:
        meta["review_handoff"] = {}
        schema_version, producer_skill = _extract_meta(
            loaded["review_handoff"], "review-handoff", errors
        )
        meta["review_handoff"]["schema_version"] = schema_version
        meta["review_handoff"]["producer_skill"] = producer_skill
        validate_review_handoff(loaded["review_handoff"], errors)

    schema_versions = sorted(
        {v.get("schema_version", "") for v in meta.values() if v.get("schema_version")}
    )
    if len(schema_versions) > 1:
        errors.append(f"schema_version mismatch: {schema_versions}")

    for k, v in meta.items():
        producer = v.get("producer_skill")
        if producer and producer != "arc:score":
            errors.append(f"{k}.producer_skill expected arc:score, got {producer}")

    if "review_handoff" not in loaded and paths["review_handoff"].exists() is False:
        warnings.append("review handoff not present (handoff/review-input.json)")
    if "bugfix_grades" not in loaded and paths["bugfix_grades"].exists() is False:
        warnings.append("bugfix grades not present (analysis/bugfix-grades.json)")

    ok = len(errors) == 0
    result = {
        "ok": ok,
        "score_dir": str(score_dir),
        "files": {k: str(p) for k, p in paths.items()},
        "meta": meta,
        "schema_versions": schema_versions,
        "errors": errors,
        "warnings": warnings,
    }

    if args.output:
        out_path = Path(args.output)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(json.dumps(result, indent=2), encoding="utf-8")

    if ok:
        print("OK: arc:score artifacts valid")
        return 0

    print("FAIL: arc:score artifacts invalid")
    for e in errors:
        print(f"- {e}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
