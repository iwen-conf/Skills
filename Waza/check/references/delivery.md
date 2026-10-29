# Delivery Evidence and Actions

Adapted from Waza's release and public-reply workflows; [MIT license](../../LICENSE).
Use only the sections matching the authorized action. Derive the hosting CLI/API from the project;
GitHub examples are not instructions to move another host's workflow to GitHub.

## Establish the candidate

Record the review base, current `HEAD`, owned dirty changes, target branch/account, distribution
lane and applicable project gates. For a release, use the latest published stable tag as the
baseline unless the project specifies a different one. A local version field is not a release
record. A preview artifact cannot prove stable readiness.

Read only the evidence needed for the requested outcome:

| Surface | Evidence |
|---|---|
| Source | Review base and candidate SHA; staged, unstaged and untracked changes accounted for |
| Remote | Intended remote, authenticated account, target ref and current remote SHA |
| Versions | Relevant manifests, lockfiles, changelog, update feed and artifact metadata agree |
| Gates | Actual commands and results for this candidate; required CI jobs identified |
| Distribution | Named lane, built package contents, checksums and generated outputs |
| Installed runtime | An isolated install/run observation when the installable surface changed |
| Published state | Registry, release asset, update feed or deployment readback |
| Public thread | Correct issue/PR, author, comment and closure state |

Mark inapplicable surfaces with a reason. Unknown evidence is a gap, never an implicit pass.
Use the existing Arc evidence and gate semantics; do not replace them with an overall score.

## Commit and push

- Check the intended diff and any existing staged changes before staging. Do not commit another
  contributor's staged work as part of this task. Stage owned hunks or use an isolated branch when
  files mix ownership; never reset their index to make the task easier.
- Serialize index and git write operations. Record `HEAD` before staging, then re-read status and
  `HEAD` immediately before commit and before push. Unexpected commits or changes require
  reconciliation; do not automatically rebase, force-push or include them.
- Before the first remote write, verify the remote, branch and authenticated service account.
  An explicitly named account must match. Read remote refs with the host API or `git ls-remote`
  as appropriate; a stale local remote-tracking ref alone is insufficient.
- Commit only when requested, using the project's commit convention. If intended work is already
  committed and synchronized, prove that state instead of creating an empty commit.
- After push, read the target remote SHA and required CI status for that SHA. Poll structured
  state rather than piping a watch command through output filters that hide its exit status.

### Under the arc-policy runtime

Order, each in its own tool call: `git -C <repo> add <owned files>` → `arc-policy task verify`
→ `git -C <repo> commit` → `git -C <repo> push`. Never `cd <repo> && git …`; the hook refuses
it. Pushing the commit the receipt covers needs no second verify. Verify `NOT_RUN` for missing
test commands is fixed by declaring the project's real commands, a real failure is reported,
and only the user's explicit go-ahead ("直接提交", "不 verify 了") becomes
`arc-policy task authorize --quote '<their words>'` (arc-policy skill, "When verify cannot pass").
The agent runs `git add` and `git commit` itself and never hands them to the user; the user may
push the resulting commit themselves. A "现在提交并推送，不等 X" skips the named wait, not the
receipt.

## Branches and pull requests (any host)

Host CLI follows the remote: `gh` for GitHub, `cnb` for CNB (read [cnb-runtime.md](../../../docs/cnb-runtime.md)
before any CNB call; full CNB actions live in the `cnb` skill), the project's own tool elsewhere. Confirm
every flag with the CLI's current `--help`; do not invent body-file or format options.

1. **Base branch.** Resolve `base_ref` in this order: the user's explicit target → the existing PR's real
   base → the remote default branch (`CNB_DEFAULT_BRANCH` on CNB). Validate it with
   `git check-ref-format --branch "$base_ref"` and
   `git rev-parse --verify "refs/remotes/origin/$base_ref^{commit}"` after a targeted fetch; an empty or
   invalid base stops every branch and PR write.
2. **Existing PR first.** Query PRs by repository, head and base with the host's structured API. Found →
   push to its real head branch and return its number or URL; never open a duplicate. The query fails,
   paginates incompletely or returns unknown output → report the push result and PR state unknown; do not
   create one.
3. **New branch.** No PR and the current branch is the default branch → create `work_branch`
   (`<short-task-keywords>-<4 hex>`) from the validated base, checking each step before the next; never
   commit task changes on the default branch. A checkout conflict with existing work → use a separate
   worktree; never stash, reset or overwrite it.

   ```bash
   git check-ref-format --branch "$base_ref"
   git fetch origin "refs/heads/$base_ref:refs/remotes/origin/$base_ref"
   git rev-parse --verify "refs/remotes/origin/$base_ref^{commit}"
   git switch -c "$work_branch" "refs/remotes/origin/$base_ref"
   ```
