# Interface Or Contract Comments

Load only when commenting API contracts, service interfaces, repository interfaces, domain interfaces, or request/response contract definitions.

Keep comments proportional to the contract surface; do not expand obvious method signatures into boilerplate.

For `type Xxx interface`, document only the interface responsibility. Do not aggregate all method parameters and return values on the interface type comment.

```go
// Contract 定义小说应用用例。
type Contract interface {
    // GetNovel 查询小说详情。
    GetNovel(ctx context.Context, param GetNovelParam) (*NovelResult, error)

    // ListNovels 查询小说列表。
    ListNovels(ctx context.Context, query ListNovelsParam) (*ListNovelsResult, error)
}
```

Rules:

- `type Xxx interface` 的首行必须是 `// 接口名 定义xxx。`，只说明接口整体职责、边界或端口含义。
- Each exported interface method that represents a callable contract must have its own method comment immediately above the method.
- For `internal/usecase/<module>/contract.go`, method comments must be one concise sentence by default. Do not add `参数` / `返回体` blocks for ordinary `ctx`, `param`, `query`, `result`, or `err` signatures.
- Use `参数` and `返回体` blocks only when documenting repository/domain interfaces, external API contracts, or genuinely non-obvious parameter semantics. Put those blocks on the specific method comment, not on the parent `type Xxx interface` comment.
- Use `参数` and `返回体` headings without trailing colon.
- Use `// - 参数名 参数含义` and `// - 返回体名 返回体含义`; do not include type annotations unless the project explicitly needs them.
- Omit `参数` only when the method has no parameters. Omit `返回体` only when the method has no return body or result.
- Mention optional parameters directly in the parameter meaning, for example `筛选条件（可选）`.
- If an interface embeds another interface such as `Collection`, keep the embedded line uncommented unless it needs non-obvious behavior notes.
- Treat mass-added method comment blocks as a defect when they repeat the obvious signature, such as `ctx 请求上下文`, `param xxx参数`, `result xxx结果`, and `err 查询失败时返回错误` on every method.

Usecase contract example:

```go
// Bad: mechanical parameter and return blocks add noise without clarifying the contract.
type BadContract interface {
    // GetNovel 查询小说详情。
    //
    // 参数
    // - ctx 请求上下文
    // - param 小说详情查询参数
    // 返回体
    // - result 小说详情查询结果
    // - err 查询失败时返回错误
    GetNovel(ctx context.Context, param GetNovelParam) (*NovelResult, error)
}

// Good: the method name, param type, result type, and error already carry the contract.
type Contract interface {
    // GetNovel 查询小说详情。
    GetNovel(ctx context.Context, param GetNovelParam) (*NovelResult, error)
}
```

Repository interface example:

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
