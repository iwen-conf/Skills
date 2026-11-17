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
                             "graph": execute.bundle.GRAPH, "offline_db_complete": defect != "incomplete_evidence"}
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


if __name__ == "__main__":
    unittest.main()
