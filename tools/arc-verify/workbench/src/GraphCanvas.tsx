import { useMemo, useState } from "react";
import { Background, BaseEdge, Controls, MarkerType, Position, ReactFlow } from "@xyflow/react";
import type { Edge, EdgeProps, Node } from "@xyflow/react";
import { analyzeDirected, layoutDirected, neighborhood } from "./layout";
import type { CanvasEdge, CanvasGraph, CanvasNode } from "./graph-model";
import { canvasSelectionAnchors } from "./graph-focus";
import { lineageKindLabel, lineageStatusLabel } from "./lineage";
import type { Selection, SelectionAction } from "./selection";
import DependencyPlan from "./DependencyPlan";

function ActualEdge(props: EdgeProps) {
  const { sourceX, sourceY, targetX, targetY } = props;
  const lane = typeof props.data?.lane === "number" ? props.data.lane : 0;
  const lift = 52 + lane * 24;
  const centerX = (sourceX + targetX) / 2;
  const centerY = (sourceY + targetY) / 2;
  const loop = props.source === props.target;
  const bend = Math.max(50, Math.abs(targetX - sourceX) * 0.3);
  const loopY = Math.min(sourceY, targetY) - 90 - lane * 24;
  const path = loop
    ? `M ${sourceX},${sourceY} C ${sourceX + lift},${sourceY} ${sourceX + lift},${loopY} ${centerX},${loopY} C ${targetX - lift},${loopY} ${targetX - lift},${targetY} ${targetX},${targetY}`
    : `M ${sourceX},${sourceY} C ${sourceX + bend},${sourceY - lift} ${targetX - bend},${targetY - lift} ${targetX},${targetY}`;
  return (
    <BaseEdge
      id={props.id}
      path={path}
      markerEnd={props.markerEnd}
      style={props.style}
      label={props.label}
      labelX={centerX}
      labelY={loop ? loopY : centerY - lift * 0.75}
      labelStyle={props.labelStyle}
      labelBgStyle={props.labelBgStyle}
      interactionWidth={24}
    />
  );
}

const edgeTypes = { actual: ActualEdge };

