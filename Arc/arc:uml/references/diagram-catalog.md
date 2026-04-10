# UML 图型目录与证据映射

## 图型到证据的映射

| 图型 | 英文标识 | 主要证据来源 | 常见输出价值 | 默认源文件 |
|---|---|---|---|---|
| 类图 | class | 领域模型、DTO、实体定义、接口定义 | 展示静态结构与继承/组合关系 | `diagrams/class.drawio` |
| 对象图 | object | 运行时实例样本、测试夹具、示例数据 | 展示某一时刻对象实例关系 | `diagrams/object.drawio` |
| 组件图 | component | 模块边界、服务目录、依赖声明 | 展示系统组件职责与依赖 | `diagrams/component.drawio` |
| 部署图 | deployment | k8s/docker/terraform/ansible 配置 | 展示运行节点、部署拓扑与连接 | `diagrams/deployment.drawio` |
| 包图 | package | 目录分层、命名空间、import 关系 | 展示模块分组与耦合关系 | `diagrams/package.drawio` |
| 复合结构图 | composite-structure | 核心组件内部结构、端口与连接器 | 展示组件内部协作结构 | `diagrams/composite-structure.drawio` |
| 配置文件图 | configuration | `.env`、yaml/toml/json、配置中心 | 展示配置来源、覆盖顺序与依赖 | `diagrams/configuration.drawio` |
| 用例图 | use-case | PRD、用户旅程、角色权限模型 | 展示角色与系统能力边界 | `diagrams/use-case.drawio` |
| 活动图 | activity | 业务流程、编排逻辑、状态流转代码 | 展示流程步骤、分支、并发与泳道责任 | `diagrams/activity.drawio` |
| 状态机图 | state-machine | 状态枚举、状态迁移规则、事件处理 | 展示对象生命周期与转换条件 | `diagrams/state-machine.drawio` |
| 时序图 | sequence | API 调用链、事件时序、日志链路 | 展示消息顺序与调用因果 | `diagrams/sequence.drawio` |
| 通信图 | communication | 服务交互关系、协议与通道 | 展示对象间通信连接关系 | `diagrams/communication.drawio` |
| 交互概述图 | interaction-overview | 跨场景交互编排、子序列组合 | 展示高层交互流程 | `diagrams/interaction-overview.drawio` |
| 时间图 | timing | 实时事件、超时重试、SLA/SLO 约束 | 展示时间约束与状态随时间变化 | `diagrams/timing.drawio` |

## 适用性判定建议

- `required`：该图直接支撑当前沟通目标，如评审、交接、论文答辩、上线前审查。
- `recommended`：该图能显著降低理解成本，但当前不是阻塞项。
- `not-applicable`：项目阶段、证据完整度或沟通目标不支持当前图型。

## 输出建议

对每张被判定为 `required` 或 `recommended` 的图，至少交付：

- `diagram-briefs/<diagram>.md`
- `diagrams/<diagram>.drawio`

如用户明确要求导出格式，可继续交付：

- `diagrams/<diagram>.drawio.svg`
- `diagrams/<diagram>.drawio.png`
- `diagrams/<diagram>.drawio.pdf`
- `diagrams/<diagram>.drawio.jpg`

## E-R 图（陈氏画法）

E-R 图是数据建模补充产物，不计入 UML 14 图型总数。  
当项目存在核心领域数据模型、复杂关系或数据库重构需求时，建议输出 `er-chen.drawio`。其数据来源必须是实际的数据库表设计。

| 图型 | 记法要求 | 最低交付 |
|---|---|---|
| E-R 图 | 陈氏画法（Chen Notation）+ `drawio` 原生源文件 | `diagram-briefs/er-chen.md`、`diagrams/er-chen.drawio` |

详细规范见 [notation-standards.md](./notation-standards.md)。

## 命名规范建议

- 文件名使用英文标识：`class.drawio`、`deployment.drawio`。
- 图标题可用中文业务语义，避免纯技术缩写。
- 每张图都保留“证据来源”和“禁画事项”说明，便于后续维护。
- 对课程作业、实验报告、毕业设计场景，优先保证图名与正文中的章节名称一致。
