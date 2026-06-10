# Conductor Pattern

This repository no longer maintains an Arc conductor. Codebase context retrieval is handled by local `.ai-code-index/` helpers. Cross-agent orchestration, Inbox consumption, and task dispatch are handled by `aitask`.

Arc skills can still be used as local working methods inside an `aitask` orchestration:

- `arc:define`: define a project or PRD.
- `arc:clarify`: clarify task inputs and acceptance criteria.
- `arc:docs`: initialize or maintain Lark resources only for existing `.lark.json`, a user-provided Lark project link, or explicit Lark project-space creation/update.
- `arc:build`: deliver scoped code changes.
- `arc:frontend`: handle frontend lifecycle work.
- `arc:fix`: repair evidence-backed failures.
- `arc:audit`: review quality and risk without editing code.
