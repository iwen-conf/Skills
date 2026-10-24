# HarmonyOS development workflow

Use this for implementation, debugging, refactoring, or code review in a HarmonyOS repository.

## Inspect only what the task needs

When `arc-idx` is available, use it for repository discovery (`files`, `search`, `symbol`, `ast`) and follow the `arc:idx` skill constraints. Do not fall back to `grep`, `rg`, `find`, or `fd` for discovery. Directly reading an already-known path remains appropriate.


Useful project signals include:

- `build-profile.json5` — products, build modes, modules, targets.
- `oh-package.json5` / `oh-package-lock.json5` — package dependencies.
- `hvigorfile.ts` and module hvigor files — build customization.
- `AppScope/app.json5` — application-level metadata.
- `src/main/module.json5` — module abilities, extensions, permissions, skills, metadata.
- `src/main/ets/` — ArkTS/ArkUI implementation.
- `src/main/resources/` — resources, strings, media, profiles.

Do not read the whole repository by default. Trace from the file/symbol involved in the task.

## Before editing platform-specific code

Search the official docs when the change depends on any of the following:

- a HarmonyOS import path, class, interface, enum, decorator, or callback;
- ability/extension lifecycle behavior;
- permission declaration or authorization flow;
- system capability, API level, device category, or compatibility boundary;
- background execution, networking, storage, security, media, distributed, VPN, WebView, notifications, or system service APIs;
- ArkUI component behavior that affects correctness;
- module/app configuration keys;
- DevEco/Hvigor build semantics.

General algorithms and language-local refactors do not require a documentation search unless they touch platform behavior.

## Implementation sequence

1. Reproduce or locate the current behavior.
2. Identify the smallest platform fact that must be verified.
3. Search exact symbols/config/permissions.
4. Make the smallest coherent code/config change.
5. Search again only if the implementation exposes another uncertain platform fact.
6. Run the repository's relevant checks.
7. Inspect failures: distinguish pre-existing failures from failures introduced by the change.
8. Fix introduced failures and rerun affected checks.

## Verification commands

Prefer commands already used by the repository (README, package scripts, CI, existing tooling). Typical HarmonyOS projects may use `hvigorw`/Hvigor tasks, but do not invent a task name. Discover the available project command first when uncertain.

For a pure explanation task, verification means cross-checking the relevant docs, not running unrelated builds.

## Review focus

When reviewing HarmonyOS code, prioritize:

- API existence and import correctness;
- lifecycle/context correctness;
- permission/config completeness;
- async/error handling around platform APIs;
- device/API compatibility;
- resource/config references;
- obvious ArkTS type/compile issues;
- behavior that contradicts official docs.

Do not label stylistic preferences as platform correctness issues unless they have a concrete effect.
