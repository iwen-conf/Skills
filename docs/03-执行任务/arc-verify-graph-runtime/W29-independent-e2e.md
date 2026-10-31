# W29 独立进程业务 E2E

状态：`[x]` 工具层；IM-mongo 隔离写面见 [W35](W35-cross-project-e2e.md) `[x]`

独立进程 `testdata/enroll-sut` 复用 seat-v2 合同（未另建 enroll-v2 图）。正常/跨角色/并发/幂等/故障 BLOCKED/恢复失败留锁/重测保留历史 FAIL 均有 RAW。IM-mongo `lan50`/`tx` 仍 `approved: false`，不得记 PASS。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 集成 |
| 优先级 | P0 |
| 前置依赖 | W27、W03、W11 |
| 目标面 | `testdata/enroll-sut` 独立进程 + 文件持久化；IM-mongo 仍未批准 |

## 前置条件

IM-mongo `lan50`/`tx` 为 `approved: false`。本卡不得把 seat httptest 当唯一真实系统。独立 SUT 必须有业务状态、权限、持久化和独立 Oracle。

## 与现有业务的冲突

不改 seat 夹具合同。enroll-v2 是新图，不替换 course-v2 工具自测模型。

## 具体文件

- `tools/arc-verify/testdata/enroll-sut/main.go`
- `tools/arc-verify/enroll_e2e_test.go`（复用 `examples/seat-v2.json`，不另建平行模型）

## 必须覆盖

正常链、前置失败、跨角色、并发、幂等、故障、恢复、恢复后重测。每步 `RULE → FLOW → TASK → RUN → STEP → RAW → VERDICT`。恢复失败保持 BLOCKED 并留锁。

## 验收条件

独立于 `isolated_seat_test.go` 的进程。IM-mongo 仍标 BLOCKED，不记 PASS。
