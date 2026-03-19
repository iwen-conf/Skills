#!/usr/bin/env sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <tool> [args...]" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "missing required command: go" >&2
  exit 1
fi

if ! command -v shasum >/dev/null 2>&1; then
  echo "missing required command: shasum" >&2
  exit 1
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
MODULE_DIR=$SCRIPT_DIR
TOOL=$1
shift

SOURCE_HASH=$(   find "$MODULE_DIR" -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -print     | LC_ALL=C sort     | while IFS= read -r path; do shasum -a 256 "$path"; done     | shasum -a 256     | awk '{print substr($1, 1, 16)}'
)
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
CACHE_ROOT="${XDG_CACHE_HOME:-$HOME/.cache}/arcgocore"
CACHE_DIR="$CACHE_ROOT/${TOOL}/${GOOS}-${GOARCH}-${SOURCE_HASH}"
BIN_PATH="$CACHE_DIR/${TOOL}"
LOCK_DIR="${CACHE_DIR}.lock"

if [ ! -x "$BIN_PATH" ]; then
  mkdir -p "$CACHE_ROOT"
  mkdir -p "$(dirname "$LOCK_DIR")"

  while ! mkdir "$LOCK_DIR" 2>/dev/null; do
    if [ -x "$BIN_PATH" ]; then
      export ARC_GO_CACHE_BIN="$BIN_PATH"
      exec "$BIN_PATH" "$@"
    fi
    sleep 0.1
  done

  cleanup_lock() {
    rmdir "$LOCK_DIR" 2>/dev/null || true
  }
  trap cleanup_lock EXIT INT TERM

  if [ ! -x "$BIN_PATH" ]; then
    mkdir -p "$CACHE_DIR"
    TMP_BIN="$CACHE_DIR/.${TOOL}.tmp.$$"
    (
      cd "$MODULE_DIR"
      go build -trimpath -ldflags='-s -w' -o "$TMP_BIN" "./cmd/$TOOL"
    )
    chmod +x "$TMP_BIN"
    mv "$TMP_BIN" "$BIN_PATH"
  fi
fi

export ARC_GO_CACHE_BIN="$BIN_PATH"
exec "$BIN_PATH" "$@"
