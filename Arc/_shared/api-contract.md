# Markdown API Contract

Read before adding, changing or documenting a project API. Server and client implement and accept
against one reviewable Markdown contract.

## 1. Format and ownership

1. API docs are `.md` files in the project's existing API docs directory. Without one, use
   `docs/02-API记录/` with a `README.md` that indexes the module or endpoint docs.
2. Express the contract with headings, field tables, behavior tables and acceptance checklists.
   Never substitute `.json`, OpenAPI JSON, JSON Schema or whole JSON code blocks for the
   description. Nested fields use field paths with parent/child notes; sample values go in table
   cells.
3. The in-repo Markdown is the contract baseline for development, integration and acceptance.
   - Do not call `apifox-cli`, create Apifox projects, log in, sync or export by default.
   - Use Apifox only when the user explicitly asks for platform sync; it is a downstream copy of
     the Markdown. An existing Apifox project is not a reason to sync.
   - When an old platform definition disagrees with the doc, check the implementation and the
     requirement, update the Markdown first, then sync only if asked. A platform record never
     overrides the Markdown.
4. This is a documentation-format rule only: wire encoding, existing machine-generated files and
   tool inputs/outputs keep following the project protocol. Writing docs in Markdown changes no
   runtime behavior.
5. Before writing the contract, locate the current routes, DTOs, callers, error definitions and
   tests with [code-intel.md](code-intel.md) (§4 rows 4–5 for a change). Domain fields, roles and
   enums cite real definitions from project contracts, code or user requirements
   ([execution-truth.md](../../docs/execution-truth.md) §4). Mark unknown items `待确认`; they block
   the implementation that depends on them.
6. Reuse the existing module doc or endpoint section; never write a second description of the same
   endpoint. Write the expected behavior before implementation; on change, update the doc,
   callers and affected tasks together; on delivery, add the verification evidence.
7. Before delivery is claimed, the field tables and behavior rows match the routes, DTOs and
   consumers in code.

## 2. Minimum content

| Item | Must state |
| --- | --- |
| Locator | business purpose, requirement source, method and path, callers, backend implementation location |
| Auth boundary | authentication, allowed roles or permissions, resource-ownership check; all from project definitions |
| Request fields | location (path, query, header, body), field path, type, required, nullable, default, constraints, sample value |
| Response fields | field path, type, always present, nullable, meaning, unit or time format, enum source, sample value |
| Behavior and states | actual semantics for success, empty data, invalid parameters, unauthenticated, forbidden, not found, business conflict, system failure |
| Business rules | validation and state changes; for writes: side effects, transaction boundary, idempotency or concurrency constraints; for lists: pagination, sorting, filtering |
| Compatibility and dependencies | affected callers, change points, prerequisites, linked frontend and backend tasks |
| Verification | scenario, command or action, expected result, actual evidence, verification environment |

Fill the table for the actual endpoint. For a capability that does not apply, write why. Never
invent pagination, idempotency keys, roles, error codes or API versions to fill a row.

## 3. Markdown template

Replace every prompt before delivery. Each endpoint gets its own section; a complex endpoint may
get its own file linked from the module index.

```markdown
# 模块接口

## 契约依据

- 需求/设计：链接到当前文档及章节。
- 当前实现与调用点：已检索确认的路径、符号。
- 契约状态：草案 / 已对齐 / 已验证；写明依据，文档已写不等于接口已实现。
- 关联任务：后端任务链接、前端任务链接；仅单端时注明实际范围。

## 接口名称

### 基本信息

| 项目 | 约定 |
| --- | --- |
| 业务目的 | 调用后完成什么行为 |
| 方法与路径 | 项目已确认的方法、路径 |
| 认证与权限 | 认证要求、权限来源、资源归属规则 |
| 调用端 | 页面、客户端或服务调用点 |

### 请求字段

| 位置 | 字段路径 | 类型 | 必填 | 可空 | 默认值 | 约束/含义 | 示例值 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 按实际填写 | 项目字段 | 项目类型 | 是/否 | 是/否 | 无或实际值 | 边界、单位、枚举来源 | 脱敏值 |

### 响应字段

| 字段路径 | 类型 | 必有 | 可空 | 含义与约束 | 示例值 |
| --- | --- | --- | --- | --- | --- |
| 项目字段 | 项目类型 | 是/否 | 是/否 | 单位、时间格式、枚举来源 | 脱敏值 |

### 行为与错误

| 场景 | HTTP 状态/业务结果码 | 返回语义 | 调用端行为 |
| --- | --- | --- | --- |
| 正常 | 按项目合同填写 | 业务结果 | 对应成功状态 |
| 无数据（适用时） | 成功状态 | 空集合及约定分页信息 | 展示空态 |
| 参数不合法 | 按项目合同填写 | 字段错误与原因 | 展示可纠正提示 |
| 未认证/无权限 | 分别填写 | 区分身份与权限失败 | 按项目约定处理 |
| 资源不存在/业务冲突 | 分别填写 | 明确业务语义 | 按项目约定处理 |
| 系统失败 | 按项目合同填写 | 类型化错误与可追踪信息 | 展示错误态 |

### 业务规则与兼容性

- 校验、状态变化、分页/排序/筛选：按实际填写。
- 写入副作用、事务、幂等与并发：按实际填写，不适用则说明原因。
- 变更点及受影响调用端：写明是否需要同步修改，链接对应任务。

### 验证场景

| 场景 | 前置条件与输入 | 命令/操作 | 预期结果 | 判定与证据 |
| --- | --- | --- | --- | --- |
| 本接口的具体行为 | 真实条件、脱敏字段值 | 项目命令或操作步骤 | 可观察结果 | PASS / FAIL / NOT_RUN(原因)，附证据链接 |
```

## 4. Task linkage

- Contract alignment is a prerequisite of every implementation task that depends on it. A large
  contract change gets its own task; a simple one may live inside the backend task.
- Frontend and backend tasks each link the same contract section and name the fields, behaviors
  and readiness conditions they depend on. "Wait for the backend" or "integrate later" is not a
  dependency.
- The frontend may build pages, states and mock verification against an aligned contract. A task
  whose acceptance needs the real API stays `[?]` while only mocks are verified. A passing backend
  endpoint does not complete the frontend task.
- When the shared response envelope, error semantics or auth conventions change, list every
  affected endpoint and caller and update their tasks; updating one sample endpoint is not enough.
