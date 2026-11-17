import { useMemo, useReducer, useState } from "react";
import {
  branchAppliesToFlow,
  branchOptions,
  executionCoverageScope,
  factsOf,
  selectionContext,
} from "./explorer-model";
import { entityKey, factEntities } from "./facts";
import FactPanel from "./FactPanel";
import PreconditionLedger from "./PreconditionLedger";
import "./precondition-ledger.css";
import { CampaignTrail } from "./CampaignViews";
import GraphCanvas from "./GraphCanvas";
import DependencyInspector, { LineageBlockerPanel } from "./DependencyInspector";
import { graphFor } from "./graph-model";
import { buildDependencyMap } from "./lineage-map";
import { buildLineageBlockers } from "./lineage-blockers";
import { occurrenceKey, occurrenceLabel } from "./lineage";
import { initialSelection, matchingRuns, matchesStep, reduceSelection } from "./selection";
import type { Selection, SelectionAction, SelectionContext } from "./selection";
import { predicateText } from "./contract";
import { SnapshotError } from "./snapshot";
import { Status } from "./Status";
import type { BusinessNode, BusinessView, Snapshot } from "./types";
import "@xyflow/react/dist/style.css";
import "./App.css";
import "./lineage.css";

export type AppState =
  | { snapshot: Snapshot; error?: undefined }
  | { error: SnapshotError; snapshot?: undefined };

export default function App({
  state,
  initialView = "overview",
  initialSelected = "",
  initialContext = {},
}: {
  state: AppState;
  initialView?: BusinessView;
  initialSelected?: string;
  initialContext?: Partial<Selection>;
}) {
  if (state.error) return <SnapshotFailure error={state.error} />;
  return (
    <Workbench
      key={state.snapshot.meta.graph + state.snapshot.meta.generated}
      snapshot={state.snapshot}
      initialView={initialView}
      initialSelected={initialSelected}
      initialContext={initialContext}
    />
  );
}

function SnapshotFailure({ error }: { error: SnapshotError }) {
  const diagnostic = `stage: ${error.stage}\nreason: ${error.message}`;
  return (
    <main className="failure" role="alert">
      <span className="eyebrow">快照读取失败</span>
      <h1>无法打开业务验证工作台</h1>
      <dl>
        <dt>阶段</dt>
        <dd>{error.stage}</dd>
        <dt>原因</dt>
        <dd>{error.message}</dd>
      </dl>
      <p>使用 arc verify snapshot 重新导出，再打开导出目录中的 index.html。</p>
      <button type="button" onClick={() => void navigator.clipboard?.writeText(diagnostic)}>
        复制诊断
      </button>
    </main>
  );
}

function initialize(
  snapshot: Snapshot,
  context: SelectionContext,
  initial: Partial<Selection>,
  view: BusinessView,
  node: string,
) {
  let state = initialSelection(snapshot.meta.graph, snapshot.meta.environment ?? "", node);
  state.canvas =
    view === "boundaries" || (view === "overview" && snapshot.business_lineage)
      ? "dependencies"
      : view === "verification"
        ? "run"
        : "states";
  for (const type of [
    "environment",
    "flow",
    "campaign",
    "branch",
    "variant",
    "run",
    "actor",
    "entity",
    "step",
    "occurrence",
    "node",
  ] as const) {
    // An absent actor-specific node is not a request to clear a valid step.
    if (initial[type] !== undefined && !(type === "node" && initial[type] === "")) {
      const lineageNode =
        type === "node" &&
        initial.canvas === "dependencies" &&
        snapshot.business_lineage?.branches
          .find((item) => item.branch === state.branch)
          ?.nodes.some((item) => item.id === initial[type]);
      state = reduceSelection(
        state,
        { type: lineageNode ? "lineage" : type, value: initial[type]! },
        context,
      );
    }
  }
  if (initial.canvas) state.canvas = initial.canvas;
  if (initial.focus) state.focus = initial.focus;
  if (initial.dependencyScope) state.dependencyScope = initial.dependencyScope;
  return state;
}

