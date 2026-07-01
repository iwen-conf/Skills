---
name: arc:project-architecture-conventions
description: Apply mandatory backend architecture, DIP, zap logging, and Go constant rules before coding.
---

# Project Architecture Conventions

## Overview

Use this skill before writing, changing, or reviewing backend, service, controller, repository, infrastructure, helper, logging, debugging, constants, enum-like states, or project skeleton code. Project code must follow the default backend architecture, the Dependency Inversion Principle (DIP), and the naming, layering, file, interface, logging, debugging, constant, and observability conventions defined here.

For new backend modules and project skeletons, use this architecture by default. For existing repositories, preserve established local patterns unless the task explicitly asks to migrate toward this architecture.

## When to Use

- Use before implementing backend, service, controller, repository, integration, or helper code.
- Use before creating a new module, package, business feature, command entrypoint, or project skeleton.
- Use when adding or reviewing zap logs, business observability, debugging evidence, constants, enum-like state, or error reporting.
- Use when reviewing whether code violates DIP, leaks infrastructure into business logic, places helpers in the wrong layer, logs without business value, or uses magic literals.
- Use together with `arc:code-comment-conventions` when adding comments to functions, controllers, or implementation steps.

## Architecture Preflight

1. Inspect the host repository first: package layout, constructors, contracts, DTOs, tests, and dependency direction.
2. For new modules, use the default architecture in this skill unless the repository already has a stronger local convention.
3. For migrations, change the smallest slice that can preserve behavior while moving toward the default architecture.
4. Read `references/backend-architecture.md` when deeper file-level or interface-level guidance is needed.
5. Do not invent extra layers, factories, interfaces, or helpers beyond the boundaries described here.

## Ponytail Preflight

- Before writing code, load and read the installed `ponytail` skill from the local environment.
- If the environment exposes it as `$ponytail`, use that skill first.
- If it is installed under a skill directory, read its `SKILL.md` before code edits. Known fallback locations include `~/.claude/plugins/marketplaces/ponytail/skills/ponytail/SKILL.md` and `~/.codex/.tmp/marketplaces/ponytail/skills/ponytail/SKILL.md`.
- If `ponytail` cannot be found or loaded, report that explicitly before editing code; do not invent or infer ponytail rules.
- If `ponytail` conflicts with this skill, stop and report the conflict instead of silently choosing one rule.

## Ponytail Conflict Resolution

`ponytail` rejects unrequested abstractions such as an interface with one implementation. This skill explicitly requests DIP boundary interfaces, so interfaces are allowed only when they protect business logic from infrastructure, external capabilities, framework details, transport boundaries, or cross-layer dependencies.

Resolution rules:

- Required DIP boundary interfaces are not "unrequested abstraction"; they are part of the project contract.
- Keep contracts minimal: define usecase `Contract`, `domain/repositories` interfaces, or capability contracts that cross real boundaries; do not add interfaces for every struct or helper.
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

## Backend Architecture

Use this backend architecture as the default for Go backend projects. Names may be adapted only when the host language or existing repository convention requires it; responsibilities and dependency direction must stay the same.

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
- `usecase/<module>`: Application/business workflows. Modules use `contract.go`, `main.go`, `params.go`, `results.go`, optional `errors.go`, and focused `service*.go` files. `Contract` is the controller-facing interface; `Service` implements it and depends on `domain/repositories` plus explicit external capability contracts.
- `interface/restful`: Gin/HTTP boundary. Controllers bind input, authorize, call usecase contracts, map errors, and return DTO responses. DTOs live in `dto/requests` and `dto/responses`; controllers must not touch repositories or database drivers directly. DTO packages are transport schema only: do not put entity/usecase-to-DTO mapping constructors, factories, or business helpers there.
- `infrastructure/gateways`: Concrete external gateways such as Postgres persistence, notification, storage, and recommendation. Persistence uses `postgres/models` for table models and `postgres/repository` for implementations of `domain/repositories`.
- `infrastructure/support`: Cross-cutting support capabilities such as authorization, cache, logger, and security. Use `contract.go`, `engine.go`, `service.go`, and `main.go` to separate service contracts from concrete engines.
- `wire`: Composition root. Construct repositories, support services, usecases, controllers, bootstrap, seeds, and application lifecycle. It may import concrete infrastructure; business layers may not.
- `bootstrap`: Startup domain initialization such as ensuring seed data or super-admin prerequisites after migrations and repository construction.

