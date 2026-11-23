# Go Explicit Architecture, Reuse & Comment Conventions (`iwen-example` Baseline)

Applied by `arc-plan`, `arc-code`, and `arc-audit` on all Go (`*.go`, `go.mod`) repositories. Combines the **`iwen-example` Explicit Architecture** with mature Go engineering standards (Uber Go Style Guide & Google Go Style Decisions).

## 1. Project Organization & Explicit Architecture (`iwen-example` Baseline)

### 1.1 Dependency Direction (Strictly One-Way)

```text
cmd/<app> -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
                                                         \-> internal/infrastructure -> internal/domain
                                                         \-> pkg/<topic> (or internal/pkg/<topic>, zero business deps)
```

- `internal/wire`: Sole Composition Root assembling `Config -> Infrastructure -> Support -> Usecase -> RESTful`. Business packages never reach into global singletons or service locators.
- `internal/domain`: Pure business core (`entities/`, `valueobjects/`, `errs/`, `events/`). Zero imports of Gin/HTTP, GORM/SQL, Redis, or transport JSON DTOs.
- **Vocabulary Discipline**: Use `Contract`, `Params`, `Results`, `Service`, `Repository`, `Model`, and `Engine`. **Never** name packages, files, or types `Port` or `Adapter`.

### 1.2 Standard Directory & File Responsibilities

```text
├── cmd/<app>/main.go                           # Process bootstrap, signal handling, graceful drain only
├── internal/
│   ├── wire/                                   # Dependency assembly & lifecycle wiring
│   ├── interface/restful/
│   │   ├── routers/                            # Route registration & middleware mounting
│   │   ├── middlewares/                        # Auth, trace, recovery, rate-limit HTTP middlewares
│   │   ├── dto/requests/                       # Transport input structs + binding/format tags
│   │   ├── dto/responses/                      # Transport output structs + unified envelope
│   │   └── controllers/                        # Bind Request -> build Params -> call usecase.Contract -> write Response
│   ├── usecase/<business>/
│   │   ├── contract.go                         # Public Contract interface exposed to controllers/callers
│   │   ├── params.go                           # Named input structs (*Param); HTTP DTOs never enter usecase
│   │   ├── results.go                          # Named output structs (*Result); DB Models never leak out
│   │   ├── dependencies.go                     # Consumer-owned Repository / external capability interfaces
│   │   ├── main.go                             # Service struct + New(...) returning Contract
│   │   ├── service_<workflow>.go               # Cohesive business workflows, authz & TxManager orchestration
│   │   └── <workflow>_helpers.go               # Package-private business decomposition helpers
│   ├── domain/
│   │   ├── entities/                           # Pure domain aggregates & invariant methods
│   │   ├── valueobjects/                       # Typed IDs, statuses, money/time-range invariants
│   │   └── errs/                               # Domain error codes & classification
│   └── infrastructure/
│       ├── persistence/postgres/<business>/    # main.go + model.go + mapper.go + query_<topic>.go
│       └── support/<capability>/               # Pluggable technical capabilities: contract.go + engine.go + service.go + main.go
└── pkg/<topic>/ (or internal/pkg/<topic>/)     # Shared non-business pure/technical helpers + <topic>_test.go
```

- **Constructor Contract**: `New(...)` in `usecase/<business>/main.go` returns `Contract` directly (do not write `var _ Contract = (*Service)(nil)`).
- **`Engine` Boundary**: Reserve `Engine` (`engine.go`) strictly for genuine pluggable compute/storage engines (`cache`, `fulltext`, `authorization` bitmap/vector engine) under `internal/infrastructure/support/<capability>/` — never for ordinary CRUD repositories.
- **Read/Write Asymmetry**: Commands go through `domain/entities` + `mapper.go`; read-only list/report queries may project directly into read models inside persistence without redundant 3-layer struct copying.

## 2. File Size Hard Limit (`<= 1200` Lines per `.go` File)

- **Hard Red Line**: Every authored `.go` file (production and `*_test.go`) **MUST NOT exceed `1200` lines** (`gate.sh check` fails `go-file-lines` at `1201+` lines).
- **Proactive Split Thresholds** (split at `600-800` lines by responsibility, never wait until line 1199):
  - `controllers/*_controller.go`: target `<= 600` lines; split by sub-resource (`order_controller.go`, `order_refund_controller.go`).
  - `usecase/<business>/main.go`: target `<= 200` lines (`Service` struct + `New(...)` only); split workflows into `service_<workflow>.go` (`<= 500` lines) and `<workflow>_helpers.go`.
  - `persistence/.../<business>/`: split `main.go`, `model.go`, `mapper.go`, and `query_<topic>.go`.
  - `*_test.go`: split table-driven tests by workflow (`service_create_test.go`, `service_cancel_test.go`).
- Never bypass the limit by compacting multi-statement lines or stripping required comments.

## 3. Function Reuse & Shared Non-Business Utilities (`禁止重复造轮子`)

### 3.1 Mandatory Lookup Order Before Writing Any Helper

