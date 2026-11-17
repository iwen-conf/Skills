import { useMemo, useState } from "react";
import { displayValue, predicateText, successorPolicyText } from "./contract";
import type { BusinessContract, ContractBranch, Predicate } from "./contract";
import type { LineageSelection } from "./lineage";
import type { BusinessLineageNode, BusinessLineageReference } from "./lineage-types";
import type { SelectionAction } from "./selection";
import type { SnapshotRun } from "./types";

interface Requirement {
  key: string;
  phase: "before" | "after";
  reference: BusinessLineageReference;
  conditions: BusinessLineageNode[];
  providers: BusinessLineageNode[];
}

/** Trace declared operands, not merely nodes with a similar display label. */
export function extractPreconditions(lineage: LineageSelection, branch?: ContractBranch, contract?: BusinessContract): Requirement[] {
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
      const entry = result.get(key) ?? { key, phase: "before" as const, reference: ref, conditions: [], providers: [] };
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
  // Outcome assertions are prerequisites for verification even when not part of the admission guard.
  // They have no declared preparation edge unless one is present in the actual lineage.
  const addExpected = (predicate: Predicate) => {
    for (const operand of [predicate.left, predicate.right]) {
      if (!operand?.ref) continue;
      const { subject, attribute } = operand.ref;
      const key = subject + "." + attribute;
      const afterKey = "after:" + key;
      if (result.has(afterKey)) continue;
      const declaredSubject = contract?.subjects.find((item) => item.id === subject);
      const declaredAttribute = declaredSubject?.attributes.find((item) => item.name === attribute);
      result.set(afterKey, {
        key, phase: "after",
        reference: {
          subject, attribute,
          subject_kind: declaredSubject?.kind ?? "未声明实体",
          source_ref: declaredAttribute?.source_ref,
        },
        conditions: [],
        providers: [],
      });
    }
    for (const child of predicate.args ?? []) addExpected(child);
  };
  if (branch) {
    for (const outcome of branch.expected.outcomes) {
      addExpected(outcome.predicate);
      for (const check of outcome.checks ?? []) addExpected(check.predicate);
    }
    for (const invariant of branch.expected.common_invariants ?? []) addExpected(invariant.predicate);
  }
  return [...result.values()].sort((a, b) =>
    a.phase.localeCompare(b.phase) || a.key.localeCompare(b.key));
}

/** Use one fully qualified occurrence; multiple matching facts are ambiguous, not PASS. */
export function selectScopedActual(
  lineage: LineageSelection,
  branchID: string | undefined,
  run?: SnapshotRun,
) {
  const overlay = lineage.overlay;
  if (!overlay || overlay.phase !== "before" || !branchID ||
      run?.trace.run !== overlay.run || run.evidence_status !== "PASS" ||
      overlay.evidence_status !== "PASS" || overlay.diagnostic) return {};
  const facts = (run.facts ?? []).filter((item) =>
    item.seq === overlay.step && item.evidence_status === "PASS");
  if (facts.length !== 1) return {};
  const actions = (facts[0].business_expectations ?? []).filter((item) =>
    item.actor === overlay.actor && item.action_index === overlay.action_index &&
    item.decision.frozen?.branch === branchID && item.decision.status === "READY");
  if (actions.length !== 1) return {};
  return { after: actions[0].after, comparison: actions[0].comparison };
}

