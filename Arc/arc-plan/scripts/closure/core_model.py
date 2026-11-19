"""Closure model definitions and spec loader."""
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
