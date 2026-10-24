# Properties and Model Conformance

Check that code keeps a model's promises over generated inputs and operation sequences. The model
says what must hold; this file turns it into executable cases against the real code. Closure specs
and formal specs belong to `arc:plan`; this mode reads them and never edits them. Hostile bytes into
parsers: [fuzz-campaign](fuzz-campaign.md). Database histories and isolation: [txn-faults](txn-faults.md) §3, db mode.

## 1. Method

Pick the cheapest method that can disprove the claim. Example tests for named critical paths come
first ([case-design](case-design.md) §4); generated cases explore around them and never replace the
deterministic replay of a known incident.

| Claim shape | Method | Evidence |
|---|---|---|
| Pure rule over a broad input space: price, rounding, codec, permission matrix | property (§2–§3) | property, generator, case count, seed |
| Input domain small enough to list (role × status × action) | enumerate every combination; no sampling | full table run |
| Object or service with operations and state | stateful test against a reference model | sequence count × max steps, seed |
| Lifecycle in a closure spec, or a TLA+/Quint/Alloy spec the project keeps | model conformance (§4) | transition hit ledger, divergences |
| In-process concurrent object: queue, cache, pool | parallel commands + linearizability check (§5) | history count, checker result |
| Small critical function needing "every input within bounds": overflow, index bounds, `unsafe` | bounded proof (§6) | harness, bounds, verifier output |

## 2. Properties from the contract

A property holds only when the contract makes it true (case-design §4). Sources: closure-spec
invariants (`INV-xx`), resource acquire/release rules, transitions, flow branches, API contracts,
conservation rules. Never the current output.

| Shape | Assertion | Trap |
|---|---|---|
| Invariant, conservation | after any operation sequence: held + free = total; ledger sum unchanged | checked only at the end when the claim is "never" |
| Forbidden outcome | no sequence reaches negative stock, an exit from a terminal state, a double release | sequences too short to reach the state |
| Round trip | `decode(encode(x)) == x` | equality only after normalization; name the normalization |
| Idempotence | applying a contract-idempotent operation twice equals applying it once, effects included | checking the response, not the side effect |
| Reference model | optimized path equals a simple independent model | model copied from the implementation |
| Metamorphic | a narrower filter returns a subset; reordering order lines keeps the total | relation not stated by the contract |
| Order independence | events in any order the contract allows reach the same end state | assuming order freedom the contract lacks |

"Does not throw", "returns non-null" and an expected value computed by the code under test are not
properties.

## 3. Generators, shrinking, replay

- Valid-input generators build values from project enums, columns and documented bounds; invalid
  inputs get their own generator and the rejection property (documented error, zero durable change).
- Bias toward boundaries, empty and maximum sizes, duplicate and equal keys, hot keys, time-zone and
  precision edges. Report the distribution through the library's statistics and review which
  business states were reached ([case-design](case-design.md) §7), not the iteration count.
- Keep shrinking on; custom generators compose library combinators so they still shrink. Confirm the
  shrunk case fails for the same reason as the original.
- Clock, RNG, IDs and external effects are injected or isolated (case-design §8); state resets per
  case. A property that alternates between pass and fail on one seed is a harness defect until shown
  otherwise.
- Budgets per cadence come from the project, else proposed and recorded: PR runs a fixed seed and
  replays every saved counterexample; nightly uses fresh seeds and larger counts. CI prints the seed.
- Every counterexample stays in the library's failure store and becomes a named regression test.

| Stack | Property | Stateful / model | Replay |
|---|---|---|---|
| Go | `rapid.Check(t, func(t *rapid.T) {…})` | `t.Repeat(map[string]func(*rapid.T){"": checkInvariants, "Cancel": …})` | `testdata/rapid/` fail files, `-rapid.failfile=`, `-rapid.seed=`, `-rapid.checks=` |
| TS/JS | `fc.assert(fc.property(…))` | `fc.commands([...])` + `fc.modelRun` / `fc.asyncModelRun`; `fc.scheduler()` for async interleavings | rerun with the printed `{ seed, path }` |
| Python | Hypothesis `@given` | `RuleBasedStateMachine` with `@rule`, `@precondition`, `@invariant`, `Bundle` | `.hypothesis/` database, `@example`, `--hypothesis-seed`, `--hypothesis-show-statistics` with `event()` |
| Rust | `proptest!` + `prop_assert!` | `proptest-state-machine`: `ReferenceStateMachine` + `StateMachineTest` | commit `proptest-regressions/`; `PROPTEST_CASES` |
| JVM | jqwik `@Property` + `@ForAll` | jqwik `ActionChain` | seed from the report |

Go `testing/quick` is frozen and does not shrink; keep it only where the project already uses it.
Check the installed library version's API before writing; record name and version in the evidence.

## 4. Conformance to a closure spec or formal spec

Closure spec (`docs/01-系统设计/业务闭环/03-closure-spec.yaml`, legacy `docs/arc/`) → test artifacts:

