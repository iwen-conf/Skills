# Arc 条件依赖系谱与工作台贯通

## Task Brief

- Tier / Mode：T2 / feature completion；用户要求继续实现到可用的 ETA 风格业务系谱。
- 基线：`ab15f2b2ffe0b55509392d8ff49425f603ef5f98`，直接在 `main` 修改，起始工作树干净。
- 范围：只读系谱投影、作用域观察、工作台交互、pay_gateway 模型导出和回归；保留执行器与历史账本。
- 验收发现的追加范围：共享 gate 正确调用项目已有的 Vite+ / 本地工具，避免 Bun 项目被 `npx` 驱动阻断。
- 用户闭环：能力 → 分支 → 前态及条件 → 上游准备/观察来源 → 当前运行事实 → 下游影响 → 状态/运行回放。
- Web：现有 React / TypeScript / ReactFlow；桌面四区，窄屏可读的分层目录，不引入新依赖。
- 契约：[快照系谱契约](../02-API记录/arc-verify-snapshot.md)；业务语义沿用共享确定性契约。
- 验收：语义红例、Go race、生产快照 → 前端解析/选择/SSR、类型/构建、安装与 CLI smoke。
- 真实产品边界：pay_gateway 需要已有隔离 PostgreSQL、Rust/psql、owned fixture 与账号；缺失即 BLOCKED。

## 已核实事实与裁决

| 事实 | 根因 / 裁决 | 验证方式 |
|---|---|---|
| 依赖画布只遍历 `preparation` 与 `depends_on` | 声明任务 DAG 未投影分支目标、前态、Guard 条件或真实来源；新增后端统一业务系谱投影 | 对真实 `buildSnapshot` 输出先断言目标/条件/提供者，再校验真实运行观察 |
| 选择分支/账号/实体不影响旧准备图 | 画布不消费统一上下文；只叠加同一 run/step/action-index/actor 的 before 观察 | 同名 Oracle 双账号及错 scope 反例 |
| 准备失败还未产生 Frozen，图上无法定位准备历史 | 为已有 setup RAW 派生有明确 manifest 观察主体的 readback 作用域；失败只供诊断，目标不变 READY | 真实 HTTP 激活 false→true、假成功仍 false 且目标零请求、RAW 篡改反例 |
| 前端已有最长路径、上下游、SCC 循环折叠 | 保留算法，补充业务语义、层级/分组和面板，不另造任务状态机 | 带捷径 DAG、状态自环和 OR 分支回归 |
| 全局分支在能力树可见而 flow 下拉隐藏 | 下拉直接比较 `branch.flow`，未使用共同的 flow 适用规则 | 生产 App 的选择器断言红例及修复 |
| 实体选择仅比较 subject/alias 和裸 ID | 后态同 ID 换属主可借用前态；合法不同 alias 又被误滤。用冻结 runner 映射及真实 before/readback 生成完整 `entity_ref` | 真实 HTTP 前后同 ID 换属主；跨角色、命名空间碰撞、渠道和缺身份反例 |
| 读取器只校验字段形状，未核对派生观察与原记录 | 缺 Actual/来源、FALSE 与 SATISFIED 冲突、伪造准备回读、同步裁掉条件均可能误导 | 正式解析器反例、合法 OR/NOT 对照及独立 20 项限定复核 |
| 通用 gate 经 npx 调用工作台类型工具 | 项目要求 Bun，包管理器层拒绝执行；Vite+ 的格式/测试又未被识别 | 修正本地工具路由，保留包约束、原失败退出码与无工具 NOT_RUN |
| pay_gateway 当前 main 已更新 | 最新 `56154994b09af0cc439356ba1fb72916a9c9fc65` 不继承旧 pin 的执行资格；本次保留 v2 来源和历史 | 固定源码差异审阅、现有模型静态导出；不称支付实测 |

## 分工与依赖

| 任务 | 实现 | 依赖 | 状态 |
|---|---|---|---|
| L01 系谱协议 | 本地 Markdown 契约、稳定分支节点、边和作用域观察 | 既有业务模型及事实契约 | 完成 |
| L02 Go 投影 | `business_lineage*.go`、`snapshot.go`；不增加 SQL 表 | L01；现有 RAW/事实完整性重验 | 完成；最终全量 race 通过 |
| L03 工作台 | 业务系谱、细节面板、层级/局部视图、统一选择 | L01/L02 的真实 JSON | 完成；实际导出贯通 151 项通过 |
| L04 pay_gateway | 保留来源的四分支示例、可直接导出模型的使用路径 | 现有 v2 模型、L02/L03 | 模型导出完成；产品 E2E 阻断 |
| L05 验收与发布 | 新导出、实际工具动作证据、完整检查、直接更新 main | L02–L04 | 代码、导出与验收完成；按用户授权直接交付 main |
| L06 门禁工具路由 | 项目原生格式、类型与测试入口；不隐式安装 | 实际严格门禁暴露的兼容缺陷 | 完成；8 项红绿与全部 65 项回归通过 |

## 判定边界

1. 系谱是解释与追溯投影，不在浏览器重选分支或派发准备动作。
2. 条件树保留 AND / OR / NOT；多个候选 provider 是替代可能性，不是全部必经步骤。
3. 只有声明 Effects 能独立证明条件时才连接候选 provider；同属性名不证明能建立条件。
4. 预计效果不产生 Actual；准备效果满足不证明该准备动作已经执行。
5. 目标分支的冻结 before 是本次分支前态；after 和下一次运行不得回填 before。
6. 缺少观察、观察为假、模型缺口、证据损坏分别显示。未执行页面不能显示业务 PASS。
7. 能力树、准备 DAG 与允许循环的业务状态图继续分离；静态页保持只读。
8. 失败准备的捕获值可定位问题，但图上保持 BLOCKED；成功准备 readback 也不是目标动作冻结。

