# Arc Verify 静态快照与业务条件依赖系谱

## 依据与状态

契约依据为用户的确定性业务状态机、ETA 依赖系谱和执行事实分离要求，以及
[共享业务验证语义](../../Arc/_shared/business-verification.md)。实现任务见
[条件系谱贯通](../03-执行任务/arc-eta-lineage.md)。本轮字段已在 Go / TypeScript 对齐，
四类实际导出已通过生产解析、统一选择和 SSR；产品执行及浏览器验收范围另列。

现有入口为 `arc verify snapshot`，实现于 `tools/arc-verify/snapshot.go`；
调用端为 `tools/arc-verify/workbench` 的生产解析器和只读 React 工作台。
这是本地 CLI 写出 UTF-8 JSON/HTML 的接口，不增加 HTTP 路由、远程认证或页面执行权限。
本机已有模型、轮次、RAW 读取权限沿用现有 CLI；导出不批准规则、不执行目标业务。

## 请求与输出边界

| CLI 参数 | 类型 | 必填 | 约束与含义 |
|---|---|---|---|
| `--graph` | string | 是 | 已导入 v2 业务模型的准确 ID |
| `--assets` | path | 是 | 已构建的工作台静态资源目录 |
| `--env` / `--round` | string | 否 | 已有环境/轮次；显式选择必须属于同一模型上下文 |
| `--run` | comma-separated IDs | 否 | 指定同图、同所选环境/轮次的运行；省略沿用最近 32 次导出边界 |
| `--config` | path | 否 | 既有本地配置路径；只用于现有登记项与证据验证 |
| `--out` | path | 否 | 不存在或空目录；不能覆盖历史快照 |

原有 `meta/model/views/runs/facts/evidence/contract_analysis/contract_execution` 及 campaign
字段保持原语义。输出 `snapshot.json` 与 `index.html` 内嵌数据一致。现有 64 MiB 默认快照
上限、普通文件资源检查及禁止联网的 CSP 不变。模型没有运行也可导出，必须显示未观察/未运行。

## 静态系谱：`business_lineage`

| 字段路径 | 类型 | 必有/可空 | 含义 |
|---|---|---|---|
| `business_lineage` | object | typed 模型提供；旧快照可缺省 | 从不可变业务合同派生的只读系谱，不是新业务模型 |
| `.version` | integer | 是/否 | 系谱投影协议版本，初始为 1 |
| `.contract_sha256` | string | 是/否 | 归属业务合同摘要 |
| `.branches` | array | 是/否 | 每个已声明分支的图；无分支为空集合 |
| `.branches[].branch/action/target` | string | 是/否 | 分支 ID、业务动作 ID、图内目标节点 ID |
| `.branches[].flow` | string | 否/否 | 所属 flow；缺省分支按既有全局适用规则处理 |
| `.branches[].nodes/edges` | array | 是/否 | 当前分支图内稳定节点与有方向的关系 |

### 节点

| 节点字段 | 类型 | 必有/可空 | 含义 |
|---|---|---|---|
| `id/name/branch/source_ref` | string | 是/否 | 图内稳定身份、显示名、分支及规则来源 |
| `kind` | enum | 是/否 | `target/state/condition/logic/preparation` |
| `action/preparation` | string | 否/否 | 已声明的业务动作/准备 ID，不从显示名推断 |
| `predicate` | Predicate | 否/否 | 原始 typed 谓词；`logic` 保留 AND/OR/NOT，不展平成全部必需条件 |
| `from_states` | string[] | 否/否 | 分支明确允许的前态；来源是分支合同 |
| `references` | BusinessLineageReference[] | 否/否 | 所引用的 subject + subject_kind + attribute，及可选属性 `source_ref` |
| `effects` | PreparationEffect[] | 否/否 | 仅声明的预计效果，不作为实际状态 |
| `confirmed` | boolean | 否/否 | 目标规则是否确认；不表示已运行 |

节点 ID 以分支、拥有者及谓词路径确定。属性自身的来源仍在原合同 `subjects[].attributes[].source_ref`。
状态前态、Guard 根与准备所需条件分别存在，不把动作状态图的循环复制进准备依赖 DAG。

### 边

