from __future__ import annotations

import contextlib
from contextlib import ExitStack
import io
import json
import os
import signal
import subprocess
import sys
import tempfile
import threading
from types import SimpleNamespace
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))

import concurrency_scenario_runner
import backup_restore_rehearsal
import compatibility_audit
import contention_fuzz_runner
import explain_capture
import failover_recovery_runner
import isolation_litmus_runner
import logical_replication_audit
import lock_order_analyzer
import longrun_monitor
import migration_rehearsal
import pgbench_runner
import query_property_runner
import retry_idempotency_runner
import transaction_scenario_runner
import workload_replay

SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"


def command(code: str) -> list[str]:
    return [sys.executable, "-B", "-c", code]


class FakePostgres:
    """Exercise harness verdicts; this supplies no PostgreSQL qualification evidence."""

    def __init__(
        self,
        *,
        observer_error=False,
        assertion_error=False,
        worker_error=False,
        worker_sqlstate=None,
        collation_error=False,
        subscription_error=False,
        explain_error=False,
        oracle_rows=None,
        worker_timestamp=None,
        actual=1,
    ):
        self.observer_error = observer_error
        self.assertion_error = assertion_error
        self.worker_error = worker_error
        self.worker_sqlstate = worker_sqlstate
        self.actual = actual
        self.sqls = []
        self.collation_error = collation_error
        self.subscription_error = subscription_error
        self.explain_error = explain_error
        self.oracle_rows = [(1,)] if oracle_rows is None else oracle_rows
        self.worker_timestamp = worker_timestamp

    def connect(self, *args, **kwargs):
        return FakeConnection(self)


class FakeConnection:
    def __init__(self, pg):
        self.pg = pg
        self.description = None
        self.sql = ""
        self.fetched = False

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()

    def cursor(self):
        return self

    def execute(self, sql, params=None):
        self.pg.sqls.append(sql)
        self.sql = sql
        self.description = None
        self.fetched = False
        if "pg_stat_activity" in sql and self.pg.observer_error:
            raise RuntimeError("observer unavailable")
        if "pg_database_collation_actual_version" in sql and self.pg.collation_error:
            raise ValueError("collation observer unavailable")
        if "pg_stat_subscription_stats" in sql:
            if self.pg.subscription_error:
                raise ValueError("subscription stats unavailable")
            self.description = [SimpleNamespace(name="apply_error_count")]
        if sql.startswith("EXPLAIN") and self.pg.explain_error:
            raise TypeError("explain observer unavailable")
        if sql == "select invariant" and self.pg.assertion_error:
            raise RuntimeError("assertion query unavailable")
        if self.pg.worker_error and (
            sql == "select action"
            or (threading.current_thread() is not threading.main_thread() and "sqlverify_litmus" in sql)
        ):
            error = RuntimeError("worker action error")
            error.sqlstate = self.pg.worker_sqlstate
            raise error
        if sql in ("select invariant", "select current_database()"):
            self.description = ("value",)

    def fetchone(self):
        if self.fetched:
            return None
        self.fetched = True
        if "extract(epoch" in self.sql:
            return {"apply_age_ms": 1}
        if "current_database() as db" in self.sql:
            return {"db": "runner_test", "server_version_num": 180000, "version_num": 180000, "version": "fixture"}
        if "current_database(), current_setting" in self.sql:
            return ("runner_test", 180000)
        return ("runner_test",) if self.sql == "select current_database()" else (self.pg.actual,)

    def fetchall(self):
        if "pg_stat_subscription_stats" in self.sql:
            return [{"apply_error_count": 0}]
        if "pg_stat_subscription order" in self.sql:
            return [{"pid": 1, "latest_end_time": self.pg.worker_timestamp}]
        if self.sql == "select query":
            return [(1,)]
        if self.sql == "select oracle":
            return self.pg.oracle_rows
        return []

    def rollback(self):
        pass

    def commit(self):
        pass

    def close(self):
        pass


