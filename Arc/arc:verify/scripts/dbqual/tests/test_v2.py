from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from lock_order_analyzer import analyze
from regression_compare import bootstrap_delta_ci
from workload_replay import load_trace
from evidence_report import render_markdown
from concurrency_scenario_runner import _graph_cycles


class LockOrderAnalyzerTests(unittest.TestCase):
    def test_detects_cross_operation_cycle(self):
        catalog = {
            "operations": [
                {"id": "a", "lock_order": ["account:*:low", "account:*:high"]},
                {"id": "b", "lock_order": ["account:*:high", "account:*:low"]},
            ]
        }
        out = analyze(catalog)
        self.assertEqual(len(out["cycles"]), 1)
        self.assertEqual(out["cycles"][0]["cycle"][0], out["cycles"][0]["cycle"][-1])

    def test_consistent_order_has_no_cycle(self):
        catalog = {
            "operations": [
                {"id": "a", "lock_order": ["account:min", "account:max"]},
                {"id": "b", "lock_order": ["account:min", "account:max"]},
            ]
        }
        out = analyze(catalog)
        self.assertEqual(out["cycles"], [])


class BootstrapRegressionTests(unittest.TestCase):
    def test_bootstrap_is_deterministic(self):
        cur = [120, 121, 119, 123, 122]
        base = [100, 99, 101, 102, 98]
        self.assertEqual(
            bootstrap_delta_ci(cur, base, seed=42, samples=500),
            bootstrap_delta_ci(cur, base, seed=42, samples=500),
        )

    def test_bootstrap_requires_repetitions(self):
        self.assertIsNone(bootstrap_delta_ci([1, 2], [1, 2, 3]))


class ReplayTraceTests(unittest.TestCase):
    def test_requires_session_offset_and_sql(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "trace.jsonl"
            p.write_text(json.dumps({"session": "a", "offset_ms": 1}), encoding="utf-8")
            with self.assertRaises(ValueError):
                load_trace(p)


    def test_transaction_control_and_dependency_trace(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "trace.jsonl"
            p.write_text(
                "\n".join([
                    json.dumps({"id":"a","session":"s1","offset_ms":0,"op":"begin"}),
                    json.dumps({"id":"b","session":"s2","offset_ms":1,"op":"sql","after":"a","sql":"select 1"}),
                    json.dumps({"id":"c","session":"s1","offset_ms":2,"op":"commit"}),
                ]), encoding="utf-8")
            rows = load_trace(p)
            self.assertEqual(rows[0]["op"], "begin")
            self.assertEqual(rows[1]["after"], ["a"])

    def test_unknown_dependency_is_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "trace.jsonl"
            p.write_text(json.dumps({"session":"s1","offset_ms":0,"sql":"select 1","after":"missing"}), encoding="utf-8")
            with self.assertRaises(ValueError):
                load_trace(p)

    def test_preserves_all_trace_rows(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "trace.jsonl"
            p.write_text(
                "\n".join(
                    [
                        json.dumps({"session": "a", "offset_ms": 0, "sql": "select 1"}),
                        json.dumps({"session": "b", "offset_ms": 10, "sql": "select 2"}),
                    ]
                ),
                encoding="utf-8",
            )
            self.assertEqual(len(load_trace(p)), 2)


class WaitForGraphTests(unittest.TestCase):
    def test_cycle_requires_current_edges(self):
        self.assertEqual(_graph_cycles({(1, 2)}), [])
        self.assertTrue(_graph_cycles({(1, 2), (2, 1)}))


class ReportV2Tests(unittest.TestCase):
    def test_report_contains_qualification_sections(self):
        evs = [
            {"id": "lock-order.analysis", "kind": "lock_order_analysis", "status": "PASS", "observed": {"operation_count": 2, "resource_count": 2, "edge_count": 1, "cycles": []}, "violations": [], "warnings": []},
            {"id": "longrun.resource", "kind": "longrun_monitor", "status": "PASS", "observed": {"samples": [], "peaks": {"connections": 1, "idle_in_tx": 0, "lock_waiters": 0, "max_xact_age_ms": 1}, "db_delta": {"deadlocks": 0, "temp_bytes": 0}, "wal_delta": {"wal_bytes": 10}}, "violations": [], "warnings": []},
        ]
        md = render_markdown(evs, {"status": "PASS", "failures": [], "warnings": []}, "test", None)
        self.assertIn("Deterministic concurrency / deadlock evidence", md)
        self.assertIn("Fault injection / recovery", md)
        self.assertIn("Long-run / resource stability", md)
        self.assertIn("Baseline regression", md)


if __name__ == "__main__":
    unittest.main()
