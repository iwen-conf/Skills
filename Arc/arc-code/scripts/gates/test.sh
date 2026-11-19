#!/usr/bin/env bash
# Atomic verify_cmd runner: executes .arc-task.json verify_cmd (or explicit --cmd)
# and emits compact JSON capped to --max-errors lines on failure.
set -euo pipefail

TASK_FILE=".arc-task.json"
EXPLICIT_CMD=""
JSON_OUT=0

while [ $# -gt 0 ]; do
  case "$1" in
    --task-file) TASK_FILE="$2"; shift 2 ;;
    --cmd) EXPLICIT_CMD="$2"; shift 2 ;;
    --json) JSON_OUT=1; shift ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "not inside a git repo" >&2; exit 2; }
cd "$ROOT"

python3 - "$TASK_FILE" "$EXPLICIT_CMD" "$JSON_OUT" <<'PY'
import json, subprocess, sys
from pathlib import Path

task_file, explicit_cmd, json_out_s = sys.argv[1:4]
json_out = json_out_s == "1"

cmd = explicit_cmd.strip()
if not cmd and Path(task_file).is_file():
    task = json.loads(Path(task_file).read_text(encoding="utf-8"))
    cmd = str(task.get("verify_cmd", "")).strip()

if not cmd:
    payload = {"gate": "test", "status": "NOT_RUN", "reason": "no verify_cmd in .arc-task.json or --cmd"}
    print(json.dumps(payload, indent=2) if json_out else "test: NOT_RUN (no verify_cmd)")
    sys.exit(0)

res = subprocess.run(["sh", "-c", cmd], text=True, capture_output=True)
combined = (res.stdout + "\n" + res.stderr).strip().splitlines()
status = "PASS" if res.returncode == 0 else "FAIL"
payload = {
    "gate": "test",
    "status": status,
    "cmd": cmd,
    "exit_code": res.returncode,
    "tail": combined[-10:] if combined else [],
}
if json_out:
    print(json.dumps(payload, indent=2, ensure_ascii=False))
else:
    print(f"test: {status} (exit {res.returncode})")
    if status != "PASS":
        for l in payload["tail"]:
            print("  " + l)
sys.exit(0 if status == "PASS" else 1)
PY
