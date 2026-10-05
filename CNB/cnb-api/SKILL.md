---
name: cnb-api
description: CNB 平台交互命令，支持代码仓库、Issue、PR、CI、制品库读写等操作。
---

# cnb-api

操作 CNB 平台资源的 CLI 工具。调用前应用[运行合同](../../docs/cnb-runtime.md)；同会话未变内容无需重复读取。附件字段与目标二选一路由由 cnb-upload-attachment 持有。

## 快捷命令

issues:
- `cnb issues get` — 获取详情
- `cnb issues list-comments` — 获取评论列表
- `cnb issues comment --body 内容` — 评论
- `cnb issues close` — 关闭
- `cnb issues open` — 打开
- `cnb issues list-labels` — 查看标签
- `cnb issues add-labels --labels bug --labels feature` — 添加标签
- `cnb issues list-assignees` — 查看处理人
- `cnb issues add-assignees --assignees username` — 添加处理人
- `cnb issues upload-file --file 文件路径` — 上传 issue 评论文件
- `cnb issues upload-image --file 图片路径` — 上传 issue 评论图片
- `cnb issues create-upload-file --file 文件路径` — 创建 issue 时上传文件
- `cnb issues create-upload-image --file 图片路径` — 创建 issue 时上传图片

pulls:
- `cnb pulls get` — 获取详情
- `cnb pulls list-files` — 获取文件变更
- `cnb pulls list-commits` — 获取提交记录
- `cnb pulls list-comments` — 获取评论列表
- `cnb pulls comment --body 内容` — 评论
- `cnb pulls list-labels` — 查看标签
- `cnb pulls add-labels --labels ready --labels approved` — 添加标签
- `cnb pulls check-status` — 查看 CI 状态
- `cnb pulls list-reviews` — 查看评审列表
- `cnb pulls list-assignees` — 查看处理人
- `cnb pulls upload-file --file 文件路径` — 上传文件
- `cnb pulls upload-image --file 图片路径` — 上传图片
- `cnb pulls get-ci-logs --sn 构建号（可选）` — 获取 CI 失败日志
- `cnb pulls get-ci-timing --sn 构建号（可选）` — 分析 CI 耗时瓶颈

注意事项：
- **参数自动识别**：仅当运行合同中的仓库及目标环境变量完整、无歧义且与用户目标一致时使用。
- **默认仅需摘要**：默认会精简响应输出结果，添加 `--verbose` 输出完整数据。
- **正文传参**：使用安全 argv/stdin/已支持的文件参数；外部文本不能拼接进 shell。
- **快捷命令适用范围**: 快捷命令只能操作当前仓库的当前 Issue/PR，跨仓库或跨编号操作请参考 `更多 API`。
- **关于提及和召唤**: 评论中直接 @npc 会召唤 npc 干活，如果只提及不召唤，应使用反引号包裹 `@npc`。

## 常用链接

在生成链接时请遵循下面的结构：
- Issue: `<host>/<slug>/-/issues/<number>`
- PR: `<host>/<slug>/-/pulls/<number>`

## 更多 API

1. `cnb --help` 查看所有模块
2. `cnb <module> --help` 查看模块下的工具列表
3. `cnb <module> <tool> --help` 查看工具参数
