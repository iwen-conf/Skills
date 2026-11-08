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

"$bin_dir/arc" db init --repo . > init.json
python3 -c 'import json; assert json.load(open("init.json"))["active"] is True'
"$bin_dir/arc" task create --title "smoke test" > task.json
task_id="$(python3 -c 'import json; print(json.load(open("task.json"))["id"])')"
"$bin_dir/arc" task transition --id "$task_id" --to ready --expected-version 1 > ready.json
echo "new code" > feature.go
"$bin_dir/arc" file update --path feature.go --task "$task_id" > file.json
"$bin_dir/arc" file show --path feature.go > file-show.json
python3 -c 'import json; assert json.load(open("file-show.json"))["version"]==1'

# Policy task ID import must be idempotent, and a change must retain its owner.
"$bin_dir/arc" task ensure --id policy-smoke-01 --title "policy smoke (arc-code)" > policy-task.json
"$bin_dir/arc" task ensure --id policy-smoke-01 --title "policy smoke (arc-code)" > policy-duplicate.json
python3 -c 'import json; assert json.load(open("policy-duplicate.json"))["duplicate"] is True'
"$bin_dir/arc" update --changed-only --task policy-smoke-01 > update.json
python3 -c 'import json; assert json.load(open("update.json"))["changed_files"]>0'
"$bin_dir/arc" event list --resource task --id policy-smoke-01 --limit 10 > events.json
python3 -c 'import json; a=json.load(open("events.json")); assert a["count"]==1 and a["events"][0]["kind"]=="task.imported"'
"$bin_dir/arc" db status > status.json
python3 -c 'import json; a=json.load(open("status.json")); assert a["active"] and a["pending_events"]>0'

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
"$bin_dir/arc" db migrate --confirm-offline > migrated.json
python3 -c 'import json; x=json.load(open("migrated.json")); assert x["active"] and x["migrated_graphs"]==1'
"$bin_dir/arc" db status > status.json
python3 -c 'import json; x=json.load(open("status.json")); assert x["graph_count"]==1'
