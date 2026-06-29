---
name: project-architecture-conventions
description: Apply mandatory DIP, real ONC backend architecture naming/layering/interface conventions, and Go stdlib constant rules before writing or reviewing project code. Use when implementing or reviewing backend, service, controller, repository, infrastructure, helper, or project skeleton code that should learn from ONC.
---

# Project Architecture Conventions

## Overview

Use this skill before writing, changing, or reviewing backend, service, controller, repository, infrastructure, helper, or project skeleton code. Project code must follow the Dependency Inversion Principle (DIP) and learn naming, layering, and interface design from the real ONC backend architecture.

The ONC codebase, when available, is the source of truth for ONC architecture. Known local source path: `/Users/iluwen/Documents/Code/Go/ONC/backend`.

Before applying ONC architecture to a non-trivial change, inspect the current ONC source if it is available. If ONC source cannot be found, report that explicitly and use `references/onc-architecture.md` only as a cached baseline; do not invent ONC facts. Do not call this "ONC-style" or "ONC-inspired" when the source has been checked; call it ONC architecture.

## When to Use

- Use before implementing backend, service, controller, repository, integration, or helper code.
- Use before creating a new module, package, business feature, command entrypoint, or project skeleton.
- Use when reviewing whether code violates DIP, leaks infrastructure into business logic, or places helpers in the wrong layer.
- Use together with `code-comment-conventions` when adding comments to functions, controllers, or implementation steps.

## ONC Preflight

1. Locate ONC source. Prefer `$ONC_REPO/backend` when `ONC_REPO` is set, then `/Users/iluwen/Documents/Code/Go/ONC/backend`, then a nearby `../ONC/backend`.
2. Inspect real ONC code before using ONC facts. At minimum check `internal/domain`, `internal/usecase`, `internal/interface/restful`, `internal/infrastructure`, and `internal/wire`.
3. Read `references/onc-architecture.md` after source inspection, or as a fallback when the source is unavailable.
4. If ONC source and the cached reference disagree, trust the source and mention the observed difference.
5. Preserve the host repository's established patterns unless the task explicitly asks to align it with ONC.

## Ponytail Preflight

- Before writing code, load and read the installed `ponytail` skill from the local environment.
- If the environment exposes it as `$ponytail`, use that skill first.
- If it is installed under a skill directory, read its `SKILL.md` before code edits. Known fallback locations include `~/.claude/plugins/marketplaces/ponytail/skills/ponytail/SKILL.md` and `~/.codex/.tmp/marketplaces/ponytail/skills/ponytail/SKILL.md`.
- If `ponytail` cannot be found or loaded, report that explicitly before editing code; do not invent or infer ponytail rules.
- If `ponytail` conflicts with this skill, stop and report the conflict instead of silently choosing one rule.

## Ponytail Conflict Resolution

`ponytail` rejects unrequested abstractions such as an interface with one implementation. This skill explicitly requests DIP and ONC boundary interfaces, so interfaces are allowed only when they protect business logic from infrastructure, external capabilities, framework details, transport boundaries, or cross-layer dependencies.

Resolution rules:

- Required DIP boundary interfaces are not "unrequested abstraction"; they are part of the project contract.
- Keep `contract` minimal: define ports that business logic consumes, not interfaces for every struct or helper.
- Do not create service interfaces, factories, config objects, or adapter interfaces solely because a folder exists.
- Do not add an interface for private pure business code, local helpers, or same-layer calls unless a real boundary, test seam, or second implementation exists.
- Prefer one small constructor-injection path over frameworks, registries, reflection, or generated wiring.
- If a ponytail simplification would remove a required DIP boundary, keep the boundary and shrink everything around it.

## DIP Rules

All project code must obey DIP:

- High-level business policy depends on abstractions, not concrete storage, SDK, network, queue, cache, or framework implementations.
- `usecase/<module>` owns application/business behavior and consumes `domain/repositories` interfaces or explicit capability contracts.
- Concrete infrastructure implements domain or capability contracts and is wired in `internal/wire`; it is not constructed inside business logic.
- Controllers translate transport input into usecase calls and transport output; they must not contain core business decisions.
- Domain/usecase code must not import infrastructure-only packages unless those capabilities are expressed through contracts.
- New dependencies must be injected through constructors or explicit parameters, not pulled from globals or created ad hoc inside business functions.

## ONC Architecture

Use real ONC backend architecture as the reference for Go backend projects. Names may be adapted only when the host language or existing repository convention requires it; responsibilities and dependency direction must stay the same.

```text
backend/
├── cmd/server/
├── configs/
├── internal/
│   ├── bootstrap/
│   ├── constants/
│   ├── domain/{entities,events,filters,repositories,services}/
│   ├── usecase/<module>/
│   ├── interface/restful/{controllers,dto/{requests,responses},middlewares,router/routes}/
│   ├── infrastructure/{gateways,support}/
│   └── wire/
└── migrations/{up,down}/
```

Layer responsibilities:

