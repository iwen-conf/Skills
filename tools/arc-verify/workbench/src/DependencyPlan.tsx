import { useRef } from "react";
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
  const groupElements = useRef(new Map<string, HTMLDetailsElement>());
  const nodeElements = useRef(new Map<string, HTMLButtonElement>());
  const byID = new Map(graph.nodes.map((node) => [node.id, node]));
  const assigned = new Set<string>();
  const groups = (graph.groups ?? []).flatMap((group) => {
    const members = [...new Set(group.members)].filter((id) => byID.has(id));
    members.forEach((id) => assigned.add(id));
    return members.length ? [{ ...group, members }] : [];
  });
  const ungrouped = graph.nodes.filter((node) => !assigned.has(node.id));
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
    </section>
  );
}
