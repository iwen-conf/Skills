import { useMemo, useState } from "react";
import { Background, Controls, MarkerType, Position, ReactFlow } from "@xyflow/react";
import type { Edge, Node } from "@xyflow/react";
import { layoutNodes } from "./layout";
import { drillRoot, evidenceRun, visibleNodes } from "./views";
import { SnapshotError, businessViews } from "./snapshot";
import type {
  BusinessView,
  Obligation,
  ProjectedNode,
  Projection,
  Snapshot,
  SnapshotEvidence,
  SnapshotRun,
  Verdict,
  Workspace,
} from "./types";
import "@xyflow/react/dist/style.css";
import "./App.css";

export type AppState =
  | { snapshot: Snapshot; error?: undefined }
  | { error: SnapshotError; snapshot?: undefined };

const viewInfo: Record<BusinessView, { name: string; workspace: Workspace }> = {
  overview: { name: "系统业务", workspace: "model" },
  capabilities: { name: "业务能力", workspace: "model" },
  boundaries: { name: "业务边界", workspace: "model" },
  flows: { name: "业务流程", workspace: "model" },
  verification: { name: "测试验证", workspace: "verify" },
  impact: { name: "业务影响", workspace: "analyze" },
};
const spaces: { id: Workspace; name: string }[] = [
  { id: "model", name: "业务建模" },
  { id: "verify", name: "业务验证" },
  { id: "analyze", name: "业务分析" },
];
const verdictLabels: Record<Verdict, string> = {
  PASS: "全部通过",
  PARTIAL: "部分验证",
  FAIL: "确认失败",
  NOT_RUN: "尚未验证",
  BLOCKED: "验证阻塞",
  STALE: "待复验",
  FLAKY: "结果不稳定",
  EXCLUDED: "模型排除",
};
const kindLabels: Record<string, string> = {
  system: "系统",
  domain: "业务领域",
  capability: "能力",
  boundary: "业务边界",
  role: "角色",
  action: "行为",
  state: "状态",
  condition: "条件",
  rule: "规则",
  invariant: "不变量",
  data: "数据",
  contract: "契约",
  deployment: "部署",
  test: "测试",
  flow: "流程",
  risk: "风险",
};
const readinessLabels: Record<string, string> = {
  described: "已描述",
  implemented: "已实现",
  verified: "已验证",
  live: "已上线",
};

function Status({ value }: { value: string }) {
  return (
    <span className={`status status-${value.toLowerCase()}`}>
      <span aria-hidden="true" />
      {verdictLabels[value as Verdict] ?? value}
    </span>
  );
}

export default function App({
  state,
  initialView = "overview",
}: {
  state: AppState;
  initialView?: BusinessView;
}) {
  if (state.error) return <SnapshotFailure error={state.error} />;
  return <Workbench snapshot={state.snapshot} initialView={initialView} />;
}

function SnapshotFailure({ error }: { error: SnapshotError }) {
  const diagnostic = `stage: ${error.stage}\nreason: ${error.message}\npage: ${typeof location === "undefined" ? "" : location.href}`;
  return (
    <main className="failure" role="alert">
      <h1>无法打开业务图谱快照</h1>
      <dl>
        <dt>阶段</dt>
        <dd>{error.stage}</dd>
        <dt>原因</dt>
        <dd>{error.message}</dd>
      </dl>
      <p>用 arc-verify snapshot 重新导出，再从导出目录打开 index.html。</p>
      <button type="button" onClick={() => void navigator.clipboard?.writeText(diagnostic)}>
        复制诊断
      </button>
    </main>
  );
}

