#!/usr/bin/env python3
"""
scaffold_score_case.py - 创建 arc:score 工作目录结构

用法:
    python scaffold_score_case.py --project /path/to/project [--output /output/dir]
"""

import argparse
import json
from datetime import datetime
from pathlib import Path


def scaffold_score_case(project_path: str, output_dir: str | None = None) -> dict:
    """
    创建 arc:score 工作目录结构

    Args:
        project_path: 项目根目录
        output_dir: 输出目录，默认 .arc/score/<project-name>

    Returns:
        创建的目录结构信息
    """
    project_path = Path(project_path).resolve()
    project_name = project_path.name

    if output_dir is None:
        output_dir = project_path / ".arc" / "score" / project_name
    else:
        output_dir = Path(output_dir)

    # 创建目录结构
    dirs = ["context", "analysis", "score", "handoff"]

    created_dirs = []
    for d in dirs:
        dir_path = output_dir / d
        dir_path.mkdir(parents=True, exist_ok=True)
        created_dirs.append(str(dir_path))

    # 创建项目快照
    snapshot_path = output_dir / "context" / "project-snapshot.md"
    if not snapshot_path.exists():
        snapshot_content = generate_project_snapshot(project_path)
        snapshot_path.write_text(snapshot_content, encoding="utf-8")

    # 创建元数据
    metadata = {
        "project_name": project_name,
        "project_path": str(project_path),
        "output_dir": str(output_dir),
        "created_at": datetime.now().isoformat(),
        "version": "1.0.0",
    }

    metadata_path = output_dir / "metadata.json"
    metadata_path.write_text(
        json.dumps(metadata, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    return {
        "success": True,
        "project_name": project_name,
        "output_dir": str(output_dir),
        "created_dirs": created_dirs,
        "snapshot_path": str(snapshot_path),
        "metadata_path": str(metadata_path),
    }


def generate_project_snapshot(project_path: Path) -> str:
    """生成项目快照文档"""
    project_name = project_path.name

    # 统计文件
    file_counts = {}
    extensions = {
        ".py",
        ".ts",
        ".tsx",
        ".js",
        ".jsx",
        ".go",
        ".java",
        ".rs",
        ".md",
        ".yaml",
        ".json",
    }

    for ext in extensions:
        count = len(list(project_path.rglob(f"*{ext}")))
        if count > 0:
            file_counts[ext] = count

    # 检测主要语言
    lang_map = {
        ".py": "Python",
        ".ts": "TypeScript",
        ".tsx": "TypeScript",
        ".js": "JavaScript",
        ".jsx": "JavaScript",
        ".go": "Go",
        ".java": "Java",
        ".rs": "Rust",
    }

    lang_counts = {}
    for ext, count in file_counts.items():
        lang = lang_map.get(ext, "Unknown")
        lang_counts[lang] = lang_counts.get(lang, 0) + count

    primary_language = (
        max(lang_counts, key=lang_counts.get) if lang_counts else "Unknown"
    )

    # 生成快照内容
    content = f"""# 项目快照: {project_name}

## 基本信息

| 属性 | 值 |
|------|-----|
| 项目名称 | {project_name} |
| 项目路径 | {project_path} |
| 快照时间 | {datetime.now().strftime("%Y-%m-%d %H:%M:%S")} |
| 主要语言 | {primary_language} |

## 文件统计

| 扩展名 | 文件数 |
|--------|--------|
"""

    for ext, count in sorted(file_counts.items(), key=lambda x: -x[1]):
        content += f"| {ext} | {count} |\n"

    content += """
## 语言分布

| 语言 | 文件数 |
|------|--------|
"""

    for lang, count in sorted(lang_counts.items(), key=lambda x: -x[1]):
        content += f"| {lang} | {count} |\n"

    content += """
## 检测状态

| 检测项 | 状态 |
|--------|------|
| Code Smell | 待执行 |
| Bugfix 分级 | 待执行 |
| 架构检查 | 待执行 |
| 评分聚合 | 待执行 |
"""

    return content


def main():
    parser = argparse.ArgumentParser(description="创建 arc:score 工作目录结构")
    parser.add_argument("--project", required=True, help="项目根目录路径")
    parser.add_argument("--output", help="输出目录路径 (可选)")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出")

    args = parser.parse_args()

    result = scaffold_score_case(args.project, args.output)

    if args.json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print(f"✓ 创建工作目录: {result['output_dir']}")
        print(f"✓ 项目快照: {result['snapshot_path']}")
        print(f"✓ 元数据: {result['metadata_path']}")


if __name__ == "__main__":
    main()
