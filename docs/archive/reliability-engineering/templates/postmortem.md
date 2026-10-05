# Reliability Incident Postmortem

## Impact
What users/data/money/devices were affected? Quantify duration and blast radius.

## Detection
How was it detected? What should have detected it earlier?

## Timeline
Use factual timestamps and actions.

## Correctness contract violated
- Invariant/contract ID:
- Expected behavior:
- Actual behavior:

## Contributing conditions
Describe technical and organizational/system conditions without individual blame.

## Why verification missed it
Choose all that apply and explain:
- specification gap
- missing failure model
- weak generator/distribution
- weak/circular oracle
- missing integration realism
- concurrency/interleaving gap
- lifecycle/upgrade/recovery gap
- monitoring/detection gap
- rollout/blast-radius gap
- operator/config guardrail gap
- architecture not testable/deterministic enough

## Reproduction
- Minimal reproducer:
- Seed/schedule/corpus:
- Environment/version:

## Durable actions
| Action | Prevent / Detect / Contain / Recover | Owner | Due | Verification of action |
|---|---|---|---|---|

## Regression artifact
Test/corpus/model/failpoint/check added:

## Follow-up validation
How will we prove the action actually catches or reduces recurrence?