## Go Response DTO Composition

For Go REST APIs, plan response bodies as named DTO compositions. Do not return dynamic maps or catch-all envelopes from controllers.

Define the shared response base in `internal/interface/restful/dto/responses/base.go`; if the host repository already uses singular `dto/response`, keep that local package name instead of renaming only for this rule. Preserve the repository's established base type name, such as `Base`, `BaseResponse`, or `ResponseBase`.

```go
type Base struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

var SuccessBase = Base{
    Success: true,
    Message: "ok",
}
```

Every endpoint response must define its own named response struct in the response DTO package. Embed `Base` and use explicit concrete fields.

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type UserResp struct {
    Base
    Data User `json:"data"`
}

type UserListResp struct {
    Base
    Data []User `json:"data"`
}

type Page struct {
    Page     int   `json:"page"`
    PageSize int   `json:"page_size"`
    Total    int64 `json:"total"`
}

type UserPageResp struct {
    Base
    Data []User `json:"data"`
    Page Page   `json:"page"`
}

type UserCursorResp struct {
    Base
    Data   []User                    `json:"data"`
    Cursor pagination.CursorResponse `json:"cursor"`
}
```

Use `Base: SuccessBase` directly in the controller response literal.

Rules:

- Split shared response schema by responsibility: `base.go` contains only the base envelope and base constants/helpers, `meta.go` contains shared response metadata such as `Meta`, `Pagination`, cursor, or page structs, and feature files such as `activity_category.go` contain only that resource's DTOs and endpoint response structs.
- Do not put `BaseResponse`, `Meta`, `Pagination`, and feature DTOs in one feature response file. Shared schema belongs in shared files.
- `Data` must be a concrete DTO type or slice of a concrete DTO type, such as `User` or `[]User`.
- Page-number pagination uses `Page Page` metadata (`page`, `page_size`, `total`); cursor pagination uses cursor metadata (`next_cursor`, `prev_cursor`, `has_more`). Do not mix the two contracts.
- Use `github.com/iwen-conf/utils-pkg/pagination` when available: `CursorRequest`, `CursorResponse`, and `NewHMACCodec` for cursor pagination; `OffsetRequest` and `OffsetResponse` for offset/limit APIs. If the public API is `page/page_size`, map it explicitly to query offset/limit instead of exposing cursor fields.
- Runtime response bodies must not use `any`, `interface{}`, `map[string]any`, `gin.H`, anonymous structs, or generic catch-all envelopes such as `Response[T any]`.
- Response DTO packages must not import `internal/domain`, `internal/usecase`, repositories, database models, Gin, or database drivers for mapping. Keep DTO files as named wire-contract structs plus harmless envelope constants/types.
- Do not add constructors, factories, or mapper helpers only to satisfy this rule; direct struct literals are fine unless the repository already has a helper pattern.
- If conversion from entities or usecase results is nontrivial, put the mapper at the HTTP boundary that owns the transport contract, usually `internal/interface/restful/controllers` or a focused mapper file in that package. Do not add functions like `responses.NewUser(entity)` or `responses.NewUserList(usecaseResult)`.

## Dependency Direction

Use this dependency direction:

```text
cmd -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
internal/infrastructure -> internal/domain
```

Forbidden imports:

```text
internal/domain -> internal/infrastructure, internal/usecase, internal/interface, framework/driver SDKs
internal/usecase -> gin, net/http, pgx, database/sql, internal/interface
internal/interface/restful/controllers -> domain/repositories, pgx, database/sql, pgxpool
internal/interface/restful/dto -> internal/domain, internal/usecase, internal/infrastructure, framework/driver SDKs
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

## Zap Logging

Use zap as the default structured logging backend for Go services, but add logs only where they improve diagnosis, auditability, or operational visibility.

Architecture rules:

