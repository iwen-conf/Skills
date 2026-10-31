# W35 跨项目 E2E

状态：`[x]`（iso28886 已授权写入）；图谱 TTY 批准仍 `[!]`

## T2 简报

- 目标：把 IM-mongo 夹具改到隔离 `iso28886`，在该面写数据和做业务级故障（越权领取、并发抢工单），用 ARC Verify 跑已知规则、探索与模式对照。
- 范围：`IM-mongo/.arc/verify/fixtures.json`；Skills `tools/arc-verify/im_iso*.go`、本卡与 tracker。
- 非目标：第七个 Skill、常驻服务、共享 `28885/28895`/生产写、重启隔离 writer 部署变异二进制、伪造 27 条 RULE 的 TTY `APPROVE`。
- 验收：夹具不再绑 `lan50`；`refuseSharedIMTarget` 拒绝 28885/tx/28895；iso28886 已知规则与探索有 RAW；28885 只读仍 200。

层次 A：独立产品 `IM-mongo` 隔离 writer `http://192.168.1.50:28886`（pid 416143）。已知规则 8/8 PASS。层次 B：`im-support-v1.1` `flow.intake` 探索。层次 C：脚本规划器已接线；本轮 AI 模式未触发 `planner_calls`（确定性候选未耗尽），启发式独有 2 条 UNSPECIFIED。

变异二进制部署：`NOT_RUN`。隔离 writer 是已登记候选，本轮不重启 28886。

IM-mongo 图谱 27 条规则仍须交互 TTY `APPROVE`；本轮只在 Skills 测试库导入图谱，不改产品 `data.db` 批准位。
