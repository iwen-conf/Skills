#!/usr/bin/env bash
# Cursor Agent hook entry. Fail-open: hook.py already swallows engine errors.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PLUGIN_ROOT="$ROOT"
export CLAUDE_PLUGIN_ROOT="$ROOT"
export ARC_POLICY_CURSOR_NATIVE=1
exec python3 "$ROOT/hooks/hook.py"
