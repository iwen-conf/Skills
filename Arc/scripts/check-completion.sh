#!/usr/bin/env bash
# Completion gate for code rot #34 / #35.
# "Done" means: no placeholders, scope under control, and the project still verifies.
# Composes the existing Arc scripts — adds no new detection logic of its own.
# Run from the project root. Exits non-zero if any gate fails.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=> Completion gate: checking that 'done' is actually done..."

FAILED=0

run_gate() {
  local label="$1"
  local script="$2"
  shift 2
  echo "----------------------------------------------------------------"
  echo "=> Gate: $label"
  if bash "$SCRIPT_DIR/$script" "$@"; then
    echo "=> [PASS] $label"
  else
    echo "=> [FAIL] $label"
    FAILED=1
  fi
}

# 1. No leftover placeholders / half-implemented markers (#35).
run_gate "No placeholders" "check-placeholders.sh"

# 2. Scope under control — a finished task should not sprawl (#32 / scope creep).
run_gate "Scope in bounds" "check-scope.sh"

# 3. Project still runs and passes its own checks (#34).
run_gate "Project verifies" "verify-project.sh"

echo "================================================================"
if [ $FAILED -ne 0 ]; then
  echo "=> [ERROR] Completion gate FAILED. The work is not done — fix the gates above before reporting completion."
  echo "=> Do not claim 'done' while any gate is red (code rot #34 / #35)."
  exit 1
else
  echo "=> [OK] Completion gate passed: no placeholders, scope bounded, project verifies."
  exit 0
fi