class RunnerVerdictTests(unittest.TestCase):
    def run_database_runner(self, module, section, scenario, pg):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            cfg = root / "config.json"
            cfg.write_text("{}", encoding="utf-8")
            (root / "up.sql").write_text("select action", encoding="utf-8")
            (root / "query.sql").write_text("select query", encoding="utf-8")
            (root / "oracle.sql").write_text("select oracle", encoding="utf-8")
            (root / "corpus.json").write_text("{}", encoding="utf-8")
            (root / "params.json").write_text('{"params":[]}', encoding="utf-8")
            (root / "trace.jsonl").write_text(
                json.dumps({"id": "action", "session": "a", "offset_ms": 0, "sql": "select action"}) + "\n",
                encoding="utf-8",
            )
            out = root / "evidence.json"
            config = {
                "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                section: [dict(id="case", **scenario)],
            }
            option = {
                "contention_fuzz": "--fuzz",
                "migration_rehearsal": "--migration",
                "workload_replay": "--replay",
                "property_test": "--property",
                "compatibility_audit": "--case",
                "logical_replication_audit": "--case",
                "isolation_litmus": "--case",
                "retry_idempotency": "--case",
                "query": "--query",
            }.get(section, "--scenario")
            if section in ("backup_restore_rehearsal", "failover_rehearsal"):
                option = "--case"
            argv = [module.__name__, "--config", str(cfg), option, "case", "--out", str(out)]
            if section not in ("logical_replication_audit", "isolation_litmus"):
                argv += ["--root", str(root)]
            if section == "query":
                argv += ["--parameter-set", "case"]
            if section in ("backup_restore_rehearsal", "failover_rehearsal"):
                argv += ["--ack-isolated-environment"]
            driver = (pg, None) if section in ("compatibility_audit", "logical_replication_audit") else pg
            with ExitStack() as stack:
                stack.enter_context(patch.object(sys, "argv", argv))
                stack.enter_context(patch.object(module, "load_config", return_value=config))
                if hasattr(module, "_psycopg"):
                    stack.enter_context(
                        patch.object(module, "_psycopg", side_effect=driver)
                        if isinstance(driver, BaseException)
                        else patch.object(module, "_psycopg", return_value=driver)
                    )
                stack.enter_context(patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}))
                stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
                if section == "property_test":
                    stack.enter_context(
                        patch.object(
                            module, "generate", return_value={"count": 1, "seed": 1, "cases": [{"params": []}]}
                        )
                    )
                rc = module.main()
            return rc, json.loads(out.read_text(encoding="utf-8"))

    def scenario(self):
        return {
            "timeout_s": 1,
            "require_observer": False,
            "session": [{"name": name, "step": [{"op": "sql", "sql": "select action"}]} for name in ("a", "b")],
            "assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}],
        }

    def test_transaction_and_concurrency_observer_gaps_are_blocked(self):
        for module, section in (
            (transaction_scenario_runner, "transaction_scenario"),
            (concurrency_scenario_runner, "concurrency_scenario"),
        ):
            for error in (False, True):
                with self.subTest(module=module.__name__, observer_error=error):
                    sc = self.scenario()
                    sc["require_observer"] = True
                    rc, record = self.run_database_runner(module, section, sc, FakePostgres(observer_error=error))
                    self.assertEqual(rc, 2)
                    self.assertEqual(record["status"], "BLOCKED")

    def test_transaction_and_concurrency_harness_errors_are_blocked(self):
        for module, section in (
            (transaction_scenario_runner, "transaction_scenario"),
            (concurrency_scenario_runner, "concurrency_scenario"),
        ):
            for fault in ("assertion_error", "worker_error"):
                with self.subTest(module=module.__name__, fault=fault):
                    rc, record = self.run_database_runner(
                        module, section, self.scenario(), FakePostgres(**{fault: True})
                    )
                    self.assertEqual(rc, 2)
                    self.assertEqual(record["status"], "BLOCKED")

    def test_valid_independent_invariant_violation_stays_fail(self):
        for module, section in (
            (transaction_scenario_runner, "transaction_scenario"),
            (concurrency_scenario_runner, "concurrency_scenario"),
        ):
            with self.subTest(module=module.__name__):
                rc, record = self.run_database_runner(module, section, self.scenario(), FakePostgres(actual=2))
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "FAIL")
                self.assertFalse(record["observed"]["assertions"][0]["passed"])

    def test_contention_worker_exception_cannot_disappear_into_pass(self):
        sc = {
            "runs": 1,
            "jitter_ms": 0,
            "session": [{"name": n, "sql": ["select action"]} for n in ("a", "b")],
            "assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}],
        }
        rc, record = self.run_database_runner(
            contention_fuzz_runner, "contention_fuzz", sc, FakePostgres(worker_error=True)
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_contention_capture_limit_cannot_hide_an_invariant_failure(self):
        sc = {
            "runs": 1,
            "jitter_ms": 0,
            "max_failure_examples": 0,
            "session": [{"name": n, "sql": ["select action"]} for n in ("a", "b")],
            "assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}],
        }
        rc, record = self.run_database_runner(contention_fuzz_runner, "contention_fuzz", sc, FakePostgres(actual=2))
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")

    def test_observed_unexpected_sqlstate_stays_fail(self):
        for module, section in (
            (transaction_scenario_runner, "transaction_scenario"),
            (concurrency_scenario_runner, "concurrency_scenario"),
        ):
            with self.subTest(module=module.__name__):
                rc, record = self.run_database_runner(
                    module, section, self.scenario(), FakePostgres(worker_error=True, worker_sqlstate="40001")
                )
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "FAIL")

    def test_migration_assertion_exception_is_blocked(self):
        rc, record = self.run_database_runner(
            migration_rehearsal,
            "migration_rehearsal",
            {"up_sql_file": "up.sql", "post_assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}]},
            FakePostgres(assertion_error=True),
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_migration_candidate_sql_error_stays_fail(self):
        rc, record = self.run_database_runner(
            migration_rehearsal,
            "migration_rehearsal",
            {"up_sql_file": "up.sql", "post_assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}]},
            FakePostgres(worker_error=True, worker_sqlstate="42601"),
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")

    def test_migration_does_not_apply_after_failed_precondition(self):
        pg = FakePostgres(actual=2)
        rc, record = self.run_database_runner(
            migration_rehearsal,
            "migration_rehearsal",
            {"up_sql_file": "up.sql", "pre_assertion": [{"id": "initial", "sql": "select invariant", "expected": 1}]},
            pg,
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")
        self.assertNotIn("select action", pg.sqls, "migration ran after its required initial state was rejected")

    def test_missing_interleaving_does_not_manufacture_sqlstate_failure(self):
        for module, section in (
            (transaction_scenario_runner, "transaction_scenario"),
            (concurrency_scenario_runner, "concurrency_scenario"),
        ):
            with self.subTest(module=module.__name__):
                sc = self.scenario()
                sc.update(require_observer=True, required_sqlstates=["40001"])
                rc, record = self.run_database_runner(module, section, sc, FakePostgres())
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_longrun_observer_exception_is_blocked(self):
        with tempfile.TemporaryDirectory() as td:
            out = Path(td) / "evidence.json"
            argv = ["longrun", "--dsn-env", "RUNNER_TEST_DSN", "--duration-s", "1", "--out", str(out)]
            with (
                patch.object(sys, "argv", argv),
                patch.object(longrun_monitor, "_psycopg", return_value=FakePostgres()),
                patch.object(longrun_monitor, "snap", side_effect=RuntimeError("observer unavailable")),
                patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = longrun_monitor.main()
            self.assertEqual(rc, 2)
            self.assertEqual(json.loads(out.read_text(encoding="utf-8"))["status"], "BLOCKED")

    def test_unreadable_lock_catalog_is_not_a_business_failure(self):
        with tempfile.TemporaryDirectory() as td:
            out = Path(td) / "evidence.json"
            with (
                patch.object(
                    sys, "argv", ["lock-order", "--catalog", str(Path(td) / "missing.json"), "--out", str(out)]
                ),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = lock_order_analyzer.main()
            self.assertEqual(rc, 2)
            self.assertEqual(json.loads(out.read_text(encoding="utf-8"))["status"], "BLOCKED")

    def test_missing_collation_and_replication_observations_cannot_pass(self):
        cases = [
            (compatibility_audit, "compatibility_audit", {}, FakePostgres(collation_error=True)),
            (
                logical_replication_audit,
                "logical_replication_audit",
                {"subscriber_dsn_env": "RUNNER_TEST_DSN"},
                FakePostgres(subscription_error=True),
            ),
        ]
        for module, section, sc, pg in cases:
            with self.subTest(module=module.__name__):
                rc, record = self.run_database_runner(module, section, sc, pg)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_valid_replication_dictionary_rows_remain_usable(self):
        rc, record = self.run_database_runner(
            logical_replication_audit,
            "logical_replication_audit",
            {"subscriber_dsn_env": "RUNNER_TEST_DSN"},
            FakePostgres(),
        )
        self.assertEqual(rc, 0)
        self.assertEqual(record["status"], "PASS")
        self.assertEqual(record["observed"]["apply_error_count"], 0)

    def test_replication_freshness_uses_dictionary_rows(self):
        rc, record = self.run_database_runner(
            logical_replication_audit,
            "logical_replication_audit",
            {"subscriber_dsn_env": "RUNNER_TEST_DSN", "max_apply_age_ms": 10},
            FakePostgres(worker_timestamp="2026-10-30T00:00:00Z"),
        )
        self.assertEqual(rc, 0)
        self.assertEqual(record["status"], "PASS")
        self.assertEqual(record["observed"]["max_apply_age_ms"], 1)

    def test_previous_valid_failure_survives_later_observer_failure(self):
        rc, record = self.run_database_runner(
            compatibility_audit, "compatibility_audit", {"expected_major": 17}, FakePostgres(collation_error=True)
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "FAIL")

    def test_explain_observer_error_is_blocked(self):
        sc = {
            "sql_file": "query.sql",
            "param_style": "psycopg",
            "parameter_set": [{"name": "case", "params_file": "params.json"}],
        }
        rc, record = self.run_database_runner(explain_capture, "query", sc, FakePostgres(explain_error=True))
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_isolation_and_replay_worker_errors_cannot_pass(self):
        cases = [
            (isolation_litmus_runner, "isolation_litmus", {"type": "write_skew", "min_final_on_call": 1}),
            (workload_replay, "workload_replay", {"trace_file": "trace.jsonl", "max_runtime_s": 1}),
        ]
        for module, section, sc in cases:
            with self.subTest(module=module.__name__):
                rc, record = self.run_database_runner(module, section, sc, FakePostgres(worker_error=True))
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_invalid_count_oracle_is_blocked(self):
        sc = {
            "kind": "count_matches_rows",
            "query_file": "query.sql",
            "oracle_query_file": "oracle.sql",
            "corpus_spec": "corpus.json",
            "param_style": "psycopg",
            "max_cases": 1,
        }
        for rows in ([], [(None,)], [(1.5,)], [(True,)]):
            with self.subTest(rows=rows):
                rc, record = self.run_database_runner(
                    query_property_runner, "property_test", sc, FakePostgres(oracle_rows=rows)
                )
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_zero_property_cases_cannot_pass(self):
        sc = {
            "kind": "count_matches_rows",
            "query_file": "query.sql",
            "oracle_query_file": "oracle.sql",
            "corpus_spec": "corpus.json",
            "param_style": "psycopg",
            "max_cases": 0,
        }
        rc, record = self.run_database_runner(query_property_runner, "property_test", sc, FakePostgres())
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_nullable_equality_does_not_evaluate_unrequested_operators(self):
        sc = {
            "operation_command": command("pass"),
            "attempts": 1,
            "assertion": [{"id": "nullable", "sql": "select invariant", "expected": None, "op": "=="}],
        }
        rc, record = self.run_database_runner(
            retry_idempotency_runner, "retry_idempotency", sc, FakePostgres(actual=None)
        )
        self.assertEqual(rc, 0)
        self.assertEqual(record["status"], "PASS")

    def test_restore_and_failover_cleanup_failure_cannot_pass(self):
        cases = [
            (
                backup_restore_rehearsal,
                "backup_restore_rehearsal",
                {
                    "verify_backup": False,
                    "restore_command": command("pass"),
                    "healthcheck_command": command("pass"),
                    "restored_dsn_env": "RUNNER_TEST_DSN",
                    "assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}],
                },
            ),
            (
                failover_recovery_runner,
                "failover_rehearsal",
                {
                    "inject_failover_command": command("pass"),
                    "healthcheck_command": command("pass"),
                    "oracle_command": [command("pass")],
                },
            ),
        ]
        for module, section, sc in cases:
            with self.subTest(module=module.__name__):
                sc["cleanup_command"] = command("raise SystemExit(2)")
                rc, record = self.run_database_runner(module, section, sc, FakePostgres())
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_missing_restore_driver_is_not_a_liveness_failure(self):
        sc = {
            "verify_backup": False,
            "restore_command": command("pass"),
            "restored_dsn_env": "RUNNER_TEST_DSN",
            "healthcheck_timeout_s": 0.02,
            "healthcheck_poll_s": 0.005,
            "cleanup_command": command("pass"),
            "assertion": [{"id": "effect", "sql": "select invariant", "expected": 1}],
        }
        rc, record = self.run_database_runner(
            backup_restore_rehearsal, "backup_restore_rehearsal", sc, ImportError("fixture dependency unavailable")
        )
        self.assertEqual(rc, 2)
        self.assertEqual(record["status"], "BLOCKED")

    def test_required_after_benchmark_observation_cannot_be_a_warning(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            cfg_path = root / "config.json"
            cfg_path.write_text("{}", encoding="utf-8")
            (root / "query.sql").write_text("select 1", encoding="utf-8")
            evdir = root / "evidence"
            cfg = {
                "database": {"dsn_env": "RUNNER_TEST_DSN", "safe_database_pattern": "_test$"},
                "workload": [
                    {
                        "id": "case",
                        "clients": [1],
                        "repetitions": 1,
                        "duration_s": 1,
                        "scripts": [{"path": "query.sql"}],
                        "require_pg_stat_statements": True,
                    }
                ],
            }
            parsed = {"latency_ms": {"p50": 1, "p95": 1, "p99": 1, "max": 1}}
            result = {
                "id": "case.c1.r1",
                "returncode": 0,
                "tps": 1,
                "processed": 1,
                "failed": 0,
                "parsed": parsed,
                "stderr": "",
                "stdout": "",
            }
            argv = [
                "pgbench",
                "--config",
                str(cfg_path),
                "--workload",
                "case",
                "--root",
                str(root),
                "--evidence-dir",
                str(evdir),
            ]
            with (
                patch.object(sys, "argv", argv),
                patch.object(pgbench_runner, "load_config", return_value=cfg),
                patch.object(pgbench_runner, "which", return_value="test-double"),
                patch.object(pgbench_runner, "safe_db_check", return_value=(True, "runner_test")),
                patch.object(pgbench_runner, "run_one", return_value=result),
                patch.object(pgbench_runner, "capture_pgss", side_effect=[{}, None]),
                patch.dict(os.environ, {"RUNNER_TEST_DSN": "test-double-only"}),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                rc = pgbench_runner.main()
            record = json.loads((evdir / "bench.case.c1.json").read_text(encoding="utf-8"))
            self.assertEqual(rc, 2)
            self.assertEqual(record["status"], "BLOCKED")


class FaultRunnerTests(unittest.TestCase):
    def spec(self, root):
        ready, recovered = root / "ready", root / "recovered"
        target = (
            "import pathlib,threading,time;"
            f"pathlib.Path({str(ready)!r}).write_text('ready');"
            f"end=time.monotonic()+2\nwhile not pathlib.Path({str(recovered)!r}).exists():\n"
            " if time.monotonic()>end: raise RuntimeError('recovery never observed')\n"
            " threading.Event().wait(.01)\n"
        )
        await_ready = (
            "import pathlib,threading,time;end=time.monotonic()+1\n"
            f"while not pathlib.Path({str(ready)!r}).exists():\n"
            " if time.monotonic()>end: raise RuntimeError('target never ready')\n"
            " threading.Event().wait(.01)\n"
        )
        return {
            "id": "case",
            "isolated_environment_required": True,
            "timeout_s": 3,
            "command_timeout_s": 1,
            "target_timeout_s": 1,
            "fault_after_s": 0,
            "fault_hold_s": 0,
            "target": command(target),
            "ready_oracle": [command(await_ready)],
            "inject": [command("pass")],
            "fault_oracle": [command("pass")],
            "recover": [command(f"import pathlib;pathlib.Path({str(recovered)!r}).write_text('recovered')")],
            "recovery_oracle": [command(f"import pathlib;assert pathlib.Path({str(recovered)!r}).exists()")],
            "oracle": [command("pass")],
            "oracle_failure_returncodes": [1],
            "cleanup": [command("pass")],
        }

    def run_fault(self, root, spec):
        config, out = root / "scenario.json", root / "evidence.json"
        config.write_text(json.dumps(spec), encoding="utf-8")
        cp = subprocess.run(
            [
                sys.executable,
                "-B",
                str(SCRIPTS / "fault_scenario_runner.py"),
                "--scenario-file",
                str(config),
                "--out",
                str(out),
                "--ack-isolated-environment",
            ],
            capture_output=True,
            text=True,
            check=False,
            timeout=8,
        )
        self.assertTrue(out.is_file(), f"runner lost its verdict: {cp.stderr}")
        return cp.returncode, json.loads(out.read_text(encoding="utf-8"))

    def test_missing_action_or_oracle_cannot_pass(self):
        for field in ("target", "ready_oracle", "fault_oracle", "recovery_oracle", "oracle"):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as td:
                root = Path(td)
                spec = self.spec(root)
                del spec[field]
                rc, record = self.run_fault(root, spec)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")

    def test_cleanup_error_is_blocked_and_preserved(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            spec = self.spec(root)
            spec["cleanup"] = [command("raise SystemExit(2)")]
            rc, record = self.run_fault(root, spec)
            self.assertEqual(rc, 2)
            self.assertEqual(record["status"], "BLOCKED")
            self.assertTrue(
                any(r["phase"] == "cleanup" and r["returncode"] == 2 for r in record["observed"]["records"])
            )

    def test_oracle_start_failure_is_blocked_but_asserted_violation_is_fail(self):
        for oracle, expected in (
            (["/nonexistent/sqlverify-oracle"], "BLOCKED"),
            (command("raise SystemExit(1)"), "FAIL"),
        ):
            with self.subTest(expected=expected), tempfile.TemporaryDirectory() as td:
                root = Path(td)
                spec = self.spec(root)
                spec["oracle"] = [oracle]
                rc, record = self.run_fault(root, spec)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], expected)

    @unittest.skipUnless(os.name == "posix", "requires POSIX process observation")
    def test_injection_failure_stops_target_before_returning(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            spec = self.spec(root)
            pidfile = root / "target.pid"
            spec["target"] = command(
                f"import os,pathlib,threading;pathlib.Path({str(pidfile)!r}).write_text(str(os.getpid()));"
                f"pathlib.Path({str(root / 'ready')!r}).write_text('ready');threading.Event().wait()"
            )
            spec["inject"] = [command("raise SystemExit(2)")]
            try:
                rc, record = self.run_fault(root, spec)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")
                self.assertTrue(pidfile.is_file(), "target never started")
                pid = int(pidfile.read_text(encoding="utf-8"))
                state = subprocess.run(
                    ["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True, check=False, timeout=1
                ).stdout.strip()
                self.assertTrue(not state or state.startswith("Z"), f"target survived failed injection: {state}")
            finally:
                if pidfile.is_file():
                    try:
                        os.kill(int(pidfile.read_text(encoding="utf-8")), signal.SIGKILL)
                    except ProcessLookupError:
                        pass

    @unittest.skipUnless(os.name == "posix", "requires POSIX process observation")
    def test_target_exit_before_injection_cleans_its_descendants(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            spec = self.spec(root)
            pidfile = root / "descendant.pid"
            child = command("import threading;threading.Event().wait()")
            spec["target"] = command(
                f"import subprocess,pathlib;p=subprocess.Popen({child!r});"
                f"pathlib.Path({str(pidfile)!r}).write_text(str(p.pid));"
                f"pathlib.Path({str(root / 'ready')!r}).write_text('ready')"
            )
            try:
                rc, record = self.run_fault(root, spec)
                self.assertEqual(rc, 2)
                self.assertEqual(record["status"], "BLOCKED")
                pid = int(pidfile.read_text(encoding="utf-8"))
                state = subprocess.run(
                    ["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True, check=False, timeout=1
                ).stdout.strip()
                self.assertTrue(not state or state.startswith("Z"), f"target descendant survived: {state}")
            finally:
                if pidfile.is_file():
                    try:
                        os.kill(int(pidfile.read_text(encoding="utf-8")), signal.SIGKILL)
                    except ProcessLookupError:
                        pass


if __name__ == "__main__":
    unittest.main()
