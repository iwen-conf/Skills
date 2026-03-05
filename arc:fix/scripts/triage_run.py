#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class Artifacts:
    report_md: Path | None
    accounts_jsonc: Path | None
    action_log_md: Path | None
    screenshot_manifest_md: Path | None
    events_jsonl: Path | None
    failures_dir: Path | None
    screenshots_dir: Path | None


def _fail(msg: str, *, code: int = 2) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(code)


def _read_text(path: Path, *, max_bytes: int = 2_000_000) -> str:
    data = path.read_bytes()
    if len(data) > max_bytes:
        data = data[:max_bytes]
    return data.decode("utf-8", errors="replace")


def _strip_jsonc(text: str) -> str:
    out: list[str] = []
    i = 0
    in_string = False
    escape = False
    in_line_comment = False
    in_block_comment = False

    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""

        if in_line_comment:
            if ch in "\r\n":
                in_line_comment = False
                out.append(ch)
            i += 1
            continue

        if in_block_comment:
            if ch == "*" and nxt == "/":
                in_block_comment = False
                i += 2
            else:
                i += 1
            continue

        if in_string:
            out.append(ch)
            if escape:
                escape = False
            else:
                if ch == "\\":
                    escape = True
                elif ch == '"':
                    in_string = False
            i += 1
            continue

        if ch == '"':
            in_string = True
            out.append(ch)
            i += 1
            continue

        if ch == "/" and nxt == "/":
            in_line_comment = True
            i += 2
            continue

        if ch == "/" and nxt == "*":
            in_block_comment = True
            i += 2
            continue

        out.append(ch)
        i += 1

    return "".join(out)


def _discover_artifacts(run_dir: Path) -> Artifacts:
    report_md = run_dir / "report.md"
    accounts_jsonc = run_dir / "accounts.jsonc"
    action_log_md = run_dir / "action-log.md"
    screenshot_manifest_md = run_dir / "screenshot-manifest.md"
    events_jsonl = run_dir / "events.jsonl"
    failures_dir = run_dir / "failures"
    screenshots_dir = run_dir / "screenshots"

    return Artifacts(
        report_md=report_md if report_md.is_file() else None,
        accounts_jsonc=accounts_jsonc if accounts_jsonc.is_file() else None,
        action_log_md=action_log_md if action_log_md.is_file() else None,
        screenshot_manifest_md=screenshot_manifest_md if screenshot_manifest_md.is_file() else None,
        events_jsonl=events_jsonl if events_jsonl.is_file() else None,
        failures_dir=failures_dir if failures_dir.is_dir() else None,
        screenshots_dir=screenshots_dir if screenshots_dir.is_dir() else None,
    )


def _extract_simple_kv(text: str) -> dict[str, str]:
    kv: dict[str, str] = {}
    patterns: dict[str, re.Pattern[str]] = {
        "run_id": re.compile(r"(?im)^(?:\\*\\*\\s*)?Run ID(?:\\s*\\*\\*)?\\s*:\\s*`?([^`\\n]+)`?\\s*$"),
        "objective": re.compile(r"(?im)^(?:\\*\\*\\s*)?Objective(?:\\s*\\*\\*)?\\s*:\\s*`?([^`\\n]+)`?\\s*$"),
        "target_url": re.compile(r"(?im)^(?:\\*\\*\\s*)?Target URL(?:\\s*\\*\\*)?\\s*:\\s*`?([^`\\n]+)`?\\s*$"),
        "result": re.compile(r"(?im)^(?:\\*\\*\\s*)?Result(?:\\s*\\*\\*)?\\s*:\\s*`?([^`\\n]+)`?\\s*$"),
    }
    for k, pat in patterns.items():
        m = pat.search(text)
        if m:
            kv[k] = m.group(1).strip()
    return kv


def _parse_markdown_table(text: str) -> tuple[list[str], list[list[str]]]:
    lines = [ln.rstrip() for ln in text.splitlines()]
    for i, line in enumerate(lines):
        if not line.strip().startswith("|"):
            continue
        header = [c.strip() for c in line.strip().strip("|").split("|")]
        if not header or not any("step" in c.lower() for c in header):
            continue
        if i + 1 >= len(lines):
            continue
        sep = lines[i + 1].strip()
        if not (sep.startswith("|") and "-" in sep):
            continue
        rows: list[list[str]] = []
        for j in range(i + 2, len(lines)):
            row_line = lines[j].strip()
            if not row_line.startswith("|"):
                break
            row = [c.strip() for c in row_line.strip("|").split("|")]
            if len(row) != len(header):
                break
            rows.append(row)
        return header, rows
    return [], []


