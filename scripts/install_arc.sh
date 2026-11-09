#!/usr/bin/env bash
# Build all current Arc engines, then publish the single public CLI command.
# Engines go to <prefix>/libexec/arc, outside PATH; only `arc` is callable by name.
# Usage: bash scripts/install_arc.sh [BIN_DIR]
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
dest="${1:-${HOME}/.local/bin}"
mkdir -p -- "$dest"
dest="$(cd -- "$dest" && pwd)"
private="$(dirname -- "$dest")/libexec/arc"

if ! command -v go >/dev/null 2>&1; then
  printf 'arc installer: Go compiler not found on PATH\n' >&2
  exit 127
fi

stage="$(mktemp -d "${TMPDIR:-/tmp}/arc-cli.XXXXXXXX")"
trap 'rm -rf -- "$stage"' EXIT

for engine in arc-idx arc-verify arc; do
  printf 'building %s ...\n' "$engine" >&2
  go -C "$root/tools/$engine" build -o "$stage/$engine" .
done

# Build every binary successfully before publishing any of them.
mkdir -p -- "$private"
for engine in arc-idx arc-verify; do
  mv -f -- "$stage/$engine" "$private/$engine"
done
mv -f -- "$stage/arc" "$dest/arc"

# Earlier installers published the engines beside arc; that copy would stay callable.
for engine in arc-idx arc-verify; do
  if [ -e "$dest/$engine" ] || [ -L "$dest/$engine" ]; then
    rm -f -- "$dest/$engine"
    printf 'removed public engine %s/%s (now private in %s)\n' "$dest" "$engine" "$private" >&2
  fi
done

"$dest/arc" version
"$dest/arc" capabilities --json
printf 'arc installed at %s/arc (ensure this directory is on PATH)\n' "$dest" >&2
