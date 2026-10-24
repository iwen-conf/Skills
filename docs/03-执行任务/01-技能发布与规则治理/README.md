# 技能发布与规则治理

需求：[Skill 可用性与规则优化](../../00-产品需求/01-Skill可用性与规则优化.md)。
设计依据：[Skill 工程契约](../../skill-engineering.md)、[执行真实性](../../execution-truth.md)。

先读 [前置约束](00-前置约束.md) 和 [进度跟踪表](进度跟踪表.md)，再执行 [T01 工具与技能治理](tasks/T01-工具与技能治理/README.md)。

本任务只涉及技能内容、本机发布工具及验证；没有产品前端或服务端 API。按安装完整性、校验行为、文档规则、本机集成四个可验证结果拆分。安装需要在统一规则完成后最终重跑。

本次全量修订：[T02 全量审计修订](tasks/T02-全量审计修订/README.md)。

Arc 架构重构：[T03 Arc 架构重构](tasks/T03-Arc架构重构/README.md)，设计依据 [ARC 技能架构重构评审](../../01-系统设计/01-ARC技能架构重构评审.md)。

Arc 业务闭环联合：[T04 Arc 闭环联合与本机安装](tasks/T04-Arc闭环联合/README.md)，共享协议见 [workflow.md](../../../Arc/_shared/workflow.md)。

Waza 精选整合：[T05 Waza 精选整合与安装](tasks/T05-Waza精选整合/README.md)，逐项取舍见 [Waza 清单](../../../Waza/README.md)。

HTTP API 设计审查：[T06 HTTP API 设计审查与安装](tasks/T06-HTTP-API设计审查/README.md)，入口为 `arc:audit` 的 `api-design` 模式，设计/实现/验证共享规则。

原生客户端方向：[T07 原生平台方向与安装](tasks/T07-原生平台方向/README.md)，替换跨平台 UI 默认，按原生目标规划、实现与验收。

全量技能收敛：[T08 全量技能收敛](tasks/T08-全量技能收敛/README.md)，设计依据 [全量已安装技能架构评审](../../01-系统设计/02-全量技能架构评审.md)；162 个技能目录收敛为 22 个入口。

业务流测试体系：[T09 业务流测试体系](tasks/T09-业务流测试体系/README.md)，设计依据 [ARC 测试技能业务流审计](../../01-系统设计/03-ARC测试技能业务流审计.md)；目录见 [业务流总清单](../../07-测试质量/业务流/README.md)。
