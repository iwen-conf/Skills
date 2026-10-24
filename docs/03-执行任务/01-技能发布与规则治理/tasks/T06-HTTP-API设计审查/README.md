# T06 HTTP API 设计审查与安装

状态：`[x]`。日期：2026-10-06。所属端：技能文档；优先级 P1；难度 D3。

需求：[HTTP API 设计审查](../../../../00-产品需求/04-HTTP-API设计审查.md)。

Mode: implement · Tier: T2（多个技能共享合同）· Scope: Arc API 参考、四入口路由、行为语料与文档 · Task: http-api-design。

## 计划与影响面

1. `[x]` 查现有 API 合同、审计/规划/实施/验证入口，网上检索 RFC、JSON Schema、OpenAPI、OWASP 原文。
2. `[x]` 扩展共享规则与 `arc:audit api-design`；设计/实现/验收引用同一约束，保留现有范围与授权；75 入口结构校验通过。
3. `[x]` 行为语料、独立前向评审与完整包验证通过；6 包本机安装、274 文件内容/权限一致，两个宿主各 6 个链接和实际参考路径回读通过。

无产品代码/API/数据库改动；不新增依赖、扫描器或顶层技能。保留基线 79 个已有修改/未跟踪文件。声明位置：`api-contract.md`；调用方：plan/code/audit/verify；序列化字段为审查内容，无实际产品字段迁移；配置、数据迁移、构建部署不受影响；发布影响为共享参考闭包；验证用现有 validator、pytest、packager 与人工行为评估。

<Context-Queries>

```bash
arc-idx files api --format json
arc-idx files 'Arc/arc:audit' --format json
arc-idx files behavior_corpus --format json
arc-idx search 'f:Arc JSON' --max 30 --context 2 --format ai-md
arc-idx search 'f:Arc api-contract.md' --max 20 --context 2 --format ai-md
```

</Context-Queries>

## 验收命令

```bash
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests/test_behavior_corpus.py tests/test_triggers.py tests/test_sync_skills.py -q
.venv/bin/python scripts/build_skills_index.py
bash Arc/arc:code/scripts/gate.sh check --task http-api-design --max-files 22 --strict
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --dry-run
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --check
```

前向评估使用孤立 API 草案和候选技能，要求输出依据、缺陷/未知区别、反例与修复建议，检查不会凭空制定限额、把建议当 RFC 强制或执行网络探测。安装后核对共享参考及宿主链接。

## 证据与限制

- 归属：保留 6 个 Arc 入口，`arc:audit` 增 `api-design`；plan/code/audit/verify 为 2.3.0，explore/closure 仍为 2.1.0。共享 [HTTP 规则](../../../../../Arc/_shared/http-api-design.md)、JSON/upload 专题及审核/验证参考只按需加载。
- 来源：本回合网上检索并打开 RFC 9110/9111/7578/8259/9457、JSON Schema 2020-12、OpenAPI 3.0.4/3.1.2、OWASP API3/API4 与 File Upload Cheat Sheet。精确 URL、章节、版本与日期在共享参考中；没有把安全建议、draft 或项目限额升级为协议 MUST。
- 前向评估：`.arc/artifacts/http-api-design/reports/forward-review.md`。独立执行者对仅有草案的 45 行输入完成只读评审，确认 10 项设计错误并保留预算/权限/生命周期及运行证据缺口；核验了重复 multipart part、DELETE 不同响应、OAS 3.0 nullable、未知字段与自定义错误格式等不应误报的项目。草案 SHA-256 前后相同；未请求业务服务、上传文件或安装工具。
- 评估修订：修正文档缺陷确认与主检查表的源码 trace 冲突；将 git status 的结论收窄并补被审输入内容哈希；二进制 maxLength 同时引用 OAS 3.0.4/3.1.2 且明确非排他。独立执行者已定向复核三项，记录在原报告尾部，未冒称另一次完整实验。
- 验证：75 入口校验通过；严格门禁 `.arc/artifacts/http-api-design/reports/gate-20261006T145320Z.txt` 为 `0 FAIL / 2 NOT_RUN / 0 WARN`，完整测试 `122 passed / 1 skipped`。4 个新增行为场景覆盖无服务评审、不发明限额、禁止请求及已授权修复交接；静态关键词结果不是实际路由证据。
- 安装：`/Users/iluwen/.agents/skills`；旧 6 包备份在 `/Users/iluwen/.agents/skills-backup-a0z_jsij`。`.arc/artifacts/http-api-design/reports/installed-proof.json` 记录 274 文件 SHA-256/权限一致、Codex/Claude 同目标链接和四个使用入口的 API 参考可达。
- 安装探针纠正：首次错误地要求每个包复制别的技能拥有的参考。按打包器实际设计改为检查拥有者链接的目标存在及安装根约束；共享规则仍随使用包复制，不修改分发逻辑。

限制：本轮为文档/语料改动，门禁格式检查因无已知 formatter 的变动文件而 NOT_RUN，Python lint 因无 Python 改动而 NOT_RUN；1 项既有 R8 protobuf 环境用例跳过。本轮没有指定业务 API，也没有执行真实上传、网关、存储或线上授权验证。技能前向评估和包完整性不代表任何真实 API 已通过审查或获得发布资格。未提交/推送；下轮可加载新版本。
