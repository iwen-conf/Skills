# CLAUDE.md 生成计划

## 生成清单

| 序号 | 目录 | 层级 | 显著性评分 | 生成顺序 |
|------|------|------|----------|---------|
| 1 | simulate/ | 模块级 | 8 | 1 |
| 2 | triage/ | 模块级 | 6 | 2 |
| 3 | loop/ | 模块级 | 6 | 3 |
| 4 | review/ | 模块级 | 5 | 4 |
| 5 | deliberate/ | 模块级 | 5 | 5 |
| 6 | init/ | 模块级 | 5 | 6 |
| 7 | agent/ | 模块级 | 5 | 7 |
| 8 | refine/ | 模块级 | 4 | 8 |
| 9 | ./ | 根级 | - | 9（最后） |

## 排除目录
| 目录 | 排除原因 |
|------|---------|
| .git/ | 版本控制元数据 |
| .arc/ | 工作目录，不应生成文档 |
| simulate/reports/ | 运行时输出，含敏感信息 |
| simulate/.ace-tool/ | 工具缓存 |

## 评分依据
- **simulate/** (8分): 有 scripts(6个) + templates + examples + reports + 显著性最高
- **triage/** (6分): 有 scripts + references + 与 simulate 强关联
- **loop/** (6分): 有 scripts + assets + references
- **review/** (5分): 有 references/dimensions.md + 完整 SKILL.md
- **deliberate/** (5分): 完整 SKILL.md + OpenSpec 集成
- **init/** (5分): 有 references + 完整 SKILL.md
- **agent/** (5分): 元技能，统一入口
- **refine/** (4分): 轻量级技能，仅 SKILL.md

## 特殊说明
- 根级 CLAUDE.md 已存在，将增量更新而非覆盖
- 保留现有 Changelog 历史
- 模块级 CLAUDE.md 需要面包屑导航指向根级
