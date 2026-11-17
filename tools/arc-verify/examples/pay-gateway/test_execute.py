"""CLI orchestration contracts; no product request is sent by these tests."""
import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import execute


class PayExecutionContracts(unittest.TestCase):
    def test_dirty_checkout_is_rejected_before_any_arc_command(self):
        with patch.object(execute.adapter, "read_json", return_value={}), \
             patch.object(execute, "verify_source_binding", side_effect=execute.adapter.Blocked("tracked source changed")), \
             patch.object(execute.subprocess, "run") as cli, \
             patch("sys.argv", ["execute.py", "--session", "/candidate/session.json", "--bundle", "/candidate/bundle"]), \
             contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(execute.main(), 2)
        cli.assert_not_called()
        self.assertEqual(json.loads(output.getvalue())["verdict"], "BLOCKED")

    def test_missing_exports_and_late_source_change_cannot_report_complete_delivery(self):
        for defect in ("none", "explain", "snapshot", "source_binding", "incomplete_evidence"):
            with self.subTest(defect=defect), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                bundle = root / "bundle"
                bundle.mkdir()
                session = {"scope": "owned-case", "pay_repo": str(root), "caller_id": "caller-one"}
                documents = {
                    "session.json": session,
                    "caller-private.json": {"value": "private-placeholder-for-tool-test"},
                    "runner.json": {"graph": execute.bundle.GRAPH, "isolation_scope": "owned-case", "test": "TEST_REFUND_DUPLICATE_CHAIN", "id": "runner"},
                    "fixtures.json": {"fixtures": [{"id": "caller", "refs": {"principal": "caller-one"}}, {"id": "case"}]},
                }
                def cli(argv, **_kwargs):
                    value = {"active": True, "id": "round", "run": "observed-run", "verdict": "PASS",
                             "graph": execute.bundle.GRAPH, "offline_db_complete": defect != "incomplete_evidence",
                             "status": "PROVEN_IN_SCOPE", "complete_in_scope": True, "contract_sha256": "a" * 64}
                    code = 2 if defect in {"explain", "snapshot"} and defect in argv else 0
                    return execute.subprocess.CompletedProcess(argv, code, json.dumps(value), "")
                source_results = [None, execute.adapter.Blocked("source changed")] if defect == "source_binding" else [None, None]
                with patch.object(execute.adapter, "read_json", side_effect=lambda path: documents[Path(path).name]), \
                     patch.object(execute, "verify_source_binding", side_effect=source_results), \
                     patch.object(execute.subprocess, "run", side_effect=cli), \
                     patch("sys.argv", ["execute.py", "--session", str(root / "session.json"), "--bundle", str(bundle), "--assets", str(root / "assets")]), \
                     contextlib.redirect_stdout(io.StringIO()) as output:
                    self.assertEqual(execute.main(), 0 if defect == "none" else 2)
                result = json.loads(output.getvalue())
                self.assertEqual(result["run"], "observed-run")
                self.assertEqual(result["verdict"], "PASS")
                self.assertEqual(result["delivery_status"], "COMPLETE" if defect == "none" else "BLOCKED")
                if defect != "none":
                    self.assertIn("BLOCKED", result["exports"].values())

    def test_old_same_id_or_changed_contract_cannot_reach_any_target_execution(self):
        for defect in ("old_encoding", "changed_source", "changed_version"):
            with self.subTest(defect=defect), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                output_bundle = root / "bundle"
                output_bundle.mkdir()
                documents = {
                    "session.json": {"scope": "owned-case", "pay_repo": str(root), "caller_id": "caller-one"},
                    "caller-private.json": {"value": "private-placeholder-for-tool-test"},
                    "runner.json": {"graph": execute.bundle.GRAPH, "isolation_scope": "owned-case", "test": "TEST_REFUND_DUPLICATE_CHAIN", "id": "runner"},
                    "fixtures.json": {"fixtures": [{"id": "caller", "refs": {"principal": "caller-one"}}, {"id": "case"}]},
                }
                calls = []
                def cli(argv, **_kwargs):
                    calls.append(argv)
                    value = {"active": True, "id": execute.bundle.GRAPH, "status": "PROVEN_IN_SCOPE",
                             "complete_in_scope": True, "contract_sha256": "a" * 64}
                    if "contract" in argv and "--graph" in argv:
                        value["contract_sha256"] = "b" * 64
                        if defect == "changed_version":
                            value.update(status="MODEL_ERROR", complete_in_scope=False)
                    return execute.subprocess.CompletedProcess(argv, 0, json.dumps(value), "")
                with patch.object(execute.adapter, "read_json", side_effect=lambda path: documents[Path(path).name]), \
                     patch.object(execute, "verify_source_binding"), \
                     patch.object(execute.subprocess, "run", side_effect=cli), \
                     patch("sys.argv", ["execute.py", "--session", str(root / "session.json"), "--bundle", str(output_bundle)]), \
                     contextlib.redirect_stdout(io.StringIO()) as output:
                    self.assertEqual(execute.main(), 2)
                result = json.loads(output.getvalue())
                self.assertEqual(result["verdict"], "BLOCKED")
                self.assertIn("new model version", result["reason"])
                self.assertTrue(any("contract" in call and "--file" in call for call in calls))
                self.assertTrue(any("contract" in call and "--graph" in call for call in calls))
                self.assertFalse(any("execute" in call or "fixtures" in call or "runner" in call or "round" in call for call in calls))

    def test_contract_proof_comparison_requires_real_complete_hashes(self):
        valid = {"status": "PROVEN_IN_SCOPE", "complete_in_scope": True, "contract_sha256": "c" * 64}
        execute.verify_contract_proofs(valid, dict(valid))
        for invalid in ({}, None, {**valid, "contract_sha256": None}, {**valid, "complete_in_scope": False},
                        {**valid, "status": "BUDGET_EXHAUSTED"}, {**valid, "contract_sha256": "not-a-hash"}):
            with self.subTest(invalid=invalid), self.assertRaises(execute.adapter.Blocked):
                execute.verify_contract_proofs(valid, invalid)


if __name__ == "__main__":
    unittest.main()
