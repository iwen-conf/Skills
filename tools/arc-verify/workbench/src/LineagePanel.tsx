import { displayValue, predicateText } from "./contract";
import {
  lineageKindLabel,
  lineageNodeDetail,
  lineageStatusLabel,
  occurrenceKey,
  occurrenceLabel,
} from "./lineage";
import type { LineageSelection } from "./lineage";
import type { BusinessLineageNode } from "./lineage-types";
import type { SelectionAction } from "./selection";

export default function LineagePanel({
  lineage,
  node,
  dispatch,
}: {
  lineage: LineageSelection;
  node: BusinessLineageNode;
  dispatch?: (action: SelectionAction) => void;
}) {
  const reading = lineage.readings.get(node.id);
  const graph = lineage.graph!;
  const incoming = graph.edges.filter((edge) => edge.to === node.id);
  const outgoing = graph.edges.filter((edge) => edge.from === node.id);
  const overlay = lineage.overlay;
  const connections = (edges: typeof incoming, upstream: boolean) =>
    edges.map((edge) => {
      const other = graph.nodes.find((item) => item.id === (upstream ? edge.from : edge.to))!;
      return (
        <li key={edge.id}>
          <button type="button" onClick={() => dispatch?.({ type: "lineage", value: other.id })}>
            {other.name}
          </button>
          <small>
            {edge.kind === "candidate_provider"
              ? "候选来源 · 可替代"
              : edge.kind === "reads"
                ? "读取前置字段"
                : edge.kind === "expects"
                  ? "执行后预期"
                  : edge.kind === "verifies"
                    ? "执行后核验"
              : edge.kind === "depends_on"
                ? "声明先后依赖"
                : edge.kind === "logical_operand"
                  ? "逻辑子条件"
                  : "前置要求"}
          </small>
        </li>
      );
    });
  return (
    <section className="lineage-inspector" aria-label="系谱节点事实">
      <div className="lineage-inspector-heading">
        <span>{lineageKindLabel[node.kind]}</span>
        <span
          className={`lineage-status lineage-status-${reading?.status.toLowerCase() ?? "not_observed"}`}
        >
          {lineageStatusLabel(node, reading)}
        </span>
      </div>
      <h3>{node.name}</h3>
      <code>{node.id}</code>
      <dl className="lineage-definition">
        <dt>声明条件 / 效果</dt>
        <dd>{lineageNodeDetail(node)}</dd>
        {node.predicate && node.kind === "preparation" && (
          <>
            <dt>效果约束</dt>
            <dd>{predicateText(node.predicate)}</dd>
          </>
        )}
        <dt>规则来源</dt>
        <dd>
          <code>{node.source_ref || "未关联来源"}</code>
        </dd>
        {reading?.observed_state && (
          <>
            <dt>观察前态</dt>
            <dd>{reading.observed_state}</dd>
          </>
        )}
      </dl>
      {node.kind === "logic" && (
        <p className="lineage-hint">
          保留 {node.predicate?.op.toUpperCase()} 语义。OR 中一个子条件为假或 NOT
          中子条件为假，不单独构成目标阻断。
        </p>
      )}
      {node.kind === "preparation" && (
        <p className="lineage-hint">
          此处显示声明效果和当前前态中的观察；“效果已观察”不表示这个准备动作曾被执行。实际动作收据见准备账本。
        </p>
      )}
      <div className="lineage-observation">
        <h4>实际前态观察</h4>
        <p>{lineage.reason}</p>
        {overlay && (
          <>
            <p className="lineage-occurrence">
              <code>{overlay.run}</code> · {occurrenceLabel(overlay)}
            </p>
            <button
              type="button"
              className="lineage-fact-link"
              onClick={() => {
                dispatch?.({ type: "step", value: String(overlay.step) });
                dispatch?.({ type: "actor", value: overlay.actor });
                dispatch?.({ type: "occurrence", value: occurrenceKey(overlay) });
                dispatch?.({ type: "lineage", value: node.id });
              }}
            >
              定位对应步骤与证据
            </button>
          </>
        )}
        {reading?.reason && <p>{reading.reason}</p>}
        {reading?.evaluation && (
          <p>
            记录的谓词求值 <strong>{reading.evaluation.value}</strong>
            {reading.evaluation.unknown?.length
              ? ` · 未知 ${reading.evaluation.unknown.join("、")}`
              : ""}
          </p>
        )}
        {(reading?.values ?? []).map((value) => (
          <div className="lineage-value" key={`${value.subject}/${value.attribute}`}>
            <strong>
              {value.subject}.{value.attribute}
            </strong>
            <small>
              {value.subject_kind} · 绑定 {value.binding ?? "未观察"}
            </small>
            <p>
              实际值 <code>{displayValue(value.actual)}</code>
            </p>
            {value.sources.length ? (
              value.sources.map((source, index) => (
                <div className="lineage-source" key={`${source.kind}/${source.oracle}/${index}`}>
                  <span>
                    {source.kind === "request_capture"
                      ? "请求输入留档 · 非独立 Oracle"
                      : source.kind === "preparation_readback"
                        ? "准备独立回读 · 摘要属于 setup RAW"
                        : "独立 Oracle"}
                  </span>
                  <code>{source.oracle}</code>
                  {source.boundary && <small>{source.boundary}</small>}
                  {source.sha256 && <code className="lineage-digest">{source.sha256}</code>}
                </div>
              ))
            ) : (
              <span className="lineage-hint">未附观察来源，不能据此确认条件。</span>
            )}
          </div>
        ))}
      </div>
      {!!incoming.length && (
        <details open>
          <summary>上游来源 · {incoming.length}</summary>
          <ul className="lineage-connections">{connections(incoming, true)}</ul>
        </details>
      )}
      {!!outgoing.length && (
        <details open>
          <summary>下游依赖 · {outgoing.length}</summary>
          <ul className="lineage-connections">{connections(outgoing, false)}</ul>
        </details>
      )}
    </section>
  );
}
