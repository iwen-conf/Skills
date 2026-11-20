# Claude Code Host Adapter

1. Follow root `AGENTS.md`. Run `arc-wave status` (or `arc db status --repo .` when `.arc/arc.db` exists) before broad exploration.
2. Edit strictly inside the task's `write_scope`; update frozen contracts first when cross-boundary signatures must change.
3. Run `~/.claude/skills/arc-code/scripts/gate.sh check --task <slug>` and `arc update --changed-only` before stopping or handing off.
