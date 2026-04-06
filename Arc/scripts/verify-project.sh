#!/usr/bin/env bash
# Auto-detect and run project verification (lint + typecheck + tests).
# Run from the project root. Exits non-zero on failure.

set -euo pipefail

echo "Running automated verification..."

# Function to run and report
run_check() {
  local cmd="$1"
  echo "=> Executing: $cmd"
  if eval "$cmd"; then
    echo "=> [PASS] $cmd"
    return 0
  else
    echo "=> [FAIL] $cmd"
    return 1
  fi
}

FAILED=0

# Go
if [ -f go.mod ]; then
  run_check "go fmt ./..." || FAILED=1
  run_check "go vet ./..." || FAILED=1
  run_check "staticcheck ./..." || echo "=> [WARN] staticcheck not found or failed, continuing..."
  run_check "go test ./..." || FAILED=1

# Rust
elif [ -f Cargo.toml ]; then
  run_check "cargo fmt -- --check" || FAILED=1
  run_check "cargo clippy -- -D warnings" || FAILED=1
  run_check "cargo test" || FAILED=1

# Node / TypeScript
elif [ -f package.json ]; then
  if [ -f tsconfig.json ]; then
    run_check "npx tsc --noEmit" || FAILED=1
  fi
  
  if grep -q '"lint"' package.json; then
    run_check "npm run lint" || FAILED=1
  fi
  
  if grep -q '"test"' package.json; then
    run_check "npm test" || FAILED=1
  fi

# Python (uv / pip / standard)
elif [ -f pyproject.toml ] || [ -f requirements.txt ] || [ -f uv.lock ]; then
  if command -v ruff >/dev/null 2>&1; then
    run_check "ruff check ." || FAILED=1
    run_check "ruff format --check ." || FAILED=1
  fi
  
  if command -v mypy >/dev/null 2>&1; then
    run_check "mypy ." || FAILED=1
  fi
  
  if command -v pytest >/dev/null 2>&1; then
    run_check "pytest" || FAILED=1
  fi

# Makefile fallback
elif [ -f Makefile ] && grep -q '^test:' Makefile; then
  run_check "make test" || FAILED=1

else
  echo "(no standard test command detected — ask the user for the verification command)"
  exit 1
fi

if [ $FAILED -ne 0 ]; then
  echo "=> [ERROR] Verification failed."
  exit 1
else
  echo "=> [OK] All verifications passed."
  exit 0
fi
