# Lazycat Migration Candidate and License Gate

## 职责范围

只负责迁移候选进入门禁：发现候选、查重、只读获取源码、审查许可证和商业使用条件。它不判断运行可行性、不估工作量、不修改包装层、不打包、不提审。

候选通过后交给 boundary 模式。

## 输入

- GitHub 仓库 URL，或候选搜索需求，或开发者中心未上架应用盘点需求。
- 应用商店访问方式：公开页面、已有登录态或 `LAZYCAT_APPSTORE_USERNAME` / `LAZYCAT_APPSTORE_PASSWORD`。
- 开发者中心访问方式：已有登录态或 `LAZYCAT_DEVELOPMENT_USERNAME` / `LAZYCAT_DEVELOPMENT_PASSWORD`。
- 本地项目父目录和 clone 目标目录，若任务需要对比或拉取候选。

## 凭据

按 [SKILL.md](../SKILL.md) 的凭据约定读取，只用环境变量，不输出明文。

## 输出

- 候选来源与仓库地址。
- 应用商店已上架查重结论。
- 开发者中心待审核/未上架查重结论。
- 本地目录匹配或只读 clone 结果。
- 许可证证据、资产许可证据、依赖许可风险。
- 是否允许商业使用、修改、再分发和上架。
- 上架所需原作者名称和源项目或代码地址。

## 前置条件

1. 已确认任务对象是第三方、开源、自托管或待迁移项目；原创项目改用 original 模式。
2. 需要浏览器查看应用商店或开发者中心时，必须使用 `agent-browser`；不可用时输出阻塞。
3. 涉及开发者中心时只读查看；不得创建、编辑、提交、撤回、删除或改变审核状态。
4. 只 clone 公开仓库或用户明确给出的仓库 URL。

## 允许执行

- 只读搜索 GitHub、应用商店和开发者中心。
- 只读 clone 公开 GitHub 仓库到用户指定工作目录。
- 读取许可证、README、package metadata、release 资产许可、字体/图标/模型/数据集许可。
- 对比本地项目目录名、`package.yml`、`lzc-manifest.yml`、README 和 Git remote。
- 生成候选短名单和许可证门禁结论。

## 禁止执行

- 不创建 issue、PR、fork、评论、review、discussion、star、watch、follow。
- 不修改第三方业务代码、包装层、manifest、构建脚本或本地项目内容。
- 不安装、打包、提审、发布或更新已安装应用。
- 不使用第三方文章替代上游仓库、官方许可证文本或项目自带许可文件。
- 不把缺少许可证、许可证不明或资产许可缺失默认视为可商用。
- 不把真实微服域名、账号、密码、Token、Cookie 写入文件或报告。

## 候选来源

支持三类来源，结果统一进入同一个许可证门禁：

- `Provided Repo`: 用户给出 GitHub 仓库。
- `Candidate Discovery`: 用户要求搜索可迁移项目。
- `Developer Center Inventory`: 用户要求从开发者中心盘点未上架且非待审核应用，并与本地项目目录对比。

开发者中心盘点规则：

1. 状态标准化为 `Published`、`Pending`、`Unlisted`、`Unknown`。
2. 只保留 `Unlisted`：未上架、下架、草稿、驳回、待补充、未提交或等价状态。
3. 排除 `Pending`：待审核、审核中、已提交、排队审核或等价状态。
4. `Unknown` 不进入候选，输出 `Needs Manual Review`。
5. 本地匹配优先级：`package.yml.package`、Git remote/homepage、README 上游链接、规范化目录名/应用名。

## 候选过滤

必须保留“有页面 + 有后端”的项目，且审核员能在应用内走完一条主流程：

- 允许：前后端分离 Web 应用、带 Web 控制台且控制台可完成主流程的服务端应用、Agent/多 Agent/RAG/自动化工作台。
- 排除：纯前端静态站、纯后端库、CLI-only、SDK、starter template、教程、纯 prompt、纯文档站、无界面的数据库/队列/中间件/协议服务。
- 降级：完全依赖私有 SaaS、缺少启动方式、缺少自托管证据；有控制台但主价值是给其他程序调用的中间件/队列/数据库。后者记录 `Listing risk: middleware`，交给 boundary 模式 判断应用内主流程是否足够上架。

## 查重规则

每个候选必须分别检查：

- 懒猫应用商店：名称、中文名、英文别名、包名、功能关键词、上游仓库 URL、作者/组织。
- 开发者中心：待审核、审核中、驳回待重提、草稿或已占用同类项目。

查重结论：

- `Clear`: 两处都查过，未发现同名或高重合项目。
- `Duplicate Published`: 应用商店已有同名或高重合项目。
- `Duplicate Pending`: 开发者中心待审或已占用项目高度重合。
- `Incomplete`: 缺少访问权限或证据不足，不能下最终无重复结论。

只有 `Clear` 或用户明确接受差异化理由时，才继续许可证判断。

## 许可证决策

- `Pass`: 明确允许商业使用、修改和再分发，且没有明显资产许可阻塞。
- `Pass with Obligations`: 可商用但有署名、许可证保留、源码提供、修改说明、网络服务条款等义务。
- `Unclear`: 无许可证、多许可证冲突、资产许可缺失、上下游声明冲突。
- `Blocked`: 非商用、禁止再分发、禁止修改、专有闭源或许可证不允许上架所需行为。

GPL、AGPL、LGPL、MPL 等 copyleft 许可证不是“不可商用”，但必须记录分发、源码和网络服务义务。

迁移项目通过门禁时，必须记录用于开发者中心上架表单的原作者名称和源项目或代码地址。原作者优先来自仓库 owner、项目官网或 LICENSE/NOTICE/README 中明确声明的作者；源地址优先使用上游 GitHub/GitLab 仓库或官方源码页面。不得使用占位符或猜测值。

## 后置条件

- 所有候选均有来源、查重、许可证和下一步结论。
- `Pass` 或 `Pass with Obligations` 且查重可接受的候选，进入 boundary 模式。
- `Blocked`、`Unclear`、`Duplicate Published`、`Duplicate Pending`、`Incomplete` 不进入迁移可行性评估，除非用户明确接受风险或补齐证据。

## 输出格式

```text
Phase: Migration Candidate and License Gate
Mode: Provided Repo / Candidate Discovery / Developer Center Inventory
Repository: <owner/repo or URL>

Candidate Evidence
- UI:
- Backend:
- In-app main flow:
- Listing risk:
- Deploy:
- Source location:
- Local match:

Duplicate Checks
- Lazycat App Store: Clear / Duplicate Published / Incomplete
- Developer Center: Clear / Duplicate Pending / Incomplete

License Evidence
- Code:
- Assets:
- Dependencies:
- Commercial use:
- Obligations:
- Original author:
- Source project/code:

Decision: Pass / Pass with Obligations / Unclear / Blocked
Next: Proceed to boundary 模式 / Stop / Needs Manual Review
```
