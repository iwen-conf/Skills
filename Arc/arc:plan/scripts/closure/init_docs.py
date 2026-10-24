#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""初始化项目的业务闭环文档（不覆盖已有文件）。

新项目写入 docs/01-系统设计/业务闭环/；已有 docs/arc/03-closure-spec.yaml 的旧项目继续使用 docs/arc/。

用法：python init_docs.py <项目根目录> [--with-rules] [--refresh-tool]
  --with-rules  把"业务闭环铁律"追加进项目的 CLAUDE.md（已存在同名段落则跳过）
  --refresh-tool  更新项目内检查器副本；不覆盖已有文档，更新后必须重新 check
"""
import argparse
import os
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ASSETS = os.path.join(HERE, "assets")
MARK = "## 业务闭环铁律（arc:plan model）"
LEGACY_MARKS = ("## 业务闭环铁律（arc-closure）",)
NEW_BASE = ("docs", "01-系统设计", "业务闭环")
LEGACY_BASE = ("docs", "arc")


def put(src, dst, created, skipped):
    if os.path.exists(dst):
        skipped.append(dst)
        return
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    shutil.copyfile(src, dst)
    created.append(dst)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("root")
    ap.add_argument("--with-rules", action="store_true")
    ap.add_argument("--refresh-tool", action="store_true")
    a = ap.parse_args()
    legacy = os.path.join(a.root, *LEGACY_BASE)
    # 旧项目已有 docs/arc/ 模型时沿用原路径，避免出现两份 canonical spec。
    base = legacy if os.path.exists(os.path.join(legacy, "03-closure-spec.yaml")) else os.path.join(a.root, *NEW_BASE)
    created, skipped = [], []
    for name in sorted(os.listdir(os.path.join(ASSETS, "templates"))):
        if name == "06-audit-report.md":
            continue  # 审查报告只由审查阶段写到 .arc/artifacts/，不初始化进设计文档。
        target = "README.md" if name == "00-README.md" and base.endswith("业务闭环") else name
        put(os.path.join(ASSETS, "templates", name), os.path.join(base, target), created, skipped)
    put(os.path.join(ASSETS, "spec.template.yaml"), os.path.join(base, "03-closure-spec.yaml"), created, skipped)
    tool_source = os.path.join(HERE, "check_closure.py")
    tool_target = os.path.join(base, "tools", "check_closure.py")
    if a.refresh_tool and os.path.exists(tool_target):
        shutil.copyfile(tool_source, tool_target)
        created.append(tool_target + "（已更新；需重新 check）")
    else:
        put(tool_source, tool_target, created, skipped)
    if a.with_rules:
        cm = os.path.join(a.root, "CLAUDE.md")
        snippet = open(os.path.join(ASSETS, "claude-md-snippet.md"), encoding="utf-8").read()
        old = open(cm, encoding="utf-8").read() if os.path.exists(cm) else ""
        if MARK in old or any(mark in old for mark in LEGACY_MARKS):
            skipped.append(cm + "（已含闭环铁律）")
        else:
            with open(cm, "a", encoding="utf-8") as f:
                f.write(("\n\n" if old.strip() else "") + snippet)
            created.append(cm + "（已追加闭环铁律）")
    print("已创建：")
    for p in created:
        print("  +", p)
    if skipped:
        print("已存在，跳过：")
        for p in skipped:
            print("  =", p)
    rel = os.path.relpath(base, a.root)
    print("\n下一步：在 01-requirements.md 链接既有需求及 REQ ID，填写 03-closure-spec.yaml，然后运行\n"
          f"  python {rel}/tools/check_closure.py check {rel}/03-closure-spec.yaml")


if __name__ == "__main__":
    sys.exit(main())
