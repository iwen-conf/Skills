# W04 本地接口

状态：`[-]`

2026-10-31 用户裁决"改成静态快照"（BG25）：本卡的前台服务、会话令牌与 `/api/*` 接口作废，由 [W13 静态快照导出](W13-snapshot.md) 取代；已写的 `workbench.go` 服务代码在 W13 中移除，DTO 与证据校验复用。重启条件：用户重新要求在页面内执行、批准或重试。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P1，按模型→义务→执行→接口→工作台→完整证据的依赖推进 |
| 难度 | D2 |
| 难度依据 | 复用同一 CLI/模型/执行方法；本地 HTTP 的 Host/Origin/令牌和边界需要 handler 级测试。 |
| 目标面 | 本仓当前工作树、本地 Go/Turso；Rust 仅远程 .60；真实业务用已授权隔离目标 |
| 前置依赖 | W01–W03：同一引擎入口可用 |
| 下游影响 | 工作台、真实 runner、六技能交接与发布证据 |

## 目标与范围

需求为 [BG01–BG18](../../00-产品需求/06-业务模型驱动验证.md)。范围：main.go；新增 foreground workbench HTTP handler/命名 DTO 和接口测试。新增文件属于既有工具模块，文件名在实施时回写。

共享设计与接口：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。不覆盖其他会话，不修改未授权远程业务数据，不用静态演示或历史 PASS 代替运行。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'tools/arc-verify' --format json
arc-idx search 'f:tools/arc-verify/ (run|openDB)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [ ] 从合同写可达入口的失败断言，保存 red，之后才改行为。
- [ ] 实施本端能力、所有相关调用点/fixture/配置并保持输入验证与错误可见。
- [ ] 验证：显式前台 loopback 服务有总时限；未知方法/参数、无令牌、跨 Origin/Host、超限输入拒绝；导入→确认→规划→执行→查回走真实 handler。
- [ ] 回写实际文件、命令/结果、未验范围，并同步 tracker。

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行；前端命令在新增工作台工程执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
