import { displayValue, predicateText, successorPolicyText } from "./contract";
import { CampaignPanel } from "./CampaignViews";
import type { BusinessCampaign } from "./campaign";
import type { BusinessAssertionResult, BusinessExpectedCheck } from "./contract";
import { actorsOf, executionCoverageScope, projectionScope } from "./explorer-model";
import type { BranchOption } from "./explorer-model";
import { entityKey, factEntities } from "./facts";
import type {
  ActionObservation,
  PreparationExecution,
  RequestObservation,
  UnifiedStepFact,
} from "./facts";
import type { Selection, SelectionAction } from "./selection";
import { Status } from "./Status";
import type { Snapshot, SnapshotRun, TraceStep } from "./types";

function ValuesTable({
  before,
  after,
  label = "独立回读",
}: {
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  label?: string;
}) {
  const keys = [...new Set([...Object.keys(before ?? {}), ...Object.keys(after ?? {})])].sort();
  if (!keys.length) return <p className="empty">没有导出属性回读；不补写预期值。</p>;
  return (
    <div className="table-scroll">
      <table aria-label={label}>
        <thead>
          <tr>
            <th>主体属性</th>
            <th>动作前</th>
            <th>动作后</th>
          </tr>
        </thead>
        <tbody>
          {keys.map((key) => (
            <tr key={key}>
              <td>
                <code>{key}</code>
              </td>
              <td>{displayValue(before?.[key])}</td>
              <td>{displayValue(after?.[key])}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DeclaredChecks({ checks }: { checks?: BusinessExpectedCheck[] }) {
  if (!checks?.length) return null;
  return (
    <ul className="declared-checks">
      {checks.map((check) => (
        <li key={check.id}>
          <strong>
            {check.id} · {check.kind}
          </strong>
          <p>{predicateText(check.predicate)}</p>
          <small>
            独立判据 {check.oracle} · 来源 {check.source_ref}
          </small>
        </li>
      ))}
    </ul>
  );
}

function AssertionResults({ checks }: { checks?: BusinessAssertionResult[] }) {
  if (!checks?.length) return null;
  return (
    <ul className="declared-checks">
      {checks.map((check) => (
        <li key={check.id}>
          <strong>
            {check.id} · {check.evaluation.value}
          </strong>
          <p>{check.evaluation.reason}</p>
          {check.evaluation.unknown?.length ? (
            <p className="warning">缺失 {check.evaluation.unknown.join("、")}</p>
          ) : null}
          <small>
            {check.kind} · {check.oracle} · {check.source_ref}
          </small>
        </li>
      ))}
    </ul>
  );
}

function RequestCapture({ request }: { request: RequestObservation }) {
  return (
    <div className="binding">
      <strong>
        {request.actor} / 动作 {request.action_index} · {request.node}
      </strong>
      <small>来源 request-capture · 冻结前捕获的实际请求输入</small>
      {request.resolved_path && <code>{request.resolved_path}</code>}
      <dl className="facts" aria-label="实际请求输入">
        {Object.entries(request.values ?? {}).map(([key, value]) => (
          <div key={key}>
            <dt>{key}</dt>
            <dd>{displayValue(value)}</dd>
          </div>
        ))}
      </dl>
      {!Object.keys(request.values ?? {}).length && <p className="empty">没有导出请求测量字段。</p>}
      <small>
        请求 <code>{request.request_sha256}</code>
      </small>
    </div>
  );
}

function ActionCapture({
  action,
  actionIndex,
}: {
  action: ActionObservation;
  actionIndex: number | string;
}) {
  return (
    <div className="binding">
      <strong>
        {action.actor || "主体未绑定"} / 动作 {actionIndex} · {action.node || "未执行"}
      </strong>
      <span>
        principal {action.principal || "未绑定"} · HTTP {action.status_code ?? "未记录"} · exit{" "}
        {action.exit_code} · {action.denied ? "已拒绝" : "未拒绝"}
      </span>
      {action.resolved_path && <code>{action.resolved_path}</code>}
      <dl className="facts" aria-label="实际动作响应值">
        {Object.entries(action.response_values ?? {}).map(([key, value]) => (
          <div key={key}>
            <dt>{key}</dt>
            <dd>{displayValue(value)}</dd>
          </div>
        ))}
      </dl>
      {!Object.keys(action.response_values ?? {}).length && (
        <p className="empty">没有捕获声明的响应字段。</p>
      )}
      <small>
        {action.started || "未记录时间"} → {action.completed || "未结束"}
      </small>
      <details>
        <summary>动作捕获身份</summary>
        <p>
          请求 <code>{action.request_sha256 || "缺少 hash"}</code>
        </p>
        <p>
          响应 <code>{action.response_sha256 || "缺少 hash"}</code>
        </p>
      </details>
    </div>
  );
}

function PreparationFacts({
  execution,
  actor,
  evidenceStatus,
}: {
  execution: PreparationExecution;
  actor: string;
  evidenceStatus: string;
}) {
  const steps = execution.steps.filter((step) => !actor || step.receipt.action.actor === actor);
  return (
    <section className="detail-section">
      <h3>本次运行准备 · 计划与实际回读</h3>
      <p className="panel-status">
        <Status value={execution.status} raw /> 证据 <Status value={evidenceStatus} raw />
      </p>
      <p>{execution.reason}</p>
      <p>
        目标前置条件 {execution.target_ready ? "已观察到就绪" : "尚未就绪"} · 独立回读{" "}
        {execution.observation_reads} 次 · 实际{execution.parallel_executed ? "并行" : "串行"}执行
      </p>
      {execution.first_failure_index !== undefined && (
        <p className="warning">首次准备失败步骤 {execution.first_failure_index}</p>
      )}
      <p className="muted">
        汇总属于当前运行的准备阶段；分层预计效果不代表已执行。
        {actor ? ` 以下动作按主体 ${actor} 筛选。` : ""}
      </p>
      <ValuesTable
        before={execution.initial_observed?.values}
        after={execution.final_observed?.values}
        label="准备阶段实际初末回读"
      />
      <details>
        <summary>准备计划历史 · {execution.plans.length}</summary>
        {execution.plans.map((plan, i) => (
          <div className="binding" key={i}>
            <strong>
              计划 {i + 1} · {plan.status}
            </strong>
            <p>{plan.reason}</p>
            {plan.layers.map((layer) => (
              <div key={layer.index}>
                <h4>依赖层 {layer.index}</h4>
                {layer.steps.map((step) => (
                  <p key={step.id}>
                    <code>{step.id}</code> · {step.action}
                    <br />
                    预计{" "}
                    {step.predicted_effects
                      .map(
                        (effect) =>
                          `${effect.subject}.${effect.attribute} → ${displayValue(effect.value)}`,
                      )
                      .join("；")}
                  </p>
                ))}
              </div>
            ))}
            {plan.blocked?.length ? (
              <p className="warning">阻断 {plan.blocked.join("、")}</p>
            ) : null}
            {plan.gaps?.length ? <p className="warning">缺口 {plan.gaps.join("、")}</p> : null}
            <small>{plan.requires_readback ? "执行后必须回读" : "回读约束未记录"}</small>
          </div>
        ))}
      </details>
      <h4>实际执行的准备步骤 · {steps.length}</h4>
      {steps.map((step) => (
        <details key={step.index} open>
          <summary>
            准备 {step.index} · {step.planned.id} · {step.status}
          </summary>
          <p>{step.reason}</p>
          <p>
            前置条件 {step.requirements.value} · 后置回读 {step.ensures.value}
          </p>
          <p className="muted">
            回读 #{step.before_read} → #{step.after_read}
          </p>
          <ActionCapture action={step.receipt.action} actionIndex={step.index} />
          <ValuesTable
            before={step.before?.values}
            after={step.after?.values}
            label={`准备 ${step.index} 实际回读`}
          />
          {step.ensures.reason && <p>{step.ensures.reason}</p>}
        </details>
      ))}
      <details>
        <summary>准备独立回读记录 · {execution.readbacks.length}</summary>
        {execution.readbacks.map((readback) => (
          <div className="binding" key={readback.index}>
            <strong>
              #{readback.index} · {readback.phase}
            </strong>
            {readback.error ? (
              <p className="warning">{readback.error}</p>
            ) : (
              <dl className="facts">
                {Object.entries(readback.observation?.values ?? {}).map(([key, value]) => (
                  <div key={key}>
                    <dt>{key}</dt>
                    <dd>{displayValue(value)}</dd>
                  </div>
                ))}
              </dl>
            )}
          </div>
        ))}
      </details>
    </section>
  );
}

export default function FactPanel({
  snapshot,
  selection,
  branch,
  run,
  fact,
  trace,
  campaign,
  dispatch,
}: {
  snapshot: Snapshot;
  selection: Selection;
  branch?: BranchOption;
  run?: SnapshotRun;
  fact?: UnifiedStepFact;
  trace?: TraceStep;
  campaign?: BusinessCampaign;
  dispatch?: (action: SelectionAction) => void;
}) {
  const preparation = snapshot.model.data.business_contract?.preparation?.find(
    (item) => item.id === selection.node,
  );
  const node = snapshot.model.data.nodes.find(
    (item) => item.id === selection.node || item.id === preparation?.action,
  );
  const readinessView = snapshot.views.capabilities.nodes.some((item) => item.id === selection.node)
    ? snapshot.views.capabilities
    : snapshot.views.overview;
  const readiness = readinessView.nodes.find((item) => item.id === selection.node);
  const readinessScope = projectionScope(snapshot, selection, readinessView);
  const exploration = snapshot.exploration?.[selection.node];
  const contract = branch?.contract;
  const branchExecution = snapshot.contract_execution?.branches.find(
    (item) => item.branch === branch?.id,
  );
  const executionScope = executionCoverageScope(snapshot, selection);
  const decisions = (fact?.business_expectations ?? []).filter(
    (item) =>
      (!selection.actor || item.actor === selection.actor) &&
      (!item.decision.frozen ||
        !selection.branch ||
        branch?.unresolved ||
        item.decision.frozen.branch === selection.branch),
  );
  const oracles = (fact?.oracles ?? []).filter(
    (item) => !selection.actor || item.actor === selection.actor,
  );
  const conditions = (fact?.conditions ?? []).filter(
    (item) => !selection.actor || !item.observer_actor || item.observer_actor === selection.actor,
  );
  const waits = (fact?.waits ?? []).filter(
    (item) => !selection.actor || !item.actor || item.actor === selection.actor,
  );
  const entities = factEntities(fact).filter(
    (item) => !selection.entity || entityKey(item) === selection.entity,
  );
  const sessions = (fact?.sessions ?? []).filter(
    (item) => !selection.actor || item.actor === selection.actor,
  );
  const actions = (fact?.actions ?? []).filter(
    (item) => !selection.actor || item.actor === selection.actor,
  );
  const requests = (
    fact?.requests ??
    (fact?.business_expectations ?? []).flatMap((item) => (item.request ? [item.request] : []))
  ).filter((item) => !selection.actor || item.actor === selection.actor);
  const extractions = (fact?.extractions ?? []).filter(
    (item) => !selection.actor || !item.source_actor || item.source_actor === selection.actor,
  );
  const observedActors = actorsOf(fact);
  const focusedActor =
    selection.actor || (observedActors.length === 1 ? observedActors[0] : undefined);
  const identity = fact?.participants?.find((item) => item.alias === focusedActor);
  const account = identity?.account ?? (focusedActor === fact?.actor ? fact?.account : undefined);
  const principal =
    identity?.principal ?? (focusedActor === fact?.actor ? fact?.principal : undefined);
  const actorDecision = selection.actor && decisions.length === 1 ? decisions[0] : undefined;
  const canUseStepSummary = !selection.actor || selection.actor === fact?.actor;
  const actionIndex = (value?: number) =>
    value ?? ((fact?.evidence_version ?? 0) >= 2 ? 0 : "历史身份未记录");
  const preparationFact = run?.facts?.find(
    (item) => item.run === run.trace.run && item.graph === snapshot.meta.graph && item.preparation,
  );
  const factTrust = fact?.evidence_status ?? run?.evidence_status ?? "LEGACY";
  return (
    <aside className="fact-panel" aria-label="分支与事实面板">
      <div className="panel-heading">
        <span className="eyebrow">03 / 分支与事实</span>
        <h2>{node?.name ?? "选择节点查看业务含义"}</h2>
        {node && <code>{node.id}</code>}
      </div>
      {campaign && dispatch && (
        <details className="campaign-context" open={!selection.run}>
          <summary>Campaign 当前状态与候选 · {campaign.status}</summary>
          <CampaignPanel campaign={campaign} selection={selection} dispatch={dispatch} />
        </details>
      )}
      {readiness && (
        <div className="node-summary">
          <p className="muted">节点汇总范围：{readinessScope.label}</p>
          {readinessScope.appliesToSelection ? (
            <p className="panel-status">
              <small>节点汇总</small>
              <Status value={readiness.status} />
              <span>{readiness.reason}</span>
            </p>
          ) : (
            <p className="notice">节点汇总不适用于当前选择：{readinessScope.reason}。</p>
          )}
          <p className="muted">节点汇总不代替当前运行、主体或步骤的判定。</p>
        </div>
      )}
      {branch ? (
        <section className="detail-section">
          <h3>
            {branch.legacy
              ? "历史测试范围"
              : branch.unresolved
                ? "分支未解析 · 运行诊断"
                : "当前业务分支"}
          </h3>
          <code>{branch.id}</code>
          {branchExecution && (
            <div>
              <p className="muted">分支汇总范围：{executionScope.label}</p>
              {executionScope.appliesToSelection ? (
                <p className="panel-status">
                  分支实际覆盖 <Status value={branchExecution.status} raw />{" "}
                  {branchExecution.reason}
                </p>
              ) : (
                <p className="notice">{executionScope.reason}；此汇总不作为当前选择的判定。</p>
              )}
              <p className="muted">分支汇总不证明每个 Runner 输入变体均已执行。</p>
            </div>
          )}
          {contract ? (
            <>
              <p className="muted">适用状态 {contract.from_states?.join("、") || "未声明"}</p>
              <p className="predicate">
                <strong>当</strong> {predicateText(contract.when)}
              </p>
              <h4>预期操作与结果</h4>
              <p>
                {snapshot.model.data.nodes.find((item) => item.id === contract.action)?.name ??
                  contract.action}
              </p>
              <ul className="outcomes">
                {contract.expected.outcomes.map((outcome) => (
                  <li key={outcome.id}>
                    <strong>{outcome.id}</strong>
                    {outcome.state && <span>状态 → {outcome.state}</span>}
                    {outcome.denied !== undefined && (
                      <span>{outcome.denied ? "应拒绝操作" : "应允许操作"}</span>
                    )}
                    <span>{predicateText(outcome.predicate)}</span>
                    <DeclaredChecks checks={outcome.checks} />
                    <small>声明后续动作限制 {successorPolicyText(outcome.next_allowed)}</small>
                  </li>
                ))}
              </ul>
              {contract.expected.common_invariants?.length ? (
                <>
                  <h4>共同不变量</h4>
                  <DeclaredChecks checks={contract.expected.common_invariants} />
                </>
              ) : null}
              <p className="muted">
                {contract.expected.mode === "allowed_outcomes"
                  ? `已声明允许结果集合：${contract.expected.reason ?? "原因未声明"}`
                  : "确定性单一结果"}{" "}
                · {contract.confirmed ? "规则已确认" : "候选规则，尚未确认"}
              </p>
              <p className="source-line">
                来源 {contract.source} · <code>{contract.source_ref}</code>
              </p>
              <details>
                <summary>主体属性类型与来源</summary>
                {snapshot.model.data.business_contract?.subjects.map((subject) => (
                  <div className="binding" key={subject.id}>
                    <strong>
                      {subject.id} · {subject.kind}
                    </strong>
                    {subject.attributes.map((attribute) => (
                      <p key={attribute.name}>
                        <code>
                          {subject.id}.{attribute.name}
                        </code>{" "}
                        · {attribute.type}
                        {attribute.encoding ? ` / ${attribute.encoding}` : ""}
                        {attribute.encoding === "json_i64" && (
                          <small>保留 JSON 整数原始词法；不是字符串编码，判定由 Go 执行。</small>
                        )}
                        {attribute.source_ref && <small>来源 {attribute.source_ref}</small>}
                      </p>
                    ))}
                  </div>
                ))}
              </details>
            </>
          ) : (
            <>
              <p className="notice">
                {branch.unresolved
                  ? "本组运行没有可用的已持久化分支关联。保留失败与未知原因，不从实际结果补写业务预期。"
                  : "旧格式测试可回看；未声明新的确定性业务契约。"}
              </p>
              <p>{branch.legacy?.name}</p>
              <p>预期 {branch.legacy?.expected ?? "未声明"}</p>
            </>
          )}
        </section>
      ) : (
        <section className="detail-section">
          <h3>先选择业务分支</h3>
          <p className="empty">
            分支描述“条件成立时应发生什么”，即使还没有运行，也可以审阅条件和预期。
          </p>
        </section>
      )}
      {preparation && (
        <section className="detail-section">
          <h3>准备动作 · 计划声明</h3>
          <p>需要 {predicateText(preparation.requires)}</p>
          <p>确保 {predicateText(preparation.ensures)}</p>
          <p className="notice">效果为规划声明，执行后仍须独立回读。</p>
          <ul>
            {preparation.effects.map((effect, index) => (
              <li key={index}>
                <code>
                  {effect.subject}.{effect.attribute}
                </code>{" "}
                → {displayValue(effect.value)}
              </li>
            ))}
          </ul>
        </section>
      )}
      <section className="detail-section">
        <h3>所选步骤事实</h3>
        {!run ? (
          <p className="empty">尚未选择运行。模型条件不等于观测事实。</p>
        ) : !selection.step ? (
          <p className="empty">
            当前运行在所选节点和账号范围内没有对应步骤；不会显示上一个节点的事实。
          </p>
        ) : (
          <>
            <div className="scope-card">
              <code>
                {run.trace.run} / 第 {selection.step} 步
              </code>
              <span>环境 {run.environment || selection.environment || "未绑定"}</span>
              <span>
                轮次 {run.trace.round || "未绑定"} · Runner {run.runner_id || "历史未记录"}
              </span>
              {run.runner_sha256 && (
                <small>
                  Runner 指纹 <code>{run.runner_sha256}</code>
                </small>
              )}
              {focusedActor && <span>动作主体 {focusedActor}</span>}
              {account && <span>账号 {account}</span>}
              {principal && (
                <span>
                  principal <code>{principal}</code>
                </span>
              )}
            </div>
            <p className="panel-status">
              步骤汇总 <Status value={fact?.verdict ?? trace?.verdict ?? "NOT_RUN"} /> 证据{" "}
              <Status value={factTrust} raw />
            </p>
            {factTrust !== "PASS" && (
              <p className="notice">
                {factTrust === "LEGACY"
                  ? "历史格式或身份不完整：保留原记录，未升级为强证据。"
                  : "证据未通过完整校验；可诊断，不能据此点绿业务。"}
              </p>
            )}
            <div className="result-pair">
              <div>
                <span>{selection.actor ? "当前主体冻结预期" : "步骤汇总预期"}</span>
                <strong>
                  {actorDecision?.decision.frozen?.expected.outcomes
                    .map((outcome) => outcome.state ?? outcome.id)
                    .join("、") ||
                    (canUseStepSummary ? (fact?.expected_state ?? trace?.expected) : undefined) ||
                    "未记录"}
                </strong>
              </div>
              <div>
                <span>{selection.actor ? "当前主体实际观察" : "步骤汇总观察"}</span>
                <strong>
                  {actorDecision?.after?.state ||
                    (canUseStepSummary ? (fact?.observed_state ?? trace?.observed) : undefined) ||
                    "未观察"}
                </strong>
              </div>
            </div>
            <p>{fact?.reason ?? fact?.decision ?? trace?.decision}</p>
            {!fact && (
              <p className="notice">
                此运行未导出统一 step_facts。下方只展示历史 trace 和 RAW 摘要。
              </p>
            )}
          </>
        )}
      </section>
      {preparationFact?.preparation && (
        <PreparationFacts
          execution={preparationFact.preparation}
          actor={selection.actor}
          evidenceStatus={preparationFact.evidence_status ?? run?.evidence_status ?? "LEGACY"}
        />
      )}
      {decisions.map((item, index) => (
        <section className="detail-section" key={`${item.actor}/${item.action_index}/${index}`}>
          <h3>
            确定性判定 · {item.actor || "主体未绑定"} / 动作 {item.action_index}
          </h3>
          <p className="panel-status">
            <Status value={item.decision.status} raw />
            {item.decision.reason}
          </p>
          {item.decision.matches?.length > 0 && <p>匹配分支 {item.decision.matches.join("、")}</p>}
          {item.decision.unknown?.length ? (
            <p className="notice">缺失观测 {item.decision.unknown.join("、")}</p>
          ) : null}
          {item.decision.frozen && (
            <>
              <p>
                <code>{item.decision.frozen.branch}</code> · 规则{" "}
                <code>{item.decision.frozen.rule}</code>
              </p>
              <h4>执行前冻结的期望</h4>
              <ul className="outcomes">
                {item.decision.frozen.expected.outcomes.map((outcome) => (
                  <li key={outcome.id}>
                    <strong>{outcome.id}</strong>
                    <span>
                      {outcome.state ?? "无状态约束"}
                      {outcome.denied === undefined ? "" : outcome.denied ? " · 拒绝" : " · 允许"}
                    </span>
                    <span>{predicateText(outcome.predicate)}</span>
                    <DeclaredChecks checks={outcome.checks} />
                    <small>冻结后续动作限制 {successorPolicyText(outcome.next_allowed)}</small>
                  </li>
                ))}
              </ul>
              {item.decision.frozen.expected.common_invariants?.length ? (
                <>
                  <h4>冻结的共同不变量</h4>
                  <DeclaredChecks checks={item.decision.frozen.expected.common_invariants} />
                </>
              ) : null}
              <ValuesTable
                before={item.decision.frozen.before.values}
                after={item.after?.values}
                label="冻结前置条件与实际结果"
              />
              <details>
                <summary>冻结身份与绑定</summary>
                <p>
                  契约 <code>{item.decision.frozen.contract_sha256}</code>
                </p>
                <p>
                  冻结内容 <code>{item.decision.frozen.sha256}</code>
                </p>
                <dl className="facts">
                  {Object.entries(item.decision.frozen.before.bindings ?? {}).map(
                    ([key, value]) => (
                      <div key={key}>
                        <dt>{key}</dt>
                        <dd>{value}</dd>
                      </div>
                    ),
                  )}
                </dl>
              </details>
            </>
          )}
          {item.request && (
            <small>
              本次决策的请求捕获 <code>{item.request.request_sha256}</code> · {item.request.actor} /
              动作 {item.request.action_index}
            </small>
          )}
          {item.comparison && (
            <>
              <p className="panel-status">
                <Status value={item.comparison.status} raw /> {item.comparison.reason}
              </p>
              <p>实际匹配结果 {item.comparison.outcome || "未记录唯一结果身份"}</p>
              {item.comparison.unknown?.length ? (
                <p className="notice">
                  后端未确认项 {item.comparison.unknown.join("、")}
                  ；当前动作判定与后继限制分别保留。
                </p>
              ) : null}
              <ul>
                {item.comparison.checks.map((check) => (
                  <li key={check.outcome}>
                    <code>{check.outcome}</code> · 条件 {check.predicate.value} · 状态{" "}
                    {check.state_passed === undefined ? "未约束" : String(check.state_passed)} ·
                    拒绝{" "}
                    {check.denied_passed === undefined ? "未约束" : String(check.denied_passed)}
                    {check.predicate.reason && <p>{check.predicate.reason}</p>}
                    {check.predicate.unknown?.length ? (
                      <small>结果条件未确认 {check.predicate.unknown.join("、")}</small>
                    ) : null}
                    <AssertionResults checks={check.assertions} />
                  </li>
                ))}
              </ul>
              {item.comparison.invariant_checks?.length ? (
                <>
                  <h4>共同不变量回读结果</h4>
                  <AssertionResults checks={item.comparison.invariant_checks} />
                </>
              ) : null}
              <p>比较结果的后续动作限制 {successorPolicyText(item.comparison.next_allowed)}</p>
            </>
          )}
        </section>
      ))}
      {fact && (
        <>
          <section className="detail-section">
            <h3>
              实际请求输入 <small>{requests.length}</small>
            </h3>
            {requests.length ? (
              requests.map((request, index) => (
                <RequestCapture
                  key={`${request.actor}/${request.action_index}/${index}`}
                  request={request}
                />
              ))
            ) : (
              <p className="empty">未导出请求捕获；不会从期望结果补写输入。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              实际操作与响应 <small>{actions.length}</small>
            </h3>
            <p className="muted">响应字段来自动作捕获；数据库与业务实体状态见独立回读。</p>
            {actions.length ? (
              actions.map((action, index) => (
                <ActionCapture
                  key={`${action.actor}/${action.action_index ?? index}`}
                  action={action}
                  actionIndex={actionIndex(action.action_index)}
                />
              ))
            ) : (
              <p className="empty">此范围没有导出的实际动作；阻断的决策不会补写动作。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              条件与主体 <small>{conditions.length}</small>
            </h3>
            {conditions.length ? (
              <div className="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>条件 / 主体</th>
                      <th>期望</th>
                      <th>实际</th>
                    </tr>
                  </thead>
                  <tbody>
                    {conditions.map((condition, index) => (
                      <tr key={index}>
                        <td>
                          <strong>{condition.condition}</strong>
                          <code>
                            {condition.subject ?? "主体未绑定"}
                            {condition.attribute ? `.${condition.attribute}` : ""}
                          </code>
                          <small>
                            {condition.observer_actor} · {condition.oracle_id ?? "无独立回读"}
                            {condition.annotation_only ? " · 仅注释映射" : ""}
                          </small>
                        </td>
                        <td>
                          {condition.expected !== undefined
                            ? displayValue(condition.expected)
                            : String(condition.expected_bool)}
                        </td>
                        <td>
                          {condition.actual_bool === undefined ? (
                            <Status value="UNKNOWN" />
                          ) : (
                            String(condition.actual_bool)
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="empty">本步骤没有条件读数。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              业务实体 <small>{entities.length}</small>
            </h3>
            {entities.length ? (
              entities.map((entity) => (
                <div className="binding" key={entityKey(entity)}>
                  <strong>
                    {entity.alias} · {entity.kind}
                  </strong>
                  <code>{entity.id}</code>
                  <span>
                    owner {entity.owner_actor} / {entity.owner_principal}
                  </span>
                  {fact.before_entities && (
                    <small>
                      动作前
                      {fact.before_entities.some(
                        (before) => entityKey(before) === entityKey(entity),
                      )
                        ? `绑定 ${entity.id}`
                        : "未记录此身份"}{" "}
                      → 动作后
                      {fact.entities?.some((after) => entityKey(after) === entityKey(entity))
                        ? `绑定 ${entity.id}`
                        : "未记录此身份"}
                    </small>
                  )}
                  {entity.channel_account_id && <span>渠道账号 {entity.channel_account_id}</span>}
                  <small>
                    来源步骤 {entity.source_step} · {entity.source_field}
                  </small>
                </div>
              ))
            ) : (
              <p className="empty">没有导出实体绑定。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              会话状态 <small>{sessions.length}</small>
            </h3>
            {sessions.length ? (
              sessions.map((session, index) => (
                <div className="binding" key={index}>
                  <strong>
                    {session.actor} · {session.active ? "有效" : "失效"}
                  </strong>
                  <span>
                    {session.principal} / {session.scope}
                  </span>
                  {fact.before_sessions && (
                    <small>
                      动作前{" "}
                      {(() => {
                        const before = fact.before_sessions?.find(
                          (item) => item.actor === session.actor && item.scope === session.scope,
                        );
                        return before ? (before.active ? "有效" : "失效") : "未记录";
                      })()}{" "}
                      → 动作后 {session.active ? "有效" : "失效"}
                    </small>
                  )}
                  <small>
                    获得于步骤 {session.acquired_step} · digest <code>{session.digest}</code>
                  </small>
                </div>
              ))
            ) : (
              <p className="empty">没有导出会话读数。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              异步等待 <small>{waits.length}</small>
            </h3>
            {waits.length ? (
              waits.map((wait, index) => (
                <div className="binding" key={index}>
                  <strong>
                    {wait.oracle_id} · {wait.timed_out ? "已超时" : "等待已结束"}
                  </strong>
                  <span>
                    {wait.actor ?? "历史身份未绑定"} · 动作 {actionIndex(wait.action_index)}
                  </span>
                  <span>
                    {wait.attempts} 次 / 上限 {wait.max_wait_ms} ms / 稳定窗{" "}
                    {wait.stable_window_ms ?? 0} ms
                  </span>
                  <span>最终 {wait.final_state || "未观察"}</span>
                  {wait.violated_intermediate && (
                    <p className="warning">等待期间出现禁止的中间状态</p>
                  )}
                  <small>{wait.observed_states?.join(" → ")}</small>
                </div>
              ))
            ) : (
              <p className="empty">本步骤没有等待记录。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              独立判据与 DB 回读 <small>{oracles.length}</small>
            </h3>
            {oracles.length ? (
              oracles.map((oracle, index) => (
                <details key={index} open>
                  <summary>
                    {oracle.id} · {oracle.actor} / 动作 {actionIndex(oracle.action_index)}
                  </summary>
                  <p>
                    状态 {oracle.before?.state ?? "未观察"} → {oracle.after?.state ?? "未观察"}
                  </p>
                  <ValuesTable
                    before={oracle.before?.values}
                    after={oracle.after?.values}
                    label={`判据 ${oracle.id} 属性`}
                  />
                  <ValuesTable
                    before={oracle.before?.conditions ?? undefined}
                    after={oracle.after?.conditions ?? undefined}
                    label={`判据 ${oracle.id} 条件`}
                  />
                  <small>
                    边界 {oracle.boundary} · principal{" "}
                    {oracle.after?.principal || oracle.before?.principal || "未绑定"}
                  </small>
                </details>
              ))
            ) : (
              <p className="empty">未导出独立回读；操作响应不能替代数据库事实。</p>
            )}
          </section>
          <section className="detail-section">
            <h3>
              确定性断言 <small>{fact.assertions?.length ?? 0}</small>
            </h3>
            {fact.assertions?.length ? (
              fact.assertions.map((assertion, index) => (
                <div className="assertion" key={index}>
                  <p>
                    <Status value={assertion.verdict} raw /> <strong>{assertion.name}</strong>
                  </p>
                  <p>
                    {assertion.subject} · {assertion.operator ?? assertion.kind}
                  </p>
                  <div className="result-pair">
                    <div>
                      <span>预期</span>
                      {assertion.expected ?? "未声明"}
                    </div>
                    <div>
                      <span>实际</span>
                      {assertion.actual ?? "未观察"}
                    </div>
                  </div>
                  <p>{assertion.reason}</p>
                  <small>{assertion.oracle_id}</small>
                </div>
              ))
            ) : (
              <p className="empty">没有导出的断言结果。</p>
            )}
          </section>
          <details className="detail-section">
            <summary>提取绑定与证据身份</summary>
            {extractions.map((binding, index) => (
              <p key={index}>
                <code>{binding.name}</code> ={" "}
                {binding.redacted ? "已脱敏" : (binding.value ?? "未记录")} · {binding.source_actor}{" "}
                / {binding.oracle_id} / 动作 {actionIndex(binding.action_index)} ·{" "}
                <code>{binding.path}</code>
              </p>
            ))}
            <p>
              来源 {fact.evidence_source ?? "未记录"} · 版本 {fact.evidence_version ?? "历史"}
            </p>
            {fact.recovery_mode && <p>恢复记录模式 {fact.recovery_mode}</p>}
            <code>{fact.evidence_sha256 ?? "缺少证据 hash"}</code>
          </details>
        </>
      )}
      {run?.trace.recovery && (
        <section className="detail-section">
          <h3>
            {run.trace.recovery.mode === "checkpoint" ? "现场检查点 · 保留状态" : "运行恢复记录"}
          </h3>
          <p className="panel-status">
            <Status value={run.trace.recovery.verdict} raw /> {run.trace.recovery.reason}
          </p>
          <p>
            模式 {run.trace.recovery.mode || "历史未声明"} · 观察状态{" "}
            {run.trace.recovery.state || "未观察"}
          </p>
          {run.trace.recovery.mode === "checkpoint" && (
            <p className="notice">
              只读确认并保留本次运行现场，不表示环境已恢复。跨运行不继承变量、Cookie
              或会话。Campaign
              仅允许从已注册独立判据的同一测量字段路径重新读取并绑定非敏感实体；缺少新鲜回读时阻断。
            </p>
          )}
          <small>
            记录的安全标记 {String(run.trace.recovery.safe)} ·{" "}
            <code>{run.trace.recovery.sha256 || "未导出 hash"}</code>
          </small>
        </section>
      )}
      {run && selection.step && (
        <details className="detail-section">
          <summary>RAW 导出校验与夹具</summary>
          {run.evidence
            .filter((entry) => entry.phase !== "action" || String(entry.step) === selection.step)
            .map((entry, index) => (
              <div key={index} className={entry.error ? "evidence-failed" : "binding"}>
                <strong>
                  {entry.phase === "fixture"
                    ? "夹具恢复"
                    : entry.phase === "setup"
                      ? "隔离检查"
                      : `第 ${entry.step} 步`}
                  {entry.error ? "：导出时校验失败" : ""}
                </strong>
                {entry.error ? (
                  <p>{entry.error}</p>
                ) : (
                  <>
                    <code>{entry.sha256}</code>
                    <p>
                      期望 {entry.data?.expected ?? "未记录"} · 观察{" "}
                      {entry.data?.observed ?? "未记录"} · 判据 {entry.data?.oracle ?? "未记录"}
                    </p>
                  </>
                )}
              </div>
            ))}
        </details>
      )}
      {exploration && (
        <details className="detail-section">
          <summary>探索与验证 · 保留历史</summary>
          <p>
            业务验证状态 <Status value={exploration.business_verdict ?? "NOT_RUN"} />
          </p>
          <p>证据可信状态 {exploration.evidence_trust ?? "UNVERIFIED"}</p>
          <p>
            节点义务{" "}
            {readinessScope.appliesToSelection
              ? readiness?.readiness.verified
                ? "已验证"
                : "尚未验证"
              : "当前范围无适用汇总"}{" "}
            · {exploration.tested ? "已测试" : "尚未测试"}
          </p>
          {Object.entries(exploration)
            .filter(([, value]) => Array.isArray(value))
            .map(([key, value]) => (
              <p key={key}>
                <code>{key}</code> {(value as string[]).join("、") || "无"}
              </p>
            ))}
          <p>
            {exploration.budget_exhausted ? "预算耗尽" : ""} {exploration.evidence_missing}
          </p>
        </details>
      )}
    </aside>
  );
}
