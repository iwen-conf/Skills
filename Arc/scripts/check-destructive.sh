#!/usr/bin/env bash
# Block destructive commands during automated tool usage.
# Returns 0 if safe, exits non-zero if blocked.

set -euo pipefail

# Read command from stdin or argument
INPUT="${1:-$(cat)}"

if [[ -z "$INPUT" ]]; then
  exit 0
fi

# Patterns that must never run automatically
DESTRUCTIVE_PATTERNS=(
  'rm -rf /'
  'rm -rf ~'
  'git push --force'
  'git push -f '
  'git reset --hard'
  'git clean -f'
  'git checkout .'
  'DROP DATABASE'
  'DROP TABLE'
  'TRUNCATE TABLE'
  'chmod -R 777 /'
  'chown -R '
  '> /etc/'
  '>> /etc/'
  'mkfs'
  'dd if='
)

# Loop prevention (basic heuristic for unbatched concurrent spawns)
if echo "$INPUT" | grep -qE '(while true; do|for i in .* \*)'; then
  if echo "$INPUT" | grep -qE 'sleep '; then
    # Has a sleep, might be okay
    :
  else
    echo "BLOCK: Unbounded loop detected without sleep. Prevented CPU spike." >&2
    exit 2
  fi
fi

for pattern in "${DESTRUCTIVE_PATTERNS[@]}"; do
  if echo "$INPUT" | grep -qF "$pattern"; then
    echo "BLOCK: Destructive command detected: $pattern" >&2
    echo "Confirm with user before proceeding." >&2
    exit 2
  fi
done

# Code rot #36: unreviewed project-wide bulk rewrites.
# In-place mass edits (sed -i / perl -i / find -exec sed / xargs sed) and
# whole-tree reverts can break the whole project with no easy rollback.
BULK_REWRITE_PATTERNS=(
  'sed -i'
  'perl -i'
  'find .* -exec sed'
  'find .* -exec perl'
  'xargs .*sed'
  'xargs .*perl'
  'git checkout -- \.'
  'git checkout \.'
  'git restore \.'
)

for pattern in "${BULK_REWRITE_PATTERNS[@]}"; do
  if echo "$INPUT" | grep -qE "$pattern"; then
    echo "BLOCK: Unreviewed bulk rewrite detected (code rot #36): $pattern" >&2
    echo "Ensure a clean commit or stash exists for rollback, then edit directory-by-directory and verify between steps." >&2
    exit 2
  fi
done

exit 0
