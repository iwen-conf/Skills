# W30 工作台连接探索结果

状态：`[x]` 2026-10-31：快照增加 `exploration`（含 `tested`）。能力节点区分已描述/已实现/已测试/已完整验证；PASS 仍只来自确认义务。`file://` 浏览器只读打开，无外部请求。站点 `.arc/artifacts/verify-explore-e2e/site/`。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 前端/快照 |
| 优先级 | P1 |
| 前置依赖 | W13、W05、W27 |

## 前置条件

保留六视图和静态快照。不增加前端服务，不重做 UI。

## 与现有业务的冲突

Snapshot 增加 `exploration` 字段。types.ts 必须同步，否则字段比对失败。不按探索次数或信心点绿；PASS 分母仍只来自确认义务。

## 具体文件

- `tools/arc-verify/snapshot.go`
- `tools/arc-verify/workbench/src/types.ts`
- `tools/arc-verify/workbench/src/App.tsx`

## 验收条件

能力节点能看到规则覆盖、已探索/未探索、UNSPECIFIED、确认 FAIL、当前 PASS、待恢复、预算耗尽原因。区分已描述 / 已实现 / 已测试 / 已完整验证。
