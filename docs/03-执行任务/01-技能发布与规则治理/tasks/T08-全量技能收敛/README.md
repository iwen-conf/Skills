# T08 全量技能收敛

- 状态：`[x]`（本地门禁与安装已验证；宿主真实路由、远程服务见"未验证"）
- 来源：[02 全量已安装技能架构评审](../../../../01-系统设计/02-全量技能架构评审.md)，用户 2026-10-08 指示"你直接干"
- 范围：本仓全部技能家族、5 个宿主的技能目录、`~/.agents/skills`、4 份全局指令（Codex、Claude、Grok、Pi）
- 不做范围：宿主内置技能（Claude `/code-review` 等、Codex `.system`）；Lazycat、apifox-cli、lanhu-design 的上游仓库；
  未在本机的 lark-cli、sunmao 仓库

## 完成内容

| 阶段 | 结果 | 证据 |
|---|---|---|
| P0 止血 | 全局指令改用现行名称，6.6K → 2.4K token；Grok 的 arc-dbqual 段与画板路径改指现行技能；Android 远程测试步骤搬入 `docs/android-execution.md`（sudo 口令只留在全局文件）；夹具、ponytail、过期副本、孤儿、失效链接移入备份 | `grep` 无退役名；`link_hosts.py --check` 全 OK |
| P1 Arc 内收敛 | closure 并入 arc:plan model 模式（`scripts/closure/`，新路径 `docs/01-系统设计/业务闭环/`，旧 `docs/arc/` 兼容）；删除 Trigger 节；`workflow.md` 改为按需；`coding-discipline.md`、`go-backend-structure.md` 拆出；code-rot G 家族；web-perf 并入 verify；场景加载上限 | closure 自测 25 项通过；5 个 SKILL.md ≤3.5K；8 个场景在上限内 |
| P2 自有家族 | research-write（read+learn+write+AnySearch）；check 覆盖任意宿主的分支与 PR；cnb（13→1，cnb-code-commit 删除）；harmonyos（3→1，官方文档检索的 key 移到 `~/.config/harmonyos-docs/`，仓内配置不含 key）；lazycat（5→1，本仓接管真源） | 安全建分支脚本由测试实际执行；官方文档离线自测 PASS |
| P3 路由基础设施 | 打包器 `metadata.library`、`ENTRY.md`、禁止单装库成员、嵌套 SKILL.md 检查、`node_modules` 跳过、目录链接修复 | 3 个新打包测试 |
| P4 上游与第三方 | lark（29 条目）、android（19）、ui-design（13）、cloudflare（14，取最新副本）、threejs（25） | 5 个路由完整打包 |
| P5 路由回归 | `scripts/route_probe.py` + 63 条语料，覆盖 22 个入口的相邻对 | 第 3 轮 63/63 通过（报告 `.arc/artifacts/T08/route-probe-3.json`） |
| P6 安装 | `~/.agents/skills` 20 个顶层入口；Claude 18、Codex 21、Gemini 3、Grok 8、Pi 3；100 个库条目；无嵌套 SKILL.md | `sync_skills.py --check --also-root` 通过；`link_hosts.py --check` 全 OK |

门禁：`validate_skills.py` 通过（16 个可发现入口、48 个仓内库条目）；`pytest` 129 通过、1 跳过（R8 protobuf 环境，既有跳过）。

## 偏离方案之处

| 方案 | 实际 | 原因 |
|---|---|---|
| Arc 场景加载 ≤13K | Go 修复 15.1K、全栈 API 17.5K、PR 评审 8.8K；上限按实测设在 9.5K–18K | 再降需要删除实质规则；门禁防止回升 |
| Claude description 约 2.3K | 2,973 token | 路由 description 要覆盖整个领域的触发词与 URL 模式 |
| 4 个飞书重定向空壳删除 | 保留为 lark 库条目 | 它们带 23 份老 references，lark-doc 仍链接 lark-note |
| 第三方内容纳入版本库 | `Library/` 子目录 git-ignored，仅 README 入库 | 13 个 UI 技能无许可证，仓库远程是 GitHub |
| Three.js 25 个合并为 8 份 references 并删 API 表 | 原文放入路由库，按症状路由 | 改写第三方内容的收益不抵风险；库条目不占发现预算 |
| 项目级技能放入对应仓库 | 移入备份 | lark-cli、sunmao 仓库不在本机 |
| 落地页请求必须首选 arc-code | 语料接受 arc-code 或 ui-design | ui-design 不写代码、不定技术栈，产出 DESIGN.md 后交给 arc-code，两条路径都不再与 Arc 冲突 |

## 未验证

- 宿主真实路由：探针只模拟"仅凭 description 选技能"，不是宿主的选择逻辑。
- Codex/Gemini 是否直接扫描 `~/.agents/skills`：未核实；已按保守口径把该目录收敛为 20 个入口。
- 鸿蒙官方文档在线检索：Meilisearch 健康检查超时，记 NOT_RUN。
- 新路由在真实任务中的效果（例如 lark 的二级路由选择）：未在真实项目上运行。

## 回滚

- 安装前全量快照：`~/.skills-backup/2026-10-08-T08/hosts-and-agents.tgz`（5 个宿主目录、`~/.agents/skills`、Codex/Claude 全局指令）；Grok/Pi 全局指令另存同目录。
- 移出的条目：`~/.skills-backup/2026-10-08-T08/retired/`，宿主链接：`~/.skills-backup/2026-10-08-085430-link-hosts/`，被替换的包：`~/.agents/skills-backup-n8s30m5z/`。
- 仓内改动按用户决定不提交，可用 `git diff` / `git status` 审阅。

## 用户决定（2026-10-08）

- Grok 全局指令里的 pnpm 与飞书画板两条规则不同步到其他宿主。
- chinese-novelist 不保留：备份目录副本已删除，安装前快照中的 76 个条目已剔除（快照 5119 → 5043 项）。
- 本次改动不提交。
