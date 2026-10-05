# T02-SKILLS-01 CNB执行合同

状态：`[x]`

上级：[技能工作包](README.md)。所属端：技能/工具。优先级：P0。难度：D3，Git基点、HTTP失败与部分审查共同影响结果可信度。前置依赖：无；共享文档变更由本工作包统一核对。

## 范围与执行

以[原始审计](../audit-source.md)中本家族每项为验收映射；先读入口与关联脚本，再修错误路径、收敛范围/停止规则，最后核对引用。无问题项保留。禁止用免责声明覆盖仍存在的错误示例。

<Context-Queries>

```bash
arc-idx files 'CNB/' --format json
arc-idx files 'SKILL.md' --format json
```

</Context-Queries>

## 验证

`.venv/bin/python scripts/validate_skills.py`；新增/修复脚本执行相应反例；最终 `.venv/bin/python -m pytest tests -q` 与打包 dry-run。只在本地修订验收全部满足后标完成；外部行为验证单列 NOT_RUN。

实现已修改，等待整体校验与 converter/分支反例。

## 验收终态

2026-10-05：本地修订验收完成，80入口校验、82项测试、79运行包 dry-run 及730可达资源链接检查通过。逐项映射和未执行平台范围见 [修订报告](../remediation-report.md) 与 [处置清单](../remediation.csv)；不声明远程 API、原生设备或真实 Agent 行为通过。
