from __future__ import annotations

import contextlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import test_runner_verdicts as harness
import test_runner_contract_edges as scalar_harness

import canary_evaluator
import contention_fuzz_runner
import failover_recovery_runner
import plan_sensitivity_runner
import replica_consistency_runner


class SQLText(str):
    def format(self, *args):
        return SQLText(str(self).format(*args))

    def join(self, args):
        return SQLText(str(self).join(args))


SQL = SimpleNamespace(SQL=SQLText, Identifier=SQLText, Literal=lambda v: SQLText(str(v)))


class ReplicaConnection(harness.FakeConnection):
    def __init__(self, pg, replica):
        super().__init__(pg)
        self.replica = replica

    def execute(self, sql, params=None):
        super().execute(sql, params)
        if self.replica and sql.startswith("select count(*)"):
            self.value = self.pg.reads.pop(0) if self.pg.reads else self.pg.last_read
            self.pg.last_read = self.value
            if isinstance(self.value, Exception):
                raise self.value
        if sql.startswith("delete from") and self.pg.cleanup_error:
            raise RuntimeError("probe cleanup was not observed")

    def fetchone(self):
        if self.fetched:
            return None
        if self.sql.startswith("select count(*)"):
            self.fetched = True
            return (self.value,)
        return super().fetchone()


class ReplicaPostgres(harness.FakePostgres):
    def __init__(self, reads, *, cleanup_error=False):
        super().__init__()
        self.reads = list(reads)
        self.last_read = reads[-1]
        self.cleanup_error = cleanup_error

    def connect(self, dsn, **kwargs):
        return ReplicaConnection(self, dsn == "replica-double")


class PlanConnection(harness.FakeConnection):
    def execute(self, sql, params=None):
        super().execute(sql, params)
        if sql.startswith("SET LOCAL"):
            self.pg.generic = "force_generic_plan" in sql
        if sql.startswith("EXPLAIN"):
            self.pg.explains += 1
            if self.pg.error_after is not None and self.pg.explains > self.pg.error_after:
                raise RuntimeError("plan observer unavailable after first parameter set")

    def fetchone(self):
        if self.fetched:
            return None
        if self.sql.startswith("EXPLAIN"):
            self.fetched = True
            return (
                [{}]
                if self.pg.empty_plan
                else [{"Plan": {"Node Type": "Result", "Total Cost": 20 if self.pg.generic else 1}}],
            )
        return super().fetchone()


class PlanPostgres(harness.FakePostgres):
    def __init__(self, *, error_after=None, empty_plan=False):
        super().__init__()
        self.error_after = error_after
        self.empty_plan = empty_plan
        self.explains = 0
        self.generic = False

    def connect(self, *args, **kwargs):
        return PlanConnection(self)


