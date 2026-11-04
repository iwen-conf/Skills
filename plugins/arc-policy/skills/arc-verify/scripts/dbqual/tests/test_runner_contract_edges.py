from __future__ import annotations

import contextlib
import io
import json
import os
import sys
import tempfile
import threading
import unittest
from contextlib import ExitStack
from pathlib import Path
from unittest.mock import patch

import test_runner_verdicts as harness

import backup_restore_rehearsal
import compatibility_audit
import dataset_audit
import failover_recovery_runner
import invariant_audit
import maintenance_health_audit
import migration_rehearsal
import pgbench_runner
import schema_integrity_audit
import security_boundary_audit


class AuditConnection(harness.FakeConnection):
    def execute(self, sql, params=None):
        self.pg.executions.append((threading.current_thread().name, sql))
        super().execute(sql, params)
        self.scalar_index = 0
        if sql == "select later_error":
            raise RuntimeError("independent observer stopped after the first check")
        if sql == "select blocker" and self.pg.blocker_error:
            raise RuntimeError("the interfering transaction was never established")
        if sql == "select action" and self.pg.probe_error and threading.current_thread() is threading.main_thread():
            self.pg.probe_error = False
            raise RuntimeError("probe observer unavailable")

    def fetchone(self):
        if self.sql == "select invariant":
            if self.scalar_index == len(self.pg.scalar_rows):
                return None
            row = self.pg.scalar_rows[self.scalar_index]
            self.scalar_index += 1
            return {str(i): value for i, value in enumerate(row)} if self.pg.dictionary else row
        if self.pg.dictionary and "current_database() as db" not in self.sql:
            if self.fetched:
                return None
            self.fetched = True
            return {"n": 0}
        return super().fetchone()


class AuditPostgres(harness.FakePostgres):
    def __init__(self, *, dictionary=False, rows=None, blocker_error=False, probe_error=False):
        super().__init__()
        self.dictionary = dictionary
        self.scalar_rows = [(1,)] if rows is None else rows
        self.blocker_error = blocker_error
        self.probe_error = probe_error
        self.executions = []

    def connect(self, *args, **kwargs):
        return AuditConnection(self)


class MaintenanceConnection(harness.FakeConnection):
    def execute(self, sql, params=None):
        super().execute(sql, params)
        if self.pg.failed_catalog in sql:
            raise RuntimeError("required catalog could not be observed")

    def fetchone(self):
        return {
            "db": "runner_test",
            "max_connections": 100,
            "n": 0,
            "age": 1,
            "oldest_xact_s": 0,
            "oldest_idle_s": 0,
            "max_age": 1,
            "max_ratio": 0,
            "max_mod_ratio": 0,
            "never_analyzed": 0,
            "oldest_stats_age_s": 0,
            "max_util": 0,
        }


class MaintenancePostgres(harness.FakePostgres):
    dictionary = True

    def __init__(self, failed_catalog):
        super().__init__()
        self.failed_catalog = failed_catalog

    def connect(self, *args, **kwargs):
        return MaintenanceConnection(self)


