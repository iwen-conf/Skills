#!/usr/bin/env bash
# Detects lazy LLM code generation (placeholders, TODOs) in the current git diff.
# Exits non-zero if lazy placeholders are found.

set -euo pipefail

echo "=> Checking for lazy placeholders in unstaged/staged code..."

# If not in a git repo, skip
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "=> [SKIP] Not a git repository."
  exit 0
fi

# Patterns that indicate the LLM was lazy
LAZY_PATTERNS=(
  '// \.\.\.'
  '/\* \.\.\. \*/'
  '# \.\.\.'
  '<!-- \.\.\. -->'
  '// existing code'
  '# existing code'
  '// rest of'
  '# rest of'
  'TODO: implement'
  'TBD'
)

FAILED=0

# Check both unstaged and staged changes
for pattern in "${LAZY_PATTERNS[@]}"; do
  if git diff HEAD -U0 | grep -qFi "+$pattern" || git diff HEAD -U0 | grep -qEi "^\+.*$pattern"; then
    echo "=> [FAIL] Lazy placeholder detected in diff: $pattern"
    FAILED=1
  fi
done

if [ $FAILED -ne 0 ]; then
  echo "=> [ERROR] You generated incomplete code with placeholders. Please replace them with actual implementation."
  exit 1
else
  echo "=> [OK] No lazy placeholders detected."
  exit 0
fi
