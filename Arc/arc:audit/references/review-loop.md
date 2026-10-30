# Re-review and Independent Verification

Extends [diff-review.md](diff-review.md). §1 applies when prior findings exist for the same
change; §2 applies at `deep` depth.

## 1. Re-review must converge

Prior findings come from an earlier report (`.arc/artifacts/<slug>/reports/findings.md`), PR
threads the user supplied, or `.lark.json` records. Each report records `Round: N` and
`Reviewed head: <sha>` so the next round can start from them.

- Round 1 reviews the whole diff. From round 2, review only the increment
  `git diff <reviewed-head>..HEAD` plus §4 impact rows for symbols the increment changed.
- Carry every prior finding forward under its ID: `fixed` (cite the increment hunk), `open`,
  `resolved-with-reason` (the author's reason, kept unless new evidence contradicts it) or
  `disputed`. Never file a prior finding again under a new ID.
- A defect found in code the earlier rounds already read is `pre-existing` unless the increment
  made it reachable or worse.
- Each card names its defect class and lists every location of that class in the reviewed
  scope, so one fix round closes the class instead of one line.
- IF round ≥ 3 and a blocking finding is still `open` THEN stop line-level review. Report
  `FAIL: re-evaluate approach` with the recurring classes and recommend a design change (an
  `arc:plan` brief) or a human decision. Start round 4 only on the user's explicit request.

## 2. Independent verification (deep)

- Give each `confirmed` or `likely` blocking finding to a fresh-context verifier (a subagent
  without the reviewer's notes) with only the claim, its `file:line`, the stated intent and the
  diff. Ask it to disprove the claim: unreachable, guarded upstream, intended per intent or
  `REVIEW.md`, or pre-existing.
- Verifier disproves it → `false-positive` with that evidence. Disagrees without a disproof →
  `needs-triage`. Fails to disprove → status kept, recorded as `verified: independent`.
- Without subagent support, repeat the disproving check from the entry point instead of the
  hunk and record `verified: self`.
- Deep output drops style, diagnostic and cleanup-only items; they stay advisory elsewhere.
