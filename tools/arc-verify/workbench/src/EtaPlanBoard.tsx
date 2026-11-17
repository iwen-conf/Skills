import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { etaReadPlan, proofState } from "./eta-read-plan";
import type { CanvasEdge, CanvasGraph, CanvasNode } from "./graph-model";
import { lineageKindLabel, lineageStatusLabel } from "./lineage";
import "./eta-board.css";

/**
 * Visual translation of ETA's WidePlan / PhonePlan, adapted to Arc's immutable
 * business evidence. Links are strictly read-only: changing a dependency or
 * executing a business operation must be done through the CLI and verified.
 * ETA task-plan-view.tsx (MIT), compared at 0xfe-project/eta@aad7db8.
 */
interface Group { id: string; label: string; members: string[] }
interface Box { left: number; top: number; right: number; bottom: number }

const curve = (x1: number, y1: number, x2: number, y2: number) => {
  const middle = (x1 + x2) / 2;
  return "M" + x1 + " " + y1 +
    "C" + middle + " " + y1 + " " + middle + " " + y2 + " " + (x2 - 5) + " " + y2;
};
const arrow = (x: number, y: number) =>
  "M" + (x - 6) + " " + (y - 3.5) + "L" + x + " " + y + "L" + (x - 6) + " " + (y + 3.5) + "z";
const labelState = (node: CanvasNode) =>
  node.lineage
    ? lineageStatusLabel(node.lineage, node.reading)
    : node.status === "NOT_OBSERVED" || !node.status
      ? "未观察"
      : node.status;
const kindOf = (node: CanvasNode) =>
  node.lineage
    ? lineageKindLabel[node.lineage.kind]
    : lineageKindLabel[node.kind as keyof typeof lineageKindLabel] ?? node.kind;
const circle = { OBSERVED:"✓", UNOBSERVED:"○", MISMATCH:"!", BLOCKED:"×" } as const;

