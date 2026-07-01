# Backend Architecture Reference

Use this file as the detailed reference for the default backend architecture. Keep `SKILL.md` as the short operating contract and load this file when a task needs file-level naming, interface boundaries, transaction placement, or review checks.

## Directory Layout

```text
backend/
├── cmd/server/
├── configs/
├── internal/
│   ├── bootstrap/
│   ├── constants/
│   ├── domain/
│   │   ├── entities/
│   │   ├── events/
│   │   ├── filters/
│   │   ├── repositories/
│   │   └── services/
│   ├── usecase/<module>/
│   ├── interface/restful/
│   │   ├── controllers/
│   │   ├── dto/requests/
│   │   ├── dto/responses/
│   │   ├── middlewares/
│   │   └── router/routes/
│   ├── infrastructure/
│   │   ├── gateways/
│   │   └── support/
│   └── wire/
└── migrations/{up,down}/
```

## Dependency Direction

```text
cmd -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
internal/infrastructure -> internal/domain
```

Rules:

- `domain` must not import infrastructure, usecase, interface, Gin, pgx, Redis, Casbin, or other driver/framework packages.
- `domain/entities` must not import REST DTO packages or expose transport conversion methods such as `ToDTO`, `ToResponse`, or methods returning `dto/responses` types.
- `usecase` must not import Gin, `net/http`, pgx, `database/sql`, or interface-layer packages.
- `interface/restful/controllers` must not import `domain/repositories`, pgx, `database/sql`, or `pgxpool`.
- `wire` is allowed to import concrete infrastructure and assemble the object graph.
- `infrastructure` implements domain repository interfaces or capability contracts; business logic must not construct it directly.

## Layer Responsibilities

- `domain/entities`: Business objects with identity, lifecycle state, domain behavior, and invariants. Entity is not DTO and not ORM model. Put entity-local rules here when they protect consistency, such as status transitions, balance changes, freeze/unfreeze, or field changes with business validation. Do not put HTTP/DTO conversion methods on entities.
- `domain/repositories`: Repository interfaces consumed by usecases. Define business persistence capabilities, such as `FindByID`, `Save`, or `ListByOwner`. Do not mention SQL tables, Mongo collections, HTTP, pgx, GORM, or driver-specific types. Example: `type Course interface { List(...); GetByID(...); Create(...); Update(...) }`.
- `domain/services`: Pure domain services when a rule spans multiple entities or value objects and does not naturally belong to one entity. Keep them free of persistence, transport, logging, and workflow orchestration.
- `usecase/<module>`: Application workflows, authorization decisions that are application-specific, transaction orchestration, calls to repository ports, and calls to explicit external capability contracts. Controllers depend on the module `Contract`. Usecases coordinate entities; they should not become storage adapters or HTTP handlers.
- `interface/restful/controllers`: HTTP handlers. They bind/validate request input, authorize at the transport boundary when applicable, call usecase contracts, map errors, map entity/usecase results to response DTOs, and respond. They own transport mapping helpers when conversion is needed.
- `interface/restful/dto/requests`: Named request DTOs and reusable request fragments. Keep this package as transport schema only.
- `interface/restful/dto/responses`: Named response DTOs, response envelope base types, and harmless response constants. Keep this package as transport schema only; do not put entity/usecase-to-DTO mapper constructors, factories, or business helpers here.
- `infrastructure/gateways/persistence/postgres/models`: Database row/table models and storage-shape conversion. ORM/database models are not domain entities.
- `infrastructure/gateways/persistence/postgres/repository`: Postgres implementations of `domain/repositories`. Repositories translate between storage models and domain entities.
- `infrastructure/gateways/<capability>`: External capability gateways such as notification, storage, payment, and recommendation. They adapt external protocols behind contracts.
- `infrastructure/support/<capability>`: Cross-cutting infrastructure such as authorization, cache, logger, and security.
- `constants`: Truly application-wide constants with multiple legitimate consumers. Do not use this as a dumping ground for module-specific business limits, labels, or state.
- `wire`: Repository set, usecase set, controller construction, bootstrap, seeds, reset, and application lifecycle.
- `bootstrap`: Startup initialization that runs after migrations and repository construction.

## DTO, Entity, And Repository Model Boundaries

Keep API contracts, domain models, and persistence models separate.

### DTO