- `domain`: Pure business entities, repository interfaces, events, filters, and domain services. It must not import infrastructure, usecase, interface, framework, or driver packages.
- `usecase/<module>`: Application/business workflows. ONC modules use `contract.go`, `main.go`, `params.go`, `results.go`, optional `errors.go`, and focused `service*.go` files. `Contract` is the controller-facing interface; `Service` implements it and depends on `domain/repositories` plus explicit external capability contracts.
- `interface/restful`: Gin/HTTP boundary. Controllers bind input, authorize, call usecase contracts, map errors, and return DTO responses. DTOs live in `dto/requests` and `dto/responses`; controllers must not touch repositories or database drivers directly.
- `infrastructure/gateways`: Concrete external gateways such as Postgres persistence, notification, storage, and recommendation. Persistence uses `postgres/models` for table models and `postgres/repository` for implementations of `domain/repositories`.
- `infrastructure/support`: Cross-cutting support capabilities such as authorization, cache, logger, and security. ONC uses `contract.go`, `engine.go`, `service.go`, and `main.go` to separate service contracts from concrete engines.
- `wire`: Composition root. Construct repositories, support services, usecases, controllers, bootstrap, seeds, and application lifecycle. It may import concrete infrastructure; business layers may not.
- `bootstrap`: Startup domain initialization such as ensuring seed data or super-admin prerequisites after migrations and repository construction.

## Dependency Direction

Use ONC's dependency direction:

```text
cmd -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
internal/infrastructure -> internal/domain
```

Forbidden ONC-aligned imports:

```text
internal/domain -> internal/infrastructure, internal/usecase, internal/interface, framework/driver SDKs
internal/usecase -> gin, net/http, pgx, database/sql, internal/interface
internal/interface/restful/controllers -> domain/repositories, pgx, database/sql, pgxpool
```

## Main And Injection

Keep object creation and wiring in `internal/wire`, `main`, or the project composition root:

```go
func main() {
    app, err := wire.NewApplication()
    if err != nil {
        panic(err)
    }
    if err := app.Start(); err != nil {
        panic(err)
    }
}
```

Rules:

- `wire` / `main` may know concrete implementations because it is the injection point.
- `usecase/<module>.New(...)` must accept domain repository interfaces and explicit capability contracts, not concrete adapters.
- Business methods must not call repository constructors, `sql.Open`, SDK constructors, HTTP client setup, or queue/cache constructors.
- If dependency construction requires configuration, parse configuration before injection and pass typed values into constructors.

## Go Native Constants

For Go code, treat standard-library exported constants and typed values as mandatory when they represent the intended literal.

- MUST use Go standard-library constants instead of equivalent magic strings or numbers.
- MUST replace date/time layout literals with `time` constants when available, such as `time.DateTime` instead of `"2006-01-02 15:04:05"`, `time.DateOnly` instead of `"2006-01-02"`, `time.TimeOnly` instead of `"15:04:05"`, and `time.RFC3339` / `time.RFC3339Nano` for RFC3339 layouts.
- MUST use standard-library semantic constants for HTTP methods, status codes, file modes, TLS versions, crypto hashes, and other native package domains when the package exposes one.
- MUST NOT introduce raw string literals for values already defined by the Go standard library. If no standard-library constant exists, define a local named constant at the narrowest useful scope instead of repeating the literal.
- During review, flag equivalent literals as defects even when the code compiles.

## Helper And Shared Code

Follow ONC's helper placement:

1. Keep module-specific usecase helpers inside the same `internal/usecase/<module>` package, using focused files such as `helpers.go`, `service_<feature>.go`, or `services.go` when the module already uses them.
2. Put application-layer shared types such as pagination or common business errors in `internal/usecase/shared`; do not duplicate them in each feature module.
3. Put REST request fragments in `internal/interface/restful/dto/requests` and response payloads in `internal/interface/restful/dto/responses`; controllers should call mapper helpers instead of declaring private DTO structs inline for runtime responses.
4. Keep controller helpers focused inside `internal/interface/restful/controllers` only when they are transport-boundary helpers. Move pure business helpers down into usecase.
5. Avoid vague `common`, `misc`, `tools`, or broad `utils` buckets. Extract only after real reuse and with a specific package purpose.

## Review Checklist

- ONC source was inspected, or its absence was explicitly reported before applying cached ONC guidance.
- Business logic is in `usecase/<module>`, not controllers, infrastructure, or `wire`.
- Business code depends on `domain/repositories` or explicit capability contracts instead of concrete infrastructure.
- Each interface is justified by a layer boundary, external capability, test seam, or multiple implementations.
- Concrete infrastructure is wired in `internal/wire` and implements domain or capability contracts.
- Usecase `contract.go` contains the exported controller-facing contract, not concrete service or adapter logic.
- List/query contracts return success with empty collections for no-data results; single-resource missing cases are represented intentionally as `not found` only when the product flow needs a missing-resource error state.
- Controllers and frontend DTOs can distinguish empty, not-found, permission-denied, validation error, and system error without relying on generic error text.
- Go code uses standard-library constants for native semantic literals, especially date/time layouts such as `time.DateTime`; raw equivalent strings are not accepted.
- `helpers` are business-local unless proven reusable.
- Shared helpers extracted late live under `pkg/utils/<specific-name>` and do not import business modules.
- `ponytail` was read before coding, or its absence was reported before editing.
