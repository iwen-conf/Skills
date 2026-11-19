# Evidence provenance, harness trust and waiver standard

G31 exists because a sophisticated test program can still lie through missing observations, stale baselines, parser bugs or administrative waivers.

## Provenance

For release-blocking evidence preserve enough identity to decide whether results are comparable and reproducible:

```text
commit/build identity
PostgreSQL version
settings/schema fingerprint
data profile + seed
workload/trace/script hash
tool version
runner/hardware identity when performance matters
start/end time
status and raw/minimal failure artifacts
```

## Harness trust

Unit-test parsers, percentile math, SQLSTATE accounting, plan interpretation, graph-cycle detection, report generation and gate semantics. Exercise fail-closed behavior by deliberately removing dependencies/credentials and confirming the suite becomes BLOCKED/FAIL rather than PASS.

Observers that are required by a claim are part of the test. If the lock observer, metrics scraper, history recorder or reconciliation query fails, the claim is BLOCKED even if the workload itself exits zero.

Validate the evidence schema, unique JSON fields and evidence IDs before counting. Empty
selectors/policies, zero or invalid minima, and selected FAIL/BLOCKED/NOT_RUN/FLAKY records cannot
qualify a release. Metrics need finite numeric observations; warning severity applies to a
measured threshold breach, never to missing, boolean or invalid measurements.

The release gate freezes policy and the complete current evidence/RAW file set before evaluation
and checks them again with the candidate afterward. Its `input_binding` carries policy path/hash
and evidence hashes. Baseline promotion revalidates those inputs and the candidate, re-evaluates
the policy, and copies the complete evidence/RAW set. Regression requires a nonempty, complete,
unchanged baseline manifest. Superseded runs remain history; they cannot satisfy current claims.
Keep gate output outside the evidence directory. Same-user hashes do not establish independent
runner authenticity; protected CI must produce its own evidence.

## Waivers

A waiver must include:

```text
gate/claim
specific missing/failing evidence
risk being accepted
owner
expiry
containment
production detection
rollback/repair path
```

No anonymous, permanent or silently auto-renewed waiver. A waiver changes the release decision process; it does not rewrite FAIL/BLOCKED evidence into PASS.
