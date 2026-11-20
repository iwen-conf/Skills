# HarmonyOS NEXT Hard-Spec Cheatsheet

## 1. Platform & SDK Baseline

- **Production Default**: **HarmonyOS 6.1.1 Release / API 24** on **Stage model** (`UIAbility`, `.ets`, `module.json5`, `oh-package.json5`). FA model is legacy migration context only.
- **Preview Isolation**: **HarmonyOS 7 / API 26 Beta1** (DevEco 26 Beta / Node 24) APIs and behaviors must never leak into API 24 production code unless explicitly requested.
- **Imports**: Use `@kit.*` module imports (`@kit.ArkUI`, `@kit.AbilityKit`, `@kit.NetworkKit`, `@kit.ArkData`).

## 2. ArkTS Compile-Time Strictness (TS -> ArkTS Traps)

- **Banned Constructs**: `any`, `unknown`, structural duck-typing, runtime property addition/deletion, `Symbol`, `in` operator, untyped object literals (`const cfg = { a: 1 }` without explicit class/interface), and dynamic `Object.assign` layout changes.
- Declare explicit `interface` / `class` types, initialize fields deterministically, and handle `null`/`undefined` union branches explicitly—never hide build errors behind unsafe casts or shims.

## 3. State Management & Lifecycle

- **V2 Reactive State** (`@ComponentV2`): `@Local`, `@Param`, `@Once`, `@Event`, `@ObservedV2` + `@Trace`.
- **V1 State** (`@Component`): `@State`, `@Prop`, `@Link`, `@Observed` + `@ObjectLink`. Never mix V1 and V2 decorators on the same component or observed class.
- Render long lists with `LazyForEach` + deterministic key generator; clean up timers, subscriptions, and native handles in `aboutToDisappear` and `UIAbility` lifecycle callbacks.