- Use exactly one logging library/facade per project. Prefer `go.uber.org/zap`; do not mix zap with `log`, `slog`, logrus, or ad hoc `fmt.Println` logging.
- Initialize zap in `cmd`, `wire`, or `infrastructure/support/logger`; call `Sync()` during graceful shutdown when appropriate.
- Inject the logger or a narrow project logger contract through constructors. Do not create new zap loggers inside controllers, usecases, repositories, or helper functions.
- Keep `domain` pure: no zap imports and no logging in entities, value objects, filters, or pure domain services.
- If `usecase` code needs logs, prefer a narrow logger contract or the repository's existing project logger abstraction. Import zap directly there only when the repository already standardizes on direct zap injection.
- Use structured fields with stable keys, such as `operation`, `request_id`, `tenant_id`, `user_id`, `resource`, `resource_id`, `status`, `duration_ms`, and `error`.
- Never log secrets, tokens, passwords, raw authorization headers, private keys, full request/response bodies, large payloads, or personal data beyond the minimum identifier needed for diagnosis.
- When debugging a backend failure, capture reproducible logs to a local file such as `.arc/artifacts/<task>/logs/backend.log` or `tmp/logs/backend.log`. Do not rely only on terminal scrollback or memory.

Log these when business or operations benefit:

- Application startup, shutdown, migration/bootstrap outcomes, and configuration choices that are safe to disclose.
- Request boundary summaries in middleware, including method, path/route, status, latency, request ID, actor or tenant when available, and error class.
- Business state changes such as create/update/delete/approve/reject/publish/pay/refund/import/export, with actor, target resource, result, and idempotency key when relevant.
- Security and authorization events that require traceability, such as login failure throttling, permission denial on sensitive actions, tenant boundary rejection, or suspicious replay.
- External dependency calls and failures: database, cache, queue, object storage, payment, notification, recommendation, and third-party APIs. Include operation, target, retry count, duration, and sanitized error.
- Slow operations and retry exhaustion at the boundary that owns the timeout or retry policy.

Do not log these by default:

- Every successful read/list/detail query; rely on request middleware and metrics unless the query is business-critical.
- Normal validation failures, empty results, not-found results in expected flows, or branch decisions that are already returned to the caller.
- Both caller and callee for the same failure. Log once at the boundary with enough context; wrap and return errors elsewhere.
- Tight loops, per-row processing, pagination item details, health checks, readiness checks, or high-frequency background ticks unless sampled or rate-limited.
- Sensitive payload dumps added for debugging. Add targeted sanitized fields instead.

Level rules:

- `Debug`: development-only diagnostics or sampled details that can be disabled in production.
- `Info`: successful lifecycle events and meaningful business state changes.
- `Warn`: recoverable anomalies, retries, throttling, suspicious but handled security events, and slow operations above the project threshold.
- `Error`: failed operations that require attention and are not normal user input outcomes.
- `Fatal`/`Panic`: only at process boundaries when the service cannot continue safely.

Debugging evidence rules:

- For bug hunts and incident repair, run the failing path with logs persisted to files before large code edits. Capture command stdout/stderr, application logs, request IDs, relevant timestamps, and sanitized dependency errors.
- Add temporary diagnostic logs only when they test a concrete hypothesis. Keep them level-gated, structured, and removed or converted into useful permanent logs before completion.
- Prefer searching saved log files for exact error strings, request IDs, SQL states, external status codes, and stack frames. Do not keep rereading code without runtime evidence when the failure can be reproduced.
- Keep log artifacts out of commits unless the repository explicitly tracks sanitized evidence. Never persist secrets, tokens, raw authorization headers, full payloads, or personal data in debug files.

## Go Constants And Enums

For Go code, constants are compile-time semantic names, not C-style global macros. Prefer the narrowest useful scope, idiomatic names, untyped literals when flexibility is useful, and typed custom constants when modeling domain state.

