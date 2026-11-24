---
name: check
version: 2.1.0
description: Execute gate-verified Git, PR, package, release, and CNB (`cnb.cool` / `.cnb.yml` / TAPD) delivery actions. Triggers on 提交, 推送, 开PR, 发版, 合并分支, 打包出包, CNB, cnb.cool, .cnb.yml, CNB流水线, TAPD关联, 值不值得发版.
---

# check

Carry verified changes to the requested Git or CNB delivery state without touching unowned files.

## Protocol

1. **Worktree Safety & Multi-Agent `HEAD` Guard**: Start with `git status --short --branch -uall` and confirm `gate.sh check` / `arc-verify` PASS evidence matches `HEAD`. Treat dirty/untracked files as user work — ban unapproved `git switch`, `git checkout`, `git reset --hard`, `git clean`, `git stash -u/-a`, `gh pr checkout`, or moving untracked WIP into `/tmp` (use a detached temp worktree if a clean checkout is needed). Record `git rev-parse HEAD` before staging and re-verify `git status` + `HEAD` before commit and before push; stop on unexpected drift.
2. **Identity, At-Most-Once & Layered Delivery Proof**: Stage only task-owned paths and forbid AI `Co-Authored-By:` attribution. Before first remote write, verify `git remote -v`, branch, and authenticated identity (stop on mismatch). Treat network/API timeouts on at-most-once ops (publish/tag/release) as `UNKNOWN` — re-read state instead of blind re-publishing. Never collapse source, CI, package, release asset, auto-deployed channel, and public-thread state into one "done" claim; close multi-step deliveries with a numbered completion ledger (`done / not applicable / remaining`).
3. **CNB Platform Work**: Whenever the repo or request uses CNB (`CNB_*` env, `.cnb.yml`, PR `!iid`, TAPD), load `../_shared/cnb.md` for safe CLI calls, right-side inline review comments, and pipeline syntax.
4. **Release Worthiness Mode (`值不值得发版`)**: Classify commits since the last published tag (`N feat / N fix / N chore`), state a `release` or `skip` verdict, recommended semver bump, and top risk. Read back live remote/artifact state after every authorized action.