DTO is the external communication contract.

- Place request DTOs in `internal/interface/restful/dto/requests`.
- Place response DTOs in `internal/interface/restful/dto/responses`.
- Allow JSON/form/query binding tags and API-facing field names.
- Do not add business behavior, domain invariants, repository calls, persistence tags, or entity/usecase mapper constructors.
- DTOs may change with API contracts without forcing domain or storage shape changes.

### Entity

Entity is the domain model.

- Place entities in `internal/domain/entities`.
- Model identity, lifecycle state, domain behavior, and invariants.
- Put entity-local behavior here when it protects consistency, such as `Cancel`, `Pay`, `Freeze`, `ChangeEmail`, or validated status transitions.
- Do not add JSON/API response conversion, ORM tags, SQL/Mongo-specific fields, repository calls, logging, or framework imports.
- Do not use entities as direct database row structs or response DTOs.

### Repository Model

Repository Model is the persistence mapping shape.

- Place storage models in infrastructure persistence packages such as `internal/infrastructure/gateways/persistence/postgres/models` or the equivalent local store package.
- Allow ORM/DB tags, table/collection field names, nullable storage fields, denormalized columns, and storage-optimized shapes.
- Do not add business behavior or domain invariants.
- Convert repository models to entities before returning from repository implementations, and convert entities to repository models before persisting.

Data flow:

```text
HTTP request -> request DTO -> usecase params -> entity/usecase workflow -> repository interface -> repository model -> database
database -> repository model -> repository interface -> entity/usecase result -> response DTO -> HTTP response
```

Conversion responsibility:

- HTTP boundary converts REST request DTOs to usecase params and usecase/entity results to REST response DTOs. REST DTOs must not enter `usecase`.
- Usecases convert application params/results into entity operations and coordinate entities, domain services, repositories, and transactions.
- Domain repository interfaces accept and return entities or domain value objects, not repository models. A method such as `FindByID` returns `*entities.Order`; `Save` accepts `*entities.Order`.
- Infrastructure repository implementations convert repository models to entities on reads and entities to repository models on writes.
- Repository models are private to infrastructure persistence implementations. Do not expose them from domain repository interfaces, usecases, or controllers.
- Read-optimized projections may bypass entities only for explicit query/read-model paths such as CQRS, reports, dashboards, or SSR view reads. Those projections must not enter domain behavior or write workflows.

Mapper placement:

- Do not create broad packages such as `internal/mapper` that import REST DTOs, domain entities, and repository models together.
- Do not put `DTO -> Entity`, `Entity -> DTO`, `Entity -> Model`, and `Model -> Entity` in one `order_mapper.go`. That file crosses transport, domain, and persistence boundaries.
- Put REST DTO mapping beside the HTTP boundary, for example `internal/interface/restful/controllers/order_mapper.go` or a focused mapper file in the same transport package.
- Put repository model mapping beside the persistence implementation, for example `internal/infrastructure/gateways/persistence/postgres/repository/order_mapper.go`.
- If two conversions share field assignments, prefer a tiny unexported helper inside the owning boundary package. Do not centralize mappings across layers to avoid a few repeated field names.
- Accept small repeated field assignments across HTTP and persistence mappers when they protect dependency direction.
- If repeated mapping code encodes real business normalization, defaulting, validation, or invariant construction, move that behavior to domain constructors, value objects, or entity methods. Do not hide business rules in a cross-layer mapper.

Forbidden shortcuts:

- Do not use DTOs as entities.
- Do not use entities as ORM/database models.
- Do not return repository models from domain repository interfaces, usecases, or controllers.
- Do not put `ToDTO`, `ToResponse`, `TableName`, ORM hooks, or JSON wire-shape methods on domain entities.
- Do not put domain rules in repository models or DTOs.
- Do not use a global mapper package as a shortcut around dependency direction.

## Usecase Module Shape

```text
internal/usecase/<module>/
├── contract.go
├── main.go
├── params.go
├── results.go
├── errors.go          # optional
├── service.go         # optional
├── services.go        # optional helper contracts/configs
└── service_<feature>.go
```

Conventions:

- `contract.go` exposes `type Contract interface { ... }` for controller-facing operations.
- `main.go` defines `type Service struct { ... }` and `New(...) Contract`.
- `params.go` contains input/request parameter structs for usecase methods.
- `results.go` contains application result/view structs returned to controllers.
- `service_<feature>.go`, `service.go`, or `services.go` hold focused workflow implementations or helper contracts when the module is too large for `main.go`.
- `internal/usecase/shared` holds shared application types such as pagination and common errors.
- Keep `main.go` small; split workflows into focused service files when needed.

## Interface Design

Use explicit contracts at real boundaries:

- Usecase boundary: `internal/usecase/<module>/Contract`.
- Repository boundary: `internal/domain/repositories.<Entity>`. Repository interfaces speak domain language and accept/return entities or domain value objects, never infrastructure repository models.
- External capability boundary: package-local `Contract` interfaces in `infrastructure/gateways/*` or `infrastructure/support/*`.
- Engine boundary for swappable low-level implementations: `Engine` interfaces in support/gateway packages such as cache or storage.

Avoid interfaces for private helpers or same-package pure functions. Add an interface only for a layer boundary, external capability, test seam, or multiple implementations.

## Constructor And Naming Patterns

Common constructor patterns:

- Usecase: `func New(...domain repositories..., options/config...) Contract`.
- Repository: `func New<Course>Repository(pool *pgxpool.Pool) repositories.Course` or `New<Course>DomainRepository`.
- Support/gateway service: `func NewService(engine Engine, ...) (Contract, error)`.
- Wire: `newRepositorySet`, `newUsecaseSet`, `newControllers`, `NewApplication`.

Common type names:

- `Contract` for the exported service boundary of a usecase or capability package.
- `Service` for the concrete implementation behind a `Contract`.
- `Engine` for low-level pluggable adapters inside infrastructure support/gateway packages.
- `<Entity>Repository` or `<Entity>DomainRepository` for concrete Postgres repository structs.
- `Params` for usecase inputs and `Result` for usecase outputs.

## Transaction Boundary

Put transaction orchestration in usecase logic through `domain/repositories.TxManager`.

- `domain/repositories/transaction.go` defines `Tx`, `TxFunc`, and `TxManager`.
- Postgres implements it in `infrastructure/gateways/persistence/postgres/repository/transaction.go`.
- Repositories execute SQL and can reuse transaction context; they should not start ad hoc transactions for business workflows.
- Controllers never see transactions.

## Controller Rules

Controllers should:

- Use named request and response DTOs.
- Use mapper helpers only when conversion is nontrivial or the repository already has that pattern.
- Keep mapper helpers in the controller package or another HTTP-boundary package, not in `dto/requests` or `dto/responses`.
- Return successful empty list/query results as success responses with empty collections and pagination metadata.
- Keep HTTP status and business error mapping explicit at call sites when the flow is small.
- Avoid `gin.H` for runtime response bodies; named DTO structs are preferred.
- Keep business decisions, transaction logic, SQL, and repository calls out of controllers.

## Response DTO Composition

Use composition for every Go REST response body. Keep the response DTO package as the wire contract and avoid generic response catch-alls.

Define the shared base envelope in `internal/interface/restful/dto/responses/base.go`. If the host repository already uses singular `dto/response`, keep that established package name. Preserve the repository's established base type name, such as `Base`, `BaseResponse`, or `ResponseBase`.

```go
package responses

type Base struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

var SuccessBase = Base{
    Success: true,
    Message: "ok",
}
```

For each endpoint, define a response-specific DTO. Embed `Base`; do not repeat `Success` and `Message` fields in every response.

