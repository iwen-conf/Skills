#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tags_file="${AI_CODE_TAGS_FILE:-$repo_root/.ai-code-index/tags}"

if ! command -v ctags >/dev/null 2>&1; then
  echo "ctags is not installed or not on PATH." >&2
  exit 127
fi

cd "$repo_root"
ctags \
  --recurse=yes \
  --tag-relative=yes \
  --exclude=.git \
  --exclude=node_modules \
  --exclude=dist \
  --exclude=build \
  --exclude=target \
  --exclude=.next \
  --exclude=.nuxt \
  --exclude=coverage \
  --exclude=tmp \
  --exclude=vendor \
  --exclude=.arc \
  --exclude=.magi \
  --exclude=.ace-tool \
  --exclude=.ai-code-index \
  --exclude=.venv \
  --exclude=.cocoindex_code \
  --exclude=.pytest_cache \
  --exclude=.mypy_cache \
  --exclude=.ruff_cache \
  --exclude=__pycache__ \
  -f "$tags_file" \
  .

if [[ $# -gt 0 ]]; then
  grep -i -- "$*" "$tags_file" || true
else
  echo "Wrote $tags_file"
fi
