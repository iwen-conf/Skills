# W05 静态快照工作台

状态：`[x]`

2026-10-31：用户裁决"改成静态快照"（BG25）。`workbench/src/App.tsx`、`api.ts` 已有按 `/api/*` 写的未提交实现，需改造成只读读取内嵌快照；改造前先确认没有其他会话在改这些文件。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 前端 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 六视图、三空间、同 ID 下钻和回放都要从同一份快照 JSON 投影；`file://` 下零网络请求；缺证据、FAIL、陈旧、截断都不能被界面隐藏或改成绿色。 |
| 目标面 | `tools/arc-verify/workbench/` 构建产物，由 W13 的 `snapshot` 复制进快照目录，用真实浏览器从 `file://` 打开 |
| 前置依赖 | [W13](W13-snapshot.md)：`snapshot.json` 结构已定、`types.ts` 与 Go DTO 字段比对测试通过、能从 `academic-v1` 导出样例快照 |
| 下游影响 | W06 联合验收、W14 技能文档 |

## 目标与范围

需求为 [BG01–BG25](../../00-产品需求/06-业务模型驱动验证.md)，重点 BG04、BG05、BG10–BG13、BG25；设计见 [05 工作台](../../01-系统设计/05-业务图谱工作台.md)"静态快照"和"六视图的职责与导航合同"。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。

范围：`tools/arc-verify/workbench/src/` 的 React/TS、CSS、组件测试与 `vite.config.ts`。

- 启动时只读 `<script type="application/json" id="arc-snapshot">`；缺失或 JSON 损坏显示错误状态（写明阶段和原因），不发请求、不回退到样例数据。
- 删除 `api.ts` 的 `fetch`、会话令牌和 `X-Arc-*` 头；删除导入、批准、规划、执行、重试的按钮和表单。
- 顶部显示快照生成时间、图版本、环境、轮次，同一事实只出现一次。
- `vite.config.ts` 设 `base: './'`，构建产物从 `file://` 可直接打开。
- 开发样例只用 W13 经 CLI 导出的 `tools/arc-verify/workbench/dev/sample-snapshot.json`（v2 course 参考模型；`academic-v1` 是 v1 做不了多视图），不手写；只在 `import.meta.env.DEV` 下加载，生产构建不得打包它。
- `file://` 下 Chrome 拒绝跨源加载 `type="module"` 脚本：构建产物要改成非 module 脚本（如 IIFE 单文件），以真实浏览器打开为准。
- `types.ts` 已由 W13 按 Go DTO 重写（快照 JSON 合同），本卡按它改 `App.tsx`，不改字段。

不做：不在前端计算 PASS 或覆盖分母；不新增图布局依赖（不装 ELK/Dagre）；不保留"服务不可用时用样例数据"之类兜底；不改 Go 代码（属 W13）。

## 2026-10-31 实施记录

开工核查：`api.ts`、`layout.ts` 已被另一个会话在 12:45 改成抛错/返回空的桩（对应测试仍期待真实实现），属于已作废 API 方案的半成品；原文件已在 W13 打包备份到 `.arc/artifacts/verify-snapshot/pre-snapshot-backup/`。

实际改动（`tools/arc-verify/workbench/`）：

- 删除 `src/api.ts`、`src/api.test.ts`；新增 `src/snapshot.ts`（只读 `#arc-snapshot`，缺失 → `snapshot.missing`、坏 JSON → `snapshot.parse`、缺视图/非法状态/重复 ID → `snapshot.views` 等，失败不修补、不回落到样例）与 `src/views.ts`（只做展示筛选和下钻判断：按状态、完整度、领域、角色——角色取它 `serves` 的实体及其下级——不动覆盖分母）。
- 重写 `src/App.tsx`：三空间六视图、顶部上下文各显示一次（图版本、hash、环境、轮次、导出时间）；关系图/层级/矩阵三种展示；实体面板（状态、完成度四项、跨视图定位，不含该实体的视图写明原因）；测试验证视图的完整义务表与运行回放（首个偏离、每个 Oracle 的动作前后读数、并发批次提示、逐条证据，导出时校验失败的证据显示为失败条目）；出错页写明阶段与原因并可复制诊断。没有任何写操作入口。
- 实现 `src/layout.ts`（边界只装 `owns` 的实体、`contains` 不算归属；流程按层遍历、有环也终止；确定性排序）。
- `src/App.css` 重写（沿用 `index.css` 的语义变量，`:focus-visible`、`prefers-reduced-motion`）；`src/main.tsx` 改为读取内嵌快照，开发态才加载 `dev/sample-snapshot.json`。
- `vite.config.ts`：`base: "./"`、IIFE 单脚本、构建时把 `type="module" crossorigin` 改成 `defer` 普通脚本（`file://` 下浏览器拒绝 module 脚本）；`index.html` 的图标改相对路径；`.prettierignore` 忽略生成的 `dev/`。
- `layout.test.ts`（其他会话所写）只做了格式化。

浏览器验收中发现并修复：下钻后根节点被切视图清掉（改为层级视图按子树收窄、其他视图保留选中并定位，`views.test.ts` 覆盖）；义务被选中实体筛空时没有说明（加空状态和"显示全部义务"）；失败步骤期望与观察都为 ready 却判 FAIL、看不出原因（加独立判据读数，可见 `capacity=0、enrolled=1`）；边界容器标题被子节点遮挡、长名称截断状态（调大节点与标题区）；人工验收要求的领域、角色筛选原先缺失（补上，`views.test.ts` 覆盖，浏览器中按"学员"筛选只剩学员、其服务的领域与能力，截图 `08-role-filter.png`）。诊断与合同变更示例快照里没有，改由组件测试用合成数据覆盖（`OWNERSHIP_CONFLICT`/`DEPENDENCY_CYCLE` 带对象显示；有上一版本时显示变更、受影响实体、传播原因与待复验测试，没有时写明无差异）。