class ObserverContinuityTests(unittest.TestCase):
    run_database_runner = harness.RunnerVerdictTests.run_database_runner

    def run_replica(self, pg, **overrides):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            cfg, out = root / "config.json", root / "evidence.json"
            cfg.write_text("{}", encoding="utf-8")
            case = {
                "id": "case",
                "replica_dsn_env": "RUNNER_REPLICA_DSN",
                "runs": 1,
                "poll_ms": 1,
                "max_visibility_ms": 3,
                "create_probe_table": False,
                **overrides,
            }
            config = {
                "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                "replica_consistency": [case],
            }
            argv = ["replica", "--config", str(cfg), "--case", "case", "--out", str(out)]
            with (
                patch.object(sys, "argv", argv),
                patch.object(replica_consistency_runner, "load_config", return_value=config),
                patch.object(replica_consistency_runner, "_psycopg", return_value=(pg, SQL)),
                patch.dict(os.environ, {"RUNNER_TEST_DSN": "primary-double", "RUNNER_REPLICA_DSN": "replica-double"}),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = replica_consistency_runner.main()
            return rc, json.loads(out.read_text(encoding="utf-8"))

    def run_plan(self, pg, *, parameter_sets=2, **overrides):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            cfg, out = root / "config.json", root / "evidence.json"
            cfg.write_text("{}", encoding="utf-8")
            (root / "query.sql").write_text("select $1", encoding="utf-8")
            (root / "params.json").write_text('{"params":[1]}', encoding="utf-8")
            case = {
                "id": "case",
                "query": "q",
                "param_types": ["int8"],
                "max_generic_custom_cost_ratio": 2,
                **overrides,
            }
            config = {
                "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                "plan_sensitivity": [case],
                "query": [
                    {
                        "id": "q",
                        "sql_file": "query.sql",
                        "parameter_set": [
                            {"name": str(i), "params_file": "params.json"} for i in range(parameter_sets)
                        ],
                    }
                ],
            }
            argv = ["plan", "--config", str(cfg), "--case", "case", "--root", str(root), "--out", str(out)]
            with (
                patch.object(sys, "argv", argv),
                patch.object(plan_sensitivity_runner, "load_config", return_value=config),
                patch.object(plan_sensitivity_runner, "_psycopg", return_value=(pg, SQL)),
                patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = plan_sensitivity_runner.main()
            return rc, json.loads(out.read_text(encoding="utf-8"))

    def run_canary(self, data, policy):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            metrics, policy_file, out = root / "metrics.json", root / "policy.json", root / "evidence.json"
            metrics.write_text(json.dumps(data), encoding="utf-8")
            policy_file.write_text(json.dumps(policy), encoding="utf-8")
            argv = ["canary", "--metrics", str(metrics), "--policy", str(policy_file), "--out", str(out)]
            with patch.object(sys, "argv", argv), contextlib.redirect_stdout(io.StringIO()):
                rc = canary_evaluator.main()
            return rc, json.loads(out.read_text(encoding="utf-8"))

    def test_fuzz_counts_first_failed_iteration_before_later_observer_error(self):
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
        self.assertEqual(record["observed"]["failed_iterations"], 1)

    def test_fuzz_example_cap_retains_first_failure_without_losing_counts(self):
        for cap in (0, 1):
            with self.subTest(cap=cap):
                case = {
                    "runs": 3,
                    "jitter_ms": 0,
                    "max_failure_examples": cap,
                    "session": [{"name": name, "sql": ["select action"]} for name in ("a", "b")],
                    "assertion": [{"id": "value", "sql": "select invariant", "expected": 2}],
                }
                rc, record = self.run_database_runner(
                    contention_fuzz_runner, "contention_fuzz", case, scalar_harness.AuditPostgres()
                )
                self.assertEqual(rc, 2)
                self.assertEqual(record["observed"]["failed_iterations"], 3)
                self.assertEqual(len(record["observed"]["failure_examples"]), cap)
                self.assertEqual(record["observed"]["first_failure"]["iteration"], 0)

    def test_replica_observer_failure_is_not_an_invisibility_assertion(self):
        rc, record = self.run_replica(ReplicaPostgres([RuntimeError("replica observer disconnected")]))
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")
        self.assertTrue(record["observed"]["samples"])
        self.assertIsNone(record["observed"]["samples"][0]["visible"])

    def test_replica_first_monotonic_failure_survives_later_observer_error(self):
        rc, record = self.run_replica(ReplicaPostgres([1, 0, RuntimeError("replica observer disconnected")]))
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertEqual(record["observed"]["monotonic_failures"], 1)
        self.assertIn(0, record["observed"]["samples"][0]["monotonic_reads"])

    def test_replica_valid_invisibility_survives_cleanup_error(self):
        rc, record = self.run_replica(ReplicaPostgres([0], cleanup_error=True))
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertTrue(record["observed"]["samples"])
        self.assertFalse(record["observed"]["samples"][0]["visible"])

    def test_replica_zero_runs_and_missing_budget_are_blocked(self):
        for options in ({"runs": 0}, {"max_visibility_ms": None}):
            with self.subTest(options=options):
                rc, record = self.run_replica(ReplicaPostgres([1]), **options)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_plan_first_cost_violation_survives_later_observer_error(self):
        rc, record = self.run_plan(PlanPostgres(error_after=2))
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertEqual(record["observed"]["cases"][0]["generic_custom_cost_ratio"], 20)

    def test_plan_needs_parameters_observations_and_explicit_comparator(self):
        for options, pg in (
            ({"param_types": []}, PlanPostgres()),
            ({"parameter_sets": 0}, PlanPostgres()),
            ({"max_generic_custom_cost_ratio": None}, PlanPostgres()),
            ({}, PlanPostgres(empty_plan=True)),
        ):
            with self.subTest(options=options, empty_plan=pg.empty_plan):
                rc, record = self.run_plan(pg, **options)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_canary_sample_gaps_are_blocked(self):
        for data in (
            {"control": {"latency": [1, 1, 1]}, "canary": {"latency": [1]}},
            {"control": {"latency": [1, 1, 1]}, "canary": {"latency": [1, 1, float("nan")]}},
        ):
            with self.subTest(data=data):
                rc, record = self.run_canary(
                    data, {"bootstrap_samples": 10, "metric": [{"name": "latency", "absolute_max": 2}]}
                )
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_canary_required_bake_missing_is_not_an_observed_violation(self):
        rc, record = self.run_canary(
            {"control": {"latency": [1, 1, 1]}, "canary": {"latency": [1, 1, 1]}},
            {"min_bake_seconds": 30, "bootstrap_samples": 10, "metric": [{"name": "latency", "absolute_max": 2}]},
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_canary_first_comparison_failure_survives_later_invalid_observation(self):
        rc, record = self.run_canary(
            {
                "control": {"latency": [1, 1, 1], "later": [1, 1, 1]},
                "canary": {"latency": [5, 5, 5], "later": [1, 1, 1]},
            },
            {
                "bootstrap_samples": 10,
                "metric": [{"name": "latency", "absolute_max": 2}, {"name": "later", "absolute_max": "bad-budget"}],
            },
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertEqual(record["observed"]["comparisons"][0]["canary_median"], 5)

    def test_canary_metric_without_comparator_is_blocked(self):
        rc, record = self.run_canary(
            {"control": {"latency": [1, 1, 1]}, "canary": {"latency": [1, 1, 1]}},
            {"bootstrap_samples": 10, "metric": [{"name": "latency"}]},
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_failover_natural_nonzero_workload_exit_is_blocked(self):
        case = {
            "workload_command": harness.command("pass"),
            "ready_oracle": [harness.command("pass")],
            "inject_failover_command": harness.command("pass"),
            "healthcheck_command": harness.command("pass"),
            "oracle_command": [harness.command("pass")],
        }
        process = SimpleNamespace(poll=Mock(side_effect=[None, 127]), returncode=127)
        with (
            patch.object(
                failover_recovery_runner, "run", return_value=subprocess.CompletedProcess(["double"], 0, "", "")
            ),
            patch.object(failover_recovery_runner.subprocess, "Popen", return_value=process),
            patch.object(failover_recovery_runner, "wait_process", return_value={"blocked": False, "returncode": 127}),
        ):
            rc, record = self.run_database_runner(
                failover_recovery_runner, "failover_rehearsal", case, harness.FakePostgres()
            )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")
        self.assertEqual(record["observed"]["workload"]["returncode"], 127)


if __name__ == "__main__":
    unittest.main()
