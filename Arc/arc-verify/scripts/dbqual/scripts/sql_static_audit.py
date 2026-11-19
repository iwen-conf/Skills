#!/usr/bin/env python3
"""Static SQL gate: dialect parse and lint with SQLFluff, migration lock safety with Squawk.

A clean parse proves the text is valid in the configured dialect. It does not prove that tables
and columns exist or that results are right: schema-bound EXPLAIN (query gate), property tests and
migration rehearsal stay required. Nothing checked is BLOCKED, never PASS.
"""

from __future__ import annotations

import argparse
import glob
import json
import subprocess
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, safe_relpath, sha256_file
from sqlverify_config import load_config

# Lexing, parsing and templating errors: the tool never understood the file.
SYNTAX_CODES = {"LXR", "PRS", "TMP"}


def expand(root: Path, patterns: list[str]) -> list[Path]:
    files: set[Path] = set()
    for pattern in patterns:
        for match in glob.glob(str(root / pattern), recursive=True):
            path = Path(match)
            if path.is_file():
                files.add(path.resolve())
    return sorted(files)


def run_tool(argv: list[str], cwd: Path, timeout_s: float) -> tuple[int | None, str, str]:
    try:
        cp = subprocess.run(argv, cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout_s)
    except FileNotFoundError:
        return None, "", f"command not found: {argv[0]}"
    except subprocess.TimeoutExpired:
        return None, "", f"timed out after {timeout_s}s: {argv[0]}"
    return cp.returncode, cp.stdout, cp.stderr


def tool_version(argv: list[str], cwd: Path) -> str | None:
    rc, out, err = run_tool([*argv, "--version"], cwd, 120)
    return (out or err).strip().splitlines()[-1] if rc == 0 and (out or err).strip() else None


def evaluate_sqlfluff(report: list[dict[str, Any]], expected: list[str], fail_on: list[str]) -> dict[str, Any]:
    """report: `sqlfluff lint --format json`; expected: the file paths passed to it, as reported."""
    seen = {str(entry.get("filepath")) for entry in report}
    out: dict[str, Any] = {
        "syntax_errors": [],
        "blocking": [],
        "advisory": {},
        "unreported_files": sorted(set(expected) - seen),
    }
    block_all = "*" in fail_on
    for entry in report:
        for v in entry.get("violations", []):
            code = str(v.get("code"))
            item = {
                "file": entry.get("filepath"),
                "line": v.get("start_line_no"),
                "code": code,
                "rule": v.get("name"),
                "description": v.get("description"),
            }
            if code in SYNTAX_CODES:
                out["syntax_errors"].append(item)
            elif not v.get("warning") and (block_all or code in fail_on):
                out["blocking"].append(item)
            else:
                out["advisory"][code] = out["advisory"].get(code, 0) + 1
    return out