```go
import "github.com/iwen-conf/utils-pkg/pagination"

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

Use response structs directly at the HTTP boundary:

```go
c.JSON(http.StatusOK, responses.UserListResp{
    Base: responses.SuccessBase,
    Data: users,
})
```

Page-number pagination, offset pagination, and cursor pagination are different contracts:

- Page-number pagination exposes `page`, `page_size`, and `total`; use it when the UI needs total count or jump-to-page behavior.
- Offset pagination exposes `offset`, `limit`, `total`, and `has_more`; when the API exposes offset/limit, use `github.com/iwen-conf/utils-pkg/pagination.OffsetRequest` and `OffsetResponse`.
- Cursor pagination exposes `cursor`/`limit` on request and `next_cursor`, `prev_cursor`, `has_more` on response; use `pagination.CursorRequest`, `pagination.CursorResponse`, and `pagination.NewHMACCodec` for signed opaque cursors. Cursor responses do not expose `page` or `total`.

Strict rules:

- Every runtime response body must be a named struct in the response DTO package, such as `GetCourseResponse`, `ListCoursesResponse`, or `DeleteCourseResponse`.
- Split shared response schema by responsibility: `base.go` contains only the base envelope and base constants/helpers, `meta.go` contains shared response metadata such as `Meta`, `Pagination`, cursor, or page structs, and feature files such as `activity_category.go` contain only that resource's DTOs and endpoint response structs.
- Do not put `BaseResponse`, `Meta`, `Pagination`, and feature DTOs in one feature response file. Shared schema belongs in shared files.
- The `Data` field type must be a concrete DTO type or slice of one, such as `Data User` or `Data []User`. Do not use `Data any`, `Data interface{}`, `Data map[string]any`, or inline `struct{...}`.
- Do not use `gin.H`, `map[string]any`, anonymous structs, or generic catch-all envelopes such as `Response[T any]` for runtime response bodies.
- Keep page metadata and cursor metadata as separate top-level response fields, not hidden inside `Data`.
- DTO packages must not import `internal/domain`, `internal/usecase`, repositories, database models, Gin, or database drivers for mapping.
- Domain entities must not import `dto/responses` or expose response conversion methods. Do not add methods like `func (a ActivityCategory) ToDTO() responses.ActivityCategoryDTO`; that reverses the dependency direction.
- Do not add constructors, factories, or mapper helpers only to satisfy this rule. Use direct struct literals unless the repository already has a helper pattern.
- If conversion from entities or usecase results is nontrivial, place it at the HTTP boundary that owns the transport contract, usually `internal/interface/restful/controllers` or a focused mapper file in that package. Do not add functions like `responses.NewUser(entity)` or `responses.NewUserList(usecaseResult)`.

## Infrastructure Rules

Persistence:

- Put table models in `postgres/models`.
- Put SQL repository implementations in `postgres/repository`.
- Return domain entities from repositories, not database models.
- Keep one physical table concept aligned to one repository interface/implementation where possible.
- Do not create repository interfaces for projections that have no physical persistence boundary; query through existing real repositories or usecase logic.

Support/gateway wrappers:

- Use `contract.go`, `engine.go`, `service.go`, and `main.go` when a capability has a service boundary and one or more engines.
- Validate required engines in `NewService(...)` and return constructor errors; do not repeat "engine not configured" checks in every method.

## Logging Rules

Use zap as the default structured logger and keep logging as an explicit observability design choice.

- Initialize zap in `cmd`, `wire`, or `infrastructure/support/logger`; inject it or a narrow project logger contract through constructors.
- Keep `domain` free of logging imports. In `usecase`, log only business-significant state changes, security-sensitive decisions, idempotency conflicts, or workflow failures that need traceability.
- Prefer centralized request logging middleware for HTTP request summaries. Do not duplicate the same successful request log in every controller.
- Repositories and gateways may log slow operations, retries, retry exhaustion, and external dependency failures with sanitized structured fields.
- Use stable fields: `operation`, `request_id`, `tenant_id`, `user_id`, `resource`, `resource_id`, `status`, `duration_ms`, and `error`.
- Do not log normal validation failures, expected empty/not-found results, every successful read query, per-row loop details, health checks, secrets, tokens, raw authorization headers, full payloads, or personal data beyond minimal identifiers.
- Log each failure once at the owning boundary. Wrap and return errors in lower layers instead of producing duplicate caller/callee logs.

## Debugging Log Evidence

When a backend failure is reproducible or observable, preserve runtime evidence in files before broad inspection or edits.

- Capture application logs, command stdout/stderr, request IDs, timestamps, stack frames, SQL states, external status codes, retry counts, and sanitized dependency errors into a local artifact path such as `.arc/artifacts/<task>/logs/backend.log` or `tmp/logs/backend.log`.
- Use project-native logging configuration when available. For local commands, redirect or tee output to a file so the evidence can be searched and rechecked.
- Search saved logs for exact failure strings and correlation IDs before changing code. Avoid long code-reading loops when the failing path can provide runtime evidence.
- Add temporary diagnostic logs only to test a concrete hypothesis. They must be structured, level-gated, sanitized, and removed or promoted into useful permanent observability before the fix is complete.
- Do not commit raw debug logs unless the repository explicitly tracks sanitized evidence. Never store secrets, tokens, raw authorization headers, full payloads, or personal data in debug artifacts.

```bash
mkdir -p .arc/artifacts/debug/logs
go test ./... 2>&1 | tee .arc/artifacts/debug/logs/go-test.log
go run ./cmd/server 2>&1 | tee .arc/artifacts/debug/logs/server.log
```

## Constant And Enum Rules

Use Go constants as compile-time semantic names. They should clarify business meaning, preserve type safety, and avoid repeated literals without becoming global macro buckets.

Naming and scope:

- Use Go `MixedCaps` / `mixedCaps`. Do not use `SNAKE_CASE`, `ALL_CAPS`, or package prefixes inside the name when the package already gives context.
- Export a constant only when another package should compile against it. Keep private constants lower-case and close to the behavior they configure.
- Prefer constants beside the code that uses them: entity state constants in `domain/entities`, workflow limits in the owning `usecase/<module>`, transport literals in the REST package, and integration-specific constants in the gateway/support package.
- Use `internal/constants` only for stable application-wide constants that are legitimately shared across several layers or modules. Do not create a catch-all `constants.go` for unrelated values.
- Replace repeated magic numbers and strings with a named constant that expresses business meaning, such as `MaxBatchSize`, `DefaultPageSize`, or `TokenExpirySkew`.

Typed and untyped constants:

- Prefer untyped constants for plain scalar literals when the value can safely adapt to context, such as numeric limits, ratios, string labels, and pure compile-time defaults.
- Add an explicit type when the type is part of the API contract, prevents invalid domain values, is required by a dependency signature, or models enum-like state.
- Use standard-library constants whenever they already represent the value: `time.DateTime`, `time.DateOnly`, `time.TimeOnly`, `time.RFC3339`, `http.MethodGet`, `http.StatusOK`, file modes, TLS versions, crypto hashes, and similar semantic constants.

```go
const (
    MaxBatchSize = 50
    defaultLimit = 20
)

