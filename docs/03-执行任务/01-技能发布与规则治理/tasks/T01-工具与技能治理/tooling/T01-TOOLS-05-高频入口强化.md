# T01-TOOLS-05 高频入口强化

状态：`[/]`

- 上级：[T01-TOOLS](README.md)；所属端：技能文档；优先级：P1；难度：D2。
- 难度依据：build/fix/test/idx 重复工作流和共享红线，精简时须保留已有可靠性、日志、环境与完成契约。
- 依赖：现有 79 入口校验与 53 项测试通过；下游：06 安装去重。
- 范围：四个高频 Arc 入口、技能工程契约、发布说明；其余领域能力保留。

<Context-Queries>

```bash
arc-idx files SKILL.md --profile skills --format json
arc-idx search 'f:Arc/ "Quick Contract"' --max 20 --format json
arc-idx files execution-truth.md --format json
arc-idx files test-stacks.md --format json
```

</Context-Queries>

执行：按任务读取相关参考；去除重复的输入表、红旗、工作流复述；共享规则以单一引用承载。入口保留触发边界、不可省略的约束、可执行工作流和证据输出。不得改变既有技术栈、全局检索限制或可靠性要求。

验收：四入口体积下降；必要约束可达；校验、pytest、独立情境评估、临时安装包校验通过。静态语料通过不冒充真实宿主行为。

当前证据：基线快照保存在本机缓存；待修改和验证。
