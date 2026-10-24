# CNB 链接转换

读取[运行合同](../../../docs/cnb-runtime.md)。`SKILL_ROOT` 为本入口目录。
必须提供 `CNB_WEB_ENDPOINT`（HTTP(S)站点）、`CNB_REPO_SLUG`、`CNB_BRANCH`（当前内容对应分支或 SHA），缺少任何一项即停止。通过安全 argv 传入**一个完整字符串**：

```bash
node "$SKILL_ROOT/scripts/convertLink.js" "$content"
```

只输出转换结果，不写文件、不访问网络、不发布评论。输入不是 Markdown/HTML 解析器的完整语法：只处理脚本支持的内联链接、图片及引号包裹的 src/href；代码块、行内代码、绝对 URL 和裸文本路径保持原样。复杂引用式 Markdown 不宣称已转换。

以 `https://cnb.cool`、`example/repo`、`main` 为环境：

- `[实现](./src/helper.ts)` → `[实现](https://cnb.cool/example/repo/-/git/raw/main/src/helper.ts)`。
- 裸文本 `./src/helper.ts` 保持原样。
- `./`、`../` 与 `/` 开头的链接按仓库根路径约定处理：脚本移除 `.`/`..` 段，不进行源文档目录解析。需要真正相对文档位置的解析时停止并说明此能力不支持。
