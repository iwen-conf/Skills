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

exit 0
