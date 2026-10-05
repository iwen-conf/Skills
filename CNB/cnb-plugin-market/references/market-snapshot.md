## 常见场景速查

以下为高频需求与推荐插件的快速参考（**基于当前市场快照，完整列表以 `plugins.json` 为准**）：

| 需求 | 推荐插件 | 镜像 |
|------|---------|------|
| **通知** | | |
| 发企业微信通知 | wecom-message | `tencentcom/wecom-message` |
| 发飞书通知 | feishu-message | `tencentcom/feishu-message` |
| 发钉钉通知 | dingtalk-bot-msg | `tencentcom/dingtalk-bot-msg` |
| Webhook 通知 | webhook | `cnbcool/webhook` |
| **代码质量** | | |
| AI 代码评审 | code-review | `cnbcool/code-review` |
| Commit 规范检查 | commitlint | `cnbcool/commitlint` |
| PR 标题规范检查 | git-pr-title-lint | `cnbcool/git-pr-title-lint` |
| PR 变更量检查 | git-pr-limit | `cnbcool/git-pr-limit` |
| **构建与发布** | | |
| 发布 npm 包 | npm | `tencentcom/npm` |
| Maven 构建 | maven | `tencentcom/maven` |
| Gradle 构建 | gradle | `tencentcom/gradle` |
| 生成 changelog | changelog | `cnbcool/changelog` |
| Semantic Release | semantic-release | `tencentcom/semantic-release` |
| 多架构镜像合并 | manifest | `cnbcool/manifest` |
| 微信小程序 CI | miniprogram-ci | `tencentcom/miniprogram-ci` |
| **部署** | | |
| 部署到 TKE | deploy-to-tke | `tencentcom/deploy-to-tke` |
| 部署到 SCF | scf | `tencentcom/tencentyun-scf` |
| 部署 EdgeOne Pages | deploy-eopages | `tencentcom/deploy-eopages` |
| Kaniko 构建镜像 | kaniko | `banzaicloud/drone-kaniko` |
| kubectl 操作 | kubectl | `alpine/kubectl` |
| Terraform 部署 | terraform | `jmccann/drone-terraform` |
| **Git & 协作** | | |
| 自动打 tag | git-auto-tag | `cnbcool/git-auto-tag` |
| 创建 PR | create-pr | `cnbcool/create-pr` |
| SSH 远程执行 | ssh | `cnbcool/ssh` |
| **文件与工具** | | |
| 上传下载 COS | coscli | `tencentcom/coscli` |
| 环境变量替换 | envsubst | `tencentcom/envsubst` |

> **注**：open-source 类插件多为命令行工具镜像，使用时**可能需配合 `args:`** 传递命令参数（参考插件版本文档）。完整列表请走当前市场目录。


此为历史导航快照；参数与镜像版本以当前官方插件文档为准。
