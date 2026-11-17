import type { BusinessCampaign, BusinessCampaignCandidate } from "./campaign";
import { isCampaignDiagnostic } from "./campaign";
import { displayValue, predicateText, successorPolicyText } from "./contract";
import type { BusinessObservation } from "./contract";
import { entityKey } from "./facts";
import { reduceSelection } from "./selection";
import type { Selection, SelectionAction, SelectionContext } from "./selection";
import { Status } from "./Status";
import type { Snapshot } from "./types";
import "./campaign.css";

const eligibilityNames: Record<string, string> = {
  READY: "已满足候选条件",
  PREPARABLE: "需要准备与回读",
  NOT_APPLICABLE: "当前前态不适用",
  BLOCKED: "证据或环境阻塞",
  MODEL_ERROR: "模型冲突",
  SPECIFICATION_GAP: "规格缺口",
  BUDGET: "预算不足",
  NEXT_DISALLOWED: "后续动作限制",
};

function ObservedState({ value, label }: { value?: BusinessObservation; label: string }) {
  if (!value) return <p className="empty">{label}未导出；不使用期望值补齐。</p>;
  return (
    <div className="campaign-observation" aria-label={label}>
      <h4>{label}</h4>
      <p>
        状态 <code>{value.state || "未观察"}</code> · 流程 {value.flow || "未记录"}
      </p>
      <dl className="facts">
        {Object.entries(value.values ?? {}).map(([key, actual]) => (
          <div key={key}>
            <dt>{key}</dt>
            <dd>{displayValue(actual)}</dd>
          </div>
        ))}
      </dl>
      {!Object.keys(value.values ?? {}).length && <p className="empty">没有记录主体属性。</p>}
      {(Object.keys(value.bindings ?? {}).length > 0 ||
        Object.keys(value.sources ?? {}).length > 0) && (
        <details>
          <summary>本次观察的主体绑定与来源</summary>
          {Object.entries(value.bindings ?? {}).map(([key, binding]) => (
            <p key={key}>
              <code>{key}</code> → {binding}
            </p>
          ))}
          {Object.entries(value.sources ?? {}).map(([key, sources]) => (
            <p key={key}>
              <code>{key}</code> · {sources.join("、")}
            </p>
          ))}
        </details>
      )}
    </div>
  );
}

