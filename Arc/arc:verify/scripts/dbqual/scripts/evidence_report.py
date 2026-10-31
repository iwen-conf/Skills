#!/usr/bin/env python3
from __future__ import annotations

import argparse
import html
import json
import tomllib
from collections import Counter
from pathlib import Path
from typing import Any

from sqlverify_common import json_load, now_iso
from release_gate import revalidate_gate


def load_evidence(d: Path) -> list[dict[str, Any]]:
    out = []
    for p in sorted(d.glob("*.json")):
        try:
            x = json_load(p)
            if isinstance(x, dict) and "id" in x and "status" in x:
                x["_file"] = p.name
                out.append(x)
        except Exception as e:
            out.append(
                {
                    "id": p.name,
                    "status": "BLOCKED",
                    "kind": "invalid_evidence",
                    "violations": [str(e)],
                    "warnings": [],
                    "_file": p.name,
                }
            )
    return out


def fmt(v: Any) -> str:
    if v is None:
        return "—"
    if isinstance(v, bool):
        return "yes" if v else "no"
    if isinstance(v, float):
        if abs(v) < 0.01 and v != 0:
            return f"{v:.6f}"
        return f"{v:.3f}"
    return str(v)


def pct(v: Any) -> str:
    return "—" if not isinstance(v, (int, float)) else f"{v * 100:.2f}%"


def load_config(path: Path | None) -> dict[str, Any] | None:
    if not path or not path.exists():
        return None
    with path.open("rb") as f:
        return tomllib.load(f)


