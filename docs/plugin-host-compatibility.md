# Plugin 宿主兼容合同

核对日期：2026-11-04。本文记录官方接口与本仓适配范围；不把“文件存在”“单元测试通过”或“宿主宣称兼容另一产品”当作真实会话验收。

宿主加载状态以实际安装版本的诊断输出为准。0.4.0 的 `plugins/arc-policy/` 分发 policy runtime、`arc-policy` 入口、六个工作流技能，以及固定版本的官方 `arch-diagram`。六技能的唯一源码仍在 `Arc/`，由 `scripts/build_arc_plugin.py` 复用既有打包器生成完整资源闭包。Python、`arc-idx` 和按项目启用的 `arc-verify` 是单独诊断的外部前提；绘图按需使用 Node 与本地浏览器。

## 证据层次

| 检查 | 能证明什么 | 不能证明什么 |
|---|---|---|
| 本仓 schema、链接与 Python 测试 | 文件格式、路径关系、给定 payload 下的适配器行为 | 宿主已加载、信任或执行插件 |
| 宿主插件详情与发现清单 | 当前版本实际发现的技能、hooks、来源、启用与信任状态 | 模型已收到上下文、工具实际被拒绝 |
| 隔离仓库内的真实宿主事件 | 对应宿主版本的一次输入、输出与实际控制结果 | 所有版本、所有事件、子 Agent 或其他运行模式都相同 |

测试记录应包含宿主版本、插件版本、加载路径、事件、可观察结果。不要把构造的 Claude 格式 payload 标注为 Grok 或 Codex 实机会话。

## Grok Build

### 产品、来源与版本

产品是官方 **Grok Build**，安装后的命令为 `grok`。官方 macOS / Linux / WSL 安装入口是 `https://x.ai/cli/install.sh`，Windows 使用 `https://x.ai/cli/install.ps1`；不是第三方同名 `grok-cli` 项目。