function Workbench({
  snapshot,
  initialView,
  initialSelected,
  initialContext,
}: {
  snapshot: Snapshot;
  initialView: BusinessView;
  initialSelected: string;
  initialContext: Partial<Selection>;
}) {
  const branches = useMemo(() => branchOptions(snapshot), [snapshot]);
  const context = useMemo(() => selectionContext(snapshot, branches), [snapshot, branches]);
  const [selection, dispatch] = useReducer(
    (state: Selection, action: SelectionAction) => reduceSelection(state, action, context),
    undefined,
    () => initialize(snapshot, context, initialContext, initialView, initialSelected),
  );
  const [treeRoot, setTreeRoot] = useState("");
  const [search, setSearch] = useState("");
  const [showLegacy, setShowLegacy] = useState(!snapshot.model.data.business_contract);
  const availableRuns = matchingRuns(context, selection);
  const campaigns = (snapshot.business_campaigns ?? []).filter(
    (item) =>
      (!selection.environment || item.environment === selection.environment) &&
      (!selection.flow || item.flow === selection.flow),
  );
  const campaign = campaigns.find((item) => item.id === selection.campaign);
  const run = snapshot.runs.find(
    (item) =>
      item.trace.run === selection.run &&
      availableRuns.some((candidate) => candidate.id === item.trace.run),
  );
  const runContext = context.runs.find((item) => item.id === run?.trace.run);
  const stepContexts = runContext?.steps ?? [];
  const currentStep = stepContexts.find(
    (item) => item.id === selection.step && matchesStep(item, selection),
  );
  const fact =
    run && currentStep
      ? factsOf(snapshot, run).find((item) => String(item.seq) === currentStep.id)
      : undefined;
  const trace =
    run && currentStep ? run.trace.steps.find((item) => item.step === currentStep.id) : undefined;
  const branch = branches.find((item) => item.id === selection.branch);
  const actorIDs = [...new Set(stepContexts.flatMap((step) => step.actors))];
  const entities = [
    ...new Map(
      (run ? factsOf(snapshot, run) : [])
        .flatMap(factEntities)
        .map((entity) => [entityKey(entity), entity]),
    ).entries(),
  ];
  const canonical = branches.filter((item) => item.contract);
  const reachable = new Set(treeRoot ? [treeRoot] : []);
  if (treeRoot) {
    for (let pass = 0; pass < snapshot.model.data.nodes.length; pass++) {
      let changed = false;
      for (const edge of snapshot.model.data.edges)
        if (
          ["contains", "implements"].includes(edge.kind) &&
          reachable.has(edge.from) &&
          !reachable.has(edge.to)
        ) {
          reachable.add(edge.to);
          changed = true;
        }
      if (!changed) break;
    }
  }
  const visibleBranches = branches.filter(
    (item) =>
      branchAppliesToFlow(item, selection.flow) &&
      (showLegacy || !item.legacy) &&
      (!treeRoot || reachable.has(item.action)) &&
      (!search ||
        `${item.label} ${item.contract ? predicateText(item.contract.when) : ""}`
          .toLowerCase()
          .includes(search.toLowerCase())),
  );
  const dependencyScope =
    selection.dependencyScope ?? (selection.branch ? "branch" : selection.flow ? "flow" : "model");
  const dependencyMap =
    snapshot.business_lineage &&
    selection.canvas === "dependencies" &&
    (!selection.branch || dependencyScope !== "branch")
      ? buildDependencyMap(
          snapshot,
          dependencyScope === "model" ? "" : selection.flow,
          treeRoot
            ? branches.filter((item) => reachable.has(item.action)).map((item) => item.id)
            : undefined,
        )
      : undefined;
  const graph =
    dependencyMap?.graph ??
    graphFor(
      snapshot,
      selection,
      branches,
      run,
      stepContexts.filter((item) => matchesStep(item, selection)),
    );
  const blockers = graph.lineage ? buildLineageBlockers(graph.lineage) : undefined;
  const blockerNodes = blockers
    ? [
        ...new Set(
          blockers.issues.flatMap((issue) => [
            ...issue.path,
            issue.node,
            ...issue.alternatives.map((entry) => entry.node),
          ]),
        ),
      ]
    : [];
  const diagnostics = [
    ...new Map(
      Object.values(snapshot.views)
        .flatMap((view) => view.diagnostics)
        .map((item) => [`${item.code}/${item.subjects.join("/")}`, item]),
    ).values(),
  ];
  const proof = snapshot.contract_analysis;
  const executionScope = executionCoverageScope(snapshot, selection);
  return (
    <div className="workbench">
      <header className="masthead">
        <div className="brand-lockup">
          <span className="brand-mark">A</span>
          <div>
            <strong>
              ARC <span>业务验证</span>
            </strong>
            <p>模型分支 · 真实运行 · 独立事实</p>
          </div>
        </div>
        <div className="snapshot-identity">
          <span className="read-only">离线只读快照</span>
          <code title={snapshot.meta.graph_sha256}>{snapshot.meta.graph_sha256.slice(0, 12)}</code>
          <time dateTime={snapshot.meta.generated}>{snapshot.meta.generated}</time>
        </div>
      </header>
      <section
        className={`global-context${snapshot.business_campaigns?.length ? " has-campaigns" : ""}`}
        aria-label="统一业务上下文"
      >
        <label>
          模型
          <select
            aria-label="模型"
            value={selection.model}
            onChange={(event) => dispatch({ type: "model", value: event.target.value })}
          >
            <option value={snapshot.meta.graph}>{snapshot.meta.graph}</option>
          </select>
        </label>
        <label>
          环境
          <select
            aria-label="环境"
            value={selection.environment}
            onChange={(event) => dispatch({ type: "environment", value: event.target.value })}
          >
            <option value="">全部已导出环境</option>
            {context.environments.map((environment) => (
              <option key={environment} value={environment}>
                {environment}
              </option>
            ))}
          </select>
        </label>
        <label>
          流程
          <select
            aria-label="流程"
            value={selection.flow}
            onChange={(event) => {
              setTreeRoot("");
              dispatch({ type: "flow", value: event.target.value });
            }}
          >
            <option value="">全部声明流程</option>
            {snapshot.model.data.flows?.map((flow) => (
              <option key={flow.id} value={flow.id}>
                {snapshot.model.data.nodes.find((node) => node.id === flow.capability)?.name ??
                  flow.id}{" "}
                · {flow.id}
              </option>
            ))}
          </select>
        </label>
        {!!snapshot.business_campaigns?.length && (
          <label className="campaign-select">
            有界探索 / Campaign
            <select
              aria-label="Campaign"
              value={selection.campaign}
              onChange={(event) => dispatch({ type: "campaign", value: event.target.value })}
            >
              <option value="">查看全部独立运行</option>
              {campaigns.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.id} · {item.status} · {item.round}
                </option>
              ))}
            </select>
          </label>
        )}
        <label className="branch-select">
          <span>
            <b>1</b> 业务分支
          </span>
          <select
            aria-label="业务分支"
            value={selection.branch}
            onChange={(event) => dispatch({ type: "branch", value: event.target.value })}
          >
            <option value="">先选择条件与预期</option>
            <optgroup label="声明业务分支">
              {canonical
                .filter((item) => branchAppliesToFlow(item, selection.flow))
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.label}
                  </option>
                ))}
            </optgroup>
            <optgroup label="历史测试（旧格式）">
              {branches
                .filter((item) => item.legacy && (!selection.flow || item.flow === selection.flow))
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.label}
                  </option>
                ))}
            </optgroup>
            <optgroup label="未决记录诊断">
              {branches
                .filter((item) => item.unresolved)
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.label}
                  </option>
                ))}
            </optgroup>
          </select>
        </label>
        <label className="run-select">
          <span>
            <b>2</b> 运行
          </span>
          <select
            aria-label="运行"
            value={selection.run}
            disabled={!selection.branch}
            onChange={(event) => dispatch({ type: "run", value: event.target.value })}
          >
            <option value="">
              {selection.branch ? `选择已导出的运行（${availableRuns.length}）` : "先选择业务分支"}
            </option>
            {availableRuns.map((item) => (
              <option key={item.id} value={item.id}>
                {item.id} · {item.environment || "环境未绑定"} ·{" "}
                {snapshot.runs.find((entry) => entry.trace.run === item.id)?.runner_id ||
                  "Runner 未记录"}{" "}
                · {snapshot.runs.find((entry) => entry.trace.run === item.id)?.trace.status}
              </option>
            ))}
          </select>
        </label>
      </section>
      <div className="context-strip">
        <span>
          <i className="context-dot" />
          {selection.branch
            ? branch?.legacy
              ? "历史测试范围"
              : branch?.unresolved
                ? "分支关联缺失 · 仅诊断"
                : "已定位业务分支"
            : "正在审阅模型 · 尚未选择运行"}
        </span>
        <span>
          导出轮次 {snapshot.meta.round || "未选择"} · 运行 {snapshot.runs.length} /{" "}
          {snapshot.available_runs ?? snapshot.runs.length}
        </span>
        {snapshot.runs_truncated && (
          <span className="warning">历史被截断；未导出的运行不计为通过</span>
        )}
        {campaign && (
          <span>
            Campaign <code>{campaign.id}</code> · {selection.variant || "尚未选择输入变体"}
          </span>
        )}
        {snapshot.campaigns_truncated && (
          <span className="warning">
            Campaign 导出 {snapshot.business_campaigns?.length ?? 0} /{" "}
            {snapshot.available_campaigns ?? "未知"}；其余历史未包含
          </span>
        )}
      </div>
      <div className="workspace">
        <nav className="capability-rail" aria-label="能力树与分支导航">
          <div className="panel-heading">
            <span className="eyebrow">01 / 业务导航</span>
            <h2>业务能力树</h2>
            <p>从能力找到业务分支</p>
          </div>
          <CapabilityTree
            snapshot={snapshot}
            selected={treeRoot || selection.node}
            choose={(id) => {
              setTreeRoot(id);
              dispatch({ type: "flow", value: "" });
              dispatch({ type: "node", value: id });
            }}
          />
          {treeRoot && (
            <button type="button" className="clear-scope" onClick={() => setTreeRoot("")}>
              显示全部能力的分支
            </button>
          )}
          <div className="branch-heading">
            <h3>
              业务分支 <span>{visibleBranches.length}</span>
            </h3>
            <input
              aria-label="搜索业务分支"
              placeholder="搜索动作、条件或分支 ID"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          <div className="branch-list">
            {visibleBranches.map((item) => (
              <button
                type="button"
                key={item.id}
                className={selection.branch === item.id ? "branch-card current" : "branch-card"}
                onClick={() => dispatch({ type: "branch", value: item.id })}
              >
                <span className="branch-card-title">
                  {snapshot.model.data.nodes.find((node) => node.id === item.action)?.name ??
                    (item.unresolved ? "未关联分支的运行" : item.action)}
                </span>
                <span className="branch-condition">
                  {item.contract
                    ? predicateText(item.contract.when)
                    : item.unresolved
                      ? "UNKNOWN / CONFLICT / 历史关联缺失"
                      : item.legacy?.name}
                </span>
                <code>{item.id}</code>
                {item.contract && (
                  <span className="branch-execution">
                    {!executionScope.appliesToSelection ? (
                      "当前选择无适用覆盖汇总"
                    ) : snapshot.contract_execution?.branches.find(
                        (entry) => entry.branch === item.id,
                      ) ? (
                      <Status
                        value={
                          snapshot.contract_execution.branches.find(
                            (entry) => entry.branch === item.id,
                          )!.status
                        }
                        raw
                      />
                    ) : (
                      "未导出分支判定"
                    )}
                  </span>
                )}
                <small>
                  {item.contract?.confirmed
                    ? "已确认"
                    : item.legacy
                      ? "旧格式"
                      : item.unresolved
                        ? "诊断"
                        : "候选"}{" "}
                  ·{" "}
                  {
                    context.runs.filter(
                      (candidate) =>
                        candidate.branch === item.id || candidate.branches?.includes(item.id),
                    ).length
                  }{" "}
                  个导出运行
                </small>
              </button>
            ))}
            {!visibleBranches.length && (
              <p className="empty">当前范围没有声明分支。不会从运行结果反推规则。</p>
            )}
          </div>
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={showLegacy}
              onChange={(event) => setShowLegacy(event.target.checked)}
            />
            显示历史测试
          </label>
          <details className="rail-meta">
            <summary>模型范围与来源</summary>
            <p>{snapshot.model.data.scope}</p>
            <p>
              {snapshot.model.data.nodes.length} 节点 · {snapshot.model.data.edges.length} 关系
            </p>
            <p>
              {canonical.length} 契约分支 · {snapshot.model.approved ? "模型已确认" : "模型未确认"}
            </p>
            <code>{snapshot.meta.graph_sha256}</code>
          </details>
        </nav>
        <main className="canvas-panel">
          <div className="panel-heading canvas-title">
            <div>
              <span className="eyebrow">02 / 关系与状态</span>
              <h1>
                {branch?.contract
                  ? (snapshot.model.data.nodes.find((node) => node.id === branch.action)?.name ??
                    "业务分支")
                  : "业务运行地图"}
              </h1>
            </div>
            <div className="segmented" role="group" aria-label="画布视图">
              {(["dependencies", "states", "run"] as const).map((mode) => (
                <button
                  type="button"
                  key={mode}
                  aria-pressed={selection.canvas === mode}
                  onClick={() => dispatch({ type: "canvas", value: mode })}
                >
                  {{ dependencies: "准备依赖", states: "业务状态", run: "运行轨迹" }[mode]}
                </button>
              ))}
            </div>
          </div>
          <div className="canvas-toolbar">
            {selection.canvas === "dependencies" && snapshot.business_lineage && (
              <label>
                系谱范围
                <select
                  aria-label="系谱范围"
                  value={dependencyScope}
                  onChange={(event) =>
                    dispatch({
                      type: "dependencyScope",
                      value: event.target.value as NonNullable<Selection["dependencyScope"]>,
                    })
                  }
                >
                  <option value="model">全模型声明</option>
                  <option value="flow">当前流程声明</option>
                  <option value="branch" disabled={!selection.branch}>
                    当前分支与观察
                  </option>
                </select>
              </label>
            )}
            <label>
              局部范围
              <select
                aria-label="局部范围"
                value={selection.focus}
                onChange={(event) =>
                  dispatch({ type: "focus", value: event.target.value as Selection["focus"] })
                }
              >
                <option value="all">完整当前范围</option>
                <option value="upstream">所选节点的上游</option>
                <option value="downstream">所选节点的下游</option>
                <option value="critical" disabled={selection.canvas !== "dependencies"}>
                  声明关系最长链
                </option>
                <option value="blocked" disabled={!graph.lineage?.overlay}>
                  当前观察阻断链
                </option>
              </select>
            </label>
            <label>
              主体 / 账号
              <select
                aria-label="主体 / 账号"
                value={selection.actor}
                disabled={!run}
                onChange={(event) => dispatch({ type: "actor", value: event.target.value })}
              >
                <option value="">本次运行全部主体</option>
                {actorIDs.map((id) => (
                  <option key={id} value={id}>
                    {id}
                  </option>
                ))}
              </select>
            </label>
            <label>
              业务实体
              <select
                aria-label="业务实体"
                value={selection.entity}
                disabled={!run}
                onChange={(event) => dispatch({ type: "entity", value: event.target.value })}
              >
                <option value="">本次运行全部实体</option>
                {entities.map(([key, entity]) => (
                  <option key={key} value={key}>
                    {entity.alias} · {entity.kind} · {entity.id} · {entity.owner_actor} /{" "}
                    {entity.owner_principal}
                    {entity.channel_account_id ? ` · 渠道 ${entity.channel_account_id}` : ""}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {selection.canvas === "dependencies" && graph.lineage && (
            <div className="lineage-context-bar">
              <span>
                分支 <code>{selection.branch || "未选择"}</code>
              </span>
              <label>
                动作发生记录
                <select
                  aria-label="动作发生记录"
                  disabled={!graph.lineage.candidates.length}
                  value={graph.lineage.overlay ? occurrenceKey(graph.lineage.overlay) : ""}
                  onChange={(event) => dispatch({ type: "occurrence", value: event.target.value })}
                >
                  <option value="">
                    {graph.lineage.candidates.length ? "选择独立的观察作用域" : "尚无对应前态观察"}
                  </option>
                  {graph.lineage.candidates.map((item) => (
                    <option key={occurrenceKey(item)} value={occurrenceKey(item)}>
                      {occurrenceLabel(item)}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          )}
          {dependencyMap && (
            <p className="canvas-caption">
              {dependencyScope === "model"
                ? "全模型声明"
                : selection.flow
                  ? `流程 ${selection.flow}`
                  : "全部声明流程"}
              {treeRoot ? " · 限定所选业务能力" : ""} · 共享节点保留分支来源；未叠加任何运行结果。
            </p>
          )}
          {blockers && graph.lineage && (
            <LineageBlockerPanel
              lineage={graph.lineage}
              report={blockers}
              choose={(node) => {
                dispatch({ type: "lineage", value: node });
                dispatch({ type: "focus", value: "upstream" });
              }}
              focus={() => dispatch({ type: "focus", value: "blocked" })}
            />
          )}
          {graph.lineage?.graph && <PreconditionLedger
            lineage={graph.lineage}
            contract={snapshot.model.data.business_contract}
            run={run}
            branch={snapshot.model.data.business_contract?.branches.find((item) => item.id === selection.branch)}
            dispatch={dispatch}
          />}
          <GraphCanvas
            graph={graph}
            selection={selection}
            dispatch={dispatch}
            blockerNodes={blockerNodes}
            onSelectNode={(node) => {
              if (node.actual) dispatch({ type: "observed", value: node.actual });
              else if (dependencyMap || node.lineage) dispatch({ type: "lineage", value: node.id });
              else if (node.step) dispatch({ type: "step", value: node.step });
              else if (node.branch) dispatch({ type: "branch", value: node.branch });
              else dispatch({ type: "node", value: node.subject });
            }}
            onSelectEdge={(edge) => {
              if (edge.actual) dispatch({ type: "observed", value: edge.actual });
              else if (dependencyMap || graph.lineage)
                dispatch({ type: "lineage", value: edge.to });
              else if (edge.branch) dispatch({ type: "branch", value: edge.branch });
              else if (edge.action) dispatch({ type: "node", value: edge.action });
            }}
          />
          {graph.stateOverlay && (
            <section className="state-path-review" aria-label="本次运行的实际状态路径">
              <h3>本次运行的实际状态路径</h3>
              <p>
                Expected 保留契约定义；Actual
                来自对应动作的独立前后态。点击记录可定位账号、步骤和证据。
              </p>
              <ol>
                {graph.stateOverlay.occurrences.map((item) => (
                  <li key={item.id}>
                    <button
                      type="button"
                      aria-pressed={selection.occurrence === item.occurrence}
                      onClick={() => dispatch({ type: "observed", value: item })}
                    >
                      <strong>
                        {item.from_state} → {item.to_state}
                      </strong>
                      <span>
                        第 {item.step} 步 · {item.actor} / 动作 {item.action_index} · {item.branch}
                      </span>
                      <Status value={item.verdict} raw />
                    </button>
                  </li>
                ))}
              </ol>
              {graph.stateOverlay.diagnostics.map((diagnostic, index) => (
                <p className="notice" key={index}>
                  {diagnostic.step ? `第 ${diagnostic.step} 步 · ` : ""}
                  {diagnostic.actor ? `${diagnostic.actor} · ` : ""}
                  {diagnostic.reason}
                </p>
              ))}
            </section>
          )}
          <div className="model-disclosures">
            <details>
              <summary>
                业务规则覆盖 <Status value={proof?.status ?? "LEGACY"} raw />
              </summary>
              {proof ? (
                <>
                  <p>{proof.reason}</p>
                  <p>
                    范围 {proof.scope} · {proof.proof_kind} · 已检查 {proof.checked_cases} /{" "}
                    {proof.required_cases} · 预算 {proof.budget}
                  </p>
                  <p>
                    {proof.complete_in_scope
                      ? "仅在声明的有限域中完成检查；不是运行验收。"
                      : "覆盖不完整；不能用已运行的几条路径替代全域证明。"}
                  </p>
                  {proof.issues?.map((issue, index) => (
                    <p key={index}>
                      <Status value={issue.status} raw /> {issue.action} · {issue.reason}
                    </p>
                  ))}
                </>
              ) : (
                <p>快照未包含确定性契约分析；不把测试数量当成业务完整性。</p>
              )}
            </details>
            <details>
              <summary>
                导出范围的分支实际覆盖{" "}
                <Status value={snapshot.contract_execution?.status ?? "UNKNOWN"} raw />
              </summary>
              {snapshot.contract_execution ? (
                <>
                  <p>
                    <strong>{executionScope.label}</strong>
                  </p>
                  {!executionScope.appliesToSelection && (
                    <p className="notice">
                      {executionScope.reason}；以下保留原导出范围，不代表当前筛选已通过。
                    </p>
                  )}
                  <p>{snapshot.contract_execution.reason}</p>
                  <p>
                    已验证 {snapshot.contract_execution.passed} /{" "}
                    {snapshot.contract_execution.total} · 失败 {snapshot.contract_execution.failed}{" "}
                    · 阻塞 {snapshot.contract_execution.blocked} · 未运行{" "}
                    {snapshot.contract_execution.not_run}
                  </p>
                  <p>
                    仅汇总该轮次每个冻结分支的独立证据，不代表所选运行、账号或全部 Runner
                    输入变体通过；规则空间证明不替代真实执行。
                  </p>
                  {snapshot.contract_execution.evidence_errors?.map((entry, index) => (
                    <p className="warning" key={index}>
                      <code>{entry.run}</code> {entry.reason}
                    </p>
                  ))}
                </>
              ) : (
                <p>没有导出逐分支执行判定；节点状态与测试数量不代替分支证明。</p>
              )}
            </details>
            <details open={initialView === "boundaries"}>
              <summary>模型诊断 {diagnostics.length}</summary>
              <ul className="diagnostics">
                {diagnostics.map((item, index) => (
                  <li className={`severity-${item.severity}`} key={index}>
                    <code>{item.code}</code>
                    <p>{item.reason}</p>
                    <small>{item.subjects.join("、")}</small>
                  </li>
                ))}
              </ul>
              {!diagnostics.length && <p className="empty">导出时没有模型诊断。</p>}
            </details>
            <details open={initialView === "impact"}>
              <summary>业务影响与历史</summary>
              {snapshot.impact ? (
                <>
                  <p>
                    与 {snapshot.impact.previous} 比较 · {snapshot.impact.scope}
                  </p>
                  <p>变化 {snapshot.impact.changed.join("、")}</p>
                  <p>待复验测试 {snapshot.impact.tests.join("、") || "未声明"}</p>
                  {Object.entries(snapshot.impact.reasons).map(([id, reasons]) => (
                    <p key={id}>
                      <code>{id}</code> {reasons.join("、")}
                    </p>
                  ))}
                </>
              ) : (
                <p>当前版本没有声明上一版本，没有可比较的差异。</p>
              )}
            </details>
          </div>
        </main>
        {dependencyMap ? (
          <DependencyInspector
            snapshot={snapshot}
            map={dependencyMap}
            node={selection.node}
            chooseNode={(node) => dispatch({ type: "lineage", value: node })}
            chooseBranch={(id) => dispatch({ type: "branch", value: id })}
          />
        ) : (
          <FactPanel
            snapshot={snapshot}
            selection={selection}
            branch={branch}
            run={run}
            fact={fact}
            trace={trace}
            campaign={campaign}
            dispatch={dispatch}
          />
        )}
        <section className="trace-panel" aria-label="执行轨迹">
          {campaign && (
            <CampaignTrail
              campaign={campaign}
              snapshot={snapshot}
              selection={selection}
              context={context}
              dispatch={dispatch}
            />
          )}
          <div className="trace-heading">
            <div>
              <span className="eyebrow">04 / 执行轨迹</span>
              <h2>{run ? "同一运行中的步骤与首次偏离" : "选择运行后查看真实轨迹"}</h2>
            </div>
            {run && (
              <div className="trace-verdict">
                <small>运行记载</small>
                <Status value={run.trace.status} />
                <small>证据</small>
                <Status value={run.evidence_status ?? "LEGACY"} raw />
                {run.trace.first_divergence && (
                  <button
                    type="button"
                    onClick={() => {
                      dispatch({ type: "actor", value: "" });
                      dispatch({ type: "entity", value: "" });
                      dispatch({ type: "step", value: run.trace.first_divergence! });
                    }}
                  >
                    定位首次偏离 · 第 {run.trace.first_divergence} 步
                  </button>
                )}
              </div>
            )}
          </div>
          {!selection.branch ? (
            <p className="empty">先审阅业务分支的条件和预期，再选择已有运行。</p>
          ) : !run ? (
            <p className="empty">
              {availableRuns.length
                ? `当前分支有 ${availableRuns.length} 个导出运行，请从顶部选择。`
                : "当前分支没有已导出的运行。NOT_RUN 不等于规则通过。"}
            </p>
          ) : (
            <>
              <p className="trace-scope">
                <code>{run.trace.run}</code> ·{" "}
                {run.environment || selection.environment || "环境未绑定"} · {run.trace.task} ·
                Runner <code>{run.runner_id || "历史未记录"}</code> · 轮次{" "}
                {run.trace.round || "未绑定"}
              </p>
              <ol className="trace-steps">
                {stepContexts.map((step) => {
                  const stepFact = factsOf(snapshot, run).find(
                    (item) => String(item.seq) === step.id,
                  );
                  const stepTrace = run.trace.steps.find((item) => item.step === step.id);
                  return (
                    <li key={step.id}>
                      <button
                        type="button"
                        disabled={!matchesStep(step, selection)}
                        aria-current={selection.step === step.id ? "step" : undefined}
                        className={step.id === run.trace.first_divergence ? "divergence" : ""}
                        onClick={() => dispatch({ type: "step", value: step.id })}
                      >
                        <span className="step-number">{step.preparation ? "准备" : step.id}</span>
                        <span>
                          <strong>
                            {step.preparation
                              ? "前态准备 · 目标业务尚未执行"
                              : (snapshot.model.data.nodes.find((node) => node.id === step.node)
                                  ?.name ??
                                (step.node || "未记录动作"))}
                          </strong>
                          <small>{step.actors.join(" · ") || "主体身份未导出"}</small>
                          <code>{step.branches?.join(" / ") || "未关联分支"}</code>
                        </span>
                        <span>
                          <small>步骤记载</small>
                          <Status value={stepFact?.verdict ?? stepTrace?.verdict ?? "NOT_RUN"} />
                          <small>证据</small>
                          <Status
                            value={stepFact?.evidence_status ?? run.evidence_status ?? "LEGACY"}
                            raw
                          />
                        </span>
                      </button>
                    </li>
                  );
                })}
              </ol>
              {run.evidence_errors?.map((error, index) => (
                <p className="notice error-notice" key={index}>
                  {error}
                </p>
              ))}
              {!stepContexts.length && <p className="empty">此运行没有记录任何动作步骤。</p>}
              <p className="muted">
                点击步骤会同步节点和已记录的分支；同一账号和业务实体始终限定在本次运行内。
              </p>
            </>
          )}
        </section>
      </div>
      <footer className="page-footer">
        <span>静态快照不操作目标系统；模型和事实由 CLI 写入。</span>
        <span>
          {snapshot.runs_truncated ? "历史导出有限" : "按导出范围审阅"} · 缺失观测保留 UNKNOWN
        </span>
      </footer>
    </div>
  );
}

function CapabilityTree({
  snapshot,
  selected,
  choose,
}: {
  snapshot: Snapshot;
  selected: string;
  choose: (id: string) => void;
}) {
  const nodes = snapshot.model.data.nodes.filter((node) =>
    ["system", "domain", "capability"].includes(node.kind),
  );
  const byID = new Map(nodes.map((node) => [node.id, node]));
  const children = new Map<string, BusinessNode[]>();
  const childIDs = new Set<string>();
  for (const edge of snapshot.model.data.edges)
    if (edge.kind === "contains" && byID.has(edge.from) && byID.has(edge.to)) {
      children.set(edge.from, [...(children.get(edge.from) ?? []), byID.get(edge.to)!]);
      childIDs.add(edge.to);
    }
  const draw = (node: BusinessNode, ancestors = new Set<string>()): React.ReactNode => {
    if (ancestors.has(node.id))
      return (
        <li key={node.id}>
          <span className="warning">包含关系成环：{node.name}</span>
        </li>
      );
    const next = new Set([...ancestors, node.id]);
    const descendants = children.get(node.id) ?? [];
    const button = (
      <button
        type="button"
        className={selected === node.id ? "tree-node selected" : "tree-node"}
        onClick={() => choose(node.id)}
      >
        <span className={`tree-dot kind-${node.kind}`} />
        <span>{node.name}</span>
      </button>
    );
    return (
      <li key={node.id}>
        {descendants.length ? (
          <details open>
            <summary>{button}</summary>
            <ul>{descendants.map((child) => draw(child, next))}</ul>
          </details>
        ) : (
          button
        )}
      </li>
    );
  };
  const roots = nodes.filter((node) => !childIDs.has(node.id));
  return (
    <div className="capability-tree">
      <ul>{(roots.length ? roots : nodes).map((node) => draw(node))}</ul>
    </div>
  );
}