def _extract_failing_steps_from_report(report_text: str) -> list[dict[str, Any]]:
    header, rows = _parse_markdown_table(report_text)
    if not header:
        return []
    col_index: dict[str, int] = {name.lower(): idx for idx, name in enumerate(header)}

    def find_col(*names: str) -> int | None:
        for n in names:
            if n.lower() in col_index:
                return col_index[n.lower()]
        return None

    step_col = find_col("step")
    role_col = find_col("role")
    action_col = find_col("action")
    expected_col = find_col("expected")
    actual_col = find_col("actual")
    evidence_col = find_col("evidence")
    result_col = find_col("result")

    failing: list[dict[str, Any]] = []
    for row in rows:
        result = row[result_col].upper() if result_col is not None else ""
        if "FAIL" not in result:
            continue
        step = row[step_col] if step_col is not None else ""
        failing.append(
            {
                "step": step,
                "role": row[role_col] if role_col is not None else "",
                "action": row[action_col] if action_col is not None else "",
                "expected": row[expected_col] if expected_col is not None else "",
                "actual": row[actual_col] if actual_col is not None else "",
                "evidence": row[evidence_col] if evidence_col is not None else "",
                "result": result,
            }
        )
    return failing


def _extract_failure_files(failures_dir: Path) -> list[dict[str, str]]:
    failures: list[dict[str, str]] = []
    for path in sorted(failures_dir.glob("*.md")):
        text = _read_text(path, max_bytes=200_000)
        title = ""
        for line in text.splitlines():
            if line.startswith("#"):
                title = line.lstrip("#").strip()
                break
        step = ""
        for pat in [r"\\bSTEP\\s*(\\d{4})\\b", r"\\bstep\\s*(\\d{4})\\b", r"\\b(\\d{4})\\b"]:
            m = re.search(pat, text)
            if m:
                step = m.group(1)
                break
        failures.append({"file": str(path), "title": title, "step": step})
    return failures


def _extract_screenshot_refs(text: str) -> list[str]:
    refs = sorted(set(re.findall(r"(screenshots/[^\\s)]+?\\.png)", text)))
    return refs


def _git_info(cwd: Path) -> dict[str, Any] | None:
    try:
        root = subprocess.run(
            ["git", "-C", str(cwd), "rev-parse", "--show-toplevel"],
            text=True,
            capture_output=True,
            check=True,
        ).stdout.strip()
        head = subprocess.run(["git", "-C", str(cwd), "rev-parse", "HEAD"], text=True, capture_output=True, check=True).stdout.strip()
        status = subprocess.run(["git", "-C", str(cwd), "status", "--porcelain"], text=True, capture_output=True, check=True).stdout.splitlines()
        return {"root": root, "head": head, "dirty": bool(status), "status": status}
    except Exception:  # noqa: BLE001
        return None


