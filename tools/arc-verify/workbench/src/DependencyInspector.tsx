import { neighborhood } from "./layout";
import { lineageKindLabel } from "./lineage";
import type { DependencyMap } from "./lineage-map";
import type { LineageBlockerReport } from "./lineage-blockers";
import type { LineageSelection } from "./lineage";
import type { Snapshot } from "./types";
import "./dependency-inspector.css";

export default function DependencyInspector({
  snapshot,
  map,
  node,
  chooseNode,
  chooseBranch,
}: {
  snapshot: Snapshot;
  map: DependencyMap;
  node: string;
  chooseNode: (id: string) => void;
  chooseBranch: (id: string) => void;
}) {
  const selected = map.graph.nodes.find((item) => item.id === node);
  const members = selected ? (map.members.get(selected.id) ?? []) : [];
  const downstream = selected
    ? neighborhood(map.graph.edges, selected.id, "downstream")
    : new Set(map.graph.nodes.map((item) => item.id));
  const impacted = new Set(
    map.graph.nodes
      .filter((item) => item.kind === "target" && downstream.has(item.id))
      .flatMap((item) => (map.members.get(item.id) ?? []).map((member) => member.branch)),
  );
  const branches = (snapshot.model.data.business_contract?.branches ?? []).filter((branch) =>
    impacted.has(branch.id),
  );
  return (
    <aside className="fact-panel dependency-inspector" aria-label="分支与事实面板">
      <div className="panel-heading">
        <span className="eyebrow">03 / 共享依赖</span>
        <h2>共享依赖与影响分支</h2>
        <p>先审阅声明关系，再进入明确分支查看独立观察。</p>
      </div>
      <p className="notice">{map.reason}</p>
      {map.warnings.map((warning) => (
        <p className="notice" key={warning}>
          {warning}
        </p>
      ))}
      {selected ? (
        <section aria-label="共享节点来源">
          <h3>{selected.name}</h3>
          <p>{selected.detail}</p>
          <p className="muted">此处共享声明不表示共享账号、业务实体或执行结果。</p>
          <details>
            <summary>原始分支节点与规则来源 · {members.length}</summary>
            <ul className="dependency-origin-list">
              {members.map((member) => (
                <li key={`${member.branch}/${member.node}`}>
                  <strong>{member.branch}</strong>
                  <span>{lineageKindLabel[member.declaration.kind]}</span>
                  <code>{member.node}</code>
                  <code>{member.source_ref}</code>
                </li>
              ))}
            </ul>
          </details>
          {(["upstream", "downstream"] as const).map((direction) => {
            const edges = map.graph.edges.filter((edge) =>
              direction === "upstream" ? edge.to === selected.id : edge.from === selected.id,
            );
            return (
              <section key={direction} className="dependency-neighbors">
                <h4>{direction === "upstream" ? "直接依赖来源" : "直接下游"}</h4>
                {!edges.length && <p className="empty">当前范围未声明对应关系。</p>}
                <ul>
                  {edges.map((edge) => {
                    const id = direction === "upstream" ? edge.from : edge.to;
                    return (
                      <li key={edge.id}>
                        <button type="button" onClick={() => chooseNode(id)}>
                          {map.graph.nodes.find((item) => item.id === id)?.name ?? id}
                          <small>
                            {edge.label}
                            {edge.alternative ? " · 可替代来源" : ""}
                          </small>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </section>
            );
          })}
        </section>
      ) : (
        <p className="empty">选择图中的条件或准备来源，查看它影响的业务分支。</p>
      )}
      <section aria-label="受影响业务分支">
        <h3>{selected ? "沿声明关系可影响的分支" : "当前范围的业务分支"}</h3>
        <p className="muted">包含候选和可替代路径；是否阻断需进入分支查看实际条件。</p>
        <ul className="dependency-branch-list">
          {branches.map((branch) => (
            <li key={branch.id}>
              <button type="button" onClick={() => chooseBranch(branch.id)}>
                <strong>
                  {snapshot.model.data.nodes.find((item) => item.id === branch.action)?.name ??
                    branch.action}
                </strong>
                <code>{branch.id}</code>
                <small>
                  前态 {branch.from_states.join(" / ")} ·{" "}
                  {branch.confirmed ? "已确认分支" : "候选分支"}
                </small>
                <span>进入分支与运行 →</span>
              </button>
            </li>
          ))}
        </ul>
        {!branches.length && <p className="empty">当前选择没有可到达的已导出分支。</p>}
      </section>
    </aside>
  );
}

const reportTitles: Record<LineageBlockerReport["status"], string> = {
  CLEAR: "本次观察未发现前态阻断",
  BLOCKED: "前态条件仍不满足",
  UNOBSERVED: "尚缺实际观察",
  DIAGNOSTIC: "证据阻断",
  UNSELECTED: "选择一次独立观察",
  LIMIT: "阻断分析超过预算",
};

export function LineageBlockerPanel({
  lineage,
  report,
  choose,
  focus,
}: {
  lineage: LineageSelection;
  report: LineageBlockerReport;
  choose: (node: string) => void;
  focus: () => void;
}) {
  const name = (id: string) => lineage.graph?.nodes.find((node) => node.id === id)?.name ?? id;
  return (
    <section
      className={`lineage-blockers blocker-${report.status.toLowerCase()}`}
      aria-label="前态阻断解释"
    >
      <header>
        <h3>{reportTitles[report.status]}</h3>
        {!!report.issues.length && (
          <button type="button" onClick={focus}>
            在图中定位阻断链
          </button>
        )}
      </header>
      <p>{report.reason}</p>
      {report.scope.run && (
        <p className="muted">
          当前观察：<code>{report.scope.run}</code> · {report.scope.actor} ·
          {report.scope.phase === "preparation"
            ? `准备回读 ${report.scope.readback_index}`
            : `第 ${report.scope.step} 步 / 动作 ${report.scope.action_index}`}
        </p>
      )}
      {report.status === "CLEAR" && (
        <p className="muted">仅解释这次历史前态观察，不代表现在仍可执行或业务结果通过。</p>
      )}
      <ol className="blocker-list">
        {report.issues.map((issue) => (
          <li key={issue.id}>
            <button type="button" onClick={() => choose(issue.node)}>
              {name(issue.node)}
            </button>
            <p>{issue.reason}</p>
            <small>
              需要 {issue.required_truth} · 观察 {issue.observed_truth}
            </small>
            <nav aria-label="阻断依赖路径">
              {issue.path.map((id, index) => (
                <button type="button" key={`${id}/${index}`} onClick={() => choose(id)}>
                  {name(id)}
                </button>
              ))}
            </nav>
            {!!issue.alternatives.length && (
              <details>
                <summary>条件组合内的不同选择</summary>
                <ul>
                  {issue.alternatives.map((alternative) => (
                    <li key={alternative.node}>
                      <button type="button" onClick={() => choose(alternative.node)}>
                        {name(alternative.node)}
                      </button>
                      <small>
                        需要 {alternative.required_truth} · 观察 {alternative.observed_truth}
                      </small>
                    </li>
                  ))}
                </ul>
              </details>
            )}
            {!!issue.providers.length && (
              <details>
                <summary>候选准备来源 · 尚需核对其前提和效果</summary>
                <ul>
                  {issue.providers.map((id) => (
                    <li key={id}>
                      <button type="button" onClick={() => choose(id)}>
                        {name(id)}
                      </button>
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </li>
        ))}
      </ol>
    </section>
  );
}