function Workbench({ snapshot, initialView }: { snapshot: Snapshot; initialView: BusinessView }) {
  const [view, setView] = useState<BusinessView>(initialView);
  const [selected, setSelected] = useState("");
  const [root, setRoot] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [readinessFilter, setReadinessFilter] = useState("");
  const [domainFilter, setDomainFilter] = useState("");
  const [roleFilter, setRoleFilter] = useState("");
  const modelNodes = snapshot.model.data.nodes;
  const [display, setDisplay] = useState<"graph" | "tree" | "matrix">("graph");
  const [runIndex, setRunIndex] = useState(snapshot.runs.length ? 0 : -1);
  const projection = snapshot.views[view];
  const visible = useMemo(
    () =>
      visibleNodes(projection, {
        root,
        status: statusFilter,
        readiness: readinessFilter,
        domain: domainFilter,
        role: roleFilter,
      }),
    [projection, root, statusFilter, readinessFilter, domainFilter, roleFilter],
  );
  const selectedNode = projection.nodes.find((node) => node.id === selected);
  const meta = snapshot.meta;

  function changeView(next: BusinessView, nextRoot = "") {
    setView(next);
    setRoot(nextRoot);
    if (next === "capabilities" && display === "graph") setDisplay("tree");
  }
  // Hierarchical views narrow to the entity's subtree; the others keep the full
  // view with the same entity selected, so its form in that view stays in context.
  function drill(next: BusinessView) {
    changeView(next, drillRoot(snapshot.views[next], selected));
  }
  function openRun(run: string) {
    const index = snapshot.runs.findIndex((item) => item.trace.run === run);
    if (index >= 0) {
      setRunIndex(index);
      setView("verification");
    }
  }

  return (
    <div className="workbench">
      <header className="masthead">
        <strong className="brand">业务图谱</strong>
        <dl className="context">
          <div>
            <dt>图版本</dt>
            <dd>{meta.graph}</dd>
          </div>
          <div>
            <dt>hash</dt>
            <dd title={meta.graph_sha256}>{meta.graph_sha256.slice(0, 12)}</dd>
          </div>
          <div>
            <dt>环境</dt>
            <dd>{meta.environment || "未选择"}</dd>
          </div>
          <div>
            <dt>轮次</dt>
            <dd>{meta.round || "未选择"}</dd>
          </div>
          <div>
            <dt>导出时间</dt>
            <dd>
              <time dateTime={meta.generated}>{meta.generated}</time>
            </dd>
          </div>
        </dl>
      </header>
      {!meta.round && (
        <p className="notice">没有选择执行轮次：页面只展示模型与义务，不代表任何执行结果。</p>
      )}
      <div className="layout">
        <nav className="spaces" aria-label="工作空间与视图">
          {spaces.map((space) => (
            <section key={space.id}>
              <h2>{space.name}</h2>
              <ul>
                {businessViews
                  .filter((id) => viewInfo[id].workspace === space.id)
                  .map((id) => (
                    <li key={id}>
                      <button
                        type="button"
                        className={id === view ? "current" : ""}
                        aria-current={id === view ? "page" : undefined}
                        onClick={() => changeView(id)}
                      >
                        <span>{viewInfo[id].name}</span>
                        <Status value={snapshot.views[id].verdict} />
                      </button>
                    </li>
                  ))}
              </ul>
            </section>
          ))}
        </nav>
        <main className="content">
          <div className="toolbar">
            <h1>{viewInfo[view].name}</h1>
            <CoverageLine projection={projection} />
            <label>
              状态
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
              >
                <option value="">全部</option>
                {Object.entries(verdictLabels).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              完整度
              <select
                value={readinessFilter}
                onChange={(event) => setReadinessFilter(event.target.value)}
              >
                <option value="">全部</option>
                {Object.entries(readinessLabels).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            {(["domain", "role"] as const).map((kind) => (
              <label key={kind}>
                {kind === "domain" ? "领域" : "角色"}
                <select
                  value={kind === "domain" ? domainFilter : roleFilter}
                  onChange={(event) =>
                    (kind === "domain" ? setDomainFilter : setRoleFilter)(event.target.value)
                  }
                >
                  <option value="">全部</option>
                  {modelNodes
                    .filter((node) => node.kind === kind)
                    .map((node) => (
                      <option key={node.id} value={node.id}>
                        {node.name}
                      </option>
                    ))}
                </select>
              </label>
            ))}
            {view !== "verification" && view !== "impact" && (
              <div className="segmented" role="group" aria-label="展示方式">
                {(["graph", "tree", "matrix"] as const).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    aria-pressed={display === mode}
                    onClick={() => setDisplay(mode)}
                  >
                    {{ graph: "关系图", tree: "层级", matrix: "矩阵" }[mode]}
                  </button>
                ))}
              </div>
            )}
          </div>
          {root && (
            <p className="drill">
              从 <strong>{projection.nodes.find((node) => node.id === root)?.name ?? root}</strong>{" "}
              下钻
              <button type="button" onClick={() => setRoot("")}>
                返回全部
              </button>
            </p>
          )}
          {projection.truncated && (
            <p className="notice">
              视图被截断：导出了 {projection.nodes.length} / {projection.total}{" "}
              个实体，未导出的部分不计为通过。
            </p>
          )}
          {projection.diagnostics.length > 0 && (
            <ul className="diagnostics" aria-label="模型诊断">
              {projection.diagnostics.map((item) => (
                <li
                  key={`${item.code}/${item.subjects.join(",")}`}
                  className={`severity-${item.severity}`}
                >
                  <code>{item.code}</code> {item.reason} <small>{item.subjects.join("、")}</small>
                </li>
              ))}
            </ul>
          )}
          {view === "verification" ? (
            <VerificationPanel
              projection={projection}
              statusFilter={statusFilter}
              selected={selected}
              snapshot={snapshot}
              runIndex={runIndex}
              setRunIndex={setRunIndex}
              clearSelection={() => setSelected("")}
            />
          ) : view === "impact" ? (
            <ImpactPanel snapshot={snapshot} />
          ) : visible.length === 0 ? (
            <p className="empty">
              当前筛选下没有实体。清除筛选可以看到全部 {projection.nodes.length} 个实体。
            </p>
          ) : display === "graph" ? (
            <BusinessCanvas
              projection={projection}
              nodes={visible}
              selected={selected}
              choose={setSelected}
            />
          ) : display === "tree" ? (
            <CapabilityTree nodes={visible} selected={selected} choose={setSelected} />
          ) : (
            <NodeMatrix nodes={visible} selected={selected} choose={setSelected} />
          )}
        </main>
        <aside className="entity" aria-label="所选实体">
          {selected ? (
            <EntityPanel
              snapshot={snapshot}
              view={view}
              id={selected}
              node={selectedNode}
              drill={drill}
              openRun={openRun}
            />
          ) : (
            <p className="empty">
              选择一个实体，查看它的状态、相关义务，并在其他视图中定位同一实体。
            </p>
          )}
        </aside>
      </div>
    </div>
  );
}

