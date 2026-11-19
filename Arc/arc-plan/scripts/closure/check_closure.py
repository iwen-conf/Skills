#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""arc-plan model 模式的通用业务闭环检查器（模块化 CLI 入口）。"""
from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
import sys
from pathlib import Path

_DIR = str(Path(__file__).resolve().parent)
if _DIR not in sys.path:
    sys.path.insert(0, _DIR)

from core_model import (  # noqa: E402
    MAX_CASCADE, RES_CAP, ROOT_SOURCES, RULES, STATUS_VERSION,
    VALID_ACTOR_KINDS, VALID_KINDS, VALID_SOURCES,
    Ent, Finding, Model, S, Trans, as_list, as_map, build_model, load_spec,
)
from core_engine import (  # noqa: E402
    CascadeOverflow, model_check, model_complete, run_checks,
    sources_of, static_checks, strict_passed, summarize,
)
from core_render import render_mermaid, render_report  # noqa: E402


def _checker_digest() -> str:
    h = hashlib.sha256()
    me = Path(__file__).resolve()
    h.update(me.read_bytes())
    for name in ("core_model.py", "core_engine.py", "core_render.py"):
        p = me.parent / name
        if p.is_file():
            h.update(p.read_bytes())
    return h.hexdigest()


def main(argv=None):
    ap = argparse.ArgumentParser(description="业务闭环检查器（arc-plan model）")
    ap.add_argument("cmd", choices=["check", "gate", "render"])
    ap.add_argument("spec")
    ap.add_argument("--report", help="报告输出路径（默认与 spec 同目录 04-closure-report.md）")
    ap.add_argument("--status", help="状态文件路径（默认与 spec 同目录 .closure-status.json）")
    ap.add_argument("--max-states", type=int, default=300000)
    ap.add_argument("--max-errors", type=int, default=5, help="终端/JSON 最多输出的致命错误数")
    ap.add_argument("--allow-warn", action="store_true", help="仅放宽 check 的诊断退出码；WARN 仍阻断 gate")
    ap.add_argument("--json", action="store_true", help="同时向 stdout 打印紧凑 JSON 结果")
    a = ap.parse_args(argv)
    if a.max_states < 1:
        ap.error("--max-states 必须大于 0")
    d = os.path.dirname(os.path.abspath(a.spec))
    status_path = a.status or os.path.join(d, ".closure-status.json")
    checker_sha = _checker_digest()
    try:
        spec, sha = load_spec(a.spec)
    except Exception as ex:  # noqa
        print(f"[closure] 无法读取 spec：{ex}", file=sys.stderr)
        return 2
    if a.cmd == "gate":
        try:
            with open(status_path, encoding="utf-8") as status_file:
                st = json.load(status_file)
        except (OSError, ValueError) as ex:
            print(f"⛔ 模型闸门关闭：检查记录缺失或无法读取（{ex}）。先运行 check_closure.py check。")
            return 1
        if not isinstance(st, dict) or st.get("status_version") != STATUS_VERSION:
            print("⛔ 模型闸门关闭：检查记录格式过期或无效，必须重新 check。")
            return 1
        if st.get("checker_sha256") != checker_sha:
            print("⛔ 模型闸门关闭：检查器已变更或记录缺少检查器指纹，必须重新 check。")
            return 1
        if st.get("spec_sha256") != sha:
            print("⛔ 闸门关闭：spec 在上次检查后被修改过，必须重新检查。禁止写代码。")
            return 1
        if st.get("passed") is not True or not strict_passed(st.get("summary"), st.get("stats")):
            print("⛔ 模型闸门关闭：需要零未豁免 ERROR/WARN 和完整的状态空间检查。")
            return 1
        print(f"✅ 模型闸门 PASS（{st.get('checked_at')}）。衔接 arc-code / arc-verify。")
        return 0
    if a.cmd == "render":
        m = build_model(spec, lambda *x, **k: None)
        print(render_mermaid(m))
        return 0
    m, findings, stats = run_checks(spec, a.max_states)
    summ = summarize(findings)
    passed = strict_passed(summ, stats)
    report_path = a.report or os.path.join(d, "04-closure-report.md")
    with open(report_path, "w", encoding="utf-8") as f:
        f.write(render_report(m, findings, stats, summ, passed, sha, a.spec))
    with open(status_path, "w", encoding="utf-8") as f:
        json.dump(
            {
                "status_version": STATUS_VERSION,
                "passed": passed,
                "spec_sha256": sha,
                "checker_sha256": checker_sha,
                "check_options": {"max_states": a.max_states, "allow_warn": a.allow_warn},
                "summary": summ,
                "stats": stats,
                "checked_at": datetime.datetime.now().isoformat(timespec="seconds"),
                "findings": [x.to_dict() for x in findings if not x.waived and x.level != "INFO"],
            },
            f,
            ensure_ascii=False,
            indent=2,
        )
    live = [x for x in findings if not x.waived]
    order = {"ERROR": 0, "WARN": 1, "INFO": 2}
    ordered = sorted(live, key=lambda x: (order[x.level], x.rule))
    cap = max(1, a.max_errors)
    for x in ordered[:cap]:
        print(f"[{x.level}] {x.rule} {x.target}: {x.msg}")
        if x.trace:
            print(f"        反例：{x.trace}")
    if len(ordered) > cap:
        print(f"... 其余 {len(ordered) - cap} 条见 {report_path}")
    print(
        f"\n{'✅ 通过' if passed else '❌ 未通过'}  ERROR={summ['errors']} WARN={summ['warns']} "
        f"INFO={summ['infos']} 已豁免={summ['waived']}"
        + (f"  状态空间={stats['states']}" if stats else "")
    )
    print(f"报告：{report_path}")
    diagnostic_ok = a.allow_warn and summ["errors"] == 0 and model_complete(stats)
    if diagnostic_ok and not passed:
        print("--allow-warn 仅令诊断退出码为 0；模型闸门仍为 FAIL。")
    if a.json:
        print(
            json.dumps(
                {
                    "passed": passed,
                    "summary": summ,
                    "top_findings": [x.to_dict() for x in ordered[:cap]],
                    "truncated": len(ordered) > cap,
                },
                ensure_ascii=False,
            )
        )
    return 0 if passed or diagnostic_ok else 1


if __name__ == "__main__":
    sys.exit(main())
