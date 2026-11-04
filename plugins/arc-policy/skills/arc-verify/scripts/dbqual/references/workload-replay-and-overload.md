# Workload replay, overload and recovery

## Production-shape replay

Synthetic workloads are useful for controlled parameter sweeps but can miss session shape and concurrency dependencies. `workload_replay.py` provides a sanitized JSONL trace format with:

- logical session ID;
- relative start offset;
- SQL text or test-safe prepared equivalent;
- sanitized parameters;
- optional tag/fetch behavior.

The replayer preserves per-session order and approximate relative timing on an isolated database and records schedule lag, latency and SQLSTATEs.

Do not capture secrets or sensitive production values. Production traces must be sampled, sanitized/tokenized and approved for test use.

The replayer is intentionally simpler than Oracle Database Replay: it does not claim exact kernel-level reproduction of production transaction dependencies. Its purpose is to bring the *principle* of timing/concurrency-aware replay to PostgreSQL business testing.

## Overload testing

A capacity test should answer more than “maximum TPS.” Use both:

- gradual ramp to reveal the knee;
- impulse/spike load to expose queueing/cold-cache behavior;
- load beyond the expected operating region;
- return to nominal load to verify recovery.

Record:

- achieved TPS;
- P95/P99 execution latency;
- schedule lag under arrival-rate tests;
- failures, retries and skips;
- lock/connection/resource counters;
- recovery P99 and TPS relative to the pre-overload nominal phase.

A system that handles 10k TPS but stays degraded after overload is not healthy.