本次同时核对了[官网入门](https://docs.x.ai/build/overview)、[CLI reference](https://docs.x.ai/build/cli/reference)和官方 [xai-org/grok-build](https://github.com/xai-org/grok-build) Rust 实现。源码证据固定于 commit `2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`，对应 `SOURCE_REV` 为 `559751fdcec02d413e4c57c8832ab275e4f44980`。开源镜像与安装的发行版不能直接画等号。

[官方 changelog](https://x.ai/changelog/build) 在核对日显示最新版本为 `1.0.70`（2026-10-23）。相关发布节点：

| 版本 | 官方记录与适配影响 |
|---|---|
| `0.2.107` | Stop hooks 可通过反馈继续 Agent；更早版本不能据此承诺相同门禁 |
| `1.0.59` | 修复 hook 的 `continue: false` 被忽略 |
| `1.0.64` | 未信任文件夹只运行 managed hooks；headless 的 user hooks 有单独信任行为 |
| `1.0.70` | 插件列表只统计实际加载的技能，并报告因链接越出插件目录而跳过的数量 |

这些节点不构成所有功能的统一最低版本。部署前记录 `grok --version`；本次源码核对没有证明用户机器已安装上述版本。

官网 [Hooks 概述](https://docs.x.ai/build/features/hooks) 仍保留“只有 PreToolUse 阻断”的旧说明。下面的 Stop 合同以官方仓库 [Hooks guide](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-pager/docs/user-guide/10-hooks.md)、实现与 changelog 为证据，不能沿用旧概述判定 Stop 不受支持。

### 安装、manifest 与启用

官方命令支持以下路径；`--trust` 是执行安装的明确选择，不是诊断选项：

```bash
grok plugin marketplace add iwen-conf/Skills
grok plugin install arc-policy --trust
grok plugin enable arc-policy
grok plugin validate ./plugins/arc-policy
grok plugin details arc-policy
grok inspect --json
```

也可以直接安装本地 `./plugins/arc-policy`。添加 marketplace 只登记来源；加载还取决于安装、启用和信任。[官方插件指南](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-pager/docs/user-guide/09-plugins.md) 定义了 `.grok-plugin/marketplace.json`，本仓的 `{"type":"local","path":"./plugins/arc-policy"}` 是受支持的 source 格式。

[manifest.rs](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-agent/src/plugins/manifest.rs) 定义的查找顺序是根 `plugin.json`、`.grok-plugin/plugin.json`、`.claude-plugin/plugin.json`；找到一个后不合并后面的文件。不存在 manifest 时仍可按约定目录发现组件。未知字段忽略；没有原生 `hooks` 字段时读取 `hooks/hooks.json`。

因此，`extensions.com.openai.hooks` 不是 Grok 的 hooks 配置接口，但本仓约定路径仍能加载 hooks。不要为了这个差异复制另一份无效的 fallback manifest。插件名称使用小写字母、数字与连字符，最长 64 字符。

### 技能名称、frontmatter 与多来源发现

依据：[Skills 官网](https://docs.x.ai/build/features/skills-plugins-marketplaces)、[官方技能指南](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-pager/docs/user-guide/08-skills.md)、[frontmatter parser](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-tools/src/implementations/skills/discovery.rs) 与 [model invocation 判定](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-tools/src/types/resources.rs)。

| 字段或机制 | 已核实合同 |
|---|---|
| `name` | 最长 64 字符；当前 parser 将非 ASCII 字母数字正规化为连字符，包括 `arc:verify` → `arc-verify` |
| `description`、`when-to-use` | 提供用途与触发信息；后者也接受 `when_to_use` |
| `user-invocable: false` | 当前实现不仅隐藏 slash command，也不允许模型通过 skill tool 自动调用 |
| `disable-model-invocation: true` | 保留用户显式调用，禁用模型自动调用 |
| `allowed-tools` | 解析为工具列表，不是宿主工具权限边界 |
| `paths`、`argument-hint`、`metadata` | 文件路径触发、命令提示与显示元数据；未知键忽略 |
| `model`、`effort` | parser 保存字段；官网与仓库指南对是否应用 override 的表述不同，本文不承诺它们控制运行模型 |

本仓打包器默认将 Arc 名称及正文引用发布为连字符形式，并把版本移入 `metadata`；不要为了 Grok 改写 canonical 源码里的整个命名体系。

Grok 同时扫描项目和用户层的 `.grok/skills`、`.agents/skills`，并默认兼容 Claude、Cursor 技能目录；另有启用插件与 `[skills].paths`。按规范化路径与名称去重，因此 `~/.grok/skills` 的链接数量不等于宿主最终发现的技能数量。

需要限制发现范围时使用官方 `[skills].disabled`、精确 `ignore` 或兼容开关，并检查 `grok inspect --json`。不要先忽略整个 `~/.agents/skills`，再期望指向该目录的 Grok symlink 仍能加载：[路径过滤实现](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-agent/src/prompt/skills.rs) 会解析 symlink 后比较 canonical path。

插件边界与用户技能链接不是同一规则。插件组件必须留在插件包内；不能用指向外部 `Arc/` 的 symlink 充当可移植插件包。需要打包六技能时使用发布后的完整文件与引用资源。这个限制不意味着现有用户层 `~/.grok/skills` → `~/.agents/skills` 链接本身无效。

### Hook 输入、输出与环境

官方 [runner](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-hooks/src/runner/command.rs) 和 [输出解析器](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-hooks/src/runner/mod.rs) 是控制结果的依据。

stdin 是 JSON。`hookEventName` 使用 `pre_tool_use` 等 snake_case 值；兼容键 `hook_event_name` 使用 `PreToolUse` 等 PascalCase 值。会话和工具字段主要是 `sessionId`、`cwd`、`workspaceRoot`、`toolName`、`toolInput`、`toolUseId`、`toolInputTruncated`。Stop 带 `stopHookActive` 与 `lastAssistantMessage`。本仓适配器已支持这些主要 camelCase 字段。

| 事件 | 官方输出行为 | 本仓覆盖范围 |
|---|---|---|
| `SessionStart` | Observe 事件不把 `additionalContext` 注入模型；`systemMessage` 可显示为用户 annotation | 状态初始化、CLI/任务提示；不承诺自动向模型加载工作流 |
| `PreToolUse` | 原生 `decision: deny` 或 `hookSpecificOutput.permissionDecision: deny` 阻断；exit 2 也可拒绝 | 编辑及受控命令检查；依赖宿主实际加载并运行 hook |
| `PostToolUse` | 工具已运行；可以给模型反馈，不撤销操作 | 记录工具窗口和写入归属 |
| `Stop` | `decision: block` 加 reason 可继续本轮；`additionalContext` 也可请求继续；`continue: false` 强制停止 | 主 Agent 的结束检查；非 `end_turn` 通知不触发验收 |
| `SubagentStop` | 子 Agent 的结束 gate；插件层的 Stop 不自动等价于它 | 当前不注册该事件；需先设计子会话、任务与文件归属再启用 |

SessionStart 的差异可直接追踪至 runner 的 `GateKind::Observe` 分支与 [hook_dispatch.rs](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/session/acp_session_impl/hook_dispatch.rs)：`systemMessage` 通过 `HookAnnotation` 发到界面，不进入模型对话。工作流规范使用已有 AGENTS.md / skill 入口，不依赖该通知隐式加载。

Stop 最多允许 8 次 continuation，超限结束；错误、取消和 session teardown 有不同路径。有效的 JSON 决定优先于把 exit 0 简化为“允许”。Stop 无可用 JSON 时，exit 2 使用 stderr 请求继续；其他失败通常 fail-open。引擎成功输出明确 deny 与宿主杀死超时进程是不同情形，不能把前者的 fail-closed 承诺扩展到后者。

Hook 类型是 `command` 或 `http`。matcher 支持正式工具别名，例如 `Bash` → `run_terminal_command`、`Write` / `Edit` → `search_replace`。未知事件不会因此自动获得业务语义。

Runner 注入 `GROK_HOOK_EVENT`、`GROK_HOOK_NAME`、`GROK_SESSION_ID`、`GROK_WORKSPACE_ROOT` 和兼容别名 `CLAUDE_PROJECT_DIR`。插件额外注入 `GROK_PLUGIN_ROOT` / `GROK_PLUGIN_DATA`，以及 `CLAUDE_PLUGIN_ROOT` / `CLAUDE_PLUGIN_DATA`。[官方 plugin hook adapter](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-agent/src/plugins/hooks_adapter.rs) 直接设置并替换这四个插件路径变量。因此本仓 `${CLAUDE_PLUGIN_ROOT}` 是有效别名，无须仅因名称不同重写。

### 当前尚未证明的范围

- 用户实际安装版本、启用状态及 trust 状态，需要宿主诊断输出。
- 单元测试中的 Grok payload 不证明一次真实会话的拒绝、模型反馈或插件安装成功。
- 当前只承诺主 Agent 的 Stop 接口适配；本文不新增 SubagentStop 门禁，也不把父会话的结果当作子会话归属证明。
- 同用户插件与 hook 不是独立可信执行边界；真实交付仍须满足项目自己的验证与证据合同。

## Claude Code

依据：[插件 manifest](https://code.claude.com/docs/en/plugins-reference)、[插件使用](https://code.claude.com/docs/en/plugins)、[Hooks](https://code.claude.com/docs/en/hooks)。本轮核对的是 2026-11-04 的官方文档，未取得用户安装版本。

- 使用 `.claude-plugin/plugin.json`；技能和 hooks 资源保留在插件根的 `skills/`、`hooks/`，相对路径必须以 `./` 开始并留在包内。
- `skills` 自定义路径是默认发现之外的补充；不能把源码 checkout 外的资源或越界链接当成已打包资源。当前生成器发布六技能的真实副本。
- 技能使用插件命名空间。例：源码 `arc:code` → 包名 `arc-code` → `/arc-policy:arc-code`。保持插件 ID `arc-policy`，避免改名破坏已有配置。
- `CLAUDE_PLUGIN_ROOT` 指向安装目录；更新后目录可变化。当前文档还说明插件 `bin/` 可加入 Bash 的 PATH，但长期 Git 配置仍应使用稳定安装入口。
- `SessionStart` 可通过 `hookSpecificOutput.additionalContext` 提供模型上下文；PreToolUse 的拒绝使用 `hookSpecificOutput.permissionDecision: deny`。
- Stop 的 `decision: block` 请求继续工作；当前引擎读取 `stop_hook_active`，避免把递归 Stop 不断当新一轮。
- Claude 另有 `PostToolUseFailure` 与 `SubagentStop` 事件。本插件当前未单独注册这两类事件；失败工具的窗口在 Stop 收口，不承诺对子 Agent 的完整结束归属。

安装与静态结构验证：

```bash
claude plugin marketplace add iwen-conf/Skills
claude plugin install arc-policy@skills
claude plugin validate ./plugins/arc-policy
```

本环境没有 `claude` 可执行文件，以上原生命令与实际会话验收为 `NOT_RUN`。不能将 Python 测试标成 Claude 实机通过。

## Codex

依据：[OpenAI 插件打包](https://developers.openai.com/plugins/build/plugins)、[Hooks](https://learn.chatgpt.com/docs/hooks)、[CLI 命令](https://learn.chatgpt.com/docs/developer-commands)，并固定核对 `openai/codex` 的 `rust-v0.155.1` 与 `rust-v0.166.0`。

### 打包与信任

根 `plugin.json` 是 portable manifest；`extensions.com.openai` 指定 OpenAI 的组件配置，`.codex-plugin/plugin.json` 提供兼容描述。显式 hooks 路径取代默认 hooks 路径。本仓两处均选择 `./hooks/codex.json`，以 `--host codex` 调用同一 Python 策略入口。

原生 marketplace 是 `.agents/plugins/marketplace.json`，本仓 local source 为 `./plugins/arc-policy`，相对于仓库根。此前 `plugins/marketplace.json` 不在已核对的识别布局中，0.3.0 已移至正确位置；原有 Claude-compatible catalog 不意味着那个旧文件有效。实现依据：[marketplace.rs](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/core-plugins/src/marketplace.rs)。

```bash
codex plugin marketplace add /absolute/path/to/Skills
codex plugin add arc-policy@skills
codex plugin list --json
```

项目 `.codex/config.toml` 可使用 `[plugins."arc-policy@skills"]` / `enabled = true`；项目配置要求信任。交互式会话通过 `/hooks` 审查并信任 handlers，插件安装不授予 hook trust；脚本变更可能要求重新审查。不要以绕过 trust 的启动参数替代安装验证。

### 输入输出合同

Codex 确实提供 `PLUGIN_ROOT` 和兼容别名 `CLAUDE_PLUGIN_ROOT`；原共享配置使用 Claude 名称本身不是缺陷。主要输入是 `session_id`、`cwd`、`hook_event_name`、`tool_name`、`tool_input` 与 `tool_use_id`，不能凭猜测要求 `CODEX_PLUGIN_ROOT` 或把所有 session ID 改成 `thread_id`。

真正的错误是输出混入其他宿主字段。官方 [schema.rs](https://github.com/openai/codex/blob/rust-v0.155.1/codex-rs/hooks/src/schema.rs) 使用严格未知字段检查；[PreToolUse 实现](https://github.com/openai/codex/blob/rust-v0.155.1/codex-rs/hooks/src/events/pre_tool_use.rs) 对不符合格式的 JSON 标记 Failed，未把 `should_block` 改为 true。旧 envelope 中的 `permission`、`user_message`、`agent_message`、`additional_context`、`additionalContext`、`followup_message` 因事件不同会触发此问题。

0.3.0 在输出边界投影合法字段，正常路径、runtime 加载失败、策略异常、fallback 不可用和 protocol 模块缺失的紧急兜底都覆盖。允许的工具调用保持空 stdout 与退出 0；拒绝使用标准 `hookSpecificOutput`；Stop 只发该事件支持的决定与理由。

官方四类事件的八份输入/输出 schema 原样固定在 `tests/fixtures/codex-hooks/`，`sources.json` 记录 Git blob SHA 和来源；上述两个版本的八份 SHA 逐一相同。`events.json` 是基于合同构造的测试输入，不是伪称的实机录制。

| 事件或工具 | 当前适配边界 |
|---|---|
| SessionStart | 标准 additionalContext 与用户提示 |
| PreToolUse / PostToolUse | 匹配 Bash 和 apply_patch；前置拒绝、后置归属记录 |
| exec_command | 宿主规范化为 Bash；无需猜测一组终端别名 |
| apply_patch | 官方 handler 将 freeform patch 包装进 `tool_input.command`，现有路径解析支持 |
| write_stdin | 不重新触发 PreToolUse；已有命令结束时可能补发 PostToolUse |
| MCP / hosted 工具 | 当前 matcher 不覆盖任意 MCP；hosted 工具本身也不走相同本地 hook 路径 |
| Stop | block 请求继续验证，递归回调降为提示；不是权限拒绝 |

本环境没有 `codex` 可执行文件，因此原生安装、trust 与端到端模型回合为 `NOT_RUN`。官方 schema 合同测试和真实 Python 入口子进程证明的是适配结果，不能升级为宿主全路径证明。

## 持久运行时与升级

`arc-policy install` 离线复制当前包和已准备的 PyYAML（含许可证），先以隔离的 `python -S` 自检，再切换稳定 `current` 指针。Git hooks 引用稳定 launcher，不持有 Claude/Grok/Codex 的版本缓存路径。升级插件后重新执行安装；不自动删除仍可能被进程使用的旧快照。外部 Python 被卸载仍需重新准备运行环境。

Git hook 能力通过临时仓库实际执行探测，不把“未知配置可写入”当作支持。旧 Git 使用默认目录的文件 hook 链接原有脚本，保留参数与 pre-push stdin；有自定义 `core.hooksPath` 时明确要求支持配置式 hooks 的 Git，不覆盖用户配置。人工 Git 操作先判定 agent marker，再检查 ARC runtime 是否存在。

`doctor` 报告 `engine_ready`、`skills_ready`、`cli_ready`、`git_gate_installable`、外部工具和 `workflow_ready`。`host_hooks_ready` 保持 null，直到有真实宿主证据，不能把本地准备成功称为已启用全部门禁。

0.3.0 的会话文件键加入完整 ID 的 SHA-256，避免字符替换和截断碰撞。升级后结束旧会话并启动新会话；旧日志不继承，仓库 task/receipt 格式不变。

## 0.3.0 验证记录

验证环境：Linux、Python 3.12.14、Git 2.51.1；测试依赖准备在隔离虚拟环境中。当前持久 launcher 和 Git 兼容脚本使用 POSIX shell，适用于 macOS / Linux / WSL 路线；原生 Windows 安装与执行未认证。

| 检查 | 结果 | 范围或限制 |
|---|---|---|
| 根目录 `pytest -q` | 570 PASS / 1 FAIL / 4 SKIP | 已合入上游 `5702077` 的最新业务图/技能更新；唯一失败是修改前已有的完整家族打包缺 `cloudflare-platform` 外部源；4 跳过与 Go / gofmt / protobuf 环境有关 |
| dbqual 独立测试 | 160 PASS / 1 SKIP，139 subtests PASS | 未配置真实 SQLFluff 命令的集成项跳过 |
| 插件构建 `--check` | PASS | 六技能及引用闭包、生成元数据、版本、mode、额外文件检测；不把正常 Python 缓存当漂移 |
| 新增安装、分发、协议代码静态检查 | PASS | Ruff 与 whitespace 检查；不声称全仓原有代码都符合新检查 |
| 实际 Git 2.51.1 commit / pre-push | PASS | agent 无回执拒绝、人工操作放行、已验证提交放行；config-hook 升级迁移另用故障注入测试 |
| 全家族 `validate_skills.py` | BLOCKED / FAIL | 当前 checkout 缺少忽略管理的 Design / Cloudflare / ThreeJS 外部 Library 源包；Arc 插件内部闭包检查通过 |
| Claude / Grok / Codex 原生会话 | NOT_RUN | 环境无对应二进制；未声明安装、信任或模型实际续跑通过 |
| Go CLI race / vet | NOT_RUN | 环境无 Go；本轮未修改 `tools/arc-verify` 源码 |

这些失败和跳过未通过删除测试、放宽断言或伪造回执消除。宿主验收仍应在独立临时 Git 仓库记录：安装版本与实际加载路径、禁止编辑是否真的未落盘、task begin 后编辑、未验证 Stop 反馈、验证后的 commit，以及升级后运行路径。子 Agent 与交互式终端输入另列覆盖范围。

## 0.4.0 技能协作与绘图集成

本轮在 0.3.0 的宿主协议与持久安装基础上接入最新 Arc 技能。`arc:verify` 源码版本为 3.8.0；
其他五个 Arc 入口保持现有职责。插件提供八个可发现入口：六个 Arc 技能、policy bridge、固定上游 arch-diagram。

- `arc-policy skills [--name <skill>]` 返回当前安装内的真实入口和版本；技术栈探测只给提示，不代替用户意图选择。
- 六组 `arc:*` / `arc-*` 名称在任务、历史回执及图门禁中统一识别；未知外部技能名保持原样。
- `task switch` 在同一仓库锁内归档旧任务、回执和原因，再创建无回执的新阶段；同技能 mode 复用当前任务。
- `task begin` 默认不替换未完成任务；显式遗留 `--force` 仍会归档旧证据，不再静默丢失。
- 绘图以现有 v2 `snapshot.json` 为输入；仅中文名称、真实包名的来源证据与局部范围由展示映射补充。
  不打开 DB、不批准规则、不执行业务目标。官方 renderer 的 pin、清单摘要、各文件摘要和完整 MIT 许可随包保留。
- Diagram build、layout check、人工看图与业务 verdict 分别记录。旧 FAIL、RISK、候选、RAW 错误、AND/OR/否定条件及省略 ID 保留。
- 只读／只计划／只建图不因 Stop 提示扩大成项目测试。当前未增加 artifact-only PASS 回执；任务可保持未验证，
  只报告已产出图和 runtime NOT_RUN。`--incomplete` 不解除 Stop 或 Git 门禁，不能冒充业务验证。

本轮全仓回归为 **659 passed、1 failed、4 skipped**；唯一失败仍是 checkout 缺少被忽略管理的
`Library/cloudflare/cloudflare-platform` 外部源。完整 skill validator 的失败也仅来自既有 Design / Cloudflare /
ThreeJS 外部源缺失；没有删除测试、放宽断言或生成假源包。
新增阶段切换用例 49 项，后续绘图转换与快照并发读取用例 23 项均通过。
最后的 Stop/Codex/hosts/regressions 定向回归为 **224 passed**；
只读范围提示没有改变一次提醒规则、任务状态或提交门禁。
DBqual 独立回归为 **160 passed / 1 skipped / 139 subtests passed**，跳过项仍是未配置的真实 SQLFluff 命令。

[两份官方 renderer 生成的业务图](examples/arc-business-diagram/README.md) 已随仓库提交。
实际 HTML build 成功；浏览器缺失使布局检查 BLOCKED、截图／目视检查 NOT_RUN。
Claude Code、Codex CLI、Grok Build 本机原生会话以及 Go race/vet 仍未在当前环境运行，
不把 Python fixture、静态 schema 或 HTML build 记作宿主／业务 E2E PASS。

## 本仓宿主链接治理

`scripts/link_hosts.py` 管理 `schemas/host_profiles.yaml` 的 desired 条目，以及本仓 `LIFECYCLE_PHASES` / `RETIRED_ARC_SKILLS` 明确列出的 Arc 名称（冒号和连字符形式）。它不因名称有 `arc-` 前缀就取得对用户技能的归属。

| 命令 | 行为 |
|---|---|
| `--check` | declared 链接和已知 Arc 条目漂移返回 1；无关用户条目列为 preserved，不导致失败 |
| `--apply` | 修复 declared 链接，已知 Arc 的过期条目移入备份；保留无关用户目录、文件和悬空链接 |
| `--check --prune-unmanaged` | 预览完整清理范围，无关条目也算漂移；不写入 |
| `--apply --prune-unmanaged` | 显式恢复完整清理行为，把无关条目移入备份；不删除 |
| 未知 `--host` | 在任何修改前报错，退出 2；混用已知与未知宿主也不部分执行 |

隐藏条目和宿主 `ignore` 清单始终保留，包括显式 prune。缺少源包、错误目标等真实 managed 漂移仍然使检查失败。备份位于 `~/.skills-backup/`；这项文件系统检查不代替任何宿主自己的加载清单。
