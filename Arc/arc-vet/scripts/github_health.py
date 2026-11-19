#!/usr/bin/env python3
"""Maintenance and license precheck for GitHub repositories (arc:vet).

Reads the public GitHub REST API and prints JSON signals with warning codes. It is a
precheck: it cannot judge whether commits are substantive, license obligations, or
whether an advisory is exploitable, and an empty result never means "safe".

Python 3 standard library only. GITHUB_TOKEN or GH_TOKEN raises the rate limit.
Exit status: 0 all repositories fetched, 1 any repository failed, 2 usage error.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

API = "https://api.github.com/"
COMMIT_SAMPLE = 50
RELEASE_SAMPLE = 20
DAYS_PER_MONTH = 365.2425 / 12
BOT_MARKERS = ("[bot]", "dependabot", "renovate", "github-actions", "pre-commit-ci")
DEPRECATION_WORDS = ("deprecated", "unmaintained", "no longer maintained", "archived", "superseded", "moved to", "maintained at")
UNKNOWN_LICENSES = {"", "NOASSERTION", "OTHER"}


def fetch_json(path: str) -> Any:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "arc-vet-github-health/1.0",
        "X-GitHub-Api-Version": "2022-11-28",
    }
    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    url = API + path
    try:
        with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=20) as response:
            return json.load(response)
    except urllib.error.HTTPError as exc:
        detail = f"HTTP {exc.code} for {url}"
        if exc.code in (403, 429):
            detail += " (rate limited or forbidden; export GITHUB_TOKEN, for example from `gh auth token`)"
        raise RuntimeError(detail) from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"connection failed for {url}: {exc.reason}") from exc


def parse_time(value: str | None) -> dt.datetime | None:
    if not value:
        return None
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


def months_between(earlier: dt.datetime, later: dt.datetime) -> float:
    return max(0.0, (later - earlier).total_seconds() / 86400 / DAYS_PER_MONTH)


def is_bot(commit: dict[str, Any]) -> bool:
    author = commit.get("author") or {}
    if author.get("type") == "Bot":
        return True
    names = (
        author.get("login"),
        (commit.get("committer") or {}).get("login"),
        ((commit.get("commit") or {}).get("author") or {}).get("name"),
    )
    return any(marker in (name or "").lower() for name in names for marker in BOT_MARKERS)


def maintenance_tier(months: float | None) -> str:
    if months is None:
        return "unverified"
    if months >= 36:
        return "reject-36m"
    if months > 24:
        return "high-risk-24m"
    if months > 12:
        return "watch-12m"
    return "active"


def assess(
    name: str,
    repo: dict[str, Any],
    commits: list[dict[str, Any]],
    releases: list[dict[str, Any]],
    now: dt.datetime,
) -> dict[str, Any]:
    """Turn raw API documents into signals. Pure: no network, no clock."""
    human = []
    for commit in commits:
        date = parse_time(((commit.get("commit") or {}).get("committer") or {}).get("date"))
        if date and not is_bot(commit):
            human.append((date, (commit.get("sha") or "")[:8]))
    human.sort(reverse=True)
    last_human = human[0] if human else None
    human_months = months_between(last_human[0], now) if last_human else None

    stable = [
        (parse_time(item["published_at"]), item.get("tag_name"))
        for item in releases
        if item.get("published_at") and not item.get("draft") and not item.get("prerelease")
    ]
    stable.sort(reverse=True)
    latest = stable[0] if stable else None
    release_months = months_between(latest[0], now) if latest else None

    license_id = (repo.get("license") or {}).get("spdx_id") or ""
    tier = maintenance_tier(human_months)
    warnings = []
    if repo.get("archived") or repo.get("disabled"):
        warnings.append("ARCHIVED_OR_DISABLED: reject as a new direct dependency; look for a successor")
    description = (repo.get("description") or "").lower()
    if any(word in description for word in DEPRECATION_WORDS):
        warnings.append("DEPRECATION_HINT: description suggests deprecation; confirm and find the successor")
    if license_id in UNKNOWN_LICENSES:
        warnings.append("LICENSE_UNKNOWN: no adoption or code copying until the license is confirmed")
    tier_warnings = {
        "unverified": f"HUMAN_ACTIVITY_NOT_VERIFIED: no non-bot commit in the last {len(commits)} commits",
        "reject-36m": "STALE_36M: default reject unless an evidenced exception is recorded",
        "high-risk-24m": "STALE_24M: high maintenance risk",
        "watch-12m": "STALE_12M: confirm maintainer response and compatibility",
    }
    if tier in tier_warnings:
        warnings.append(tier_warnings[tier])
    if release_months is None:
        warnings.append("NO_STABLE_RELEASE: check tags and the registry's published versions")
    elif release_months > 24:
        warnings.append("RELEASE_OLDER_THAN_24M: confirm the project still ships")
    warnings.append("MANUAL_CHECK_REQUIRED: core-path diffs, issues, advisories, license compatibility, transitive deps")

    def stamp(value: dt.datetime) -> str:
        return value.astimezone(dt.timezone.utc).isoformat(timespec="seconds")

    return {
        "repository": repo.get("full_name") or name,
        "url": repo.get("html_url"),
        "as_of": stamp(now),
        "archived": bool(repo.get("archived")),
        "license_spdx_github_metadata": license_id or "UNKNOWN",
        "pushed_at": repo.get("pushed_at"),
        "last_nonbot_commit": {"sha": last_human[1], "date": stamp(last_human[0])} if last_human else None,
        "months_since_nonbot_commit": round(human_months, 1) if human_months is not None else None,
        "commit_sample_size": len(commits),
        "maintenance_tier": tier,
        "last_stable_release": {"tag": latest[1], "date": stamp(latest[0])} if latest else None,
        "months_since_stable_release": round(release_months, 1) if release_months is not None else None,
        "warnings": warnings,
    }


def check(name: str, now: dt.datetime) -> dict[str, Any]:
    owner, sep, repo_name = name.partition("/")
    if not sep or not owner or not repo_name or "/" in repo_name:
        raise ValueError(f"expected OWNER/REPO, got {name!r}")
    path = f"repos/{urllib.parse.quote(owner, safe='')}/{urllib.parse.quote(repo_name, safe='')}"
    repo = fetch_json(path)
    commits = fetch_json(f"{path}/commits?per_page={COMMIT_SAMPLE}")
    releases = fetch_json(f"{path}/releases?per_page={RELEASE_SAMPLE}")
    if not isinstance(repo, dict) or not isinstance(commits, list) or not isinstance(releases, list):
        raise RuntimeError(f"unexpected API response shape for {name}")
    return assess(name, repo, commits, releases, now)


def as_of_time(value: str | None) -> dt.datetime:
    if value is None:
        return dt.datetime.now(dt.timezone.utc)
    day = dt.date.fromisoformat(value)
    return dt.datetime(day.year, day.month, day.day, 23, 59, 59, tzinfo=dt.timezone.utc)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("repositories", nargs="+", metavar="OWNER/REPO")
    parser.add_argument("--as-of", metavar="YYYY-MM-DD", help="check date for the month counts (default: now, UTC)")
    args = parser.parse_args(argv)
    try:
        now = as_of_time(args.as_of)
    except ValueError:
        parser.error(f"--as-of must be YYYY-MM-DD, got {args.as_of!r}")
    results: list[dict[str, Any]] = []
    failed = False
    for name in args.repositories:
        try:
            results.append(check(name, now))
        except (ValueError, RuntimeError) as exc:
            failed = True
            results.append({"repository": name, "as_of": now.isoformat(timespec="seconds"), "error": str(exc)})
    print(json.dumps(results, indent=2, ensure_ascii=False))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
