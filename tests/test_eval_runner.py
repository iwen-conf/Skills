from pathlib import Path

from arc_core.eval_runner import EvalRunner


ROOT = Path("/Users/iluwen/Documents/Code/Skills")


def test_resolve_skill_dir_prefers_real_arc_namespace_path() -> None:
    runner = EvalRunner(ROOT)
    resolved = runner.resolve_skill_dir("arc:e2e")
    assert resolved == ROOT / "arc:e2e"


def test_find_evals_locates_arc_namespaced_skill_file() -> None:
    runner = EvalRunner(ROOT)
    evals_path = runner.find_evals("arc:e2e")
    assert evals_path == ROOT / "arc:e2e/evals.json"


def test_stage_eval_workspace_copies_existing_fixture_tree() -> None:
    runner = EvalRunner(ROOT)
    skill_dir = ROOT / "arc:e2e"
    eval_def = {
        "id": "compile_then_gate_minimal_run",
        "inputs": {
            "fixtures": [
                {
                    "src": "arc:e2e/evals/fixtures/minimal_run",
                    "dest": "reports/minimal_run",
                }
            ]
        },
    }
    workspace_root = ROOT / ".arc" / "tmp-eval-runner-test"
    if workspace_root.exists():
        import shutil

        shutil.rmtree(workspace_root)
    workspace_root.mkdir(parents=True, exist_ok=True)
    try:
        work_dir = runner.stage_eval_workspace(skill_dir, eval_def, workspace_root)
        assert (work_dir / "reports/minimal_run/report.md").exists()
        assert (work_dir / "reports/minimal_run/events.jsonl").exists()
    finally:
        import shutil

        if workspace_root.exists():
            shutil.rmtree(workspace_root)