def render_markdown(
    evs: list[dict[str, Any]], gate: dict[str, Any] | None, title: str, cfg: dict[str, Any] | None
) -> str:
    counts = Counter(e.get("status", "UNKNOWN") for e in evs)
    by_kind: dict[str, list[dict[str, Any]]] = {}
    for e in evs:
        by_kind.setdefault(e.get("kind", "unknown"), []).append(e)

    lines = [f"# {title}", "", f"Generated: `{now_iso()}`", ""]

    # Executive result
    lines += ["## 1. Executive result", ""]
    gate_status = gate.get("status") if gate else "NOT_RUN"
    lines += [
        f"**Release gate:** `{gate_status}`",
        "",
        "Evidence status: " + ", ".join(f"{k}={v}" for k, v in sorted(counts.items())),
        "",
    ]
    if gate and gate.get("failures"):
        lines.append(f"Blocking gate findings: **{len(gate['failures'])}**")
    if gate and gate.get("warnings"):
        lines.append(f"Gate warnings: **{len(gate['warnings'])}**")
    lines.append("")

    coverage = next(iter(by_kind.get("qualification_coverage", [])), None)
    lines += ["## 1.1 Qualification coverage / applicability", ""]
    if coverage:
        co = coverage.get("observed", {}) or {}
        lines.append(
            f"Risk class: **{co.get('risk_class', '—')}**; profile: **{co.get('profile', '—')}**; coverage status: **{coverage.get('status')}**."
        )
        lines += ["", "| Gate | Applicable | Requirements / rationale |", "|---|---:|---|"]
        for claim in co.get("claims", []):
            if claim.get("applicable"):
                reqs = []
                for req in claim.get("requirements", []):
                    reqs.append(f"{req.get('label')}: {req.get('satisfied')}/{req.get('required')} satisfied")
                detail = "; ".join(reqs) or "no requirements"
            else:
                detail = str(claim.get("rationale") or "—")
            lines.append(f"| {claim.get('gate')} | {fmt(claim.get('applicable'))} | {detail} |")
        lines.append("")
    else:
        lines += [
            "No qualification-coverage evidence was collected. For expert pre-release use, supply an explicit G0-G31 applicability manifest.",
            "",
        ]

    # Scope/config
    lines += ["## 2. Scope and verification contract", ""]
    if cfg:
        project = cfg.get("project", {})
        lines += [f"Project: **{project.get('name', '—')}**", ""]
        queries = cfg.get("query", [])
        workloads = cfg.get("workload", [])
        props = cfg.get("property_test", [])
        txs = cfg.get("transaction_scenario", [])
        advtxs = cfg.get("concurrency_scenario", [])
        fuzzes = cfg.get("contention_fuzz", [])
        replays = cfg.get("workload_replay", [])
        overloads = cfg.get("overload_profile", [])
        migrations = cfg.get("migration_rehearsal", [])
        plan_sens = cfg.get("plan_sensitivity", [])
        isolations = cfg.get("isolation_litmus", [])
        retries_cfg = cfg.get("retry_idempotency", [])
        replicas = cfg.get("replica_consistency", [])
        logical_cfg = cfg.get("logical_replication_audit", [])
        compatibility_cfg = cfg.get("compatibility_audit", [])
        restores = cfg.get("backup_restore_rehearsal", [])
        failovers = cfg.get("failover_rehearsal", [])
        histories = cfg.get("history_check", [])
        invariants = cfg.get("invariant", [])
        faults_cfg = cfg.get("fault_scenario", [])
        longruns_cfg = cfg.get("longrun_profile", [])
        lines.append(
            f"Configured query contracts: **{len(queries)}**; property/metamorphic tests: **{len(props)}**; "
            f"independent invariants: **{len(invariants)}**; workloads: **{len(workloads)}**; legacy transaction scenarios: **{len(txs)}**; "
            f"deterministic concurrency scenarios: **{len(advtxs)}**; contention fuzz profiles: **{len(fuzzes)}**; "
            f"workload replays: **{len(replays)}**; overload/recovery profiles: **{len(overloads)}**; "
            f"migration rehearsals: **{len(migrations)}**; plan-sensitivity cases: **{len(plan_sens)}**; "
            f"isolation litmus cases: **{len(isolations)}**; retry/idempotency cases: **{len(retries_cfg)}**; "
            f"replica-consistency cases: **{len(replicas)}**; logical-replication audits: **{len(logical_cfg)}**; compatibility audits: **{len(compatibility_cfg)}**; backup/restore rehearsals: **{len(restores)}**; "
            f"failover rehearsals: **{len(failovers)}**; external history checks: **{len(histories)}**; "
            f"fault scenarios: **{len(faults_cfg)}**; long-run profiles: **{len(longruns_cfg)}**."
        )
        if workloads:
            lines += [
                "",
                "| Workload | Clients | Duration s | Warmup s | Repetitions | Protocol | Rate | Seed |",
                "|---|---|---:|---:|---:|---|---:|---:|",
            ]
            for w in workloads:
                lines.append(
                    f"| `{w.get('id')}` | {', '.join(map(str, w.get('clients', [])))} | {fmt(w.get('duration_s'))} | {fmt(w.get('warmup_s', 0))} | {fmt(w.get('repetitions'))} | {w.get('protocol', '—')} | {fmt(w.get('rate_tps'))} | {fmt(w.get('random_seed'))} |"
                )
    else:
        lines.append(
            "Configuration was not supplied to the report generator; scope is reconstructed from evidence only."
        )
    lines.append("")

    # Environment/toolchain
    lines += ["## 3. Environment and comparability", ""]
    tool = next(iter(by_kind.get("toolchain", [])), None)
    if tool:
        checks = (tool.get("observed") or {}).get("checks", [])
        lines += ["| Tool | Required | Available | Version/path |", "|---|---:|---:|---|"]
        for c in checks:
            detail = c.get("version") or c.get("path") or c.get("error") or "—"
            lines.append(f"| {c.get('name')} | {fmt(c.get('required'))} | {fmt(c.get('ok'))} | {detail} |")
        lines.append("")
    snaps = by_kind.get("postgres_snapshot", [])
    if snaps:
        lines += [
            "| Snapshot | PostgreSQL | Settings hash | Schema hash | CPU arch | Active sessions | Deadlocks |",
            "|---|---|---|---|---|---:|---:|",
        ]
        for e in snaps:
            o = e.get("observed", {})
            dbs = o.get("pg_stat_database") or {}
            lines.append(
                f"| `{e.get('id')}` | {o.get('server_version_num', '—')} | `{str(o.get('settings_sha256', '—'))[:16]}` | `{str(o.get('schema_fingerprint_sha256', '—'))[:16]}` | {(o.get('host') or {}).get('machine', '—')} | {fmt(o.get('active_sessions_snapshot'))} | {fmt(dbs.get('deadlocks'))} |"
            )
        lines.append("")
        pre = next((e for e in snaps if e.get("id") == "postgres.snapshot.pre"), None)
        post = next((e for e in snaps if e.get("id") == "postgres.snapshot.post"), None)
        if pre and post:
            pgo = pre.get("observed", {}) or {}
            qgo = post.get("observed", {}) or {}
            pdb = pgo.get("pg_stat_database") or {}
            qdb = qgo.get("pg_stat_database") or {}
            pwal = pgo.get("pg_stat_wal") or {}
            qwal = qgo.get("pg_stat_wal") or {}

            def delta(a, b, key):
                try:
                    return float(b.get(key, 0) or 0) - float(a.get(key, 0) or 0)
                except Exception:
                    return None

            lines += [
                "Suite-level PostgreSQL counter deltas (covering the full captured pre→post suite window):",
                "",
                "| Counter | Delta |",
                "|---|---:|",
            ]
            for label, val in [
                ("xact_commit", delta(pdb, qdb, "xact_commit")),
                ("xact_rollback", delta(pdb, qdb, "xact_rollback")),
                ("blks_read", delta(pdb, qdb, "blks_read")),
                ("blks_hit", delta(pdb, qdb, "blks_hit")),
                ("temp_files", delta(pdb, qdb, "temp_files")),
                ("temp_bytes", delta(pdb, qdb, "temp_bytes")),
                ("deadlocks", delta(pdb, qdb, "deadlocks")),
                ("wal_records", delta(pwal, qwal, "wal_records")),
                ("wal_fpi", delta(pwal, qwal, "wal_fpi")),
                ("wal_bytes", delta(pwal, qwal, "wal_bytes")),
                ("wal_buffers_full", delta(pwal, qwal, "wal_buffers_full")),
            ]:
                lines.append(f"| {label} | {fmt(val)} |")
            lines.append("")
    reg = next(iter(by_kind.get("regression_compare", [])), None)
    if reg:
        envc = (reg.get("observed") or {}).get("environment", {})
        lines.append("Baseline comparability: " + ", ".join(f"{k}={fmt(v)}" for k, v in envc.items()) + ".")
        if (reg.get("observed") or {}).get("dataset_comparable") is not None:
            lines.append(
                f"Dataset profile/seed comparable: **{fmt((reg.get('observed') or {}).get('dataset_comparable'))}**."
            )
        lines.append("")

    # Dataset
    lines += ["## 4. Dataset fidelity", ""]
    datasets = by_kind.get("dataset_audit", [])
    if datasets:
        for e in datasets:
            o = e.get("observed", {})
            lines.append(
                f"Profile: `{o.get('profile', '—')}`, seed: `{o.get('seed', '—')}`, status: **{e.get('status')}**"
            )
            lines += ["", "| Check | Actual | Rule | Expected | Result |", "|---|---:|---|---:|---:|"]
            for c in o.get("checks", []):
                lines.append(
                    f"| `{c.get('id')}` | {fmt(c.get('actual'))} | {c.get('op')} | {fmt(c.get('expected'))} | {'PASS' if c.get('passed') else 'FAIL'} |"
                )
            lines.append("")
    else:
        lines += ["No dataset-audit evidence was collected.", ""]

    # Benchmarks
    benches = by_kind.get("pgbench", [])
    lines += ["## 5. Workload benchmark results", ""]
    if benches:
        lines += [
            "Percentiles below are the **median of each repetition's percentile**; TPS is the median across successful repetitions.",
            "",
        ]
        lines += [
            "| ID | Clients | Reps ok/total | TPS | TPS CV | P50 exec ms | P95 exec ms | P99 exec ms | P99 scheduled ms | P99 lag ms | Failure rate | Retries | Skipped |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
        for e in benches:
            o = e.get("observed", {})
            lat = o.get("latency_ms", {}) or {}
            sched = o.get("scheduled_latency_ms") or {}
            lag = o.get("schedule_lag_ms") or {}
            lines.append(
                f"| `{e.get('id')}` | {fmt(o.get('clients'))} | {fmt(o.get('successful_repetitions'))}/{fmt(o.get('repetitions'))} | {fmt(o.get('tps'))} | {pct(o.get('tps_cv'))} | {fmt(lat.get('p50'))} | {fmt(lat.get('p95'))} | {fmt(lat.get('p99'))} | {fmt(sched.get('p99'))} | {fmt(lag.get('p99'))} | {pct(o.get('failure_rate'))} | {fmt(o.get('total_retries'))} | {fmt(o.get('skipped_transactions'))} |"
            )
        lines.append("")
        for e in benches:
            o = e.get("observed", {})
            pgss = o.get("pg_stat_statements_delta")
            if pgss:
                lines += [
                    f"### Per-query cost attribution — `{e.get('id')}`",
                    "",
                    "Top statements by total execution time delta during the measured window:",
                    "",
                    "| QueryID | Calls | Total exec ms | Mean exec ms | Shared reads | Temp written | WAL bytes |",
                    "|---|---:|---:|---:|---:|---:|---:|",
                ]
                for r in pgss[:10]:
                    lines.append(
                        f"| `{r.get('queryid')}` | {fmt(r.get('calls'))} | {fmt(r.get('total_exec_time'))} | {fmt(r.get('mean_exec_time_ms_delta'))} | {fmt(r.get('shared_blks_read'))} | {fmt(r.get('temp_blks_written'))} | {fmt(r.get('wal_bytes'))} |"
                    )
                lines.append("")
    else:
        lines += ["No pgbench workload evidence was collected.", ""]

    # Plans
    plans = by_kind.get("explain", [])
    lines += ["## 6. Query-plan evidence", ""]
    if plans:
        lines += [
            "EXPLAIN ANALYZE is diagnostic and instrumented; use workload latency above for primary SLO decisions.",
            "",
        ]
        lines += [
            "| ID | Status | Exec ms | Plan ms | Max estimate error | Shared reads* | Temp written* | WAL bytes* | Node types |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---|",
        ]
        for e in plans:
            a = e.get("observed", {}).get("analysis", {}) or {}
            nodes = ", ".join(f"{k}×{v}" for k, v in sorted((a.get("node_types") or {}).items()))
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(a.get('execution_time_ms'))} | {fmt(a.get('planning_time_ms'))} | {fmt(a.get('max_estimate_error_ratio'))} | {fmt(a.get('shared_read_blocks_sum_nodes'))} | {fmt(a.get('temp_written_blocks_sum_nodes'))} | {fmt(a.get('wal_bytes_sum_nodes'))} | {nodes} |"
            )
        lines += [
            "",
            "*Block/WAL totals are node-summed diagnostics from captured plans, not a replacement for workload-level resource accounting.*",
            "",
        ]
        for e in plans:
            a = e.get("observed", {}).get("analysis", {}) or {}
            worst = a.get("worst_estimate_node")
            if worst:
                lines.append(
                    f"- `{e.get('id')}` worst cardinality estimate: {worst.get('node_type')} at `{worst.get('path')}`, planned {worst.get('plan_rows')} vs actual/loop {worst.get('actual_rows_per_loop')} ({fmt(worst.get('ratio'))}× q-error); loops={worst.get('loops')}, cumulative actual rows={worst.get('actual_total_rows')}."
                )
        lines.append("")
    else:
        lines += ["No EXPLAIN evidence was collected.", ""]

    # Property/metamorphic
    props = by_kind.get("query_property", [])
    lines += ["## 7. Property / metamorphic SQL evidence", ""]
    if props:
        lines += [
            "| ID | Status | Property | Seed | Executed | Failed | Max case ms |",
            "|---|---:|---|---:|---:|---:|---:|",
        ]
        for e in props:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {o.get('kind', '—')} | {fmt(o.get('seed'))} | {fmt(o.get('executed_cases'))} | {fmt(o.get('failed_cases'))} | {fmt(o.get('max_case_elapsed_ms'))} |"
            )
        lines.append("")
    else:
        lines += ["No generated property/metamorphic SQL evidence was collected.", ""]

    # Concurrency
    txs = by_kind.get("transaction_scenario", [])
    lines += ["## 8. Concurrency / transaction evidence", ""]
    if txs:
        lines += [
            "| ID | Status | SQLSTATEs | Lock-wait samples | Approx max continuous lock wait ms | Assertions |",
            "|---|---:|---|---:|---:|---|",
        ]
        for e in txs:
            o = e.get("observed", {})
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {', '.join(o.get('sqlstates', [])) or '—'} | {fmt(o.get('lock_waiting_sample_count'))} | {fmt(o.get('max_lock_wait_ms'))} | {fmt(o.get('assertions_passed'))}/{fmt(o.get('assertions_total'))} |"
            )
        lines.append("")
        for e in txs:
            assertions = (e.get("observed") or {}).get("assertions", [])
            if assertions:
                lines.append(f"### Assertions — `{e.get('id')}`")
                lines.append("")
                for a in assertions:
                    lines.append(
                        f"- `{a.get('id')}`: actual `{a.get('actual')}` {a.get('op')} expected `{a.get('expected')}` → **{'PASS' if a.get('passed') else 'FAIL'}**"
                    )
                lines.append("")
    else:
        lines += ["No deterministic transaction/concurrency evidence was collected.", ""]

    # Lock-order and advanced concurrency
    locks = by_kind.get("lock_order_analysis", [])
    lines += ["## 9. Lock-order / deadlock design analysis", ""]
    if locks:
        for e in locks:
            o = e.get("observed", {}) or {}
            cycles = o.get("cycles", [])
            lines.append(
                f"`{e.get('id')}`: operations={fmt(o.get('operation_count'))}, resources={fmt(o.get('resource_count'))}, edges={fmt(o.get('edge_count'))}, potential cycles={len(cycles)}, status=**{e.get('status')}**."
            )
            for c in cycles[:20]:
                lines.append(f"- cycle: `{' -> '.join(map(str, c.get('cycle', [])))}`")
        lines.append("")
    else:
        lines += ["No static lock-order analysis collected.", ""]

    advtx = by_kind.get("concurrency_scenario", [])
    lines += ["## 10. Deterministic concurrency / deadlock evidence", ""]
    if advtx:
        lines += [
            "| ID | Status | SQLSTATE counts | Wait samples | Max lock wait ms | Wait-for cycles | Assertions |",
            "|---|---:|---|---:|---:|---:|---|",
        ]
        for e in advtx:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | `{json.dumps(o.get('sqlstate_counts', {}), ensure_ascii=False)}` | {fmt(o.get('lock_wait_sample_count'))} | {fmt(o.get('max_lock_wait_ms'))} | {len(o.get('wait_for_cycles', []) or [])} | {fmt(o.get('assertions_passed'))}/{fmt(o.get('assertions_total'))} |"
            )
        lines.append("")
    else:
        lines += ["No v2 deterministic concurrency evidence collected.", ""]

    fuzzes = by_kind.get("contention_fuzz", [])
    lines += ["## 11. Contention / schedule exploration", ""]
    if fuzzes:
        lines += ["| ID | Status | Runs | SQLSTATE counts | Captured failing schedules |", "|---|---:|---:|---|---:|"]
        for e in fuzzes:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('runs'))} | `{json.dumps(o.get('sqlstate_counts', {}), ensure_ascii=False)}` | {len(o.get('failure_examples', []) or [])} |"
            )
        lines.append("")
    else:
        lines += ["No seeded contention exploration evidence collected.", ""]

    replays = by_kind.get("workload_replay", [])
    lines += ["## 12. Workload replay", ""]
    if replays:
        lines += [
            "| ID | Status | Sessions | Events | P95 ms | P99 ms | Schedule lag P99 ms | SQLSTATEs |",
            "|---|---:|---:|---:|---:|---:|---:|---|",
        ]
        for e in replays:
            o = e.get("observed", {}) or {}
            lat = o.get("latency_ms") or {}
            lag = o.get("schedule_lag_ms") or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('sessions'))} | {fmt(o.get('executed_events'))}/{fmt(o.get('events'))} | {fmt(lat.get('p95'))} | {fmt(lat.get('p99'))} | {fmt(lag.get('p99'))} | `{json.dumps(o.get('sqlstate_counts', {}), ensure_ascii=False)}` |"
            )
        lines.append("")
    else:
        lines += ["No sanitized timing/concurrency replay evidence collected.", ""]

    overloads = by_kind.get("overload_recovery", [])
    lines += ["## 13. Overload and recovery", ""]
    if overloads:
        for e in overloads:
            lines.append(f"### `{e.get('id')}` — {e.get('status')}")
            lines += [
                "",
                "| Phase | Kind | Rate | TPS | P99 ms | Lag P99 ms | Failure rate | Retries |",
                "|---|---|---:|---:|---:|---:|---:|---:|",
            ]
            for p in (e.get("observed", {}) or {}).get("phases", []):
                lines.append(
                    f"| {p.get('name')} | {p.get('kind')} | {fmt(p.get('rate_tps'))} | {fmt(p.get('tps'))} | {fmt(p.get('p99_ms'))} | {fmt(p.get('schedule_lag_p99_ms'))} | {pct(p.get('failure_rate'))} | {fmt(p.get('retries'))} |"
                )
            lines.append("")
    else:
        lines += ["No overload/recovery evidence collected.", ""]

    migrations = by_kind.get("migration_rehearsal", [])
    lines += ["## 14. Migration / rollback rehearsal", ""]
    if migrations:
        lines += [
            "| ID | Status | Apply ms | Rollback ms | Lock probe SQLSTATE | Post assertions | Rollback assertions |",
            "|---|---:|---:|---:|---|---:|---:|",
        ]
        for e in migrations:
            o = e.get("observed", {}) or {}
            lp = o.get("lock_probe") or {}
            post_assertions = o.get("post_assertions") or []
            rollback_assertions = o.get("rollback_assertions") or []
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('apply_elapsed_ms'))} | {fmt(o.get('rollback_elapsed_ms'))} | {fmt(lp.get('sqlstate'))} | {sum(1 for a in post_assertions if a.get('passed'))}/{len(post_assertions)} | {sum(1 for a in rollback_assertions if a.get('passed'))}/{len(rollback_assertions)} |"
            )
        lines.append("")
    else:
        lines += ["No migration rehearsal evidence collected.", ""]

    faults = by_kind.get("fault_scenario", [])
    lines += ["## 15. Fault injection / recovery", ""]
    if faults:
        lines += ["| ID | Status | Steps recorded | Violations | Warnings |", "|---|---:|---:|---:|---:|"]
        for e in faults:
            o = e.get("observed", {}) or {}
            records = o.get("records") or []
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {len(records)} | {len(e.get('violations', []) or [])} | {len(e.get('warnings', []) or [])} |"
            )
        lines.append("")
    else:
        lines += [
            "No fault-injection/recovery evidence collected. This is expected on PR profiles unless the change requires it.",
            "",
        ]

    longruns = by_kind.get("longrun_monitor", [])
    lines += ["## 16. Long-run / resource stability", ""]
    if longruns:
        lines += [
            "| ID | Status | Samples | Peak connections | Peak idle-in-tx | Peak lock waiters | Max xact age ms | Deadlock delta | Temp bytes delta | WAL bytes delta |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
        for e in longruns:
            o = e.get("observed", {}) or {}
            peaks = o.get("peaks") or {}
            dbd = o.get("db_delta") or {}
            wald = o.get("wal_delta") or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {len(o.get('samples') or [])} | {fmt(peaks.get('connections'))} | {fmt(peaks.get('idle_in_tx'))} | {fmt(peaks.get('lock_waiters'))} | {fmt(peaks.get('max_xact_age_ms'))} | {fmt(dbd.get('deadlocks'))} | {fmt(dbd.get('temp_bytes'))} | {fmt(wald.get('wal_bytes'))} |"
            )
        lines.append("")
    else:
        lines += [
            "No long-run resource evidence collected. Nightly/pre-release qualification should add it for long-lived/high-risk database paths.",
            "",
        ]

    # v3 lifecycle and expert-level evidence
    invariants = by_kind.get("invariant_audit", [])
    lines += ["## 17. Independent business/data invariants", ""]
    if invariants:
        for e in invariants:
            checks = (e.get("observed", {}) or {}).get("checks", [])
            lines.append(f"### `{e.get('id')}` — {e.get('status')}")
            lines += ["", "| Check | Actual | Rule | Expected | Result |", "|---|---:|---|---:|---:|"]
            for c in checks:
                lines.append(
                    f"| `{c.get('id')}` | {fmt(c.get('actual'))} | {c.get('op')} | {fmt(c.get('expected'))} | {'PASS' if c.get('passed') else 'FAIL'} |"
                )
            lines.append("")
    else:
        lines += ["No independent invariant-audit evidence collected.", ""]

    schema_int = by_kind.get("schema_integrity", [])
    lines += ["## 18. Schema integrity / relational design hazards", ""]
    if schema_int:
        lines += [
            "| ID | Status | Tables without PK | Unindexed FKs | Invalid indexes | Unvalidated constraints | Default partition max rows |",
            "|---|---:|---:|---:|---:|---:|---:|",
        ]
        for e in schema_int:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('tables_without_primary_key'))} | {fmt(o.get('unindexed_foreign_keys'))} | {fmt(o.get('invalid_indexes'))} | {fmt(o.get('unvalidated_constraints'))} | {fmt(o.get('max_default_partition_rows'))} |"
            )
        lines.append("")
    else:
        lines += ["No schema-integrity evidence collected.", ""]

    sec = by_kind.get("security_boundary", [])
    lines += ["## 19. Database security / privilege boundary", ""]
    if sec:
        lines += [
            "| ID | Status | Unexpected superusers | SECURITY DEFINER without explicit search_path | Required RLS checks |",
            "|---|---:|---:|---:|---:|",
        ]
        for e in sec:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {len(o.get('unexpected_superusers') or [])} | {len(o.get('security_definer_without_explicit_search_path') or [])} | {len(o.get('required_rls') or [])} |"
            )
        lines.append("")
    else:
        lines += ["No database security-boundary evidence collected.", ""]

    sensitivities = by_kind.get("plan_sensitivity", [])
    lines += ["## 20. Prepared-plan / parameter sensitivity", ""]
    if sensitivities:
        lines += [
            "| Evidence | Parameter set | Generic/custom cost ratio | Plan shape differs |",
            "|---|---|---:|---:|",
        ]
        for e in sensitivities:
            for c in (e.get("observed", {}) or {}).get("cases", []):
                lines.append(
                    f"| `{e.get('id')}` | `{c.get('parameter_set')}` | {fmt(c.get('generic_custom_cost_ratio'))} | {fmt(c.get('plan_changed'))} |"
                )
        lines.append("")
    else:
        lines += ["No generic/custom prepared-plan sensitivity evidence collected.", ""]

    isol = by_kind.get("isolation_litmus", [])
    lines += ["## 21. Isolation-level litmus / anomaly evidence", ""]
    if isol:
        lines += [
            "| ID | Status | Type | Isolation | Commits | SQLSTATE counts | Final invariant |",
            "|---|---:|---|---|---:|---|---|",
        ]
        for e in isol:
            o = e.get("observed", {}) or {}
            final = o.get("final_on_call_count") if o.get("type") == "write_skew" else o.get("final_value")
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {o.get('type', '—')} | {o.get('isolation', '—')} | {fmt(o.get('commits'))} | `{json.dumps(o.get('sqlstate_counts', {}), ensure_ascii=False)}` | {fmt(final)} |"
            )
        lines.append("")
    else:
        lines += ["No standard isolation litmus evidence collected.", ""]

    retries = by_kind.get("retry_idempotency", [])
    lines += ["## 22. Retry / idempotency / ambiguous-result qualification", ""]
    if retries:
        lines += ["| ID | Status | Attempts | Assertions passed | Fault injected |", "|---|---:|---:|---:|---:|"]
        for e in retries:
            o = e.get("observed", {}) or {}
            aa = o.get("assertions") or []
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {len(o.get('attempts') or [])} | {sum(1 for a in aa if a.get('passed'))}/{len(aa)} | {fmt(bool(o.get('fault')))} |"
            )
        lines.append("")
    else:
        lines += ["No end-to-end retry/idempotency evidence collected.", ""]

    maint = by_kind.get("maintenance_health", [])
    lines += ["## 23. Maintenance / MVCC / WAL-retention health", ""]
    if maint:
        lines += [
            "| ID | Status | Conn util | Oldest xact s | Idle-in-tx s | DB XID age | Max table XID age | Dead tuple ratio | Mod/analyze ratio | Never analyzed | Stats age s | Invalid indexes | Unvalidated constraints | Inactive slot retained bytes | Lost slots |",
            "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
        ]
        for e in maint:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {pct(o.get('connections_utilization'))} | {fmt(o.get('oldest_transaction_s'))} | {fmt(o.get('oldest_idle_in_transaction_s'))} | {fmt(o.get('database_xid_age'))} | {fmt(o.get('max_table_xid_age'))} | {pct(o.get('max_dead_tuple_ratio'))} | {pct(o.get('max_mod_since_analyze_ratio'))} | {fmt(o.get('never_analyzed_tables'))} | {fmt(o.get('oldest_statistics_age_s'))} | {fmt(o.get('invalid_indexes'))} | {fmt(o.get('unvalidated_constraints'))} | {fmt(o.get('max_inactive_slot_retained_bytes'))} | {fmt(o.get('lost_replication_slots'))} |"
            )
        lines.append("")
    else:
        lines += ["No maintenance-health evidence collected.", ""]

    replicas = by_kind.get("replica_consistency", [])
    lines += ["## 24. Replica visibility / consistency", ""]
    if replicas:
        lines += [
            "| ID | Status | Visible runs | P95 visibility ms | P99 visibility ms | Max visibility ms | Monotonic failures |",
            "|---|---:|---:|---:|---:|---:|---:|",
        ]
        for e in replicas:
            o = e.get("observed", {}) or {}
            lat = o.get("visibility_ms") or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('visible_runs'))}/{fmt(o.get('runs'))} | {fmt(lat.get('p95'))} | {fmt(lat.get('p99'))} | {fmt(lat.get('max'))} | {fmt(o.get('monotonic_failures'))} |"
            )
        lines.append("")
    else:
        lines += ["No replica-consistency evidence collected.", ""]

    logical = by_kind.get("logical_replication", [])
    lines += ["## 25. Logical replication / CDC apply health", ""]
    if logical:
        lines += [
            "| ID | Status | Active workers | Apply errors | Conflicts | Max apply age ms |",
            "|---|---:|---:|---:|---:|---:|",
        ]
        for e in logical:
            o = e.get("observed", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('active_workers'))} | {fmt(o.get('apply_error_count'))} | {fmt(o.get('conflict_count'))} | {fmt(o.get('max_apply_age_ms'))} |"
            )
        lines.append("")
    else:
        lines += ["No logical-replication/CDC apply evidence collected.", ""]

    physical = by_kind.get("physical_integrity", [])
    lines += ["## 26. Physical/index integrity", ""]
    if physical:
        for e in physical:
            o = e.get("observed", {}) or {}
            lines.append(f"- `{e.get('id')}` — **{e.get('status')}**, pg_amcheck rc={fmt(o.get('returncode'))}")
        lines.append("")
    else:
        lines += ["No pg_amcheck/physical-integrity evidence collected.", ""]

    restores = by_kind.get("backup_restore_rehearsal", [])
    lines += ["## 27. Backup verification / restore / PITR rehearsal", ""]
    if restores:
        lines += [
            "| ID | Status | Backup verifier rc | Healthy | RTO s | Assertions passed |",
            "|---|---:|---:|---:|---:|---:|",
        ]
        for e in restores:
            o = e.get("observed", {}) or {}
            b = o.get("backup_verification") or {}
            r = o.get("restore") or {}
            aa = o.get("assertions") or []
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(b.get('returncode'))} | {fmt(r.get('healthy'))} | {fmt(r.get('rto_s'))} | {sum(1 for a in aa if a.get('passed'))}/{len(aa)} |"
            )
        lines.append("")
    else:
        lines += ["No backup/restore rehearsal evidence collected.", ""]

    failovers = by_kind.get("failover_rehearsal", [])
    lines += ["## 28. Failover / failback qualification", ""]
    if failovers:
        lines += ["| ID | Status | RTO s | Healthy after failover | Steps |", "|---|---:|---:|---:|---:|"]
        for e in failovers:
            o = e.get("observed", {}) or {}
            hc = o.get("healthcheck") or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('rto_s'))} | {fmt(hc.get('healthy'))} | {len(o.get('steps') or [])} |"
            )
        lines.append("")
    else:
        lines += ["No failover/failback evidence collected.", ""]

    compat = by_kind.get("compatibility_audit", [])
    lines += ["## 29. Version / extension / collation compatibility", ""]
    if compat:
        lines += [
            "| ID | Status | PostgreSQL major | Extensions | Collation version mismatches | Check command rc |",
            "|---|---:|---:|---:|---:|---:|",
        ]
        for e in compat:
            o = e.get("observed", {}) or {}
            cc = o.get("check_command") or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(o.get('major'))} | {len(o.get('extensions') or [])} | {len(o.get('collation_version_mismatches') or [])} | {fmt(cc.get('returncode'))} |"
            )
        lines.append("")
    else:
        lines += ["No upgrade/compatibility evidence collected.", ""]

    histories = by_kind.get("history_check", [])
    lines += ["## 30. Transaction history / model checking", ""]
    if histories:
        lines += ["| ID | Status | Model | Checker rc |", "|---|---:|---|---:|"]
        for e in histories:
            o = e.get("observed", {}) or {}
            r = e.get("reproduction", {}) or {}
            lines.append(
                f"| `{e.get('id')}` | {e.get('status')} | {fmt(r.get('model'))} | {fmt(o.get('returncode'))} |"
            )
        lines.append("")
    else:
        lines += [
            "No external Jepsen/Elle/project history-check evidence collected. For DB-R4/R5, absence must be an explicit applicability decision, not assumed coverage.",
            "",
        ]

    # Regression
    lines += ["## 31. Baseline regression", ""]
    if reg:
        comps = (reg.get("observed") or {}).get("comparisons", [])
        if comps:
            lines += [
                "| Evidence | Kind | P95 Δ | P99 Δ | TPS Δ | EXPLAIN time Δ | Added plan nodes |",
                "|---|---|---:|---:|---:|---:|---|",
            ]
            for c in comps:
                lines.append(
                    f"| `{c.get('id')}` | {c.get('kind')} | {pct(c.get('p95_delta'))} | {pct(c.get('p99_delta'))} | {pct(c.get('tps_delta'))} | {pct(c.get('execution_time_delta'))} | {', '.join(c.get('node_types_added', [])) or '—'} |"
                )
            lines.append("")
        else:
            lines += ["Regression evidence exists but no matching baseline IDs were comparable.", ""]
    else:
        lines += ["No baseline comparison was supplied for this run.", ""]

    canaries = by_kind.get("canary_evaluation", [])
    lines += ["## 32. Canary / post-deploy database guardrails", ""]
    if canaries:
        for e in canaries:
            lines.append(f"### `{e.get('id')}` — {e.get('status')}")
            lines += [
                "",
                "| Metric | Control median | Canary median | Relative delta | CI95 |",
                "|---|---:|---:|---:|---|",
            ]
            for c in (e.get("observed", {}) or {}).get("comparisons", []):
                ci = c.get("delta_ci95") or {}
                ci_text = "—" if not ci else f"[{pct(ci.get('low'))}, {pct(ci.get('high'))}]"
                lines.append(
                    f"| `{c.get('name')}` | {fmt(c.get('control_median'))} | {fmt(c.get('canary_median'))} | {pct(c.get('relative_delta'))} | {ci_text} |"
                )
            lines.append("")
    else:
        lines += [
            "No canary-evaluation Evidence attached. Post-deploy guardrails remain a separate production claim.",
            "",
        ]

    # Inventory
    lines += ["## 33. Evidence inventory", "", "| ID | Kind | Status | Elapsed ms | File |", "|---|---|---:|---:|---|"]
    for e in evs:
        lines.append(
            f"| `{e.get('id')}` | {e.get('kind', '')} | **{e.get('status')}** | {fmt(e.get('elapsed_ms'))} | `{e.get('_file', '')}` |"
        )
    lines.append("")

    # Findings
    all_violations = [(e.get("id"), v) for e in evs for v in e.get("violations", [])]
    all_warnings = [(e.get("id"), v) for e in evs for v in e.get("warnings", [])]
    if gate and gate.get("failures"):
        all_violations += [("release_gate", json.dumps(x, ensure_ascii=False)) for x in gate["failures"]]
    if gate and gate.get("warnings"):
        all_warnings += [("release_gate", json.dumps(x, ensure_ascii=False)) for x in gate["warnings"]]

    lines += ["## 34. Blocking findings", ""]
    if all_violations:
        for eid, v in all_violations:
            lines.append(f"- **{eid}** — {v}")
    else:
        lines.append("No blocking finding in collected evidence.")
    lines += ["", "## 35. Non-blocking warnings", ""]
    if all_warnings:
        for eid, v in all_warnings:
            lines.append(f"- **{eid}** — {v}")
    else:
        lines.append("No warning in collected evidence.")

    # Reproduction
    lines += ["", "## 36. Reproducibility", ""]
    lines.append(
        "Evidence IDs below preserve the relevant seed/config/hash/environment inputs. Credentials are intentionally excluded or redacted."
    )
    lines.append("")
    for e in evs:
        r = e.get("reproduction") or {}
        if r:
            compact = json.dumps(r, ensure_ascii=False, sort_keys=True, default=str)
            if len(compact) > 1200:
                compact = compact[:1200] + "…"
            lines.append(f"- `{e.get('id')}` — `{compact}`")

    # Limitations
    limited = [e for e in evs if e.get("status") in {"BLOCKED", "NOT_RUN", "FLAKY"}]
    lines += ["", "## 37. Limitations / untested risk", ""]
    if limited:
        for e in limited:
            lines.append(
                f"- `{e.get('id')}` status is **{e.get('status')}**; this evidence must not be treated as PASS."
            )
    else:
        lines.append(
            "No evidence item is marked BLOCKED, NOT_RUN, or FLAKY. This does not prove absence of unmodeled production risks."
        )

    lines += [
        "",
        "## 38. Interpretation rule",
        "",
        "This report separates measured facts and gate decisions from engineering hypotheses. A Seq Scan, high estimate error, temp spill, or lock wait is evidence to investigate; it is not by itself proof of a specific root cause or proof that an index/configuration change is correct.",
        "",
    ]
    return "\n".join(lines)


