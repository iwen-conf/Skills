from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))

from sqlverify_common import RUN_ID_ENV, RUN_MANIFEST, begin_run, evidence_base, load_evidence_dir

GIT_ENV = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
GIT_ENV.update({"GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"})


def git(repo: Path, *args: str) -> None:
    subprocess.run(
        ["git", "-c", "user.name=t", "-c", "user.email=t@example.invalid", *args],
        cwd=repo,
        env=GIT_ENV,
        check=True,
        capture_output=True,
    )


def make_repo(base: Path) -> Path:
    repo = base / "app"
    repo.mkdir()
    git(repo, "init", "-q")
    (repo / "orders.sql").write_text("select 1;\n", encoding="utf-8")
    (repo / "sql-verification.toml").write_text("[project]\nname = 't'\n", encoding="utf-8")
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "-m", "init")
    return repo


def evidence(eid: str, kind: str, run_id: str | None, status: str = "PASS", **observed) -> dict:
    return {
        "schema_version": 1,
        "kind": kind,
        "id": eid,
        "run_id": run_id,
        "status": status,
        "started_at": "t",
        "observed": observed,
        "violations": [],
        "warnings": [],
        "reproduction": {},
    }


def write(path: Path, obj: dict) -> None:
    path.write_text(json.dumps(obj), encoding="utf-8")


def script(name: str, *args: str) -> tuple[int, dict]:
    cp = subprocess.run([sys.executable, str(SCRIPTS / name), *args], capture_output=True, text=True, env=GIT_ENV)
    out_flag = args.index("--out") + 1 if "--out" in args else None
    data = json.loads(Path(args[out_flag]).read_text(encoding="utf-8")) if out_flag else {}
    return cp.returncode, data


def failure_types(gate: dict) -> set[str]:
    return {f["type"] for f in gate["failures"]}


