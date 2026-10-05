# 技能目录与分发边界

2026-10-05 Arc 重构后基线：69 个运行入口，0 个来源包，0 个归档入口。T02 审计时的 80 项清单见 [remediation.csv](03-执行任务/01-技能发布与规则治理/tasks/T02-全量审计修订/remediation.csv)；其中 16 个 Arc 入口与 1 个来源包已按 [ARC 技能架构重构评审](01-系统设计/01-ARC技能架构重构评审.md) 合并为 5 个 Arc 技能，`arc:docs` 迁入 Lark 家族。

| 角色 | 范围 | 数量 | 校验 / 安装 |
| --- | --- | ---: | --- |
| active | Arc 5、Android 19、HarmonyOS 2、CNB 14、Lark 29 | 69 | 默认校验并可按 family/only 打包 |
| source | 无（reliability-engineering-skill 已归档到 [docs/archive](archive/reliability-engineering/)） | 0 | — |
| archive | 当前无归档 SKILL.md | 0 | 不把 references、SKILL.zh.md 或历史审计当运行入口 |

运行目录与来源名单由 [skill.py](../src/arc_core/domain/skill.py) 维护；[校验器](../src/arc_core/usecase/validate_skills.py) 分别收集并统计。Arc 路由索引与触发词表只覆盖 Arc，其通过不代表其他家族已通过行为验证。

Arc 目录只允许技能包和 `Arc/_shared/`；`_shared/` 没有 `SKILL.md`，不被发现，被引用的文件由打包器复制进引用它的技能包。PostgreSQL 发布资格工具包作为 `arc:verify` 的 db 模式随包安装（`scripts/dbqual/`），其中文说明在 [dbqual/README.zh-CN.md](dbqual/README.zh-CN.md)。

[sync_skills.py](../scripts/sync_skills.py) 将声明的技能依赖随包安装，并复制可达文件/共享文档。跨技能的可选路由链接不自动激活相邻技能；必需入口必须写 metadata.requires.skills（或 siblings）。例如 perfetto-trace-analysis 显式依赖 perfetto-sql。

完整 family dry-run 验证所有运行包的可达资料。未执行远程 API、原生应用或真实 Agent 行为时，报告 NOT_RUN，不用静态校验代替。
