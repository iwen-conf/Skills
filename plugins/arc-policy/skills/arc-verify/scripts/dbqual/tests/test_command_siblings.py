from __future__ import annotations

import contextlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import ExitStack
from pathlib import Path
from unittest.mock import patch

import test_runner_verdicts as harness
import test_runner_contract_edges as scalar_harness

import concurrency_scenario_runner
import contention_fuzz_runner
import history_checker_adapter
import longrun_monitor
import maintenance_health_audit
import overload_recovery_runner
import physical_integrity_runner
import retry_idempotency_runner
import transaction_scenario_runner


class LaterCatalogConnection(scalar_harness.MaintenanceConnection):
    def fetchone(self):
        row = super().fetchone()
        if "count(*)::int as n from pg_stat_activity" in self.sql:
            row["n"] = 90
        return row


class LaterCatalogPostgres(scalar_harness.MaintenancePostgres):
    def connect(self, *args, **kwargs):
        return LaterCatalogConnection(self)


class CommandSiblingTests(unittest.TestCase):
    run_database_runner = harness.RunnerVerdictTests.run_database_runner
    run_audit = scalar_harness.ContractEdgeTests.run_audit

    def test_command_failures_need_declared_assertion_exit_codes(self):
        for module, section in (
            (history_checker_adapter, "history_check"),
            (physical_integrity_runner, "physical_integrity"),
        ):
            for code, declared, expected in ((127, [], "BLOCKED"), (1, [1], "FAIL"), (2, [1], "BLOCKED")):
                with self.subTest(module=module.__name__, code=code), tempfile.TemporaryDirectory() as td:
                    root = Path(td)
                    cfg, out = root / "config.json", root / "evidence.json"
                    cfg.write_text("{}", encoding="utf-8")
                    (root / "history.json").write_text("[]", encoding="utf-8")
                    case = {
                        "id": "case",
                        "history_file": "history.json",
                        "checker_command": harness.command(f"raise SystemExit({code})"),
                        "enabled": True,
                        "oracle_failure_returncodes": declared,
                    }
                    config = {
                        "database": {"dsn_env": "RUNNER_TEST_DSN"},
                        section: [case] if section == "history_check" else case,
                    }
                    argv = [module.__name__, "--config", str(cfg), "--out", str(out)]
                    if section == "history_check":
                        argv += ["--case", "case", "--root", str(root)]
                    with ExitStack() as stack:
                        stack.enter_context(patch.object(sys, "argv", argv))
                        stack.enter_context(patch.object(module, "load_config", return_value=config))
                        stack.enter_context(patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}))
                        stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
                        if section == "physical_integrity":
                            stack.enter_context(patch.object(module, "which", return_value="test-double"))
                            stack.enter_context(patch.object(module, "_libpq_cli_env", return_value=([], {}, [])))
                            stack.enter_context(
                                patch.object(
                                    module,
                                    "run",
                                    return_value=subprocess.CompletedProcess(
                                        ["test-double"], code, "", "explicit test result"
                                    ),
                                )
                            )
                        rc = module.main()
                    record = json.loads(out.read_text(encoding="utf-8"))
                    self.assertEqual(rc, 2)
                    self.assertEqual(record["status"], expected)
                    self.assertEqual(record["observed"]["returncode"], code)

    def test_retry_unknown_operation_result_is_blocked_and_stops_attempts(self):
        case = {
            "operation_command": harness.command("raise SystemExit(127)"),
            "attempts": 2,
            "allowed_returncodes": [0],
            "assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}],
        }
        rc, record = self.run_database_runner(
            retry_idempotency_runner, "retry_idempotency", case, harness.FakePostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")
        self.assertEqual(len(record["observed"]["attempts"]), 1)

    def test_retry_has_no_pass_without_independent_business_assertions(self):
        rc, record = self.run_database_runner(
            retry_idempotency_runner,
            "retry_idempotency",
            {"operation_command": harness.command("pass"), "attempts": 1},
            harness.FakePostgres(),
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")
        self.assertEqual(record["observed"]["attempts"], [])

    def test_retry_keeps_first_assertion_raw_after_observer_failure(self):
        case = {
            "operation_command": harness.command("pass"),
            "attempts": 1,
            "assertion": [
                {"id": "first", "sql": "select invariant", "expected": 2},
                {"id": "later", "sql": "select later_error", "expected": 1},
            ],
        }
        rc, record = self.run_database_runner(
            retry_idempotency_runner, "retry_idempotency", case, scalar_harness.AuditPostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertIn("assertions", record["observed"])
        self.assertFalse(record["observed"]["assertions"][0]["passed"])

    def test_scenario_and_fuzz_scalar_oracles_do_not_pass_absent_rows(self):
        cases = [
            (
                transaction_scenario_runner,
                "transaction_scenario",
                {
                    "require_observer": False,
                    "session": [{"name": name, "step": [{"op": "sql", "sql": "select action"}]} for name in ("a", "b")],
                },
            ),
            (
                concurrency_scenario_runner,
                "concurrency_scenario",
                {
                    "require_observer": False,
                    "session": [{"name": name, "step": [{"op": "sql", "sql": "select action"}]} for name in ("a", "b")],
                },
            ),
            (
                contention_fuzz_runner,
                "contention_fuzz",
                {
                    "runs": 1,
                    "jitter_ms": 0,
                    "session": [{"name": name, "sql": ["select action"]} for name in ("a", "b")],
                },
            ),
        ]
        for module, section, case in cases:
            with self.subTest(module=module.__name__):
                case["assertion"] = [{"id": "value", "sql": "select invariant", "expected": None}]
                rc, record = self.run_database_runner(module, section, case, scalar_harness.AuditPostgres(rows=[]))
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_fuzz_keeps_first_failure_before_a_later_observer_error(self):
        case = {
            "runs": 1,
            "jitter_ms": 0,
            "session": [{"name": name, "sql": ["select action"]} for name in ("a", "b")],
            "assertion": [
                {"id": "first", "sql": "select invariant", "expected": 2},
                {"id": "later", "sql": "select later_error", "expected": 1},
            ],
        }
        rc, record = self.run_database_runner(
            contention_fuzz_runner, "contention_fuzz", case, scalar_harness.AuditPostgres()
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertTrue(record["observed"]["failure_examples"])
        self.assertFalse(record["observed"]["failure_examples"][0]["assertions"][0]["passed"])

    def test_health_keeps_an_observed_violation_before_later_catalog_error(self):
        rc, record = self.run_audit(
            maintenance_health_audit,
            "maintenance_audit",
            {"max_connections_utilization": 0.5},
            LaterCatalogPostgres("pg_stat_user_tables"),
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertEqual(record["observed"]["connections_utilization"], 0.9)

    def test_soak_keeps_a_valid_first_sample_violation(self):
        with tempfile.TemporaryDirectory() as td:
            out = Path(td) / "evidence.json"
            sample = {"connections": 3, "idle_in_tx": 2, "lock_waiters": 0, "max_xact_age_ms": 1, "db": {}, "wal": {}}
            argv = [
                "longrun",
                "--dsn-env",
                "RUNNER_TEST_DSN",
                "--duration-s",
                "1",
                "--sample-s",
                ".001",
                "--max-idle-in-tx",
                "0",
                "--out",
                str(out),
            ]
            with (
                patch.object(sys, "argv", argv),
                patch.object(longrun_monitor, "_psycopg", return_value=harness.FakePostgres()),
                patch.object(longrun_monitor, "snap", side_effect=[sample, RuntimeError("observer disconnected")]),
                patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = longrun_monitor.main()
            record = json.loads(out.read_text(encoding="utf-8"))
            self.assertEqual(rc, 2)
            self.assertEqual(record["status"], "FAIL")
            self.assertEqual(record["observed"]["samples"][0]["idle_in_tx"], 2)

    def test_overload_requires_observed_metrics_and_explicit_recovery_budgets(self):
        for damage in ("counts", "latency", "recovery-budget"):
            with self.subTest(damage=damage), tempfile.TemporaryDirectory() as td:
                root = Path(td)
                cfg, out = root / "config.json", root / "evidence"
                cfg.write_text("{}", encoding="utf-8")
                profile = {
                    "id": "case",
                    "workload": "base",
                    "max_failure_rate": 0,
                    "phase": [{"name": "nominal"}, {"name": "recovery"}],
                }
                if damage == "latency":
                    profile["phase"][0]["max_p99_ms"] = 1
                if damage == "recovery-budget":
                    profile.update(nominal_phase="nominal", recovery_phase="recovery")
                config = {
                    "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                    "overload_profile": [profile],
                    "workload": [{"id": "base", "clients": [1]}],
                }
                result = {
                    "id": "case",
                    "returncode": 0,
                    "processed": None if damage == "counts" else 1,
                    "failed": None if damage == "counts" else 0,
                    "tps": 1,
                    "random_seed": 1,
                    "parsed": {"latency_ms": {"p95": 1, "p99": None if damage == "latency" else 1}},
                }
                argv = [
                    "overload",
                    "--config",
                    str(cfg),
                    "--profile",
                    "case",
                    "--root",
                    str(root),
                    "--evidence-dir",
                    str(out),
                ]
                with (
                    patch.object(sys, "argv", argv),
                    patch.object(overload_recovery_runner, "load_config", return_value=config),
                    patch.object(overload_recovery_runner, "which", return_value="test-double"),
                    patch.object(overload_recovery_runner, "safe_db_check", return_value=(True, "runner_test")),
                    patch.object(overload_recovery_runner, "run_one", return_value=result),
                    patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}),
                    contextlib.redirect_stdout(io.StringIO()),
                ):
                    rc = overload_recovery_runner.main()
                record = json.loads((out / "overload.case.json").read_text(encoding="utf-8"))
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")


if __name__ == "__main__":
    unittest.main()
