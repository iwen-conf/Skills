from __future__ import annotations

import json
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path


def _utc_iso() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def _write_json(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def _run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    p = subprocess.run(cmd, text=True, capture_output=True, check=False)
    if p.returncode != 0:
        raise RuntimeError(
            f"command failed: {' '.join(cmd)}\nstdout:\n{p.stdout}\nstderr:\n{p.stderr}\n"
        )
    return p


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    generate_review_handoff = repo_root / "score" / "scripts" / "generate_review_handoff.py"
    validate_score_artifacts = repo_root / "score" / "scripts" / "validate_score_artifacts.py"
    integrate_score = repo_root / "review" / "scripts" / "integrate_score.py"
    check_gate = repo_root / "Arc" / "arc:gate" / "scripts" / "check_gate.py"

    for p in [generate_review_handoff, validate_score_artifacts, integrate_score, check_gate]:
        if not p.exists():
            raise RuntimeError(f"missing script: {p}")

    with tempfile.TemporaryDirectory(prefix="arc-integration-smoke-") as td:
        project_root = Path(td) / "demo-project"
        project_root.mkdir(parents=True, exist_ok=True)

        project_name = project_root.name
        score_dir = project_root / ".arc" / "score" / project_name
        review_dir = project_root / ".arc" / "review" / project_name

        overall_path = score_dir / "score" / "overall-score.json"
        smell_path = score_dir / "analysis" / "smell-report.json"
        bugfix_path = score_dir / "analysis" / "bugfix-grades.json"

        now_iso = _utc_iso()
        project_path_str = str(project_root)

        _write_json(
            overall_path,
            {
                "schema_version": "1.0.0",
                "producer_skill": "arc-score",
                "score": 95.0,
                "weighted_score": 92.0,
                "grade": "A",
                "dimension_scores": {"security": 100, "code_quality": 95, "tech_debt": 80},
                "by_severity": {"critical": 0, "high": 0},
                "critical_issues": [],
                "calculated_at": now_iso,
                "violation_count": 1,
                "project_path": project_path_str,
            },
        )

        _write_json(
            smell_path,
            {
                "schema_version": "1.0.0",
                "producer_skill": "arc-score",
                "summary": {
                    "total_violations": 1,
                    "by_severity": {"low": 1},
                    "by_category": {"code_quality": 1},
                    "files_scanned": 1,
                },
                "violations": [
                    {
                        "id": "test-violation",
                        "category": "code_quality",
                        "name": "TestViolation",
                        "severity": "low",
                        "file": "src/foo.py",
                        "line": 10,
                        "message": "dummy",
                        "suggestion": "fix it",
                    }
                ],
                "scan_timestamp": now_iso,
                "project_path": project_path_str,
            },
        )

        _write_json(
            bugfix_path,
            {
                "schema_version": "1.0.0",
                "producer_skill": "arc-score",
                "summary": {
                    "total_commits": 0,
                    "by_grade": {"A": 0, "B": 0, "C": 0},
                    "by_tag": {},
                    "total_lines_changed": 0,
                },
                "commits": [],
                "analysis_timestamp": now_iso,
                "project_path": project_path_str,
                "branch": "main",
            },
        )

        _run([sys.executable, str(generate_review_handoff), "--score-dir", str(score_dir)])

        handoff_path = score_dir / "handoff" / "review-input.json"
        if not handoff_path.exists():
            raise RuntimeError("expected handoff/review-input.json to be generated")

        context_hub_index = project_root / ".arc" / "context-hub" / "index.json"
        _write_json(
            context_hub_index,
            {
                "generated_at": now_iso,
                "artifacts": [
                    {
                        "producer_skill": "arc-score",
                        "path": str(handoff_path.relative_to(project_root)),
                        "expires_at": "2099-01-01T00:00:00Z",
                    }
                ],
            },
        )

        _run([sys.executable, str(validate_score_artifacts), "--score-dir", str(score_dir), "--require-bugfix", "--require-review-handoff"])

        _run([sys.executable, str(integrate_score), "--project-path", str(project_root), "--review-dir", str(review_dir)])

        quantitative_input_path = review_dir / "quantitative-input.json"
        quantitative_section_path = review_dir / "quantitative-section.md"
        if not quantitative_input_path.exists():
            raise RuntimeError("expected quantitative-input.json")
        if not quantitative_section_path.exists():
            raise RuntimeError("expected quantitative-section.md")

        quantitative_input = json.loads(quantitative_input_path.read_text(encoding="utf-8"))
        if quantitative_input.get("integration_version") != "1.0.0":
            raise RuntimeError("unexpected integration_version")
        if quantitative_input.get("quantitative_grade") != "A":
            raise RuntimeError("unexpected quantitative_grade")

        _run([sys.executable, str(check_gate), "--project", str(project_root), "--exit-code"])

        gate_report_dir = project_root / ".arc" / "gate-reports"
        if not (gate_report_dir / "gate-result.json").exists():
            raise RuntimeError("expected gate-result.json")
        if not (gate_report_dir / "summary.md").exists():
            raise RuntimeError("expected summary.md")

    print("OK: integration smoke test passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
