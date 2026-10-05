---
name: cnb-code-commit
description: 编写代码、提交推送并创建 PR，适用于功能开发、Bug 修复、重构优化等场景。
---

# 自动编码并提交推送

根据用户需求选择：
1. 新建分支 -> 编写代码 -> 推送代码 -> 并创建 PR
2. 切换到 PR 源分支 -> 编写代码 -> 推送代码更新 PR

## 诊断与修复门禁

任务属于 Bug、故障、回归或基于问题的优化时，必须先加载
[`docs/execution-truth.md`](../../docs/execution-truth.md) 的 evidence-first root-cause repair
门禁：先确认问题与根因，再确定同类路径和完整修复边界。未确认时不得修改生产行为或
宣称已修复；只允许为取证增加有界、行为保持的日志、探针或复现覆盖。

## 步骤

### 1. 理解需求

分析用户输入，明确要修改的文件、功能/Bug 描述、目标分支（PR 的 base）。
如需了解项目结构，直接在当前工作区读取代码。如需了解 PR 变更，可使用 cnb-pr-diff Skill。

### 2. 判断场景并准备分支

先记录 `git status --short`、已暂存 diff 和当前分支。保留已有修改；切换会冲突时用独立 worktree，不自动 stash、reset 或覆盖用户工作。应用[运行合同](../../docs/cnb-runtime.md)。

解析目标分支为 `base_ref`：用户明确指定优先，其次当前 PR 的真实 base，最后才是 `CNB_DEFAULT_BRANCH`。为空、分支无效或 PR 查询失败时停止分支/PR 写入。

- 已有 PR：通过结构化查询核对仓库、head 和 base，在真实源分支上继续；不以远端 push 提示字串判断 PR 是否存在。
- 新 PR：先确认查询完整且确实不存在，再从已验证的目标分支创建任务分支。以下命令逐步检查成功后才继续：

```bash
git check-ref-format --branch "$base_ref"
git fetch origin "refs/heads/$base_ref:refs/remotes/origin/$base_ref"
git rev-parse --verify "refs/remotes/origin/$base_ref^{commit}"
git switch -c "$work_branch" "refs/remotes/origin/$base_ref"
```

`work_branch` 使用精简任务关键词及短随机后缀，如 `auto/login-error-b2c1`；已有 PR 不另建分支。

### 3. 编写代码

直接在工作区中读取和修改文件，实现代码变更。遵循项目已有的代码风格，确保 import/依赖完整。

**生成/修改 `.cnb.yml` 时的特殊处理**：
- 优先加载 `cnb-pipeline` skill 的 SKILL.md，参照其中的语法文档和校验方式生成并验证配置
- 生成完成后需通过 cnb-pipeline skill 提供的校验方式验证，校验通过后才能提交；校验失败则修复后重新校验

### 4. 验证与提交

```bash
# 查看变更概览
git diff --stat
# 如有构建/lint 命令则执行验证
# 暂存并提交
git add -- "$task_file"
# 多文件逐个指定；与用户修改同文件时只暂存本任务 hunks
git diff --cached
git commit -m "{类型}: {简短描述}"
```

已有暂存内容不属于本任务时，必须先可靠隔离本任务提交（独立 worktree 或临时 index），保留原暂存状态；不能直接执行普通 `git commit` 把它们一并提交。同文件改动无法可靠分离时报告阻塞，不自动取消用户暂存。

类型：`feat`(新功能) / `fix`(修复) / `refactor`(重构) / `docs`(文档) / `chore`(其他)

### 5. 推送并查询 PR 状态

核对暂存差异仅包含本任务，项目门禁通过后 `git push origin HEAD`。
使用当前 CNB CLI help 确认的结构化 PR 查询，按真实仓库/head/base 查找：

| 结果 | 动作 |
| --- | --- |
| 已存在匹配 PR | 返回实际编号/URL，不重复创建 |
| 完整查询确认无 PR | 执行第 6 步 |
| 网络/权限失败、分页未完成、输出未知 | 报告推送结果与 PR 状态未知，停止创建 |

不把 push 输出中的自然语言当作 API 合同。

### 6. 创建 PR（仅推送后确认无 PR 时执行）

```bash
cnb pulls post-pull --repo "$CNB_REPO_SLUG" \
  --base "$base_ref" \
  --head "$(git branch --show-current)" \
  --title "{类型}: {简短描述}" \
  --body '{变更目的与主要改动，支持 Markdown 真换行}

Ref #'"$CNB_ISSUE_IID"
```

正文通过[运行合同](../../docs/cnb-runtime.md)的安全 argv/body file 方式传递，保留真实换行；非 Issue 触发不添加 Ref 行。创建成功须取得真实 PR 标识。

## 注意事项

- 仓内发现先走 arc-idx；明确范围与验收后才修改。
- `.cnb.yml` 的生成与验证归 cnb-pipeline。
- 推送不等于 CI 通过；只有确认自动失败回调可用才依赖回调，否则报告 CI 待验证。

## 禁止的操作

- ❌ PR 不存在时，禁止在默认分支上修改代码，需新建分支并创建 PR
- ❌ 未经明确覆盖历史授权使用 `--force` 或 `--force-with-lease`；有授权时仍核对远端状态并优先 lease 保护。
- ❌ 无限轮询 CI 或把未执行的 CI 验证写为通过。
