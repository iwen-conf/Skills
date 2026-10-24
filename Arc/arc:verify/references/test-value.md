# Test Value

A test exists to show that business code meets a business expectation. It earns its place only
by failing on a realistic defect in that expectation; passing is not its purpose. Use §1–§2 as a
checklist when writing tests and §3 to review an existing suite (测试价值评估, 无效测试).
Case fields and oracle rules: [case-design.md](case-design.md).

## 1. Three answers per test

| Question | Valid answer |
|---|---|
| Which business rule? | a rule ID, contract section, requirement or bug report |
| Where does the expected value come from? | the contract, a hand-derived literal, an independent model or reconciliation; never the code under test or its current output |
| Which realistic defect makes it fail, at which assertion? | e.g. dedup removed → second charge row exists → `count == 1` fails |

A test without all three is a candidate to strengthen or remove. A business rule without a test
is a gap, reported separately.

## 2. Tests that only mirror the implementation

| Pattern | Why it proves nothing | Replace with |
|---|---|---|
| Expected value computed with the same formula, helper or constant table as the code | shares the bug | literal or independent derivation from the rule |
| Snapshot or golden file re-blessed whenever output changes | asserts "whatever it does now" | contracted fields; a snapshot diff is reviewed as a behavior change |
| Mocked collaborators with call order or count assertions | breaks on refactor, passes when the result is wrong | the outcome: response, persisted state, event, forbidden effect |
| The unit under test mocked, or the database/broker whose semantics the claim needs | proves the mock | real dependency in the test environment, else `BLOCKED` |
| No assertion, `is not None`, status code only | executes code, checks nothing | identity, values, state and absence of forbidden effects |
| Private helpers tested instead of the owning boundary | locks the implementation, misses wiring | test through handler, usecase, consumer or component |
| `toThrow()` / bare `except` without type or message | any failure passes | the contracted error type, code or message |
| Constants or setup echoed back | cannot fail | delete |
| Timeouts, retries or sleeps raised until green | hides the defect | flaky protocol ([release-ops](release-ops.md) §2) |
| The same permutation repeated at every layer | cost without new defects | the cheapest layer that reveals the failure |

## 3. Reviewing an existing suite

1. Inventory each test in scope: test → rule → oracle source → defect caught (§1). Flag tests
   with no rule or an oracle taken from the implementation.
2. Classify: `holds a claim`, `strengthen` (weak oracle), `mirror` (§2), `redundant` (another
   test kills the same defects), `invalid` (cannot fail).
3. Sample sensitivity on flagged and critical tests: a bounded mutation or known-bad input in a
   disposable worktree ([case-design](case-design.md) §6). A test that survives the mutation it
   should catch is `strengthen`; a mutation no test catches is a gap.
4. Report a table: test, rule, classification, evidence (mutation and result, or reason),
   proposal. Merging or deleting tests is a proposal the user approves. Never delete a test
   because it fails, and never remove the only test of a rule.

## 4. Unit or end-to-end

Pick the layer by the failure the test must reveal, not by ratios. A unit test of a pure rule
(price, quota, state transition) stays even when an E2E journey passes through it, as long as it
catches boundary defects the journey cannot reach cheaply; remove it only when sensitivity
evidence shows another test catches the same mutations. E2E proves wiring and the journey, not
input combinations.

## 5. A red test is information

When a test whose expectation comes from the contract fails, the product is wrong: `FAIL` and a
repair packet. Change a test only for a confirmed test defect (wrong oracle, broken setup) and
keep the evidence; never to match current output. A green suite with known untested rules is
reported with those gaps, never as "all verified".
