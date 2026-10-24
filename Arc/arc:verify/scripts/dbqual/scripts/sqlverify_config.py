#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
import tomllib
from pathlib import Path
from typing import Any

REQUIRED_TOP = {"project", "database", "dataset"}


def load_config(path: Path) -> dict[str, Any]:
    with path.open("rb") as f:
        cfg = tomllib.load(f)
    return cfg


_SAFE_PG_TYPE = re.compile(r"^(?:[A-Za-z_][A-Za-z0-9_$]*\.)?[A-Za-z_][A-Za-z0-9_$]*(?:\[\])?$")
_SAFE_IDENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_$]*$")


def validate(cfg: dict[str, Any], root: Path) -> list[str]:
    errors: list[str] = []
    missing = REQUIRED_TOP - cfg.keys()
    if missing:
        errors.append(f"missing top-level sections: {sorted(missing)}")

    project = cfg.get("project", {})
    if not project.get("name"):
        errors.append("project.name is required")

    db = cfg.get("database", {})
    if not db.get("dsn_env"):
        errors.append("database.dsn_env is required")
    if not db.get("safe_database_pattern"):
        errors.append("database.safe_database_pattern is required")

    ds = cfg.get("dataset", {})
    if not isinstance(ds.get("seed", 0), int):
        errors.append("dataset.seed must be integer")
    manifest = ds.get("manifest")
    if manifest and not (root / manifest).exists():
        errors.append(f"dataset.manifest not found: {manifest}")

    ids: set[str] = set()
    for i, w in enumerate(cfg.get("workload", [])):
        wid = w.get("id")
        if not wid:
            errors.append(f"workload[{i}].id is required")
            continue
        if wid in ids:
            errors.append(f"duplicate workload id: {wid}")
        ids.add(wid)
        scripts = w.get("scripts", [])
        if not scripts:
            errors.append(f"workload {wid}: scripts must not be empty")
        for s in scripts:
            p = root / s.get("path", "")
            if not p.exists():
                errors.append(f"workload {wid}: script not found: {s.get('path')}")
            if int(s.get("weight", 1)) < 1:
                errors.append(f"workload {wid}: script weight must be >=1")
        clients = w.get("clients", [])
        if not clients or any(int(c) < 1 for c in clients):
            errors.append(f"workload {wid}: clients must be non-empty positive integers")
        if int(w.get("duration_s", 0)) <= 0:
            errors.append(f"workload {wid}: duration_s must be >0")
        if int(w.get("repetitions", 0)) <= 0:
            errors.append(f"workload {wid}: repetitions must be >0")

    qids: set[str] = set()
    for i, q in enumerate(cfg.get("query", [])):
        qid = q.get("id")
        if not qid:
            errors.append(f"query[{i}].id is required")
            continue
        if qid in qids:
            errors.append(f"duplicate query id: {qid}")
        qids.add(qid)
        if int(q.get("statement_timeout_ms", 30000)) <= 0:
            errors.append(f"query {qid}: statement_timeout_ms must be >0")
        p = root / q.get("sql_file", "")
        if not p.exists():
            errors.append(f"query {qid}: sql_file not found: {q.get('sql_file')}")
        for ps in q.get("parameter_set", []):
            pp = root / ps.get("params_file", "")
            if not pp.exists():
                errors.append(f"query {qid}: params_file not found: {ps.get('params_file')}")

    pids: set[str] = set()
    allowed_property_kinds = {"exact_multiset", "count_matches_rows", "subset"}
    for i, prop in enumerate(cfg.get("property_test", [])):
        pid = prop.get("id")
        if not pid:
            errors.append(f"property_test[{i}].id is required")
            continue
        if pid in pids:
            errors.append(f"duplicate property_test id: {pid}")
        pids.add(pid)
        if prop.get("kind") not in allowed_property_kinds:
            errors.append(f"property_test {pid}: kind must be one of {sorted(allowed_property_kinds)}")
        for key in ("query_file", "oracle_query_file", "corpus_spec"):
            pp = root / prop.get(key, "")
            if not pp.exists():
                errors.append(f"property_test {pid}: {key} not found: {prop.get(key)}")
        if int(prop.get("max_cases", 1)) <= 0:
            errors.append(f"property_test {pid}: max_cases must be >0")

    tids: set[str] = set()
    for i, s in enumerate(cfg.get("transaction_scenario", [])):
        sid = s.get("id")
        if not sid:
            errors.append(f"transaction_scenario[{i}].id is required")
            continue
        if sid in tids:
            errors.append(f"duplicate transaction scenario id: {sid}")
        tids.add(sid)
        sessions = s.get("session", [])
        if len(sessions) < 2:
            errors.append(f"transaction scenario {sid}: at least 2 sessions required")
        barrier_names = {step.get("name") for sess in sessions for step in sess.get("step", []) if step.get("op") == "barrier"}
        for b in barrier_names:
            for sess in sessions:
                hits = sum(1 for step in sess.get("step", []) if step.get("op") == "barrier" and step.get("name") == b)
                if hits != 1:
                    errors.append(f"transaction scenario {sid}: barrier {b!r} must occur exactly once in every session; {sess.get('name')} has {hits}")
        if float(s.get("timeout_s", 15)) <= 0:
            errors.append(f"transaction scenario {sid}: timeout_s must be >0")
        if int(s.get("statement_timeout_ms", max(1000, int(float(s.get('timeout_s',15))*1000)))) <= 0:
            errors.append(f"transaction scenario {sid}: statement_timeout_ms must be >0")
        if not s.get("assertion", []):
            errors.append(f"transaction scenario {sid}: at least one independent final assertion required")


    # v2 release-qualification sections
    lock_catalog = cfg.get("lock_order_catalog")
    if lock_catalog:
        cp = root / lock_catalog.get("path", "")
        if not cp.exists():
            errors.append(f"lock_order_catalog.path not found: {lock_catalog.get('path')}")

    cids: set[str] = set()
    for i, s in enumerate(cfg.get("concurrency_scenario", [])):
        sid = s.get("id")
        if not sid:
            errors.append(f"concurrency_scenario[{i}].id is required"); continue
        if sid in cids: errors.append(f"duplicate concurrency_scenario id: {sid}")
        cids.add(sid)
        sessions = s.get("session", [])
        if len(sessions) < 2: errors.append(f"concurrency scenario {sid}: at least 2 sessions required")
        if not s.get("assertion", []): errors.append(f"concurrency scenario {sid}: at least one independent final assertion required")
        barriers = {st.get("name") for ss in sessions for st in ss.get("step", []) if st.get("op") == "barrier"}
        for b in barriers:
            for ss in sessions:
                hits=sum(1 for st in ss.get("step", []) if st.get("op")=="barrier" and st.get("name")==b)
                if hits != 1: errors.append(f"concurrency scenario {sid}: barrier {b!r} must occur exactly once in every session; {ss.get('name')} has {hits}")

    fids: set[str] = set()
    for i, fz in enumerate(cfg.get("contention_fuzz", [])):
        fid=fz.get("id")
        if not fid: errors.append(f"contention_fuzz[{i}].id is required"); continue
        if fid in fids: errors.append(f"duplicate contention_fuzz id: {fid}")
        fids.add(fid)
        if len(fz.get("session", [])) < 2: errors.append(f"contention_fuzz {fid}: at least 2 sessions required")
        if int(fz.get("runs", 0)) <= 0: errors.append(f"contention_fuzz {fid}: runs must be >0")
        if not fz.get("assertion", []): errors.append(f"contention_fuzz {fid}: final assertions required")

    rids: set[str] = set()
    for i, rp in enumerate(cfg.get("workload_replay", [])):
        rid=rp.get("id")
        if not rid: errors.append(f"workload_replay[{i}].id is required"); continue
        if rid in rids: errors.append(f"duplicate workload_replay id: {rid}")
        rids.add(rid)
        tp=root/rp.get("trace_file","")
        if not tp.exists(): errors.append(f"workload_replay {rid}: trace_file not found: {rp.get('trace_file')}")

    oids: set[str] = set()
    known_workloads={w.get('id') for w in cfg.get('workload',[])}
    for i, op in enumerate(cfg.get("overload_profile", [])):
        oid=op.get("id")
        if not oid: errors.append(f"overload_profile[{i}].id is required"); continue
        if oid in oids: errors.append(f"duplicate overload_profile id: {oid}")
        oids.add(oid)
        if op.get('workload') not in known_workloads: errors.append(f"overload_profile {oid}: unknown workload {op.get('workload')}")
        if not op.get('phase',[]): errors.append(f"overload_profile {oid}: phase list required")

    mids: set[str] = set()
    for i, m in enumerate(cfg.get("migration_rehearsal", [])):
        mid=m.get("id")
        if not mid: errors.append(f"migration_rehearsal[{i}].id is required"); continue
        if mid in mids: errors.append(f"duplicate migration_rehearsal id: {mid}")
        mids.add(mid)
        up=root/m.get('up_sql_file','')
        if not up.exists(): errors.append(f"migration_rehearsal {mid}: up_sql_file not found: {m.get('up_sql_file')}")
        if m.get('down_sql_file') and not (root/m['down_sql_file']).exists(): errors.append(f"migration_rehearsal {mid}: down_sql_file not found: {m.get('down_sql_file')}")


    # v3 expert-level lifecycle/operability sections
    inv_ids: set[str] = set()
    for i, c in enumerate(cfg.get("invariant", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"invariant[{i}].id is required"); continue
        if cid in inv_ids: errors.append(f"duplicate invariant id: {cid}")
        inv_ids.add(cid)
        if not c.get("sql") and not c.get("sql_file"):
            errors.append(f"invariant {cid}: sql or sql_file is required")
        if c.get("sql_file") and not (root / c["sql_file"]).exists():
            errors.append(f"invariant {cid}: sql_file not found: {c.get('sql_file')}")
        if c.get("op", "==") not in {"==","!=",">=",">","<=","<"}:
            errors.append(f"invariant {cid}: unsupported op {c.get('op')}")

    ps_ids: set[str] = set()
    known_queries = {q.get("id") for q in cfg.get("query", [])}
    for i, c in enumerate(cfg.get("plan_sensitivity", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"plan_sensitivity[{i}].id is required"); continue
        if cid in ps_ids: errors.append(f"duplicate plan_sensitivity id: {cid}")
        ps_ids.add(cid)
        if c.get("query") not in known_queries:
            errors.append(f"plan_sensitivity {cid}: unknown query {c.get('query')}")
        if not c.get("param_types"):
            errors.append(f"plan_sensitivity {cid}: param_types is required")
        else:
            for t in c.get("param_types", []):
                if not _SAFE_PG_TYPE.fullmatch(str(t).strip()):
                    errors.append(f"plan_sensitivity {cid}: unsafe/unsupported param type {t!r}; use simple PostgreSQL type aliases")

    rep_ids: set[str] = set()
    for i, c in enumerate(cfg.get("replica_consistency", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"replica_consistency[{i}].id is required"); continue
        if cid in rep_ids: errors.append(f"duplicate replica_consistency id: {cid}")
        rep_ids.add(cid)
        if not c.get("replica_dsn_env"):
            errors.append(f"replica_consistency {cid}: replica_dsn_env is required")
        if int(c.get("runs", 1)) <= 0:
            errors.append(f"replica_consistency {cid}: runs must be >0")
        if float(c.get("max_visibility_ms", 1)) <= 0:
            errors.append(f"replica_consistency {cid}: max_visibility_ms must be >0")
        for key, default in (("probe_schema", "sqlverify_probe"), ("probe_table", "replica_visibility")):
            value = str(c.get(key, default))
            if not _SAFE_IDENT.fullmatch(value):
                errors.append(f"replica_consistency {cid}: {key} must be a simple PostgreSQL identifier")

    br_ids: set[str] = set()
    for i, c in enumerate(cfg.get("backup_restore_rehearsal", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"backup_restore_rehearsal[{i}].id is required"); continue
        if cid in br_ids: errors.append(f"duplicate backup_restore_rehearsal id: {cid}")
        br_ids.add(cid)
        if c.get("verify_backup", True) and not c.get("backup_path"):
            errors.append(f"backup_restore_rehearsal {cid}: backup_path is required when verify_backup=true")
        if not c.get("restore_command"):
            errors.append(f"backup_restore_rehearsal {cid}: restore_command is required")

    fo_ids: set[str] = set()
    for i, c in enumerate(cfg.get("failover_rehearsal", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"failover_rehearsal[{i}].id is required"); continue
        if cid in fo_ids: errors.append(f"duplicate failover_rehearsal id: {cid}")
        fo_ids.add(cid)
        for key in ("inject_failover_command", "healthcheck_command"):
            if not c.get(key): errors.append(f"failover_rehearsal {cid}: {key} is required")

    fault_ids: set[str] = set()
    for i, c in enumerate(cfg.get("fault_scenario", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"fault_scenario[{i}].id is required"); continue
        if cid in fault_ids: errors.append(f"duplicate fault_scenario id: {cid}")
        fault_ids.add(cid)
        fp = root / c.get("scenario_file", "")
        if not fp.exists(): errors.append(f"fault_scenario {cid}: scenario_file not found: {c.get('scenario_file')}")

    long_ids: set[str] = set()
    for i, c in enumerate(cfg.get("longrun_profile", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"longrun_profile[{i}].id is required"); continue
        if cid in long_ids: errors.append(f"duplicate longrun_profile id: {cid}")
        long_ids.add(cid)
        if int(c.get("duration_s", 0)) <= 0: errors.append(f"longrun_profile {cid}: duration_s must be >0")
        if float(c.get("sample_s", 0)) <= 0: errors.append(f"longrun_profile {cid}: sample_s must be >0")
        cmd = c.get("command", [])
        if cmd and not isinstance(cmd, list): errors.append(f"longrun_profile {cid}: command must be argv list")

    hist_ids: set[str] = set()
    for i, c in enumerate(cfg.get("history_check", [])):
        cid = c.get("id")
        if not cid:
            errors.append(f"history_check[{i}].id is required"); continue
        if cid in hist_ids: errors.append(f"duplicate history_check id: {cid}")
        hist_ids.add(cid)
        hp = root / c.get("history_file", "")
        if not hp.exists(): errors.append(f"history_check {cid}: history_file not found: {c.get('history_file')}")
        if not c.get("checker_command"): errors.append(f"history_check {cid}: checker_command is required")

    ma = cfg.get("maintenance_audit", {})
    if ma and ma.get("max_connections_utilization") is not None:
        x = float(ma["max_connections_utilization"])
        if x <= 0 or x > 1: errors.append("maintenance_audit.max_connections_utilization must be in (0,1]")

    # v4 expert qualification additions
    si = cfg.get("schema_integrity", {})
    if si and si.get("max_tables_without_primary_key") is not None and int(si["max_tables_without_primary_key"]) < 0:
        errors.append("schema_integrity.max_tables_without_primary_key must be >=0")
    if si and si.get("max_unindexed_foreign_keys") is not None and int(si["max_unindexed_foreign_keys"]) < 0:
        errors.append("schema_integrity.max_unindexed_foreign_keys must be >=0")

    sec = cfg.get("security_audit", {})
    if sec:
        for t in sec.get("require_rls", []):
            if not isinstance(t, str) or t.count(".") != 1:
                errors.append(f"security_audit.require_rls entries must be schema.table strings: {t!r}")

    lit_ids: set[str] = set()
    for i, c in enumerate(cfg.get("isolation_litmus", [])):
        cid=c.get("id")
        if not cid: errors.append(f"isolation_litmus[{i}].id is required"); continue
        if cid in lit_ids: errors.append(f"duplicate isolation_litmus id: {cid}")
        lit_ids.add(cid)
        if str(c.get("type","write_skew")) not in {"write_skew","lost_update_detection"}: errors.append(f"isolation_litmus {cid}: unsupported type")
        if str(c.get("isolation","serializable")).lower() not in {"read committed","repeatable read","serializable"}: errors.append(f"isolation_litmus {cid}: unsupported isolation")

    ri_ids: set[str] = set()
    for i, c in enumerate(cfg.get("retry_idempotency", [])):
        cid=c.get("id")
        if not cid: errors.append(f"retry_idempotency[{i}].id is required"); continue
        if cid in ri_ids: errors.append(f"duplicate retry_idempotency id: {cid}")
        ri_ids.add(cid)
        if not isinstance(c.get("operation_command"), list) or not c.get("operation_command"): errors.append(f"retry_idempotency {cid}: operation_command argv list required")
        if int(c.get("attempts",2)) < 2: errors.append(f"retry_idempotency {cid}: attempts must be >=2")
        if not c.get("assertion",[]): errors.append(f"retry_idempotency {cid}: at least one durable-state assertion required")

    lr_ids: set[str] = set()
    for i, c in enumerate(cfg.get("logical_replication_audit", [])):
        cid=c.get("id")
        if not cid: errors.append(f"logical_replication_audit[{i}].id is required"); continue
        if cid in lr_ids: errors.append(f"duplicate logical_replication_audit id: {cid}")
        lr_ids.add(cid)
        if not c.get("subscriber_dsn_env"): errors.append(f"logical_replication_audit {cid}: subscriber_dsn_env required")

    ca_ids: set[str] = set()
    for i, c in enumerate(cfg.get("compatibility_audit", [])):
        cid=c.get("id")
        if not cid: errors.append(f"compatibility_audit[{i}].id is required"); continue
        if cid in ca_ids: errors.append(f"duplicate compatibility_audit id: {cid}")
        ca_ids.add(cid)
        if c.get("check_command") is not None and (not isinstance(c.get("check_command"), list) or not c.get("check_command")):
            errors.append(f"compatibility_audit {cid}: check_command must be argv list")

    ss = cfg.get("sql_static")
    if ss is not None:
        if not ss.get("dialect"): errors.append("sql_static.dialect is required")
        if not isinstance(ss.get("paths"), list) or not ss.get("paths"): errors.append("sql_static.paths must be a non-empty list of globs")
        if "migration_paths" in ss and (not isinstance(ss["migration_paths"], list) or not ss["migration_paths"]):
            errors.append("sql_static.migration_paths must be a non-empty list of globs when set")
        if not isinstance(ss.get("fail_on", []), list): errors.append("sql_static.fail_on must be a list of rule codes")
        for key in ("sqlfluff_command", "squawk_command"):
            if key in ss and (not isinstance(ss[key], list) or not ss[key]): errors.append(f"sql_static.{key} must be an argv list")
        excluded = ss.get("squawk_excluded_rules", {})
        if not isinstance(excluded, dict): errors.append("sql_static.squawk_excluded_rules must map rule -> rationale")
        else:
            for rule, why in excluded.items():
                if not str(why).strip(): errors.append(f"sql_static.squawk_excluded_rules.{rule}: rationale required")

    return errors


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("config")
    ap.add_argument("--root", default=".")
    ap.add_argument("--json", action="store_true")
    ns = ap.parse_args()
    root = Path(ns.root).resolve()
    cfg = load_config(Path(ns.config))
    errors = validate(cfg, root)
    out = {"status": "PASS" if not errors else "FAIL", "errors": errors}
    print(json.dumps(out, ensure_ascii=False, indent=2) if ns.json else ("PASS" if not errors else "\n".join(errors)))
    return 0 if not errors else 2


if __name__ == "__main__":
    raise SystemExit(main())
