#!/usr/bin/env bash
# Enforces strict typing by rejecting newly added type bypasses in the git diff.

set -euo pipefail

echo "=> Checking for forbidden type bypasses in unstaged/staged code..."

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "=> [SKIP] Not a git repository."
  exit 0
fi

# Patterns that bypass the type system or linters
FORBIDDEN_PATTERNS=(
  '@ts-ignore'
  '@ts-expect-error'
  '@ts-nocheck'
  'eslint-disable'
  '# type: ignore'
  'as any'
  ': any'
)

FAILED=0

# Check diff for additions containing forbidden patterns
for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
  # Use grep to find lines starting with '+' (additions) that contain the pattern
  # Ignoring case for flexibility
  if git diff HEAD -U0 | grep -qEi "^\+.*$pattern"; then
    echo "=> [FAIL] Forbidden type/lint bypass detected in diff: '$pattern'"
    FAILED=1
  fi
done

if [ $FAILED -ne 0 ]; then
  echo "=> [ERROR] You are attempting to bypass the type system or linter."
  echo "=> [ERROR] The Iron Law requires you to fix the underlying type misalignment instead of suppressing it."
  echo "=> [ERROR] Remove the bypass, fix the types, and try again."
  exit 1
else
  echo "=> [OK] No type bypasses detected."
  exit 0
fi
