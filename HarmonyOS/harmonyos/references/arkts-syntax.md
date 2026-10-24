# ArkTS Syntax Assistant

Syntax mode of `harmonyos`, adapted from the MIT-licensed ArkTS syntax assistant ([LICENSE](arkts/LICENSE.txt); 中文：[README.zh.md](arkts/README.zh.md)).

## Evidence Gate

For compile-error resolution or performance optimization, MUST load the evidence-first root-cause repair
gate in [`docs/execution-truth.md`](../../../docs/execution-truth.md). Preserve the exact build error or
measured baseline, verify the target SDK/toolchain and owning language or component boundary, and do
not use casts, compatibility shims, or syntax-only rewrites to hide an unconfirmed cause.

## Overview

ArkTS is the default development language for OpenHarmony applications. It builds
upon TypeScript with enhanced static typing to improve program stability and
performance.

## Core Features

- **Static Typing**: All types determined at compile time, reducing runtime checks
- **No Dynamic Object Layout**: Object structure fixed at compile time, cannot be
  modified at runtime
- **Restricted Operators**: Some operator behaviors are restricted to encourage
  clearer code semantics
- **No Structural Typing**: Structural typing is currently not supported

## Reference Documentation

| Scenario | Document |
|----------|----------|
| Syntax Learning | [references/en/introduction-to-arkts.md](arkts/en/introduction-to-arkts.md) |
| Quick Overview | [references/en/arkts-get-started.md](arkts/en/arkts-get-started.md) |
| TS Migration | [references/en/typescript-to-arkts-migration-guide.md](arkts/en/typescript-to-arkts-migration-guide.md) |
| Migration Background | [references/en/arkts-migration-background.md](arkts/en/arkts-migration-background.md) |
| Performance | [references/en/arkts-high-performance-programming.md](arkts/en/arkts-high-performance-programming.md) |
| More Cases | [references/en/arkts-more-cases.md](arkts/en/arkts-more-cases.md) |

## Workflows

### 1. Syntax Questions

```
User Question -> Identify Question Type -> Consult Documentation -> Provide Code Example
```

**Common Syntax Questions**:
- Variable declaration -> Use `let`/`const` with explicit type or inference
- Function definition -> Supports optional parameters, defaults, rest parameters,
  arrow functions
- Classes and interfaces -> Must initialize fields, supports inheritance and
  implementation
- Generics -> Supports constraints and default values
- Null safety -> Nullable types ((T | null)), non-null assertion ((!)), optional
  chaining ((?.))

### 2. TypeScript Migration

```
Identify TS Code -> Check Incompatible Features -> Consult Migration Rules -> Provide ArkTS Alternative
```

**Key Migration Rules Quick Reference**:

| TypeScript | ArkTS Alternative |
|------------|-------------------|
| `var x` | `let x` |
| `any`/`unknown` | Specific types |
| `{n: 42}` object literal | Define class/interface first |
| `[index: T]: U` index signature | `Record<T, U>` |
| `A & B` intersection type | `interface C extends A, B` |
| `function(){}` function expression | `() => {}` arrow function |
| `<Type>value` type assertion | `value as Type` |
| Destructuring `[a, b] = arr` | Individual access `arr[0]`, `arr[1]` |
| `for..in` | `for` loop or `for..of` |
| Constructor parameter properties | Explicit field declaration |

### 3. Performance Optimization

```
Analyze Code -> Identify Performance Issues -> Consult Optimization Guide -> Provide Solutions
```

**High-Performance Programming Key Points**:

- **Declarations**: Use `const` for invariants; avoid mixing integer and float
- **Loops**: Extract loop invariants; avoid numeric overflow
- **Functions**: Parameter passing preferred over closures; avoid optional
  parameters
- **Arrays**: Use TypedArray for numeric values; avoid sparse arrays and union
  type arrays
- **Exceptions**: Avoid throwing in loops; use return values instead

### 4. Compile Error Resolution

```
Get Error Message -> Search Migration Rules -> Find Related Case -> Provide Fix
```

## Common Questions

For syntax examples (JSON parsing, object types, generics and migration), read [syntax FAQ](arkts/syntax-faq.md).

## Prohibited Standard Library APIs

The following are prohibited in ArkTS:

- **Global**: `eval`
- **Object**: `__proto__`, `defineProperty`, `freeze`, `getPrototypeOf`, etc.
- **Reflect**: `apply`, `construct`, `defineProperty`, etc.
- **Proxy**: All handler methods

## Build Scripts

The scripts directory provides quick compilation scripts for ArkTS projects
(including dependency installation):

| Platform | Script | Purpose |
|----------|--------|---------|
| macOS/Linux | `scripts/arkts/run.sh` | Execute `ohpm install` + `hvigorw assembleApp` |
| Windows | `scripts/arkts/run.ps1` | Execute `ohpm install` + `hvigorw assembleApp` |

`SKILL_ROOT` is the loaded skill directory; `PROJECT_ROOT` is the discovered project root. The scripts use the project wrapper and existing registry, with explicit exit-code checks. Prefer existing project build commands when already established.

Usage:
```bash
# macOS/Linux
bash "$SKILL_ROOT/scripts/run.sh" "$PROJECT_ROOT"

# Windows PowerShell
& "$SKILL_ROOT/scripts/run.ps1" -ProjectRoot "$PROJECT_ROOT"
```

Script execution steps:
1. Install dependencies (`ohpm install --all`)
2. Build project (`hvigorw assembleApp`)

## Mandatory Requirements

**CRITICAL**: When this skill generates ArkTS code, the following workflow MUST be followed:

1. **Compilation Verification**: After generating code, you MUST compile the project using the build scripts:
   - macOS/Linux: `bash "$SKILL_ROOT/scripts/run.sh" "$PROJECT_ROOT"`
   - Windows: `& "$SKILL_ROOT/scripts/run.ps1" -ProjectRoot "$PROJECT_ROOT"`

2. **Retry Strategy**: If compilation fails:
   - Analyze the error output
   - Fix the issue and retry compilation
   - Maximum of **3 compilation attempts**

3. **Stop**: Missing toolchain/environment or the same unexplained failure is BLOCKED; after at most 3 evidence-driven attempts, report the blocker via the runtime's available user-input/message channel. Do not treat silence as permission or label skipped compilation passed.

4. **Error Reporting**: Show the first root error and a bounded excerpt; keep the full log as an artifact with its path.

## Answer Guidelines

1. **Prioritize code examples**: Show correct vs incorrect syntax comparison
2. **Reference official documentation**: For detailed explanations, guide users to
   consult corresponding documents in references/
3. **Explain reasons**: Explain why ArkTS has this restriction (performance,
   stability)
4. **Provide alternatives**: For unsupported features, provide feasible
  alternatives

## License

MIT License - see [LICENSE.txt](arkts/LICENSE.txt)
