# T03 Arc 架构重构

状态：`[x]`。日期：2026-10-05。设计依据：[ARC 技能架构重构评审](../../../../01-系统设计/01-ARC技能架构重构评审.md)，实施偏差见其附录 C。

## 目标与范围

- 16 个 Arc 技能、仓库外 `arc:dbqual`、`reliability-engineering-skill` 合并为 5 个：`arc:explore`、`arc:plan`、`arc:code`、`arc:verify`、`arc:audit`。
- `arc:docs` 迁为 `Lark/lark-workflow-project-docs`；`arc:prewalk` 与 ponytail 前置删除。
- 新增不可发现的共享层 `Arc/_shared/` 与 `arc:code/scripts/gate.sh`；校验器强制 11 个固定标题、Token 预算、"Not for"、退役名与目录布局。
- 不做：别名、提交或推送、第三方技能的重复与断链清理（T01-TOOLS-06）。

## 结果

| 子项 | 状态 | 证据 |
|---|---|---|
| 5 个 Arc 技能与共享层 | `[x]` | 校验 69/69 通过；单技能 dry-run 均通过 |
| dbqual 纳入为 `arc:verify` db 模式 | `[x]` | 包内 43 项单测通过；`CHECKSUMS.sha256` 134 行全部一致 |
| gate.sh | `[x]` | `tests/test_gate_script.py` 17 项通过（macOS bash 3.2） |
| lark-workflow-project-docs | `[x]` | `.lark.json` 格式不变；`.cursor/skills` 链接已加 |
| 退役目录与文档删除、可靠性包归档 | `[x]` | 删除前完整备份 `.arc/backups/arc-restructure-20261005-135750.tar.gz` |
| 校验器、语料、测试、索引、README、CHANGELOG | `[x]` | 仓库测试 108 passed |
| 本机安装 | `[x]` | 见下节与 [验证记录](evidence/verification.txt) |

## 本机安装

- 69 个运行技能安装到 `/Users/iluwen/.agents/skills`（dash 命名）；被替换的旧包在 `/Users/iluwen/.agents/skills-backup-30k98oe2`。
- 16 个退役包（15 个旧 Arc + `arc-dbqual` 原始来源）移到 `/Users/iluwen/.agents/skills-retired-arc-yg05f5h_`，未删除。
- Claude、Codex 各移出 16/17 个旧链接，新建 `arc-explore`、`arc-plan`、`arc-code`、`arc-verify`、`lark-workflow-project-docs`，保留 `arc-audit`。Grok 移出 `arc-dbqual` 与已断开的 `arc:prewalk`，新建 `arc-verify` 替代 dbqual。
- 各根的备份目录与 `manifest.json`（saved / created / retained）见 [宿主链接清单](evidence/installation-hosts.json)。

恢复方法：按 manifest 移出 `created` 链接、把 `saved` 移回原根；shared 根把 `skills-retired-arc-*` 中的包移回，并用 `skills-backup-30k98oe2` 替换本次安装的同名包。

## 未验证

- 宿主真实路由与 Agent 行为；新技能在下一轮对话重新发现后生效。
- DAST、外部扫描器、PostgreSQL 资格脚本在真实目标上的运行；shellcheck 未安装。
- `docs/archive/reliability-engineering/MANIFEST.sha256` 中 `GUIDE.md` 一条保留上游哈希，与含 T02 修订的文件不一致，属预期。
