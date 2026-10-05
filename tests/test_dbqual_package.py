import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKAGE = ROOT / "Arc" / "arc:verify" / "scripts" / "dbqual"


def test_dbqual_package_unit_suite_passes() -> None:
    result = subprocess.run(
        [sys.executable, "-m", "unittest", "discover", "-s", "tests"],
        cwd=PACKAGE,
        capture_output=True,
        text=True,
        timeout=120,
    )
    assert result.returncode == 0, result.stdout + result.stderr


def test_dbqual_package_has_no_nested_entrypoint() -> None:
    assert not (PACKAGE / "SKILL.md").exists()
    manifest = (PACKAGE / "MANIFEST.txt").read_text(encoding="utf-8").splitlines()
    assert "SKILL.md" not in manifest
    assert "README.zh-CN.md" not in manifest
