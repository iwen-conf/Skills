# CNB 官方文档

自动查询 CNB 云原生构建平台官方文档，为用户提供准确的使用指引和操作说明。

## 文档来源

> 下方 URL 中的 `${CNB_WEB_PROTOCOL}` 和 `${CNB_WEB_HOST}` 为环境变量，使用前先 `echo` 获取实际值再拼接。

CNB 官方文档站点：`${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}`

提供两种文档索引：

| 索引 | URL | 说明 |
|------|-----|------|
| 增量索引 | `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms.txt` | 文档目录索引，包含所有文档的标题和路径链接 |
| 全量文档 | `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms-full.txt` | 包含所有文档的完整内容（体积较大） |

官方索引是目录权威；不在入口维护静态领域目录。使用当前运行时可用的网页获取工具，WebFetch 仅为示意。

## 使用流程

### 第一步：分析用户问题

仔细分析用户提出的问题，提取关键词，判断用户想了解哪个领域的文档。

### 第二步：获取文档索引

使用 WebFetch 获取文档目录索引：

```
WebFetch url: ${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms.txt
prompt: 仅返回匹配的标题和路径链接，找到与「{用户问题的关键词}」相关的文档路径
```

### 第三步：获取具体文档内容

根据索引中匹配到的文档路径，拼接完整 URL 并获取文档内容：

```
WebFetch url: ${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}{文档路径}
prompt: 提取相关小节，保留其前提、代码示例与配置约束；不足时扩读相邻小节
```

**注意**：索引中的路径格式为 `/zh/category/topic.md`，拼接后完整 URL 为 `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/category/topic.md`。

### 第四步：整理并回答

将获取到的文档内容整理后，根据用户问题：
1. 提取与用户问题最相关的内容
2. 保留重要的代码示例和配置片段
3. 简明呈现给用户
4. 内容较长时适当精简，并附原文档链接

## 注意事项

- 增量索引无匹配先换关键词；仍无结果时有限查询全量文档，读取失败明确未知，不推测 API。全量地址： `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/llms-full.txt`
- 单个文档无法完全回答时，可获取多个文档综合回答
- 始终基于官方文档回答，不臆造信息
- 附带文档来源链接
