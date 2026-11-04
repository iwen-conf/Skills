from __future__ import annotations

import json
import shutil
from pathlib import Path

import pytest
from arc_policy.cli import main
from test_arc_policy import make_repo

ROOT = Path(__file__).resolve().parents[1]


def test_skill_cli_resolves_source_alias_and_pinned_dependency(capsys) -> None:
    assert main(["skills", "--name", "arc:verify"]) == 0
    skill = json.loads(capsys.readouterr().out)
    assert skill["name"] == "arc-verify"
    assert Path(skill["entry"]).is_file()
    assert main(["skills", "--name", "arc-policy:arch-diagram"]) == 0
    diagram = json.loads(capsys.readouterr().out)
    assert diagram["source_check"] == "PASS"
    assert diagram["source"]["commit"] == "879ca7fa503a64ab46b5bc77d2062ded32fadc84"


@pytest.mark.skipif(
    shutil.which("node") is None, reason="Node is required by the upstream renderer"
)
def test_diagram_cli_uses_cwd_without_creating_a_task_or_ledger(
    tmp_path: Path, capsys, monkeypatch
) -> None:
    repo = make_repo(tmp_path)
    sample = repo / "snapshot.json"
    shutil.copyfile(
        ROOT / "tools/arc-verify/workbench/dev/sample-snapshot.json", sample
    )
    before = sample.read_bytes()
    monkeypatch.chdir(tmp_path)

    assert (
        main(
            [
                "--cwd",
                str(repo),
                "diagram",
                "--snapshot",
                "snapshot.json",
                "--out",
                "review",
                "--no-check",
            ]
        )
        == 0
    )
    report = json.loads(capsys.readouterr().out)
    assert report["readiness"]["build"] == "READY"
    assert report["readiness"]["layout_check"] == "NOT_RUN"
    assert report["business_verdict"]["recorded"] == "BLOCKED"
    assert report["business_verdict"]["reverified"] is False
    assert Path(report["artifacts"]["html"]).parent == repo / "review"
    assert not (repo / ".arc").exists()
    assert sample.read_bytes() == before

    assert (
        main(
            [
                "--cwd",
                str(repo),
                "diagram",
                "--snapshot",
                "snapshot.json",
                "--out",
                "review",
                "--no-check",
            ]
        )
        == 1
    )
    assert "already exist" in json.loads(capsys.readouterr().out)["reason"]
