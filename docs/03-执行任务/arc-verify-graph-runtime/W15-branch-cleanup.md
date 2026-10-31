# W15 分支与 worktree 清理（只保留 main）

状态：`[?]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 交付（执行技能 check） |
| 优先级 | P3，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D1 |
| 难度依据 | 只涉及 git 引用；风险在于误删他人或未合并的提交，所以逐个核对独有提交和作者，删除前请用户确认。 |
| 目标面 | 本仓本地分支、`/private/tmp` 下的 worktree、远端 `origin` |
| 前置依赖 | 无；建议在 W06 后执行，避免删掉仍在用的 worktree |
| 下游影响 | 无 |

## 目标与范围

需求：对话 L5079 用户原话"提交并推送，合并成一个分支。并且只保留一个分支"。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

2026-10-31 现状：本地分支 `verify-hardening-2921`（worktree `/private/tmp/Skills-verify-hardening.9We0N5`，已标 prunable）；分离 HEAD 的 worktree `/private/tmp/verify-hardening.z0345T`；远端 `feat/arc-verify-local-graph`（PR #1 已 squash 合并为 `ce5e973`）。

- 每个分支：列出相对 `main` 的独有提交与作者；只处理作者是本机 git 用户 `iwen-conf` 的分支；他人分支只列出。
- 有独有提交的先报告，不删。
- 删除清单交给用户确认后才执行。

不做：不动 `main`；不强推；不删有未合并独有提交或未提交改动的 worktree；不删别人的分支。

## 2026-10-31 核查结果（只读，未删除任何东西）

原始输出：`.arc/artifacts/verify-branch-cleanup/branches.txt`。

| 对象 | 事实 | 建议 |
| --- | --- | --- |
| 本地分支 `verify-hardening-2921`（作者 `iwen-conf`） | 相对 `main` 0 个独有提交（`git rev-list --count main..verify-hardening-2921` = 0，`git cherry` 为空） | 可删：`git -C <repo> branch -d verify-hardening-2921` |
| worktree `/private/tmp/Skills-verify-hardening.9We0N5` | 已标 prunable，目录已不存在 | 可清：`git -C <repo> worktree prune` |
| 分离 HEAD worktree `/private/tmp/verify-hardening.z0345T` | HEAD = `f230ae9`，但有 33 处未提交改动（暂存与修改），像是其他会话的在途工作 | **保留**，不删 |
| 远端 `origin/feat/arc-verify-local-graph` | 作者 `iwen`（不是本机用户 `iwen-conf`），有 42 个不在 `origin/main` 祖先链上的提交；PR #1 已 squash 合并为 `ce5e973` | 只列出；删远端分支属于对外操作，由用户决定 |

等待用户确认删除清单（前两项）后执行；`main` 不动、不强推。

## 上下文检索

<Context-Queries>

```bash
git -C /Users/iluwen/Documents/Code/Skills branch -a -vv
git -C /Users/iluwen/Documents/Code/Skills worktree list --porcelain
git -C /Users/iluwen/Documents/Code/Skills log --oneline main..verify-hardening-2921
git -C /Users/iluwen/Documents/Code/Skills branch -r --merged origin/main
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [ ] 跑上面的命令，整理"可删/需确认/保留"三栏清单（含独有提交与 worktree 是否 dirty）。
- [ ] 请用户确认清单；确认后 `git worktree prune`、删本地分支、删远端已合并分支。
- [ ] 回写结果并同步 tracker。

## 验收

```bash
git -C /Users/iluwen/Documents/Code/Skills branch -a   # 期望：只有 main 与 origin/main（及 origin/HEAD）
git -C /Users/iluwen/Documents/Code/Skills worktree list   # 期望：只有主工作树
```

删除属于不可逆操作，必须有本会话内用户的明确确认。
