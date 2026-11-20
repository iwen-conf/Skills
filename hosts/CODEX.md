# Codex Host Adapter

1. Follow root `AGENTS.md`. Prefer deterministic CLI proof (`arc idx`, `arc-wave`, `gate.sh check`, `arc verify`) over prose summaries.
2. Honor `.arc/arc.db` optimistic CAS versions (`--expected-version`) and frozen OpenAPI / Protobuf / SQL contracts.
3. Record exit codes, touched `file:line`, and `arc-wave handoff` packet before switching agents.
