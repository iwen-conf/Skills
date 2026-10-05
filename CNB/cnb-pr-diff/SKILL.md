---
name: cnb-pr-diff
description: 获取 PR 的 diff 变更信息。当需要查看 PR 代码变更、获取文件差异时使用此技能。
---

# 获取 PR Diff

获取 PR 的代码变更差异信息，用于代码评审、变更总结等场景。

## 1. 判断场景并获取 SHA

### 场景 A：PR 评论触发（`CNB_PULL_REQUEST_IID` 存在）

直接从环境变量获取：
- head SHA：`CNB_PULL_REQUEST_SHA`
- target SHA：`CNB_PULL_REQUEST_TARGET_SHA`

若环境变量不存在，通过 API 查询当前 PR 详情。

### 场景 B：Issue 或其他场景引用 PR

用户输入中提及了 PR 编号（如 `!45`），需要：
1. 从用户输入中解析 PR 编号
2. 通过 API 查询该 PR 详情获取 SHA
3. fetch 对应的 commit（本地可能没有）

### API 查询 PR 详情

按[运行合同](../../docs/cnb-runtime.md)先验证 HTTP 与 JSON；示例必须在每步成功后继续，不能忽略 stderr：

```bash
pr_info=$(curl --fail-with-body --silent --show-error \
  "${CNB_API_ENDPOINT:-https://api.cnb.cool}/${CNB_REPO_SLUG}/-/pulls/${pr_iid}" \
  -H "Accept: application/vnd.cnb.api+json" \
  -H "Authorization: Bearer $CNB_TOKEN")
head_sha=$(printf '%s' "$pr_info" | jq -er '.head.sha | select(type == "string" and test("^[0-9a-fA-F]{40,64}$"))')
target_sha=$(printf '%s' "$pr_info" | jq -er '.base.sha | select(type == "string" and test("^[0-9a-fA-F]{40,64}$"))')
git fetch origin "$head_sha" "$target_sha"
git cat-file -e "$head_sha^{commit}"
git cat-file -e "$target_sha^{commit}"
git merge-base "$target_sha" "$head_sha"
```

A/B 两种场景都检查对象与共同祖先；环境变量提供的 SHA 同样验证。浅 clone 缺历史时针对当前 refs 有界 deepen；仍无 merge-base、远端拒绝 fetch 或权限错误则报告 BLOCKED，不改用双点 diff、不输出空差异。

## 2. 获取 Diff

### 获取变更文件列表

```bash
git diff --name-only "${target_sha}...${head_sha}"
```

### 获取变更统计

```bash
git diff --stat "${target_sha}...${head_sha}"
```

### 获取完整 Diff

```bash
git diff -U5 "${target_sha}...${head_sha}"
```

### 获取特定文件的 Diff

```bash
git diff -U5 "${target_sha}...${head_sha}" -- path/to/file.ts
```

## 3. 按目的分类（共享规则）

先保留完整文件列表，再决定逐行读取范围：

- 源码、配置、测试和删除：按行为影响评审/总结。
- 锁文件：不必逐行复述，但检查实际依赖版本、来源、校验值和 manifest 一致性；不能完全跳过。
- 图片、字体、二进制、压缩包和生成产物：可跳过逐行 diff，保留文件角色、影响与未审原因。SVG 或生成代码涉及执行/安全时不能仅按扩展名排除。
- 所有过滤与截断记入 coverage；summary 记录未详读项，review 未完成时输出 partial。

## 4. 使用建议

1. **diff 过大时**：先用 `--name-only` 查看文件列表，再按需获取具体文件
2. **代码评审**：按上方共享规则分类，保留依赖和删除语义
3. **变更总结**：可保留更多文件，但排除二进制和依赖目录
4. **上下文行数**：`-U5` 提供 5 行上下文，可按需调整
