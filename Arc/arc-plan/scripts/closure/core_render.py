"""Markdown report and Mermaid renderer for closure specs."""
from __future__ import annotations
import datetime
import os
from core_model import ROOT_SOURCES, RULES
from core_engine import model_complete

def render_report(m, findings, stats, summ, passed, sha, path):
    L = []
    now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    L.append(f"# 业务闭环检查报告\n\n> 自动生成，请勿手改。spec：`{os.path.basename(path)}`　时间：{now}　sha256：`{sha[:12]}`\n")
    L.append(f"## 模型检查：{'PASS——模型检查通过' if passed else 'FAIL——模型检查未通过'}\n")
    L.append(f"- ERROR（闭环断裂）：**{summ['errors']}**　WARN（风险/不完整）：**{summ['warns']}**　"
             f"INFO：{summ['infos']}　已豁免：{summ['waived']}")
    if stats:
        L.append(f"- 状态空间：穷举 {stats['states']} 个可达全局状态，其中 {stats['complete']} 个为完结状态"
                 f"{'（⚠ 已截断，结果不完整）' if stats['truncated'] else ''}")
    else:
        L.append("- 状态空间：NOT_RUN（结构错误导致动态检查未执行）；豁免不补足执行证据。")
    L.append("- 实现与运行时验证：NOT_RUN（本命令不执行产品测试）；模型 PASS 不代表实现完成或可发布。")
    L.append("")
    live = [f for f in findings if not f.waived]
    if live:
        L.append("## 问题清单\n")
        order = {"ERROR": 0, "WARN": 1, "INFO": 2}
        for f in sorted(live, key=lambda x: (order[x.level], int(x.rule[1:]) if x.rule[1:].isdigit() else 99, x.target)):
            L.append(f"- **[{f.level}] {f.rule} {RULES.get(f.rule, '')} · {f.target}**：{f.msg}")
            if f.trace:
                L.append(f"  - 反例路径：`{f.trace}`")
        L.append("")
    wv = [f for f in findings if f.waived]
    if wv:
        L.append("## 已豁免（需评审人确认理由成立）\n")
        for f in wv:
            L.append(f"- {f.rule} · {f.target}：{f.msg}\n  - 理由：{f.reason}")
        L.append("")
    ig = [(e.name, ev, sorted(ss), why) for e in m.ents.values() for ev, ss, why in e.ignores]
    if ig:
        L.append("## 忽略清单（声明\"此状态下收到该事件可安全忽略\"，评审必看）\n")
        L.append("| 实体 | 事件 | 在这些状态忽略 | 理由 |\n|---|---|---|---|")
        for n, ev, ss, why in ig:
            L.append(f"| {n} | {ev} | {'、'.join(ss)} | {why} |")
        L.append("")
    L.append("## 状态流转总表\n")
    for e in m.ents.values():
        L.append(f"### {e.name}（初始：{e.initial}；终态：{'、'.join(sorted(e.terminal))}）\n")
        L.append("| id | 当前状态 | 事件(来源) | 角色 | 下一状态 | 类型 | 联动事件 |\n|---|---|---|---|---|---|---|")
        for t in e.trans:
            L.append(f"| {t.id} | {t.frm} | {t.ev}({m.src.get(t.ev)}) | {t.actor} | {t.to} | {t.kind} | {'、'.join(t.emits)} |")
        L.append("")
    rp = []
    for e in m.ents.values():
        for s in e.states:
            roots = sorted({t.ev for t in e.out(s) if m.src.get(t.ev) in ROOT_SOURCES})
            if len(roots) >= 2:
                rp.append(f"| {e.name}.{s} | {'、'.join(roots)} | {e.race.get(s) or '❌ 未声明'} |")
    if rp:
        L.append("## 竞态点\n\n| 位置 | 可能同时到来的事件 | 裁决策略 |\n|---|---|---|")
        L += rp
        L.append("")
    L.append("---\n规则说明见 skill 的 `references/rules.md`。通过仅表示所建模型在本检查器边界内未发现未豁免问题；"
             "业务来源与人工推演见 `references/closure-loopholes.md`，真实行为由 arc:verify 验证。"
             "编码还须满足现有授权和范围，未决业务假设、豁免与范围决策需确认；已有明确授权不重复索取。")
    return "\n".join(L) + "\n"


def render_mermaid(m):
    out = ["# 状态图（由 spec 自动生成）\n"]
    for e in m.ents.values():
        out.append(f"## {e.name}\n\n```mermaid\nstateDiagram-v2")
        out.append(f"    [*] --> {e.initial}")
        for t in e.trans:
            lab = f"{t.ev}[{t.actor}]" + ("" if t.kind == "normal" else " ⚠")
            out.append(f"    {t.frm} --> {t.to}: {lab}")
        for s in sorted(e.terminal):
            out.append(f"    {s} --> [*]")
        out.append("```\n")
    return "\n".join(out)

