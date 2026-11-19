#!/usr/bin/env bash
# Atomic hygiene gate: scans added git diff lines for placeholders, lint suppressions,
# and swallowed-error smells. Emits compact JSON capped to --max-errors (default 5).
set -euo pipefail

MAX_ERRORS=5
JSON_OUT=0

while [ $# -gt 0 ]; do
  case "$1" in
    --max-errors) MAX_ERRORS="$2"; shift 2 ;;
    --json) JSON_OUT=1; shift ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "not inside a git repo" >&2; exit 2; }
cd "$ROOT"

python3 - "$MAX_ERRORS" "$JSON_OUT" <<'PY'
import json, re, subprocess, sys

max_errors = max(1, int(sys.argv[1]))
json_out = sys.argv[2] == "1"

RULES = [
    ("SUPPRESSION", re.compile(r"(//\s*nolint|eslint-disable|@ts-ignore|#\s*type:\s*ignore|pyright:\s*ignore)")),
    ("PLACEHOLDER", re.compile(r"\b(TODO|FIXME|XXX|HACK)\b|raise\s+NotImplementedError")),
    ("SLEEP_SYNC", re.compile(r"\b(time\.Sleep|time\.sleep)\s*\(")),
]

diff = subprocess.run(["git", "diff", "-U0", "HEAD"], text=True, capture_output=True)
cur_file = ""
errors = []
for line in diff.stdout.splitlines():
    if line.startswith("+++ b/"):
        cur_file = line[6:]
        continue
    if not line.startswith("+") or line.startswith("+++"):
        continue
    if cur_file.startswith(".arc/") or cur_file.endswith("_test.go") or "test_" in cur_file:
        continue
    added = line[1:]
    for code, rx in RULES:
        if rx.search(added):
            errors.append({"code": code, "path": cur_file, "message": added.strip()[:120]})
            break

status = "PASS" if not errors else "FAIL"
payload = {
    "gate": "hygiene",
    "status": status,
    "total_errors": len(errors),
    "truncated": len(errors) > max_errors,
    "errors": errors[:max_errors],
}
if json_out:
    print(json.dumps(payload, indent=2, ensure_ascii=False))
else:
    print(f"hygiene: {status} ({len(errors)} findings)")
    for e in payload["errors"]:
        print(f"  - [{e['code']}] {e['path']}: {e['message']}")
sys.exit(0 if status == "PASS" else 1)
PY
