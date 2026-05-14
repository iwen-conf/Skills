#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <language> <ast-grep-pattern> [paths...]" >&2
  exit 2
fi

if ! command -v sg >/dev/null 2>&1; then
  echo "ast-grep command 'sg' is not installed or not on PATH." >&2
  exit 127
fi

language="$1"
pattern="$2"
shift 2

cd "$repo_root"
if [[ $# -gt 0 ]]; then
  sg run --lang "$language" --pattern "$pattern" "$@"
else
  sg run --lang "$language" --pattern "$pattern" .
fi