1. **Go Standard Library (Go 1.21+)**: Check `slices` (`Contains`, `IndexFunc`, `SortFunc`, `Compact`), `maps` (`Clone`, `Keys`, `Equal`), `cmp` (`Or`, `Compare`), `strings`, `strconv`, `filepath`, `time`, `crypto/*`, `errors` (`Is`, `As`, `Join`), and `golang.org/x/sync/errgroup` first. Hand-rolling loops for `containsString`, `minInt`, or `uniqueSlice` when stdlib covers it is forbidden.
2. **Repository Index (`arc idx`)**: Run `arc idx search <keyword>` and `arc idx symbol <name>` across `pkg/`, `internal/pkg/`, and `internal/infrastructure/support/` before writing any utility, validator, mapper, or middleware.
3. **Extract Generic Non-Business Helpers Immediately**:
   - Any domain-agnostic function (pagination offset/limit math, slice/map transforms absent from stdlib, string masking/normalization, crypto/HMAC/hash, UTC time-window helpers, safe JSON/I/O/retry helpers) **must be extracted** into a topic-named shared package: `pkg/<topic>` (cross-module/CLI reusable) or `internal/pkg/<topic>` (service-internal shared).
   - **Topic Naming (Uber/Google Go Standard)**: Name shared packages as clear single-word nouns describing capability (`pkg/paging`, `pkg/strx`, `pkg/timex`, `pkg/cryptox`, `pkg/httpx`). **Strictly forbid** catch-all `util`, `utils`, `common`, `misc`, or `helper` packages.
   - **Purity Invariant**: `pkg/<topic>` and `internal/pkg/<topic>` must never import `internal/domain`, `internal/usecase`, or `internal/interface`, must not hold mutable package-level state, and must ship focused table-driven unit tests (`<topic>_test.go`).
4. **Business-Scoped Reuse**:
   - Shared domain invariants -> methods on `internal/domain/entities` or `internal/domain/valueobjects`.
   - Stateful cross-usecase technical capabilities -> `internal/infrastructure/support/<capability>`.
   - Single-usecase private decomposition -> `internal/usecase/<business>/<workflow>_helpers.go` (promote on second caller).

## 4. Mandatory Comment Conventions (`函数外部契约注释 + 函数内部逐行编号注释`)

### 4.1 External Function Header Comment (Required on Every Function & Method)

Every Go function and method must carry the exact 3-part header block (`函数名 函数作用` / `请求体` / `响应体`) directly above `func`:

```go
// 函数名 函数作用
// 请求体
// 参数1 参数1意义
// 参数2 参数2意义
// 响应体
// 响应体参数1 响应体参数1含义
// 响应体参数2 响应体参数2含义
```

- **Parameter & Return Alignment**: Document every input parameter in declaration order under `// 请求体` and every return value in declaration order under `// 响应体` (use named return identifiers in documentation or signature; when a struct like `param CreateParam` or Gin `c *gin.Context` carries a structured payload, state the struct/context parameter and summarize or list its key business fields).
- **Empty Inputs or Outputs**: Keep the `// 请求体` and `// 响应体` headings intact and write `// 无` on the line beneath whichever section has zero parameters/returns.

### 4.2 Internal Step-by-Step Comment (`// 1.xxx` 一行注释、一行核心代码)

Inside functions, organize execution linearly using sequential numbered comments (`// 1. xxx`, `// 2. xxx`, `// 3. xxx`) paired directly with each atomic statement:

1. **One Step Comment -> One Atomic Code Statement**: Place `// N. <动作与业务意图>` immediately above each core line/call (keeping its immediate `if err != nil { return ... }` guard attached to that step rather than numbering bare `return err` branches).
2. **Concrete Intent Over Noise**: State *what invariant is checked or what collaborator is called and why* (e.g., `// 2. 调用 repo.GetByID 校验笔记存在性与租户归属`), never meaningless restatements (`// 定义变量 a`).
3. **Flat Control Flow & Sequential Renumbering**: Prefer guard clauses / early returns (max nesting depth `<= 3`). Whenever steps are inserted, merged, or removed, renumber `1..N` continuously with zero gaps or out-of-order numbers.

### 4.3 Canonical Go Example (`usecase/<business>/service_create.go`)

```go
// CreateNote 校验分类权限并创建持久化笔记记录
// 请求体
// ctx 请求上下文（承载超时控制与链路追踪信息）
// param 创建笔记参数（Title 标题、Content 正文、CategoryID 分类ID、OperatorID 操作人ID）
// 响应体
// result 创建后的笔记摘要（ID 笔记ID、CreatedAt 创建UTC时间）
// err 权限不足、分类不存在或持久化失败错误
func (s *Service) CreateNote(ctx context.Context, param CreateParam) (result NoteResult, err error) {
	// 1. 清洗标题首尾空白并校验必填长度约束
	title := strx.TrimCollapseSpace(param.Title)
	if title == "" {
		return NoteResult{}, errs.ErrInvalidNoteTitle
	}
	// 2. 查询目标分类并校验当前操作人具备写入权限
	category, err := s.categoryRepo.GetByID(ctx, param.CategoryID)
	if err != nil {
		return NoteResult{}, err
	}
	if !category.CanWrite(param.OperatorID) {
		return NoteResult{}, errs.ErrCategoryForbidden
	}
	// 3. 构造纯领域笔记实体并赋予 UTC 创建时间
	note := entities.NewNote(title, param.Content, category.ID, param.OperatorID, s.clock.NowUTC())
	// 4. 持久化笔记实体到仓储层
	saved, err := s.noteRepo.Create(ctx, note)
	if err != nil {
		return NoteResult{}, err
	}
	// 5. 组装用例层标准响应结果返回
	return NoteResult{ID: saved.ID, CreatedAt: saved.CreatedAt}, nil
}
```

## 5. Type Safety, Concurrency & SQL Invariants

1. **Type Safety**: Ban `any` / `interface{}` / `map[string]any` in `domain` and `usecase` signatures. Wrap IDs and statuses in distinct typed values; validate invariants at boundary construction.
2. **Concurrency & Context**: Propagate `context.Context` as first parameter; bound every goroutine lifetime with cancellation and error propagation (`errgroup`); never fire-and-forget background writes without shutdown drain.
3. **Transactions & SQL**: Bind multi-row invariants inside explicit `TxManager` transactions; guard state transitions with `WHERE status = expected` CAS checks and verify `RowsAffected()`.
