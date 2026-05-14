#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
index_root="${AI_CODE_INDEX_DIR:-$HOME/.cache/ai-code-index/zoekt}"
repo_name="$(basename "$repo_root")"
index_dir="$index_root/$repo_name"

mkdir -p "$index_dir"

if ! command -v zoekt-index >/dev/null 2>&1; then
  echo "zoekt-index is not installed or not on PATH." >&2
  exit 127
fi

cd "$repo_root"
rm -f "$index_dir"/*.zoekt
zoekt-index \
  -index "$index_dir" \
  -ignore_dirs ".git,node_modules,dist,build,target,.next,.nuxt,coverage,tmp,vendor,.arc,.magi,.ace-tool,.venv,.cocoindex_code,.pytest_cache,.mypy_cache,.ruff_cache,__pycache__" \
  .

echo "Indexed $repo_root into $index_dir"
