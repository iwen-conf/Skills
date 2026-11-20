---
name: check
description: Execute gate-verified Git, PR, package, release, and CNB (`cnb.cool` / `.cnb.yml` / TAPD) delivery actions. Triggers on 提交, 推送, 开PR, 发版, 合并分支, 打包出包, CNB, cnb.cool, .cnb.yml, CNB流水线, TAPD关联.
---

# check

Carry verified changes to the requested Git or CNB delivery state without touching unowned files.

## Protocol

1. Inspect `git status --short --branch -uall` and confirm current `gate.sh check` / `arc-verify` PASS evidence matches `HEAD`. Block delivery if required gates failed or were invalidated by later edits.
2. **CNB Platform Work**: Whenever the repo or request uses CNB (`CNB_*` env, `.cnb.yml`, PR `!iid`, TAPD), load `../_shared/cnb.md` for safe CLI calls, right-side inline review comments, and pipeline syntax.
3. Stage only task-owned paths; never `git reset --hard` or `git clean` unrelated work. Read back live remote/artifact state after every authorized action.
