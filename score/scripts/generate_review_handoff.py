#!/usr/bin/env python3
"""generate_review_handoff.py - Generate handoff/review-input.json from arc-score outputs.

Usage:
    python generate_review_handoff.py --score-dir .arc/score/<project-name>
"""

import argparse
import hashlib
import json
from datetime import datetime
from pathlib import Path


SCHEMA_VERSION = "1.0.0"
PRODUCER_SKILL = "arc-score"


_SEVERITY_RANK = {"critical": 0, "high": 1, "medium": 2, "low": 3}


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def load_json(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def to_review_dimension_key(key: str) -> str:
    mapping = {
        "code_quality": "code-quality",
        "tech_debt": "tech-debt",
    }
    return mapping.get(key, key)


def pick_top_violations(smell_report: dict, limit: int) -> list[dict]:
    violations = smell_report.get("violations")
    if not isinstance(violations, list):
        return []

    def sort_key(v: dict) -> tuple:
        sev = str(v.get("severity", "medium")).lower()
        return (
            _SEVERITY_RANK.get(sev, 99),
            str(v.get("id", "")),
            str(v.get("file", "")),
            int(v.get("line", 0) or 0),
        )

    top: list[dict] = []
    for v in sorted(violations, key=sort_key):
        top.append(
            {
                "id": v.get("id"),
                "category": v.get("category"),
                "name": v.get("name"),
                "severity": v.get("severity"),
                "file": v.get("file"),
                "line": v.get("line"),
                "message": v.get("message"),
                "suggestion": v.get("suggestion"),
            }
        )
        if len(top) >= limit:
            break

    return top


def build_artifact_ref(score_dir: Path, artifact_path: Path) -> dict:
    rel = artifact_path
    try:
        rel = artifact_path.relative_to(score_dir)
    except Exception:
        pass

    return {
        "path": str(rel),
        "sha256": sha256_file(artifact_path),
    }


def main() -> None:
    parser = argparse.ArgumentParser(
        description="生成 arc-score 交接产物 handoff/review-input.json"
    )
    parser.add_argument(
        "--score-dir", required=True, help=".arc/score/<project-name> 目录"
    )
    parser.add_argument(
        "--output",
        help="输出路径 (可选，默认 <score-dir>/handoff/review-input.json)",
    )
    parser.add_argument(
        "--top-violations",
        type=int,
        default=30,
        help="写入交接文件的违规详情条数上限",
    )
    parser.add_argument(
        "--require-bugfix",
        action="store_true",
        help="强制要求 analysis/bugfix-grades.json 存在",
    )

    args = parser.parse_args()

    score_dir = Path(args.score_dir).resolve()
    smell_path = score_dir / "analysis" / "smell-report.json"
    overall_path = score_dir / "score" / "overall-score.json"
    bugfix_path = score_dir / "analysis" / "bugfix-grades.json"

    if not smell_path.exists():
        raise SystemExit(f"missing smell report: {smell_path}")
    if not overall_path.exists():
        raise SystemExit(f"missing overall score: {overall_path}")
    if args.require_bugfix and not bugfix_path.exists():
        raise SystemExit(f"missing bugfix grades: {bugfix_path}")

    smell_report = load_json(smell_path)
    overall_score = load_json(overall_path)
    bugfix_data = load_json(bugfix_path) if bugfix_path.exists() else None

    project_path = (
        overall_score.get("project_path")
        or smell_report.get("project_path")
        or (bugfix_data.get("project_path") if isinstance(bugfix_data, dict) else None)
        or ""
    )

    dim_scores = overall_score.get("dimension_scores")
    if not isinstance(dim_scores, dict):
        dim_scores = {}

    dim_scores_review = {to_review_dimension_key(k): v for k, v in dim_scores.items()}

    artifacts = {
        "overall_score": build_artifact_ref(score_dir, overall_path),
        "smell_report": build_artifact_ref(score_dir, smell_path),
    }
    if bugfix_path.exists():
        artifacts["bugfix_grades"] = build_artifact_ref(score_dir, bugfix_path)

    handoff = {
        "schema_version": SCHEMA_VERSION,
        "producer_skill": PRODUCER_SKILL,
        "generated_at": datetime.now().isoformat(),
        "project_path": project_path,
        "score_dir": str(score_dir),
        "artifacts": artifacts,
        "overall": {
            "score": overall_score.get("score"),
            "weighted_score": overall_score.get("weighted_score"),
            "grade": overall_score.get("grade"),
            "dimension_scores": dim_scores,
            "dimension_scores_review": dim_scores_review,
        },
        "smell_summary": smell_report.get("summary", {}),
        "bugfix_summary": (
            bugfix_data.get("summary", {}) if isinstance(bugfix_data, dict) else None
        ),
        "top_violations": pick_top_violations(smell_report, args.top_violations),
        "source_schema_versions": {
            "smell_report": smell_report.get("schema_version"),
            "overall_score": overall_score.get("schema_version"),
            "bugfix_grades": (
                bugfix_data.get("schema_version")
                if isinstance(bugfix_data, dict)
                else None
            ),
        },
    }

    out_path = (
        Path(args.output)
        if args.output
        else (score_dir / "handoff" / "review-input.json")
    )
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(
        json.dumps(handoff, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    print(f"✓ 交接产物已生成: {out_path}")


if __name__ == "__main__":
    main()
