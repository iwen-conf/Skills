#!/usr/bin/env bash
# Reject newly added type-check or lint suppression shortcuts in the git diff.

set -euo pipefail

echo "=> Checking for forbidden type/lint suppressions in unstaged/staged code..."

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "=> [SKIP] Not a git repository."
  exit 0
fi

# Patterns that bypass type checking or linters
FORBIDDEN_PATTERNS=(
  'eslint-disable'
  '# type: ignore'
  'pyright: ignore'
)

FAILED=0

# Check diff for additions containing forbidden patterns
for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
  # Use grep to find lines starting with '+' (additions) that contain the pattern
  # Ignoring case for flexibility
  if git diff HEAD -U0 | grep -qEi "^\+.*$pattern"; then
    echo "=> [FAIL] Forbidden type/lint suppression detected in diff: '$pattern'"
    FAILED=1
  fi
done

if [ $FAILED -ne 0 ]; then
  echo "=> [ERROR] You are attempting to bypass type checking or linting."
  echo "=> [ERROR] The Iron Law requires you to fix the underlying contract mismatch instead of suppressing it."
  echo "=> [ERROR] Remove the suppression, fix the issue, and try again."
  exit 1
else
  echo "=> [OK] No type/lint suppressions detected."
  exit 0
fi