function CandidateDetails({ candidate }: { candidate: BusinessCampaignCandidate }) {
  const decision = candidate.decision;
  return (
    <div className="candidate-details">
      <p className="scope-card">
        <strong>{candidate.variant_id}</strong>
        <span>分支 {candidate.branch}</span>
        <span>Runner {candidate.runner}</span>
      </p>
      <p>
        <span className={`eligibility eligibility-${candidate.eligibility.toLowerCase()}`}>
          {eligibilityNames[candidate.eligibility] ?? candidate.eligibility}
        </span>{" "}
        <code>{candidate.eligibility}</code>
      </p>
      <p>{candidate.reason}</p>
      <p className="muted">
        候选为导出时的只读记录。预分配运行 ID {candidate.run || "未分配"} 不证明已执行；selected=
        {String(candidate.selected)} 仅表示规划选择。
      </p>
      <ObservedState value={candidate.before} label="候选回读的实际前态" />
      {candidate.observation?.participants?.map((participant) => (
        <p className="binding" key={participant.alias}>
          <strong>观察主体 {participant.alias}</strong>
          <span>
            账号 {participant.account} · principal {participant.principal}
          </span>
        </p>
      ))}
      {candidate.input && (
        <details open>
          <summary>候选的实际请求输入</summary>
          <p>
            {candidate.input.actor} / 动作 {candidate.input.action_index} · {candidate.input.node}
          </p>
          <dl className="facts">
            {Object.entries(candidate.input.values ?? {}).map(([key, value]) => (
              <div key={key}>
                <dt>{key}</dt>
                <dd>{displayValue(value)}</dd>
              </div>
            ))}
          </dl>
          <small>
            request-capture · <code>{candidate.input.request_sha256}</code>
          </small>
        </details>
      )}
      {decision && (
        <details open>
          <summary>后端候选决策 · {decision.status}</summary>
          <p>{decision.reason}</p>
          <p>匹配分支 {decision.matches?.join("、") || "未匹配"}</p>
          {!!decision.unknown?.length && (
            <p className="warning">缺少观察 {decision.unknown.join("、")}</p>
          )}
          {decision.frozen && (
            <>
              <p>
                候选期望分支 {decision.frozen.branch} · {decision.frozen.expected.mode}
              </p>
              <ul className="outcomes">
                {decision.frozen.expected.outcomes.map((outcome) => (
                  <li key={outcome.id}>
                    <strong>{outcome.id}</strong>
                    <span>
                      {outcome.state || "无状态约束"} · {predicateText(outcome.predicate)}
                    </span>
                  </li>
                ))}
              </ul>
            </>
          )}
          <p className="muted">这项候选决策不代替子运行中的耐久冻结收据、真实动作与实际比较。</p>
        </details>
      )}
      {candidate.preparation && (
        <details open>
          <summary>准备依赖计划 · {candidate.preparation.status}</summary>
          <p>{candidate.preparation.reason}</p>
          <ObservedState value={candidate.preparation_before} label="准备规划前的独立观察" />
          {candidate.preparation_observation && (
            <details>
              <summary>准备观察的来源身份</summary>
              <p>
                {candidate.preparation_observation.node} · 轮次{" "}
                {candidate.preparation_observation.round}
              </p>
              <p>
                Runner <code>{candidate.preparation_observation.runner_sha256}</code>
              </p>
              {candidate.preparation_observation.participants?.map((participant) => (
                <p key={participant.alias}>
                  {participant.alias} · {participant.account} / {participant.principal}
                </p>
              ))}
              {candidate.preparation_observation.error && (
                <p className="warning">{candidate.preparation_observation.error}</p>
              )}
            </details>
          )}
          <p className="notice">
            这是预计准备路径；执行与独立回读必须查看子运行。requires_readback=
            {String(candidate.preparation.requires_readback)}
          </p>
          {(candidate.preparation.layers ?? []).map((layer) => (
            <div className="binding" key={layer.index}>
              <strong>依赖层 {layer.index}</strong>
              {(layer.steps ?? []).map((step) => (
                <div key={step.id}>
                  <p>
                    <code>{step.id}</code> · {step.action}
                  </p>
                  <small>
                    需要 {predicateText(step.requires)}；确保 {predicateText(step.ensures)}
                  </small>
                  {step.predicted_effects?.map((effect, index) => (
                    <p key={index}>
                      预计 {effect.subject}.{effect.attribute} → {displayValue(effect.value)}
                    </p>
                  ))}
                </div>
              ))}
            </div>
          ))}
          {!!candidate.preparation.blocked?.length && (
            <p>阻塞 {candidate.preparation.blocked.join("、")}</p>
          )}
          {!!candidate.preparation.gaps?.length && (
            <p>缺口 {candidate.preparation.gaps.join("、")}</p>
          )}
        </details>
      )}
    </div>
  );
}