def evaluate_squawk(report: list[dict[str, Any]]) -> dict[str, Any]:
    """report: `squawk --reporter json`; excluded rules never appear in it."""
    out: dict[str, Any] = {"syntax_errors": [], "blocking": []}
    for v in report:
        item = {
            "file": v.get("file"),
            "line": int(v.get("line", 0)) + 1,
            "rule": v.get("rule_name"),
            "message": v.get("message"),
        }
        out["syntax_errors" if v.get("rule_name") == "syntax-error" else "blocking"].append(item)
    return out


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Static SQL parse/lint gate (SQLFluff) and migration lock-safety lint (Squawk)"
    )
    ap.add_argument("--config", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    started = time.monotonic()
    ev = evidence_base("sql_static", "sql.static")
    root = Path(ns.root).resolve()
    policy = load_config(Path(ns.config)).get("sql_static") or {}
    blockers: list[str] = []
    observed: dict[str, Any] = {"dialect": policy.get("dialect"), "fail_on": policy.get("fail_on", []), "tools": {}}
    timeout_s = float(policy.get("timeout_s", 300))

    files = expand(root, list(policy.get("paths", [])))
    rel = [safe_relpath(p, root) for p in files]
    observed["files_checked"] = len(rel)
    ev["reproduction"]["files"] = [{"path": r, "sha256": sha256_file(p)} for r, p in zip(rel, files)]
    if not policy.get("dialect"):
        blockers.append("sql_static.dialect is not set; the dialect is a project decision")
    if not files:
        blockers.append(f"sql_static.paths matched no file under {root}: nothing was checked")
    if not blockers:
        cmd = [str(x) for x in policy.get("sqlfluff_command", ["sqlfluff"])]
        argv = [*cmd, "lint", "--dialect", str(policy["dialect"]), "--format", "json", *rel]
        ev["command"] = argv
        observed["tools"]["sqlfluff"] = tool_version(cmd, root)
        rc, out, err = run_tool(argv, root, timeout_s)
        if rc not in (0, 1):
            blockers.append(f"sqlfluff did not lint (exit {rc}): {err.strip()[:500]}")
        else:
            try:
                result = evaluate_sqlfluff(json.loads(out), rel, [str(c) for c in policy.get("fail_on", [])])
            except (json.JSONDecodeError, AttributeError) as exc:
                blockers.append(f"sqlfluff output is not the JSON report: {exc!r}")
            else:
                observed["sqlfluff"] = result
                if result["unreported_files"]:
                    blockers.append(f"sqlfluff reported nothing for {result['unreported_files']}")
                ev["violations"] += [
                    f"syntax {x['file']}:{x['line']} {x['code']} {x['description']}" for x in result["syntax_errors"]
                ]
                ev["violations"] += [
                    f"{x['file']}:{x['line']} {x['code']} {x['description']}" for x in result["blocking"]
                ]
                ev["warnings"] += [f"advisory {code}: {n}" for code, n in sorted(result["advisory"].items())]

    migrations = list(policy.get("migration_paths", []))
    if migrations:
        mfiles = [safe_relpath(p, root) for p in expand(root, migrations)]
        excluded = {str(k): str(v) for k, v in (policy.get("squawk_excluded_rules") or {}).items()}
        observed["migration_files_checked"] = len(mfiles)
        observed["squawk_excluded_rules"] = excluded
        if not mfiles:
            blockers.append("sql_static.migration_paths matched no file: no migration was checked")
        else:
            cmd = [str(x) for x in policy.get("squawk_command", ["squawk"])]
            argv = [*cmd, "--reporter", "json"]
            if policy.get("pg_version"):
                argv += ["--pg-version", str(policy["pg_version"])]
            if excluded:
                argv += ["--exclude", ",".join(sorted(excluded))]
            argv += mfiles
            ev["reproduction"]["squawk_command"] = argv
            observed["tools"]["squawk"] = tool_version(cmd, root)
            rc, out, err = run_tool(argv, root, timeout_s)
            if rc not in (0, 1):
                blockers.append(f"squawk did not lint (exit {rc}): {err.strip()[:500]}")
            else:
                try:
                    result = evaluate_squawk(json.loads(out) if out.strip() else [])
                except (json.JSONDecodeError, AttributeError, TypeError, ValueError) as exc:
                    blockers.append(f"squawk output is not the JSON report: {exc!r}")
                else:
                    observed["squawk"] = result
                    ev["violations"] += [
                        f"migration syntax {x['file']}:{x['line']} {x['message']}" for x in result["syntax_errors"]
                    ]
                    ev["violations"] += [
                        f"migration {x['file']}:{x['line']} {x['rule']}: {x['message']}" for x in result["blocking"]
                    ]

    observed["blockers"] = blockers
    ev["observed"] = observed
    status = "FAIL" if ev["violations"] else ("BLOCKED" if blockers else "PASS")
    ev["violations"] += blockers
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps({"status": status, "violations": ev["violations"][:20]}, ensure_ascii=False, indent=2))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
