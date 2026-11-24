# Root-Cause Debugging & Conditional Diff Audit (`Waza Hunt + Audit Baseline`)

Applied by `arc-code` on bugs/regressions and by `arc-audit` on deep/risk-tier diff reviews alongside `contract.md`.

## 1. Root-Cause Debugging & Regression Protocol (`arc-code` Bug/Regression Mode)

- **Testable 1-Sentence Hypothesis First**: Before editing source, state `"Root cause is [X at file:line] because [evidence]."` covering **every** observed symptom (partial coverage is a symptom guess, not a root cause).
- **Hard Stops**: Same symptom recurring after a fix, or **3 failed hypotheses in a row** -> STOP, surface what was checked, ruled out, and unknown, and ask how to proceed. Never "just try this" or restart `> 2` times without new evidence.
- **Probe Strategy by Bug Class**:
  - *Async / lifecycle / event-order / focus / timer*: Instrument with a yes/no runtime probe before writing a fix (remove temporary logs before handoff; gate persistent diagnostics behind debug flags).
  - *Visual / compositor / paint*: Trace stacking contexts and paint layers in DevTools first before adding logs.
  - *Performance / memory*: Measure baseline wall-clock, profile, or RSS first and report before/after numbers ("feels faster" is not evidence).
  - *System / tooling / external failures*: Measure/diagnose the lower layer (OS, runtime, compiler, network, auth) before switching tools or blaming UI.
- **Bisect Mode (`以前是好的`)**: Run `git status --short --branch -uall` first; NEVER `git bisect` inside a dirty worktree (use a detached temporary worktree and remove it after `git bisect reset`). Inspect `git diff <last-good>..HEAD -- <suspect>` before running full bisect.
- **Scope Blast Sibling Sweep (`举一反三`)**: After fixing a root-cause pattern, query the repo (`arc idx search` / `arc idx ast`, excluding generated/vendor dirs) for the same bug shape, classify every match (`same bug` / `safe` / `unsure`), and report `checked N / fixed M / left K`.
- **5-Rung Runtime Evidence Ladder**: (1) source trace `file:line` -> (2) deterministic repro -> (3) runtime logs/DB/cache state -> (4) narrow build/test -> (5) real runtime/visual check (compile-only is NEVER proof for UI, native, or generated-artifact fixes).
- **Top Recurring Failure Patterns**:
  - *Stale verifier / tool cache*: Confirm reported paths exist in the current repo rather than deleted temp worktrees before acting.
  - *Generated / bundle rebuild drift*: Inspect built outputs (`dist`, app archives, generated files), not just source diffs.
  - *Guard lifetime race*: Trace auth/permission guards across cold launch, warm launch, deep link, and relaunch paths.
  - *Non-atomic user file replacement*: Write to a sibling temp file with a signal cleanup trap and atomically rename only when complete — never truncate existing user files in place.
  - *Subprocess pipe backpressure*: Drain stdout/stderr streams while running before `wait()`, and test with output exceeding typical pipe buffers.
  - *CLI dry-run vs executor predicate drift*: Share one predicate function for preview/dry-run/size calculations and actual execution.
  - *Snapshot rebuild dropping carried fields*: Audit rebuild/mapping paths so default-initialized fields are explicitly carried forward and unit-tested.

## 2. Conditional Diff & Architecture Audit Traps (`arc-audit` Deep/Risk Review)

- **Risk Tiers Follow Paths, Not Diff Size**: Changes touching CI workflows, release/signing, auth, billing, installers, privilege, or deletion sinks always require deep audit and explicit revert/verification evidence regardless of line count.
- **Duplicated Derivations**: Flag classification, ordering, threshold, count, or eligibility rules computed independently in two places (summary vs list, preview vs executor); require one shared pure function or constant.
- **Destructive Matcher Breadth**: Flag prefix wildcards, fallback regex, or display-name matching feeding delete/purge sinks; require exact IDs or canonical symlink-resolved paths enforced inside the deletion primitive itself.
- **Test Surface Fidelity**: Flag tests that pin helpers production never reaches, assert literal command strings instead of shipped entrypoints, or build inputs blind to transformed/untagged production shapes.
- **Never-Shipped Migrations**: Flag migration scaffolding or backward-compat carry-forward logic for keys/schemas that have never shipped in a published tag.
- **Dependency & Security PR Hygiene**: Diff full manifest files for bot reserialization artifacts (escaped CJK or reordered keys), verify actual build reachability before repeating scanner severity, and check `git log -S'<pin>'` before bumping a pinned dependency.
- **Deletion & Simplification Audit**: When removing/consolidating code or rules, classify every removed behavior (`folded into X` / `redundant with Y` / `behavior removed`) and search whole-repo callers (including CI, docs, manifests, and dynamic lookups) before claiming dead code.
