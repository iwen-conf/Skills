# T01-TOOLS-01 发布闭包

状态：`[x]`

- 上级：[T01-TOOLS](README.md)；所属端：工具；优先级：P1；难度：D3。
- 难度依据：`scripts/sync_skills.py` 的包复制、跨包共享引用、别名转换和符号链接替换共同影响安装结果，必须一起验证。
- 依赖：无；下游：04 本机安装。
- 范围：同步脚本、必要的共享发布实现、`tests/test_sync_skills.py`、README 发布说明。没有服务 API。

<Context-Queries>

```bash
arc-idx symbol sync_one --exact --format json
arc-idx files test_sync_skills.py --format json
arc-idx files execution-truth.md --format json
```

</Context-Queries>

执行：收集源包真实文件依赖；在临时目录打包；按宿主选择单一名称并转换 Markdown/YAML；校验完整引用后替换安装，保留恢复副本。不得静默跳过缺失引用或删除无关技能。

验收：独立目录完整安装、选择安装的依赖、名称/元数据一致、缺失资源失败且原安装不变、符号链接目标不被破坏；运行相应 pytest。

当前验证：11 项发布回归通过（含声明依赖、回滚、未引用断链不打包、跨技能资源进 `_shared`、15 个 Arc 包与会议包可移植）。宿主单一命名；缺失依赖不改目标目录。本机安装见 04。
