# T01-TOOLS-02 校验与意图验收

状态：`[x]`

- 上级：[T01-TOOLS](README.md)；所属端：工具；优先级：P2；难度：D2。
- 难度依据：文件发现、YAML 解析和注册表共享调用点；关键词测试只能验证静态词汇，需与行为评估分开。
- 依赖：无；下游：04 本机安装。
- 范围：`src/arc_core/usecase/validate_skills.py`、`infrastructure/markdown.py`、相关测试、触发语料与评估说明；不接入新模型 API。

<Context-Queries>

```bash
arc-idx symbol collect_skill_files --exact --format json
arc-idx symbol winner --exact --format json
arc-idx files test_skill_validation.py --format json
```

</Context-Queries>

执行：收集入口不依赖成功解析；解析失败和非映射 frontmatter 明确报错；把关键词检查标注为静态检查；新增否定、多意图、范围与授权保持的真实验收场景。不得以硬编码修正两条句子来宣称解决通用语义路由。

验收：坏入口被计入并报错；已有注册表行为保留；测试语料和评估结果能区分静态门禁与实际执行证据；运行 pytest 和校验 CLI。

当前验证：入口收集与 YAML 内容验证已分离；坏 YAML/缺失头部/非映射类型计入门禁。`winner()` 标明为静态词表；`schemas/behavior_corpus.yaml` 覆盖否定、多意图、范围、授权，且静态命中不得当作运行时证据。
