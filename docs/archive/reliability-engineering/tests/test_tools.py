import argparse
import io
import json
import subprocess
import sys
import tempfile
import textwrap
import unittest
from contextlib import redirect_stdout
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))
PY = sys.executable

import business_test_plan
import business_txn_lab
import verification_toolchain


class ReliabilityGateTests(unittest.TestCase):
    def test_example_manifest_passes_cli(self):
        p = subprocess.run(
            [PY, str(SCRIPTS / "reliability_gate.py"), str(ROOT / "reliability.example.toml"), "--today", "2026-09-18"],
            text=True, capture_output=True, timeout=30,
        )
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn("STATUS: PASS", p.stdout)

    def test_missing_oracle_fails_cli(self):
        manifest = textwrap.dedent("""
            [policy]
            critical_min_independent_evidence = 0
            critical_require_runtime_detection = false
            critical_require_release_gate = false

            [[invariants]]
            id = "INV-X"
            title = "broken"
            risk = "critical"
            statement = "must hold"
        """)
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "reliability.toml"
            path.write_text(manifest, encoding="utf-8")
            p = subprocess.run(
                [PY, str(SCRIPTS / "reliability_gate.py"), str(path), "--today", "2026-09-18"],
                text=True, capture_output=True, timeout=30,
            )
            self.assertEqual(p.returncode, 1, p.stdout + p.stderr)
            self.assertIn("missing oracle", p.stdout)


class SeedRunnerTests(unittest.TestCase):
    def test_seed_is_exported(self):
        p = subprocess.run(
            [PY, str(SCRIPTS / "seed_runner.py"), "--runs", "2", "--timeout", "30", "--",
             "/bin/sh", "-c", 'test "$TEST_SEED" = 1 -o "$TEST_SEED" = 2'],
            text=True, capture_output=True, timeout=90,
        )
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn("PASS: 2 seeded runs", p.stdout)


class VerificationToolchainTests(unittest.TestCase):
    def test_plan_rust_coverage_and_mutation_resolution(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / "Cargo.toml").write_text('[package]\nname="x"\nversion="0.1.0"\nedition="2024"\n', encoding="utf-8")
            selected = verification_toolchain.resolve_tools(root, ["coverage", "mutation"], [], [])
            selected = verification_toolchain.topo_expand(selected)
            self.assertIn("cargo-llvm-cov", selected)
            self.assertIn("cargo-mutants", selected)

    def test_project_dependency_is_marked_as_project_mutation(self):
        self.assertTrue(verification_toolchain.TOOLS["proptest"].project_mutation)
        cmd, _ = verification_toolchain.install_command(verification_toolchain.TOOLS["proptest"], Path("."))
        self.assertEqual(cmd[:3], ["cargo", "add", "--dev"])

    def test_network_fault_plan_has_pinned_toxiproxy(self):
        tool = verification_toolchain.TOOLS["toxiproxy"]
        self.assertEqual(tool.install_args, ("ghcr.io/shopify/toxiproxy:2.12.0",))
        self.assertIn("container-engine", tool.requires)

    def test_go_foundation_plan_uses_native_and_user_tools(self):
        suite = verification_toolchain.SUITES["go-foundation"]
        self.assertIn("go-race", suite)
        self.assertIn("gotestsum", suite)
        self.assertIn("govulncheck", suite)
        self.assertIn("staticcheck", suite)

    def test_foundation_suites_exist(self):
        self.assertIn("rust-foundation", verification_toolchain.SUITES)
        self.assertIn("go-foundation", verification_toolchain.SUITES)
        self.assertIn("cargo-audit", verification_toolchain.SUITES["rust-foundation"])

    def test_tool_catalog(self):
        ids = set(verification_toolchain.TOOLS)
        self.assertIn("cargo-fuzz", ids)
        self.assertIn("go-race", ids)
        self.assertIn("toxiproxy", ids)

    def test_plan_mode_uses_shallow_probe_for_container_engine(self):
        present, info = verification_toolchain.shallow_tool_present(
            verification_toolchain.TOOLS["container-engine"], Path(".")
        )
        self.assertIsInstance(present, bool)
        self.assertNotIn("Server.Version", info)


