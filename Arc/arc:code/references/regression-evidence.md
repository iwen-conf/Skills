# Regression and Runtime Evidence

Additional probes adapted from Waza's Hunt workflow; [MIT license](../../../Waza/LICENSE).
Use the sections matching the observed failure. The owning procedure remains
[repair.md](repair.md); hypotheses, scope, evidence verdicts and investigation limits stay there.

## Earlier work

Other agents (Grok, Codex, another Claude session) edit the same repositories. A recent commit
or dirty change that reverted or replaced this task's earlier work is reported with its diff
before editing over it.

When the user says the problem was already fixed or the feature already built (昨天修过、之前做过、
按照昨天那个、某个 agent 改过), find that change before re-deriving it:

1. `git log --all --since=<date> --format='%h %an %ci %d %s'` in every repository of the product
   (client, server, other platforms), plus `git stash list`, unmerged branches, `git worktree list`
   and the task progress table.
2. Diff the found change against the current branch: is it merged, partly merged, reverted by a
   later commit, or only in a stash or another worktree?
3. Reuse it as the baseline; take only its owned parts when a stash or branch mixes unrelated work,
   and name what was left out. Report where it was found. Nothing found → say so with the queries run.

## Known-good history

- Record the reported last-good commit/version and a reproducible pass/fail command. For a small
  release delta, inspect `git diff <good>..HEAD -- <discovered-path>` before starting a bisect.
- Never bisect a shared dirty checkout. Use an isolated detached worktree, keeping modified,
  staged and untracked user files in place. If a safe isolated environment is unavailable, report
  the gap instead of stashing or discarding user work.
- A bisect requires a noninteractive check that fails for the product symptom. Record good/bad
  classifications in git, inspect the culprit diff, reset the bisect, and remove only the temporary
  worktree owned by this investigation. A missing build dependency is not a bad product revision.

## Reference oracle

List all reported symptoms, including intermittent or seemingly unrelated ones. Identify the
last-good build, screenshot, fixture or documented expected state and the exact current delta.
Define the observable check before editing. A rendering regression is not merely a taste request;
compile success does not prove the affected view or artifact is correct.

When the reporter's environment cannot be reproduced, prepare a bounded, read-only diagnostic
probe that captures the disputed measurement and relevant state without secrets. Discover the
reporter's install method, runtime and supported surface instead of assuming the investigator's
layout. A simulator without the required capability cannot validate that feature. A probe that
misses the failing path leaves an evidence gap; it does not refute the user's observation.

## Choose a probe that distinguishes layers

| Observed shape | Probe and invariant |
|---|---|
| Native freeze or first-open lag | Capture the blocked/spinning thread and runtime timing; inspect first-frame work, synchronous metadata/path scans, callbacks and overlay teardown |
| Lifecycle or async ordering | Log correlation IDs, event timestamps, state and object lifetime while forming the hypothesis; each observation must discriminate competing explanations |
| Source works, installed command fails | Compare the actual wrapper/PATH target, package contents, executable bit and generated artifact against the candidate; reproduce through the installed entrypoint |
| Dry-run differs from mutation | Trace the preview selector and executor input to their owning rule; assert selected, completed and skipped items across partial failure |
| Child process hangs on large output | Observe stdout/stderr draining and wait order; reproduce with output beyond the pipe buffer rather than adding a larger timeout |
| Parsed metrics differ by locale or user | Inspect raw bytes, locale, permissions, units and first-sample semantics; distinguish unavailable data from zero |
| Empty process/file inventory drives cleanup | Distinguish timeout, denied reads and decode failure from absence; corroborate absence before any destructive decision |
| Repair never restores missing state | Check whether repair itself requires the missing artifact; verify tool paths and the resulting system state instead of command-text shape |
| Healthy slow operation trips watchdog | Compare the bound with measured healthy cold/slow cases and actual progress; account for every exit/fork that should cancel the watchdog |

For rendering/compositor faults inspect layers and the real output; generic logs alone cannot
show paint order. For timing faults, if logging changes the symptom, retain that fact as evidence
of scheduling or lifetime sensitivity. Temporary instrumentation follows repair.md's scope and
cleanup rules.

Search analogous paths with `arc-idx` using the actual symbol, API call or structure. Keep the
existing sibling list and independent before/after oracle; do not create a second repair ledger.
