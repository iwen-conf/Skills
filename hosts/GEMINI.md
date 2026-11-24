# Gemini CLI Host Adapter

1. Follow root `AGENTS.md`. Run `arc-wave status` (or `arc capabilities --json && arc db status --repo .` when `.arc/arc.db` exists) before broad exploration.
2. Edit strictly inside the task's `write_scope`; update frozen contracts first when cross-boundary signatures must change.
3. Run `bash ~/.gemini/config/skills/arc-code/scripts/gate.sh check --task <slug>`, `arc update --changed-only`, and `arc index flush --repo .` before stopping or handing off.
