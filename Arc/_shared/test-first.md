# Test First (red before code)

A test states the business expectation before the code exists; watching it fail proves it can
catch the missing behavior. `arc:code` follows this in implement and repair modes; `arc:verify`
red mode writes acceptance tests for `arc:code` to turn green (§5).

## 1. Iron law

No production code for a behavior change until a test for that behavior has run against the
current code and failed on the assertion that names the missing behavior.

- Code written before its red is discarded (own edits only; user changes in the baseline are
  never touched) and rewritten from the test. Do not keep it "as a reference": the test will be
  bent to fit it.
- Applies to every feature, fix and behavior change, however small.
- Exceptions need the user's explicit approval, stated in the report: a throwaway spike (deleted
  before test-first work starts), generated code (test the generator or the generated contract),
  configuration or docs only, comment mode, a pure refactor (characterization tests green before
  and after instead, [refactor.md](../arc:code/references/refactor.md)).

## 2. Cycle

| Step | Action | Evidence |
|---|---|---|
| Expectation | Write one behavior's expected result from the requirement, contract or bug report: input, observable outcome, forbidden effect. Never run the code to learn what to expect. | claim and source |
| Red | The smallest test asserting it through the owning boundary; run `gate.sh red -- <focused test command>`. | `red/red-NNN.json`, kind `assertion` |
| Check the red | The message names the missing behavior (wrong value, missing state, absent event). A build or import error is not a red: add the minimal signature stub (zero value, no logic), declare it with `--allow <path>`, run red again. A passing command means the behavior exists or the test asserts the wrong thing: fix the test. | red log |
| Green | The minimal code that passes this test: no extra options, no speculative generality. | passing run |
| Refactor | Clean up with the test green; re-run after each step. | passing run |
| Next | One behavior per cycle; the next one starts with its own expectation and red. | — |

`gate.sh check --require-red` re-runs every recorded red command (each must pass now) and warns
about a stub larger than a signature or a test edited after its red. Prefer a focused command
(one test or file) so the red names one behavior.

## 3. Writing the red

- Assert the business outcome at the owning boundary (handler, usecase, consumer, component),
  not private helpers, internal call order or mock interactions.
- Expected values are literals or independent derivations from the requirement (hand-computed
  amounts, reference tables), never the implementation's formula or its current output.
- Real collaborators by default; a double only for a boundary outside the claim (clock, third
  party). Mocking the unit under test makes the test invalid.
- One behavior per test, named for it (`cancel releases the hold exactly once`).
- A bug's reproducer from [repair.md](../arc:code/references/repair.md) §2 is its red.

## 4. Rationalizations

Each of these means: stop and write the red.

| Thought | Reality |
|---|---|
| "Too simple to need a test" | Simple code breaks; the test costs a minute. |
| "I'll add tests after" | A test written after the code passes at once, proves nothing about catching the defect and tends to restate the implementation. |
| "I already checked it by hand" | A manual check is neither recorded nor re-runnable. |
| "Hours of work would be thrown away" | Unverified code is the cost; rewriting from a test is cheaper than debugging later. |
| "I need to explore first" | Explore in a spike, discard it, then start with the red. |
| "The test is hard to write" | Hard to test means hard to use; add a seam ([case-design](../arc:verify/references/case-design.md) §8) or ask. |
| "This case is different" | It is not. |

## 5. Acceptance first (arc:verify red mode)

When claims exist before the implementation (requirement, closure model, bug report),
`arc:verify` writes acceptance tests at the real entry point from the claims table, runs them on
the current code and keeps each failure on its targeted assertion. It hands `arc:code` the tests,
the claims table and the red logs. `arc:code` re-runs them through `gate.sh red` in its own task,
adds unit-level reds for internal rules, and implements. `arc:verify` then re-runs the same tests
unchanged on the candidate; red-before plus green-after on the same test is its sensitivity record.