function GroupPlan({
  group, graph, selected, onSelectNode,
}: {
  group: Group;
  graph: CanvasGraph;
  selected: ReadonlySet<string>;
  onSelectNode: (node: CanvasNode) => void;
}) {
  const plan = etaReadPlan(graph);
  const critical = new Set(plan.critical);
  const byId = new Map(graph.nodes.map((node) => [node.id, node]));
  const memberIds = new Set(group.members.filter((id) => byId.has(id)));
  const members = group.members.filter((id) => memberIds.has(id)).map((id) => byId.get(id)!);
  const columns = new Map<number, CanvasNode[]>();
  for (const node of members) {
    const level = plan.levels.get(node.id) ?? 0;
    const column = columns.get(level) ?? [];
    column.push(node);
    columns.set(level, column);
  }
  const steps = [...columns.keys()].sort((a, b) => a - b);
  const edges = graph.edges.filter((edge) => memberIds.has(edge.from) && memberIds.has(edge.to));
  const crossEdges = graph.edges.filter((edge) =>
    memberIds.has(edge.from) !== memberIds.has(edge.to) &&
    (memberIds.has(edge.from) || memberIds.has(edge.to)));
  const [hovered, setHovered] = useState<string>();
  const boxRef = useRef<HTMLDivElement>(null);
  const nodeRefs = useRef(new Map<string, HTMLElement>());
  const [boxes, setBoxes] = useState<Map<string, Box>>(() => new Map());
  const register = useCallback((id: string, element: HTMLElement | null) => {
    if (element) nodeRefs.current.set(id, element);
    else nodeRefs.current.delete(id);
  }, []);
  const measure = useCallback(() => {
    const origin = boxRef.current?.getBoundingClientRect();
    if (!origin) return;
    const next = new Map<string, Box>();
    for (const [id, element] of nodeRefs.current) {
      const r = element.getBoundingClientRect();
      next.set(id, {
        left: r.left - origin.left, top: r.top - origin.top,
        right: r.right - origin.left, bottom: r.bottom - origin.top,
      });
    }
    setBoxes((prior) => {
      if (prior.size !== next.size) return next;
      for (const [id, item] of next) {
        const before = prior.get(id);
        if (!before ||
          before.left !== item.left || before.top !== item.top ||
          before.right !== item.right || before.bottom !== item.bottom) return next;
      }
      return prior;
    });
  }, []);
  const layoutKey = members.map((node) => node.id + ":" + node.name + ":" +
    (plan.levels.get(node.id) ?? 0)).join("|") +
    edges.map((edge) => edge.from + ">" + edge.to).join("|");
  useLayoutEffect(measure, [measure, layoutKey]);
  useEffect(() => {
    const root = boxRef.current;
    if (!root || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(root);
    return () => observer.disconnect();
  }, [measure]);

  const select = (id: string) => {
    const node = byId.get(id);
    if (node) onSelectNode(node);
  };
  const card = (node: CanvasNode, mobile: boolean) => {
    const state = proofState(node);
    return (
      <div
        key={node.id}
        data-plan-node={node.id}
        data-eta-status={state}
        data-critical={critical.has(node.id) ? "true" : undefined}
        className={"eta-board-card eta-board-" + state.toLowerCase() +
          (selected.has(node.id) ? " eta-board-selected" : "")}
        ref={(element) => { if (!mobile) register(node.id, element); }}
      >
        <button
          type="button"
          aria-label={"查看依赖节点：" + node.name}
          aria-pressed={selected.has(node.id)}
          title={node.detail}
          onClick={() => select(node.id)}
          className="eta-board-node-button"
        >
          <span aria-hidden="true" className={"eta-board-status-symbol eta-board-symbol-" +
            state.toLowerCase()}>{circle[state]}</span>
          <strong className="eta-board-title">{node.name}</strong>
        </button>
        <div className="eta-board-meta">
          <code>{kindOf(node)} · {node.id}</code>
          <small>{labelState(node)}</small>
        </div>
      </div>
    );
  };
  const observed = members.filter((node) => proofState(node) === "OBSERVED").length;
  const outstanding = members.length - observed;
  return (
    <section
      className="eta-board-group"
      aria-label={group.label + " ETA计划"}
      data-eta-group={group.id}
    >
      <div className="eta-board-group-heading">
        <span className="eta-board-group-dot" aria-hidden="true">◈</span>
        <strong>{group.label}</strong>
        <span>{outstanding ? "尚需观察 " + outstanding + " 项" : "全部已有观察"}</span>
        <small>{observed}/{members.length} 个条件符合观察</small>
      </div>
      <div className="eta-board-wide" aria-label={group.label + "桌面依赖计划"}>
        <div ref={boxRef} className="eta-board-wide-inner">
          <svg
            className="eta-board-wires"
            role="group"
            aria-label={group.label + "依赖连线"}
          >
            {edges.map((edge: CanvasEdge) => {
              const from = boxes.get(edge.from);
              const to = boxes.get(edge.to);
              const criticalEdge = critical.has(edge.from) && critical.has(edge.to) &&
                plan.critical.some((id, index) =>
                  id === edge.from && plan.critical[index + 1] === edge.to);
              const alternative = !!edge.alternative || edge.kind === "candidate_provider";
              const coordinates = from && to
                ? { x1:from.right, y1:(from.top + from.bottom) / 2,
                    x2:to.left, y2:(to.top + to.bottom) / 2 }
                : undefined;
              const focus = hovered === edge.id;
              return (
                <g
                  key={edge.id}
                  data-eta-edge={edge.id}
                  data-critical={criticalEdge ? "true" : undefined}
                  data-alternative={alternative ? "true" : undefined}
                  tabIndex={0}
                  role="button"
                  aria-label={"查看依赖：" + (byId.get(edge.from)?.name ?? edge.from) +
                    " → " + (byId.get(edge.to)?.name ?? edge.to)}
                  className={"eta-board-wire " +
                    (focus || criticalEdge ? "eta-board-wire-active" : "") +
                    (alternative ? " eta-board-wire-alternative" : "")}
                  onFocus={() => setHovered(edge.id)}
                  onBlur={() => setHovered(undefined)}
                  onPointerEnter={() => setHovered(edge.id)}
                  onPointerLeave={() => setHovered(undefined)}
                  onClick={() => select(edge.to)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      select(edge.to);
                    }
                  }}
                >
                  <title>{edge.label || (alternative ? "候选提供关系" : "必要前置依赖")}</title>
                  {coordinates && (
                    <>
                      <path
                        d={curve(coordinates.x1,coordinates.y1,coordinates.x2,coordinates.y2)}
                        fill="none" stroke="transparent" strokeWidth={12}/>
                      <path
                        d={curve(coordinates.x1,coordinates.y1,coordinates.x2,coordinates.y2)}
                        fill="none" stroke="currentColor"
                        strokeWidth={criticalEdge ? 1.7 : 1.2}
                        strokeDasharray={alternative ? "4 3" : undefined}/>
                      <path d={arrow(coordinates.x2,coordinates.y2)} fill="currentColor"/>
                    </>
                  )}
                </g>
              );
            })}
          </svg>
          <ol className="eta-board-columns">
            {steps.map((level) => (
              <li key={level} data-eta-step={level + 1} className="eta-board-step">
                <h3>
                  <span>步骤 {level + 1}</span>
                  <span>{columns.get(level)!.filter((node) =>
                    proofState(node) !== "OBSERVED").length
                    ? "有未观察的依赖"
                    : "已有观察"}
                  </span>
                </h3>
                <div className="eta-board-step-cards">
                  {columns.get(level)!.map((node) => card(node, false))}
                </div>
              </li>
            ))}
          </ol>
        </div>
      </div>
      <ol className="eta-board-phone" aria-label={group.label + "手机依赖计划"}>
        {steps.map((level) => (
          <li key={level} className="eta-board-phone-step" data-eta-phone-step={level + 1}>
            <h3>步骤 {level + 1} · {columns.get(level)!.length} 个条件</h3>
            <div className="eta-board-phone-cards">
              {columns.get(level)!.map((node) => card(node, true))}
            </div>
          </li>
        ))}
      </ol>
      {!!crossEdges.length && (
        <details className="eta-board-cross-group">
          <summary>跨业务组依赖 · {crossEdges.length} 条</summary>
          <ol>
            {crossEdges.map((edge) => (
              <li key={edge.id}>
                <button type="button" onClick={() =>
                  select(memberIds.has(edge.from) ? edge.to : edge.from)}>
                  {(byId.get(edge.from)?.name ?? edge.from) + " → " +
                    (byId.get(edge.to)?.name ?? edge.to)}
                </button>
                {edge.alternative ? " · 候选/可替代关系" : " · 必要前置"}
              </li>
            ))}
          </ol>
        </details>
      )}
    </section>
  );
}

export default function EtaPlanBoard({
  graph, groups, selected, onSelectNode,
}: {
  graph: CanvasGraph;
  groups: Group[];
  selected: ReadonlySet<string>;
  onSelectNode: (node: CanvasNode) => void;
}) {
  if (graph.nodes.length > 800) {
    return (
      <div role="status" className="eta-board-limit">
        当前范围有 {graph.nodes.length} 个节点。为避免产生不可操作的超大画布，
        请通过搜索缩小范围，或切换“完整分层目录”。节点与关系不会被静默丢弃。
      </div>
    );
  }
  if (!groups.length) return <p className="eta-board-empty">当前范围没有依赖节点。</p>;
  return (
    <div className="eta-board" aria-label="ETA式业务依赖计划">
      <p className="eta-board-readonly">
        ETA 布局 · 只读业务证据。实线表示必要依赖，虚线表示候选/OR 来源。
        同一步骤不代表可以并行；观察符合不代表已完成业务操作。
      </p>
      {groups.map((group) => (
        <GroupPlan
          key={group.id}
          group={group}
          graph={graph}
          selected={selected}
          onSelectNode={onSelectNode}
        />
      ))}
    </div>
  );
}
