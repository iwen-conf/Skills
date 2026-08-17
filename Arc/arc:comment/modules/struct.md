# Struct And Field Comments

Load only when commenting structs and their fields.

```go
// 结构体名 结构体中文含义
type xxx struct {
    字段名 类型 // 字段的中文含义
}
```

Rules:

- Add a struct-level comment immediately before the `type` declaration.
- Field comments are inline `// 字段的中文含义` comments after the field type and tags only when the field is part of a DTO, API contract, storage schema, config schema, or exported data model.
- Do not add inline comments to private dependency fields whose meaning is already clear from the field name and type. For injected repository dependencies, put the role in the field name with a `Repo` suffix, such as `novelCommentRepo repositories.NovelComment`, instead of `comments repositories.NovelComment // 小说评论仓储`.
- Keep field comments concise and business-oriented.
- Do not repeat the field name as the whole field comment.
- Do not use comments to compensate for vague dependency names. Rename `comments`, `reports`, or `readingHistory` to names that expose the dependency role.

```go
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

// ApprovalRequest 审批请求
type ApprovalRequest struct {
    ID     string `json:"id"`     // 审批请求 ID
    Status string `json:"status"` // 审批状态
}
```
