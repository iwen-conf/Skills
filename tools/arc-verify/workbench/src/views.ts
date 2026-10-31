import type { ProjectedNode, Projection } from "./types";

export interface NodeFilter {
  root?: string;
  status?: string;
  readiness?: string;
  domain?: string;
  role?: string;
}

/** Display-only narrowing; coverage and denominators always come from the projection. */
export function visibleNodes(projection: Projection, filter: NodeFilter): ProjectedNode[] {
  const parents = new Map(projection.nodes.map((node) => [node.id, node.parent ?? ""]));
  const under = (id: string, ancestors: Set<string>) => {
    for (
      let current = id, depth = 0;
      current && depth < 256;
      current = parents.get(current) ?? "", depth++
    ) {
      if (ancestors.has(current)) return true;
    }
    return false;
  };
  // A role reaches what it serves and everything contained beneath it.
  const served = new Set<string>(filter.role ? [filter.role] : []);
  for (const edge of projection.edges) {
    if (edge.kind === "serves" && edge.from === filter.role) served.add(edge.to);
  }
  return projection.nodes.filter(
    (node) =>
      (!filter.root || under(node.id, new Set([filter.root]))) &&
      (!filter.status || node.status === filter.status) &&
      (!filter.readiness ||
        node.readiness[filter.readiness as keyof ProjectedNode["readiness"]] === true) &&
      (!filter.domain || node.id === filter.domain || node.domain === filter.domain) &&
      (!filter.role || under(node.id, served)),
  );
}

/** A hierarchical view narrows to the entity's subtree; other views keep every node. */
export function drillRoot(target: Projection, id: string): string {
  return visibleNodes(target, { root: id }).some((node) => node.id !== id) ? id : "";
}