def _inline_html(text: str) -> str:
    out = html.escape(text)
    out = __import__("re").sub(r"`([^`]+)`", r"<code>\1</code>", out)
    out = __import__("re").sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", out)
    return out


def render_html(md: str, title: str) -> str:
    """Render the report's intentionally small Markdown subset without external dependencies."""
    lines = md.splitlines()
    body: list[str] = []
    i = 0
    in_code = False
    code_lines: list[str] = []
    in_list = False
    while i < len(lines):
        line = lines[i]
        if line.startswith("```"):
            if not in_code:
                in_code = True
                code_lines = []
            else:
                body.append("<pre><code>" + html.escape("\n".join(code_lines)) + "</code></pre>")
                in_code = False
                code_lines = []
            i += 1
            continue
        if in_code:
            code_lines.append(line)
            i += 1
            continue
        if in_list and not line.startswith("- "):
            body.append("</ul>")
            in_list = False
        if not line.strip():
            i += 1
            continue
        if line.startswith("|") and i + 1 < len(lines) and lines[i + 1].startswith("|") and "---" in lines[i + 1]:
            headers = [x.strip() for x in line.strip("|").split("|")]
            i += 2
            rows = []
            while i < len(lines) and lines[i].startswith("|"):
                rows.append([x.strip() for x in lines[i].strip("|").split("|")])
                i += 1
            body.append(
                "<div class='table-wrap'><table><thead><tr>"
                + "".join(f"<th>{_inline_html(x)}</th>" for x in headers)
                + "</tr></thead><tbody>"
            )
            for row in rows:
                body.append("<tr>" + "".join(f"<td>{_inline_html(x)}</td>" for x in row) + "</tr>")
            body.append("</tbody></table></div>")
            continue
        if line.startswith("#"):
            level = min(len(line) - len(line.lstrip("#")), 6)
            text = line[level:].strip()
            body.append(f"<h{level}>{_inline_html(text)}</h{level}>")
            i += 1
            continue
        if line.startswith("- "):
            if not in_list:
                body.append("<ul>")
                in_list = True
            body.append("<li>" + _inline_html(line[2:]) + "</li>")
            i += 1
            continue
        body.append("<p>" + _inline_html(line) + "</p>")
        i += 1
    if in_list:
        body.append("</ul>")
    if in_code:
        body.append("<pre><code>" + html.escape("\n".join(code_lines)) + "</code></pre>")
    css = """
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;max-width:1280px;margin:40px auto;padding:0 28px;line-height:1.55;color:#1f2328;background:#fff}
h1,h2,h3{line-height:1.25;margin-top:1.6em}code{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;background:#f6f8fa;padding:.1em .35em;border-radius:4px}pre{background:#f6f8fa;padding:18px;border:1px solid #d0d7de;border-radius:8px;overflow:auto}pre code{padding:0;background:none}.table-wrap{overflow:auto;margin:16px 0}table{border-collapse:collapse;width:100%;font-size:14px}th,td{border:1px solid #d0d7de;padding:8px 10px;text-align:left;vertical-align:top}th{background:#f6f8fa}strong{font-weight:700}ul{padding-left:24px}@media print{body{max-width:none;margin:0;font-size:10pt}table{font-size:8pt}h2{break-after:avoid}.table-wrap{overflow:visible}}
"""
    return (
        "<!doctype html><html><head><meta charset='utf-8'><meta name='viewport' content='width=device-width,initial-scale=1'><title>"
        + html.escape(title)
        + "</title><style>"
        + css
        + "</style></head><body>"
        + "\n".join(body)
        + "</body></html>"
    )


