from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from evidence_report import render_markdown
from qualification_coverage_gate import evaluate_coverage
from release_gate import evaluate as evaluate_release_gate
from schema_integrity_audit import evaluate as evaluate_schema_integrity


class FinalQualificationManifestTests(unittest.TestCase):
    def test_final_manifest_declares_all_gates(self):
        manifest = json.loads((ROOT / "examples" / "qualification-manifest-prerelease.json").read_text(encoding="utf-8"))
        gates = [c["gate"] for c in manifest["claims"]]
        self.assertEqual(gates, [f"G{i}" for i in range(32)])
        self.assertTrue(manifest["strict_complete"])

    def test_strict_complete_accepts_g0_through_g31(self):
        claims = [
            {"gate": f"G{i}", "applicable": False, "rationale": "not applicable in unit test"}
            for i in range(32)
        ]
        out = evaluate_coverage([], {"strict_complete": True, "claims": claims})
        self.assertEqual(out["failures"], [])

    def test_missing_g31_fails_closed(self):
        claims = [
            {"gate": f"G{i}", "applicable": False, "rationale": "unit test"}
            for i in range(31)
        ]
        out = evaluate_coverage([], {"strict_complete": True, "claims": claims})
        self.assertTrue(any("G31" in x for x in out["failures"]))


class ExpertReleaseGateTests(unittest.TestCase):
    def test_required_evidence_can_select_kind_and_count(self):
        evidence = [
            {"id": "a", "kind": "history_check", "status": "PASS", "violations": []},
            {"id": "b", "kind": "history_check", "status": "PASS", "violations": []},
        ]
        out = evaluate_release_gate(evidence, {"required_evidence": [{"kind": "history_check", "min_count": 2}]})
        self.assertEqual(out["status"], "PASS")

    def test_selector_missing_fails(self):
        out = evaluate_release_gate([], {"required_evidence": [{"kind": "longrun_monitor", "min_count": 1}]})
        self.assertEqual(out["status"], "FAIL")

    def test_pass_with_violations_is_harness_contract_failure(self):
        out = evaluate_release_gate(
            [{"id": "x", "kind": "x", "status": "PASS", "violations": ["contradiction"]}],
            {"required_evidence": [{"id": "x"}]},
        )
        self.assertEqual(out["status"], "FAIL")
        self.assertTrue(any(x["type"] == "inconsistent_pass" for x in out["failures"]))


class SchemaIntegrityPolicyTests(unittest.TestCase):
    def test_schema_policy_is_claim_based(self):
        obs = {
            "invalid_indexes": 0,
            "unvalidated_constraints": 1,
            "orphaned_sequences": 0,
            "tables_without_primary_key": 2,
            "unindexed_foreign_keys": 1,
            "max_default_partition_rows": 10,
        }
        policy = {
            "require_no_invalid_indexes": True,
            "require_no_unvalidated_constraints": True,
            "max_tables_without_primary_key": 0,
            "max_unindexed_foreign_keys": 0,
            "max_default_partition_rows": 0,
        }
        failures = evaluate_schema_integrity(obs, policy)
        self.assertEqual(len(failures), 4)


class FinalReportTests(unittest.TestCase):
    def test_final_report_contains_expert_sections(self):
        evidence = [
            {"id": "schema.integrity", "kind": "schema_integrity", "status": "PASS", "observed": {}, "violations": [], "warnings": []},
            {"id": "isolation.x", "kind": "isolation_litmus", "status": "PASS", "observed": {}, "violations": [], "warnings": []},
            {"id": "retry.x", "kind": "retry_idempotency", "status": "PASS", "observed": {}, "violations": [], "warnings": []},
            {"id": "logical.x", "kind": "logical_replication", "status": "PASS", "observed": {}, "violations": [], "warnings": []},
            {"id": "compat.x", "kind": "compatibility_audit", "status": "PASS", "observed": {}, "violations": [], "warnings": []},
            {"id": "canary.database", "kind": "canary_evaluation", "status": "PASS", "observed": {"comparisons": []}, "violations": [], "warnings": []},
        ]
        md = render_markdown(evidence, {"status": "PASS", "failures": [], "warnings": []}, "Final", None)
        for heading in [
            "Schema integrity / relational design hazards",
            "Isolation-level litmus / anomaly evidence",
            "Retry / idempotency / ambiguous-result qualification",
            "Logical replication / CDC apply health",
            "Version / extension / collation compatibility",
            "Canary / post-deploy database guardrails",
        ]:
            self.assertIn(heading, md)


if __name__ == "__main__":
    unittest.main()
