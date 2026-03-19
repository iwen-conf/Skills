import json
import subprocess
import sys
from pathlib import Path


ROOT = Path("/Users/iluwen/Documents/Code/Skills")
SCRIPT = ROOT / "Arc" / "arc:cartography" / "scripts" / "cartographer.py"


def run_script(*args: str, cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        cwd=str(cwd) if cwd else None,
        capture_output=True,
        text=True,
        check=False,
    )


def test_cartographer_init_changes_update_and_export(tmp_path: Path) -> None:
    project = tmp_path / "repo"
    (project / "src").mkdir(parents=True)
    (project / "src" / "main.ts").write_text("export const ok = true;\n", encoding="utf-8")
    (project / "src" / "main.test.ts").write_text("test('ok', () => {});\n", encoding="utf-8")
    (project / ".gitignore").write_text("node_modules/\n", encoding="utf-8")

    init = run_script(
        "init",
        "--root",
        str(project),
        "--include",
        "src/**/*.ts",
        "--exclude",
        "**/*.test.ts",
    )
    assert init.returncode == 0, init.stderr
    assert (project / ".slim" / "cartography.json").exists()
    assert (project / "src" / "codemap.md").exists()

    (project / "src" / "main.ts").write_text("export const ok = false;\n", encoding="utf-8")
    changes = run_script("changes", "--root", str(project))
    assert changes.returncode == 0, changes.stderr
    assert "modified" in changes.stdout
    assert "src/main.ts" in changes.stdout

    update = run_script("update", "--root", str(project))
    assert update.returncode == 0, update.stderr

    export_path = project / "codemap.json"
    export = run_script("export", "--root", str(project), "--tier", "2", "--output", str(export_path))
    assert export.returncode == 0, export.stderr
    payload = json.loads(export_path.read_text(encoding="utf-8"))
    assert payload["tier"] == 2
    assert any(entry["path"] == "src/main.ts" for entry in payload["entries"])
