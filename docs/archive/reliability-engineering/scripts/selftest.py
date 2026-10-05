#!/usr/bin/env python3
"""Fast hermetic self-test for the skill's core planning/analysis logic.

This intentionally avoids Docker, databases, network access, Rust/Go toolchain
invocation, and privileged host checks. Real-environment checks belong to
explicit integration tests/doctor commands.
"""
from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
sys.path.insert(0, str(HERE))

import business_scenario_runner  # noqa: E402
import business_test_plan  # noqa: E402
import business_txn_lab  # noqa: E402
import verification_toolchain  # noqa: E402


def check(name: str, condition: bool, detail: str = "") -> None:
    if not condition:
        raise AssertionError(f"{name}: {detail}")
    print(f"PASS {name}")


def main() -> int:
    plan = business_test_plan.load_plan(ROOT / "examples/business-test-plan.toml")
    issues = business_test_plan.validate(plan)
    check("business-plan", not any(x["severity"] == "error" for x in issues), str(issues))

    binding = business_scenario_runner.load_toml(ROOT / "examples/business-scenario-bindings.toml")
    binding_errors = business_scenario_runner.validate_binding(binding, plan)
    check("scenario-binding", not binding_errors, str(binding_errors))

    catalog = business_txn_lab.load_toml(ROOT / "examples/business-transactions.toml")
    report = business_txn_lab.analyze_catalog(catalog)
    codes = {x["code"] for x in report["findings"]}
    check("transaction-analyzer", "TXN-LOCK-ORDER-INVERSION" in codes or "TXN-DYNAMIC-KEY-ORDER" in codes,
          f"expected adversarial example finding, got {sorted(codes)}")

    suites = verification_toolchain.SUITES
    check("toolchain-catalog", "rust-foundation" in suites and "go-foundation" in suites)
    check("no-score-gate", "score" not in (plan.get("release") or {}), "release plan must be scenario/evidence based")

    print("SELFTEST: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
