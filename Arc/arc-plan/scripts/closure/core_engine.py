"""Static rules R1-R20 and state-space model checker."""
from __future__ import annotations
import collections
from core_model import (
    RULES, VALID_SOURCES, ROOT_SOURCES, VALID_ACTOR_KINDS, VALID_KINDS,
    MAX_CASCADE, RES_CAP, STATUS_VERSION, S, as_list, as_map,
    Finding, Trans, Ent, Model, load_spec, build_model,
)

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
