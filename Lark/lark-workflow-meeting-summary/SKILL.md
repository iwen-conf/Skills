---
name: lark-workflow-meeting-summary
version: 1.0.0
description: "会议纪要整理工作流：汇总指定时间范围内的会议纪要并生成结构化报告。当用户需要整理会议纪要、生成会议周报、回顾一段时间内的会议内容时使用。"
metadata:
  requires:
    bins: ["lark-cli"]
    skills: ["lark-meeting"]
---

# 会议纪要汇总工作流

**CRITICAL — 开始前 MUST 先完整读取 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md) 和 [`../lark-meeting/SKILL.md`](../lark-meeting/SKILL.md)**。认证、身份和权限以 lark-shared 为准；会议与产物关系、产物选择和逐字稿路由以 lark-meeting 为准。

## 适用场景

- "帮我整理这周的会议纪要" / "总结最近的会议" / "生成会议周报"
- "看看今天开了哪些会" / "回顾过去一周开了哪些会"

## 前置条件

仅支持 **user 身份**，所有调用显式 `--as user`，关联产物沿用 source_identity=user。复用有效授权，仅在缺少 scope 时按需授权：

```bash
lark-cli auth login --domain vc        # 基础（查询+纪要）
lark-cli auth login --domain vc,drive   # 含读取纪要文档正文、生成文档
lark-cli auth login --domain vc,drive,minutes  # 含无 note_id 时的妙记备选路径
```

## 工作流

```
{时间范围} ─► vc +search ──► 会议列表 (meeting_ids)
                   │
                   ▼
               vc +detail ──► 获取 note_id 
                   │
                   ▼
               note +detail ──► 纪要文档 tokens
                   │
                   ▼
               drive metas batch_query 纪要元数据
                   │
                   ▼
               读取纪要正文 / 逐字稿 ──► 内容报告（仅元数据时只能输出索引）
```

### Step 1: 确定时间范围

默认**过去 7 天**，先确定用户时区并写入报告。推断规则："今天"→当天，"这周"→本周一~now，"上周"→上周一~上周日，"这个月"→1日~now。

> **注意**：日期转换必须调用系统命令（如 `date`），不要心算。时间范围参数需根据 CLI 实际要求格式化（通常为 `YYYY-MM-DD` 或 ISO 8601）。

### Step 2: 查询会议记录

```bash
# page-size 最大为 30
lark-cli vc +search --start "<YYYY-MM-DD>" --end "<YYYY-MM-DD>" --format json --page-size 30 --as user
```

- 时间范围拆分：搜索的时间范围最大为 1 个月。搜索更长时间范围的会议，需要拆分为多次时间范围为一个月查询。
- `--end` 为**包含当天**的日期（即查"今天"时 start 和 end 都填今天）
- `--format json` 输出 JSON 格式，你更佳擅长解析 JSON 数据。
- `--page-size 30` 每页最多 30 条。
- 有 `page_token` 时必须继续翻页，收集所有 `id` 字段（meeting-id）

### Step 3: 获取会议内容与来源

按 [lark-meeting](../lark-meeting/SKILL.md) 的 [查询会议与产物](../lark-meeting/scenes/query-meeting-and-artifacts.md) 逐会议取得正文和需要的逐字稿。所有命令显式 `--as user`，下游沿用 source_identity=user。保留会议 ID、标题、时间、来源 URL、已读取产物与覆盖范围；按该场景处理批量限制和分页。

产物不变量由 meeting 维护：没有 note 不等于没有内容；normal/unified 的正文与逐字稿路由不同，不能猜 token 或链接。缺权限按原身份处理并标记缺口；申请访问是单独外部动作，需有效授权。`drive metas batch_query` 仅用于链接元数据，不作为内容总结依据。

### Step 4: 读取内容并整理报告

先按 [会议产物路由](../lark-meeting/SKILL.md) 读取纪要正文；需要发言依据时继续读取对应逐字稿。`drive metas batch_query` 仅返回元数据和 URL，不能支持会议内容总结。按会议记录读取状态、来源链接、时间与覆盖范围；提炼讨论要点、已达成决策、行动项（负责人/期限只取原文明确内容）和未决问题，跨会议矛盾保留来源。

只有标题、时间、链接时，输出“会议索引”，明确未读取正文；无权限、空产物、截断或分页未完成时报告缺口，不宣称内容汇总完成。产物选择细节由 lark-meeting 的场景文档维护。

根据时间跨度选择输出格式：

- **单日汇总**（"今天"/"昨天"）：用"今日会议概览"标题，逐会议列出会议时间、主题、纪要链接、逐字稿链接（`unified` 纪要无逐字稿链接，标注"unified 纪要，逐字稿需 `note +transcript` 拉取"）。
- **多日/周报**（"这周"/"过去 7 天"等）：用"会议纪要周报"标题，含概览统计、逐会议详情。

### Step 5: 生成文档（可选，用户要求时）

阅读 [`../lark-doc/SKILL.md`](../lark-doc/SKILL.md) 学习云文档技能。

```bash
lark-cli docs +create --doc-format markdown --content $'<title>会议纪要汇总 (<start> - <end>)</title>\n<内容>' --as user
# 或追加到已有文档
lark-cli docs +update --doc "<url_or_token>" --command append --doc-format markdown --content $'<内容>' --as user
```

## 参考

- [lark-shared](../lark-shared/SKILL.md) — 认证、权限（必读）
- [lark-meeting](../lark-meeting/SKILL.md) — 会议与产物统一路由
- [查询会议与会议产物](../lark-meeting/scenes/query-meeting-and-artifacts.md) — 搜索、消歧、产物获取与逐字稿分析流程
- [查询妙记及关联产物](../lark-meeting/scenes/query-minutes-and-artifacts.md) — 无 `note_id` 时的妙记备选路径
- [`vc +search`](../lark-meeting/references/lark-vc-search.md)、[`vc +detail`](../lark-meeting/references/lark-vc-detail.md)、[`note +detail`](../lark-meeting/references/lark-note-detail.md)、[`note +transcript`](../lark-meeting/references/lark-note-transcript.md)、[`minutes +detail`](../lark-meeting/references/lark-minutes-detail.md)、[`minutes +apply-permission`](../lark-meeting/references/lark-minutes-apply-permission.md) — 命令细节
- [lark-doc](../lark-doc/SKILL.md) — `+fetch`、`+create`、`+update` 详细用法
