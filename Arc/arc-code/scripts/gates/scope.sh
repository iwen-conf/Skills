#!/usr/bin/env bash
# Atomic scope gate: validates modified files against .arc-task.json (write_scope + frozen_contracts)
# or --scope / --max-files. Emits compact JSON capped to --max-errors (default 5).
set -euo pipefail

TASK_FILE=".arc-task.json"
SCOPE_FILE=""
MAX_FILES=0
MAX_ERRORS=5
JSON_OUT=0

while [ $# -gt 0 ]; do
  case "$1" in
    --task-file) TASK_FILE="$2"; shift 2 ;;
    --scope) SCOPE_FILE="$2"; shift 2 ;;
    --max-files) MAX_FILES="$2"; shift 2 ;;
    --max-errors) MAX_ERRORS="$2"; shift 2 ;;
    --json) JSON_OUT=1; shift ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "not inside a git repo" >&2; exit 2; }
cd "$ROOT"

python3 - "$TASK_FILE" "$SCOPE_FILE" "$MAX_FILES" "$MAX_ERRORS" "$JSON_OUT" <<'PY'
import fnmatch, json, subprocess, sys
from pathlib import Path

task_file, scope_file, max_files_s, max_errors_s, json_out_s = sys.argv[1:6]
max_files = int(max_files_s)
max_errors = max(1, int(max_errors_s))
json_out = json_out_s == "1"

def norm(p: str) -> str:
    p = p.strip().replace("\\", "/")
    while p.startswith("./"):
        p = p[2:]
    return p.rstrip("/")

def matches(path: str, pattern: str) -> bool:
    np, pat = norm(path), norm(pattern)
    if not pat:
        return False
    if pat.endswith("/**"):
        prefix = pat[:-3].rstrip("/")
        return np == prefix or np.startswith(prefix + "/")
    if fnmatch.fnmatch(np, pat):
        return True
    if not any(c in pat for c in "*?["):
        return np == pat or np.startswith(pat + "/")
    return False

res = subprocess.run(["git", "status", "--porcelain", "-uall"], text=True, capture_output=True, check=True)
changed = []
for line in res.stdout.splitlines():
    if len(line) < 4:
        continue
    p = line[3:].strip().strip('"')
    if " -> " in p:
        p = p.split(" -> ", 1)[1].strip().strip('"')
    if p in {".arc-task.json"} or p.startswith(".arc/"):
        continue
    changed.append(norm(p))

write_scope = []
frozen = []
if Path(task_file).is_file():
    task = json.loads(Path(task_file).read_text(encoding="utf-8"))
    write_scope = [str(x) for x in task.get("write_scope") or []]
    frozen = [str(x) for x in task.get("frozen_contracts") or []]

if scope_file and Path(scope_file).is_file():
    for raw in Path(scope_file).read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if line and not line.startswith("#"):
            write_scope.append(line)

errors = []
for p in changed:
    for locked in frozen:
        if matches(p, locked):
            errors.append({"code": "FROZEN_CONTRACT_MODIFIED", "path": p, "message": f"touched frozen contract {locked}"})
    if write_scope and not any(matches(p, pat) for pat in write_scope):
        errors.append({"code": "OUT_OF_WRITE_SCOPE", "path": p, "message": f"outside allowed write_scope ({len(write_scope)} rules)"})

if max_files > 0 and len(changed) > max_files:
    errors.append({"code": "MAX_FILES_EXCEEDED", "path": ".", "message": f"{len(changed)} changed files exceed --max-files {max_files}"})

status = "PASS" if not errors else "FAIL"
payload = {
    "gate": "scope",
    "status": status,
    "changed_count": len(changed),
    "total_errors": len(errors),
    "truncated": len(errors) > max_errors,
    "errors": errors[:max_errors],
}
if json_out:
    print(json.dumps(payload, indent=2, ensure_ascii=False))
else:
    print(f"scope: {status} ({len(changed)} changed files, {len(errors)} errors)")
    for e in payload["errors"]:
        print(f"  - [{e['code']}] {e['path']}: {e['message']}")
sys.exit(0 if status == "PASS" else 1)
PY