4. **Create the PR** only after a complete query confirmed none exists: title `<type>: <summary>`, body
   with purpose and main changes passed through a body file or stdin with real newlines; add the issue
   reference only when an issue triggered the work. Record the returned PR number; creation without one
   is `blocked`.
5. **No history rewrite** (`--force`, `--force-with-lease`) without explicit authorization for that
   branch; with authorization, read the remote state first and prefer lease protection.

## Branch consolidation (合并分支、删除多余分支)

1. Inventory every repository the request names (each platform's repository is separate):
   `git for-each-ref --format='%(refname:short) %(authoremail) %(committerdate:short) %(upstream:track)' refs/heads refs/remotes`
   plus `git worktree list` and `git stash list`.
2. Scope is the user's own branches: branches whose unique commits (`git log <base>..<branch>`)
   are all authored by the configured `user.email`/`user.name`, unless the user names others.
   Another person's branch is listed, never merged, reset or deleted. The default branch
   (main/master) is not changed unless the request says so.
3. Write the plan as a table before the first write: branch → keep / merge into `<target>` /
   delete, with its unique commit count. A content conflict where the user said "按新的来" keeps
   the target's content while recording history (`merge -s ours` or a resolved merge); any other
   conflict is the user's decision.
4. Merge with history preserved; new branch names follow the user's language and naming rule
   (Chinese names when asked). Delete only after `git merge-base --is-ancestor <branch> <target>`
   proves it merged, then remote and local refs and their worktrees. Remote deletion is a push:
   it needs the receipt or the user's authorization like any other.
5. Report the final branch list per repository and every branch left untouched with its reason.

## Packages handed to a device, tester or file server

- Build from the track and branch matching the named environment (行内测试, 生产, 实验室) per
  [execution-truth](../../../docs/execution-truth.md) §1; the project's `AGENTS.md` names the build host for each target (for
  example a Windows build machine, a remote Linux host, or the local Mac only where a native
  macOS app requires it).
- Report the artifact path, version/build number, environment and server addresses baked in,
  SHA-256 and signer, and which fixes it contains. A debug-only switch (crash dialog, skipped
  zero-trust login) is checked against the target environment before handing over.
- Install or upload only to the named target, then read it back: installed version on the
  device, or the download URL and checksum on the file server. Building is not installing;
  installing is not testing on the device, and the report says which ones happened.

## Publish or release

1. Read project release commands and the previous published release's title/language conventions.
   Draft notes from distinct user-visible outcomes actually present in the candidate artifact.
2. Apply the project's required release gates and review unresolved findings. New consequential
   findings are not covered by earlier assumptions; fix within scope or obtain the applicable risk
   decision before the dependent release. Preserve original finding and gate verdicts.
3. Check generated artifacts and required assets, including ignored distribution outputs. Build
   using the real package rule and inspect contents; use a supported dry-run where useful.
   The [installed runtime procedure](installed-runtime.md) applies to installable changes.
4. Publish only to the authorized account and lane. Separate automatically deployed sites or
   services from binaries that reach users only with a release; report each surface independently.
5. Read back the registry, release assets or deployment. Prefer asset downloads, digest comparison,
   archive entry inspection or metadata queries over release-page text, file sizes or a green job.
6. If a publish/create operation times out, its outcome is unknown. Inspect the end state before
   retrying; delayed registry visibility is not proof that an upload failed. Bound read-only polls
   by the provider's documented behavior and the task budget. If unresolved, report the exact
   operation and evidence gap without repeating the mutation.

Cancelled or repeatedly reworked candidates need review from the last stable release across the
affected shipped surfaces. Distinguish whether a preview may continue testing from whether stable
release preparation can proceed. Do not infer either from the last small patch alone.

## Issue, PR and release follow-through

- Posting, editing, closing, merging and reactions each require authorization applicable to the
  current task. A local guideline describing usual follow-through is not authorization to message
  someone. Preserve an already authorized chain without introducing another confirmation round.
- Before an authorized write, re-read the live issue/PR and author through the host API or CLI.
  Compose the reply in its language, stating the exact availability state and a useful next step.
  Use the available writing skill for prose when needed; never include private logs or credentials.
- Edit an owned comment in place only before anyone has replied after it. Otherwise add a new
  reply so the conversation's history remains coherent. Do not delete/repost for cosmetic cleanup.
- Read back the comment body, author, target and thread state after a write. Close only when
  closure is authorized and its premise is true: shipped fix, invalid/duplicate report, or the
  maintainer's explicit closure request. A fix awaiting release normally leaves the issue open.
- Add only the reactions the user asked for, after the relevant release and assets exist, and
  verify them by readback. Never add an unsolicited set of positive reactions.

Finish by reconciling each requested action with current local and remote evidence. Authentication,
OTP, network or provider failures block the affected action; do not claim its desired end state.
