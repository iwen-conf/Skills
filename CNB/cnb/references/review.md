# PR 代码评审

对 PR 的代码变更进行专业评审，发现安全漏洞、Bug、代码质量和性能问题，并通过 PR Review API 发送行级评论。

## 证据门禁

必须加载 [`docs/execution-truth.md`](../../../docs/execution-truth.md) 的 evidence-first
root-cause repair 门禁。代码气味、扫描器结果或 diff 片段只是调查信号；只有在读取必要
上下文、确认可达路径、业务契约和根因后，才能作为确定性问题发送评论。证据不足时标记为
假设或不评论，禁止用泛化建议制造误报。

## 评审步骤

### 1. 获取 PR 变更

使用 pr-diff 模式获取 PR 的代码变更。

文件分类遵循 [pr-diff 模式](pr-diff.md)：先盘点全部变更，锁文件检查版本、来源和清单一致性，二进制记录未审原因。

单批预算可用 30 文件 / 100000 字符；超出时分批，不截断后冒充完整。保留已审、未审文件及原因；预算耗尽输出 `partial`。

### 2. 分析代码

**评审重点**（按优先级）：

1. **安全漏洞**：SQL 注入、XSS、CSRF、硬编码凭证、敏感信息泄露、不安全加密
2. **潜在 Bug**：空指针、数组越界、并发问题、资源泄漏、异常处理不当
3. **代码质量**：可读性、命名规范、代码重复
4. **性能问题**：N+1 查询、低效算法、不必要的 I/O

**评审原则**：
- 评审全部变更语义，包括删除鉴权/校验和必要上下文；避免对无关未改代码发建议。
- 行评论只使用 API 支持的真实 diff 锚点；纯删除问题无右侧锚点时写总结评论，不伪造新文件行号。
- **忽略**：EOF 换行符、行尾空格、缩进风格（应由 linter 处理）

**理解上下文**：
当 diff 上下文不足以理解代码时（如需要看函数定义、类型、依赖关系），使用 Read 工具读取相关源文件理解上下文，并确认准确行号。

### 3. 输出评审结果

**必须严格输出以下 JSON 格式**：

```json
{
  "status": "partial",
  "coverage": {"complete": false, "reviewed_files": [], "unreviewed_files": ["src/auth.ts"]},
  "issues": [
    {
      "severity": "critical | warning | info",
      "file": "文件路径",
      "start_line": 12,
      "end_line": 14,
      "problem": "问题：描述问题",
      "suggestion": "建议：修复建议"
    }
  ]
}
```

**字段说明**：
- `status`: 评审状态
  - `passed`: 所需范围已完整审查且无确认问题
  - `partial`: 存在未审范围；同时保留已发现问题与严重度，不可发布“通过”
  - `needs_modification`: 需要修改
  - `critical`: 存在严重问题
- `severity`: 问题严重程度
  - `critical`: 严重（安全漏洞、崩溃 Bug）
  - `warning`: 警告（潜在问题）
  - `info`: 建议（代码质量改进）
- `start_line` / `end_line`: 问题代码的行号范围（对应新文件行号）
- `problem` / `suggestion`: 使用 Markdown 格式，提高可读性
  - 使用 \`代码\` 标记变量名、函数名
  - 使用代码块展示修复示例

**无问题时**：`issues` 为空数组 `[]`；只有 coverage.complete=true 才允许 passed。示例数字只是格式样本，实际锚点必须核实。

### 4. 发送评审评论

仅在用户明确要求发布评审或宿主已授权此动作时发送；否则交付本地结果。应用[运行合同](../../../docs/cnb-runtime.md)。下方 payload 用 JSON serializer 写成 body file，再以 `--data-binary @文件` 传入；检查 HTTP 与业务标识，不把请求发出当评论成功。

**重要**：API 基础地址从环境变量 `CNB_API_ENDPOINT` 获取（默认 `https://api.cnb.cool`），仓库名从环境变量 `CNB_REPO_SLUG` 获取，PR 编号从环境变量 `CNB_PULL_REQUEST_IID` 获取。

**有问题时**：调用 PostPullReview 接口发送行级评审评论：

```bash
curl -X POST \
  "${CNB_API_ENDPOINT:-https://api.cnb.cool}/${CNB_REPO_SLUG}/-/pulls/${CNB_PULL_REQUEST_IID}/reviews" \
  -H "Accept: application/vnd.cnb.api+json" \
  -H "Authorization: Bearer $CNB_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
  "body": "**评审结果**: 需要修改",
  "event": "comment",
  "comments": [
    {
      "body": "问题: ...\n建议: ...",
      "path": "文件路径",
      "start_line": 12,
      "start_side": "right",
      "end_line": 14,
      "end_side": "right",
      "subject_type": "line"
    }
  ]
}'
```

**完整审查且无问题时**：调用 PostPullComment 接口发送总结评论；partial 只能说明覆盖范围与缺口：

```bash
curl -X POST \
  "${CNB_API_ENDPOINT:-https://api.cnb.cool}/${CNB_REPO_SLUG}/-/pulls/${CNB_PULL_REQUEST_IID}/comments" \
  -H "Accept: application/vnd.cnb.api+json" \
  -H "Authorization: Bearer $CNB_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
  "body": "**评审结果**: 通过\n\n代码质量良好，未发现明显问题。"
}'
```

## 注意事项

1. 单次最多发送 10 条评论，完整本地结果保留其余问题并标注未发布数量；发布限额不改变审查结论。
2. 行号必须对应 diff 中的**新文件行号**（右侧行号），务必通过读取源文件确认
3. 每条评论需说明问题原因和修复建议
4. 评审结果必须是有效 JSON
