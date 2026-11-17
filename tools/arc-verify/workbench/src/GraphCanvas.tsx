import { useMemo, useState } from "react";
import { Background, Controls, MarkerType, Position, ReactFlow } from "@xyflow/react";
import type { Edge, Node } from "@xyflow/react";
import { analyzeDirected, layoutDirected, neighborhood } from "./layout";
import type { CanvasEdge, CanvasGraph, CanvasNode } from "./graph-model";
import { canvasSelectionAnchors } from "./graph-focus";
import { lineageKindLabel, lineageStatusLabel } from "./lineage";
import type { Selection, SelectionAction } from "./selection";

export default function GraphCanvas({
  graph,
  selection,
  dispatch,
}: {
  graph: CanvasGraph;
  selection: Selection;
  dispatch: (action: SelectionAction) => void;
}) {
  const [foldCycles, setFoldCycles] = useState(true);
  const analysis = useMemo(() => analyzeDirected(graph.nodes, graph.edges), [graph]);
  const critical = useMemo(
    () =>
      graph.lineage
        ? analyzeDirected(
            graph.nodes,
            graph.edges.filter((edge) => !edge.alternative),
          )
        : analysis,
    [graph, analysis],
  );
  const cyclic = new Set(analysis.cycles.flat());
  const selectedIDs = canvasSelectionAnchors(graph, selection);
  const focus = selection.focus;
  let allowed: Set<string> | undefined;
  if (focus === "critical") allowed = critical.criticalNodes;
  else if (focus !== "all")
    allowed = new Set([...selectedIDs].flatMap((id) => [...neighborhood(graph.edges, id, focus)]));
  const scopeNodes = graph.nodes.filter((node) => !allowed || allowed.has(node.id));
  const scopeIDs = new Set(scopeNodes.map((node) => node.id));
  const scopeEdges = graph.edges.filter((edge) => scopeIDs.has(edge.from) && scopeIDs.has(edge.to));
  const aliases = new Map<string, string>();
  const displayNodes: (CanvasNode & { members?: string[] })[] = [];
  if (selection.canvas === "states" && foldCycles) {
    for (const members of analysis.cycles) {
      const visible = members.filter((id) => scopeIDs.has(id));
      if (!visible.length) continue;
      const id = `cycle:${members.join("|")}`;
      visible.forEach((member) => aliases.set(member, id));
      displayNodes.push({
        id,
        subject: visible[0],
        name: `循环状态 · ${visible.length}`,
        kind: "cycle",
        detail: visible
          .map((member) => graph.nodes.find((node) => node.id === member)?.name ?? member)
          .join(" ↔ "),
        members: visible,
      });
    }
  }
  displayNodes.push(...scopeNodes.filter((node) => !aliases.has(node.id)));
  const displayEdges: CanvasEdge[] = scopeEdges
    .map((edge) => ({
      ...edge,
      from: aliases.get(edge.from) ?? edge.from,
      to: aliases.get(edge.to) ?? edge.to,
    }))
    .filter(
      (edge) =>
        edge.from !== edge.to ||
        !aliases.has(scopeEdges.find((item) => item.id === edge.id)?.from ?? ""),
    );
  const positions = new Map(
    layoutDirected(
      displayNodes,
      displayEdges,
      graph.lineage ? { width: 264, height: 148 } : undefined,
    ).map((node) => [node.id, node]),
  );
  const nodes: Node[] = displayNodes.map((node) => {
    const position = positions.get(node.id)!;
    const selected = node.members?.some((id) => selectedIDs.has(id)) || selectedIDs.has(node.id);
    return {
      id: node.id,
      position: { x: position.x, y: position.y },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        source: node,
        label: (
          <>
            <span className="graph-kind">
              {node.lineage
                ? lineageKindLabel[node.lineage.kind]
                : node.kind === "preparation"
                  ? "准备动作"
                  : node.kind === "cycle"
                    ? "可展开的状态环"
                    : node.kind === "expected_result"
                      ? "预期约束 · 后态未声明"
                      : node.kind === "preparation_step"
                        ? "前态准备"
                        : node.kind === "step"
                          ? `第 ${node.step} 步`
                          : "业务状态"}
              {cyclic.has(node.id) ? " · 循环" : ""}
            </span>
            <strong>{node.name}</strong>
            <span className="node-detail">{node.detail}</span>
            {node.status && (
              <span
                className={
                  node.lineage
                    ? `lineage-status lineage-status-${node.status.toLowerCase()}`
                    : `node-verdict status-${node.status.toLowerCase()}`
                }
              >
                {node.lineage ? lineageStatusLabel(node.lineage, node.reading) : node.status}
              </span>
            )}
          </>
        ),
      },
      className: `business-node${node.lineage ? ` lineage-node lineage-node-${node.kind}` : ""}${selected ? " selected" : ""}${node.members ? " cycle-node" : ""}${critical.criticalNodes.has(node.id) && selection.canvas === "dependencies" ? " critical-node" : ""}`,
      style: {
        width: position.width,
        minHeight: position.height,
        ...(node.lineage ? { height: position.height } : {}),
      },
    };
  });
  const levels = [...new Set(scopeNodes.map((node) => analysis.levels.get(node.id) ?? 0))].sort(
    (a, b) => a - b,
  );
  const dense = Boolean(graph.lineage && scopeNodes.length > 16);
  const viewportIDs = new Set([
    ...selectedIDs,
    ...scopeEdges.filter((edge) => selectedIDs.has(edge.to)).map((edge) => edge.from),
  ]);
  if (graph.lineage) {
    for (const level of levels) {
      const members = scopeNodes.filter((node) => analysis.levels.get(node.id) === level);
      const member = positions.get(members[0].id);
      if (!member) continue;
      nodes.push({
        id: `arc-lineage-level:${level}`,
        position: { x: member.x, y: -28 },
        data: {
          label: (
            <>
              <strong>依赖层 {level + 1}</strong>
              <span>{members.length} 个节点</span>
            </>
          ),
        },
        className: "lineage-level-heading",
        selectable: false,
        focusable: false,
        style: { width: member.width },
      });
    }
  }
  const edges: Edge[] = displayEdges.map((edge) => ({
    id: edge.id,
    source: edge.from,
    target: edge.to,
    type: "smoothstep",
    label: edge.label.length > 88 ? `${edge.label.slice(0, 85)}…` : edge.label,
    data: { source: edge },
    markerEnd: { type: MarkerType.ArrowClosed },
    style: {
      stroke:
        critical.criticalEdges.has(edge.id) && selection.canvas === "dependencies"
          ? "#2563eb"
          : edge.alternative
            ? "#aa8542"
            : "#94a3b8",
      strokeWidth:
        critical.criticalEdges.has(edge.id) && selection.canvas === "dependencies" ? 2.4 : 1.5,
      strokeDasharray: edge.legacy || edge.alternative ? "6 4" : undefined,
    },
    labelStyle: { fill: "#334155", fontSize: 11 },
    labelBgStyle: { fill: "#fff", fillOpacity: 0.96 },
  }));
  return (
    <>
      <p className="canvas-caption">{graph.caption}</p>
      {graph.lineage?.graph && (
        <div className="lineage-legend" aria-label="系谱关系图例">
          <span>
            <i className="lineage-required-line" />
            前置要求 / 逻辑关系
          </span>
          <span>
            <i className="lineage-alternative-line" />
            候选来源 / OR 可替代路径
          </span>
          <span>
            <i className="lineage-critical-line" />
            声明关系最长链
          </span>
          <small>
            层级是关系深度。候选边不计入关键链；同层不保证动作可并行。
            {dense ? "当前先聚焦所选节点附近，可拖动画布或使用下方完整分层目录。" : ""}
          </small>
        </div>
      )}
      {analysis.cycles.length > 0 && (
        <div className={selection.canvas === "dependencies" ? "notice error-notice" : "notice"}>
          {selection.canvas === "dependencies"
            ? `准备依赖包含 ${analysis.cycles.length} 个环，不能形成可执行 DAG。`
            : `此状态图包含 ${analysis.cycles.length} 个循环组件，允许重试与恢复。`}
          {selection.canvas === "states" && (
            <button type="button" onClick={() => setFoldCycles(!foldCycles)}>
              {foldCycles ? "展开循环" : "折叠循环"}
            </button>
          )}
        </div>
      )}
      {nodes.length ? (
        <div
          className="canvas"
          aria-label={
            selection.canvas === "dependencies"
              ? "准备依赖画布"
              : selection.canvas === "states"
                ? "业务状态画布"
                : "运行步骤画布"
          }
        >
          <ReactFlow
            key={`${selection.canvas}/${selection.flow}/${selection.branch}/${selection.run}/${selection.focus}/${selection.focus === "all" && !dense ? "" : selection.node}/${foldCycles}`}
            nodes={nodes}
            edges={edges}
            fitView
            fitViewOptions={{
              padding: 0.18,
              maxZoom: 1,
              minZoom: graph.lineage ? 0.65 : 0.25,
              ...(dense && viewportIDs.size
                ? { nodes: [...viewportIDs].map((id) => ({ id })) }
                : {}),
            }}
            minZoom={0.25}
            maxZoom={1.8}
            nodesDraggable={false}
            nodesConnectable={false}
            onNodeClick={(_, item) => {
              const node = item.data.source as CanvasNode & { members?: string[] };
              if (!node) return;
              if (node.members) {
                setFoldCycles(false);
                return;
              }
              if (node.lineage) {
                dispatch({ type: "lineage", value: node.id });
                return;
              }
              dispatch(
                node.step
                  ? { type: "step", value: node.step }
                  : node.branch
                    ? { type: "branch", value: node.branch }
                    : {
                        type: "node",
                        value: selection.canvas === "dependencies" ? node.id : node.subject,
                      },
              );
            }}
            onEdgeClick={(_, item) => {
              const edge = item.data?.source as CanvasEdge;
              if (graph.lineage) {
                dispatch({ type: "lineage", value: edge.to });
                return;
              }
              if (edge.branch) dispatch({ type: "branch", value: edge.branch });
              else if (edge.action) dispatch({ type: "node", value: edge.action });
            }}
          >
            <Background gap={24} size={1} />
            <Controls showInteractive={false} />
          </ReactFlow>
        </div>
      ) : (
        <div className="canvas-empty">
          <strong>还没有可显示的{selection.canvas === "run" ? "运行" : "关系"}</strong>
          <p>{graph.empty}</p>
        </div>
      )}
      <div className="canvas-summary">
        <span>
          {scopeNodes.length} / {graph.nodes.length} 节点 · {scopeEdges.length} 条关系
        </span>
        <span>{selection.focus === "all" ? "完整当前范围" : "局部视图 · 未显示节点不计通过"}</span>
      </div>
      {graph.lineage?.graph && (
        <details className="lineage-directory" open={dense}>
          <summary>依赖系谱分层目录 · {scopeNodes.length} 个节点</summary>
          <ol aria-label="依赖系谱分层目录">
            {levels.map((level) => (
              <li key={level}>
                <h3>依赖层 {level + 1}</h3>
                {(["preparation", "state", "condition", "logic", "target"] as const).map((kind) => {
                  const members = scopeNodes.filter(
                    (node) => analysis.levels.get(node.id) === level && node.lineage?.kind === kind,
                  );
                  if (!members.length) return null;
                  return (
                    <section key={kind} aria-label={lineageKindLabel[kind]}>
                      <h4>
                        {lineageKindLabel[kind]} · {members.length}
                      </h4>
                      <ul>
                        {members.map((node) => (
                          <li key={node.id}>
                            <button
                              type="button"
                              aria-pressed={selectedIDs.has(node.id)}
                              onClick={() => dispatch({ type: "lineage", value: node.id })}
                            >
                              <strong>{node.name}</strong>
                              <span>{node.detail}</span>
                              <small
                                className={`lineage-status lineage-status-${node.status?.toLowerCase()}`}
                              >
                                {lineageStatusLabel(node.lineage!, node.reading)}
                              </small>
                            </button>
                          </li>
                        ))}
                      </ul>
                    </section>
                  );
                })}
              </li>
            ))}
          </ol>
        </details>
      )}
    </>
  );
}
