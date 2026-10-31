# Business Flow Model

Flows come before cases: a suite is designed from the business flows a change reaches, never from
an endpoint, function or file list. A flow is one business goal from its trigger to every
business-meaningful end. One flow may span several endpoints, jobs and services; one endpoint may
fork into several flows by input, state or role. Matrix, ledger and change impact:
[flow-matrix.md](flow-matrix.md).

## 1. Catalog

| Item | Rule |
|---|---|
| Location | `docs/07-测试质量/业务流/`; an existing project test-plan location wins. Read-only scope: `.arc/artifacts/<task-slug>/flows/` |
| `README.md` | the inventory (§2); a modeled flow's row links its file; per-flow coverage and residual risk |
| `FLOW-NNN-<name>.md` | card (§5), chart (§3), terminal and branch tables ([flow-matrix](flow-matrix.md) §5) |
| Freshness | the card records the source revision and every node's `file:line`; a diff touching a listed file makes the flow `stale` until re-traced |
| Check | `python3 <skill-dir>/scripts/flow_ledger.py check <catalog>` exits 0 before any verdict is reported; on failure fix the chart or rows from the code trace (≤2 rounds), else report the remaining errors and no verdicts. Evidence: catalog paths, exit code and branch counts |

IDs are stable: `FLOW-NNN`; decisions `Nxx`; terminals `Txx`; branches `FLOW-NNN-Nxx-Bxx`. Never
renumber; retire an ID with a note.

## 2. Discover from the implementation

This is the implementation inventory. First confirm expected business functionality from user
intent and requirements; keep its catalog separate ([business-graph](business-graph.md)). A code
trace explains what exists, never what is correct. Preserve expected-but-unfound features as
candidate gaps, and unconfirmed semantics as RISK or SPECIFICATION GAP.

Docs, READMEs and comments are leads; the executed code path is the fact. A difference between
them is a candidate finding, never a silent choice.

1. **Entry points.** Query with code-intel: routes and handlers, RPC, CLI commands, consumers and
   subscribers, schedulers and timers, webhooks and callbacks, operator tools, migrations and
   backfills; record each `file:line`. Example: `arc-idx search 'Router|HandleFunc|Subscribe|cron'
   --format ai-md`, then `arc-idx symbol <handler> --format ai-md`.
2. **Trace each entry to its business ends**: validation, authentication and authorization, rule
   checks, reads, durable writes (table, file, key), transaction begin and commit, events or outbox,
   external calls, cache, lock or idempotency key, error mapping, retry owner, and what each failure
   leaves behind. One line per node: `file:line · reads · writes · emits · on failure`.
3. **Follow async edges**: the consumer of each emitted event or scheduled job is its own entry;
   link the two flows.
4. **Group by business goal and type each flow**: `main` (core goal), `auxiliary` (query, review,
   notify, reconcile), `exception` (reject, fail, timeout, conflict, cancel), `recovery` (retry,
   undo, replay, manual repair), `cross-flow` (flows coupled through events, state or shared data).
   Exception and recovery paths that share the parent's trigger and state stay branches of the
   parent chart; they become their own flow when they have their own trigger (timer, operator,
   consumer), linked to the parent branch ID.
5. **Inventory row** per flow: ID · name · type · goal · trigger and actor · entry `file:line` ·
   modules and external dependencies · normal and failure terminals · chart (file link or
   `not modeled`) · matrix status · risk · open questions. A flow found but not yet traced keeps its
   row as `not modeled`; never drop rows to look complete.

Never invent a component the code lacks (queue, saga, retry, tenant). A search that finds nothing is
written `not found (queries …)`.

## 3. Chart rules

One or more `flowchart TD` blocks per flow file; node IDs are unique per file. The checker accepts
this subset:

| Element | Syntax | Rule |
|---|---|---|
| Start | `S([start: trigger, actor])` | exactly one |
| Step | `P1[tx: write order HELD; hold stock]` | names the read, durable write or side effect; `[[FLOW-NNN …]]` links a sub-flow |
| Decision | `N01{stock covers the quantity?}` | a business question with ≥2 branches |
| Branch | `N01 -->|B01 available ≥ quantity| P1` | the label names the business condition; never a bare yes/no or success/failure |
| Terminal | `T01([normal: order HELD, stock held])` | no outgoing edge; its kind is in the terminal table |

- Every node is reachable from `S` and reaches a terminal. Edges use `-->`, `-.->` or `==>` with an
  optional `|label|`; no arrows inside node text.
- The branches of one decision are mutually exclusive and cover every case the code handles; an
  input the code does not handle gets its own branch to a `gap` terminal.
- Draw only mechanisms that exist. A failure path without implemented rollback, retry,
  compensation or manual repair ends in a `gap` terminal named for the missing handling; an
  ambiguous rule ends in `gap` with its question. Never chart the desired design as current behavior.
- Async: the producer path ends at an `async` terminal ("accepted; pending <event>") and links the
  consumer flow; an eventual-consistency end names its completion condition and the deadline's source.
- Mark transaction boundaries in step text (`tx begin`, `tx commit`) where they decide what a
  failure leaves behind.

## 4. Closure kinds

A closed flow is not one where every error becomes success: every path ends in a defined,
observable, explainable state. Terminal and branch `闭环` values:

| Kind | The chart shows | Tests assert |
|---|---|---|
| `正常` normal | legal trigger → rules pass → writes commit → downstream done or a named pending state | final state and data are queryable; exactly the contracted effects; nothing duplicated or missing |
| `拒绝` reject | parameter, permission or rule refusal | the contracted response; zero durable change and zero external effect (pre/post diff); a traceable reason |
| `失败` failure | an error mid-flow, then the implemented rollback, state mark or retry | no undefined state; the failure is observable and queryable; the retry or re-submit rule holds |
| `恢复` recovery | retry, compensation, undo, replay, manual repair | no duplicate effect; original failure and recovery both traceable; the contracted final state |
| `异步` async | an event or job handoff | trigger source; duplicate, reordered and late delivery; retry, dead-letter or manual path as implemented; completion condition |
| `缺口` gap | missing handling or an undecided rule | nothing passes; a gap item with an owner ([flow-matrix](flow-matrix.md) §8) |

Add cancel, undo, partial success, concurrent conflict, timeout and dependency-down paths wherever the
flow has them. A timeout means outcome unknown, never "did not happen".

## 5. Flow card

```text
FLOW-NNN <name> · type · revision <git hash> · status current|stale
Goal and user value:
Trigger, actor, preconditions, permissions:
Entry points (file:line):
Modules, stores, external dependencies:
Key inputs and business data:
Steps (node → file:line · reads · writes · emits · on failure):
States and legal transitions (closure-spec IDs when a model exists):
Normal end · failure conditions · terminals:
Transaction boundary and consistency (strong or eventual; window):
Idempotency key · retry owner · concurrency guard · compensation:
Observation points (queries, APIs, logs, events, audit):
Open questions (each with its node ID):
```

## 6. With a closure model

When `docs/01-系统设计/业务闭环/03-closure-spec.yaml` (legacy `docs/arc/`) exists, take states,
transitions and invariants from it and cite them in step text (`X-03`, `INV-02`); never redefine
them. A code path with no matching transition, or a modeled transition that no chart reaches, is a
model/implementation difference: hand it to `arc:plan` model review with both sources; never edit the
spec here. Every spec scenario maps to at least one path through the charts. Generated sequences
checked against the spec: [model-conformance](model-conformance.md) §4.
