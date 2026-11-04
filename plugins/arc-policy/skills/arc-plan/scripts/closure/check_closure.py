#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
arc:plan model 模式的通用业务闭环检查器（与具体业务无关）。

读取一份符合 references/closure-spec-format.md 的 spec（YAML 或 JSON），
执行 R1–R20 检查，并对"状态机 + 事件联动"做状态空间穷举（模型检查）。

用法：
  python check_closure.py check  docs/01-系统设计/业务闭环/03-closure-spec.yaml   # 检查并生成报告与状态文件
  python check_closure.py gate   docs/01-系统设计/业务闭环/03-closure-spec.yaml   # 模型闸门：严格检查通过且 spec/检查器未改动
  python check_closure.py render docs/01-系统设计/业务闭环/03-closure-spec.yaml   # 输出 Mermaid 状态图

退出码：0 通过 / 1 存在问题 / 2 输入无法读取；check --allow-warn 仅放宽诊断退出码，不开启闸门。
依赖：Python 3.8+；读取 YAML 需要 PyYAML（pip install pyyaml），或把 spec 存成 .json。
"""
from __future__ import annotations

import argparse
import collections
import datetime
import hashlib
import json
import os
import sys

try:
    import yaml  # type: ignore
except ImportError:  # pragma: no cover
    yaml = None

RULES = {
    "R1": "初始态/终态完整", "R2": "所有状态可达", "R3": "无死胡同、无死循环",
    "R4": "同一(状态+事件)结果确定", "R5": "事件有人发、有人收", "R6": "引用完整、无重名、格式正确",
    "R7": "角色已定义且被使用", "R8": "每个非终态都有异常出口", "R9": "数据有写有读",
    "R10": "场景衔接且覆盖所有流转", "R11": "每个实体有异常场景", "R12": "需求可追溯到场景",
    "R13": "事件联动环有终止条件", "R14": "不变量在所有可达状态成立",
    "R15": "任何可达状态都能走到完结", "R16": "资源占用与释放守恒",
    "R17": "迟到/重复/乱序事件有处理", "R18": "人工操作有反馈",
    "R19": "等待态有兜底(超时/联动)", "R20": "竞态点有裁决策略", "W": "豁免有效性",
}
VALID_SOURCES = {"actor", "external", "timer", "internal"}
ROOT_SOURCES = {"actor", "external", "timer"}      # 可在任意时刻由外部到来的事件
VALID_ACTOR_KINDS = {"human", "system", "external"}
VALID_KINDS = {"normal", "exception"}
MAX_CASCADE = 60
RES_CAP = 3
STATUS_VERSION = 2


# ----------------------------------------------------------------- 基础工具
def S(x):
    return None if x is None else str(x)


def as_list(x):
    if x is None:
        return []
    return list(x) if isinstance(x, (list, tuple)) else [x]


def as_map(x):
    return x if isinstance(x, dict) else {}


class Finding:
    def __init__(self, rule, level, target, msg, trace=""):
        self.rule, self.level, self.target, self.msg, self.trace = rule, level, target, msg, trace
        self.waived, self.reason = False, ""

    def to_dict(self):
        return {"rule": self.rule, "level": self.level, "target": self.target,
                "msg": self.msg, "trace": self.trace, "waived": self.waived, "reason": self.reason}


class Trans:
    def __init__(self, ent, d):
        self.ent = ent
        self.id, self.frm, self.ev = S(d.get("id")), S(d.get("from")), S(d.get("event"))
        self.actor, self.to = S(d.get("actor")), S(d.get("to"))
        self.kind = S(d.get("kind") or "normal")
        self.guard = (S(d.get("guard")) or "").strip()
        self.emits = [S(x) for x in as_list(d.get("emits"))]
        self.feedback = (S(d.get("feedback")) or "").strip()
        self.acq = [S(x) for x in as_list(d.get("acquires"))]
        self.rel = [S(x) for x in as_list(d.get("releases"))]
        self.term_by = (S(d.get("terminates_by")) or "").strip()


class Ent:
    def __init__(self, name):
        self.name = name
        self.states, self.initial, self.terminal = [], None, set()
        self.created = None            # {event, actor, emits, acq, rel}
        self.trans = []
        self.ignores = []              # [(event, set(states), reason)]
        self.race = {}
        self.fields = {}
        self.by_se = collections.defaultdict(list)   # (state,event) -> [Trans]

    def out(self, s):
        return [t for t in self.trans if t.frm == s]


class Model:
    pass


# ----------------------------------------------------------------- 读取与建模
def load_spec(path):
    with open(path, "rb") as f:
        raw = f.read()
    sha = hashlib.sha256(raw).hexdigest()
    text = raw.decode("utf-8")
    if path.lower().endswith(".json"):
        return json.loads(text), sha
    if yaml is None:
        raise RuntimeError("读取 YAML 需要 PyYAML：pip install pyyaml（或把 spec 存成 .json）")
    return yaml.safe_load(text), sha


def build_model(spec, add):
    m = Model()
    spec = as_map(spec)
    m.meta = as_map(spec.get("meta"))
    m.actors = {S(k): as_map(v) for k, v in as_map(spec.get("actors")).items()}
    m.events = {S(k): as_map(v) for k, v in as_map(spec.get("events")).items()}
    m.resources = {S(k): v for k, v in as_map(spec.get("resources")).items()}
    m.reqs = {S(k): v for k, v in as_map(spec.get("requirements")).items()}
    m.waivers = [as_map(w) for w in as_list(spec.get("waivers"))]
    m.ents = collections.OrderedDict()
    m.scen = collections.OrderedDict()
    m.invs = []
    m.fatal = False

    if not m.actors:
        add("R6", "ERROR", "actors", "未定义任何角色(actors)")
    for a, d in m.actors.items():
        if S(d.get("kind")) not in VALID_ACTOR_KINDS:
            add("R6", "ERROR", a, "角色 kind 必须是 human / system / external 之一")
    for ev, d in m.events.items():
        if S(d.get("source")) not in VALID_SOURCES:
            add("R6", "ERROR", ev, "事件 source 必须是 actor / external / timer / internal 之一")
            m.fatal = True
    ev_src = {ev: S(d.get("source")) for ev, d in m.events.items()}
    m.src = ev_src

    raw_ents = as_map(spec.get("entities"))
    if not raw_ents:
        add("R6", "ERROR", "entities", "未定义任何实体(entities)")
        m.fatal = True
    seen_ids = {}
    for name, d in raw_ents.items():
        name = S(name)
        d = as_map(d)
        e = Ent(name)
        m.ents[name] = e
        e.states = [S(x) for x in as_list(d.get("states"))]
        if not e.states:
            add("R6", "ERROR", name, "实体缺少 states 列表")
            m.fatal = True
        init = d.get("initial")
        if isinstance(init, (list, dict)) or init is None:
            add("R1", "ERROR", name, "必须有且仅有一个 initial(单个状态名)")
            m.fatal = True
        else:
            e.initial = S(init)
            if e.initial not in e.states:
                add("R6", "ERROR", name, f"initial「{e.initial}」不在 states 中")
                m.fatal = True
        e.terminal = {S(x) for x in as_list(d.get("terminal"))}
        for s in e.terminal:
            if s not in e.states:
                add("R6", "ERROR", name, f"terminal「{s}」不在 states 中")
        if len(set(e.states)) != len(e.states):
            add("R6", "ERROR", name, "states 中有重复状态名")
        cb = d.get("created_by")
        if not isinstance(cb, dict) or not cb.get("event") or not cb.get("actor"):
            add("R6", "ERROR", name, "实体缺少 created_by（必须写明由哪个 event、哪个 actor 创建）")
            m.fatal = True
        else:
            e.created = {"event": S(cb["event"]), "actor": S(cb["actor"]),
                         "emits": [S(x) for x in as_list(cb.get("emits"))],
                         "acq": [S(x) for x in as_list(cb.get("acquires"))],
                         "rel": [S(x) for x in as_list(cb.get("releases"))]}
            c = e.created
            if c["event"] not in m.events:
                add("R6", "ERROR", f"{name}.created_by", f"引用了未定义事件「{c['event']}」")
            if c["actor"] not in m.actors:
                add("R6", "ERROR", f"{name}.created_by", f"引用了未定义角色「{c['actor']}」")
            for x in c["emits"]:
                if x not in m.events:
                    add("R6", "ERROR", f"{name}.created_by", f"emits 引用了未定义事件「{x}」")
            for x in c["acq"] + c["rel"]:
                if x not in m.resources:
                    add("R6", "ERROR", f"{name}.created_by", f"引用了未定义资源「{x}」")
        for td in as_list(d.get("transitions")):
            td = as_map(td)
            t = Trans(name, td)
            miss = [k for k, v in (("id", t.id), ("from", t.frm), ("event", t.ev),
                                   ("actor", t.actor), ("to", t.to)) if not v]
            if miss:
                add("R6", "ERROR", f"{name}.{t.id or '?'}", f"流转缺少字段：{', '.join(miss)}")
                m.fatal = True
                continue
            if t.id in seen_ids:
                add("R6", "ERROR", t.id, f"流转 id 重复（也出现在 {seen_ids[t.id]}）")
                continue
            seen_ids[t.id] = name
            if t.frm not in e.states or t.to not in e.states:
                add("R6", "ERROR", t.id, f"from/to 引用了 {name} 中不存在的状态（{t.frm}→{t.to}）")
                m.fatal = True
                continue
            if t.ev not in m.events:
                add("R6", "ERROR", t.id, f"引用了未定义事件「{t.ev}」")
                m.fatal = True
                continue
            if t.actor not in m.actors:
                add("R6", "ERROR", t.id, f"引用了未定义角色「{t.actor}」")
            if t.kind not in VALID_KINDS:
                add("R6", "ERROR", t.id, "kind 必须是 normal 或 exception")
            for x in t.emits:
                if x not in m.events:
                    add("R6", "ERROR", t.id, f"emits 引用了未定义事件「{x}」")
            for x in t.acq + t.rel:
                if x not in m.resources:
                    add("R6", "ERROR", t.id, f"引用了未定义资源「{x}」")
            e.trans.append(t)
            e.by_se[(t.frm, t.ev)].append(t)
        for ig in as_list(d.get("ignores")):
            ig = as_map(ig)
            evn, why = S(ig.get("event")), (S(ig.get("reason")) or "").strip()
            toks = [S(x) for x in as_list(ig.get("states"))]
            if evn not in m.events:
                add("R6", "ERROR", f"{name}.ignores", f"引用了未定义事件「{evn}」")
                continue
            if not why:
                add("R6", "ERROR", f"{name}.ignores", f"忽略事件「{evn}」必须写 reason")
            ss = set()
            for tk in toks:
                if tk == "@terminal":
                    ss |= e.terminal
                elif tk == "@others":
                    ss |= {s for s in e.states if not e.by_se.get((s, evn))}
                elif tk in e.states:
                    ss.add(tk)
                else:
                    add("R6", "ERROR", f"{name}.ignores", f"states 引用了不存在的状态「{tk}」")
            e.ignores.append((evn, ss, why))
        for s, why in as_map(d.get("race_policy")).items():
            if S(s) not in e.states:
                add("R6", "ERROR", f"{name}.race_policy", f"引用了不存在的状态「{s}」")
            e.race[S(s)] = (S(why) or "").strip()
        e.fields = {S(k): as_map(v) for k, v in as_map(d.get("fields")).items()}

    tid = {t.id: t for e in m.ents.values() for t in e.trans}
    m.tid = tid

    for sname, sd in as_map(spec.get("scenarios")).items():
        sname, sd = S(sname), as_map(sd)
        sc = Model()
        sc.name, sc.type, sc.entity = sname, S(sd.get("type")), S(sd.get("entity"))
        sc.req = [S(x) for x in as_list(sd.get("req"))]
        sc.steps = []
        if sc.type not in ("normal", "exception"):
            add("R6", "ERROR", sname, "场景 type 必须是 normal 或 exception")
        if sc.entity not in m.ents:
            add("R6", "ERROR", sname, f"场景主实体「{sc.entity}」未定义")
        for sid in as_list(sd.get("steps")):
            if S(sid) not in tid:
                add("R6", "ERROR", sname, f"步骤引用了不存在的流转「{sid}」")
            else:
                sc.steps.append(tid[S(sid)])
        for r in sc.req:
            if r not in m.reqs:
                add("R6", "ERROR", sname, f"引用了未登记的需求「{r}」")
        m.scen[sname] = sc

    for i, inv in enumerate(as_list(spec.get("invariants"))):
        inv = as_map(inv)
        iv = Model()
        iv.id = S(inv.get("id")) or f"INV-{i + 1}"
        iv.desc = S(inv.get("desc")) or ""
        iv.when = {S(k): [S(x) for x in as_list(v)] for k, v in as_map(inv.get("when")).items()}
        iv.then = {S(k): [S(x) for x in as_list(v)] for k, v in as_map(inv.get("then")).items()}
        if not iv.when or not iv.then:
            add("R6", "ERROR", iv.id, "不变量必须同时有 when 和 then")
        for part in (iv.when, iv.then):
            for en, ss in part.items():
                if en not in m.ents:
                    add("R6", "ERROR", iv.id, f"引用了未定义实体「{en}」")
                    continue
                for s in ss:
                    if s != "@none" and s not in m.ents[en].states:
                        add("R6", "ERROR", iv.id, f"实体 {en} 没有状态「{s}」")
        m.invs.append(iv)

    for w in m.waivers:
        if not S(w.get("rule")) or not S(w.get("target")) or not (S(w.get("reason")) or "").strip():
            add("R6", "ERROR", "waivers", "每条豁免必须有 rule、target、reason")
    return m


# ----------------------------------------------------------------- 静态检查
def sources_of(m, ev):
    return m.src.get(ev)


def static_checks(m, add):
    ents = m.ents
    # R1 / R2 / R3 / R4
    for e in ents.values():
        if not e.terminal:
            add("R1", "ERROR", e.name, "没有终态(terminal)，生命周期无法终结")
        for s in sorted(e.terminal):
            if e.out(s):
                add("R1", "ERROR", f"{e.name}.{s}", "终态不应再有出边（若还能继续流转，它就不是终态）")
        if e.initial is None:
            continue
        adj = collections.defaultdict(set)
        radj = collections.defaultdict(set)
        for t in e.trans:
            adj[t.frm].add(t.to)
            radj[t.to].add(t.frm)
        seen, q = {e.initial}, collections.deque([e.initial])
        while q:
            x = q.popleft()
            for y in adj[x]:
                if y not in seen:
                    seen.add(y)
                    q.append(y)
        for s in e.states:
            if s not in seen:
                add("R2", "ERROR", f"{e.name}.{s}", "从初始态走不到该状态（孤立/多余状态，或缺少进入它的流转）")
        good, q = set(e.terminal), collections.deque(e.terminal)
        while q:
            x = q.popleft()
            for y in radj[x]:
                if y not in good:
                    good.add(y)
                    q.append(y)
        for s in e.states:
            if s in e.terminal:
                continue
            if not e.out(s):
                add("R3", "ERROR", f"{e.name}.{s}", "非终态没有任何出口（死胡同）")
            elif s not in good and s in seen:
                add("R3", "ERROR", f"{e.name}.{s}", "只能在环内打转，永远走不到终态（死循环）")
        groups = collections.defaultdict(list)
        for t in e.trans:
            groups[(t.frm, t.ev)].append(t)
        for (s, ev), ts in groups.items():
            if len(ts) > 1:
                guards = [t.guard for t in ts]
                if any(not g for g in guards) or len(set(guards)) != len(guards):
                    add("R4", "ERROR", f"{e.name}.{s}+{ev}",
                        f"同一(状态+事件)有 {len(ts)} 条流转（{', '.join(t.id for t in ts)}），"
                        "必须每条都写互斥且不同的 guard，否则结果不确定")
    # R5
    cons, prod = collections.defaultdict(list), collections.defaultdict(list)
    for e in ents.values():
        for t in e.trans:
            cons[t.ev].append(t.id)
            for x in t.emits:
                prod[x].append(t.id)
        if e.created:
            cons[e.created["event"]].append(f"{e.name}.created_by")
            for x in e.created["emits"]:
                prod[x].append(f"{e.name}.created_by")
    for ev, d in m.events.items():
        sinks = as_list(d.get("sinks"))
        if not cons[ev] and not sinks:
            add("R5", "ERROR", ev, "事件无人处理（既没有流转消费，也没有声明 sinks）——发出去就石沉大海")
        if m.src.get(ev) == "internal" and not prod[ev]:
            add("R5", "ERROR", ev, "internal 事件无人发出（没有任何流转/创建的 emits 指向它）")
        if m.src.get(ev) == "timer" and not S(d.get("after")):
            add("R5", "WARN", ev, "定时事件必须写 after（多久后触发），否则无法判断超时是否合理")
    # R7
    used = {t.actor for e in ents.values() for t in e.trans}
    used |= {e.created["actor"] for e in ents.values() if e.created}
    for a in m.actors:
        if a not in used:
            add("R7", "ERROR", a, "角色没有参与任何流转或创建——要么多余，要么缺少该角色的业务")
    # R8
    for e in ents.values():
        for s in e.states:
            if s in e.terminal:
                continue
            outs = e.out(s)
            if outs and not any(t.kind == "exception" for t in outs):
                add("R8", "ERROR", f"{e.name}.{s}", "该状态只有正向出口，没有取消/拒绝/超时/失败等异常出口")
    # R9
    nfields = 0
    for e in ents.values():
        for fname, fd in e.fields.items():
            nfields += 1
            for role, key in (("写入者", "writers"), ("读取者", "readers")):
                refs = [S(x) for x in as_list(fd.get(key))]
                if not refs:
                    add("R9", "ERROR", f"{e.name}.{fname}",
                        f"字段没有{role}（{'只读不写，数据从哪来？' if key == 'writers' else '只写不读，写了给谁用？'}）")
                for r in refs:
                    if not r.startswith("@") and r not in m.tid:
                        add("R6", "ERROR", f"{e.name}.{fname}", f"{key} 引用了不存在的流转「{r}」（外部来源/用途请以 @ 开头，如 @创建、@报表）")
    if nfields == 0:
        add("R9", "WARN", "fields", "未登记任何数据字段，无法检查数据闭环")
    # R10 / R11 / R12
    emitted0, created = set(), {n for n, e in ents.items() if e.created and m.src.get(e.created["event"]) != "internal"}
    changed = True
    while changed:
        changed = False
        for n in list(created):
            for x in ents[n].created["emits"]:
                if x not in emitted0:
                    emitted0.add(x)
                    changed = True
        for n, e in ents.items():
            if n not in created and e.created and e.created["event"] in emitted0:
                created.add(n)
                changed = True
    covered = set()
    for sc in m.scen.values():
        emitted, cur = set(emitted0), {}
        for k, t in enumerate(sc.steps):
            covered.add(t.id)
            if m.src.get(t.ev) == "internal" and t.ev not in emitted:
                add("R10", "ERROR", sc.name, f"第{k + 1}步 {t.id} 的触发事件「{t.ev}」没有被前序步骤发出——场景断链")
            e = ents[t.ent]
            if t.ent not in cur:
                if t.frm != e.initial:
                    add("R10", "ERROR", sc.name, f"第{k + 1}步 {t.id}：实体 {t.ent} 首次出现，必须从初始态「{e.initial}」出发，而它从「{t.frm}」出发")
            elif t.frm != cur[t.ent]:
                add("R10", "ERROR", sc.name, f"第{k + 1}步 {t.id}：{t.ent} 上一步已到「{cur[t.ent]}」，本步却从「{t.frm}」出发——步骤衔接断裂")
            cur[t.ent] = t.to
            emitted |= set(t.emits)
        if sc.entity in ents:
            pe = ents[sc.entity]
            if sc.entity not in cur:
                add("R10", "ERROR", sc.name, f"场景没有任何步骤属于主实体 {sc.entity}")
            elif cur[sc.entity] not in pe.terminal:
                add("R10", "ERROR", sc.name, f"主实体 {sc.entity} 最终停在「{cur[sc.entity]}」，不是终态——场景没有走完")
    for t in m.tid.values():
        if t.id not in covered:
            add("R10", "ERROR", t.id, f"流转 {t.ent}:{t.frm}--{t.ev}-->{t.to} 没有被任何场景覆盖（没人验证过这条路能走通）")
    for e in ents.values():
        ok = any(sc.type == "exception" and any(t.ent == e.name and t.kind == "exception" for t in sc.steps)
                 for sc in m.scen.values())
        if not ok:
            add("R11", "ERROR", e.name, "没有任何异常场景覆盖该实体的异常流转")
    if not m.reqs:
        add("R12", "ERROR", "requirements", "未登记任何需求，业务设计无法追溯到需求")
    covered_req = {r for sc in m.scen.values() for r in sc.req}
    for r in m.reqs:
        if r not in covered_req:
            add("R12", "ERROR", r, "需求没有被任何场景覆盖——需求没有落地成业务流程")
    for sc in m.scen.values():
        if not sc.req:
            add("R12", "ERROR", sc.name, "场景没有关联任何需求(req)——为什么存在这个场景？")
    # R13（静态）：internal 事件自动联动成环
    g = collections.defaultdict(set)
    carriers = collections.defaultdict(list)
    for e in ents.values():
        for t in e.trans:
            if m.src.get(t.ev) == "internal":
                for x in t.emits:
                    g[t.ev].add(x)
                    carriers[(t.ev, x)].append(t)
        if e.created and m.src.get(e.created["event"]) == "internal":
            for x in e.created["emits"]:
                g[e.created["event"]].add(x)
    index, low, onst, st, sccs, counter = {}, {}, set(), [], [], [0]

    def strong(v):
        index[v] = low[v] = counter[0]
        counter[0] += 1
        st.append(v)
        onst.add(v)
        for w in g[v]:
            if w not in index:
                strong(w)
                low[v] = min(low[v], low[w])
            elif w in onst:
                low[v] = min(low[v], index[w])
        if low[v] == index[v]:
            comp = []
            while True:
                w = st.pop()
                onst.discard(w)
                comp.append(w)
                if w == v:
                    break
            sccs.append(comp)

    for v in list(g):
        if v not in index:
            strong(v)
    for comp in sccs:
        if len(comp) > 1 or comp[0] in g[comp[0]]:
            cs = set(comp)
            ts = [t for e in ents.values() for t in e.trans
                  if t.ev in cs and any(x in cs for x in t.emits)]
            if not any(t.term_by for t in ts):
                add("R13", "ERROR", "|".join(sorted(cs)),
                    "事件联动形成闭环（A 触发 B 又触发 A…），没有任何流转声明 terminates_by，可能无限循环")
    # R18
    for e in ents.values():
        for t in e.trans:
            if m.actors.get(t.actor, {}).get("kind") == "human" and not t.feedback:
                add("R18", "ERROR", t.id, f"人工操作（{t.actor}：{t.ev}）没有 feedback——操作完成后他看到什么、下一步谁接手？")
    # R19
    for e in ents.values():
        for s in e.states:
            if s in e.terminal:
                continue
            outs = e.out(s)
            if outs and not any(m.src.get(t.ev) in ("timer", "internal") for t in outs):
                add("R19", "WARN", f"{e.name}.{s}", "该状态的所有出口都依赖人或外部系统，没有超时/联动兜底——对方不来就永远悬挂")
    # R20
    for e in ents.values():
        for s in e.states:
            if s in e.terminal:
                continue
            roots = {t.ev for t in e.out(s) if m.src.get(t.ev) in ROOT_SOURCES}
            if len(roots) >= 2 and not e.race.get(s):
                add("R20", "WARN", f"{e.name}.{s}",
                    f"竞态点：{'、'.join(sorted(roots))} 都可能在此状态同时到来，需在 race_policy 写明谁优先、败者如何处理")
    return cons, prod


# ----------------------------------------------------------------- 模型检查
class CascadeOverflow(Exception):
    pass


def model_check(m, add, max_states):
    ents = m.ents
    names = list(ents)
    idx = {n: i for i, n in enumerate(names)}
    res_names = list(m.resources)
    ridx = {r: i for i, r in enumerate(res_names)}
    subs = collections.defaultdict(list)
    for n in names:
        e = ents[n]
        evs = {t.ev for t in e.trans}
        if e.created:
            evs.add(e.created["event"])
        for ev in sorted(evs):
            subs[ev].append(n)
    ign = {n: {(s, ev) for ev, ss, _ in ents[n].ignores for s in ss} for n in names}
    found = {}
    ctx = {"g": None, "ev": None}
    visited = {}

    def trace_of(g, extra=None):
        evs, cur = [], g
        while visited.get(cur) is not None:
            p, ev = visited[cur]
            evs.append(ev)
            cur = p
        evs.reverse()
        if extra:
            evs.append(extra)
        return " → ".join(evs) if evs else "(初始)"

    def snap(g):
        return "；".join(f"{n}={g[0][idx[n]] or '∅'}" for n in names)

    def record(key, rule, level, target, msg, g=None, ev=None):
        if key in found:
            return
        g = g or ctx["g"]
        ev = ev if ev is not None else ctx["ev"]
        found[key] = (rule, level, target, msg, f"{trace_of(g, ev)}  ⇒  {snap(g)}")

    def apply_res(bal, acq, rel, who):
        b = list(bal)
        for r in acq:
            b[ridx[r]] += 1
            if b[ridx[r]] > RES_CAP:
                record(("R16", r, "dup"), "R16", "ERROR", r, f"资源「{r}」被重复占用超过 {RES_CAP} 次（{who}），疑似泄漏或重复获取")
                return None
        for r in rel:
            b[ridx[r]] -= 1
            if b[ridx[r]] < 0:
                record(("R16", r, "neg"), "R16", "ERROR", r, f"资源「{r}」在未占用时被释放（{who}），疑似重复释放")
                return None
        return tuple(b)

    def tasks_for(evs):
        return tuple((ev, n) for ev in evs for n in subs.get(ev, ()))

    def cascade(est, bal, tasks, depth):
        if not tasks:
            yield est, bal
            return
        if depth > MAX_CASCADE:
            raise CascadeOverflow()
        (ev, n), rest = tasks[0], tasks[1:]
        i, e = idx[n], ents[n]
        cur = est[i]
        if cur is None:
            if e.created and e.created["event"] == ev:
                c = e.created
                b2 = apply_res(bal, c["acq"], c["rel"], f"{n} 创建")
                if b2 is not None:
                    yield from cascade(est[:i] + (e.initial,) + est[i + 1:], b2, rest + tasks_for(c["emits"]), depth + 1)
            else:
                yield from cascade(est, bal, rest, depth + 1)
            return
        trs = e.by_se.get((cur, ev))
        if trs:
            for t in trs:
                b2 = apply_res(bal, t.acq, t.rel, t.id)
                if b2 is None:
                    continue
                yield from cascade(est[:i] + (t.to,) + est[i + 1:], b2, rest + tasks_for(t.emits), depth + 1)
            return
        if m.src.get(ev) != "actor" and (cur, ev) not in ign[n]:
            dup = e.created and e.created["event"] == ev
            record(("R17", n, cur, ev), "R17", "ERROR", f"{n}.{cur}+{ev}",
                   f"事件「{ev}」（{m.src.get(ev)}）{'重复到达' if dup else '在该状态下到达'}时，{n} 既没有处理流转也没有声明 ignores——"
                   "迟到/重复/乱序的事件会被悄悄吞掉或引发数据不一致")
        yield from cascade(est, bal, rest, depth + 1)

    root_events = [ev for ev, d in m.events.items() if m.src.get(ev) in ROOT_SOURCES and ev in subs]
    g0 = (tuple(None for _ in names), tuple(0 for _ in res_names))
    visited[g0] = None
    queue = collections.deque([g0])
    edges = collections.defaultdict(set)
    truncated = False

    def complete(g):
        c = [i for i, s in enumerate(g[0]) if s is not None]
        return bool(c) and all(g[0][i] in ents[names[i]].terminal for i in c)

    def holds(cond, g):
        for en, ss in cond.items():
            s = g[0][idx[en]]
            if not ((s is None and "@none" in ss) or (s is not None and s in ss)):
                return False
        return True

    while queue:
        g = queue.popleft()
        for iv in m.invs:
            if holds(iv.when, g) and not holds(iv.then, g):
                record(("R14", iv.id), "R14", "ERROR", iv.id,
                       f"不变量被违反：{iv.desc or iv.id}", g=g, ev="")
        if complete(g) and any(g[1]):
            left = "、".join(f"{res_names[i]}={v}" for i, v in enumerate(g[1]) if v)
            record(("R16", "end", g[1]), "R16", "ERROR", res_names[[i for i, v in enumerate(g[1]) if v][0]],
                   f"业务走到终态后资源仍未释放（{left}）——资源泄漏", g=g, ev="")
        for ev in root_events:
            ctx["g"], ctx["ev"] = g, ev
            try:
                outs = list(cascade(g[0], g[1], tasks_for([ev]), 0))
            except CascadeOverflow:
                record(("R13", ev), "R13", "ERROR", "cascade",
                       f"事件「{ev}」触发的联动超过 {MAX_CASCADE} 层，疑似无限联动")
                continue
            for est2, bal2 in outs:
                g2 = (est2, bal2)
                if g2 == g:
                    continue
                edges[g].add(g2)
                if g2 not in visited:
                    if len(visited) >= max_states:
                        truncated = True
                        continue
                    visited[g2] = (g, ev)
                    queue.append(g2)

    # R15：每个可达状态都能走到"完结"（所有已创建实体都处于终态）
    done = {g for g in visited if complete(g)}
    radj = collections.defaultdict(set)
    for a, bs in edges.items():
        for b in bs:
            radj[b].add(a)
    ok, q = set(done), collections.deque(done)
    while q:
        x = q.popleft()
        for y in radj[x]:
            if y not in ok:
                ok.add(y)
                q.append(y)
    if truncated:
        add("R15", "WARN", "global", f"状态空间超过 {max_states}，R15 未能穷举（可用 --max-states 调大）")
    else:
        bad = [g for g in visited if g not in ok]
        for g in bad[:3]:
            record(("R15", g), "R15", "ERROR", "global",
                   "存在可达的业务状态，从这里无论怎么操作都无法让所有实体走到终态（业务卡死/活锁）", g=g, ev="")
        if len(bad) > 3:
            add("R15", "ERROR", "global", f"……另有 {len(bad) - 3} 个同类卡死状态未列出")
    for rule, level, target, msg, trace in found.values():
        add(rule, level, target, msg, trace)
    return {"states": len(visited), "complete": len(done), "truncated": truncated,
            "root_events": len(root_events)}


# ----------------------------------------------------------------- 汇总
def run_checks(spec, max_states):
    findings = []

    def add(rule, level, target, msg, trace=""):
        findings.append(Finding(rule, level, target, msg, trace))

    m = build_model(spec, add)
    stats = None
    if not m.fatal:
        static_checks(m, add)
        blockers = [f for f in findings if f.rule in ("R1", "R2", "R3", "R4", "R6") and f.level == "ERROR"]
        if blockers:
            add("R14", "INFO", "model", "存在结构性错误(R1/R2/R3/R4/R6)，已跳过状态空间检查(R13-R17动态部分)；请先修复上面的问题")
        else:
            stats = model_check(m, add, max_states)
    used = set()
    for f in findings:
        if f.rule == "R6":
            continue
        for w in m.waivers:
            if S(w.get("rule")) == f.rule and S(w.get("target")) == f.target and (S(w.get("reason")) or "").strip():
                f.waived, f.reason = True, S(w.get("reason"))
                used.add((S(w.get("rule")), S(w.get("target"))))
    for w in m.waivers:
        k = (S(w.get("rule")), S(w.get("target")))
        if k not in used and (S(w.get("reason")) or "").strip():
            add("W", "WARN", f"{k[0]}:{k[1]}", "豁免没有命中任何问题（目标写错或问题已不存在），请删除")
    return m, findings, stats


def summarize(findings):
    live = [f for f in findings if not f.waived]
    return {"errors": sum(f.level == "ERROR" for f in live), "warns": sum(f.level == "WARN" for f in live),
             "infos": sum(f.level == "INFO" for f in live), "waived": sum(f.waived for f in findings)}


def model_complete(stats):
    return (isinstance(stats, dict) and stats.get("truncated") is False
            and type(stats.get("states")) is int and stats["states"] > 0)


def strict_passed(summary, stats):
    return (isinstance(summary, dict)
            and type(summary.get("errors")) is int and summary["errors"] == 0
            and type(summary.get("warns")) is int and summary["warns"] == 0
            and model_complete(stats))


# ----------------------------------------------------------------- 报告
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


# ----------------------------------------------------------------- 命令行
def main(argv=None):
    ap = argparse.ArgumentParser(description="业务闭环检查器（arc:plan model）")
    ap.add_argument("cmd", choices=["check", "gate", "render"])
    ap.add_argument("spec")
    ap.add_argument("--report", help="报告输出路径（默认与 spec 同目录 04-closure-report.md）")
    ap.add_argument("--status", help="状态文件路径（默认与 spec 同目录 .closure-status.json）")
    ap.add_argument("--max-states", type=int, default=300000)
    ap.add_argument("--allow-warn", action="store_true", help="仅放宽 check 的诊断退出码；WARN 仍阻断 gate")
    ap.add_argument("--json", action="store_true", help="同时向 stdout 打印 JSON 结果")
    a = ap.parse_args(argv)
    if a.max_states < 1:
        ap.error("--max-states 必须大于 0")
    d = os.path.dirname(os.path.abspath(a.spec))
    status_path = a.status or os.path.join(d, ".closure-status.json")
    with open(__file__, "rb") as checker:
        checker_sha = hashlib.sha256(checker.read()).hexdigest()
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
            print("⛔ 模型闸门关闭：需要零未豁免 ERROR/WARN 和完整的状态空间检查；诊断放宽、豁免或缺失统计不能替代证据。")
            return 1
        print(f"✅ 模型闸门 PASS（{st.get('checked_at')}）。按已有明确授权与范围衔接 arc:code；未决业务假设、豁免或范围决策仍需确认。运行时验证另交 arc:verify。")
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
        json.dump({"status_version": STATUS_VERSION, "passed": passed,
                   "spec_sha256": sha, "checker_sha256": checker_sha,
                   "check_options": {"max_states": a.max_states, "allow_warn": a.allow_warn},
                   "summary": summ, "stats": stats,
                   "checked_at": datetime.datetime.now().isoformat(timespec="seconds"),
                   "findings": [x.to_dict() for x in findings if not x.waived and x.level != "INFO"]},
                  f, ensure_ascii=False, indent=2)
    live = [x for x in findings if not x.waived]
    order = {"ERROR": 0, "WARN": 1, "INFO": 2}
    for x in sorted(live, key=lambda x: (order[x.level], x.rule)):
        print(f"[{x.level}] {x.rule} {x.target}: {x.msg}")
        if x.trace:
            print(f"        反例：{x.trace}")
    print(f"\n{'✅ 通过' if passed else '❌ 未通过'}  ERROR={summ['errors']} WARN={summ['warns']} "
          f"INFO={summ['infos']} 已豁免={summ['waived']}"
          + (f"  状态空间={stats['states']}" if stats else ""))
    print(f"报告：{report_path}")
    diagnostic_ok = a.allow_warn and summ["errors"] == 0 and model_complete(stats)
    if diagnostic_ok and not passed:
        print("--allow-warn 仅令诊断退出码为 0；模型闸门仍为 FAIL。")
    if a.json:
        print(json.dumps({"passed": passed, "summary": summ}, ensure_ascii=False))
    return 0 if passed or diagnostic_ok else 1


if __name__ == "__main__":
    sys.exit(main())
