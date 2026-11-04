from __future__ import annotations

import hashlib
import json
import shutil
from pathlib import Path

import pytest
from arc_policy.capabilities import (
    check_pinned_skill,
    resolve_skill,
    skill_catalog,
)

PLUGIN = Path(__file__).resolve().parents[1] / "plugins/arc-policy"


def test_inventory_resolves_actual_bundled_skills_without_host_path_guesses() -> None:
    catalog = skill_catalog(PLUGIN)
    assert {item["name"] for item in catalog} == {
        "arc-policy",
        "arc-explore",
        "arc-plan",
        "arc-code",
        "arc-verify",
        "arc-audit",
        "arc-vet",
        "arch-diagram",
    }
    verify = resolve_skill("arc:verify", PLUGIN)
    assert verify == resolve_skill("arc-verify", PLUGIN)
    assert Path(verify["entry"]).is_file()
    assert verify["version"] == "3.8.1"
    diagram = resolve_skill("arc-policy:arch-diagram", PLUGIN)
    assert diagram["version"] == "1.2.3"
    assert (
        diagram["source"]["repository"]
        == "https://github.com/bassnova42/arch-diagram-skill"
    )


@pytest.mark.parametrize(
    "name", ["../arc-code", "/tmp/arc-code", "ui-design", "another:arc-code"]
)
def test_unavailable_skill_does_not_resolve_to_an_invented_installation(
    name: str,
) -> None:
    with pytest.raises(ValueError, match="not bundled"):
        resolve_skill(name, PLUGIN)


def test_pinned_renderer_bytes_are_verified_before_use(tmp_path: Path) -> None:
    skill = tmp_path / "arch-diagram"
    shutil.copytree(PLUGIN / "skills/arch-diagram", skill)
    provenance = check_pinned_skill(skill)
    assert provenance["commit"] == "879ca7fa503a64ab46b5bc77d2062ded32fadc84"
    (skill / "scripts/build.mjs").write_text("throw Error('changed renderer')\n")
    with pytest.raises(ValueError, match="checksum"):
        check_pinned_skill(skill)


def test_pin_manifest_cannot_point_outside_its_skill(tmp_path: Path) -> None:
    skill = tmp_path / "arch-diagram"
    shutil.copytree(PLUGIN / "skills/arch-diagram", skill)
    path = skill / "source.json"
    data = json.loads(path.read_text())
    data["files"][0]["path"] = "../outside"
    path.write_text(json.dumps(data))
    with pytest.raises(ValueError, match="source manifest checksum"):
        check_pinned_skill(skill)


@pytest.mark.parametrize(
    "change", ["self-repinned-asset", "omitted-asset", "spoofed-identity"]
)
def test_self_consistent_changes_cannot_replace_reviewed_upstream_pin(
    tmp_path: Path, change: str
) -> None:
    skill = tmp_path / "arch-diagram"
    shutil.copytree(PLUGIN / "skills/arch-diagram", skill)
    path = skill / "source.json"
    data = json.loads(path.read_text())
    renderer = skill / "assets/engine.js"
    if change == "self-repinned-asset":
        body = b"// renderer changed without review of a new upstream commit\n"
        renderer.write_bytes(body)
        item = next(
            item for item in data["files"] if item["path"] == "assets/engine.js"
        )
        item["sha256"] = hashlib.sha256(body).hexdigest()
        item["git_blob_sha"] = hashlib.sha1(
            b"blob " + str(len(body)).encode("ascii") + b"\0" + body
        ).hexdigest()
    elif change == "omitted-asset":
        renderer.unlink()
        data["files"] = [
            item for item in data["files"] if item["path"] != "assets/engine.js"
        ]
    else:
        data["commit"] = "0" * 40
        data["version"] = "99.0.0"
    path.write_text(json.dumps(data))
    with pytest.raises(ValueError, match="source manifest checksum"):
        check_pinned_skill(skill)


def test_unlisted_pinned_script_is_rejected(tmp_path: Path) -> None:
    skill = tmp_path / "arch-diagram"
    shutil.copytree(PLUGIN / "skills/arch-diagram", skill)
    (skill / "scripts/unrecorded.mjs").write_text("// not from reviewed upstream\n")
    with pytest.raises(ValueError, match="unrecorded"):
        check_pinned_skill(skill)