export default function GraphCanvas({
  graph,
  selection,
  dispatch,
  onSelectNode,
  onSelectEdge,
  blockerNodes,
}: {
  graph: CanvasGraph;
  selection: Selection;
  dispatch: (action: SelectionAction) => void;
  onSelectNode?: (node: CanvasNode) => void;
  onSelectEdge?: (edge: CanvasEdge) => void;
  blockerNodes?: readonly string[];
}) {
  const [cycleMode, setCycleMode] = useState<"auto" | "fold" | "expand">("auto");
  const [dependencyView, setDependencyView] = useState<"plan" | "graph" | null>(null);
  const dependencyGraph = selection.canvas === "dependencies";
  // Prefer an actual genealogy for a focused branch; large full-model
  // maps begin with a structured directory to prevent unreadable spaghetti.
  const effectiveDependencyView = dependencyView ?? (graph.nodes.length > 64 ? "plan" : "graph");
  const drawDiagram = !dependencyGraph || effectiveDependencyView === "graph";
  const foldCycles =
    cycleMode === "fold" || (cycleMode === "auto" && !graph.stateOverlay?.occurrences.length);
  const analysis = useMemo(() => analyzeDirected(graph.nodes, graph.edges), [graph]);
  const critical = useMemo(
    () =>
      selection.canvas === "dependencies"
        ? analyzeDirected(
            graph.nodes,
            graph.edges.filter((edge) => !edge.alternative),
          )
        : analysis,
    [graph, analysis, selection.canvas],
  );
  const cyclic = new Set(analysis.cycles.flat());
  const selectedIDs = canvasSelectionAnchors(graph, selection);
  const focus = selection.focus;
  let allowed: Set<string> | undefined;
  if (focus === "blocked") allowed = new Set(blockerNodes ?? []);
  else if (focus === "critical") allowed = critical.criticalNodes;
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
        edge.actual ||
        edge.from !== edge.to ||
        !aliases.has(scopeEdges.find((item) => item.id === edge.id)?.from ?? ""),
    );
  const positions = new Map(
    (drawDiagram
      ? layoutDirected(
          displayNodes,
          displayEdges,
          dependencyGraph ? { width: 264, height: 148 } : undefined,
        )
      : []
    ).map((node) => [node.id, node]),
  );
  const nodes: Node[] = drawDiagram
    ? displayNodes.map((node) => {
        const position = positions.get(node.id)!;
        const selected =
          node.members?.some((id) => selectedIDs.has(id)) || selectedIDs.has(node.id);
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
                  {node.actual
                    ? "实际观察 · 模型外状态"
                    : node.lineage
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
          className: `business-node${node.lineage ? ` lineage-node lineage-node-${node.kind}` : ""}${node.actual ? ` actual-node actual-${node.actual.verdict.toLowerCase()}` : ""}${selected ? " selected" : ""}${node.members ? " cycle-node" : ""}${critical.criticalNodes.has(node.id) && selection.canvas === "dependencies" ? " critical-node" : ""}`,
          style: {
            width: position.width,
            minHeight: position.height,
            ...(node.lineage ? { height: position.height } : {}),
          },
        };
      })
    : [];
  const levels = [...new Set(scopeNodes.map((node) => analysis.levels.get(node.id) ?? 0))].sort(
    (a, b) => a - b,
  );
  const dense = dependencyGraph && scopeNodes.length > 16;
  const viewportIDs = new Set([
    ...selectedIDs,
    ...scopeEdges.filter((edge) => selectedIDs.has(edge.to)).map((edge) => edge.from),
  ]);
  if (dependencyGraph && drawDiagram) {
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
  const actualLanes = new Map<string, number>();
  const edges: Edge[] = drawDiagram
    ? displayEdges.map((edge) => {
        const pair = JSON.stringify([edge.from, edge.to]);
        const lane = actualLanes.get(pair) ?? 0;
        if (edge.actual) actualLanes.set(pair, lane + 1);
        const stroke = edge.actual
          ? edge.actual.verdict === "FAIL"
            ? "var(--fail)"
            : edge.actual.verdict === "BLOCKED"
              ? "var(--warning)"
              : "var(--accent)"
          : critical.criticalEdges.has(edge.id) && selection.canvas === "dependencies"
            ? "var(--accent)"
            : edge.alternative
              ? "var(--warning)"
              : "var(--muted)";
        return {
          id: edge.id,
          source: edge.from,
          target: edge.to,
          type: edge.actual ? "actual" : "smoothstep",
          // Repeating 'requires' on every edge creates noise. The inspector
          // retains the full typed relation; label only exceptional semantics.
          label: dependencyGraph && !edge.alternative && ["requires", "reads", "logical_operand"].includes(edge.kind)
            ? undefined : edge.label.length > 88 ? `${edge.label.slice(0, 85)}…` : edge.label,
          data: { source: edge, lane },
          markerEnd: { type: MarkerType.ArrowClosed, color: stroke },
          className: edge.actual
            ? `actual-edge actual-${edge.actual.verdict.toLowerCase()}`
            : edge.kind === "expected_outcome"
              ? "expected-edge"
              : undefined,
          zIndex: edge.actual ? 2 : 1,
          style: {
            stroke,
            strokeWidth: edge.actual
              ? 3
              : critical.criticalEdges.has(edge.id) && selection.canvas === "dependencies"
                ? 2.4
                : 1.5,
            strokeDasharray:
              edge.actual?.verdict === "BLOCKED" || edge.legacy || edge.alternative
                ? "6 4"
                : undefined,
          },
          labelStyle: { fill: edge.actual ? stroke : "var(--text)", fontSize: 11 },
          labelBgStyle: { fill: "var(--surface)", fillOpacity: 0.96 },
        };
      })
    : [];
  const selectNode = (node: CanvasNode & { members?: string[] }) => {
    if (node.members) {
      setCycleMode("expand");
      return;
    }
    if (onSelectNode) {
      onSelectNode(node);
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
  };
  const selectEdge = (edge: CanvasEdge) => {
    if (onSelectEdge) {
      onSelectEdge(edge);
      return;
    }
    if (graph.lineage) {
      dispatch({ type: "lineage", value: edge.to });
      return;
    }
    if (edge.branch) dispatch({ type: "branch", value: edge.branch });
    else if (edge.action) dispatch({ type: "node", value: edge.action });
  };
  const semanticStages = [
    { label: "前置数据", kinds: ["input_data"] },
    { label: "准备动作", kinds: ["preparation"] },
    { label: "约束与前态", kinds: ["state", "condition", "logic"] },
    { label: "业务操作", kinds: ["target"] },
    { label: "执行后核验", kinds: ["expected", "assertion", "invariant"] },
  ];
  return (
    <>
      {dependencyGraph && graph.lineage?.graph && (
        <div className="genealogy-route" aria-label="数据至结果的五层系谱">
          {semanticStages.map((stage) => (
            <div className="genealogy-route-stage" key={stage.label}>
              <strong>{graph.nodes.filter((node) => stage.kinds.includes(node.kind)).length}</strong>
              <span>{stage.label}</span>
            </div>
          ))}
        </div>
      )}
      <p className="canvas-caption">{graph.caption}</p>
      {selection.canvas === "states" && graph.stateOverlay && (
        <div className="state-path-legend" aria-label="预期与实际路径图例">
          <span>
            <i className="expected-path" /> Expected · 声明契约
          </span>
          <span>
            <i className="actual-path" /> Actual · 独立观察
          </span>
          <span>
            <i className="deviating-path" /> 实际偏离
          </span>
          <small>点击实际路径定位对应账号和动作。缺少后态或证据时保留诊断，不补写实际状态。</small>
        </div>
      )}
      {selection.canvas === "dependencies" && scopeNodes.length > 0 && (
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
            {dense
              ? "关系画布保持可读字号；选择节点可定位附近关系，也可使用业务分组查看完整范围。"
              : ""}
          </small>
        </div>
      )}
      {analysis.cycles.length > 0 && (
        <div className={selection.canvas === "dependencies" ? "notice error-notice" : "notice"}>
          {selection.canvas === "dependencies"
            ? `准备依赖包含 ${analysis.cycles.length} 个环，不能形成可执行 DAG。`
            : `此状态图包含 ${analysis.cycles.length} 个循环组件，允许重试与恢复。`}
          {selection.canvas === "states" && (
            <button type="button" onClick={() => setCycleMode(foldCycles ? "expand" : "fold")}>
              {foldCycles ? "展开循环" : "折叠循环"}
            </button>
          )}
        </div>
      )}
      {selection.canvas === "dependencies" && scopeNodes.length > 0 && (
        <div className="dependency-display-switch segmented" role="group" aria-label="依赖阅读方式">
          <button
            type="button"
            aria-pressed={effectiveDependencyView === "plan"}
            onClick={() => setDependencyView("plan")}
          >
            业务分组
          </button>
          <button
            type="button"
            aria-pressed={effectiveDependencyView === "graph"}
            onClick={() => setDependencyView("graph")}
          >
            关系画布
          </button>
        </div>
      )}
      {selection.canvas === "dependencies" && dependencyView === "plan" && scopeNodes.length ? (
        <DependencyPlan
          graph={{ ...graph, nodes: scopeNodes, edges: scopeEdges }}
          levels={analysis.levels}
          selected={selectedIDs}
          onSelectNode={selectNode}
        />
      ) : nodes.length ? (
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
            edgeTypes={edgeTypes}
            fitView
            fitViewOptions={{
              padding: 0.18,
              maxZoom: 1,
              minZoom: dependencyGraph ? 0.18 : 0.25,
              ...(dense && viewportIDs.size
                ? { nodes: [...viewportIDs].map((id) => ({ id })) }
                : {}),
            }}
            minZoom={0.15}
            maxZoom={1.8}
            nodesDraggable={false}
            nodesConnectable={false}
            onNodeClick={(_, item) => {
              const node = item.data.source as CanvasNode & { members?: string[] };
              if (!node) return;
              selectNode(node);
            }}
            onEdgeClick={(_, item) => {
              const edge = item.data?.source as CanvasEdge;
              if (edge) selectEdge(edge);
            }}
          >
            <Background gap={24} size={1} />
            <Controls showInteractive={false} />
          </ReactFlow>
        </div>
      ) : (
        <div className="canvas-empty">
          <strong>还没有可显示的{selection.canvas === "run" ? "运行" : "关系"}</strong>
          <p>
            {focus === "blocked"
              ? "当前观察没有可显示的阻断路径。未观察或证据问题请查看阻断说明。"
              : graph.empty}
          </p>
        </div>
      )}
      <div className="canvas-summary">
        <span>
          {scopeNodes.length} / {graph.nodes.length} 节点 · {scopeEdges.length} 条关系
        </span>
        <span>{selection.focus === "all" ? "完整当前范围" : "局部视图 · 未显示节点不计通过"}</span>
      </div>
      {graph.lineage?.graph && effectiveDependencyView === "graph" && (
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
