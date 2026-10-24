#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

from sqlverify_common import RUN_ID_ENV, begin_run
from sqlverify_config import load_config, validate


def call(cmd: list[str], *, keep_going: bool) -> int:
    print("+", " ".join(cmd), flush=True)
    rc = subprocess.run(cmd).returncode
    if rc != 0 and not keep_going:
        raise SystemExit(rc)
    return rc


def main() -> int:
    ap = argparse.ArgumentParser(description="Execute the industrial SQL verification pipeline")
    ap.add_argument("--config", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--evidence-dir", default=".sqlverify/evidence")
    ap.add_argument("--gate-policy")
    ap.add_argument("--baseline-dir")
    ap.add_argument("--qualification-manifest", help="explicit G0-G31 applicability/evidence manifest; fail-closed coverage check")
    ap.add_argument("--report-dir", default=".sqlverify/report")
    ap.add_argument("--fail-fast", action="store_true")
    ap.add_argument("--ack-isolated-destructive", action="store_true", help="allow configured restore/failover rehearsals that require an isolated-environment acknowledgement")
    ns = ap.parse_args()
    root = Path(ns.root).resolve(); cfg_path = Path(ns.config).resolve(); cfg = load_config(cfg_path)
    errors = validate(cfg, root)
    if errors:
        print(json.dumps({"status":"FAIL","config_errors":errors}, ensure_ascii=False, indent=2)); return 2
    scripts = Path(__file__).resolve().parent
    evdir = Path(ns.evidence_dir).resolve(); evdir.mkdir(parents=True, exist_ok=True)
    reportdir = Path(ns.report_dir).resolve(); reportdir.mkdir(parents=True, exist_ok=True)
    # Earlier evidence moves to superseded/; every runner stamps this run id, and the gates reject any other.
    run = begin_run(evdir, root, [cfg_path], [evdir, reportdir])
    os.environ[RUN_ID_ENV] = run["run_id"]
    print(json.dumps({"run_id": run["run_id"], "candidate": run["candidate"], "archived_previous_evidence": run["archived_previous_evidence"]}, ensure_ascii=False, indent=2), flush=True)
    py = sys.executable; keep = not ns.fail_fast
    rc = 0

    dsn_env = cfg["database"]["dsn_env"]
    if call([py, str(scripts/"toolchain.py"), "--dsn-env", dsn_env, "--out", str(evdir/"toolchain.doctor.json")], keep_going=keep): rc = 2
    if call([py, str(scripts/"postgres_snapshot.py"), "--dsn-env", dsn_env, "--id", "postgres.snapshot.pre", "--out", str(evdir/"postgres.snapshot.pre.json")], keep_going=keep): rc = 2

    manifest = cfg.get("dataset", {}).get("manifest")
    if manifest:
        ds_cmd = [py, str(scripts/"dataset_audit.py"), "--dsn-env", dsn_env, "--manifest", str((root/manifest).resolve()), "--safe-database-pattern", cfg["database"]["safe_database_pattern"], "--out", str(evdir/"dataset.audit.json")]
        if cfg["database"].get("allow_destructive", False): ds_cmd.append("--allow-destructive")
        if call(ds_cmd, keep_going=keep): rc = 2

    if cfg.get("sql_static"):
        if call([py, str(scripts/"sql_static_audit.py"), "--config", str(cfg_path), "--root", str(root), "--out", str(evdir/"sql.static.json")], keep_going=keep): rc = 2

    if cfg.get("schema_integrity"):
        if call([py, str(scripts/"schema_integrity_audit.py"), "--config", str(cfg_path), "--out", str(evdir/"schema.integrity.json")], keep_going=keep): rc = 2

    if cfg.get("security_audit"):
        if call([py, str(scripts/"security_boundary_audit.py"), "--config", str(cfg_path), "--out", str(evdir/"security.boundary.json")], keep_going=keep): rc = 2

    lock_catalog = cfg.get("lock_order_catalog")
    if lock_catalog:
        if call([py, str(scripts/"lock_order_analyzer.py"), "--catalog", str((root/lock_catalog["path"]).resolve()), "--out", str(evdir/"lock-order.analysis.json")], keep_going=keep): rc = 2

    for q in cfg.get("query", []):
        for ps in q.get("parameter_set", []):
            out = evdir / f"explain.{q['id']}.{ps['name']}.json"
            if call([py, str(scripts/"explain_capture.py"), "--config", str(cfg_path), "--query", q["id"], "--parameter-set", ps["name"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for prop in cfg.get("property_test", []):
        out = evdir / f"property.{prop['id']}.json"
        if call([py, str(scripts/"query_property_runner.py"), "--config", str(cfg_path), "--property", prop["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    if cfg.get("invariant"):
        if call([py, str(scripts/"invariant_audit.py"), "--config", str(cfg_path), "--root", str(root), "--out", str(evdir/"invariant.audit.json")], keep_going=keep): rc = 2

    for ps in cfg.get("plan_sensitivity", []):
        out = evdir / f"plan-sensitivity.{ps['id']}.json"
        if call([py, str(scripts/"plan_sensitivity_runner.py"), "--config", str(cfg_path), "--case", ps["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for lit in cfg.get("isolation_litmus", []):
        out = evdir / f"isolation.{lit['id']}.json"
        if call([py, str(scripts/"isolation_litmus_runner.py"), "--config", str(cfg_path), "--case", lit["id"], "--out", str(out)], keep_going=keep): rc = 2

    for sc in cfg.get("transaction_scenario", []):
        out = evdir / f"txn.{sc['id']}.json"
        if call([py, str(scripts/"transaction_scenario_runner.py"), "--config", str(cfg_path), "--scenario", sc["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for sc in cfg.get("concurrency_scenario", []):
        out = evdir / f"concurrency.{sc['id']}.json"
        if call([py, str(scripts/"concurrency_scenario_runner.py"), "--config", str(cfg_path), "--scenario", sc["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for fz in cfg.get("contention_fuzz", []):
        out = evdir / f"fuzz.{fz['id']}.json"
        if call([py, str(scripts/"contention_fuzz_runner.py"), "--config", str(cfg_path), "--fuzz", fz["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for ri in cfg.get("retry_idempotency", []):
        out = evdir / f"retry.{ri['id']}.json"
        cmd=[py, str(scripts/"retry_idempotency_runner.py"), "--config", str(cfg_path), "--case", ri["id"], "--root", str(root), "--out", str(out)]
        if ns.ack_isolated_destructive: cmd.append("--ack-isolated-environment")
        if call(cmd, keep_going=keep): rc = 2

    for rp in cfg.get("workload_replay", []):
        out = evdir / f"replay.{rp['id']}.json"
        if call([py, str(scripts/"workload_replay.py"), "--config", str(cfg_path), "--replay", rp["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for w in cfg.get("workload", []):
        if call([py, str(scripts/"pgbench_runner.py"), "--config", str(cfg_path), "--workload", w["id"], "--root", str(root), "--evidence-dir", str(evdir)], keep_going=keep): rc = 2

    for op in cfg.get("overload_profile", []):
        if call([py, str(scripts/"overload_recovery_runner.py"), "--config", str(cfg_path), "--profile", op["id"], "--root", str(root), "--evidence-dir", str(evdir)], keep_going=keep): rc = 2

    for m in cfg.get("migration_rehearsal", []):
        out = evdir / f"migration.{m['id']}.json"
        if call([py, str(scripts/"migration_rehearsal.py"), "--config", str(cfg_path), "--migration", m["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    if cfg.get("maintenance_audit"):
        if call([py, str(scripts/"maintenance_health_audit.py"), "--config", str(cfg_path), "--out", str(evdir/"maintenance.health.json")], keep_going=keep): rc = 2

    if cfg.get("physical_integrity"):
        if call([py, str(scripts/"physical_integrity_runner.py"), "--config", str(cfg_path), "--out", str(evdir/"physical.integrity.json")], keep_going=keep): rc = 2

    for rp in cfg.get("replica_consistency", []):
        out = evdir / f"replica.{rp['id']}.json"
        if call([py, str(scripts/"replica_consistency_runner.py"), "--config", str(cfg_path), "--case", rp["id"], "--out", str(out)], keep_going=keep): rc = 2

    for lr in cfg.get("logical_replication_audit", []):
        out = evdir / f"logical-replication.{lr['id']}.json"
        if call([py, str(scripts/"logical_replication_audit.py"), "--config", str(cfg_path), "--case", lr["id"], "--out", str(out)], keep_going=keep): rc = 2

    for ca in cfg.get("compatibility_audit", []):
        out = evdir / f"compatibility.{ca['id']}.json"
        if call([py, str(scripts/"compatibility_audit.py"), "--config", str(cfg_path), "--case", ca["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for hc in cfg.get("history_check", []):
        out = evdir / f"history.{hc['id']}.json"
        if call([py, str(scripts/"history_checker_adapter.py"), "--config", str(cfg_path), "--case", hc["id"], "--root", str(root), "--out", str(out)], keep_going=keep): rc = 2

    for fs in cfg.get("fault_scenario", []):
        out = evdir / f"fault.{fs['id']}.json"
        cmd=[py, str(scripts/"fault_scenario_runner.py"), "--scenario-file", str((root/fs["scenario_file"]).resolve()), "--out", str(out)]
        if ns.ack_isolated_destructive: cmd.append("--ack-isolated-environment")
        if call(cmd, keep_going=keep): rc = 2

    for lp in cfg.get("longrun_profile", []):
        out = evdir / f"longrun.{lp['id']}.json"
        cmd=[py, str(scripts/"longrun_monitor.py"), "--dsn-env", str(lp.get("dsn_env", dsn_env)), "--safe-database-pattern", cfg["database"]["safe_database_pattern"], "--duration-s", str(int(lp["duration_s"])), "--sample-s", str(float(lp.get("sample_s",5))), "--out", str(out), "--max-idle-in-tx", str(int(lp.get("max_idle_in_tx",0)))]
        if lp.get("max_lock_waiters") is not None: cmd += ["--max-lock-waiters", str(int(lp["max_lock_waiters"]))]
        if lp.get("max_xact_age_ms") is not None: cmd += ["--max-xact-age-ms", str(float(lp["max_xact_age_ms"]))]
        if lp.get("command"): cmd += ["--", *[str(x) for x in lp["command"]]]
        if call(cmd, keep_going=keep): rc = 2

    for br in cfg.get("backup_restore_rehearsal", []):
        out = evdir / f"backup-restore.{br['id']}.json"
        cmd=[py, str(scripts/"backup_restore_rehearsal.py"), "--config", str(cfg_path), "--case", br["id"], "--root", str(root), "--out", str(out)]
        if ns.ack_isolated_destructive: cmd.append("--ack-isolated-environment")
        if call(cmd, keep_going=keep): rc = 2

    for fo in cfg.get("failover_rehearsal", []):
        out = evdir / f"failover.{fo['id']}.json"
        cmd=[py, str(scripts/"failover_recovery_runner.py"), "--config", str(cfg_path), "--case", fo["id"], "--root", str(root), "--out", str(out)]
        if ns.ack_isolated_destructive: cmd.append("--ack-isolated-environment")
        if call(cmd, keep_going=keep): rc = 2

    if call([py, str(scripts/"postgres_snapshot.py"), "--dsn-env", dsn_env, "--id", "postgres.snapshot.post", "--out", str(evdir/"postgres.snapshot.post.json")], keep_going=keep): rc = 2

    if ns.baseline_dir:
        if call([py, str(scripts/"regression_compare.py"), "--current-dir", str(evdir), "--baseline-dir", str(Path(ns.baseline_dir).resolve()), "--out", str(evdir/"regression.baseline.json")], keep_going=keep): rc = 2

    if ns.qualification_manifest:
        qmanifest = Path(ns.qualification_manifest).resolve()
        if call([py, str(scripts/"qualification_coverage_gate.py"), "--manifest", str(qmanifest), "--evidence-dir", str(evdir), "--out", str(evdir/"qualification.coverage.json")], keep_going=keep): rc = 2

    gate_path = reportdir / "release-gate.json"
    if ns.gate_policy:
        if call([py, str(scripts/"release_gate.py"), "--policy", str(Path(ns.gate_policy).resolve()), "--evidence-dir", str(evdir), "--root", str(root), "--out", str(gate_path)], keep_going=True): rc = 2
    gate_args = ["--gate", str(gate_path)] if gate_path.exists() else []
    call([py, str(scripts/"evidence_report.py"), "--evidence-dir", str(evdir), "--config", str(cfg_path), *gate_args, "--out-md", str(reportdir/"sql-verification-report.md"), "--out-html", str(reportdir/"sql-verification-report.html")], keep_going=True)
    print(json.dumps({"status":"PASS" if rc == 0 else "FAIL", "run_id":run["run_id"], "evidence_dir":str(evdir), "report_dir":str(reportdir)}, ensure_ascii=False, indent=2))
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