| 边字段 | 类型 | 必有/可空 | 含义 |
|---|---|---|---|
| `id/from/to/source_ref` | string | 是/否 | 图内身份、明确端点与来源 |
| `kind` | enum | 是/否 | `requires/logical_operand/depends_on/candidate_provider` |
| `alternative` | boolean | 否/否 | 候选提供关系/逻辑替代，不能当作必须全部执行的计划 |

方向统一从上游条件/准备到消费方。`candidate_provider` 仅在准备动作的声明 Effects
单独足以证明对应谓词为 TRUE 时建立。效果相反、关系比较缺少一侧、仅属性名称相同均不成立。
准备条件的提供者只沿显式 `depends_on` 祖先关联，避免从属性重合臆造依赖或循环。
不存在 provider 只说明需要独立观察或尚无声明的准备路径，不能自行判定业务模型错误。
层级/上游/下游用于理解声明；候选路径和 OR 分支下的图层不是实际执行调度证明。

## 实际观察：`runs[].business_lineage`

| 字段路径 | 类型 | 必有/可空 | 含义 |
|---|---|---|---|
| `runs[].business_lineage` | array | 新导出提供；旧快照可缺省 | 当前运行的分支观察叠加，不复制全部 RAW |
| `[].branch/run/actor` | string | 是/否 | 冻结分支、当前 run 和执行 actor 的准确身份 |
| `[].step/action_index` | integer | 是/否 | `before` 对应实际步骤和动作序号；准备历史为 setup 的 -1 / 0，不声称目标动作已经发生 |
| `[].phase` | string | 是/否 | `before` 是目标动作冻结前态；`preparation` 是独立准备 readback，两者不可互换 |
| `[].evidence_status` | string | 是/否 | 来自现有 RAW/事实重验；非 PASS 的证据不产生满足结论 |
| `[].readback_index` | integer | 准备历史提供/否 | 准备 `readbacks` 的真实索引，0 合法；不以目标 action_index 代替 |
| `[].readback_phase` | string | 准备历史提供/否 | 原记录的 `initial/pre_action/post_action` |
| `[].evidence_sha256` | string | 准备历史提供/否 | 对应整个 setup RAW 的真实摘要，不是逐 Oracle 响应摘要 |
| `[].diagnostic/reason` | boolean/string | 否/否 | 失败准备仅供诊断的事实及原阻断原因，不能证明就绪 |
| `[].nodes` | array | 是/否 | 该作用域中的逐节点观察 |
| `[].nodes[].node/status` | string | 是/否 | 静态节点 ID、观察状态；缺值或证据坏不能显示满足 |
| `[].nodes[].evaluation` | TruthEvaluation | 否/否 | Go 内核给出的 TRUE/FALSE/UNKNOWN；浏览器不重新求值 |
| `[].nodes[].values` | array | 否/否 | 属性级实际值及绑定/来源 |
| `[].nodes[].observed_state/reason` | string | 否/否 | 实际前态或无法判定的原因 |
| `[].nodes[].values[].subject/subject_kind/attribute` | string | 是/否 | 原合同多主体属性身份 |
| `[].nodes[].values[].binding` | string | 否/否 | 真实账号/实体绑定；缺失保持缺失 |
| `[].nodes[].values[].entity_ref` | EntityBinding | 否/否 | 从冻结 subject binding 和该次前态实体记录取得的完整身份；包含 alias、kind、id、owner_actor、owner_principal、channel_account_id 及已有来源字段 |
| `[].nodes[].values[].source_ref` | string | 否/否 | 所声明属性的规则来源 |
| `[].nodes[].values[].actual` | JSON value | 否/可空 | 真实原始值；未观察与 JSON null 严格区别，数字保留原文 |
| `[].nodes[].values[].sources` | array | 是/否 | 当前作用域内的来源；不按全局 Oracle 名称归并 |
| `[].nodes[].values[].sources[].oracle/kind` | string | 是/否 | 已登记 Oracle ID；`kind` 为 `oracle/request_capture/preparation_readback` |
| `[].nodes[].values[].sources[].boundary/sha256` | string | 否/否 | 已核对边界及原始摘要；`preparation_readback` 的摘要属于整个 setup RAW |