验收结果：

- `vp check`：格式、lint、类型全部通过，无警告；`vp test`：17 项通过；`vp build`：产物只有 `index.html`、一个普通 `defer` 脚本和一个样式表，`grep -rl '/api/'` 与样例数据关键字都无结果。
- red：`.arc/artifacts/verify-static-workbench/red/red-001.json`。
- 门禁 `gate.sh check --strict --require-red --scope`（`reports/gate-20261031T102339Z.txt`）：Go 部分、scope、red-first 都 PASS；`lint(ts)`、`tests(js)` 两项 FAIL 是门禁用 npm 调 `tsc`/`vitest`，被本工程 `devEngines` 指定的 bun 拒绝（`EBADDEVENGINES`），不是代码结论。同样的检查用工程自己的命令跑过：`vpr build`（`tsc -b && vp build`，2/2 成功）、`vp test`（17 项）、`vp check`（无警告），输出在 `.arc/artifacts/verify-static-workbench/project-commands.log`。red-hygiene WARN：红灯后测试只加严（覆盖六视图与出错页、失败证据、诊断、合同变更、领域与角色筛选），源码扫描从 vitest 移到 Go 的 `TestWorkbenchSourceHasNoNetworkLayer`（vitest 工程没有 node 类型）。
- 种下缺陷 7 个全部抓到（重复 ID、伪造结论、缺视图、失败证据当有效、下钻失效、忽略 owns、出现写操作按钮）；"写操作按钮"首轮漏网（只查了正常页），补上六视图与出错页后抓到。
- 真实浏览器（agent-browser，Chrome，1440×900）从 `file://` 打开 CLI 用真实构建导出的快照（`.arc/artifacts/verify-static-workbench/site/`，由 `TestSnapshotSampleForWorkbench` 设置 `ARC_VERIFY_SAMPLE_SITE`/`ARC_VERIFY_SAMPLE_ASSETS` 导出）：网络请求只有快照目录下的 `file://` 文档、脚本、样式、图标，无外部请求；无页面错误、无控制台消息；六视图切换、选中、下钻到能力子树并返回、FAIL 与 PASS 两次运行回放、证据与恢复、Tab/Enter 键盘操作与焦点可见均正常。截图：`01-overview.png`、`02-capability-selected.png`、`03-drill-capabilities.png`、`04-verification-fail-run.png`、`05-pass-run.png`、`06-boundaries.png`、`07-keyboard-focus.png`、`08-role-filter.png`。

未验证：Safari/Firefox 未测；诊断与合同变更只在组件测试里用合成数据验证过，没有在浏览器里看真实案例；样例是本机合成夹具，IM-mongo 真实快照归 W06；减少动态效果只靠 CSS 规则，未在系统设置里实测。

## 2026-10-31 晚：首次 FAIL 回放与 seat 快照

回放改为优先打开带 `first_divergence` 的首次 FAIL（`views.ts` `evidenceRun`），避免义务上的最近 PASS 盖住历史偏离。组件测试 18 项通过。真实浏览器打开 W25 导出的 seat-v2 快照（`.arc/artifacts/verify-isolated-seat/site/`）：六视图、能力 `reservation.claim` 为 FLAKY、回放 `R-d4a8d1faf1b6e8152b1a071a3bf95bb0`（确认失败，idle→enrolled）、义务分母 33/33，网络只有同目录 `file://` 资源。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'tools/arc-verify/workbench/src' --format json
arc-idx search 'f:tools/arc-verify/workbench/src/ (fetch|X-Arc|/api/)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/workbench/src/ BusinessView' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败断言：组件测试里无快照、坏 JSON、含 `/api/` 调用三种情况必须失败，保存 red。
- [x] 改为读取内嵌快照；移除网络层与写操作入口；保留六视图、三空间、过滤、下钻和回放。
- [x] 验证：`vp check`、`vp test`、`vp build`；构建产物 `grep -rn '/api/' dist` 无输出。
- [x] 真实浏览器（agent-browser）从 `file://` 打开 W13 导出的快照，逐项完成下方人工验收，截图存 `.arc/artifacts/business-graph-workbench/`。
- [x] 回写实际文件、命令/结果、未验范围，并同步 tracker。

### 人工验收（真实浏览器，`file://` 打开快照）

- [x] 全景选领域→能力→行为流程→关联测试→实际运行→RAW；每次切换核对同一实体 ID 与上下文，返回保留选择。目标视图不适用时显示原因，不生成同名节点。
- [x] 分别检查能力树、边界容器、流程和影响图：同一实体可有不同形态；跨边界边保留；内部校验行为不自动成为独立能力；部署位置不改变业务归属。
- [x] 按领域、角色、能力完整度和覆盖筛选；筛选前后完整义务分母相同。快照只含导出时的上下文，不跨环境/轮次合并局部 PASS。
- [x] 有来源的缺测试、合法共享、冲突所有权、循环依赖与合同变更案例显示诊断及传播依据；没有发布证据的能力显示上线未知。
- [x] 回放含拒绝或失败的真实 run：输入、观察、首个偏离、RAW 与 hash 正确；导出时校验失败的证据显示为失败条目；并发顺序未知、概念模型明确标识。
- [x] 浏览器网络面板 0 个请求（除同目录静态资源）；页面无任何写操作入口；键盘可完成导航、树展开和回放，焦点可见，尊重 reduced motion。

```bash
cd tools/arc-verify/workbench && vp check && vp test && vp build
grep -rn '/api/' tools/arc-verify/workbench/dist   # 期望无输出
```

工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
