---
name: lark-skill-maker
version: 1.0.0
description: "创建 lark-cli 的自定义 Skill。当用户需要把飞书 API 操作封装成可复用的 Skill（包装原子 API 或编排多步流程）时使用。"
metadata:
  requires:
    bins: ["lark-cli"]
---

# Skill Maker

基于 lark-cli 创建新 Skill。Skill = 一份 `SKILL.md`，教 AI 用 CLI 命令完成任务。

## CLI 核心能力

```bash
lark-cli <service> <resource> <method>          # 已注册 API
lark-cli <service> +<verb>                      # Shortcut（高级封装）
lark-cli api <METHOD> <path> [--data/--params]  # 任意飞书 OpenAPI
lark-cli schema <service.resource.method>       # 查参数定义
```

优先级：Shortcut > 已注册 API > `api` 裸调。

## 调研 API

```bash
# 1. 查看已有的 API 资源和 Shortcut
lark-cli <service> --help

# 2. 查参数定义
lark-cli schema <service.resource.method>

# 3. 未注册的 API，用 api 直接调用
lark-cli api GET /open-apis/vc/v1/rooms --params '{"page_size":"50"}'
lark-cli api POST /open-apis/vc/v1/rooms/search --data '{"query":"5F"}'
```

如果以上命令无法覆盖需求（CLI 没有对应的已注册 API 或 Shortcut），使用 [lark-openapi-explorer](../lark-openapi-explorer/SKILL.md) 从飞书官方文档库逐层挖掘原生 OpenAPI 接口，获取完整的方法、路径、参数和权限信息，再通过 `lark-cli api` 裸调完成任务。

通过以上流程确定需要哪些 API、参数和 scope。

## SKILL.md 模板

先从用户指定目录或项目结构确定 `SKILLS_SOURCE_ROOT`，文件放在 `<SKILLS_SOURCE_ROOT>/lark-<name>/SKILL.md`。未知时先查项目技能布局；本地创作不自动同步全局安装目录。交付前校验 frontmatter、相对引用与目标 CLI 命令，明确未执行的远程验证：

```markdown
---
name: lark-<name>
version: 1.0.0
description: "<功能描述>。当用户需要<触发场景>时使用。"
metadata:
  requires:
    bins: ["lark-cli"]
---


# <标题>

> **前置条件：** 先阅读 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md)。

## 输入与执行边界

记录目标资源、身份、用户授权、必填输入与真实 ID 来源。明确 SKILL_ROOT（本技能目录）和 PROJECT_ROOT（用户项目目录）；脚本用绝对 SKILL_ROOT 路径，CLI 文件参数相对其 cwd。描述缺依赖时的 BLOCKED 出口。

## 命令

\```bash
# 单步操作
lark-cli api POST /open-apis/xxx --data '{...}'

# 多步编排：说明步骤间数据传递
# Step 1: ...（记录返回的 xxx_id）
# Step 2: 使用 Step 1 的 xxx_id
\```

## 输出、失败与停止

输出创建/修改的真实 ID、验证证据和完成/部分完成/失败/NOT_RUN 状态。批量逐项检查，保留成功项与恢复标识；写请求结果未知先查状态，禁止盲目重发。可恢复错误有限重试（默认最多 3 次），权限/环境不可用时停止依赖步骤。

## 验证

至少覆盖成功、输入缺失、权限不足、部分失败与重复执行场景；本地 schema/脚本检查和实际 API 行为分开记录。未执行远程动作不能宣称端到端 PASS。

## 权限

| 操作 | 所需 scope |
|------|-----------|
| xxx | `scope:name` |
```

## 关键原则

- **description 决定触发** — 包含功能关键词 + "当用户需要...时使用"
- **认证** — 说明所需 scope，只在缺失授权时按 lark-shared 登录，不重复授权
- **安全** — 写入操作前确认用户意图，建议 `--dry-run` 预览
- **编排** — 说明数据传递、失败回滚、可并行步骤