export function CampaignPanel({
  campaign,
  selection,
  dispatch,
}: {
  campaign: BusinessCampaign;
  selection: Selection;
  dispatch: (action: SelectionAction) => void;
}) {
  const visit = campaign.visits?.find((item) => item.run === selection.run);
  const variant = campaign.variants?.find((item) => item.id === selection.variant);
  const candidate = campaign.frontier?.find((item) => item.variant_id === selection.variant);
  const visibleCandidates = (campaign.frontier ?? []).filter(
    (item) => !selection.branch || item.branch === selection.branch,
  );
  const visitActorMatches = !selection.actor || visit?.actor === selection.actor;
  if (isCampaignDiagnostic(campaign))
    return (
      <section className="detail-section campaign-panel" aria-label="Campaign 当前状态与候选">
        <h3>Campaign 证据诊断</h3>
        <code>{campaign.id}</code>
        <p className="panel-status">
          <Status value="BLOCKED" raw /> {campaign.reason}
        </p>
        <div className="scope-card">
          <span>环境 {campaign.environment}</span>
          <span>
            流程 {campaign.flow} · 轮次 {campaign.round}
          </span>
        </div>
        <p className="notice error-notice">
          账本校验未通过；这里只保留可信记录身份。候选、子运行、预算消耗和覆盖结论均不可用。
        </p>
        <p className="muted">其他已导出历史仍可选择；不会把缺失细节或零值记为通过。</p>
      </section>
    );
  return (
    <section className="detail-section campaign-panel" aria-label="Campaign 当前状态与候选">
      <h3>
        有界业务探索 <small>Campaign</small>
      </h3>
      <code>{campaign.id}</code>
      <p className="campaign-stop">
        执行状态 <strong>{campaign.status}</strong>
      </p>
      <p>{campaign.reason}</p>
      <p>
        账本证据 <Status value={campaign.evidence_status} raw />
      </p>
      <p className="muted">执行状态表示推进或停止原因，不是业务 PASS，也不代表完整业务 E2E。</p>
      {campaign.evidence_status !== "PASS" && (
        <p className="notice error-notice">Campaign 证据未通过导出时校验；以下记录只用于诊断。</p>
      )}
      <div className="scope-card">
        <span>环境 {campaign.environment}</span>
        <span>
          流程 {campaign.flow} · 轮次 {campaign.round}
        </span>
        <span>
          修订 {campaign.revision} · {campaign.updated}
        </span>
      </div>
      <h4>预算与实际消耗</h4>
      <dl className="campaign-budget">
        <div>
          <dt>动作尝试</dt>
          <dd>
            {campaign.spent.actions} + 预留 {campaign.spent.reserved_actions} /{" "}
            {campaign.budget.max_actions}
          </dd>
        </div>
        <div>
          <dt>目标 / 准备 / 恢复</dt>
          <dd>
            {campaign.spent.target_actions} / {campaign.spent.preparation_actions} /{" "}
            {campaign.spent.recovery_actions}
          </dd>
        </div>
        <div>
          <dt>观察上下文 / 上限</dt>
          <dd>
            {campaign.spent.states} / {campaign.budget.max_states}
          </dd>
        </div>
        <div>
          <dt>耗时 / 截止预算</dt>
          <dd>
            {campaign.spent.elapsed_ms} / {campaign.budget.deadline_ms} ms
          </dd>
        </div>
        <div>
          <dt>派发 / 候选 probe 次数</dt>
          <dd>
            {campaign.spent.dispatches} / {campaign.spent.observations}
          </dd>
        </div>
      </dl>
      <p className="muted">预留动作仍占预算；未完成或未知执行不被视为已释放。</p>
      <p className="muted">
        观察上下文按完整候选 probe
        与子运行记录累计；不同运行、历史、输入或绑定不合并为一个状态标签。
      </p>
      <h4>本次选择的覆盖范围</h4>
      <p>
        输入变体：已尝试 {campaign.coverage.attempted_variants} /{" "}
        {campaign.coverage.selected_variants}，通过 {campaign.coverage.passed_variants}。
      </p>
      <p>
        所选变体覆盖：
        {campaign.coverage.selected_variants_covered ? "已完成所选范围" : "尚未完成所选范围"}
        。全业务覆盖标记：<strong>{String(campaign.coverage.full_business_coverage)}</strong>。
      </p>
      <p>
        绑定分支 {campaign.coverage.bound_branches} / 声明分支 {campaign.coverage.declared_branches}
      </p>
      {!!campaign.coverage.unmapped_branches?.length && (
        <p className="notice">
          未绑定输入变体的分支 {campaign.coverage.unmapped_branches.join("、")}
        </p>
      )}
      <p className="muted">同分支不同 Runner 输入变体分别记账；已选变体覆盖不升级为全业务证明。</p>
      <h4>
        后续动作限制 <code>next_allowed</code>
      </h4>
      <p>{successorPolicyText(campaign.next_allowed)}</p>
      <label className="campaign-variant-select">
        输入变体
        <select
          aria-label="Campaign 输入变体"
          value={selection.variant}
          onChange={(event) => dispatch({ type: "variant", value: event.target.value })}
        >
          <option value="">选择变体，查看本次前态与资格</option>
          {(campaign.variants ?? []).map((item) => (
            <option key={item.id} value={item.id}>
              {item.id} · {item.branch} · {item.runner}
            </option>
          ))}
        </select>
      </label>
      {variant && (
        <p className="muted">
          已选 {variant.id} · Runner {variant.runner}
          {variant.runner_sha256 && <code>{variant.runner_sha256}</code>}
        </p>
      )}
      {selection.run ? (
        visit ? (
          <div className="selected-campaign-visit">
            <h4>所选子运行的观察</h4>
            <code>{visit.run}</code>
            <p>
              第 {visit.sequence} 次 · {visit.variant_id} · 主体 {visit.actor || "未记录"}
            </p>
            <p className="muted">这里只显示该子运行；当前候选前态需重新选择输入变体查看。</p>
            {visitActorMatches ? (
              <>
                <ObservedState value={visit.before} label="所选子运行目标动作的实际前态" />
                <ObservedState value={visit.after} label="所选子运行目标动作的实际后态" />
                <p>principal {visit.principal || "未记录"}</p>
                {(visit.entities ?? [])
                  .filter((entity) => !selection.entity || entityKey(entity) === selection.entity)
                  .map((entity) => (
                    <p className="binding" key={entityKey(entity)}>
                      <strong>
                        {entity.alias} · {entity.id}
                      </strong>
                      <span>
                        {entity.owner_actor} / {entity.owner_principal}
                      </span>
                    </p>
                  ))}
              </>
            ) : (
              <p className="notice">
                所选主体与此子运行摘要主体不相同；具体读数请看下面的步骤事实。
              </p>
            )}
            {visit.comparison && (
              <p>
                此子运行的比较 <Status value={visit.comparison.status} raw />{" "}
                {visit.comparison.reason}
              </p>
            )}
            {visit.recovery && (
              <p>
                结束处理 {visit.recovery.mode || "历史未记录"} · {visit.recovery.verdict} ·{" "}
                {visit.recovery.reason}
                {visit.recovery.mode === "checkpoint" && (
                  <small>只读检查点保留现场，不表示恢复到了初态。</small>
                )}
                {visit.recovery.mode === "restore" && (
                  <small>
                    恢复边界：目标动作后还执行了恢复；后续子运行须重新观测，不表示同一实体继续。
                  </small>
                )}
                {visit.recovery.state && <small>结束处理后的独立观察 {visit.recovery.state}</small>}
              </p>
            )}
          </div>
        ) : (
          <p className="notice">所选运行没有此 Campaign 的显式子运行关联。</p>
        )
      ) : candidate ? (
        <CandidateDetails candidate={candidate} />
      ) : selection.variant ? (
        <p className="empty">此输入变体没有导出的候选前态；可从下方连续轨迹选择已有子运行。</p>
      ) : (
        <>
          <p className="muted">
            最近目标动作的实际后态来自 {campaign.last_run || "尚未记录子运行"}
            。该子运行随后可能执行恢复，或以 checkpoint 只读确认现场；当前状态仍以后续独立 probe
            为准。
          </p>
          <ObservedState value={campaign.last_actual} label="最近目标动作的实际后态" />
        </>
      )}
      <h4>
        当前分支的候选资格 <small>{visibleCandidates.length}</small>
      </h4>
      <ul className="candidate-list">
        {visibleCandidates.map((item) => (
          <li key={item.variant_id}>
            <button
              type="button"
              aria-pressed={selection.variant === item.variant_id && !selection.run}
              onClick={() => dispatch({ type: "variant", value: item.variant_id })}
            >
              <strong>{item.variant_id}</strong>
              <span>
                {item.branch} · {item.runner}
              </span>
              <span className={`eligibility eligibility-${item.eligibility.toLowerCase()}`}>
                {eligibilityNames[item.eligibility] ?? item.eligibility}
              </span>
              <small>{item.reason}</small>
              <small>next_allowed={String(item.next_allowed)} · 只读查看前态</small>
            </button>
          </li>
        ))}
      </ul>
      {!visibleCandidates.length && (
        <p className="empty">此分支没有已导出的候选评估；不补写资格。</p>
      )}
      <details className="campaign-provenance">
        <summary>Campaign 启动身份与来源</summary>
        <p>
          预动作派发意图 {campaign.dispatches?.length ?? 0} 条；派发意图不代表执行完成，子运行事实以
          visits 为准。
        </p>
        <p>
          启动主体 {campaign.identity.actor} · 账号 {campaign.identity.account} · principal{" "}
          {campaign.identity.principal}
        </p>
        <p>
          源码 {campaign.identity.source_commit} · <code>{campaign.identity.source_sha256}</code>
        </p>
        <p>
          配置 <code>{campaign.identity.config_sha256}</code>
        </p>
        <p>
          模型 <code>{campaign.identity.graph_sha256}</code>
        </p>
        <p>
          历史 <code>{campaign.history_sha256}</code>
        </p>
        <p className="muted">
          该身份属于 Campaign 的启动范围；子运行与当前步骤仍各自显示实际参与主体。
        </p>
      </details>
    </section>
  );
}

