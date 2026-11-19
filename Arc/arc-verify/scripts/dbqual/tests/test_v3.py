from __future__ import annotations

import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from evidence_report import render_markdown
from maintenance_health_audit import evaluate_health
from plan_analyzer import analyze_explain
from plan_sensitivity_runner import plan_signature, validate_param_type_name
from qualification_coverage_gate import evaluate_coverage


class PlanSensitivityTests(unittest.TestCase):
    def test_plan_analyzer_exposes_root_cost(self):
        raw = [{"Plan": {"Node Type": "Index Scan", "Startup Cost": 1.5, "Total Cost": 42.0, "Plan Rows": 2}}]
        out = analyze_explain(raw)
        self.assertEqual(out["startup_cost"], 1.5)
        self.assertEqual(out["total_cost"], 42.0)

    def test_plan_signature_tracks_node_and_index(self):
        raw = [{"Plan": {"Node Type": "Nested Loop", "Plans": [
            {"Node Type": "Index Scan", "Index Name": "a_idx"},
            {"Node Type": "Seq Scan"},
        ]}}]
        sig = plan_signature(raw)
        self.assertIn("Nested Loop", sig)
        self.assertIn("idx:a_idx", sig)
        self.assertIn("Seq Scan", sig)

    def test_plan_type_declaration_is_fail_closed(self):
        self.assertEqual(validate_param_type_name("pg_catalog.int8[]"), "pg_catalog.int8[]")
        with self.assertRaises(ValueError):
            validate_param_type_name("int8) AS SELECT pg_sleep(10); --")


class MaintenancePolicyTests(unittest.TestCase):
    def test_health_policy_fails_explicit_risks(self):
        obs = {
            "connections_utilization": 0.9,
            "oldest_transaction_s": 100,
            "oldest_idle_in_transaction_s": 1,
            "database_xid_age": 10,
            "max_table_xid_age": 10,
            "max_dead_tuple_ratio": 0.01,
            "max_mod_since_analyze_ratio": 0.01,
            "oldest_statistics_age_s": 10,
            "max_sequence_utilization": 0.1,
            "max_inactive_slot_retained_bytes": 0,
            "invalid_indexes": 1,
            "unvalidated_constraints": 0,
            "prepared_transactions": 0,
            "lost_replication_slots": 0,
            "never_analyzed_tables": 0,
        }
        policy = {"max_connections_utilization": 0.8, "max_oldest_transaction_s": 60, "require_no_invalid_indexes": True}
        failures = evaluate_health(obs, policy)
        self.assertEqual(len(failures), 3)

    def test_missing_optional_metric_is_not_zero_failure(self):
        failures = evaluate_health({}, {"max_database_xid_age": 10})
        self.assertEqual(failures, [])


class QualificationCoverageTests(unittest.TestCase):
    def test_explicit_applicability_and_evidence(self):
        manifest = {
            "strict_complete": False,
            "claims": [
                {"gate":"G1","applicable":True,"require":[{"kind":"invariant_audit"}]},
                {"gate":"G18","applicable":False,"rationale":"periodic DR gate not in this cycle"},
            ],
        }
        out = evaluate_coverage([{"id":"invariant.audit","kind":"invariant_audit","status":"PASS"}], manifest)
        self.assertEqual(out["failures"], [])

    def test_blocked_is_not_pass_and_false_requires_rationale(self):
        manifest = {
            "claims": [
                {"gate":"G7","applicable":True,"require":[{"kind":"concurrency_scenario"}]},
                {"gate":"G18","applicable":False},
            ]
        }
        out = evaluate_coverage([{"id":"concurrency.x","kind":"concurrency_scenario","status":"BLOCKED"}], manifest)
        self.assertTrue(any("does not satisfy" in x for x in out["failures"]))
        self.assertTrue(any("requires rationale" in x for x in out["failures"]))


class ExpertReportTests(unittest.TestCase):
    def test_v3_sections_exist(self):
        evs = [
            {"id":"qualification.coverage","kind":"qualification_coverage","status":"PASS","observed":{"risk_class":"DB-R4","profile":"prerelease","claims":[]},"violations":[],"warnings":[]},
            {"id":"maintenance.health","kind":"maintenance_health","status":"PASS","observed":{"connections_utilization":0.1},"violations":[],"warnings":[]},
            {"id":"physical.integrity","kind":"physical_integrity","status":"PASS","observed":{"returncode":0},"violations":[],"warnings":[]},
            {"id":"replica.x","kind":"replica_consistency","status":"PASS","observed":{"runs":1,"visible_runs":1,"visibility_ms":{"p95":2,"p99":2,"max":2},"monotonic_failures":0},"violations":[],"warnings":[]},
        ]
        md = render_markdown(evs, {"status":"PASS","failures":[],"warnings":[]}, "test", None)
        for heading in [
            "Qualification coverage / applicability",
            "Prepared-plan / parameter sensitivity",
            "Maintenance / MVCC / WAL-retention health",
            "Replica visibility / consistency",
            "Backup verification / restore / PITR rehearsal",
            "Failover / failback qualification",
            "Transaction history / model checking",
        ]:
            self.assertIn(heading, md)


if __name__ == "__main__":
    unittest.main()
