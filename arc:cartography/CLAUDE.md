[根目录](../CLAUDE.md) > **arc:cartography**

# arc:cartography -- 仓库分层代码地图生成

## 变更记录 (Changelog)

| 时间 | 操作 |
|------|------|
| 2026-03-03 | 新增模块级 CLAUDE.md，并接入根文档技能索引 |

## 模块职责

arc:cartography 提供仓库理解与分层 codemap 生成能力，帮助在陌生项目中快速建立目录级与根级代码地图。

核心能力：
- 初始化扫描目标文件并建立 `.slim/cartography.json` 状态
- 基于文件/目录哈希执行增量更新
- 生成目录级 `codemap.md` 与根级仓库 atlas

## 入口与启动

### 入口文件

| 文件 | 用途 |
|------|------|
| `SKILL.md` | Skill 定义（权威规范） |
| `scripts/cartographer.py` | `init` / `changes` / `update` 主逻辑 |
| `.slim/cartography.json` | 状态缓存（文件哈希 + 目录哈希） |

### 启动命令

```bash
python3 arc:cartography/scripts/cartographer.py init --root . --include "src/**" --exclude "**/node_modules/**"
python3 arc:cartography/scripts/cartographer.py changes --root .
python3 arc:cartography/scripts/cartographer.py update --root .
```

## 对外接口

### Skill 调用接口

通过 Claude Code 调用：`arc cartography`

### CLI 接口

| 子命令 | 说明 |
|------|------|
| `init` | 首次扫描并初始化状态与模板 |
| `changes` | 检测新增/修改/删除文件与受影响目录 |
| `update` | 仅重建受影响目录 codemap 并更新根图谱 |

## 关键产物

- `.slim/cartography.json`：增量检测状态文件
- `**/codemap.md`：目录级代码地图
- `codemap.md`：根级仓库地图（聚合索引）

## 与 arc 技能协同

- 可在 `arc:clarify` 前执行，补充问题上下文
- 可在 `arc:build` 前执行，快速定位模块边界
- 可在 `arc:audit` 前执行，辅助评审范围划分
