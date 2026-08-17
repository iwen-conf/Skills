# Controller Comments

Load only when commenting HTTP controller or handler functions.

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
