# Worked Example: Reservation, Payment Notice, Cancel

A deliberately defined example contract, not a project fact. Replace every state, operation, ID
and policy with the project's contract; never copy them as default rules or seeds. Every status
here is `NOT_RUN`.

## Contract

| Rule | Promise |
|---|---|
| R1 stock | integer `1 ≤ q ≤ Qmax` and enough available stock; reserve lowers available and creates `HELD`; no oversell |
| R2 state | `HELD → CONFIRMED` keeps the hold; `HELD → CANCELLED` releases it once; a confirm/cancel race is decided by the first committed legal transition; cancel after confirm is rejected |
| R3 duplicates | same operation ID + same parameters returns the same result with no new stock or fulfilment effect; same ID + different parameters is rejected |
| R4 notice | confirm emits a fulfilment event; redelivery may happen; exactly one fulfilment record per reservation. A payment notice after cancel creates exactly one reconciliation item; stock stays released |
| R5 auth, progress | users act only on their own reservations; the payment entry verifies its source; with healthy dependencies a confirmed reservation gets its fulfilment record within project deadline `D` |

`Qmax`, `D`, retry deadlines and the environment come from the project. Settlement, refunds,
shipping and reconciliation disposition are out of scope, so this example never shows a whole
trading system is reliable.

```text
available + Σq(HELD) + Σq(CONFIRMED) = initial_stock
available ≥ 0
quantity / owner / reservation identity match the contract
each CONFIRMED has exactly one fulfilment record (by D); each CANCELLED has none
```

The last line is conditional liveness: a record missing before `D` is not yet a failure, while
forbidden effects are checked at every moment, not only at the end.

## Why these cases are required

The change touches the conditional stock update, callback dedup and worker retry. Oversell and
duplicate effects are costly, users retry after timeouts, the worker crash window is real, and
errors may surface only at reconciliation. A help-text-only change would need none of this.

| Group | Setup and action | Strong assertions | Boundary |
|---|---|---|---|
| S1 bounds | `q` = 0, inside/outside bounds, exactly available, more than available; cross-user action; same ID with changed parameters | exact result for valid input; after a rejection, stock, holds and fulfilment of the target and other users unchanged | decision-table unit tests + one persisted check through the real entry |
| S2 last unit | two different reservations race for `initial_stock=1, q=1` through a barrier | exactly one succeeds and the other gets insufficient stock; one `HELD` with the right identity; available is 0 and conserved; no pre-picked winner unless the contract fixes one | real usecase/transaction + real database; history of both operations |
| S3 state races | concurrent duplicate cancel and confirm; both commit orders; payment notice replay after cancel | confirm wins: no release, one fulfilment effect, cancel rejected. Cancel wins: released once, no fulfilment, one reconciliation item. Duplicate transport ≠ duplicate effect | controlled interleaving + exact state/stock/event/fulfilment check |
| S4 unknown outcome, recovery | response lost after DB commit, then same-ID retry; worker crashes after writing fulfilment, before ack; restart and redeliver | no rollback inferred from a client timeout; one reservation and one effect; progress within `D`; nothing pending dropped; no second charge | real persistence/message semantics + targeted fault + independent query after recovery |
| S5 compatibility (only if formats or dedup identity change) | mixed old/new versions, full upgrade, rollback to the supported version, old and new data flowing | R1–R5 hold in every stage; old consumers handle supported formats; rollback still reads written data | representative topology, real historical data, background cycles |

S5 enters only on compatibility risk, and then it is required, not optional. Performance risk
gets its own load and resource-budget evidence; functional passes are not capacity verdicts.

## S4 expanded: redelivery must not duplicate fulfilment

**Target defect:** the consumer produces the effect before acking; after a crash and redelivery
it produces it again. "Consumer was called" or "queue is empty" cannot detect this.

1. **Prepare.** In an authorized isolated environment create a legal reservation, record its
   identity and stock baseline, confirm it. Confirm the real worker, ack mode and dedup
   constraint; the observer reads fulfilment records independently. Clean up only this run's data.
2. **Inject.** Stop the worker at "effect persisted, message not acked"; record the effect commit
   and message identity. The fault seam matches the real mechanism; mock call counts cannot stand
   in for this window.
3. **Recover.** Restart the worker; let the real delivery mechanism redeliver the same logical
   event. Record each delivery, result, ack and end state; poll the completion condition up to
   `D`, never a fixed sleep.
4. **Judge independently.** The complete fulfilment record set holds exactly one record for this
   reservation with the right object and quantity; no other reservation gained effects; stock is
   conserved; state is still `CONFIRMED`; no extra charge request; the message completes or
   reaches the contracted visible end state. Observer failure → `BLOCKED`.
5. **Sensitivity.** Run the scenario on the pre-fix version (it fails on the uniqueness
   assertion), then on the candidate. Without a historical version, mutate dedup in a disposable
   worktree; if neither is done, say so.

If the effect lives in an external service, observe it through that service's test API or
reconciliation; a local "sent" flag does not prove the remote side ran once. Without that
observer the claim is `BLOCKED` and independent rules continue.

## Looks green vs catches the bug

| Weak oracle | Misses | Used here |
|---|---|---|
| request succeeded | wrong object, not persisted, follow-up lost | identity, quantity, state, stock and fulfilment fully checked |
| one fulfilment record exists | it belongs to another reservation | exact set and field comparison plus untouched objects |
| both retries succeeded | double release or double effect | one effect per logical ID; conservation holds |
| worker healthy after restart | one task stuck forever | this task completes within `D`; backlog and failures accounted for |
| old and new versions each pass | format break during mixed deployment | coexistence, upgrade and supported rollback under live traffic |

## Binding

Locate the real selectors, entry points, transaction layer, message adapter, CI and fixtures with
code-intel queries, then bind S1–S5. Each run records case and rule, candidate/config/environment
identity, selector and tool version, interleaving or fault point, operation history, full business
observation, and failure and cleanup artifacts ([release-ops.md](release-ops.md) §1).
