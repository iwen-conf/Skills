---
name: arc:code-comment-conventions
description: Apply Chinese comment templates for controllers, interfaces, functions, structs, fields, and numbered steps.
---

# Code Comment Conventions

## Overview

Use this skill when writing or reviewing code comments that must follow the project convention for controller comments, interface comments, ordinary function comments, struct comments, field comments, and numbered implementation-step comments inside functions.

Prefer comments that explain intent, contract, parameters, return values, errors, and operational constraints. Do not add decorative comments or duplicate obvious code.

## When to Use

- Use when creating or updating controllers, service APIs, repository APIs, domain interfaces, ordinary functions, methods, structs, or DTOs.
- Use when reviewing code for comment style consistency.
- Use when a function has meaningful sequential steps that should be documented inline.
- Skip long templates for trivial private helpers only when the name and surrounding code already make behavior unambiguous and the project does not require full comments.

## In-Function Step Comments

Inside function bodies, describe meaningful execution steps with numbered `//` comments. Use imperative, outcome-oriented wording.

Format:

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

Rules:

- Start each step with `//1.`, `//2.`, `//3.` and keep numbering continuous in the local function.
- Use `调用“包名或对象名.函数名”获取/创建/更新/删除xxx` when the line calls another component.
- Comment a block of code, not every line. Merge adjacent trivial statements under one step.
- Renumber comments after inserting, deleting, or reordering steps.
- Avoid stale comments: the named callee, action, and object in the comment must match the code below it.

## Interface Or Contract Comments

Use these templates for API contracts, service interfaces, repository interfaces, domain interfaces, and request/response contract definitions.

For `type Xxx interface`, document only the interface responsibility. Do not aggregate all method parameters and return values on the interface type comment.

```go
// NovelTaxonomy 定义 novel_taxonomies 小说分类和标签字典集合仓储端口。
type NovelTaxonomy interface {
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

For DTOs or other request/response contract structs, document the contract itself and use field comments for field meaning.

Rules:

- `type Xxx interface` 的首行必须是 `// 接口名 定义xxx。`，只说明接口整体职责、边界或端口含义。
- Each exported interface method that represents a callable contract must have its own method comment immediately above the method.
- Put `参数` and `返回体` on the specific method comment, not on the parent `type Xxx interface` comment.
- Use `参数` and `返回体` headings without trailing colon.
- Use `// - 参数名 参数含义` and `// - 返回体名 返回体含义`; do not include type annotations unless the project explicitly needs them for disambiguation.
- Omit `参数` only when the method has no parameters. Omit `返回体` only when the method has no return body or result.
- Mention optional parameters directly in the parameter meaning, for example `筛选条件（可选）`.
- If an interface embeds another interface such as `Collection`, keep the embedded line uncommented unless it needs non-obvious behavior notes.

Example:

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

    // FindTaxonomies 查询分类和标签字典列表。
    //
    // 参数
    // - ctx 请求上下文
    // - query 分类和标签查询条件
    // 返回体
    // - taxonomies 匹配的分类或标签字典列表
    // - err 仓储访问失败时返回错误
    FindTaxonomies(ctx context.Context, query NovelTaxonomyQuery) ([]*entities.NovelTaxonomy, error)

    // SaveTaxonomy 保存分类或标签字典文档。
    //
    // 参数
    // - ctx 请求上下文
    // - taxonomy 分类或标签字典文档
    // 返回体
    // - err 仓储访问失败时返回错误
    SaveTaxonomy(ctx context.Context, taxonomy *entities.NovelTaxonomy) error

    // UpdateTaxonomy 更新指定类型的分类或标签字典文档。
    //
    // 参数
    // - ctx 请求上下文
    // - id 分类或标签业务主键 ID
    // - taxonomyType 字典类型
    // - patch 允许更新的字段
    // 返回体
    // - err 仓储访问失败时返回错误
    UpdateTaxonomy(ctx context.Context, id int64, taxonomyType string, patch Patch) error
}
```

## Ordinary Function Comments

Use this template for ordinary functions and methods that are not controller handlers and do not need the full interface contract template.

```go
// 函数名 函数作用
```

Rules:

- First line must be a single concise sentence: `// 函数名 函数作用`.
- Use the exact function or method name.
- Do not add parameter, return, error, or注意事项 sections for ordinary functions unless the surrounding project explicitly asks for a richer contract comment.

Example:

```go
// normalizeApprovalStatus 标准化审批状态
func normalizeApprovalStatus(status string) string {
    return strings.ToUpper(strings.TrimSpace(status))
}
```

## Struct Comments

Use this template for structs and their fields.

```go
// 结构体名 结构体中文含义
type xxx struct {
    字段名 类型 // 字段的中文含义
}
```

Rules:

- Add a struct-level comment immediately before the `type` declaration.
- Field comments are inline `// 字段的中文含义` comments after the field type and tags only when the field is part of a DTO, API contract, storage schema, config schema, or exported data model.
- Do not add inline comments to private dependency fields whose meaning is already clear from the field name and type. For injected repository dependencies, put the role in the field name with a `Repo` suffix, such as `novelCommentRepo repositories.NovelComment` or intentionally exported `NovelCommentRepo repositories.NovelComment`, instead of `comments repositories.NovelComment // 小说评论仓储`.
- Keep field comments concise and business-oriented.
- Do not repeat the field name as the whole field comment.
- Do not use comments to compensate for vague dependency names. Rename `comments`, `reports`, or `readingHistory` to names that expose the dependency role, such as `novelCommentRepo`, `reportRepo`, or `readingHistoryRepo`.

Example:

```go
// ApprovalRequest 审批请求
type ApprovalRequest struct {
    ID     string `json:"id"`     // 审批请求 ID
    Status string `json:"status"` // 审批状态
}
```

## Controller Comments

Use this template for HTTP controller or handler functions.

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

Rules:

- First line must include handler name, operation summary, and primary resource/model name when applicable.
- Keep `HTTP方法` and `API路径` exactly aligned with route registration.
- Keep all parameter groups in the template, even when a group is empty; use `//   -` for empty groups.
- Include authentication, tenant, trace, idempotency, or content negotiation headers under `Header参数`.
- If the controller accepts a body, describe the request DTO fields under the matching content type section.
- For list/search/dashboard endpoints, document the successful no-data response shape separately from error responses, such as `items=[]`, `total=0`, or an explicit empty-state field when the API uses one.
- Update the comment whenever route method, path, query/body shape, or auth requirement changes.

## Review Checklist

- Comment templates match the role: controller handler, interface/contract, ordinary function, struct/field, or internal implementation steps.
- Function names, route paths, parameter names, return types, and callee names are exact.
- Private dependency fields are not mechanically annotated with comments; their names carry the role, such as `novelCommentRepo repositories.NovelComment`.
- Optional parameters and empty controller parameter groups are explicitly marked.
- Successful empty/no-data responses are documented as normal returns, not as `错误`, unless the operation is a single-resource lookup where missing data is intentionally a `not found` error.
- Numbered step comments are continuous and describe blocks rather than single obvious statements.
- Comments explain contract and behavior without inventing guarantees that the code does not enforce.
