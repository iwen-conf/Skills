#!/usr/bin/env bash
# Detects scope creep by warning if too many files are modified during a focused task.

set -euo pipefail

MAX_FILES=${1:-5}

echo "=> Checking change scope..."

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "=> [SKIP] Not a git repository."
  exit 0
fi

CHANGED_FILES=$(git diff HEAD --name-only | wc -l | tr -d ' ')

if [ "$CHANGED_FILES" -gt "$MAX_FILES" ]; then
  echo "=> [FAIL] Scope Creep Detected: You modified $CHANGED_FILES files (Limit: $MAX_FILES)."
  echo "=> [ERROR] A focused fix/build should rarely touch more than $MAX_FILES files."
  echo "=> [ERROR] If this is an intentional 'expand' or 'cut' mode refactor, you must seek explicit user approval to proceed."
  exit 1
else
  echo "=> [OK] Change scope is within limits ($CHANGED_FILES files changed)."
  exit 0
fi