function CoverageLine({ projection }: { projection: Projection }) {
  const { obligations } = projection.coverage;
  return (
    <p className="coverage" title="完整义务分母不随筛选变化">
      <Status value={projection.verdict} /> 完整义务 {obligations.passed} /{" "}
      {obligations.total - obligations.excluded} 通过
    </p>
  );
}

function BusinessCanvas({
  projection,
  nodes,
  selected,
  choose,
}: {
  projection: Projection;
  nodes: ProjectedNode[];
  selected: string;
  choose: (id: string) => void;
}) {
  const graph = useMemo(() => {
    const shown = new Set(nodes.map((node) => node.id));
    const edges = projection.edges.filter((edge) => shown.has(edge.from) && shown.has(edge.to));
    const layout = layoutNodes(nodes, edges, projection.view);
    const index = new Map(nodes.map((node) => [node.id, node]));
    const flowNodes: Node[] = layout.map((position) => {
      const node = index.get(position.id)!;
      return {
        id: node.id,
        parentId: position.parentId,
        position: { x: position.x, y: position.y },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
        data: {
          label: (
            <>
              <span className="graph-kind">{kindLabels[node.kind] ?? node.kind}</span>
              <strong>{node.name || node.id}</strong>
              <span className="graph-id">{node.id}</span>
              {!position.container && <Status value={node.status} />}
            </>
          ),
        },
        className: `business-node ${position.container ? "boundary-container" : ""}`,
        selected: node.id === selected,
        style: { width: position.width, height: position.height },
        ariaLabel: `${kindLabels[node.kind] ?? node.kind} ${node.name} ${verdictLabels[node.status]}`,
      };
    });
    const flowEdges: Edge[] = edges.map((edge) => ({
      id: edge.id,
      source: edge.from,
      target: edge.to,
      label: edge.kind,
      type: "smoothstep",
      markerEnd: { type: MarkerType.ArrowClosed },
      className: `relation-${edge.kind}`,
    }));
    return { flowNodes, flowEdges };
  }, [projection, nodes, selected]);
  return (
    <div className="canvas" aria-label="业务关系图">
      <ReactFlow
        key={`${projection.view}/${nodes.length}`}
        nodes={graph.flowNodes}
        edges={graph.flowEdges}
        fitView
        nodesDraggable={false}
        nodesConnectable={false}
        onNodeClick={(_, node) => choose(node.id)}
        minZoom={0.15}
        maxZoom={1.7}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={22} size={1} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}

function CapabilityTree({
  nodes,
  selected,
  choose,
}: {
  nodes: ProjectedNode[];
  selected: string;
  choose: (id: string) => void;
}) {
  const ids = new Set(nodes.map((node) => node.id));
  const children = (parent?: string) =>
    nodes.filter((node) =>
      parent ? node.parent === parent : !node.parent || !ids.has(node.parent),
    );
  function branch(node: ProjectedNode, ancestors: Set<string>) {
    if (ancestors.has(node.id)) return <li key={node.id}>模型归属存在循环：{node.id}</li>;
    const nested = children(node.id);
    const button = (
      <button
        type="button"
        className={`tree-node ${selected === node.id ? "selected" : ""}`}
        onClick={() => choose(node.id)}
      >
        <span className="kind-tag">{kindLabels[node.kind] ?? node.kind}</span>
        <span>
          <strong>{node.name}</strong>
          <small>{node.id}</small>
        </span>
        <Status value={node.status} />
      </button>
    );
    return (
      <li key={node.id}>
        {nested.length ? (
          <details open>
            <summary>{button}</summary>
            <ul>{nested.map((item) => branch(item, new Set([...ancestors, node.id])))}</ul>
          </details>
        ) : (
          button
        )}
      </li>
    );
  }
  return (
    <div className="tree">
      <ul>{children().map((node) => branch(node, new Set()))}</ul>
    </div>
  );
}

function NodeMatrix({
  nodes,
  selected,
  choose,
}: {
  nodes: ProjectedNode[];
  selected: string;
  choose: (id: string) => void;
}) {
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>业务实体</th>
            <th>验证状态</th>
            <th>通过 / 必需</th>
            {Object.values(readinessLabels).map((label) => (
              <th key={label}>{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {nodes.map((node) => (
            <tr key={node.id} className={node.id === selected ? "selected" : ""}>
              <td>
                <button type="button" className="link" onClick={() => choose(node.id)}>
                  {node.name}
                  <small>{node.id}</small>
                </button>
              </td>
              <td>
                <Status value={node.status} />
              </td>
              <td>
                {node.passed} / {node.required - node.excluded}
              </td>
              <td>{node.readiness.described ? "是" : "未知"}</td>
              <td title={node.readiness.implementation_status}>
                {node.readiness.implemented ? "是" : node.readiness.implementation_status}
              </td>
              <td>{node.readiness.verified ? "是" : "未证实"}</td>
              <td title={node.readiness.deployment_status}>
                {node.readiness.live ? "是" : "未知"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EntityPanel({
  snapshot,
  view,
  id,
  node,
  drill,
  openRun,
}: {
  snapshot: Snapshot;
  view: BusinessView;
  id: string;
  node?: ProjectedNode;
  drill: (next: BusinessView) => void;
  openRun: (run: string) => void;
}) {
  const obligations = snapshot.views[view].obligations.filter(
    (item) => item.definition.id === id || item.targets.includes(id),
  );
  const rule = snapshot.model.data.rules.find((item) => item.id === id || item.action === id);
  const elsewhere = businessViews.filter((other) => other !== view);
  return (
    <div>
      <p className="kind-tag">{node ? (kindLabels[node.kind] ?? node.kind) : "实体"}</p>
      <h2>{node?.name ?? id}</h2>
      <p className="mono">{id}</p>
      {node ? (
        <>
          <p>
            <Status value={node.status} /> {node.reason}
          </p>
          <dl className="facts">
            <dt>通过 / 必需</dt>
            <dd>
              {node.passed} / {node.required - node.excluded}
            </dd>
            <dt>历史失败</dt>
            <dd>{node.history_failures}</dd>
            {Object.entries(readinessLabels).map(([key, label]) => (
              <div key={key}>
                <dt>{label}</dt>
                <dd>
                  {node.readiness[key as "described"]
                    ? "是"
                    : key === "implemented"
                      ? node.readiness.implementation_status
                      : key === "live"
                        ? node.readiness.deployment_status
                        : "否"}
                </dd>
              </div>
            ))}
          </dl>
        </>
      ) : (
        <p className="empty">当前视图不包含这个实体。</p>
      )}
      <h3>在其他视图中</h3>
      <ul className="drill-list">
        {elsewhere.map((other) =>
          snapshot.views[other].nodes.some((item) => item.id === id) ? (
            <li key={other}>
              <button type="button" onClick={() => drill(other)}>
                {viewInfo[other].name}
              </button>
            </li>
          ) : (
            <li key={other} className="muted">
              {viewInfo[other].name}：该视图不含此实体
            </li>
          ),
        )}
      </ul>
      {rule && (
        <>
          <h3>规则 {rule.id}</h3>
          <dl className="facts">
            <dt>角色</dt>
            <dd>{rule.actor}</dd>
            <dt>入口</dt>
            <dd className="mono">{rule.entry}</dd>
            <dt>前置条件</dt>
            <dd>{rule.conditions.join("、") || "无"}</dd>
            <dt>成功状态</dt>
            <dd>{rule.expected}</dd>
            <dt>拒绝状态</dt>
            <dd>{rule.rejected}</dd>
            <dt>独立判据</dt>
            <dd>{rule.oracle}</dd>
          </dl>
        </>
      )}
      <h3>相关义务 {obligations.length}</h3>
      <ObligationList obligations={obligations} openRun={openRun} runs={snapshot.runs} />
    </div>
  );
}

function ObligationList({
  obligations,
  openRun,
  runs,
}: {
  obligations: Obligation[];
  openRun: (run: string) => void;
  runs: SnapshotRun[];
}) {
  if (obligations.length === 0) return <p className="empty">没有义务指向这个实体。</p>;
  const exported = new Set(runs.map((run) => run.trace.run));
  return (
    <ul className="obligations">
      {obligations.map((item) => (
        <li key={item.definition.id}>
          <Status value={item.status} />
          <span className="mono">{item.definition.id}</span>
          <small>
            {item.definition.kind} · {item.reason}
          </small>
          {(() => {
            const run = evidenceRun(item, runs);
            if (!run) return null;
            return exported.has(run) ? (
              <button type="button" className="link" onClick={() => openRun(run)}>
                回放 {run}
              </button>
            ) : (
              <small className="muted">运行 {run} 未导出到本快照</small>
            );
          })()}
        </li>
      ))}
    </ul>
  );
}

function VerificationPanel({
  projection,
  statusFilter,
  selected,
  snapshot,
  runIndex,
  setRunIndex,
  clearSelection,
}: {
  projection: Projection;
  statusFilter: string;
  selected: string;
  snapshot: Snapshot;
  runIndex: number;
  setRunIndex: (index: number) => void;
  clearSelection: () => void;
}) {
  const obligations = projection.obligations.filter(
    (item) =>
      (!statusFilter || item.status === statusFilter) &&
      (!selected || item.definition.id === selected || item.targets.includes(selected)),
  );
  return (
    <div className="verification">
      <section>
        <h2>
          义务 {obligations.length} / {projection.obligations.length}
        </h2>
        {selected && (
          <p className="drill">
            只看指向 <strong>{selected}</strong> 的义务
            <button type="button" onClick={clearSelection}>
              显示全部义务
            </button>
          </p>
        )}
        {obligations.length === 0 ? (
          <p className="empty">
            {selected ? `没有义务指向 ${selected}` : "没有符合当前状态筛选的义务"}；完整分母仍是{" "}
            {projection.obligations.length} 条。
          </p>
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>义务</th>
                  <th>种类</th>
                  <th>状态</th>
                  <th>原因</th>
                  <th>规划</th>
                </tr>
              </thead>
              <tbody>
                {obligations.map((item) => (
                  <tr key={item.definition.id}>
                    <td className="mono">{item.definition.id}</td>
                    <td>{item.definition.kind}</td>
                    <td>
                      <Status value={item.status} />
                    </td>
                    <td>{item.reason}</td>
                    <td>{item.planned ? "已规划" : "未规划"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <Replay runs={snapshot.runs} index={runIndex} setIndex={setRunIndex} />
    </div>
  );
}

function Replay({
  runs,
  index,
  setIndex,
}: {
  runs: SnapshotRun[];
  index: number;
  setIndex: (index: number) => void;
}) {
  const [step, setStep] = useState(0);
  if (runs.length === 0 || index < 0)
    return <p className="empty">本快照没有导出运行。导出时用 --run 选择要回放的运行。</p>;
  const run = runs[index];
  const steps = run.trace.steps;
  const current = steps[Math.min(step, Math.max(steps.length - 1, 0))];
  const evidence = run.evidence.filter(
    (entry) => entry.phase !== "action" || String(entry.step) === current?.step,
  );
  return (
    <section
      className="replay"
      aria-label="运行回放"
      onKeyDown={(event) => {
        if (event.key === "ArrowRight") setStep(Math.min(step + 1, steps.length - 1));
        if (event.key === "ArrowLeft") setStep(Math.max(step - 1, 0));
      }}
    >
      <h2>运行回放</h2>
      <label>
        运行
        <select
          value={index}
          onChange={(event) => {
            setIndex(Number(event.target.value));
            setStep(0);
          }}
        >
          {runs.map((item, position) => (
            <option key={item.trace.run} value={position}>
              {item.trace.run} · {item.trace.task} ·{" "}
              {verdictLabels[item.trace.status as Verdict] ?? item.trace.status}
            </option>
          ))}
        </select>
      </label>
      <p>
        <Status value={run.trace.status} />{" "}
        {run.trace.first_divergence
          ? `首个偏离：第 ${run.trace.first_divergence} 步`
          : "没有偏离步"}
        {run.trace.evidence_status && run.trace.evidence_status !== "PASS" && (
          <span className="warning"> · 证据状态 {run.trace.evidence_status}</span>
        )}
      </p>
      {steps.length === 0 ? (
        <p className="empty">这个运行没有记录任何步骤。</p>
      ) : (
        <>
          <ol className="steps">
            {steps.map((item, position) => (
              <li key={item.step}>
                <button
                  type="button"
                  aria-current={position === step ? "step" : undefined}
                  className={item.step === run.trace.first_divergence ? "divergence" : ""}
                  onClick={() => setStep(position)}
                >
                  第 {item.step} 步 · {item.node ?? ""} <Status value={item.verdict} />
                </button>
              </li>
            ))}
          </ol>
          {current && (
            <dl className="facts">
              <dt>期望</dt>
              <dd>{current.expected}</dd>
              <dt>观察</dt>
              <dd>{current.observed}</dd>
              <dt>决定</dt>
              <dd>{current.decision}</dd>
              {current.transition && (
                <>
                  <dt>状态转移</dt>
                  <dd className="mono">{current.transition}</dd>
                </>
              )}
              <dt>RAW hash</dt>
              <dd className="mono">{current.sha256}</dd>
            </dl>
          )}
          <OracleReadings observation={current?.observation} />
          {concurrentActions(current?.observation) && (
            <p className="notice">
              并发批次：同一步里多个参与者的动作顺序没有被观察，不按列表顺序解读。
            </p>
          )}
        </>
      )}
      <h3>证据</h3>
      <ul className="evidence">
        {evidence.map((entry) => (
          <EvidenceEntry key={`${entry.phase}/${entry.step}`} entry={entry} />
        ))}
      </ul>
      {run.trace.recovery && (
        <p>
          夹具恢复：
          <Status value={run.trace.recovery.verdict} /> {run.trace.recovery.reason}
        </p>
      )}
    </section>
  );
}

type Facts = {
  state?: string;
  conditions?: Record<string, boolean>;
  values?: Record<string, unknown>;
};
type OracleReading = { actor?: string; id?: string; before?: Facts; after?: Facts };

// Each oracle's independent before/after reading; a FAIL with matching states comes
// from a property or denial check, and these values are what that check judged.
function OracleReadings({ observation }: { observation: Record<string, unknown> | undefined }) {
  const oracles = observation?.oracles;
  if (!Array.isArray(oracles) || oracles.length === 0) return null;
  const show = (facts?: Facts) =>
    [
      facts?.state ?? "未观察",
      ...Object.entries(facts?.values ?? {}).map(
        ([key, value]) => `${key}=${JSON.stringify(value)}`,
      ),
    ].join(" · ");
  return (
    <table className="readings">
      <thead>
        <tr>
          <th>独立判据</th>
          <th>动作前</th>
          <th>动作后</th>
        </tr>
      </thead>
      <tbody>
        {(oracles as OracleReading[]).map((oracle) => (
          <tr key={`${oracle.actor}/${oracle.id}`}>
            <td className="mono">
              {oracle.id} · {oracle.actor}
            </td>
            <td>{show(oracle.before)}</td>
            <td>{show(oracle.after)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// A batch with several participant actions has no observed order between them.
function concurrentActions(observation: Record<string, unknown> | undefined): boolean {
  const actions = observation?.actions;
  return Array.isArray(actions) && actions.length > 1;
}

function EvidenceEntry({ entry }: { entry: SnapshotEvidence }) {
  const phase =
    { setup: "隔离检查", action: `第 ${entry.step} 步`, fixture: "夹具恢复" }[entry.phase] ??
    entry.phase;
  if (entry.error) {
    return (
      <li className="evidence-failed">
        <strong>{phase}：导出时校验失败</strong>
        <span>{entry.error}</span>
      </li>
    );
  }
  return (
    <li>
      <strong>{phase}</strong>
      <span className="mono">sha256 {entry.sha256}</span>
      {entry.data && (
        <span>
          期望 {entry.data.expected} · 观察 {entry.data.observed} · 判据 {entry.data.oracle}
        </span>
      )}
    </li>
  );
}

function ImpactPanel({ snapshot }: { snapshot: Snapshot }) {
  const impact = snapshot.impact;
  if (!impact) return <p className="empty">当前版本没有声明上一版本，没有可比较的差异。</p>;
  return (
    <div className="impact">
      <p>
        与 <strong>{impact.previous}</strong> 比较 · {impact.scope}
      </p>
      {[
        ["变更的实体", impact.changed],
        ["受影响的实体", impact.affected],
        ["待复验测试", impact.tests],
      ].map(([title, items]) => (
        <section key={title as string}>
          <h2>
            {title as string} {(items as string[]).length}
          </h2>
          {(items as string[]).length ? (
            <ul>
              {(items as string[]).map((id) => (
                <li key={id}>
                  <span className="mono">{id}</span>
                  {impact.reasons[id] && <small> · {impact.reasons[id].join("、")}</small>}
                </li>
              ))}
            </ul>
          ) : (
            <p className="empty">无</p>
          )}
        </section>
      ))}
    </div>
  );
}
