import { useMemo, useState } from "react";
import { Background, Controls, MarkerType, Position, ReactFlow } from "@xyflow/react";
import type { Edge, Node } from "@xyflow/react";
import { analyzeDirected, layoutDirected, neighborhood } from "./layout";
import type { CanvasEdge, CanvasGraph, CanvasNode } from "./graph-model";
import { canvasSelectionAnchors } from "./graph-focus";
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
  const cyclic = new Set(analysis.cycles.flat());
  const selectedIDs = canvasSelectionAnchors(graph, selection);
  const focus = selection.focus;
  let allowed: Set<string> | undefined;
  if (focus === "critical") allowed = analysis.criticalNodes;
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
    layoutDirected(displayNodes, displayEdges).map((node) => [node.id, node]),
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
              {node.kind === "preparation"
                ? "准备动作"
                : node.kind === "cycle"
                  ? "可展开的状态环"
                  : node.kind === "expected_result"
                    ? "预期约束 · 后态未声明"
                    : node.kind === "step"
                      ? `第 ${node.step} 步`
                      : "业务状态"}
              {cyclic.has(node.id) ? " · 循环" : ""}
            </span>
            <strong>{node.name}</strong>
            <span className="node-detail">{node.detail}</span>
            {node.status && (
              <span className={`node-verdict status-${node.status.toLowerCase()}`}>
                {node.status}
              </span>
            )}
          </>
        ),
      },
      className: `business-node${selected ? " selected" : ""}${node.members ? " cycle-node" : ""}${analysis.criticalNodes.has(node.id) && selection.canvas === "dependencies" ? " critical-node" : ""}`,
      style: { width: position.width, minHeight: position.height },
    };
  });
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
        analysis.criticalEdges.has(edge.id) && selection.canvas === "dependencies"
          ? "#2563eb"
          : "#94a3b8",
      strokeWidth:
        analysis.criticalEdges.has(edge.id) && selection.canvas === "dependencies" ? 2.4 : 1.5,
      strokeDasharray: edge.legacy ? "6 4" : undefined,
    },
    labelStyle: { fill: "#334155", fontSize: 11 },
    labelBgStyle: { fill: "#fff", fillOpacity: 0.96 },
  }));
  return (
    <>
      <p className="canvas-caption">{graph.caption}</p>
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
            key={`${selection.canvas}/${selection.flow}/${selection.run}/${selection.focus}/${foldCycles}`}
            nodes={nodes}
            edges={edges}
            fitView
            fitViewOptions={{ padding: 0.18, maxZoom: 1 }}
            minZoom={0.25}
            maxZoom={1.8}
            nodesDraggable={false}
            nodesConnectable={false}
            onNodeClick={(_, item) => {
              const node = item.data.source as CanvasNode & { members?: string[] };
              if (node.members) {
                setFoldCycles(false);
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
    </>
  );
}
