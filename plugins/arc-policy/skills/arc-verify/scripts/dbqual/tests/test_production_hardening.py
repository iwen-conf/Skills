from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))

from sqlverify_common import RUN_ID_ENV, begin_run, evidence_base, json_dump, run
from run_suite import call

SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"


class ProductionHardeningTests(unittest.TestCase):
    def test_all_output_ownership_is_checked_before_any_archive(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir, reportdir = root / "evidence", root / "reports"
            begin_run(evdir, root, [], [evdir, reportdir])
            sentinel = evdir / "kept-evidence.json"
            sentinel.write_text("previous evidence", encoding="utf-8")
            reportdir.mkdir()
            (reportdir / "unowned.txt").write_text("belongs to another tool", encoding="utf-8")
            with self.assertRaises(ValueError):
                begin_run(evdir, root, [], [evdir, reportdir], report_dir=reportdir)
            self.assertTrue(sentinel.is_file(), "report ownership failure already moved valid evidence")
            self.assertFalse((evdir / "superseded").exists(), "failed preflight performed an archive")

    def test_child_start_error_is_run_bound_blocked_evidence(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir = root / "evidence"
            run = begin_run(evdir, root, [], [evdir])
            with patch.dict(os.environ, {RUN_ID_ENV: run["run_id"]}):
                try:
                    rc = call(
                        [str(root / "missing-executable")],
                        keep_going=True,
                        deadline=time.monotonic() + 1,
                        evidence_dir=evdir,
                    )
                except OSError:
                    rc = None
            records = [json.loads(p.read_text(encoding="utf-8")) for p in evdir.glob("runner.*.json")]
            self.assertEqual(rc, 2, "child start failure escaped without a runner verdict")
            self.assertEqual(len(records), 1)
            self.assertEqual(records[0]["status"], "BLOCKED")
            self.assertEqual(records[0]["run_id"], run["run_id"])
            self.assertFalse(records[0]["observed"]["launched"])

    @unittest.skipUnless(os.name == "posix", "requires POSIX process-group observation")
    def test_successful_parent_cannot_leave_an_unbounded_descendant(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir = root / "evidence"
            run = begin_run(evdir, root, [], [evdir])
            pidfile = root / "leftover.pid"
            child = "import threading;threading.Event().wait()"
            parent = (
                "import subprocess,sys,pathlib;"
                "p=subprocess.Popen([sys.executable,'-c'," + repr(child) + "]);"
                "pathlib.Path(" + repr(str(pidfile)) + ").write_text(str(p.pid))"
            )
            try:
                with patch.dict(os.environ, {RUN_ID_ENV: run["run_id"]}):
                    rc = call(
                        [sys.executable, "-B", "-c", parent],
                        keep_going=True,
                        deadline=time.monotonic() + 2,
                        evidence_dir=evdir,
                    )
                self.assertEqual(rc, 2, "successful parent hid an unbounded background operation")
                records = [json.loads(p.read_text(encoding="utf-8")) for p in evdir.glob("runner.*.json")]
                self.assertEqual(records[0]["status"], "BLOCKED")
                self.assertTrue(records[0]["observed"]["residual_processes"])
            finally:
                if pidfile.is_file():
                    try:
                        os.kill(int(pidfile.read_text(encoding="utf-8")), signal.SIGKILL)
                    except ProcessLookupError:
                        pass

    @unittest.skipUnless(os.name == "posix", "requires POSIX process-group observation")
    def test_timeout_terminates_descendants_and_preserves_partial_evidence(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir = root / "evidence"
            run = begin_run(evdir, root, [], [evdir])
            sentinel = evdir / "partial.json"
            sentinel.write_text("partial evidence", encoding="utf-8")
            pidfile = root / "child.pid"
            child = "import threading;threading.Event().wait()"
            parent = (
                "import subprocess,sys,pathlib,threading;"
                "p=subprocess.Popen([sys.executable,'-c'," + repr(child) + "]);"
                "pathlib.Path(" + repr(str(pidfile)) + ").write_text(str(p.pid));"
                "threading.Event().wait()"
            )
            # Two seconds is this probe's execution budget, not a target SLO.
            with patch.dict(os.environ, {RUN_ID_ENV: run["run_id"]}), self.assertRaises(SystemExit) as ended:
                call(
                    [sys.executable, "-B", "-c", parent],
                    keep_going=True,
                    deadline=time.monotonic() + 2,
                    evidence_dir=evdir,
                )
            self.assertEqual(ended.exception.code, 124)
            self.assertTrue(pidfile.is_file(), "probe never started the descendant")
            pid = int(pidfile.read_text(encoding="utf-8"))
            state = subprocess.run(
                ["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True, check=False, timeout=1
            ).stdout.strip()
            self.assertTrue(not state or state.startswith("Z"), f"descendant survived timeout: {state}")
            self.assertEqual(sentinel.read_text(encoding="utf-8"), "partial evidence")
            records = [json.loads(p.read_text(encoding="utf-8")) for p in evdir.glob("runner.*.json")]
            self.assertEqual(len(records), 1)
            self.assertEqual(records[0]["status"], "BLOCKED")
            self.assertEqual(records[0]["run_id"], run["run_id"])
            self.assertTrue(records[0]["observed"]["timeout"])

    @unittest.skipUnless(os.name == "posix", "requires POSIX process-tree observation")
    def test_suite_timeout_stops_nested_process_sessions(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir = root / "evidence"
            run_context = begin_run(evdir, root, [], [evdir])
            pidfile = root / "nested.pid"
            child = "import threading;threading.Event().wait()"
            parent = (
                "import subprocess,sys,pathlib,threading;"
                f"p=subprocess.Popen([sys.executable,'-c',{child!r}],start_new_session=True);"
                f"pathlib.Path({str(pidfile)!r}).write_text(str(p.pid));threading.Event().wait()"
            )
            try:
                with (
                    patch.dict(os.environ, {RUN_ID_ENV: run_context["run_id"]}),
                    self.assertRaises(SystemExit) as ended,
                ):
                    call(
                        [sys.executable, "-B", "-c", parent],
                        keep_going=True,
                        deadline=time.monotonic() + 1,
                        evidence_dir=evdir,
                    )
                self.assertEqual(ended.exception.code, 124)
                pid = int(pidfile.read_text(encoding="utf-8"))
                state = subprocess.run(
                    ["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True, check=False, timeout=1
                ).stdout.strip()
                self.assertTrue(not state or state.startswith("Z"), f"nested runner survived suite timeout: {state}")
            finally:
                if pidfile.is_file():
                    try:
                        os.killpg(int(pidfile.read_text(encoding="utf-8")), signal.SIGKILL)
                    except ProcessLookupError:
                        pass

    @unittest.skipUnless(os.name == "posix", "requires POSIX process-group observation")
    def test_command_timeout_stops_its_descendants(self):
        with tempfile.TemporaryDirectory() as td:
            pidfile = Path(td) / "command-child.pid"
            child = "import threading;threading.Event().wait()"
            parent = (
                "import subprocess,sys,pathlib,threading;"
                f"p=subprocess.Popen([sys.executable,'-c',{child!r}]);"
                f"pathlib.Path({str(pidfile)!r}).write_text(str(p.pid));threading.Event().wait()"
            )
            try:
                with self.assertRaises((subprocess.TimeoutExpired, RuntimeError)):
                    run([sys.executable, "-B", "-c", parent], timeout=1)
                pid = int(pidfile.read_text(encoding="utf-8"))
                state = subprocess.run(
                    ["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True, check=False, timeout=1
                ).stdout.strip()
                self.assertTrue(not state or state.startswith("Z"), f"command descendant survived timeout: {state}")
            finally:
                if pidfile.is_file():
                    try:
                        os.kill(int(pidfile.read_text(encoding="utf-8")), signal.SIGKILL)
                    except ProcessLookupError:
                        pass

    def test_complete_nonapplicable_manifest_cannot_claim_execution_pass(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir = root / "evidence"
            begin_run(evdir, root, [], [evdir])
            manifest = root / "applicability.json"
            json_dump(
                manifest,
                {
                    "strict_complete": True,
                    "claims": [
                        {"gate": f"G{i}", "applicable": False, "rationale": "outside the test contract"}
                        for i in range(32)
                    ],
                },
            )
            out = evdir / "qualification.coverage.json"
            cp = subprocess.run(
                [
                    sys.executable,
                    "-B",
                    str(SCRIPTS / "qualification_coverage_gate.py"),
                    "--manifest",
                    str(manifest),
                    "--evidence-dir",
                    str(evdir),
                    "--out",
                    str(out),
                ],
                capture_output=True,
                text=True,
                check=False,
                timeout=2,
            )
            record = json.loads(out.read_text(encoding="utf-8"))
            self.assertNotEqual(cp.returncode, 0, "all n/a gates qualified an execution")
            self.assertEqual(record["status"], "NOT_RUN")

    def test_evidence_directory_must_not_archive_candidate(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "src").mkdir()
            (root / "src/service.py").write_text("sentinel source\n", encoding="utf-8")
            (root / ".config.json").write_text("{}", encoding="utf-8")
            try:
                begin_run(root, root, [root / ".config.json"], [root])
            except (ValueError, RuntimeError):
                pass
            self.assertTrue((root / "src/service.py").is_file(), "evidence archive moved candidate source")
            self.assertTrue((root / ".config.json").is_file(), "evidence archive moved the input configuration")

    def test_report_rejects_gate_from_previous_run(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            evdir = root / "evidence"
            evdir.mkdir()
            current = evidence_base("toolchain", "toolchain.doctor")
            current.update(run_id="current-run", status="PASS")
            json_dump(evdir / "doctor.json", current)
            json_dump(evdir / "run-manifest.json", {"run_id": "current-run"})
            old = {
                "status": "PASS",
                "run_id": "previous-run",
                "candidate": {"git_commit": "old-commit"},
                "failures": [],
                "input_binding": {"evidence": {"absent.json": "old-hash"}},
            }
            json_dump(root / "old-gate.json", old)
            report = root / "report.md"
            cp = subprocess.run(
                [
                    sys.executable,
                    "-B",
                    str(SCRIPTS / "evidence_report.py"),
                    "--evidence-dir",
                    str(evdir),
                    "--gate",
                    str(root / "old-gate.json"),
                    "--out-md",
                    str(report),
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(cp.returncode, 0, "stale release gate was accepted")
            if report.exists():
                self.assertNotIn("**Release gate:** `PASS`", report.read_text(encoding="utf-8"))

    def test_runner_refuses_unbounded_child(self):
        # The observer's one-second budget detects an unbounded call; it is not a production SLO.
        code = (
            "import sys;sys.path.insert(0," + repr(str(SCRIPTS)) + ");"
            "from run_suite import call;"
            'call([sys.executable,"-c","import threading;threading.Event().wait()"],keep_going=True)'
        )
        proc = subprocess.Popen(
            [sys.executable, "-B", "-c", code],
            start_new_session=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        timed_out = False
        try:
            proc.communicate(timeout=1.0)
        except subprocess.TimeoutExpired:
            timed_out = True
        finally:
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.communicate()
        self.assertFalse(timed_out, "runner launched a permanently hung child without an execution budget")


if __name__ == "__main__":
    unittest.main()