class ContractEdgeTests(unittest.TestCase):
    run_database_runner = harness.RunnerVerdictTests.run_database_runner

    def run_audit(self, module, section, checks, pg):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            cfg, out = root / "config.json", root / "evidence.json"
            cfg.write_text("{}", encoding="utf-8")
            config = {
                "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                section: checks if section == "invariant" or isinstance(checks, dict) else {"assertion": checks},
            }
            argv = [module.__name__, "--config", str(cfg), "--out", str(out)]
            if section == "dataset":
                cfg.write_text(json.dumps({"checks": checks}), encoding="utf-8")
                argv = [module.__name__, "--dsn-env", "RUNNER_TEST_DSN", "--manifest", str(cfg), "--out", str(out)]
            with ExitStack() as stack:
                stack.enter_context(patch.object(sys, "argv", argv))
                if hasattr(module, "load_config"):
                    stack.enter_context(patch.object(module, "load_config", return_value=config))
                driver = (pg, None) if pg.dictionary else pg
                stack.enter_context(patch.object(module, "_psycopg", return_value=driver))
                stack.enter_context(patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}))
                stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
                rc = module.main()
            return rc, json.loads(out.read_text(encoding="utf-8"))

    def audit_modules(self):
        return (
            (invariant_audit, "invariant", False),
            (dataset_audit, "dataset", False),
            (schema_integrity_audit, "schema_integrity", True),
            (security_boundary_audit, "security_audit", True),
        )

    def test_scalar_oracles_distinguish_null_from_no_observation(self):
        check = [{"id": "value", "sql": "select invariant", "expected": None}]
        for module, section, dictionary in self.audit_modules():
            for rows, expected in (
                ([(None,)], "PASS"),
                ([], "BLOCKED"),
                ([()], "BLOCKED"),
                ([(1, 2)], "BLOCKED"),
                ([(None,), (None,)], "BLOCKED"),
            ):
                with self.subTest(module=module.__name__, rows=rows):
                    rc, record = self.run_audit(module, section, check, AuditPostgres(dictionary=dictionary, rows=rows))
                    self.assertEqual(record["status"], expected)
                    self.assertEqual(rc, 0 if expected == "PASS" else 2)

    def test_first_valid_failure_survives_a_later_observer_error(self):
        checks = [
            {"id": "first", "sql": "select invariant", "expected": 2},
            {"id": "later", "sql": "select later_error", "expected": 1},
        ]
        for module, section, dictionary in self.audit_modules():
            with self.subTest(module=module.__name__):
                rc, record = self.run_audit(module, section, checks, AuditPostgres(dictionary=dictionary))
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "FAIL")
                raw = record["observed"].get("checks", record["observed"].get("assertions", []))
                self.assertEqual(raw[0]["id"], "first")
                self.assertFalse(raw[0]["passed"])
                self.assertTrue(record["warnings"])

    def test_empty_dataset_is_not_qualification(self):
        rc, record = self.run_audit(dataset_audit, "dataset", [], AuditPostgres())
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_cleanup_and_later_observer_errors_preserve_restore_failure(self):
        checks = [
            {"id": "first", "sql": "select invariant", "expected": 2},
            {"id": "later", "sql": "select later_error", "expected": 1},
        ]
        case = {
            "verify_backup": False,
            "restore_command": harness.command("pass"),
            "healthcheck_command": harness.command("pass"),
            "restored_dsn_env": "RUNNER_TEST_DSN",
            "assertion": checks,
            "cleanup_command": harness.command("raise SystemExit(2)"),
        }
        rc, record = self.run_database_runner(
            backup_restore_rehearsal, "backup_restore_rehearsal", case, AuditPostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertFalse(record["observed"]["assertions"][0]["passed"])
        self.assertTrue(record["warnings"])

    def test_command_assertion_requires_a_declared_failure_result(self):
        for code, declared, expected in ((127, [], "BLOCKED"), (1, [1], "FAIL"), (2, [1], "BLOCKED")):
            with self.subTest(code=code, declared=declared):
                case = {
                    "check_command": harness.command(f"raise SystemExit({code})"),
                    "oracle_failure_returncodes": declared,
                }
                rc, record = self.run_database_runner(
                    compatibility_audit, "compatibility_audit", case, harness.FakePostgres()
                )
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], expected)
                self.assertEqual(record["observed"]["check_command"]["returncode"], code)

    def test_declared_oracle_failure_survives_a_later_failover_error(self):
        case = {
            "inject_failover_command": harness.command("pass"),
            "healthcheck_command": harness.command("pass"),
            "oracle_command": [harness.command("raise SystemExit(1)"), ["/nonexistent/sqlverify-oracle"]],
            "oracle_failure_returncodes": [1],
            "cleanup_command": harness.command("raise SystemExit(2)"),
        }
        rc, record = self.run_database_runner(
            failover_recovery_runner, "failover_rehearsal", case, harness.FakePostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        failed_step = next(step for step in record["observed"]["steps"] if step["step"] == "oracle[0]")
        self.assertEqual(failed_step["returncode"], 1)
        self.assertTrue(record["warnings"])

    def test_failover_workload_requires_readiness_evidence(self):
        case = {
            "workload_command": harness.command("pass"),
            "workload_warmup_s": 0,
            "inject_failover_command": harness.command("pass"),
            "healthcheck_command": harness.command("pass"),
            "oracle_command": [harness.command("pass")],
            "cleanup_command": harness.command("pass"),
        }
        rc, record = self.run_database_runner(
            failover_recovery_runner, "failover_rehearsal", case, harness.FakePostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")
        self.assertFalse(any(step["step"] == "inject_failover" for step in record["observed"]["steps"]))

    def test_unknown_healthcheck_exit_is_not_a_proven_rto_failure(self):
        case = {
            "inject_failover_command": harness.command("pass"),
            "healthcheck_command": harness.command("raise SystemExit(127)"),
            "healthcheck_timeout_s": 0.03,
            "healthcheck_poll_s": 0.005,
            "oracle_command": [harness.command("pass")],
            "cleanup_command": harness.command("pass"),
        }
        rc, record = self.run_database_runner(
            failover_recovery_runner, "failover_rehearsal", case, harness.FakePostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")
        self.assertFalse(record["violations"])

    def test_lock_probe_setup_or_observer_failure_cannot_apply_migration(self):
        for failure in ("blocker_error", "probe_error"):
            with self.subTest(failure=failure):
                pg = AuditPostgres(**{failure: True})
                case = {"up_sql_file": "up.sql", "lock_probe_sql": "select blocker", "expect_lock_timeout": True}
                rc, record = self.run_database_runner(migration_rehearsal, "migration_rehearsal", case, pg)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")
                actual_actions = [
                    sql
                    for name, sql in pg.executions
                    if name == threading.main_thread().name and sql == "select action"
                ]
                self.assertEqual(len(actual_actions), 0 if failure == "blocker_error" else 1)

    def test_real_lock_expectation_violation_remains_fail_without_apply(self):
        pg = AuditPostgres()
        case = {"up_sql_file": "up.sql", "lock_probe_sql": "select blocker", "expect_lock_timeout": True}
        rc, record = self.run_database_runner(migration_rehearsal, "migration_rehearsal", case, pg)
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        actual_actions = [
            sql for name, sql in pg.executions if name == threading.main_thread().name and sql == "select action"
        ]
        self.assertEqual(len(actual_actions), 1)

    def test_required_health_observations_cannot_default_to_zero(self):
        for policy, catalog in (
            ({"max_sequence_utilization": 0.5}, "pg_sequences"),
            ({"max_inactive_slot_retained_bytes": 1}, "pg_replication_slots"),
            ({"require_no_lost_replication_slots": True}, "pg_replication_slots"),
        ):
            with self.subTest(policy=policy):
                rc, record = self.run_audit(
                    maintenance_health_audit, "maintenance_audit", policy, MaintenancePostgres(catalog)
                )
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")
        self.assertTrue(
            maintenance_health_audit.evaluate_health(
                {"lost_replication_slots": 1}, {"require_no_lost_replication_slots": True}
            )
        )

    def run_benchmark(self, result):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            cfg, out = root / "config.json", root / "evidence"
            cfg.write_text("{}", encoding="utf-8")
            (root / "query.sql").write_text("select 1", encoding="utf-8")
            config = {
                "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                "workload": [
                    {
                        "id": "case",
                        "clients": [1],
                        "repetitions": 1,
                        "duration_s": 1,
                        "scripts": [{"path": "query.sql"}],
                    }
                ],
            }
            argv = [
                "pgbench",
                "--config",
                str(cfg),
                "--workload",
                "case",
                "--root",
                str(root),
                "--evidence-dir",
                str(out),
            ]
            with (
                patch.object(sys, "argv", argv),
                patch.object(pgbench_runner, "load_config", return_value=config),
                patch.object(pgbench_runner, "which", return_value="test-double"),
                patch.object(pgbench_runner, "safe_db_check", return_value=(True, "runner_test")),
                patch.object(pgbench_runner, "run_one", return_value=result),
                patch.object(pgbench_runner, "capture_pgss", return_value={}),
                patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = pgbench_runner.main()
            return rc, json.loads((out / "bench.case.c1.json").read_text(encoding="utf-8"))

    def test_benchmark_missing_or_empty_counts_cannot_report_zero_failure_rate(self):
        for processed, failed in ((None, None), (1, None), (0, 0)):
            with self.subTest(processed=processed, failed=failed):
                result = {
                    "id": "case.c1.r1",
                    "returncode": 0,
                    "tps": 1,
                    "processed": processed,
                    "failed": failed,
                    "parsed": {"latency_ms": {"p50": 1, "p95": 1, "p99": 1, "max": 1}},
                    "stderr": "",
                    "stdout": "",
                }
                rc, record = self.run_benchmark(result)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")
                self.assertIsNone(record["observed"].get("failure_rate"))


if __name__ == "__main__":
    unittest.main()
