# Coding Discipline

Arc rules for every implementation or repair edit; `arc:code` reads this file on every task and
`arc:audit` uses it as a review rubric. Evidence-first root-cause repair stays in
[execution-truth.md](../../docs/execution-truth.md) §6.

## 1. Think before coding

Applies to every implementation or repair request before the first edit.

1. **State assumptions.** Name the assumptions the plan depends on. If multiple readings of the
   request exist, present them — do not silently pick one.
2. **Offer the simpler path.** If a materially simpler approach exists, say so before building the
   requested one. Push back when the request itself looks wrong.
3. **Ask/autonomy boundary.** Proceed end-to-end without asking for bug fixes and for work with one
   clearly correct answer. Stop and ask only when the tradeoff touches interfaces, persisted data,
   or architecture — or when a blocking ambiguity survives reading the available context.
4. **Verifiable goal first.** Convert the task into a checkable success criterion before editing
   ("fix bug" → "write the failing reproducer, then make it pass"; "refactor X" → "tests pass
   before and after"). A strong criterion lets the agent loop autonomously; "make it work" does not.

## 2. Simplicity ladder

Solve the problem with the least code. Run the ladder only **after** understanding the problem —
read the code the change touches and trace the real flow first; the ladder does not replace
understanding. Stop at the first rung that holds:

1. Does this need to exist at all? If not, skip it (YAGNI).
2. Does the codebase already have it? Reuse the existing helper/util/pattern — do not rewrite.
3. Can the standard library do it? Use it.
4. Can a platform-native feature cover it? Prefer it (`<input type="date">` over a date-picker
   library, CSS over JS, DB constraints over app-layer checks).
5. Can an already-installed dependency solve it? Use it. Never add a new dependency for something
   a few lines of code can do.
6. Can one line do it? Write one line.
7. Only then write the minimal new code.

Boundaries:

1. Never simplify away: input validation at trust boundaries, error handling that prevents data
   loss, security measures, accessibility basics, or anything the user explicitly asked for.
2. A deliberate simplification with a known ceiling (global lock, O(n²) scan, naive heuristic)
   gets a `debt:` comment naming the ceiling and the upgrade path
   (e.g. `# debt: 全局锁，吞吐量成瓶颈时改每账户锁`).
3. Required DIP boundary interfaces ([go-backend.md](conventions/go-backend.md)) are not
   "unrequested abstraction"; keep the boundary and shrink everything around it.
4. Self-check: would a senior engineer call this over-engineered? If yes, simplify. If 200 lines
   could be 50, rewrite before delivering.

## 3. Surgical edits

1. Do not "improve" adjacent code, comments, or formatting outside the request. Match the existing
   style even when you prefer another.
2. Do not refactor what is not broken unless the refactor is the task.
3. Delete the imports/variables/functions that **your change** orphaned, in the same change.
4. Do not delete pre-existing dead code; mention it instead (cleanup becomes its own task,
   see [code-rot.md](code-rot.md) B14).
5. Traceability test: every changed line maps to the user's request or to an orphan the change
   created.
6. Bounded elegance pass: for a non-trivial change, before delivery ask "knowing everything I know
   now, is there a more elegant way?" If the fix feels hacky, rewrite — but only the code this
   change introduced, never expanding into pre-existing code. Skip the pass for simple, obvious
   fixes; do not over-design.

## 4. Architecture paper-overs

For every backend implementation or repair:

1. No temporary package-level `var` flags to bypass product rules without an explicit task and review.
2. No type assertions to grab optional infrastructure capabilities that should be injected contracts.
3. No silent hardcoding of epochs, auth schemes, or magic IDs "for now".
4. No domain/application workflow dumped into `domain/services` when it belongs in `usecase`.
5. No paper-over: changing tests only, swallowing errors, or documenting aspirational behavior as shipped.