func list(limit int32) {
    if limit > MaxBatchSize {
        limit = MaxBatchSize
    }
}
```

Enum-like domain states:

- Define a custom type for business states. Do not accept naked `int` or `string` parameters when the value represents a closed domain set.
- Reserve the zero value for `Unknown`, `Unspecified`, or a safe default that matches the product behavior.
- Use typed constants with `iota` when the underlying value is internal. Use explicit string values when they are persisted or part of a public wire contract.
- Implement `String()` for states used in logs, errors, metrics, CLI output, or operator-facing diagnostics.
- Add parse and validation helpers at boundaries where values enter from HTTP, queues, storage, or external APIs.

```go
type OrderStatus int

const (
    OrderStatusUnknown OrderStatus = iota
    OrderStatusCreated
    OrderStatusPaid
    OrderStatusShipped
    OrderStatusCompleted
)

func (s OrderStatus) String() string {
    switch s {
    case OrderStatusCreated:
        return "Created"
    case OrderStatusPaid:
        return "Paid"
    case OrderStatusShipped:
        return "Shipped"
    case OrderStatusCompleted:
        return "Completed"
    default:
        return "Unknown"
    }
}
```

Cross-service constants:

- Treat cross-service error codes, event types, and state values as API contracts, not local implementation details.
- Prefer Protobuf/OpenAPI definitions with generated code when the values cross service boundaries.
- In a monorepo, a governed shared module is acceptable only when it has version ownership and compatibility rules. Services must not copy-paste constants into private packages.
- Keep transport or wire-code enums separate from internal domain enums when their lifecycle differs. Map between them at the interface/gateway boundary.

## File Budgets And Naming

- `controllers/*` <= 600 lines.
- `usecase/*/main.go` <= 200 lines.
- `usecase/*/service_*.go` <= 400 lines.
- No Go file may contain more than two unexported/private `func` declarations. When an original file reaches two private functions, create a sibling helper file named from the original basename plus `_helpers.go`, such as `order.go` -> `order_helpers.go`, and move the private helpers there. Helper files follow the same two-private-function limit.
- Use plural package names where applicable: `entities`, `repositories`, `services`, `events`, `filters`, `controllers`, `middlewares`, `models`, `requests`, `responses`.
