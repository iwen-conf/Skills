---
name: check
description: >-
  Complete authorized delivery actions on any git host (GitHub, CNB, others): commit, push, open or update a
  pull request, tag, publish a release, consolidate and delete branches, build and hand over packages
  (apk, exe, app, binaries) to a device or file server, and follow through on issues and PRs once the
  change is done. Use for 提交, 推送, 开PR, 发布, 发版, 合并分支, 删除多余分支, 打包构建, 出包, 安装到手机,
  上传到文件服务器, 关闭 issue and release follow-through. Not while code still needs changing (use
  arc:code first), for code review (use arc:audit), or for release evidence and GO/NO-GO (use arc:verify).
---

# Check: Delivery Follow-through

Carry an authorized change from its current state to the requested delivery state. A commit,
package upload, deployed service and closed issue are separate outcomes; verify each requested one.

Adapted from [tw93/Waza](https://github.com/tw93/Waza), under the [MIT license](../LICENSE).

## Scope and routing

| Requested result | Owner |
|---|---|
| Commit, push, branch and PR on any host, tag, publish, release, authorized issue/PR follow-through | This skill |
| Branch consolidation, package builds handed to a device, tester or file server | This skill |
| Other CNB platform actions (TAPD, NPC, attachments, review comments) | `cnb` |
| Review a diff, assess release readiness or triage code risks without changes | `arc:audit` |
| Implement a plan or fix a confirmed defect | `arc:code` |
| Run tests, collect release evidence or decide GO/NO-GO | `arc:verify` |
| Decide what to build or write a plan | `arc:plan` |

Use [Arc handoff](../../Arc/_shared/workflow.md) to retain scope, findings, candidate identity and
existing authorization across phases. A bundled review/fix/verify/push request continues through
its authorized stages. Do not ask the user to repeat authorization when the action, target and
scope remain the same. A report-only request remains report-only; approval of prose or a design
does not authorize posting or publishing it.

## Workflow

1. Record the requested actions and their targets in the current task's completion ledger; do not
   create another tracker. Separate source, package, deployed channel and public-thread outcomes.
2. Read `git status --short --branch -uall` and `git rev-parse HEAD`. Treat staged, modified and
   untracked files as existing work. Stage only owned changes; do not stash, clean, move or discard
   others' work. Use an isolated worktree when a build requires a clean checkout.
3. Discover project release instructions, manifests, CI and distribution metadata with
   [code-intel](../../Arc/_shared/code-intel.md). Follow the user's repository search rules;
   this skill supplies no filesystem scanner or test-discovery script. Read the actual CI jobs
   and package rules before assuming a documented check runs.
4. Load [delivery.md](references/delivery.md) for the authorized actions. For a packaged CLI,
   skill, plugin, archive or generated distribution surface, also load
   [installed-runtime.md](references/installed-runtime.md).
5. Reuse applicable current review and verification evidence. Missing code review goes to
   `arc:audit`; missing runtime or release evidence goes to `arc:verify`. A required failed or
   unknown gate blocks the dependent action; resolve independent authorized work meanwhile.
6. Execute only the requested actions, then read back their actual state. Any change to the
   candidate, scope or required artifact invalidates affected earlier evidence.

## Completion

Lead with the actual delivery state and its SHA, tag, URL or blocker. For each authorized action,
report `done`, `not applicable` or `blocked`, with the command or remote readback that supports it.
Include current worktree state when files changed and the CI result for the pushed SHA when
required. A local commit does not satisfy a push request; a successful workflow does not prove
that the requested package contents reached users.

Do not infer permission to post comments, close issues, add reactions, merge, deploy or publish
from an earlier review or from a project convention. Carry explicit authorization already given
in this task; ask only about an uncovered action, a changed target or an unresolved decision.
