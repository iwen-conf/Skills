import json
import subprocess
import sys
from pathlib import Path


ROOT = Path("/Users/iluwen/Documents/Code/Skills")
SCRIPT = ROOT / "Arc" / "arc:context" / "scripts" / "scaffold_context_session.py"


def test_scaffold_context_session_creates_expected_artifacts(tmp_path: Path) -> None:
    project_path = tmp_path / "project"
    context_hub_dir = project_path / ".arc" / "context-hub"
    context_hub_dir.mkdir(parents=True, exist_ok=True)
    (context_hub_dir / "index.json").write_text(
        json.dumps(
            {
                "generated_at": "2026-03-13T00:00:00Z",
                "artifacts": [
                    {
                        "name": "skills.index",
                        "path": "skills.index.json",
                        "artifact_type": "skills-registry",
                        "producer_skill": "arc-registry",
                        "generated_at": "2026-03-13T00:00:00Z",
                        "expires_at": "2099-01-01T00:00:00Z",
                    }
                ],
            },
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(SCRIPT),
            "--project-path",
            str(project_path),
            "--task-name",
            "resume-auth",
            "--mode",
            "restore",
            "--objective",
            "Continue auth migration",
            "--entrypoint",
            str(project_path / "src" / "auth.ts"),
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    case_dir = project_path / ".arc" / "context" / "resume-auth"
    assert (case_dir / "context" / "context-brief.md").exists()
    assert (case_dir / "context" / "working-set.md").exists()
    assert (case_dir / "restore" / "restore-checklist.md").exists()
    manifest = json.loads((case_dir / "restore" / "recovery-manifest.json").read_text(encoding="utf-8"))
    assert manifest["mode"] == "restore"
    assert manifest["task_name"] == "resume-auth"
    assert manifest["entrypoints"] == ["src/auth.ts"]
    assert manifest["context_hub_artifacts"][0]["status"] == "fresh"
