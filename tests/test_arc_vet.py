"""arc:vet github_health.py signal rules. Offline: assess() is fed synthetic API documents."""
from __future__ import annotations

import datetime as dt
import importlib.util
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "Arc" / "arc:vet" / "scripts" / "github_health.py"
NOW = dt.datetime(2026, 10, 24, 23, 59, 59, tzinfo=dt.timezone.utc)


def _load():
    spec = importlib.util.spec_from_file_location("arc_vet_github_health", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


health = _load()


def _commit(date: str, login: str = "maintainer", kind: str = "User", sha: str = "abcdef0123") -> dict:
    return {
        "sha": sha,
        "author": {"login": login, "type": kind},
        "committer": {"login": login},
        "commit": {"author": {"name": login}, "committer": {"date": date}},
    }


def _repo(**overrides) -> dict:
    repo = {
        "full_name": "owner/lib",
        "html_url": "https://github.com/owner/lib",
        "archived": False,
        "disabled": False,
        "description": "A library",
        "license": {"spdx_id": "MIT"},
        "pushed_at": "2026-10-01T00:00:00Z",
    }
    repo.update(overrides)
    return repo


RELEASE = [{"tag_name": "v1.2.0", "published_at": "2026-06-01T00:00:00Z", "draft": False, "prerelease": False}]


def _codes(result: dict) -> set[str]:
    return {warning.split(":", 1)[0] for warning in result["warnings"]}


def test_active_repository_passes_time_gates_but_still_requires_manual_check() -> None:
    result = health.assess("owner/lib", _repo(), [_commit("2026-09-01T00:00:00Z")], RELEASE, NOW)

    assert result["maintenance_tier"] == "active"
    assert result["last_stable_release"]["tag"] == "v1.2.0"
    assert _codes(result) == {"MANUAL_CHECK_REQUIRED"}


@pytest.mark.parametrize(
    ("date", "tier", "code"),
    [
        ("2025-06-01T00:00:00Z", "watch-12m", "STALE_12M"),
        ("2024-03-01T00:00:00Z", "high-risk-24m", "STALE_24M"),
        ("2023-10-24T00:00:00Z", "reject-36m", "STALE_36M"),
    ],
)
def test_maintenance_tiers_follow_months_since_last_human_commit(date: str, tier: str, code: str) -> None:
    result = health.assess("owner/lib", _repo(), [_commit(date)], RELEASE, NOW)

    assert result["maintenance_tier"] == tier
    assert code in _codes(result)


def test_bot_commits_do_not_count_as_maintenance() -> None:
    commits = [
        _commit("2026-10-20T00:00:00Z", login="dependabot[bot]", kind="Bot"),
        _commit("2026-10-19T00:00:00Z", login="renovate-runner"),
        _commit("2022-01-01T00:00:00Z", sha="1234567890"),
    ]
    result = health.assess("owner/lib", _repo(), commits, RELEASE, NOW)

    assert result["last_nonbot_commit"]["sha"] == "12345678"
    assert result["maintenance_tier"] == "reject-36m"


def test_bot_only_sample_is_unverified_not_active() -> None:
    commits = [_commit("2026-10-20T00:00:00Z", login="github-actions[bot]", kind="Bot")]
    result = health.assess("owner/lib", _repo(), commits, RELEASE, NOW)

    assert result["maintenance_tier"] == "unverified"
    assert "HUMAN_ACTIVITY_NOT_VERIFIED" in _codes(result)


def test_archive_unknown_license_deprecation_and_release_gaps_are_flagged() -> None:
    repo = _repo(archived=True, license={"spdx_id": "NOASSERTION"}, description="DEPRECATED: use owner/lib2")
    prerelease_only = [{"tag_name": "v2.0.0-rc1", "published_at": "2026-09-01T00:00:00Z", "draft": False, "prerelease": True}]
    result = health.assess("owner/lib", repo, [_commit("2026-09-01T00:00:00Z")], prerelease_only, NOW)

    assert {"ARCHIVED_OR_DISABLED", "LICENSE_UNKNOWN", "DEPRECATION_HINT", "NO_STABLE_RELEASE"} <= _codes(result)

    moved = health.assess("owner/lib", _repo(description="ARCHIVE - now maintained at owner/lib2"), [_commit("2026-09-01T00:00:00Z")], RELEASE, NOW)
    assert "DEPRECATION_HINT" in _codes(moved)

    no_license = health.assess("owner/lib", _repo(license=None), [_commit("2026-09-01T00:00:00Z")], RELEASE, NOW)
    assert no_license["license_spdx_github_metadata"] == "UNKNOWN"


def test_old_stable_release_is_flagged_even_with_active_commits() -> None:
    old = [{"tag_name": "v0.9.0", "published_at": "2023-01-01T00:00:00Z", "draft": False, "prerelease": False}]
    result = health.assess("owner/lib", _repo(), [_commit("2026-10-01T00:00:00Z")], old, NOW)

    assert result["maintenance_tier"] == "active"
    assert "RELEASE_OLDER_THAN_24M" in _codes(result)


def test_malformed_name_fails_without_network(capsys: pytest.CaptureFixture[str]) -> None:
    assert health.main(["--as-of", "2026-10-24", "not-a-repo"]) == 1
    assert "expected OWNER/REPO" in capsys.readouterr().out


def test_invalid_as_of_is_a_usage_error() -> None:
    with pytest.raises(SystemExit) as exc:
        health.main(["--as-of", "24/10/2026", "owner/lib"])
    assert exc.value.code == 2
