import { useMemo, useState } from "react";
import { displayValue, predicateText, successorPolicyText } from "./contract";
import type { ContractBranch } from "./contract";
import type { LineageSelection } from "./lineage";
import type { BusinessLineageNode, BusinessLineageReference } from "./lineage-types";
import type { SelectionAction } from "./selection";

interface Requirement {
  key: string;
  reference: BusinessLineageReference;
  conditions: BusinessLineageNode[];
  providers: BusinessLineageNode[];
}

/** Trace declared operands, not merely nodes with a similar display label. */
export function extractPreconditions(lineage: LineageSelection): Requirement[] {
  const graph = lineage.graph;
  if (!graph) return [];
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
  const providers = new Map<string, string[]>();
  for (const edge of graph.edges) {
    if (edge.kind !== "candidate_provider") continue;
    const ids = providers.get(edge.to) ?? [];
    ids.push(edge.from);
    providers.set(edge.to, ids);
  }
  const result = new Map<string, Requirement>();
  for (const node of graph.nodes) {
    if (!["condition", "logic", "target"].includes(node.kind)) continue;
    for (const ref of node.references ?? []) {
      const key = ref.subject + "." + ref.attribute;
      const entry = result.get(key) ?? { key, reference: ref, conditions: [], providers: [] };
      if (!entry.conditions.some((item) => item.id === node.id)) entry.conditions.push(node);
      // Never infer providers by matching attribute names across separate actions.
      for (const id of providers.get(node.id) ?? []) {
        const source = nodes.get(id);
        if (source?.kind === "preparation" && !entry.providers.some((item) => item.id === id))
          entry.providers.push(source);
      }
      result.set(key, entry);
    }
  }
  return [...result.values()].sort((a, b) => a.key.localeCompare(b.key));
}

export default function PreconditionLedger({
  lineage, branch, dispatch,
}: {
  lineage: LineageSelection;
  branch?: ContractBranch;
  dispatch: (action: SelectionAction) => void;
}) {
  const requirements = useMemo(() => extractPreconditions(lineage), [lineage]);
  const [missingOnly, setMissingOnly] = useState(false);
  const annotated = requirements.map((row) => {
    const observations = row.conditions.flatMap((node) => lineage.readings.get(node.id)?.values ?? [])
      .filter((value) => value.subject + "." + value.attribute === row.key);
    const blocked = row.conditions.some((node) => lineage.readings.get(node.id)?.status === "BLOCKED");
    const evidenced = observations.filter((value) => value.actual !== undefined && value.sources.length > 0);
    return { ...row, evidenced, blocked };
  });
  const missing = annotated.filter((item) => item.blocked || !item.evidenced.length);
  const visible = missingOnly ? missing : annotated;
  return (
    <section className="precondition-ledger" aria-label="前置数据与业务结果系谱">
      <header className="precondition-ledger-header">
        <div>
          <span className="eyebrow">SOURCE / GUARD / OUTCOME</span>
          <h3>前置数据与业务结果</h3>
          <p>候选准备不是执行记录；有字段不代表有可信的实际观察。</p>
        </div>
        <div className="precondition-metrics">
          <strong>{requirements.length}</strong><small>声明字段</small>
          <strong>{missing.length}</strong><small>缺证 / 阻断</small>
        </div>
      </header>
      {branch && <div className="precondition-expectation" aria-label="当前分支状态与预期">
        <div><small>动作 / 前态</small><b>{branch.action} / {branch.from_states.join("、") || "未声明"}</b></div>
        <div><small>前置 Guard</small><b>{predicateText(branch.when)}</b></div>
        <div><small>期望模式</small><b>{branch.expected.mode === "deterministic" ? "确定性" : "允许结果集合（不强称唯一）"}</b></div>
        <div className="precondition-outcomes">{branch.expected.outcomes.map((outcome) =>
          <article key={outcome.id}>
            <b>{outcome.id}</b>
            <span>后态：{outcome.state === "UNCHANGED" ? "不变" : outcome.state ?? "未声明"}</span>
            <span>{outcome.denied === undefined ? "拒绝规则未声明" : outcome.denied ? "拒绝" : "允许"}</span>
            <small>{predicateText(outcome.predicate)}</small>
            <small>合法后继：{successorPolicyText(outcome.next_allowed)}</small>
          </article>
        )}</div>
        {!branch.confirmed && <p className="precondition-missing">规则未确认，不能认定预期正确。</p>}
      </div>}
      <div className="precondition-toolbar">
        <span>数据来源保留实体归属、规则来源及同一次运行的独立证据。</span>
        <div role="group" aria-label="前置数据筛选">
          <button type="button" aria-pressed={!missingOnly} onClick={() => setMissingOnly(false)}>全部 {requirements.length}</button>
          <button type="button" aria-pressed={missingOnly} onClick={() => setMissingOnly(true)}>缺证 {missing.length}</button>
        </div>
      </div>
      {!requirements.length ? <p className="precondition-missing">未导出属性级前置条件；不能据此认为业务不需要前置数据。</p> :
      <div className="precondition-table-wrap"><table className="precondition-table">
        <thead><tr><th>前置字段 / 实体</th><th>声明条件</th><th>候选准备来源</th><th>实际读取证据</th></tr></thead>
        <tbody>{visible.map((row) => <tr key={row.key}>
          <td><button type="button" onClick={() => dispatch({ type: "lineage", value: row.conditions[0].id })}>{row.key}</button>
            <small>{row.reference.subject_kind} · {row.reference.source_ref || "属性来源未声明"}</small></td>
          <td>{row.conditions.filter((node) => node.predicate).map((node) =>
            <button type="button" key={node.id} onClick={() => dispatch({ type: "lineage", value: node.id })}>{predicateText(node.predicate)}</button>)}</td>
          <td>{row.providers.length ? row.providers.map((node) =>
            <button type="button" key={node.id} onClick={() => dispatch({ type: "lineage", value: node.id })}>{node.name}<small>声明候选，未证实执行</small></button>) :
            <span className="precondition-missing">无已声明候选准备</span>}</td>
          <td>{row.blocked ? <span className="precondition-missing">证据阻断</span> :
            row.evidenced.length ? row.evidenced.map((value, i) => <div key={i}>
              <code>{displayValue(value.actual)}</code>
              <small>{value.sources.map((source) => source.kind + ":" + source.oracle).join(" / ")}</small>
            </div>) : <span className="precondition-missing">未取得同作用域独立证据</span>}</td>
        </tr>)}</tbody>
      </table></div>}
    </section>
  );
}