def main() -> int:
    ap = argparse.ArgumentParser(description="Generate deterministic SQL verification report from evidence JSON")
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument("--gate")
    ap.add_argument("--config")
    ap.add_argument("--root", default=".", help="candidate root used by run_suite")
    ap.add_argument("--title", default="Industrial SQL Verification Report")
    ap.add_argument("--out-md", required=True)
    ap.add_argument("--out-html")
    ns = ap.parse_args()
    evs = load_evidence(Path(ns.evidence_dir))
    gate = None
    if ns.gate:
        try:
            gate = json_load(Path(ns.gate))
            evs, issues = revalidate_gate(gate, Path(ns.evidence_dir), Path(ns.root))
        except Exception as exc:
            issues = [{"type": "unreadable_gate_inputs", "detail": str(exc)}]
        if issues:
            gate = {"status": "BLOCKED", "failures": issues, "warnings": []}
    cfg = load_config(Path(ns.config)) if ns.config else None
    md = render_markdown(evs, gate, ns.title, cfg)
    Path(ns.out_md).parent.mkdir(parents=True, exist_ok=True)
    Path(ns.out_md).write_text(md + "\n", encoding="utf-8")
    if ns.out_html:
        Path(ns.out_html).parent.mkdir(parents=True, exist_ok=True)
        Path(ns.out_html).write_text(render_html(md, ns.title), encoding="utf-8")
    print(
        json.dumps(
            {
                "evidence": len(evs),
                "gate": gate.get("status") if gate else None,
                "out_md": ns.out_md,
                "out_html": ns.out_html,
            },
            ensure_ascii=False,
        )
    )
    return 2 if gate and gate.get("status") != "PASS" else 0


if __name__ == "__main__":
    raise SystemExit(main())
