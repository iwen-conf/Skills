#!/usr/bin/env python3
from __future__ import annotations

import compileall
import json
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def run_cmd(args: list[str]) -> tuple[bool, str]:
    cp = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return cp.returncode == 0, cp.stdout


def main() -> int:
    result = {
        "compile": False,
        "unit_tests": False,
        "config": False,
        "parameter_corpus": False,
        "report_generation": False,
        "baseline_promotion_and_compare": False,
        "errors": [],
    }
    result["compile"] = bool(compileall.compile_dir(str(ROOT / "scripts"), quiet=1) and compileall.compile_dir(str(ROOT / "tests"), quiet=1))

    ok, out = run_cmd([sys.executable, "-m", "unittest", "discover", "-s", str(ROOT / "tests"), "-v"])
    result["unit_tests"] = ok
    if not ok: result["errors"].append(out)

    ok, out = run_cmd([sys.executable, str(ROOT / "scripts" / "sqlverify_config.py"), str(ROOT / "examples" / "sql-verification.toml"), "--root", str(ROOT), "--json"])
    result["config"] = ok
    if not ok: result["errors"].append(out)

    with tempfile.TemporaryDirectory() as td:
        td = Path(td)
        corpus = td / "corpus.json"
        ok, out = run_cmd([sys.executable, str(ROOT / "scripts" / "parameter_corpus.py"), str(ROOT / "examples" / "parameter-corpus-spec.json"), "--out", str(corpus)])
        result["parameter_corpus"] = ok and corpus.exists()
        if not result["parameter_corpus"]: result["errors"].append(out)

        evdir = td / "evidence"; evdir.mkdir()
        reportdir = td / "report"; reportdir.mkdir()
        (evdir / "toolchain.doctor.json").write_text(json.dumps({
            "schema_version":1,"kind":"toolchain","id":"toolchain.doctor","status":"PASS","started_at":"selftest",
            "elapsed_ms":1,"observed":{"checks":[{"name":"pgbench","required":True,"ok":True,"version":"synthetic-selftest"}]},
            "violations":[],"warnings":[],"reproduction":{}
        }), encoding="utf-8")
        (evdir / "bench.selftest.c1.json").write_text(json.dumps({
            "schema_version":1,"kind":"pgbench","id":"bench.selftest.c1","status":"PASS","started_at":"selftest","elapsed_ms":10,
            "observed":{"clients":1,"successful_repetitions":3,"repetitions":3,"tps":100,"tps_cv":0.01,"latency_ms":{"p50":1,"p95":2,"p99":3,"max":4},"failure_rate":0,"total_retries":0},
            "violations":[],"warnings":[],"reproduction":{"workload_config":{"duration_s":1},"scripts":[],"host":{"machine":"selftest","cpu_count":1}}
        }), encoding="utf-8")
        gate = reportdir / "gate.json"
        gate.write_text(json.dumps({"status":"PASS","failures":[],"warnings":[]}), encoding="utf-8")
        md = reportdir / "report.md"; htm = reportdir / "report.html"
        ok, out = run_cmd([sys.executable, str(ROOT / "scripts" / "evidence_report.py"), "--evidence-dir", str(evdir), "--gate", str(gate), "--config", str(ROOT / "examples" / "sql-verification.toml"), "--out-md", str(md), "--out-html", str(htm)])
        result["report_generation"] = ok and md.exists() and htm.exists() and "<table>" in htm.read_text(encoding="utf-8")
        if not result["report_generation"]: result["errors"].append(out)

        # Baseline path self-test uses synthetic evidence only; it checks artifact integrity/governance code, not DB performance.
        snap = {
            "schema_version":1,"kind":"postgres_snapshot","id":"postgres.snapshot.pre","status":"PASS","started_at":"selftest",
            "observed":{"server_version_num":180001,"settings_sha256":"a","schema_fingerprint_sha256":"b","host":{"machine":"selftest","cpu_count":1}},
            "violations":[],"warnings":[],"reproduction":{}
        }
        (evdir / "postgres.snapshot.pre.json").write_text(json.dumps(snap), encoding="utf-8")
        baseline = td / "baseline"
        ok1, out1 = run_cmd([sys.executable, str(ROOT / "scripts" / "baseline_promote.py"), "--evidence-dir", str(evdir), "--gate", str(gate), "--destination", str(baseline), "--root", str(ROOT)])
        reg = td / "regression.json"
        ok2, out2 = run_cmd([sys.executable, str(ROOT / "scripts" / "regression_compare.py"), "--current-dir", str(evdir), "--baseline-dir", str(baseline), "--out", str(reg)]) if ok1 else (False, "promotion failed")
        result["baseline_promotion_and_compare"] = ok1 and ok2 and reg.exists()
        if not result["baseline_promotion_and_compare"]: result["errors"].append(out1 + "\n" + out2)

    keys = ["compile", "unit_tests", "config", "parameter_corpus", "report_generation", "baseline_promotion_and_compare"]
    result["status"] = "PASS" if all(result[k] for k in keys) else "FAIL"
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
