#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
index_root="${AI_CODE_INDEX_DIR:-$HOME/.cache/ai-code-index/zoekt}"
repo_name="$(basename "$repo_root")"
index_dir="$index_root/$repo_name"

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <zoekt-query>" >&2
  exit 2
fi

if ! command -v zoekt >/dev/null 2>&1; then
  echo "zoekt is not installed or not on PATH." >&2
  exit 127
fi

if ! compgen -G "$index_dir/*.zoekt" >/dev/null; then
  echo "No Zoekt index found at $index_dir. Run .ai-code-index/reindex.sh first." >&2
  exit 1
fi

cd "$repo_root"
zoekt -index_dir "$index_dir" "$@"
