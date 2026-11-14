# arch-diagram 业务图示例

这些页面由固定的官方 arch-diagram **1.2.3** 构建。
先打开审阅页查看业务结构，源 ID、包名证据、规则和运行历史可在下方展开；原始 renderer HTML 保持不变。

| 示例 | 审阅页 | 原图与完整数据 | 来源 |
|---|---|---|---|
| 回调与异步闭环：一个域、五个同级能力 | [能力总览](async-overview.review.html) | [HTML](async-overview.html) · [PNG](async-overview.png) · [spec](async-overview.spec.json) | 用户截图结构的合成回归夹具 |
| 报名业务能力树：4 个节点、3 条关系 | [能力树审阅](capabilities.review.html) | [HTML](capabilities.html) · [spec](capabilities.spec.json) | 原有 course-v2 工具历史快照 |
| 报名业务流程：6 个节点、4 条关系 | [流程审阅](flows.review.html) | [HTML](flows.html) · [spec](flows.spec.json) | 原有 course-v2 工具历史快照 |

## 布局改动

- 真正的 `contains` 树用官方分组容器表达，五个同级能力采用居中的 3 + 2 排列；不再为包含关系画五条跨行长箭头。
- 有多个归属或环时保留显式关系。真实参与、操作和状态迁移继续使用箭头，标签靠近对应连线并避开卡片和容器边界。
- 中文业务名与完整包名单独换行；卡片移除源 ID、文件路径和零失败计数。父域状态、候选、非零历史 FAIL 与缺口仍然可见。
- 完整证据放在独立的只读审阅层；官方 HTML 被原样嵌入，窄屏保持可读字号并横向滚动，没有新增执行、批准或重试入口。
- `arc.edge_rendering` 记录每个源关系是由包含区域还是连线呈现；计数包含两者，平行源边不会因为共享几何关系而丢失。

## 来源和事实边界

[截图结构夹具](async-overview.snapshot.json) 根据用户提供的六节点截图重建，仅用于重现布局问题，
不是原始 DB 导出或真实业务验收。[展示映射](async-overview.presentation.json) 不补写规则或通过结论。
截图中出现的包名仍保存在节点原始名称里；当前没有目标源码证据，因此图面标为 `业务包待核实`。

另外两份输入来自 [sample-snapshot.json](../../../tools/arc-verify/workbench/dev/sample-snapshot.json)，
原始导出时间 `2026-10-31T08:17:41Z`，图指纹为
`45abde3ca7c55187d906d278cd54be36982701004bfb47864acb06062d2e0fdb`。
该快照记录了一次 FAIL 和后续 PASS，FLAKY 与总体 BLOCKED 继续保留；没有用后一次成功抹掉首次偏离。
快照中的临时路径仅为历史出处，RAW 完整性 PASS 不等于业务 PASS。

## 实际验证

[构建与验证记录](build-report.json) 与 [最终页面实测记录](visual-review.json) 分开记录原始命令结果和人工/Agent 看图结果：

- 三个官方 HTML build 和 layout check 均退出 0，检查结果为 0 个几何问题；三个最终截图均已目视核对。
- 浏览器为实际 Chromium 153.0.8010.0，经任务本地的 Playwright 1.62.1 私有管道兼容入口执行官方检查与 PNG 导出。
  上游脚本、引擎和样式未改动；这个测试入口没有分发到 plugin，也不代表原生 Chrome 命令或视频导出验收。
- 审阅页覆盖桌面 1280×900 和窄屏 390×844；实际展开节点、关系、规则、历史和来源。
  无外部请求、页面脚本错误或非预期页面横向溢出。窄屏的图与宽表各自滚动。
- 绘图定向回归 47 项通过；全仓 pytest 为 686 passed / 1 failed / 4 skipped。
  唯一失败仍为既有 `cloudflare-platform` 外部源缺失，跳过项涉及 protobuf/Go 环境。
- 未运行真实业务目标或原生 Claude Code / Codex CLI / Grok Build 会话。

0.4.0 当时因缺少浏览器而没有完成视觉验收；该历史结果不能当作本版检查结果。
正常 `diagram` 命令始终先返回 `visual_review: NOT_RUN`，因为自动几何检查无法代替看图。
本目录示例的后续截图检查单独记录在 `manual_visual_review`；没有自动把生成成功当作视觉通过。

重新生成时使用新的输出目录，不要覆盖已有图。用已安装的
[arch-diagram](https://github.com/bassnova42/arch-diagram-skill)
技能，输入本目录的 snapshot 与 presentation JSON，`--view capabilities`。

官方渲染器固定于
[`879ca7fa503a64ab46b5bc77d2062ded32fadc84`](https://github.com/bassnova42/arch-diagram-skill/tree/879ca7fa503a64ab46b5bc77d2062ded32fadc84)。
上游 MIT 许可证全文、版权与固定来源随 spec、原图及审阅页保留。
