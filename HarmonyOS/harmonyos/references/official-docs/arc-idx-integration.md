# arc-idx integration

Use this when the HarmonyOS skill and the `arc:idx` / `arc-idx` skill are both available.

## Ownership model

Each tool owns a different kind of truth:

| Need | Owner | Why |
|---|---|---|
| Find project files, symbols, references, usages, AST shapes | `arc-idx` | Repository-local source of truth |
| Verify HarmonyOS APIs, permissions, lifecycle, capabilities, API levels, configuration semantics | HarmonyOS official-doc Meilisearch | Platform source of truth |
| Prove the edited project still compiles/tests/lints | Repository build/test commands | Executable source of truth |

Do not make the two retrieval systems compete.

- Never use the HarmonyOS docs index to infer what the local project currently implements.
- Never treat a local use of an API as proof that the HarmonyOS API, signature, permission, lifecycle rule, or compatibility claim is correct.
- When `arc-idx` is available, it owns local repository discovery. Do not bypass it with `grep`, `rg`, `find`, `fd`, or another ad-hoc repository search path.
- Directly reading a file that is already known is fine. The restriction is on discovery/search, not on reading known files.

## Preferred combined workflow

1. **Locate with arc-idx.**
   - `arc-idx files` for path discovery.
   - `arc-idx search` for lexical/code occurrence discovery.
   - `arc-idx symbol` for symbol definitions/references when applicable.
   - `arc-idx ast` for structural questions when applicable.
2. **Extract uncertain HarmonyOS facts from the located code.**
   Examples: imported platform type, decorator, permission, lifecycle callback, JSON5 key, error code.
3. **Verify those facts in the official-doc index.**
   Search exact identifiers first with `scripts/official-docs/harmony_docs.py`.
4. **Assess impact with arc-idx.**
   Search callers/usages/config references before changing public or shared behavior.
5. **Edit only known files.**
6. **Run project-native build/tests/lint.**
7. **If a failure reveals another platform uncertainty, search the docs again.**
8. **Summarize evidence by source.**
   Separate repository evidence, official HarmonyOS evidence, and executable verification.

## Routing examples

### “修复 VpnExtensionAbility 启动失败”

- `arc-idx`: locate the extension implementation, `module.json5`, callers, and error handling.
- HarmonyOS docs: verify `VpnExtensionAbility`, required permissions/capabilities/configuration/lifecycle.
- build/test: compile and run the relevant validation.

### “这个项目哪里使用了 onWindowStageCreate？”

This is primarily repository discovery. Use `arc-idx` first. Only search HarmonyOS docs when the user also needs the callback's official semantics.

### “onWindowStageCreate 的调用时机是什么？”

This is primarily a platform-fact question. Search the HarmonyOS docs. `arc-idx` is only needed if the answer must be related to the user's current implementation.

### “把旧 API 迁移到新 API”

- HarmonyOS docs: establish deprecation/replacement and compatibility facts.
- `arc-idx`: enumerate all local usages and impacted symbols/config.
- build/test: verify migration completeness.

## Conflict rule

If repository code and official docs appear to disagree:

1. Treat the repository as evidence of current implementation only.
2. Treat the indexed official docs as evidence of documented platform behavior.
3. Do not silently choose one and hide the discrepancy.
4. Check API level, device category, SDK/version, Stage/FA model, and deprecation context.
5. Report the mismatch and make the smallest change supported by both the target project constraints and the official documentation.