- Name constants with Go `MixedCaps` / `mixedCaps`. Do not use `SNAKE_CASE` or `ALL_CAPS`; export only when another package should depend on the name.
- Prefer untyped constants for plain scalar literals such as sizes, limits, ratios, string labels, and numeric defaults. Add an explicit type only when it is part of the API contract, prevents invalid domain values, or is required by a dependency signature.
- MUST use Go standard-library constants instead of equivalent magic strings or numbers. Replace date/time layout literals with `time` constants when available, such as `time.DateTime`, `time.DateOnly`, `time.TimeOnly`, `time.RFC3339`, and `time.RFC3339Nano`.
- MUST use standard-library semantic constants for HTTP methods, status codes, file modes, TLS versions, crypto hashes, and other native package domains when the package exposes one.
- If no standard-library constant exists, define a local named constant at the narrowest useful scope instead of repeating the literal.
- Keep constants close to the behavior they describe. Do not create broad dumping-ground `constants.go` files; use `internal/constants` only for truly application-wide constants with multiple legitimate consumers.
- Model enum-like business states with a custom type plus typed `const` values, usually with a zero `Unknown` or default value. Do not pass naked `int` or `string` values as domain states.
- Implement `String()` for enum-like states used in logs, errors, metrics, serialization diagnostics, or operator-facing output. Add parse/validate helpers when values enter from transport or storage.
- For cross-service constants such as error codes, prefer versioned API contracts and generated code, such as Protobuf/OpenAPI enum definitions or a governed shared module. Do not copy constants between services manually.
- During review, flag equivalent literals, C-style names, naked enum parameters, and broad constant buckets as defects even when the code compiles.

## Helper And Shared Code

Follow this helper placement:

1. Keep module-specific usecase helpers inside the same `internal/usecase/<module>` package, using focused files such as `helpers.go`, `service_<feature>.go`, or `services.go` when the module already uses them.
2. Put application-layer shared types such as pagination or common business errors in `internal/usecase/shared`; do not duplicate them in each feature module.
3. Put REST request fragments in `internal/interface/restful/dto/requests` and response payloads in `internal/interface/restful/dto/responses`; controllers should return named DTO structs instead of declaring private DTO structs inline for runtime responses. Keep DTO packages schema-only and free of entity/usecase mapper constructors.
4. Define REST response bodies as named DTO composition structs in `dto/responses` with embedded `Base` and explicit concrete fields such as `Data User`, `Data []User`, `Page Page`, or `Cursor pagination.CursorResponse`; do not define ad hoc response structs inside controllers.
5. Keep controller helpers focused inside `internal/interface/restful/controllers` only when they are transport-boundary helpers. Move pure business helpers down into usecase.
6. Avoid vague `common`, `misc`, `tools`, or broad `utils` buckets. Extract only after real reuse and with a specific package purpose.

## Review Checklist

- Host repository patterns were inspected before applying the default backend architecture.
- Business logic is in `usecase/<module>`, not controllers, infrastructure, or `wire`.
- Business code depends on `domain/repositories` or explicit capability contracts instead of concrete infrastructure.
- Each interface is justified by a layer boundary, external capability, test seam, or multiple implementations.
- Concrete infrastructure is wired in `internal/wire` and implements domain or capability contracts.
- Usecase `contract.go` contains the exported controller-facing contract, not concrete service or adapter logic.
- List/query contracts return success with empty collections for no-data results; single-resource missing cases are represented intentionally as `not found` only when the product flow needs a missing-resource error state.
- Controllers and frontend DTOs can distinguish empty, not-found, permission-denied, validation error, and system error without relying on generic error text.
- Zap logging is initialized once, injected explicitly, structured with stable fields, and added only at useful business or operational boundaries.
- Backend debugging captures reproducible sanitized logs to a file before large edits when the failure is runnable or observable.
- Logs avoid secrets, raw payloads, duplicate caller/callee error records, normal validation noise, and high-frequency loop noise.
- Go constants use `MixedCaps` / `mixedCaps`, stay near their business context, prefer untyped literals unless typing is semantically required, and avoid broad `constants.go` buckets.
- Go code uses standard-library constants for native semantic literals, especially date/time layouts such as `time.DateTime`; raw equivalent strings are not accepted.
- Enum-like business states use custom typed constants with an `Unknown` or default zero value, and cross-service constants come from versioned generated contracts or governed shared modules.
- `helpers` are business-local unless proven reusable.
- Shared application helpers live in `internal/usecase/shared` or another focused package and do not import interface or infrastructure packages.
- Go REST responses use `dto/responses.Base` composition plus per-endpoint named response structs; `Data` uses concrete DTO types or slices, page and cursor metadata stay separate, no response body uses `any`, `interface{}`, `map[string]any`, `gin.H`, anonymous structs, or `Response[T any]`, and DTO packages do not contain entity/usecase mapping constructors.
- `ponytail` was read before coding, or its absence was reported before editing.
