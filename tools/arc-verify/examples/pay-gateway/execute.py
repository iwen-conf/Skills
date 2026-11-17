#!/usr/bin/env python3
"""Execute an imported, confirmed Arc model against its owned pay fixture."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import uuid

import adapter
import bundle


def verify_source_binding(session):
    current = adapter.preflight(session["pay_repo"], session["target"], session["run_id"])
    for key in ("binary_sha256", "database_name", "pg_fingerprint"):
        adapter.require(current["identity"].get(key) == session["identity"].get(key),
                        "source/runtime identity changed since the owned fixture was bound")
    adapter.require(current["psql_path"] == session["psql_path"], "independent SQL executable changed since binding")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--session", required=True)
    parser.add_argument("--bundle", required=True)
    parser.add_argument("--arc", default="arc")
    parser.add_argument("--assets", help="built native workbench directory; exports a real read-only run snapshot")
    args = parser.parse_args()
    try:
        session_file = Path(args.session).resolve()
        session = adapter.read_json(session_file)
        verify_source_binding(session)
        directory = Path(args.bundle).resolve()
        runner = adapter.read_json(directory / "runner.json")
        fixtures = adapter.read_json(directory / "fixtures.json")["fixtures"]
        config = directory / "config.jsonc"
        adapter.require(runner["graph"] == bundle.GRAPH and runner["isolation_scope"] == session["scope"],
                        "runner is not bound to this model/owned fixture")
        env = dict(os.environ)
        env["ARC_PAY_CALLER_KEY"] = adapter.read_json(session_file.parent / "caller-private.json")["value"]
        adapter.require(len(fixtures) == 2 and fixtures[0]["refs"]["principal"] == session["caller_id"],
                        "account binding differs from the observed fixture")
        attempt = directory / ("attempt-" + uuid.uuid4().hex)
        attempt.mkdir(mode=0o700)
        def cli(label, *arguments, allow_failure=False):
            result = subprocess.run([args.arc, *arguments], cwd=session["pay_repo"], env=env,
                                    capture_output=True, text=True, timeout=180)
            try:
                value = json.loads(result.stdout)
            except ValueError:
                value = {"status": "BLOCKED", "reason": "CLI did not return a complete JSON receipt"}
            adapter.save_json(attempt / (label + ".json"), value)
            if result.returncode and not allow_failure:
                raise adapter.Blocked("Arc " + label + " failed; inspect its protected receipt and CLI preconditions (no approval is inferred)")
            return value, result.returncode
        status, _ = cli("database-status", "db", "status", "--repo", session["pay_repo"])
        adapter.require(status.get("active"), "activate the project ledger explicitly with arc db init or arc db migrate first")
        cli("schema", "verify", "init")
        cli("model", "verify", "model", "--graph", bundle.GRAPH)
        cli("fixtures", "verify", "fixtures", "--file", str(directory / "fixtures.json"))
        cli("runner", "verify", "runner", "--file", str(directory / "runner.json"))
        cli("plan", "verify", "plan", "--graph", bundle.GRAPH)
        round_result, _ = cli("round", "verify", "round", "--graph", bundle.GRAPH,
                              "--env", adapter.ENVIRONMENT, "--root", session["pay_repo"],
                              "--config", str(config), "--commit", adapter.REVISION)
        round_id = round_result["id"]
        result, code = cli("execution", "verify", "execute", "--task", bundle.GRAPH + "/" + runner["test"],
                           "--runner", runner["id"], "--round", round_id, "--env", adapter.ENVIRONMENT,
                           "--root", session["pay_repo"], "--config", str(config), "--commit", adapter.REVISION,
                           "--account", fixtures[0]["id"], "--fixture", fixtures[1]["id"], allow_failure=True)
        run_id = result.get("run")
        exports = {}
        if run_id:
            explained, explain_code = cli("explain", "verify", "explain", "--run", run_id, allow_failure=True)
            explain_complete = (explain_code == 0 and explained.get("offline_db_complete") is True
                                and explained.get("run") == run_id and explained.get("graph") == bundle.GRAPH)
            exports["explain"] = "COMPLETE" if explain_complete else "BLOCKED"
            if args.assets:
                _, snapshot_code = cli("snapshot", "verify", "snapshot", "--graph", bundle.GRAPH, "--env", adapter.ENVIRONMENT,
                    "--round", round_id, "--run", run_id, "--config", str(config), "--assets", str(Path(args.assets).resolve()),
                    "--out", str(attempt / "workbench"), allow_failure=True)
                exports["workbench"] = "COMPLETE" if snapshot_code == 0 else "BLOCKED"
        if result.get("verdict") == "PASS":
            try:
                verify_source_binding(session)
                exports["source_binding"] = "COMPLETE"
            except adapter.Blocked:
                exports["source_binding"] = "BLOCKED"
        complete = bool(run_id) and code == 0 and result.get("verdict") == "PASS" and all(value == "COMPLETE" for value in exports.values())
        print(json.dumps({"verdict": result.get("verdict", "BLOCKED"), "run": run_id, "round": round_id,
                          "delivery_status": "COMPLETE" if complete else "BLOCKED", "exports": exports,
                          "evidence": str(attempt), "scope": "owned PostgreSQL + controlled institution; selected scenario only",
                          "complete_business_e2e": False}, ensure_ascii=False))
        return 0 if complete else 2
    except (adapter.Blocked, OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        reason = str(error) if isinstance(error, adapter.Blocked) else "execution input/runtime failure; no passing receipt produced"
        print(json.dumps({"verdict": "BLOCKED", "reason": reason, "complete_business_e2e": False}))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
