#!/usr/bin/env bash
# Real-backend local Turso smoke in disposable repositories only.
set -euo pipefail
bin_dir="${1:?usage: smoke.sh BIN_DIR SKILLS_ROOT}"
skills_root="${2:?usage: smoke.sh BIN_DIR SKILLS_ROOT}"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT

cd "$tmp"
git init -q
git config user.name "Arc CI"
git config user.email "arc@example.invalid"
echo "baseline" > README.txt
git add README.txt
git commit -qm baseline
mkdir -p .arc

"$bin_dir/arc" db init --repo . > .arc/init.json
python3 -c 'import json; assert json.load(open(".arc/init.json"))["active"] is True'
"$bin_dir/arc" task create --title "smoke test" > .arc/task.json
task_id="$(python3 -c 'import json; print(json.load(open(".arc/task.json"))["id"])')"
"$bin_dir/arc" task transition --id "$task_id" --to ready --expected-version 1 > .arc/ready.json
echo "new code" > feature.go
"$bin_dir/arc" file update --path feature.go --task "$task_id" > .arc/file.json
"$bin_dir/arc" file show --path feature.go > .arc/file-show.json
python3 -c 'import json; assert json.load(open(".arc/file-show.json"))["version"]==1'

# Policy task ID import must be idempotent, and a change must retain its owner.
"$bin_dir/arc" task ensure --id policy-smoke-01 --title "policy smoke (arc-code)" > .arc/policy-task.json
"$bin_dir/arc" task ensure --id policy-smoke-01 --title "policy smoke (arc-code)" > .arc/policy-duplicate.json
python3 -c 'import json; assert json.load(open(".arc/policy-duplicate.json"))["duplicate"] is True'
"$bin_dir/arc" update --changed-only --task policy-smoke-01 > .arc/update.json
python3 -c 'import json; assert json.load(open(".arc/update.json"))["changed_files"]>0'
# This executes the real Zoekt/ctags indexer. Capture results under .arc,
# which is excluded from source-index reconciliation.
"$bin_dir/arc" index flush --repo . > .arc/index-flush.json
python3 -c 'import json; d=json.load(open(".arc/index-flush.json")); assert d["index"]=="healthy" and d["indexed_events"]>0'
"$bin_dir/arc" db status > .arc/index-status.json
python3 -c 'import json; assert json.load(open(".arc/index-status.json"))["pending_events"]==0'
"$bin_dir/arc" event list --resource task --id policy-smoke-01 --limit 10 > .arc/events.json
python3 -c 'import json; a=json.load(open(".arc/events.json")); assert a["count"]==1 and a["events"][0]["kind"]=="task.imported"'
"$bin_dir/arc" db status > .arc/status.json
python3 -c 'import json; a=json.load(open(".arc/status.json")); assert a["active"] and a["pending_events"]==0'

# Independent verification: a separate repository holding a legacy graph.
mkdir "$tmp/old"
cd "$tmp/old"
git init -q
git config user.name "Arc CI"
git config user.email "arc@example.invalid"
echo "baseline" > README.txt
git add README.txt
git commit -qm baseline
"$bin_dir/arc-verify" init >/dev/null
"$bin_dir/arc-verify" import --file "$skills_root/tools/arc-verify/examples/academic-v1.json" >/dev/null
"$bin_dir/arc" db migrate --confirm-offline > .arc/migrated.json
python3 -c 'import json; x=json.load(open(".arc/migrated.json")); assert x["active"] and x["migrated_graphs"]==1'
"$bin_dir/arc" db status > .arc/status.json
python3 -c 'import json; x=json.load(open(".arc/status.json")); assert x["graph_count"]==1'
# Physical Policy state cutover in the same migrated Turso database.
mkdir -p .arc/policy/receipts
python3 - <<'PY'
import json
from pathlib import Path
receipt={"task_id":"smoke-task","verdict":"PASS","slug":"smoke","skill":"arc-code"}
state={"active":{"id":"smoke-task","slug":"smoke","skill":"arc-code","receipt":receipt},"history":[]}
Path(".arc/policy/state.json").write_text(json.dumps(state))
Path(".arc/policy/receipts/smoke.json").write_text(json.dumps(receipt))
PY
"$bin_dir/arc" policy state migrate --repo . --confirm-offline > .arc/policy-migrated.json
python3 -c 'import json; p=json.load(open(".arc/policy-migrated.json")); assert p["storage"]=="arc.db" and p["imported_receipts"] >= 1'
"$bin_dir/arc" policy state get --repo . > .arc/policy-state.json
python3 -c 'import json; p=json.load(open(".arc/policy-state.json")); assert p["mode"]=="arc.db" and p["state"]["active"]["id"]=="smoke-task"'
test -d .arc/policy/state.json
test ! -e .arc/policy/receipts
"$bin_dir/arc" policy state migrate --repo . --confirm-offline > .arc/policy-retry.json
python3 -c 'import json; p=json.load(open(".arc/policy-retry.json")); assert p["already_migrated"] is True'

