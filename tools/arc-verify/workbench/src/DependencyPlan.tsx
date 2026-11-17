import { useRef, useState } from "react";
import { etaReadPlan, type ProofState } from "./eta-read-plan";
import "./eta-plan.css";
import type { CanvasGraph, CanvasNode } from "./graph-model";
import { lineageKindLabel, lineageStatusLabel } from "./lineage";

export interface DependencyPlanProps {
  graph: CanvasGraph;
  levels: ReadonlyMap<string, number>;
  selected: ReadonlySet<string>;
  onSelectNode: (node: CanvasNode) => void;
}

/** A responsive reading projection; group membership never combines runtime observations. */
export default function DependencyPlan({
  graph,
  levels,
  selected,
  onSelectNode,
}: DependencyPlanProps) {
  const [query, setQuery] = useState("");
  const [proofFilter, setProofFilter] = useState<"ALL" | ProofState>("ALL");
  const plan = etaReadPlan(graph);
  const phrase = query.trim().toLocaleLowerCase();
  const include = (node: CanvasNode) =>
    (!phrase ||
      [node.name, node.id, node.detail ?? ""].some((text) =>
        text.toLocaleLowerCase().includes(phrase),
      )) &&
    (proofFilter === "ALL" || plan.state.get(node.id) === proofFilter);
  const attentionIndex = plan.attention ? plan.critical.indexOf(plan.attention) : 0;
  const shownCritical = plan.critical.length <= 48
    ? plan.critical
    : [...new Set([
        ...plan.critical.slice(0, 8),
        ...plan.critical.slice(
          Math.max(8, attentionIndex - 4),
          Math.min(plan.critical.length - 4, attentionIndex + 24),
        ),
        ...plan.critical.slice(-4),
      ])];
  const hiddenCritical = plan.critical.length - shownCritical.length;
  const groupElements = useRef(new Map<string, HTMLDetailsElement>());
  const nodeElements = useRef(new Map<string, HTMLButtonElement>());
  const byID = new Map(graph.nodes.map((node) => [node.id, node]));
  const assigned = new Set<string>();
  const groups = (graph.groups ?? []).flatMap((group) => {
    const members = [...new Set(group.members)].filter((id) => byID.has(id));
    members.forEach((id) => assigned.add(id));
    const shown = members.filter((id) => include(byID.get(id)!));
    return shown.length ? [{ ...group, members: shown }] : [];
  });
  const ungrouped = graph.nodes.filter((node) => !assigned.has(node.id) && include(node));
  if (ungrouped.length)
    groups.push({
      id: "arc:ungrouped",
      label: groups.length ? "未分组节点" : "当前分支依赖",
      members: ungrouped.map((node) => node.id),
    });
  const incoming = new Map(graph.nodes.map((node) => [node.id, [] as CanvasGraph["edges"]]));
  const outgoing = new Map(graph.nodes.map((node) => [node.id, [] as CanvasGraph["edges"]]));
  for (const edge of graph.edges) {
    if (!byID.has(edge.from) || !byID.has(edge.to)) continue;
    incoming.get(edge.to)!.push(edge);
    outgoing.get(edge.from)!.push(edge);
  }
  const choose = (node: CanvasNode) => {
    for (const group of groups) {
      if (!group.members.includes(node.id)) continue;
      const element = groupElements.current.get(group.id);
      if (element) element.open = true;
    }
    const element = nodeElements.current.get(node.id);
    element?.scrollIntoView({ block: "nearest", inline: "nearest" });
    element?.focus({ preventScroll: true });
    onSelectNode(node);
  };
  const nodeStatus = (node: CanvasNode) =>
    node.lineage
      ? lineageStatusLabel(node.lineage, node.reading)
      : node.status === "NOT_OBSERVED"
        ? "未观察"
        : node.status;
  return (
    <section className="dependency-plan" aria-label="业务分组系谱">
      <div className="dependency-plan-heading">
        <strong>依赖系谱分层目录</strong>
        <span>按业务分组查看，关系层级在各组间保持一致</span>
      </div>
      <p className="dependency-plan-note">
        点击节点查看条件与证据；展开关系可追到其他业务组。同层表示关系深度，不保证动作可并行。
      </p>
      <section className="eta-plan-overview" aria-label="ETA式依赖计划概览">
        <div className="eta-plan-summary" role="status">
          <strong>{graph.nodes.length} 个节点</strong>
          <span>{graph.edges.length} 条声明关系</span>
          <span>{plan.counts.OBSERVED} 项独立观察符合</span>
          <span>{plan.counts.UNOBSERVED} 项待观察</span>
          <span>{plan.counts.MISMATCH + plan.counts.BLOCKED} 项不符合或证据阻断</span>
        </div>
        <p>观察符合不等于任务完成、已执行准备动作或业务 PASS。同一依赖层不证明可以并行。</p>
        <div className="eta-plan-controls">
          <input
            type="search"
            aria-label="搜索依赖节点"
            placeholder="搜索条件、准备动作、业务节点"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <select
            aria-label="筛选依赖观察状态"
            value={proofFilter}
            onChange={(event) => setProofFilter(event.target.value as "ALL" | ProofState)}
          >
            <option value="ALL">全部观察状态</option>
            <option value="UNOBSERVED">尚未独立观察</option>
            <option value="MISMATCH">观察结果不符合</option>
            <option value="BLOCKED">证据阻断</option>
            <option value="OBSERVED">独立观察符合</option>
          </select>
        </div>
      </section>
      {!plan.valid && <p className="notice error-notice" role="alert">{plan.reason}</p>}
      {plan.valid && plan.critical.length > 1 && (
        <nav className="eta-plan-critical" aria-label="当前需关注的声明关键链">
          <h3>ETA 式关键依赖链 · {plan.critical.length} 个节点</h3>
          <p>{plan.reason} {plan.attention ? "本次先关注尚未验证的来源。" : "该链全部具有符合的观察；不代表业务已通过。"}</p>
          <ol>
            {shownCritical.map((id) => {
              const node = byID.get(id)!;
              return (
                <li key={id}>
                  <button
                    type="button"
                    data-attention={id === plan.attention ? "true" : undefined}
                    onClick={() => choose(node)}
                    title={node.detail}
                  >
                    {node.name}
                  </button>
                </li>
              );
            })}
          </ol>
          {hiddenCritical > 0 && (
            <p>为保持可读性，仅显示关键链中的 {shownCritical.length} 个节点；其余 {hiddenCritical} 个节点仍保留在完整分层目录中。</p>
          )}
        </nav>
      )}
      <div aria-label="依赖系谱分层目录">
        {groups.map((group) => {
          const members = group.members.map((id) => byID.get(id)!);
          const levelMembers = new Map<number, CanvasNode[]>();
          for (const node of members) {
            const level = levels.get(node.id) ?? 0;
            const bucket = levelMembers.get(level) ?? [];
            bucket.push(node);
            levelMembers.set(level, bucket);
          }
          const groupLevels = [...levelMembers.keys()].sort((a, b) => a - b);
          return (
            <details
              key={group.id}
              className="dependency-plan-group"
              data-plan-group={group.id}
              open
              ref={(element) => {
                if (element) groupElements.current.set(group.id, element);
                else groupElements.current.delete(group.id);
              }}
            >
              <summary>
                <strong>{group.label}</strong>
                <span>{members.length} 个节点</span>
              </summary>
              <div className="eta-plan-group-progress">
                <progress
                  max={members.length}
                  value={members.filter((node) => plan.state.get(node.id) === "OBSERVED").length}
                  aria-label={group.label + "独立观察符合的节点比例"}
                />
                <span>
                  {members.filter((node) => plan.state.get(node.id) === "OBSERVED").length}
                  / {members.length} 项观察符合
                </span>
                <span>不是任务完成进度</span>
              </div>
              <ol className="dependency-plan-levels" aria-label={`${group.label}依赖层级`}>
                {groupLevels.map((level) => (
                  <li key={level} className="dependency-plan-level" data-plan-level={level + 1}>
                    <h3>
                      <span>{level + 1}</span> 依赖层 {level + 1}
                    </h3>
                    <ul className="dependency-plan-nodes">
                      {levelMembers.get(level)!.map((node) => (
                        <li
                          key={node.id}
                          data-plan-node={node.id}
                          className={`dependency-plan-node${selected.has(node.id) ? " is-selected" : ""}`}
                          data-proof-state={plan.state.get(node.id)}
                          data-plan-critical={plan.critical.includes(node.id) ? "true" : "false"}
                        >
                          <button
                            type="button"
                            className="dependency-plan-select"
                            aria-pressed={selected.has(node.id)}
                            onClick={() => choose(node)}
                            ref={(element) => {
                              if (element) nodeElements.current.set(node.id, element);
                              else nodeElements.current.delete(node.id);
                            }}
                          >
                            <small>
                              {node.lineage
                                ? lineageKindLabel[node.lineage.kind]
                                : (lineageKindLabel[node.kind as keyof typeof lineageKindLabel] ??
                                  node.kind)}
                            </small>
                            <strong>{node.name}</strong>
                            {node.detail && (
                              <span className="dependency-plan-detail">{node.detail}</span>
                            )}
                            {node.status && (
                              <span
                                className={`lineage-status lineage-status-${node.status.toLowerCase()}`}
                              >
                                {nodeStatus(node)}
                              </span>
                            )}
                          </button>
                          {!!plan.waiting.get(node.id)?.length && (
                            <small className="eta-plan-waits">
                              待核对的必要来源 · {plan.waiting.get(node.id)!.map((id) => byID.get(id)?.name ?? id).join("、")}
                            </small>
                          )}
                          {!!plan.releases.get(node.id)?.length && (
                            <small className="eta-plan-releases">
                              直接影响 {plan.releases.get(node.id)!.length} 个下游节点
                            </small>
                          )}
                          <details
                            className="dependency-plan-relations"
                            open={selected.has(node.id)}
                          >
                            <summary>
                              上游 {incoming.get(node.id)!.length} · 下游{" "}
                              {outgoing.get(node.id)!.length}
                            </summary>
                            {(["incoming", "outgoing"] as const).map((direction) => {
                              const edges =
                                direction === "incoming"
                                  ? incoming.get(node.id)!
                                  : outgoing.get(node.id)!;
                              return edges.length ? (
                                <section key={direction}>
                                  <h4>{direction === "incoming" ? "依赖来源" : "影响下游"}</h4>
                                  <ul>
                                    {edges.map((edge) => {
                                      const other = byID.get(
                                        direction === "incoming" ? edge.from : edge.to,
                                      )!;
                                      return (
                                        <li
                                          key={edge.id}
                                          data-relation={edge.id}
                                          data-relation-kind={edge.kind}
                                        >
                                          <span
                                            className={
                                              edge.alternative
                                                ? "dependency-relation alternative"
                                                : "dependency-relation"
                                            }
                                          >
                                            {edge.label || edge.kind}
                                          </span>
                                          <button
                                            type="button"
                                            onClick={() => choose(other)}
                                            aria-label={`查看${direction === "incoming" ? "上游" : "下游"}：${other.name}`}
                                          >
                                            {other.name}
                                          </button>
                                        </li>
                                      );
                                    })}
                                  </ul>
                                </section>
                              ) : null;
                            })}
                            {!incoming.get(node.id)!.length && !outgoing.get(node.id)!.length && (
                              <p>当前范围没有声明关联。</p>
                            )}
                          </details>
                        </li>
                      ))}
                    </ul>
                  </li>
                ))}
              </ol>
            </details>
          );
        })}
      </div>
      {!groups.length && (
        <p className="eta-plan-empty" role="status">当前筛选没有匹配的依赖节点；可清除搜索或状态筛选。</p>
      )}
    </section>
  );
}