## 验证记录

最终命令、退出码、源码/资源/快照摘要与阻断见
[机器可读验收记录](evidence/arc-eta-lineage-2026-11-17.json)。

| 验证 | 实际结果 |
|---|---|
| 最终 verifier 全量 race | 526 主测试、410 子项通过，4 项跳过；没有 race 报告 |
| 公共 CLI / 索引 race | 37 主测试与 40 子项通过 / 2 主测试通过 |
| 安装、公共 CLI smoke、真实导出 | 全部通过；导出测试的 3 个实际 HTTP 场景均执行 |
| 前端全套与实际快照贯通 | 15 文件、151 测试通过；4 份快照资源与最终 dist 一致 |
| 前端格式 / lint / 类型 / 构建 | 全部通过；保留离线单包大于 500 kB 的原生构建提示 |
| 新 gate 回归 | 同一 8 项反例转绿；全部 65 项门禁测试通过 |
| DBQual | 160 项与 139 个子项通过；真实 SQLFluff 调用 1 项未配置跳过 |
| 最终严格 gate | 格式、Go/TS/Python 静态检查、Go/前端测试、red 回放、scope 均通过；总结果仍 exit 1，仅剩原有 Cloudflare 缺源导致的 Python 失败 |
| 最终全仓 Python | 246 通过、1 跳过、1 既存失败；跳过项需专用 protobuf 解释器 |
| 技能来源完整性 | 52 个既存外部链接错误；本次未删除引用或伪造依赖以转绿 |

最终严格 gate 为 `1 FAIL / 0 NOT_RUN / 1 WARN`。WARN 是 `test_gate_script.py` 在 red 后进行两处
formatter 换行，断言未改变，原 red 命令已再次通过。旧 gate 的 npx/Bun 不兼容已在真实工作台
调用中复核修复；没有关闭包约束，也没有把工具不存在解释为 PASS。

最终 Go / TypeScript 源码冻结后重新执行完整 `go test -race -json -count=1 -timeout=20m ./...`，
526 个主测试与 410 个子项通过，4 项跳过，没有 race 报告。两项可选导出已在显式目录参数下
另外执行并通过，另两项依赖本机缺失的 IM-mongo 配置/fixture。已安装的公共 CLI campaign 和
chain 进程检查已在完整 race 命令中启用；CLI 安装与 smoke 也通过。

首次全量 race 使用的是 entity_ref 之前已编译的 Go，而类型镜像测试后来读到了更新后的 TS，
该次完整运行保留为 FAIL。最终全量命令在 199 个 Go 文件及前端源码/资源冻结后重新运行，
没有用“前半旧版本、后半新版本”的拼接结果代替全量 PASS。

### 已生成的实际工作台

| 导出 | 内容与真实范围 | 生产读取验证 |
|---|---|---|
| pay model | 真实安装 CLI 在新账本导入已有四分支模型；0 run，模型和规则未批准 | 4 分支、60 节点；退款 30 节点 / 31 关系 / 10 层 / 3 候选来源；没有 Actual |
| prepared tool | 实际 HTTP 激活、独立 false→true 回读，再执行 1 次目标动作 | 3 个准备 readback、1 个 before occurrence；冻结预期与实际分开 |
| campaign tool | 同一业务实体的 3 个实际 child run；动作 enroll→clear→enroll | 4 个准备 readback、3 个 before occurrence；3 次准确实体选择 |
| blocked preparation tool | 准备接口返回成功但真实状态仍不满足；目标请求 0 次 | 3 个诊断 readback，所有节点 BLOCKED；没有 Frozen 或 before occurrence |

四份导出均通过生产 parser、统一选择与 SSR，资源与最终 dist 一致；共检查 45 个视图和 14 个
独立观察范围。151 项前端测试通过。原始证据有效与业务准备 BLOCKED 是两种独立状态；失败
准备不能因其证据完整而变成成功。

独立复核重新执行原来的 20 项最小探针，以及真实 HTTP 实体历史反例，原 must-fix 全部关闭。
探针原始 stdout 留在协作记录，未伪称存在持久化日志；正式拥有测试及 red 记录作为可重复证明。
`red-015` 到达实际实体映射缺失断言；`red-012/014` 只是 fixture 配置失败，不计缺陷敏感性。

### 尚未验收的范围

1. pay_gateway 真实产品 E2E 为 BLOCKED / NOT_RUN；当前缺完整固定 checkout、Rust/psql、已授权远程隔离数据库及 owned fixture。四分支来源仍固定旧 pin，不继承最新构建的运行资格。
2. 实际浏览器与窄屏视觉/点击验收为 NOT_RUN；构建、SSR、布局与选择状态验证不能代替它。没有生成或交付假运行截图。
3. 直接目标动作在没有 preparation 且尚未冻结分支时，Decision 状态/原因和原始观察仍在事实面板；当前不会把这些观察猜配到另一分支的系谱。
4. 完整支付权限矩阵、外部机构、通知投递、资金并发和无限输入域不由当前四分支模型覆盖。
5. 仓库原有 `cloudflare-platform` 源缺失和 Design/Cloudflare/ThreeJS 的 52 个外部技能链接错误分别记录，不属于本轮系谱 PASS。