class BusinessTestPlanTests(unittest.TestCase):
    def test_example_business_plan_passes(self):
        data = business_test_plan.load_plan(ROOT / "examples/business-test-plan.toml")
        issues = business_test_plan.validate(data)
        self.assertFalse([x for x in issues if x["severity"] == "error"], issues)

    def test_missing_business_oracle_fails(self):
        data = {
            "plan": {"id": "x", "title": "x", "risk": "high", "objective": "x"},
            "applicability": {k: False for k in business_test_plan.APPLICABILITY_KEYS},
            "environment": {"isolation": "temp"},
            "release": {"required_scenarios": ["S1", "S2"], "rollback": "flag"},
            "rules": [{"id": "R1", "risk": "high", "statement": "rule"}],
            "journeys": [{"id": "J1", "title": "j", "trigger": "go", "expected": ["done"],
                          "forbidden": ["wrong"], "rules": ["R1"]}],
            "scenarios": [
                {"id": "S1", "journey": "J1", "class": "normal", "given": ["ready"], "when": "go",
                 "expected": ["done"], "forbidden": ["wrong"], "rules": ["R1"], "evidence": "integration",
                 "automation": "automated"},
                {"id": "S2", "journey": "J1", "class": "invalid", "given": ["bad"], "when": "go",
                 "expected": ["rejected"], "forbidden": ["change"], "rules": ["R1"], "evidence": "integration",
                 "automation": "automated"},
            ],
        }
        codes = {x["code"] for x in business_test_plan.validate(data)}
        self.assertIn("RULE-NO-ORACLE", codes)

    def test_render_contains_business_scenario_matrix(self):
        data = business_test_plan.load_plan(ROOT / "examples/business-test-plan.toml")
        md = business_test_plan.render_markdown(data)
        self.assertIn("Scenario matrix", md)
        self.assertIn("Forbidden", md)


class BusinessTransactionLabTests(unittest.TestCase):
    def test_catalog_detects_opposite_lock_order(self):
        data = {"operation": [
            {"id": "a", "critical": True, "isolation": "read committed", "max_duration_ms": 100,
             "retry_sqlstates": ["40P01"], "resources": [{"name": "order", "mode": "write"}, {"name": "account", "mode": "write"}]},
            {"id": "b", "isolation": "read committed", "max_duration_ms": 100,
             "retry_sqlstates": ["40P01"], "resources": [{"name": "account", "mode": "write"}, {"name": "order", "mode": "write"}]},
        ]}
        report = business_txn_lab.analyze_catalog(data)
        self.assertIn("TXN-LOCK-ORDER-INVERSION", {x["code"] for x in report["findings"]})

    def test_plain_reads_do_not_create_lock_order_inversion(self):
        data = {"operation": [
            {"id": "a", "isolation": "read committed", "max_duration_ms": 100,
             "resources": [{"name": "order", "mode": "read"}, {"name": "account", "mode": "read"}]},
            {"id": "b", "isolation": "read committed", "max_duration_ms": 100,
             "resources": [{"name": "account", "mode": "read"}, {"name": "order", "mode": "read"}]},
        ]}
        report = business_txn_lab.analyze_catalog(data)
        self.assertNotIn("TXN-LOCK-ORDER-INVERSION", {x["code"] for x in report["findings"]})

    def test_dynamic_multi_key_lock_requires_canonical_order(self):
        data = {"operation": [{
            "id": "transfer", "critical": True, "isolation": "read committed", "max_duration_ms": 100,
            "retry_sqlstates": ["40P01"],
            "resources": [{"name": "account", "mode": "write", "multiplicity": "many", "key_order": "request"}],
        }]}
        report = business_txn_lab.analyze_catalog(data)
        self.assertIn("TXN-DYNAMIC-KEY-ORDER", {x["code"] for x in report["findings"]})

    def test_scenario_validation_rejects_missing_signal(self):
        errors = business_txn_lab.validate_scenario({"session": [{"name": "a", "steps": [{"sql": "select 1", "wait_for": ["never"]}]}]})
        self.assertTrue(any("signal never emitted" in x for x in errors), errors)

    def test_postgres_transaction_capability_resolves_psycopg(self):
        selected = verification_toolchain.resolve_tools(Path("."), ["business-transaction-postgres"], [], [])
        self.assertIn("psycopg3-test-adapter", selected)
        self.assertIn("psycopg[binary]==3.3.5", verification_toolchain.TOOLS["psycopg3-test-adapter"].install_args)


if __name__ == "__main__":
    unittest.main()