export default function PreconditionLedger({
  lineage, branch, contract, run, dispatch,
}: {
  lineage: LineageSelection;
  branch?: ContractBranch;
  contract?: BusinessContract;
  run?: SnapshotRun;
  dispatch: (action: SelectionAction) => void;
}) {
  const requirements = useMemo(() => extractPreconditions(lineage, branch, contract), [lineage, branch, contract]);
  const [missingOnly, setMissingOnly] = useState(false);
  const { after, comparison } = selectScopedActual(lineage, branch?.id, run);
  const annotated = requirements.map((row) => {
    const observations = row.phase === "after" ? [] : row.conditions.flatMap((node) => lineage.readings.get(node.id)?.values ?? [])
      .filter((value) => value.subject + "." + value.attribute === row.key);
    const blocked = row.conditions.some((node) => lineage.readings.get(node.id)?.status === "BLOCKED");
    const evidenced = observations.filter((value) => value.actual !== undefined && value.sources.length > 0);
    const afterValue = row.phase === "after" ? after?.values[row.key] : undefined;
    const afterSources = row.phase === "after" ? (after?.sources?.[row.key] ?? []) : [];
    return { ...row, evidenced, blocked, afterValue, afterSources,
      afterVerified: afterValue !== undefined && afterSources.length > 0 };
  });
  const missing = annotated.filter((item) =>
    item.blocked || (item.phase === "after" ? !item.afterVerified : !item.evidenced.length));
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
          <strong>{requirements.length}</strong><small>字段 / 验证义务</small>
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
            {(outcome.checks ?? []).map((check) => <small key={check.id}>
              {check.kind} · {check.id} · Oracle {check.oracle} · {check.source_ref}
            </small>)}
          </article>
        )}</div>
        {!!branch.expected.common_invariants?.length && <div className="precondition-obligations">
          <strong>跨结果不变量</strong>
          {branch.expected.common_invariants.map((item) => <span key={item.id}>
            {item.id} · {item.kind} · Oracle {item.oracle} · {predicateText(item.predicate)} · {item.source_ref}
          </span>)}
        </div>}
        <div className="precondition-result">
          <strong>Actual（所选动作）</strong>
          {comparison
            ? <span>{comparison.status} · {comparison.outcome || "结果身份未唯一确定"} · {comparison.reason}
                {comparison.checks.map((check) => ` / ${check.outcome}:${check.predicate.value}`).join("")}
              </span>
            : <span>尚无可信的同发生记录执行后比对；不是 PASS</span>}
        </div>
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
        <thead><tr><th>实体字段 / 阶段</th><th>Guard 或结果断言</th><th>准备数据来源</th><th>同发生记录的实际证据</th></tr></thead>
        <tbody>{visible.map((row) => <tr key={row.phase + "/" + row.key}>
          <td><button type="button" onClick={() => dispatch({ type: "lineage", value: row.conditions[0]?.id ?? lineage.graph?.target ?? "" })}>{row.key}</button>
            <small>{row.phase === "before" ? "执行前读取" : "执行后验证"} · {row.reference.subject_kind} · {row.reference.source_ref || "属性来源未声明"}</small></td>
          <td>{!row.conditions.length && <span className="precondition-missing">{row.phase === "after" ? "执行后预期断言" : "条件声明未覆盖"}</span>}{row.conditions.filter((node) => node.predicate).map((node) =>
            <button type="button" key={node.id} onClick={() => dispatch({ type: "lineage", value: node.id })}>{predicateText(node.predicate)}</button>)}</td>
          <td>{row.providers.length ? row.providers.map((node) =>
            <button type="button" key={node.id} onClick={() => dispatch({ type: "lineage", value: node.id })}>{node.name}<small>声明候选，未证实执行</small></button>) :
            <span className="precondition-missing">{row.phase === "after" ? "执行后核验，不是准备动作" : "无已声明候选准备"}</span>}</td>
          <td>{row.phase === "after" ? row.afterVerified
              ? <div><code>{displayValue(row.afterValue)}</code>
                <small>同动作后态：{row.afterSources.join(" / ")}</small></div>
              : <span className="precondition-missing">尚无同发生记录的后态 Oracle 证据</span> :
            row.blocked ? <span className="precondition-missing">证据阻断</span> :
            row.evidenced.length ? row.evidenced.map((value, i) => <div key={i}>
              <code>{displayValue(value.actual)}</code>
              <small>{value.sources.map((source) => source.kind + ":" + source.oracle).join(" / ")}</small>
            </div>) : <span className="precondition-missing">未取得同作用域独立证据</span>}</td>
        </tr>)}</tbody>
      </table></div>}
    </section>
  );
}