def _to_markdown(data: dict[str, Any]) -> str:
    lines: list[str] = []
    lines.append("# UI/UX Simulation Triage")
    lines.append("")
    lines.append("## Run")
    lines.append(f"- run_dir: `{data['run_dir']}`")
    lines.append(f"- run_id: `{data['run_id']}`")
    report = data.get("report") or {}
    if report.get("result"):
        lines.append(f"- result: `{report.get('result')}`")
    if report.get("objective"):
        lines.append(f"- objective: {report.get('objective')}")
    if report.get("target_url"):
        lines.append(f"- target_url: {report.get('target_url')}")
    lines.append("")

    lines.append("## Artifacts")
    artifacts = data.get("artifacts") or {}
    for k in ["accounts.jsonc", "report.md", "action-log.md", "screenshot-manifest.md", "events.jsonl", "failures/", "screenshots/"]:
        lines.append(f"- {k}: {'OK' if artifacts.get(k) else 'MISSING'}")
    lines.append("")

    accounts = data.get("accounts") or {}
    if accounts:
        lines.append("## Accounts (accounts.jsonc)")
        if accounts.get("error"):
            lines.append(f"- parse_error: {accounts.get('error')}")
        else:
            lines.append(f"- count: {accounts.get('count')}")
            roles = accounts.get("roles") or []
            users = accounts.get("usernames") or []
            created = accounts.get("created_for_verification") or []
            if roles:
                lines.append(f"- roles: {', '.join(roles)}")
            if users:
                lines.append(f"- usernames: {', '.join(users)}")
            if created:
                lines.append(f"- created_for_verification: {', '.join(created)}")
        lines.append("")

    failing_steps = data.get("failing_steps") or []
    lines.append("## Failing Steps (best-effort)")
    if not failing_steps:
        lines.append("- (No table rows with FAIL parsed from report.md)")
    else:
        lines.append("| Step | Role | Action | Expected | Actual | Evidence |")
        lines.append("|------|------|--------|----------|--------|----------|")
        for row in failing_steps[:20]:
            lines.append(
                "| {step} | {role} | {action} | {expected} | {actual} | {evidence} |".format(
                    step=str(row.get("step", "")),
                    role=str(row.get("role", "")),
                    action=str(row.get("action", "")),
                    expected=str(row.get("expected", "")),
                    actual=str(row.get("actual", "")),
                    evidence=str(row.get("evidence", "")),
                )
            )
    lines.append("")

    failures = data.get("failures") or []
    lines.append("## Failures/*.md")
    if not failures:
        lines.append("- (No failure markdown found)")
    else:
        for f in failures[:50]:
            title = f.get("title") or ""
            step = f.get("step") or ""
            step_part = f" step={step}" if step else ""
            lines.append(f"- `{f.get('file')}`{step_part}: {title}")
    lines.append("")

    screenshots = data.get("screenshots") or []
    lines.append("## Screenshot References (best-effort)")
    if not screenshots:
        lines.append("- (No screenshot refs parsed)")
    else:
        for s in screenshots[:50]:
            lines.append(f"- `{s}`")
    lines.append("")

    git = data.get("git")
    if git:
        lines.append("## Git")
        lines.append(f"- root: `{git.get('root')}`")
        lines.append(f"- head: `{git.get('head')}`")
        lines.append(f"- dirty: `{git.get('dirty')}`")
        lines.append("")

    lines.append("## Next Suggested Checks")
    lines.append("- Open the failing step screenshot(s) + backend log around the same timestamp.")
    lines.append("- Confirm whether it is (a) selector/text drift, (b) timing/flake, (c) auth/session, (d) backend error, or (e) environment/data issue.")
    lines.append("- If this run was produced after a code change, restart services before rerunning (avoid stale bundles).")
    lines.append("")
    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="triage_run.py",
        description="Best-effort triage summary for a ui-ux-simulation run_dir (reports/<run_id>/...).",
    )
    parser.add_argument("run_dir", help="Path to run directory, e.g. reports/2026-02-01_14-00-00_abcd/")
    parser.add_argument("--json-out", help="Write a machine-readable JSON summary to this file.")
    parser.add_argument("--md-out", help="Write a Markdown summary to this file (default: print to stdout).")
    args = parser.parse_args()

    run_dir = Path(args.run_dir).expanduser().resolve()
    if not run_dir.is_dir():
        _fail(f"run_dir does not exist: {run_dir}")

    artifacts = _discover_artifacts(run_dir)
    report_text = _read_text(artifacts.report_md) if artifacts.report_md else ""
    report_kv = _extract_simple_kv(report_text) if report_text else {}
    failing_steps = _extract_failing_steps_from_report(report_text) if report_text else []

    accounts_summary: dict[str, Any] = {}
    if artifacts.accounts_jsonc:
        try:
            raw = json.loads(_strip_jsonc(_read_text(artifacts.accounts_jsonc, max_bytes=2_000_000)))
            accounts_list = raw.get("accounts")
            if isinstance(accounts_list, list):
                roles = sorted({str(a.get("role", "")).strip() for a in accounts_list if isinstance(a, dict) and a.get("role")})
                usernames = sorted(
                    {str(a.get("username", "")).strip() for a in accounts_list if isinstance(a, dict) and a.get("username")}
                )
                created_for_verification = sorted(
                    {
                        str(a.get("username", "")).strip()
                        for a in accounts_list
                        if isinstance(a, dict) and a.get("created_for_verification") is True and a.get("username")
                    }
                )
                accounts_summary = {
                    "count": len(accounts_list),
                    "roles": roles,
                    "usernames": usernames,
                    "created_for_verification": created_for_verification,
                }
            else:
                accounts_summary = {"error": "missing or invalid 'accounts' array"}
        except Exception as e:  # noqa: BLE001
            accounts_summary = {"error": str(e)}

    failures: list[dict[str, str]] = []
    if artifacts.failures_dir:
        failures = _extract_failure_files(artifacts.failures_dir)

    screenshot_refs: set[str] = set()
    if artifacts.screenshot_manifest_md:
        screenshot_refs.update(_extract_screenshot_refs(_read_text(artifacts.screenshot_manifest_md, max_bytes=500_000)))
    for f in failures:
        try:
            screenshot_refs.update(_extract_screenshot_refs(_read_text(Path(f["file"]), max_bytes=200_000)))
        except Exception:  # noqa: BLE001
            pass

    git = _git_info(run_dir)

    data: dict[str, Any] = {
        "run_dir": str(run_dir),
        "run_id": run_dir.name,
        "artifacts": {
            "accounts.jsonc": bool(artifacts.accounts_jsonc),
            "report.md": bool(artifacts.report_md),
            "action-log.md": bool(artifacts.action_log_md),
            "screenshot-manifest.md": bool(artifacts.screenshot_manifest_md),
            "events.jsonl": bool(artifacts.events_jsonl),
            "failures/": bool(artifacts.failures_dir),
            "screenshots/": bool(artifacts.screenshots_dir),
        },
        "report": report_kv,
        "accounts": accounts_summary,
        "failing_steps": failing_steps,
        "failures": failures,
        "screenshots": sorted(screenshot_refs),
        "git": git,
    }

    if args.json_out:
        out_path = Path(args.json_out).expanduser()
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")

    md = _to_markdown(data)
    if args.md_out:
        out_path = Path(args.md_out).expanduser()
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(md, encoding="utf-8")
    else:
        print(md)


if __name__ == "__main__":
    main()
