import json
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))
PY = sys.executable

import business_scenario_runner
import business_test_plan
import business_verification_gate


class BusinessVerificationV3Tests(unittest.TestCase):
    def test_industrial_example_plan_and_binding_pass(self):
        plan = business_test_plan.load_plan(ROOT / "examples/business-test-plan.toml")
        binding = business_scenario_runner.load_toml(ROOT / "examples/business-scenario-bindings.toml")
        issues = business_test_plan.validate(plan)
        self.assertFalse([x for x in issues if x["severity"] == "error"], issues)
        self.assertFalse(business_scenario_runner.validate_binding(binding, plan))
        result = business_verification_gate.evaluate(plan, binding)
        self.assertEqual(result["status"], "PASS", result)

    def test_stateful_declared_without_state_model_fails(self):
        data = {
            "plan": {"id": "x", "title": "x", "risk": "medium", "objective": "x"},
            "applicability": {"stateful": True},
            "oracles": [{"id": "O1", "kind": "database", "observe": "state", "assertion": "state correct"}],
            "rules": [{"id": "R1", "risk": "medium", "statement": "rule", "oracle_ids": ["O1"]}],
            "journeys": [{"id": "J1", "title": "j", "trigger": "go", "expected": ["done"],
                          "forbidden": ["wrong"], "rules": ["R1"]}],
            "scenarios": [{"id": "S1", "journey": "J1", "class": "normal", "given": ["ready"],
                           "when": "go", "expected": ["done"], "forbidden": ["wrong"], "rules": ["R1"],
                           "oracle_ids": ["O1"], "evidence": "integration"}],
        }
        codes = {x["code"] for x in business_test_plan.validate(data)}
        self.assertIn("STATE-MODEL-MISSING", codes)
        self.assertIn("TRANSITIONS-MISSING", codes)

    def test_required_automated_scenario_without_binding_is_blocked(self):
        false_app = {k: False for k in business_test_plan.APPLICABILITY_KEYS}
        plan = {
            "plan": {"id": "x", "title": "x", "risk": "high", "objective": "x"},
            "applicability": false_app,
            "environment": {"isolation": "disposable schema"},
            "release": {"required_scenarios": ["S1", "S2"], "rollback": "flag"},
            "oracles": [{"id": "O1", "kind": "database", "observe": "state", "assertion": "state correct"}],
            "rules": [{"id": "R1", "risk": "high", "statement": "rule", "oracle_ids": ["O1"]}],
            "journeys": [{"id": "J1", "title": "j", "trigger": "go", "expected": ["done"],
                          "forbidden": ["wrong"], "rules": ["R1"]}],
            "scenarios": [
                {"id": "S1", "journey": "J1", "class": "normal", "given": ["ready"], "when": "go",
                 "expected": ["done"], "forbidden": ["wrong"], "rules": ["R1"], "oracle_ids": ["O1"],
                 "evidence": "integration", "automation": "automated"},
                {"id": "S2", "journey": "J1", "class": "invalid", "given": ["bad"], "when": "go",
                 "expected": ["rejected"], "forbidden": ["state change"], "rules": ["R1"], "oracle_ids": ["O1"],
                 "evidence": "integration", "automation": "automated"},
            ],
        }
        binding = {"execution": [{"id": "E1", "scenario_id": "S1", "steps": [{"kind": "command", "argv": ["/bin/true"]}]}]}
        result = business_verification_gate.evaluate(plan, binding)
        self.assertEqual(result["status"], "FAIL")
        self.assertIn("AUTOMATION-NO-BINDING", {x["code"] for x in result["issues"]})

    def test_scenario_runner_produces_real_evidence(self):
        binding = {"execution": [{
            "id": "E1", "scenario_id": "S1",
            "steps": [{"name": "proof", "kind": "command",
                       "argv": [PY, "-c", "print('business-ok')"],
                       "expect_stdout_contains": "business-ok"}],
        }]}
        rc, data = business_scenario_runner.execute(binding, "S1")
        self.assertEqual(rc, 0, data)
        self.assertEqual(data["status"], "PASS")
        self.assertIn("business-ok", data["steps"][0]["detail"]["stdout"])

    def test_sandbox_always_tears_down(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            marker = root / "teardown.txt"
            contract = root / "sandbox.toml"
            contract.write_text(textwrap.dedent(f'''
                [sandbox]
                isolation="temp dir"
                bootstrap=[["{PY}","-c","print('up')"]]
                teardown=[["{PY}","-c","from pathlib import Path; Path(r'{marker}').write_text('down')"]]
            '''), encoding="utf-8")
            p = subprocess.run(
                [PY, str(SCRIPTS / "sandbox_runner.py"), "--contract", str(contract), "--root", str(root),
                 "--", PY, "-c", "import sys; sys.exit(3)"],
                text=True, capture_output=True, timeout=30,
            )
            self.assertEqual(p.returncode, 1, p.stdout + p.stderr)
            self.assertTrue(marker.exists(), p.stdout + p.stderr)


if __name__ == "__main__":
    unittest.main()