| Spec element | Test artifact |
|---|---|
| `states`, `initial`, `terminal` | model state; after each command the observed state (API or durable store) equals it |
| transition (`from`, `event`, `guard` → `to`, `emits`, `acquires`, `releases`) | command enabled when the model is in `from` and the guard holds; asserts `to`, emitted events, resource delta |
| event with no transition and no `ignores` in a state | illegal command: contracted rejection, zero durable change, zero external effect |
| `ignores` | late or duplicate async event in a listed state leaves state and resources unchanged |
| `race_policy` | parallel pair (§5); winner and loser outcome as the policy states |
| `resources` | never acquired twice, never released unheld, none held at a terminal state |
| `invariants` | checked at every stable point (async edges drained), cited by `INV-xx` |
| `timer` events | driven through the injected clock; never wall time |
| `scenarios` | deterministic example tests; generated sequences explore around them |

1. `check_closure.py gate` on the spec must pass; otherwise the conformance claim is `BLOCKED: model
   gate` and goes to `arc:plan`.
2. Write the model in test code: an interpreter over the spec's transition table or a hand model
   citing transition IDs. It never imports implementation code.
3. The adapter maps each event to the real entry point (usecase, API, consumer) on the real
   dependencies the decision rules require, and reads state through the flow card's observation points.
4. Generate sequences up to K steps (project-given, else twice the longest spec scenario, recorded),
   weighted toward exception, timer and async events, with illegal events mixed in.
5. Ledger: hit count per transition ID, `ignores` entry and illegal (state, event) pair. A required
   entry with zero hits is a generator gap, never `PASS`.
6. Divergence: shrink, then classify. Code breaks the spec → `FAIL` + repair packet for `arc:code`.
   Spec silent, ambiguous or wrong → `SPECIFICATION GAP` to `arc:plan` model review with both
   sources. Adapter or observer defect → fix here. Never edit the spec or bend the model to the code.

Formal specs the project already keeps:

| Spec | Route to tests |
|---|---|
| TLA+ | TLC without counterexample is design evidence only. Trace validation: log implementation events in the txn-faults §3 history format, constrain the spec's next-state by each logged event in a trace spec, run TLC; the log not being a behavior of the spec is a divergence. TLC counterexamples become deterministic sequences |
| Quint | `quint run` with `--invariant`, `--max-steps`, `--max-samples`, `--seed`, `--out-itf` writes ITF traces; replay each step through the adapter and compare state. `--mbt` records the action and nondeterministic picks per step where the installed CLI supports it (`quint run --help`) |
| Alloy | `run` instances become fixtures for structural rules (permission graphs, hierarchies); `check` counterexamples become negative cases |

- Introducing a formal language a project does not use is the user's design decision through
  `arc:plan`, never a verify default.
- Model results and code results are separate verdicts: a passing model check says nothing about
  the code; a passing conformance run covers only the explored sequences under the model's
  abstractions and bounds, which the evidence records.

## 5. Concurrent conformance

- Run a sequential prefix, then 2–N parallel actors issuing commands; record invocation, response
  or timeout per operation; check the history is linearizable against the same sequential model.
  Go: `github.com/anishathalye/porcupine` (`CheckOperationsVerbose` with a timeout; `Unknown` is
  `BLOCKED`). JS async: `fc.scheduler()`. Rust primitives: `loom` ([stacks](stacks.md)).
- Race detector or sanitizers stay on. A clean `-race` run does not prove linearizability, and a
  linearizable history does not prove the absence of data races.
- Operations whose correctness rests on database locks or isolation use txn-faults §3–§4 on the
  real database; an in-memory model run does not cover them.

## 6. Bounded proofs

- Rust with Kani: harness in a `#[cfg(kani)]` module, `#[kani::proof]`, inputs from `kani::any()`,
  preconditions via `kani::assume`, loops bounded by `#[kani::unwind(N)]`; run `cargo kani --harness
  <name>`. Install only with authorization (`cargo install --locked kani-verifier`, `cargo kani
  setup`). Targets: integer arithmetic in money or quota code, index and slice bounds, `unsafe`
  blocks, fixed-size header parsing.
- `VERIFICATION:- SUCCESSFUL` is `PASS` for "harness H, bounds B"; a failed unwinding assertion
  means the bound is too small: `BLOCKED`, never `PASS`.
- Verus, Dafny and other verified-language code change production source and the build: that is
  `arc:code` with user approval. Where the project already uses one, run its verifier command and
  record the result as evidence.

## 7. Verdicts

| Observation | Verdict |
|---|---|
| Counterexample, model divergence, invariant break | `FAIL` with the shrunk case, seed and sequence |
| Spec undefined or ambiguous for the observed case | `SPECIFICATION GAP` |
| Budget spent, no counterexample, ledger hits every required entry | `PASS` for "property P, generator G, N cases × K steps, seed S"; never "correct" |
| Required transition or state never generated | `BLOCKED` until the generator reaches it |
| Model gate fails, or no model for a conformance claim | `BLOCKED` → `arc:plan` |
| Linearizability checker times out (`Unknown`) | `BLOCKED` with history size |
