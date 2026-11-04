# Go Backend Structure

Companion to [go-backend.md](go-backend.md); section numbers continue that file's numbering. Read when
creating a module, wiring the composition root, adding mappers or naming constructors. Comments:
[comments.md](comments.md); API contracts: [api-contract.md](../api-contract.md).

## 2. Layout

```text
backend/
├── cmd/server/
├── configs/
├── internal/
│   ├── bootstrap/
│   ├── constants/
│   ├── domain/{entities,events,filters,repositories,services}/
│   ├── usecase/<module>/
│   ├── interface/restful/{controllers,dto/requests,dto/responses,middlewares,router/routes}/
│   ├── infrastructure/{gateways,support}/
│   └── wire/
└── migrations/{up,down}/
```

1. `internal/domain` contains only `entities`, `events`, `filters`, `repositories`, `services`. Never create
   `domain/gateways`, `domain/ports`, `domain/adapters` or `domain/clients`.
2. Package names are plural (`entities`, `controllers`, `middlewares`, `models`, `requests`, `responses`).

## 5. Models, conversion and mappers

```text
HTTP request -> request DTO -> usecase params -> entity/usecase workflow -> repository interface -> repository model -> database
database -> repository model -> repository interface -> entity/usecase workflow -> usecase result -> response DTO -> HTTP response
```

1. Controllers convert request DTOs to usecase params and named usecase results to response DTOs. REST DTOs
   never enter `usecase`.
2. Repository implementations convert model → entity on reads and entity → model on writes. Repository
   models never leave infrastructure persistence packages.
3. Read projections may bypass entities only on explicit read paths (CQRS, reports, dashboards, SSR view
   reads) and never enter domain behavior or write workflows.
4. Mappers live beside their boundary: REST mapping in the controllers package (`controllers/order_mapper.go`),
   model mapping in the persistence package (`postgres/repository/order_mapper.go`). Never in `dto/*`, never
   a cross-layer `internal/mapper` package, never one file mixing DTO, entity and model conversions.
5. Add a mapper helper only when conversion is nontrivial or the repository already has that pattern;
   otherwise use struct literals. Accept small repeated field assignments across boundaries; share them
   only through a tiny unexported helper inside one boundary package.
6. IF repeated mapping encodes business normalization, defaulting, validation or invariants THEN move it
   into domain constructors, value objects or entity methods.

## 6. Usecase module

```text
internal/usecase/<module>/
├── contract.go          # type Contract interface { ... } — controller-facing operations
├── main.go              # type Service struct; func New(...) Contract
├── params.go            # method inputs
├── results.go           # named results returned to controllers
├── errors.go            # optional
├── service.go           # optional
├── services.go          # optional helper contracts/configs
└── service_<feature>.go # focused workflows
```

1. `Contract` methods return named result types, never raw entities.
2. `internal/usecase/shared` holds shared application types (pagination, common errors).

## 8. Naming and constructors

| Kind | Pattern |
|---|---|
| Usecase | `func New(...domain repositories..., options/config...) Contract` |
| Repository | `func New<Course>Repository(pool *pgxpool.Pool) repositories.Course` or `New<Course>DomainRepository`; struct `<Entity>Repository` / `<Entity>DomainRepository` |
| Support/gateway | `func NewService(engine Engine, ...) (Contract, error)` |
| Types | `Contract` boundary · `Service` implementation · `Engine` pluggable adapter · `Params` / `Result` |
| Injected repository fields | `Repo` suffix: `novelCommentRepo repositories.NovelComment` |
| Composition root / Wire | `InitPostgres`, `InitMongo`, `InitCacheService`, `InitRepositories`, `InitGateways`, `InitUseCases`, `InitControllers`, single `InitAuthUseCase`, top-level `InitApplication`; lifecycle `Start` / `Stop` / `HealthCheck` |
| Other prefixes | `NewXxx` ordinary package constructor · `EnsureXxx` idempotent constraint · `SeedXxx` fixed seed · `RegisterXxx` registration |
| Wire files | `providers.go`, `repositories.go`, `gateways.go`, `usecases.go`, `controllers.go`, `application.go`, `lifecycle.go` |

## 10. Controllers

1. Use named request and response DTOs ([go-type-safety.md](go-type-safety.md) §3).
2. Successful empty list/query results are success responses with empty collections; include pagination
   metadata only when the API contract is paginated.
3. IF the flow is small THEN map HTTP status and business errors explicitly at the call site.

## 12. Debug evidence

1. IF a backend failure is reproducible or observable THEN, before broad reading or edits, save logs,
   stdout/stderr, request IDs, stack frames, SQL states, external status codes and retry counts to
   `.arc/artifacts/<task-slug>/logs/` (`go test ./... 2>&1 | tee <that dir>/go-test.log`), then search them
   for the exact failure string and correlation IDs.
2. Temporary diagnostic logs test one named hypothesis; they are structured, level-gated, sanitized, and
   removed or promoted to permanent observability before completion.

## 14. File budgets and helpers

1. `controllers/*` ≤ 600 lines; `usecase/*/main.go` ≤ 200; `usecase/*/service_*.go` ≤ 400.
2. A new or changed file inside the authorized module or migration has at most two private `func`
   declarations. Move helpers to `<original>_helpers.go`; IF more groups remain THEN split by cohesive
   responsibility (`order_validation_helpers.go`, `order_mapping_helpers.go`), each within the same limit.
3. Never migrate unrelated legacy files to meet these budgets; never create numbered helper fragments
   without a responsibility.
