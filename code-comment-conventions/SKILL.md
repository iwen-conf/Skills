---
name: code-comment-conventions
description: Apply Chinese comment templates for functions, APIs/controllers, and numbered in-function steps.
---

# Code Comment Conventions

## Overview

Use this skill when writing or reviewing code comments that must follow the project convention for function/interface comments, controller comments, and numbered implementation-step comments inside functions.

Prefer comments that explain intent, contract, parameters, return values, errors, and operational constraints. Do not add decorative comments or duplicate obvious code.

## When to Use

- Use when creating or updating functions, methods, service APIs, repository APIs, domain interfaces, or controller handlers.
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

## Function And Interface Comments

Use this template for service methods, repository methods, domain APIs, exported functions, and interface methods that represent a reusable contract.

```go
// 函数名  功能简述
// 描述：详细说明该函数的用途和作用（如果简述已经足够，这里可以省略）
//
// 参数：
//   - 参数名（类型）：含义和使用说明
//   - 参数名（类型，可选）：说明（可省略的要标明）
//
// 返回值：
//   - 返回类型：说明
//   - 返回类型：说明
//
// 错误：
//   - 场景1：错误说明
//   - 场景2：错误说明
//   - 场景3：错误说明
//
// 注意事项：
//   - 使用限制 / 性能提示 / 并发安全说明（如有必要）
```

Rules:

- First line must be `// 函数名  功能简述`; keep two spaces between the function name and summary.
- Omit `描述` only when the first-line summary fully explains the purpose.
- Include `参数` for every non-obvious parameter. Mark optional parameters with `可选`.
- Include every return value, including `(value, error)` style returns.
- Include `错误` when the function can fail or return an error. If no error path exists, omit the section.
- Include `注意事项` only for real constraints such as authentication, transaction boundaries, idempotency, concurrency, performance, caching, or side effects.

Example:

```go
// GetApprovalRequest  获取审批请求详情
// 描述：根据审批请求 ID 查询单条审批请求，供业务页面展示当前审批状态和关联人员信息。
//
// 参数：
//   - ctx（context.Context）：请求上下文，用于超时控制和链路追踪
//   - id（string）：审批请求 ID
//
// 返回值：
//   - *ApprovalRequest：审批请求详情
//   - error：查询失败或记录不存在时返回错误
//
// 错误：
//   - 记录不存在：返回 ErrApprovalRequestNotFound
//   - 存储异常：返回底层存储错误
//
// 注意事项：
//   - 调用方需要保证 id 已完成基础格式校验
func GetApprovalRequest(ctx context.Context, id string) (*ApprovalRequest, error) {
    //1. 调用“repo.GetApprovalRequest”获取审批请求详情
    return repo.GetApprovalRequest(ctx, id)
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
- Update the comment whenever route method, path, query/body shape, or auth requirement changes.

## Review Checklist

- Comment templates match the function role: controller handler, reusable API/interface, or internal implementation steps.
- Function names, route paths, parameter names, return types, and callee names are exact.
- Optional parameters and empty controller parameter groups are explicitly marked.
- Numbered step comments are continuous and describe blocks rather than single obvious statements.
- Comments explain contract and behavior without inventing guarantees that the code does not enforce.
