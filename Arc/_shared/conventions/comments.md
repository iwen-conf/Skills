# Code Comment Conventions

Applied by arc:code in comment mode and whenever it writes code comments; checked by arc:audit.

## 1. Rules

1. Pick the template by role: HTTP controller/handler → §2; interface, usecase `Contract` or repository
   port → §3; ordinary function or method → §4; struct or field → §5; steps inside a function body → §6.
2. Comments state intent, contract, parameters, return values, errors or operational constraints. No
   decorative comments; never restate what the code already says.
3. Function names, route paths, parameter names, return types and callee names in comments are exact.
4. IF a private helper's name already makes its behavior unambiguous THEN use only the one-line §4 form or
   no comment.
5. Comment-only task: change comments only — no renames, no logic changes. Report vague names (for example
   dependency fields `comments`, `reports`, `readingHistory`) as rename suggestions. Rename only inside an
   authorized code-change scope.
6. Successful empty/no-data results are normal returns, never documented as `错误`, except single-resource
   lookups where missing data is intentionally `not found`.
7. IF route method, path, query/body shape or auth requirement changes THEN update the controller comment in
   the same change.
8. A deliberate simplification with a known ceiling uses the `debt:` comment from
   [coding-discipline.md](../coding-discipline.md) §2.
9. Not for README, release notes or other user-facing prose.

## 2. Controller / handler

```go
// ListApprovalRequests 列表查询审批请求 ApprovalRequest
// HTTP方法：GET
// API路径：/api/v1/app/approvalRequests
// 函数名：ListApprovalRequests
// 功能简述：分页返回符合条件的审批请求列表
//
// 描述：可用于查看人事相关业务的审批记录，例如志愿者调整、任职变更等，支持分页和过滤。
//
// 参数：
// Query参数
//   - limit: 单页返回记录条数（可选）
//   - cursor: 上一次响应中的 next_cursor，用于获取下一页
//   - filter: AIPS 风格过滤表达式，例如 status="PENDING"
// Params路径参数
//   -
// JSON参数（Content-Type: application/json）
//   -
// x-www-form-urlencoded参数（Content-Type: application/x-www-form-urlencoded）
//   -
// multipart/form-data参数（Content-Type: multipart/form-data）
//   -
// Header参数
//   - Authorization: Bearer <access_token>（需要鉴权）
```

1. The first line holds the handler name, the operation summary and the primary resource/model name when
   one exists.
2. `HTTP方法` and `API路径` match the route registration exactly.
3. Keep every parameter group, even empty ones; an empty group is `//   -`.
4. Authentication, tenant, trace, idempotency and content-negotiation headers go under `Header参数`.
5. A request body's DTO fields go under the matching content-type group.
6. List/search/dashboard endpoints document the successful no-data shape (`items=[]`, `total=0`, or the
   API's explicit empty-state field) separately from error responses.

## 3. Interface / contract

```go
// Contract 定义小说应用用例。
type Contract interface {
    // GetNovel 查询小说详情。
    GetNovel(ctx context.Context, param GetNovelParam) (*NovelResult, error)

    // ListNovels 查询小说列表。
    ListNovels(ctx context.Context, query ListNovelsParam) (*ListNovelsResult, error)
}
```

1. The `type Xxx interface` comment is `// 接口名 定义xxx。` and states only the interface's responsibility,
   boundary or port meaning; never aggregate method parameters or returns on it.
2. Each exported callable method has its own comment immediately above it.
3. `internal/usecase/<module>/contract.go` methods get one concise sentence. No `参数` / `返回体` blocks for
   ordinary `ctx`, `param`, `query`, `result` or `err` signatures; mass-added blocks such as `ctx 请求上下文`,
   `param xxx参数`, `result xxx结果`, `err 查询失败时返回错误` on every method are a defect.
4. `参数` / `返回体` blocks are only for repository/domain interfaces, external API contracts, or genuinely
   non-obvious parameter semantics, and sit on the method comment.
5. Block format: headings `参数` and `返回体` without a trailing colon; lines `// - 参数名 参数含义` and
   `// - 返回体名 返回体含义`; no type annotations unless the project requires them. Omit `参数` only when
   the method has no parameters, `返回体` only when it returns nothing. Mark optional parameters in the
   meaning, e.g. `筛选条件（可选）`.
6. An embedded interface line (e.g. `Collection`) stays uncommented unless it needs a non-obvious behavior
   note.

Repository port:

```go
// NovelTaxonomy 定义 novel_taxonomies 小说分类和标签字典集合仓储端口。
type NovelTaxonomy interface {
    Collection

    // FindTaxonomy 查询指定类型的分类或标签字典文档。
    //
    // 参数
    // - ctx 请求上下文
    // - id 分类或标签业务主键 ID
    // - taxonomyType 字典类型
    // 返回体
    // - taxonomy 匹配的分类或标签字典文档
    // - err 仓储访问失败时返回错误
    FindTaxonomy(ctx context.Context, id int64, taxonomyType string) (*entities.NovelTaxonomy, error)
}
```

## 4. Ordinary function or method

```go
// normalizeApprovalStatus 标准化审批状态
func normalizeApprovalStatus(status string) string {
    return strings.ToUpper(strings.TrimSpace(status))
}
```

1. One line: `// 函数名 函数作用`, with the exact function or method name.
2. No parameter, return, error or 注意事项 sections unless the project explicitly asks for richer contract
   comments.

## 5. Struct and field

```go
// 结构体名 结构体中文含义
type xxx struct {
    字段名 类型 // 字段的中文含义
}
```

```go
// ApprovalRequest 审批请求
type ApprovalRequest struct {
    ID     string `json:"id"`     // 审批请求 ID
    Status string `json:"status"` // 审批状态
}

// Bad: comments repeat the dependency role that the name should carry.
type BadService struct {
    comments       repositories.NovelComment        // 小说评论仓储
    readingHistory repositories.NovelReadingHistory // 阅读历史仓储
}

// Good: no inline comments are needed because the field names are explicit.
type Service struct {
    novelCommentRepo   repositories.NovelComment
    readingHistoryRepo repositories.NovelReadingHistory
}
```

1. The struct comment sits immediately before the `type` declaration.
2. Inline field comments go after the type and tags, only on fields of DTOs, API contracts, storage
   schemas, config schemas or exported data models; concise, business-oriented, never just the field name
   repeated.
3. Private dependency fields get no inline comment; the name carries the role with a `Repo` suffix. Never
   use a comment to compensate for a vague name (see §1 rule 5).

## 6. In-function steps

```go
func Example(ctx context.Context, id string) error {
    //1. 调用“xxx.函数名”获取xxx
    item, err := xxx.GetItem(ctx, id)
    if err != nil {
        return err
    }

    //2. 校验xxx是否满足业务条件
    if !item.Enabled {
        return ErrDisabled
    }

    //3. 调用“yyy.函数名”保存xxx
    return yyy.SaveItem(ctx, item)
}
```

1. Steps start `//1.`, `//2.`, `//3.` with continuous numbering inside the function; renumber after any
   insert, delete or reorder.
2. A step that calls another component reads `调用“包名或对象名.函数名”获取/创建/更新/删除xxx`.
3. Comment blocks, not lines; merge adjacent trivial statements under one step. Wording is imperative and
   outcome-oriented.
4. The callee, action and object named in a step match the code below it.
