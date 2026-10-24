# T04 Arc 闭环联合与本机安装

状态：`[x]`。日期：2026-10-06。所属端：技能/工具；优先级 P1；难度 D3。

## 目标与范围

用户要求：“把 Arc/arc-closure 和我们已有的arc进行联合……然后把arc的skill安装在本机”。

以现有五技能为基础加入 `arc:closure`，统一阶段交接、文档归属与证据映射，修复会阻断联合使用的闭环门禁和打包问题，验证后安装六个 Arc 技能。本轮保留用户新加入的 closure 内容；不提交、不推送、不安装无关家族，不运行真实数据库或远程业务测试。

难点：设计/实现/验证权限、已有授权和只读审查不能在阶段切换中丢失；模型门禁不能冒充运行时证明；源目录与 canonical name 不同仍需完整可移植安装。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'Arc/*' --format json
arc-idx files '*install*' --format json
arc-idx files 'tests/*' --format json
arc-idx search 'f:^Arc/ "never load"' --format ai-md --context 2 --max 20
```

</Context-Queries>

前述查询已执行。后续执行者需重跑并读取新状态；最后一项用于确认旧的禁止跨阶段加载约定已被替换。

## 执行与验收

| 工作 | 状态 | 验收 |
|---|---|---|
| 六技能及共享交接合同、文档归属和版本 | `[x]` | 70 入口校验通过；6 个包共享引用可打包；A/B/C 独立场景推演完成并修正三处冲突 |
| 闭环 checker 门禁与脚本、模板、语料、可移植安装回归 | `[x]` | checker 24 组自测通过；仓库 109 passed / 1 skipped；警告/不完整枚举/旧指纹拒绝用例通过 |
| 本机安装与宿主发现文件 | `[x]` | Arc dry-run → 安装 → check 通过；installed init/check/gate/selftest 通过；三宿主根六技能均为 2.1.0 |

## 命令

```bash
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
.venv/bin/python Arc/arc-closure/scripts/selftest.py
.venv/bin/python scripts/build_skills_index.py
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --dry-run
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --check
```

本轮主机：本机工作站；共享安装根 `~/.agents/skills`，Codex 与 Claude 通过链接使用同一份技能。
审查业务场景与真实宿主自动路由需另行观察；本轮可核验包内容、命令入口和文档/行为语料。

## 证据与交接

- `scripts/validate_skills.py`：70/70 运行入口通过；`build_skills_index.py` 已生成六技能索引。
- `python -m pytest tests -q`：109 passed，1 skipped。跳过项是既有 R8 protobuf 环境要求，与 Arc 联合无关。
- `Arc/arc-closure/scripts/selftest.py`：24 组通过，涵盖模型反例、WARN 绕过、结构豁免后未执行模型、截断、旧/坏状态、spec/checker 变更、审查输出位置和副本更新。
- `gate.sh check --task arc-closure-integration --max-files 60 --strict`：0 FAIL，2 NOT_RUN。报告：`.arc/artifacts/arc-closure-integration/reports/gate-20261006T130833Z.txt`。未装 ruff、mypy/pyright，格式和类型检查未运行；未为此新增环境依赖。
- 六技能已安装到 `/Users/iluwen/.agents/skills`，`--check` 通过；Codex 的 closure 链接已补齐，Codex/Claude 的其他链接沿用共享根。
- 最初旧包备份：`/Users/iluwen/.agents/skills-backup-ulnm3p1b`；末次文档修订前副本：`/Users/iluwen/.agents/skills-backup-fedwbg05`。
- 使用系统 `python3`（PyYAML 6.0.3）在仓外临时目录运行真实安装包的 init/check/gate/selftest，均通过；没有注入 CLAUDE.md 或审查文档。证据：`.arc/artifacts/arc-closure-integration/reports/installed-smoke.json`。
- 独立前向推演覆盖只读退款审查、checker 升级后连续实现/测试、已有 PRD 仅拆任务。已修复阶段入口基线、audit 误用实现 gate、常驻规则旧 checker 三处问题。
- TypeSafe 本回合判断：闭环归属 `noul=0.95`、任务规划归属 `0.92`、已有授权连续交接 `0.89`；这些是语义辅助，不代替权限或门禁。
- NOT_RUN：宿主下一轮自动选择的真实行为、真实业务应用/数据库的端到端验证。本轮任务交付的是技能包及本机可执行入口，不把模型自测称为业务发布资格。
