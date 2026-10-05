# 归档：reliability-engineering-skill v3.0

2026-10-05 Arc 2.0.0 重构时归档，不参与技能发现、校验或安装。

- 行为规则已并入 `arc:verify`：主张表、独立 oracle、敏感性证明、事务与故障、发布门禁、运营学习，见 [`Arc/arc:verify/SKILL.md`](../../../Arc/arc:verify/SKILL.md)。
- `GUIDE.md` 是原 `SKILL.md`，仅改名以免被宿主发现；`MANIFEST.sha256` 的对应条目同步改名，哈希仍是上游 v3.0 原文。`GUIDE.md` 含 T02 审计修订，因此该条校验不通过属预期，其余条目一致。
- `industry-testing-strategies.md` 来自原 `arc:reliability/references/`，作为出处保留。
- `scripts/`、`templates/`、`examples/` 与 `.reliability/` 项目布局未接入 `arc:verify`；需要时从这里复制到项目，或参考 [arc:verify 的 db 模式](../../../Arc/arc:verify/references/db.md) 获得 PostgreSQL 资格验证脚本。