准备的历史观察和 receipt 继续通过 `facts[].preparation` 展示，并可按 readback 在图上审阅。
准备节点若表示效果已满足，须同时核对 Ensures 与全部 Effects；这不证明该准备动作实际执行。
真实执行与否以 receipt 为准。准备观察 actor 来自已验证的冻结 manifest `Preparation.Observer.Actor`；
仅关联初始目标批次明确声明且主体范围相符的 `BusinessBranch`，不根据观察结果选择目标分支。
不能为缺少冻结 manifest 或无可验证身份的历史记录补造 scope。

实体筛选按完整 `entity_ref` 身份，不按裸 ID 或 `subject == entity.alias` 猜测。业务 subject
可以显式映射到另一实体 alias；目标动作只使用其真实 `before_entities`，准备历史只使用当次
readback 的实体上下文。动作后同 ID 换属主、换类型或换渠道不能覆盖此前身份。旧快照没有明确
映射且无法唯一关联时，保留事实并显示无法关联，不借其他实体填充满足状态。

成功准备必须通过现有完整准备重验和主体连续性检查，才能将其历史条件显示为真/假；目标节点
仍不显示 READY，因为准备观察不是目标动作的冻结契约。失败准备保留捕获值、三值评价和原阻断
原因，`diagnostic=true` 且所有节点保持 BLOCKED。缺少原 RAW 或有完整性错误时不降级猜测事实。

| 节点 `status` | 准确含义 |
|---|---|
| `SATISFIED` | 当前所选历史观察使该谓词为真；不表示目标业务通过或准备动作实际执行 |
| `UNSATISFIED` | 该谓词为假；在 NOT 或 OR 子树中不自动构成目标阻断 |
| `NOT_OBSERVED` | 没有可判定的实际值、绑定或动作冻结；保留未知原因 |
| `BLOCKED` | 证据/作用域不足，或失败准备仅供诊断；不能产生肯定就绪结论 |
| `READY` | 这个已记录的目标动作曾在冻结前态唯一匹配；不代表现在环境仍就绪或最终结果 PASS |

## 派生工作量边界

现有合同限制单谓词深度 32、节点数 2,048；本次只读投影另设全导出累计 65,536 个系谱节点、
131,072 条边和 131,072 条观察 reading 的工作量上限，避免重复展开放大内存开销。达到上限明确拒绝导出，不截断成
一张貌似完整的图。该工程预算不表示业务覆盖阈值；JSON 字节总量仍受既有 64 MiB 上限约束。

## 行为与兼容性

| 场景 | 输出与工作台行为 |
|---|---|
| 只有模型、未选运行 | 完整声明图、来源和候选路径；所有事实保持未观察 |
| 选中已核验运行/步骤/账号 | 只使用同一 scope 的 before overlay；实体筛选必须匹配真实绑定 |
| 选错分支、步骤或账号 | 显示当前选择无匹配观察；不能借用其他记录填绿 |
| Guard 是 OR/NOT | 保留逻辑节点及替代语义；不会将子项都算作待执行前提 |
| 原 RAW 缺失/损坏/身份不符 | 保留历史和诊断，证据 BLOCKED；不生成满足结论 |
| 静态边端点或 overlay 身份不合法 | 解析拒绝/可见错误，不能静默丢弃导致假完成 |
| 条件树/观察被截断、实际值或来源与原记录不符 | 解析拒绝；谓词评价与满足状态也必须一致，不让残缺输出保留 READY |
| 准备回读序号/阶段/主体与记录不一致 | 解析拒绝，不把不存在的历史读回展示为满足 |
| 旧导出没有系谱字段 | 仍可读原事实与状态图；明确未提供条件系谱，不能猜出 Actual |
| 状态重试自环 | 保持合法状态图；不参与准备 DAG 环检查 |

## 验收

L02/L03 使用同一契约：Go snapshot 专属测试和 DTO 对齐、真实独立 HTTP 工具目标的准备/运行导出、
生产 `parseSnapshot`、选择器与 SSR。四类导出的 45 个视图及 14 个独立观察范围已贯通；
实际浏览器交互仍为 NOT_RUN，没有用 SSR 冒充浏览器验收。
pay_gateway 的 model-only 导出用于检查来源和系谱可用性，不是支付执行验收。
最终命令、准确状态和摘要附于关联任务记录；未执行项不得因文档已写而改为 PASS。
