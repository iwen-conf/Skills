# Business Verification Plan

This is a compact engineering contract, not a QA scorecard. Only fill sections that are declared applicable.

## 1. Change / objective
- Capability/change:
- Business objective:
- Owner:
- Risk consequence (low/medium/high/critical):
- Requirement/design/incident references:
- In scope / out of scope:

## 2. Applicability
Declare yes/no so omission is explicit:

| Concern | Applies? | Why |
|---|---|---|
| Stateful workflow | | |
| Permissions/tenant scope | | |
| Concurrency/shared mutable state | | |
| Retry/idempotency | | |
| External side effects/events | | |
| Time/expiry/window boundaries | | |
| Compatibility/version/flag matrix | | |
| Migration/upgrade | | |
| Async delivery/callback | | |
| Destructive/irreversible action | | |
| Long-running/resource accumulation | | |

## 3. Actors and authority
| Actor | Context/tenant | Allowed | Forbidden |
|---|---|---|---|

## 4. Business rules and oracles
| Rule | Statement | Consequence if wrong | Oracle(s) | Source |
|---|---|---|---|---|

Oracles should say what is observed and what exact business assertion decides correctness.

## 5. State model (if applicable)
| State | Meaning | Terminal? |
|---|---|---|

| From | Event | To | Forbidden alternatives |
|---|---|---|---|

## 6. Side-effect model (if applicable)
| Side effect | Visibility/commit point | Duplicate/retry semantics | Compensation/recovery |
|---|---|---|---|

## 7. Primary journeys
For each journey:
- Actors and preconditions
- Trigger
- Business steps
- Expected outcomes
- Forbidden outcomes
- State changes
- External/event/audit effects
- Related rules

## 8. Scenario portfolio
| ID | Class | Given | When | Expected | Forbidden | Oracle | Execution mode | Evidence layer |
|---|---|---|---|---|---|---|---|---|

Use only relevant classes: normal, boundary, invalid, permission, state-transition, duplicate/retry, concurrency, timeout/failure, recovery, compatibility, migration, expiry, overload.

## 9. Cross-business interactions
| Flow A | Flow B | Shared state/resource | Forced condition/interleaving | Expected | Forbidden |
|---|---|---|---|---|---|

## 10. Test environment/data contract
- Isolation strategy:
- Real dependencies required:
- Provision/reset/healthcheck/teardown:
- Seed/reference data:
- Production data/privacy restrictions:

## 11. Execution bindings
Map automated scenario IDs to repository-specific HTTP/gRPC/job/SQL/device steps in a separate binding file. Keep implementation details out of the business rules.

## 12. Release contract
- Required scenarios for release:
- Required manual evidence:
- P0/P1 blockers:
- Rollback/containment:
- Time-bounded waivers, if any:

## 13. Incident/regression links
Map historical bugs/incidents to permanent scenarios, corpora, assertions, monitors or architecture controls.
