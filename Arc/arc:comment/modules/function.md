# Ordinary Function Comments

Load only when commenting ordinary functions and methods that are not controller handlers and do not need the full interface contract template.

```go
// 函数名 函数作用
```

Rules:

- First line must be a single concise sentence: `// 函数名 函数作用`.
- Use the exact function or method name.
- Do not add parameter, return, error, or注意事项 sections for ordinary functions unless the surrounding project explicitly asks for a richer contract comment.

```go
// normalizeApprovalStatus 标准化审批状态
func normalizeApprovalStatus(status string) string {
    return strings.ToUpper(strings.TrimSpace(status))
}
```