export function CampaignTrail({
  campaign,
  snapshot,
  selection,
  context,
  dispatch,
}: {
  campaign: BusinessCampaign;
  snapshot: Snapshot;
  selection: Selection;
  context: SelectionContext;
  dispatch: (action: SelectionAction) => void;
}) {
  const visits = [...(campaign.visits ?? [])].sort((a, b) => a.sequence - b.sequence);
  if (isCampaignDiagnostic(campaign))
    return (
      <section className="campaign-trail" aria-label="Campaign 连续子运行证据">
        <h3>Campaign → 子运行证据不可用</h3>
        <code>{campaign.id}</code>
        <p className="notice error-notice">
          账本校验失败，子运行列表已扣留；不能据此判断是否曾执行过动作。
        </p>
      </section>
    );
  return (
    <section className="campaign-trail" aria-label="Campaign 连续子运行证据">
      <div className="trace-heading">
        <div>
          <h3>Campaign → 连续子运行</h3>
          <p>
            <code>{campaign.id}</code> · {campaign.environment} / {campaign.flow} / {campaign.round}
          </p>
        </div>
        <span>{visits.length} 个已记载子运行</span>
      </div>
      {campaign.first_divergence && (
        <p className="notice error-notice">
          保留首次偏离：<code>{campaign.first_divergence}</code>
        </p>
      )}
      <p className="muted">
        每行保留独立变体、Runner 与子运行身份。点击定位会清空其他运行的账号、实体和步骤筛选。
      </p>
      <p className="muted">
        子运行之间不继承变量、Cookie
        或会话；实体连续性须由已注册独立判据新鲜回读，不能从相同显示别名推断。
      </p>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>次序 / 输入变体</th>
              <th>分支 / Runner</th>
              <th>目标动作前态 → 后态</th>
              <th>运行记载 / 证据</th>
              <th>子运行定位</th>
            </tr>
          </thead>
          <tbody>
            {visits.map((visit, index) => {
              const run = snapshot.runs.find((item) => item.trace.run === visit.run);
              const next = reduceSelection(selection, { type: "visit", value: visit.run }, context);
              const available = next.run === visit.run && next.campaign === campaign.id;
              return (
                <tr
                  key={visit.sequence}
                  aria-current={selection.run === visit.run ? "true" : undefined}
                >
                  <td>
                    <strong>
                      {visit.sequence} · {visit.variant_id}
                    </strong>
                    {index > 0 && visit.sequence !== visits[index - 1].sequence + 1 && (
                      <small className="warning">中间次序未导出</small>
                    )}
                    <small>{visit.actor || "主体未记录"}</small>
                  </td>
                  <td>
                    <code>{visit.branch}</code>
                    <small>{visit.runner}</small>
                  </td>
                  <td>
                    <span>
                      {visit.before?.state || "未观察"} → {visit.after?.state || "未观察"}
                    </span>
                    {visit.recovery && (
                      <small>
                        结束处理 {visit.recovery.mode || "历史未记录"} · {visit.recovery.verdict}
                        {visit.recovery.state ? ` · 处理后观察 ${visit.recovery.state}` : ""}
                      </small>
                    )}
                    {visit.recovery?.mode === "restore" && (
                      <small className="warning">
                        恢复边界 · 后续须重新回读，不推断同一实体连续
                      </small>
                    )}
                  </td>
                  <td>
                    <span>
                      记载 <Status value={visit.status} raw />
                    </span>
                    <span>
                      子运行证据 <Status value={run?.evidence_status ?? "NOT_EXPORTED"} raw />
                    </span>
                    <small>{visit.reason}</small>
                  </td>
                  <td>
                    <button
                      type="button"
                      disabled={!available}
                      aria-label={`定位子运行 ${visit.run}`}
                      onClick={() => dispatch({ type: "visit", value: visit.run })}
                    >
                      {selection.run === visit.run ? "当前子运行" : "查看子运行"}
                    </button>
                    <code>{visit.run}</code>
                    {!available && <small className="warning">子运行未导出或范围身份不匹配</small>}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {!visits.length && (
        <p className="empty">没有已导出的子运行记录；预分配运行 ID 不能单独证明执行。</p>
      )}
    </section>
  );
}
