#!/usr/bin/env python3
from __future__ import annotations

import compileall
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))

from sqlverify_common import begin_run

ROOT = Path(__file__).resolve().parents[1]


def run_cmd(args: list[str]) -> tuple[bool, str]:
    cp = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return cp.returncode == 0, cp.stdout


def main() -> int:
    result: dict[str, Any] = {
        "compile": False,
        "unit_tests": False,
        "config": False,
        "parameter_corpus": False,
        "report_generation": False,
        "baseline_promotion_and_compare": False,
        "fail_closed_controls": False,
        "errors": [],
    }
    result["compile"] = bool(
        compileall.compile_dir(str(ROOT / "scripts"), quiet=1) and compileall.compile_dir(str(ROOT / "tests"), quiet=1)
    )

    ok, out = run_cmd([sys.executable, "-m", "unittest", "discover", "-s", str(ROOT / "tests"), "-v"])
    result["unit_tests"] = ok
    if not ok:
        result["errors"].append(out)

    ok, out = run_cmd(
        [
            sys.executable,
            str(ROOT / "scripts" / "sqlverify_config.py"),
            str(ROOT / "examples" / "sql-verification.toml"),
            "--root",
            str(ROOT),
            "--json",
        ]
    )
    result["config"] = ok
    if not ok:
        result["errors"].append(out)

    with tempfile.TemporaryDirectory() as tmp:
        td = Path(tmp)
        corpus = td / "corpus.json"
        ok, out = run_cmd(
            [
                sys.executable,
                str(ROOT / "scripts" / "parameter_corpus.py"),
                str(ROOT / "examples" / "parameter-corpus-spec.json"),
                "--out",
                str(corpus),
            ]
        )
        result["parameter_corpus"] = ok and corpus.exists()
        if not result["parameter_corpus"]:
            result["errors"].append(out)

        evdir = td / "evidence"
        evdir.mkdir()
        reportdir = td / "report"
        reportdir.mkdir()
        candidate = td / "candidate"
        candidate.mkdir()
        git_env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        git_env.update({"GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"})
        (candidate / "query.sql").write_text("select 1;\n", encoding="utf-8")
        for args in (
            ["init", "-q"],
            ["add", "query.sql"],
            ["-c", "user.name=selftest", "-c", "user.email=selftest@example.invalid", "commit", "-q", "-m", "fixture"],
        ):
            subprocess.run(["git", "-C", str(candidate), *args], env=git_env, check=True, capture_output=True)
        run_id = begin_run(evdir, candidate, [], [evdir, reportdir])["run_id"]
        (evdir / "toolchain.doctor.json").write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "kind": "toolchain",
                    "id": "toolchain.doctor",
                    "run_id": run_id,
                    "status": "PASS",
                    "started_at": "selftest",
                    "elapsed_ms": 1,
                    "observed": {
                        "checks": [{"name": "pgbench", "required": True, "ok": True, "version": "synthetic-selftest"}]
                    },
                    "violations": [],
                    "warnings": [],
                    "reproduction": {},
                }
            ),
            encoding="utf-8",
        )
        (evdir / "bench.selftest.c1.json").write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "kind": "pgbench",
                    "id": "bench.selftest.c1",
                    "run_id": run_id,
                    "status": "PASS",
                    "started_at": "selftest",
                    "elapsed_ms": 10,
                    "observed": {
                        "clients": 1,
                        "successful_repetitions": 3,
                        "repetitions": 3,
                        "tps": 100,
                        "tps_cv": 0.01,
                        "latency_ms": {"p50": 1, "p95": 2, "p99": 3, "max": 4},
                        "failure_rate": 0,
                        "total_retries": 0,
                    },
                    "violations": [],
                    "warnings": [],
                    "reproduction": {
                        "workload_config": {"duration_s": 1},
                        "scripts": [],
                        "host": {"machine": "selftest", "cpu_count": 1},
                    },
                }
            ),
            encoding="utf-8",
        )
        gate = reportdir / "gate.json"
        report_policy = reportdir / "report-policy.json"
        report_policy.write_text(
            json.dumps({"required_evidence": [{"kind": "toolchain"}, {"kind": "pgbench"}]}), encoding="utf-8"
        )
        ok_report_gate, report_gate_output = run_cmd(
            [
                sys.executable,
                str(ROOT / "scripts" / "release_gate.py"),
                "--evidence-dir",
                str(evdir),
                "--policy",
                str(report_policy),
                "--root",
                str(candidate),
                "--out",
                str(gate),
            ]
        )
        md = reportdir / "report.md"
        htm = reportdir / "report.html"
        ok, out = run_cmd(
            [
                sys.executable,
                str(ROOT / "scripts" / "evidence_report.py"),
                "--evidence-dir",
                str(evdir),
                "--gate",
                str(gate),
                "--config",
                str(ROOT / "examples" / "sql-verification.toml"),
                "--root",
                str(candidate),
                "--out-md",
                str(md),
                "--out-html",
                str(htm),
            ]
        )
        result["report_generation"] = (
            ok_report_gate and ok and md.exists() and htm.exists() and "<table>" in htm.read_text(encoding="utf-8")
        )
        if not result["report_generation"]:
            result["errors"].append(report_gate_output + "\n" + out)

        # Baseline path self-test uses synthetic evidence only; it checks artifact integrity/governance code, not DB performance.
        snap = {
            "schema_version": 1,
            "kind": "postgres_snapshot",
            "id": "postgres.snapshot.pre",
            "run_id": run_id,
            "status": "PASS",
            "started_at": "selftest",
            "observed": {
                "server_version_num": 180001,
                "settings_sha256": "a",
                "schema_fingerprint_sha256": "b",
                "host": {"machine": "selftest", "cpu_count": 1},
            },
            "violations": [],
            "warnings": [],
            "reproduction": {},
        }
        (evdir / "postgres.snapshot.pre.json").write_text(json.dumps(snap), encoding="utf-8")
        baseline = td / "baseline"
        baseline_policy = reportdir / "baseline-policy.json"
        baseline_policy.write_text(
            json.dumps(
                {"required_evidence": [{"kind": "toolchain"}, {"kind": "pgbench"}, {"kind": "postgres_snapshot"}]}
            ),
            encoding="utf-8",
        )
        ok_gate, out_gate = run_cmd(
            [
                sys.executable,
                str(ROOT / "scripts" / "release_gate.py"),
                "--evidence-dir",
                str(evdir),
                "--policy",
                str(baseline_policy),
                "--root",
                str(candidate),
                "--out",
                str(gate),
            ]
        )
        ok1, out1 = (
            run_cmd(
                [
                    sys.executable,
                    str(ROOT / "scripts" / "baseline_promote.py"),
                    "--evidence-dir",
                    str(evdir),
                    "--gate",
                    str(gate),
                    "--destination",
                    str(baseline),
                    "--root",
                    str(candidate),
                ]
            )
            if ok_gate
            else (False, out_gate)
        )
        reg = td / "regression.json"
        ok2, out2 = (
            run_cmd(
                [
                    sys.executable,
                    str(ROOT / "scripts" / "regression_compare.py"),
                    "--current-dir",
                    str(evdir),
                    "--baseline-dir",
                    str(baseline),
                    "--out",
                    str(reg),
                ]
            )
            if ok1
            else (False, "promotion failed")
        )
        result["baseline_promotion_and_compare"] = ok1 and ok2 and reg.exists()
        if not result["baseline_promotion_and_compare"]:
            result["errors"].append(out1 + "\n" + out2)

        # Negative controls: removing the baseline or the run identity must never yield PASS.
        empty = td / "empty-baseline"
        empty.mkdir()
        ok_empty, out_empty = run_cmd(
            [
                sys.executable,
                str(ROOT / "scripts" / "regression_compare.py"),
                "--current-dir",
                str(evdir),
                "--baseline-dir",
                str(empty),
                "--out",
                str(td / "regression-empty.json"),
            ]
        )
        unbound = td / "unbound"
        unbound.mkdir()
        (unbound / "bench.selftest.c1.json").write_text(
            (evdir / "bench.selftest.c1.json").read_text(encoding="utf-8"), encoding="utf-8"
        )
        policy = td / "policy.json"
        policy.write_text(json.dumps({"required_evidence": [{"id": "bench.selftest.c1"}]}), encoding="utf-8")
        ok_stale, out_stale = run_cmd(
            [
                sys.executable,
                str(ROOT / "scripts" / "release_gate.py"),
                "--policy",
                str(policy),
                "--evidence-dir",
                str(unbound),
                "--root",
                str(td),
            ]
        )
        empty_status = (
            json.loads((td / "regression-empty.json").read_text(encoding="utf-8")).get("status")
            if (td / "regression-empty.json").exists()
            else None
        )
        result["fail_closed_controls"] = (
            (not ok_empty) and empty_status == "BLOCKED" and (not ok_stale) and "missing_run_manifest" in out_stale
        )
        if not result["fail_closed_controls"]:
            result["errors"].append(out_empty + "\n" + out_stale)

    keys = [
        "compile",
        "unit_tests",
        "config",
        "parameter_corpus",
        "report_generation",
        "baseline_promotion_and_compare",
        "fail_closed_controls",
    ]
    result["status"] = "PASS" if all(result[k] for k in keys) else "FAIL"
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
