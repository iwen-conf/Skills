from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from pgbench_log import parse_files
from plan_analyzer import analyze_explain
from release_gate import evaluate
from parameter_corpus import generate
from pgbench_runner import diff_pgss, summarize
from sqlverify_common import json_dump, json_load
from query_property_runner import compile_sql, compare
from explain_capture import potential_write
from decimal import Decimal


class PgbenchLogTests(unittest.TestCase):
    def test_parse(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "log"
            p.write_text("0 0 1000 0 1 1\n0 1 2000 0 1 2\n0 2 deadlock 0 1 3\n")
            out = parse_files([p], has_rate=False, has_retries=False)
            self.assertEqual(out["transactions_logged"], 3)
            self.assertEqual(out["failures"]["deadlock"], 1)
            self.assertAlmostEqual(out["latency_ms"]["p50"], 1.5)

    def test_rate_log_separates_execution_and_schedule_latency(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "log"
            p.write_text("0 0 2000 0 1 1 3000\n")
            out = parse_files([p], has_rate=True, has_retries=False)
            self.assertEqual(out["latency_ms"]["p50"], 2.0)
            self.assertEqual(out["schedule_lag_ms"]["p50"], 3.0)
            self.assertEqual(out["scheduled_latency_ms"]["p50"], 5.0)

    def test_summary_uses_full_pgbench_failure_counts(self):
        parsed = {"latency_ms":{"p50":1.0,"p95":2.0,"p99":3.0,"max":4.0}, "schedule_lag_ms":None, "scheduled_latency_ms":None, "failures":{"serialization":1}}
        runs = [
            {"id":"r1","returncode":0,"tps":100.0,"processed":90,"failed":10,"retried_transactions":5,"total_retries":8,"skipped":None,"parsed":parsed},
            {"id":"r2","returncode":0,"tps":110.0,"processed":95,"failed":5,"retried_transactions":2,"total_retries":3,"skipped":None,"parsed":parsed},
        ]
        out = summarize(runs, 16, {"log_sampling_rate":0.01})
        self.assertAlmostEqual(out["failure_rate"], 15/200)
        self.assertEqual(out["total_retries"], 11)
        self.assertEqual(out["log_sampling_rate"], 0.01)


class PlanTests(unittest.TestCase):
    def test_estimate_error_and_temp(self):
        raw = [{
            "Planning Time": 1.0,
            "Execution Time": 10.0,
            "Plan": {
                "Node Type": "Seq Scan", "Plan Rows": 10, "Actual Rows": 1000, "Actual Loops": 1,
                "Temp Written Blocks": 2, "Shared Read Blocks": 4
            }
        }]
        out = analyze_explain(raw)
        self.assertEqual(out["node_types"]["Seq Scan"], 1)
        self.assertEqual(out["max_estimate_error_ratio"], 100.0)
        self.assertEqual(out["temp_written_blocks_sum_nodes"], 2)

    def test_cardinality_error_uses_per_loop_actual_rows(self):
        raw = [{"Plan":{"Node Type":"Index Scan","Plan Rows":1,"Actual Rows":1,"Actual Loops":100},"Execution Time":1.0}]
        out = analyze_explain(raw)
        self.assertEqual(out["max_estimate_error_ratio"], 1.0)
        self.assertEqual(out["nodes"][0]["actual_total_rows"], 100)

    def test_bitmap_boolean_nodes_do_not_create_false_infinite_qerror(self):
        raw = [{"Plan":{"Node Type":"BitmapAnd","Plan Rows":10,"Actual Rows":0,"Actual Loops":1},"Execution Time":1.0}]
        out = analyze_explain(raw)
        self.assertEqual(out["max_estimate_error_ratio"], 1.0)


class GateTests(unittest.TestCase):
    def test_missing_required_fails(self):
        out = evaluate([], {"required_evidence": [{"id": "x", "allowed_status": ["PASS"]}]})
        self.assertEqual(out["status"], "FAIL")

    def test_metric_rule(self):
        ev = [{"id":"bench.x", "status":"PASS", "violations":[], "observed":{"latency_ms":{"p95":50}}}]
        ok = evaluate(ev, {"required_evidence":[{"id":"bench.x"}], "metric_rule":[{"evidence_id":"bench.x","path":"observed.latency_ms.p95","op":"<=","threshold":100}]})
        self.assertEqual(ok["status"], "PASS")


class PgStatStatementsTests(unittest.TestCase):
    def test_delta(self):
        before = {"1": {"queryid":"1", "calls":10, "total_exec_time":100, "rows":20, "shared_blks_hit":5, "shared_blks_read":2, "temp_blks_read":0, "temp_blks_written":0, "wal_bytes":Decimal("10")}}
        after = {"1": {"queryid":"1", "calls":14, "total_exec_time":180, "rows":28, "shared_blks_hit":9, "shared_blks_read":3, "temp_blks_read":0, "temp_blks_written":1, "wal_bytes":Decimal("30")}}
        out = diff_pgss(before, after)
        self.assertEqual(out[0]["calls"], 4.0)
        self.assertEqual(out[0]["mean_exec_time_ms_delta"], 20.0)
        self.assertEqual(out[0]["wal_bytes"], 20.0)


class JsonEvidenceTests(unittest.TestCase):
    def test_decimal_serialization(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "x.json"
            json_dump(p, {"x": Decimal("1.25")})
            self.assertEqual(json_load(p)["x"], 1.25)


class ExplainSafetyTests(unittest.TestCase):
    def test_detects_write_cte(self):
        self.assertEqual(potential_write("WITH x AS (UPDATE t SET a=1 RETURNING *) SELECT * FROM x"), "UPDATE")

    def test_does_not_flag_keyword_inside_literal(self):
        self.assertIsNone(potential_write("SELECT 'update t set a=1'"))


class QueryPropertyTests(unittest.TestCase):
    def test_pg_dollar_compilation_preserves_repeated_params(self):
        sql, params = compile_sql("select $2, $1, $2", ["a", "b"], "pg_dollar")
        self.assertEqual(sql, "select %s, %s, %s")
        self.assertEqual(params, ["b", "a", "b"])

    def test_count_matches_rows(self):
        ok, details = compare("count_matches_rows", [(1,), (2,)], [(2,)])
        self.assertTrue(ok)
        self.assertEqual(details["actual_rows"], 2)

    def test_subset_multiset(self):
        ok, _ = compare("subset", [(1,), (1,)], [(1,), (1,), (2,)])
        self.assertTrue(ok)


class CorpusTests(unittest.TestCase):
    def test_deterministic(self):
        spec = {"seed":7,"random_runs":10,"fixed_cases":[{"name":"a","params":[1]}],"parameters":[{"name":"x","values":[1,2,3]}]}
        self.assertEqual(generate(spec), generate(spec))


if __name__ == "__main__":
    unittest.main()