class RunBindingTests(unittest.TestCase):
    def setUp(self):
        self.td = tempfile.TemporaryDirectory()
        self.base = Path(self.td.name)
        self.repo = make_repo(self.base)
        self.cfg = self.repo / "sql-verification.toml"
        self.evdir = self.repo / ".sqlverify" / "evidence"
        self.policy = self.base / "policy.json"
        write(self.policy, {"required_evidence": [{"id": "txn.order-cancel"}]})

    def tearDown(self):
        self.td.cleanup()

    def begin(self) -> str:
        with mock.patch.dict(os.environ, GIT_ENV, clear=True):
            return begin_run(self.evdir, self.repo, [self.cfg], [self.evdir])["run_id"]

    def gate(self) -> tuple[int, dict]:
        return script(
            "release_gate.py",
            "--policy",
            str(self.policy),
            "--evidence-dir",
            str(self.evdir),
            "--root",
            str(self.repo),
            "--out",
            str(self.base / "gate.json"),
        )

    def test_evidence_base_stamps_current_run(self):
        with mock.patch.dict(os.environ, {RUN_ID_ENV: "run-1"}):
            self.assertEqual(evidence_base("k", "x")["run_id"], "run-1")

    def test_leftover_pass_from_earlier_run_fails_the_gate(self):
        self.evdir.mkdir(parents=True)
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", "old-run"))
        self.begin()
        # begin_run archives the leftover; a copy put back afterwards is still another run's evidence.
        archived = next((self.evdir / "superseded").glob("*/txn.order-cancel.json"))
        write(self.evdir / "txn.order-cancel.json", json.loads(archived.read_text(encoding="utf-8")))
        rc, gate = self.gate()
        self.assertEqual((rc, gate["status"]), (2, "FAIL"))
        self.assertTrue({"stale_evidence", "missing_evidence"} <= failure_types(gate))

    def test_evidence_without_run_manifest_fails(self):
        self.evdir.mkdir(parents=True)
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", None))
        rc, gate = self.gate()
        self.assertEqual(rc, 2)
        self.assertIn("missing_run_manifest", failure_types(gate))

    def test_current_run_evidence_passes(self):
        run_id = self.begin()
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        rc, gate = self.gate()
        self.assertEqual((rc, gate["status"], gate["run_id"]), (0, "PASS", run_id))

    def test_code_change_after_run_fails_as_candidate_mismatch(self):
        run_id = self.begin()
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        (self.repo / "orders.sql").write_text("select 2;\n", encoding="utf-8")
        rc, gate = self.gate()
        self.assertEqual(rc, 2)
        mismatch = [f for f in gate["failures"] if f["type"] == "candidate_mismatch"]
        self.assertEqual(mismatch[0]["changed"], ["worktree_changes"])
        self.assertEqual(list(mismatch[0]["paths"]), ["orders.sql"])

    def test_config_change_outside_repository_fails(self):
        outside = self.base / "outside.toml"
        outside.write_text("a = 1\n", encoding="utf-8")
        with mock.patch.dict(os.environ, GIT_ENV, clear=True):
            run_id = begin_run(self.evdir, self.repo, [outside], [self.evdir])["run_id"]
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        outside.write_text("a = 2\n", encoding="utf-8")
        rc, gate = self.gate()
        mismatch = [f for f in gate["failures"] if f["type"] == "candidate_mismatch"]
        self.assertEqual(mismatch[0]["changed"], ["config_sha256"])

    def test_unreadable_and_duplicate_evidence_fail_instead_of_being_skipped(self):
        run_id = self.begin()
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        write(self.evdir / "txn.order-cancel.copy.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        (self.evdir / "broken.json").write_text("{", encoding="utf-8")
        rc, gate = self.gate()
        self.assertEqual(rc, 2)
        self.assertTrue({"unreadable_evidence", "duplicate_evidence_id"} <= failure_types(gate))

    def test_unversioned_candidate_cannot_pass(self):
        plain = self.base / "plain"
        plain.mkdir()
        evdir = plain / "evidence"
        with mock.patch.dict(os.environ, GIT_ENV, clear=True):
            run_id = begin_run(evdir, plain, [], [evdir])["run_id"]
        write(evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        rc, gate = script(
            "release_gate.py",
            "--policy",
            str(self.policy),
            "--evidence-dir",
            str(evdir),
            "--root",
            str(plain),
            "--out",
            str(self.base / "gate.json"),
        )
        self.assertEqual(rc, 2)
        self.assertIn("candidate_unidentified", failure_types(gate))

    def test_begin_run_archives_every_leftover_including_raw_logs(self):
        (self.evdir / "raw" / "w1").mkdir(parents=True)
        (self.evdir / "raw" / "w1" / "pgbench_log.123").write_text("0 0 1000 0 1 1\n", encoding="utf-8")
        write(self.evdir / "bench.w1.c1.json", evidence("bench.w1.c1", "pgbench", "old"))
        self.begin()
        self.assertEqual(sorted(p.name for p in self.evdir.iterdir()), [RUN_MANIFEST, "superseded"])
        self.assertTrue(any((self.evdir / "superseded").glob("*/raw/w1/pgbench_log.123")))

    def test_loader_separates_other_run_evidence(self):
        self.evdir.mkdir(parents=True)
        write(self.evdir / "a.json", evidence("a", "k", "r1"))
        write(self.evdir / "b.json", evidence("b", "k", "r2"))
        loaded, issues = load_evidence_dir(self.evdir, run_id="r1")
        self.assertEqual([e["id"] for e in loaded], ["a"])
        self.assertEqual([(i["type"], i["id"]) for i in issues], [("stale_evidence", "b")])

    def test_policy_that_checks_nothing_fails_closed(self):
        # A run-bound FAIL record is present: only a gate that evaluated nothing could call this PASS.
        run_id = self.begin()
        write(
            self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id, "FAIL")
        )
        cases = [
            ({}, "empty_policy"),
            ({"required_evidence": [], "metric_rule": []}, "empty_policy"),
            ({"required_evidnce": [{"id": "txn.order-cancel"}]}, "invalid_policy"),
            ({"required_evidence": [{"id": "txn.order-cancel", "min_cont": 2}]}, "invalid_policy"),
            ({"required_evidence": {"id": "txn.order-cancel"}}, "invalid_policy"),
        ]
        for policy, expected in cases:
            with self.subTest(policy=policy):
                write(self.policy, policy)
                (self.base / "gate.json").unlink(missing_ok=True)
                rc, gate = self.gate()
                self.assertEqual((rc, gate["status"]), (2, "FAIL"))
                self.assertIn(expected, failure_types(gate))

    def test_coverage_manifest_without_claims_fails_closed(self):
        run_id = self.begin()
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", run_id))
        manifest = self.base / "manifest.json"
        for body in (
            {},
            {"claims": []},
            {"claim": [{"gate": "G9", "applicable": True, "require": [{"id": "txn.order-cancel"}]}]},
        ):
            with self.subTest(manifest=body):
                write(manifest, body)
                (self.base / "coverage.json").unlink(missing_ok=True)
                rc, out = script(
                    "qualification_coverage_gate.py",
                    "--manifest",
                    str(manifest),
                    "--evidence-dir",
                    str(self.evdir),
                    "--out",
                    str(self.base / "coverage.json"),
                )
                self.assertEqual((rc, out["status"]), (2, "FAIL"))
                self.assertTrue(any("declares no claims" in v for v in out["violations"]))

    def test_coverage_gate_rejects_other_run_evidence(self):
        run_id = self.begin()
        write(self.evdir / "txn.order-cancel.json", evidence("txn.order-cancel", "transaction_scenario", "old-run"))
        manifest = self.base / "manifest.json"
        write(manifest, {"claims": [{"gate": "G9", "applicable": True, "require": [{"id": "txn.order-cancel"}]}]})
        rc, out = script(
            "qualification_coverage_gate.py",
            "--manifest",
            str(manifest),
            "--evidence-dir",
            str(self.evdir),
            "--out",
            str(self.base / "coverage.json"),
        )
        self.assertEqual((rc, out["status"]), (2, "FAIL"))
        self.assertTrue(any("stale_evidence" in v for v in out["violations"]))
        self.assertNotEqual(run_id, "old-run")


class RegressionCompareTests(unittest.TestCase):
    def setUp(self):
        self.td = tempfile.TemporaryDirectory()
        self.base = Path(self.td.name)
        self.repo = make_repo(self.base)

    def tearDown(self):
        self.td.cleanup()

    def run_dir(self, name: str, p95_runs: list[float]) -> tuple[Path, str]:
        d = self.repo / name
        with mock.patch.dict(os.environ, GIT_ENV, clear=True):
            run_id = begin_run(d, self.repo, [], [d])["run_id"]
        write(
            d / "postgres.snapshot.pre.json",
            evidence(
                "postgres.snapshot.pre",
                "postgres_snapshot",
                run_id,
                server_version_num=180001,
                settings_sha256="s",
                schema_fingerprint_sha256="h",
            ),
        )
        p95 = sorted(p95_runs)[len(p95_runs) // 2]
        write(
            d / "bench.w1.c1.json",
            evidence(
                "bench.w1.c1",
                "pgbench",
                run_id,
                tps=100,
                latency_ms={"p95": p95, "p99": p95 * 1.5, "p95_runs": p95_runs},
            ),
        )
        return d, run_id

    def promote(self, d: Path, run_id: str) -> Path:
        gate = self.base / f"gate-{d.name}.json"
        write(gate, {"status": "PASS", "run_id": run_id, "failures": []})
        dest = self.base / f"baseline-{d.name}"
        cp = subprocess.run(
            [
                sys.executable,
                str(SCRIPTS / "baseline_promote.py"),
                "--evidence-dir",
                str(d),
                "--gate",
                str(gate),
                "--destination",
                str(dest),
                "--root",
                str(self.repo),
            ],
            capture_output=True,
            text=True,
            env=GIT_ENV,
        )
        self.assertEqual(cp.returncode, 0, cp.stdout + cp.stderr)
        return dest

    def compare(self, current: Path, baseline: Path) -> tuple[int, dict]:
        return script(
            "regression_compare.py",
            "--current-dir",
            str(current),
            "--baseline-dir",
            str(baseline),
            "--out",
            str(self.base / "regression.json"),
        )

    def test_empty_baseline_is_blocked_not_pass(self):
        current, _ = self.run_dir("cur", [10, 10, 10])
        empty = self.base / "empty"
        empty.mkdir()
        rc, out = self.compare(current, empty)
        self.assertEqual((rc, out["status"]), (2, "BLOCKED"))
        self.assertEqual(out["observed"]["effective_comparisons"], 0)

    def test_baseline_without_manifest_is_blocked(self):
        old, _ = self.run_dir("old", [10, 10, 10])
        current, _ = self.run_dir("cur", [10, 10, 10])
        rc, out = self.compare(current, old)
        self.assertEqual((rc, out["status"]), (2, "BLOCKED"))
        self.assertTrue(any("baseline-manifest.json is absent" in v for v in out["violations"]))

    def test_comparable_runs_pass_with_an_effective_comparison(self):
        baseline = self.promote(*self.run_dir("old", [10, 10, 11]))
        current, _ = self.run_dir("cur", [10, 11, 10])
        rc, out = self.compare(current, baseline)
        self.assertEqual((rc, out["status"]), (0, "PASS"), out["violations"])
        self.assertEqual(out["observed"]["effective_comparisons"], 1)

    def test_measured_regression_fails(self):
        baseline = self.promote(*self.run_dir("old", [10, 10, 10]))
        current, _ = self.run_dir("cur", [20, 21, 20])
        rc, out = self.compare(current, baseline)
        self.assertEqual((rc, out["status"]), (2, "FAIL"))

    def test_workload_missing_from_current_run_is_blocked(self):
        baseline = self.promote(*self.run_dir("old", [10, 10, 10]))
        current, _ = self.run_dir("cur", [10, 10, 10])
        (current / "bench.w1.c1.json").unlink()
        rc, out = self.compare(current, baseline)
        self.assertEqual((rc, out["status"]), (2, "BLOCKED"))
        self.assertTrue(any("no current counterpart" in v for v in out["violations"]))

    def test_promotion_refuses_a_gate_from_another_run(self):
        d, _ = self.run_dir("old", [10, 10, 10])
        gate = self.base / "gate.json"
        write(gate, {"status": "PASS", "run_id": "another-run"})
        cp = subprocess.run(
            [
                sys.executable,
                str(SCRIPTS / "baseline_promote.py"),
                "--evidence-dir",
                str(d),
                "--gate",
                str(gate),
                "--destination",
                str(self.base / "b"),
                "--root",
                str(self.repo),
            ],
            capture_output=True,
            text=True,
            env=GIT_ENV,
        )
        self.assertNotEqual(cp.returncode, 0)
        self.assertIn("does not match evidence run", cp.stderr)


class CanaryTests(unittest.TestCase):
    def test_policy_without_metrics_is_blocked(self):
        with tempfile.TemporaryDirectory() as td:
            td = Path(td)
            write(td / "metrics.json", {"control": {}, "canary": {}, "bake_seconds": 600})
            write(td / "policy.json", {"metric": []})
            rc, out = script(
                "canary_evaluator.py",
                "--metrics",
                str(td / "metrics.json"),
                "--policy",
                str(td / "policy.json"),
                "--out",
                str(td / "canary.json"),
            )
            self.assertEqual((rc, out["status"]), (2, "BLOCKED"))


if __name__ == "__main__":
    unittest.main()
