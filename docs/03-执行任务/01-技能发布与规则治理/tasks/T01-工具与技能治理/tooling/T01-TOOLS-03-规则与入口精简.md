# T01-TOOLS-03 规则与入口精简

状态：`[x]`

- 上级：[T01-TOOLS](README.md)；所属端：技能文档；优先级：P2；难度：D2。
- 难度依据：Arc 多入口重复共享契约，需保留全局红线、平台工具和完成定义，同时消除分级冲突。
- 依赖：无；下游：04 本机安装。
- 范围：Arc 入口及必要 references、共享 docs、README、CHANGELOG、技能工程预算。保留现有生命周期划分。

<Context-Queries>

```bash
arc-idx search 'f:^Arc/ "multi-step"' --max 10 --format ai-md
arc-idx search 'f:^Arc/ "narrow exact"' --max 5 --format ai-md
arc-idx files SKILL.md --profile skills --format json
```

</Context-Queries>

执行：统一使用 arc:idx 的检索边界；任务分级只有一个权威；保留当前授权，只有真实阻塞歧义才提问；精简重复正文并按需加载细节；把文件风格限制明确为团队约定。不得放宽用户给定的全局红线。

验收：旧搜索脚本/宽松 rg 例外移除；普通多步骤不会被另一入口强制创建持久任务文档；关键约束仍可达；入口预算可检查；运行仓库校验和相关测试。

当前验证：`arc:sdlc` 为任务分级唯一权威；`arc:build`/`arc:fix`/`arc:clarify`/`arc:audit` 不再把普通多步骤写成持久任务文档。检索回退与 `arc:idx` 对齐。文件风格标明为团队约定。入口预算仍由校验器检查。
