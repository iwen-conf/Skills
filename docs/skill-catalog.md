# 技能目录与分发边界

2026-10-08 全量收敛后（[02 全量已安装技能架构评审](01-系统设计/02-全量技能架构评审.md)，任务 T08）：本机唯一技能入口 22 个，其中本仓 17 个可发现入口、仓外 5 个。原 162 个唯一目录、204 份磁盘副本中，其余内容要么作为路由的库条目随包安装（宿主不可发现），要么移入 `~/.skills-backup/2026-10-08-T08/`。

## 本仓可发现入口（17）

| 层 | 入口 | 来源 | 库条目 |
| --- | --- | --- | ---: |
| 常驻核心 | `arc:explore`、`arc:plan`（含 model 模式，原 arc:closure）、`arc:code`、`arc:verify`（含 web-perf）、`arc:audit`、`arc:vet`（原 open-source-vetting-kit） | `Arc/` | — |
| 常驻核心 | `check`、`health`、`research-write`（原 read、learn、write + AnySearch） | `Waza/` | — |
| 领域路由 | `lark` | `Lark/lark`，库条目为同目录 29 个上游与自有技能 | 29 |
| 领域路由 | `android` | `Android/android`，库条目为同目录 19 个 Google 官方技能 | 19 |
| 领域入口 | `cnb`（原 13 个 CNB 技能为 references；cnb-code-commit 删除） | `CNB/cnb` | — |
| 领域入口 | `harmonyos`（原 harmonyos-development、arkts-syntax-assistant、harmonyos-official-docs） | `HarmonyOS/harmonyos` | — |
| 领域入口 | `lazycat`（原 5 个阶段技能为模式） | `Lazycat/lazycat` | — |
| 领域路由 | `ui-design` | `Design/ui-design`，库源 `Library/design/`（本机，git-ignored） | 13 |
| 领域路由 | `cloudflare` | `Cloudflare/cloudflare`，库源 `Library/cloudflare/`（本机） | 14 |
| 领域路由 | `threejs` | `ThreeJS/threejs`，库源 `Library/threejs/`（本机） | 25 |

## 仓外入口（6）

| 入口 | 位置 | 说明 |
| --- | --- | --- |
| `typesafe-ai` | `~/.agents/skills/typesafe-ai` | TypeSafe 官方技能；全局规则第 6 节强制使用 |
| `agent-browser` | `~/.agents/skills/agent-browser` | 上游桩，内容由 `agent-browser skills get` 按版本下发；core/dogfood/electron/slack/agentcore 不再另装 |
| `easyeda-api` | `~/.agents/skills/easyeda-api` | 第三方 |
| `swiftui-pro` | `~/.agents/skills/swiftui-pro` | 第三方（MIT），插件布局的嵌套旧版已移除 |
| `lanhu-design` | `~/Documents/Code/Go/lanhu-cli/skills/lanhu-design` | 随 CLI 仓发布 |
| `humanizer-zh` | `~/.agents/skills/humanizer-zh` | 第三方（MIT，op7418/Humanizer-zh）；中文润色，Claude/Codex/Grok extra |

## 宿主安装面

`schemas/host_profiles.yaml` 是唯一来源，`scripts/link_hosts.py --check` 校验：Claude 20、Codex 22、Grok 10 个入口（2026-10-24 移除未使用的 Gemini、Pi 宿主配置；2026-10-27 增加 `humanizer-zh`）。打包器只安装路由与独立入口；库成员不可单独安装，`--check` 发现残留独立副本即报错。Cloudflare `security-audit` 已于 2026-10-24 撤出全部宿主（含 DSH、Cursor）并删除本机副本，只以上游 GitHub 作为 `arc:audit` 的学习源，见 [04 安全审计上游学习](01-系统设计/04-安全审计上游学习.md)。

## 校验

`scripts/validate_skills.py` 区分可发现入口与仓内库条目；Arc 另有触发语料（`schemas/trigger_corpus.yaml`）、行为语料与场景加载上限（`ARC_LOAD_SCENARIOS`）。跨家族路由由 `scripts/route_probe.py` 与 `schemas/route_corpus.yaml` 覆盖全部 22 个入口。未执行远程 API、原生应用或宿主真实路由时，报告 NOT_RUN，不用静态校验代替。
