import type { BusinessNode, BusinessView, PositionedNode, Relation } from "./types";

const width = 236;
const height = 116;
const gapX = 88;
const gapY = 36;
const padding = 24;
const header = 100;
const compare = (left: string, right: string) => (left < right ? -1 : left > right ? 1 : 0);
const byID = <T extends { id: string }>(left: T, right: T) => compare(left.id, right.id);

export interface DirectedAnalysis {
  /** Strongly connected components form a DAG, even when the state graph has retry loops. */
  components: string[][];
  componentOf: Map<string, number>;
  cycles: string[][];
  levels: Map<string, number>;
  criticalNodes: Set<string>;
  criticalEdges: Set<string>;
}

/** Deterministic SCC condensation followed by longest dependency depth, never first-visit BFS. */
export function analyzeDirected(nodes: BusinessNode[], edges: Relation[]): DirectedAnalysis {
  const ids = [...new Set(nodes.map((node) => node.id))].sort(compare);
  const known = new Set(ids);
  const relations = edges.filter((edge) => known.has(edge.from) && known.has(edge.to));
  const outgoing = new Map(ids.map((id) => [id, new Set<string>()]));
  for (const edge of relations) outgoing.get(edge.from)!.add(edge.to);
  let nextIndex = 0;
  const index = new Map<string, number>();
  const low = new Map<string, number>();
  const stack: string[] = [];
  const active = new Set<string>();
  const components: string[][] = [];
  const visit = (id: string) => {
    index.set(id, nextIndex);
    low.set(id, nextIndex++);
    stack.push(id);
    active.add(id);
    for (const target of [...outgoing.get(id)!].sort(compare)) {
      if (!index.has(target)) {
        visit(target);
        low.set(id, Math.min(low.get(id)!, low.get(target)!));
      } else if (active.has(target)) low.set(id, Math.min(low.get(id)!, index.get(target)!));
    }
    if (low.get(id) !== index.get(id)) return;
    const members: string[] = [];
    let member: string;
    do {
      member = stack.pop()!;
      active.delete(member);
      members.push(member);
    } while (member !== id);
    components.push(members.sort(compare));
  };
  for (const id of ids) if (!index.has(id)) visit(id);
  components.sort((left, right) => compare(left[0], right[0]));
  const componentOf = new Map<string, number>();
  components.forEach((members, component) =>
    members.forEach((id) => componentOf.set(id, component)),
  );
  const forward = components.map(() => new Set<number>());
  const backward = components.map(() => new Set<number>());
  for (const edge of relations) {
    const from = componentOf.get(edge.from)!;
    const to = componentOf.get(edge.to)!;
    if (from !== to) {
      forward[from].add(to);
      backward[to].add(from);
    }
  }
  const indegree = backward.map((parents) => parents.size);
  const queue = indegree.flatMap((count, component) => (count === 0 ? [component] : []));
  const order: number[] = [];
  const depth = components.map(() => 0);
  while (queue.length) {
    queue.sort((left, right) => compare(components[left][0], components[right][0]));
    const current = queue.shift()!;
    order.push(current);
    for (const target of forward[current]) {
      depth[target] = Math.max(depth[target], depth[current] + 1);
      if (--indegree[target] === 0) queue.push(target);
    }
  }
  const remaining = components.map(() => 0);
  for (const current of [...order].reverse()) {
    for (const target of forward[current])
      remaining[current] = Math.max(remaining[current], remaining[target] + 1);
  }
  const longest = Math.max(0, ...depth);
  const levels = new Map(ids.map((id) => [id, depth[componentOf.get(id)!]]));
  const criticalNodes = new Set(
    ids.filter((id) => {
      const component = componentOf.get(id)!;
      return depth[component] + remaining[component] === longest;
    }),
  );
  const criticalEdges = new Set(
    relations
      .filter((edge) => {
        const from = componentOf.get(edge.from)!;
        const to = componentOf.get(edge.to)!;
        return from !== to && depth[from] + 1 + remaining[to] === longest;
      })
      .map((edge) => edge.id),
  );
  const cycles = components.filter(
    (members) =>
      members.length > 1 ||
      relations.some((edge) => edge.from === members[0] && edge.to === members[0]),
  );
  return { components, componentOf, cycles, levels, criticalNodes, criticalEdges };
}

/** The selected node stays in context; upstream and downstream do not invent hidden edges. */
export function neighborhood(
  edges: Relation[],
  selected: string,
  direction: "upstream" | "downstream",
): Set<string> {
  const result = new Set(selected ? [selected] : []);
  const pending = selected ? [selected] : [];
  while (pending.length) {
    const current = pending.shift()!;
    for (const edge of edges) {
      const from = direction === "downstream" ? edge.from : edge.to;
      const to = direction === "downstream" ? edge.to : edge.from;
      if (from === current && !result.has(to)) {
        result.add(to);
        pending.push(to);
      }
    }
  }
  return result;
}

export function layoutDirected(
  nodes: BusinessNode[],
  edges: Relation[],
  size = { width, height },
): PositionedNode[] {
  const sorted = [...nodes].sort(byID);
  const { levels } = analyzeDirected(sorted, edges);
  const columns = new Map<number, BusinessNode[]>();
  for (const node of sorted) {
    const level = levels.get(node.id)!;
    columns.set(level, [...(columns.get(level) ?? []), node]);
  }
  const positions = new Map<string, number>();
  // Stable barycentres reduce avoidable crossings without changing dependency depth.
  for (const level of [...columns.keys()].sort((a, b) => a - b)) {
    const column = columns.get(level)!;
    const center = (id: string) => {
      const parents = edges
        .filter((edge) => edge.to === id && positions.has(edge.from))
        .map((edge) => positions.get(edge.from)!);
      return parents.length
        ? parents.reduce((sum, value) => sum + value, 0) / parents.length
        : Number.POSITIVE_INFINITY;
    };
    column.sort((a, b) => {
      const left = center(a.id),
        right = center(b.id);
      return left === right ? compare(a.id, b.id) : left - right;
    });
    column.forEach((node, row) => positions.set(node.id, row));
  }
  return sorted.map((node) => ({
    id: node.id,
    x: levels.get(node.id)! * (size.width + gapX),
    y: positions.get(node.id)! * (size.height + gapY),
    width: size.width,
    height: size.height,
    container: false,
  }));
}

/** Boundary ownership is distinct from hierarchy. State cycles remain legal and visible. */
export function layoutNodes(
  nodes: BusinessNode[],
  edges: Relation[],
  view: BusinessView,
): PositionedNode[] {
  const sorted = [...nodes].sort(byID);
  const ids = new Set(sorted.map((node) => node.id));
  const relations = edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to)).sort(byID);
  if (view === "boundaries") return boundaryLayout(sorted, relations);
  const kinds =
    view === "flows"
      ? new Set(["transitions_to", "next", "transitions"])
      : new Set(["contains", "implements", "serves", "owns", "prepares"]);
  return layoutDirected(
    sorted,
    relations.filter((edge) => kinds.has(edge.kind)),
  );
}

function boundaryLayout(nodes: BusinessNode[], edges: Relation[]): PositionedNode[] {
  const kinds = new Map(nodes.map((node) => [node.id, node.kind]));
  const owner = new Map<string, string>();
  for (const edge of edges)
    if (
      edge.kind === "owns" &&
      kinds.get(edge.from) === "boundary" &&
      edge.to !== edge.from &&
      !owner.has(edge.to)
    )
      owner.set(edge.to, edge.from);
  const result: PositionedNode[] = [];
  let x = 0,
    tallest = 0;
  for (const boundary of nodes.filter((node) => node.kind === "boundary")) {
    const children = nodes.filter((node) => owner.get(node.id) === boundary.id);
    const containerHeight =
      header + padding + Math.max(children.length, 1) * (height + gapY) - gapY;
    const containerWidth = width + padding * 2;
    result.push({
      id: boundary.id,
      x,
      y: 0,
      width: containerWidth,
      height: containerHeight,
      container: true,
    });
    children.forEach((child, position) =>
      result.push({
        id: child.id,
        parentId: boundary.id,
        x: padding,
        y: header + position * (height + gapY),
        width,
        height,
        container: false,
      }),
    );
    x += containerWidth + gapX;
    tallest = Math.max(tallest, containerHeight);
  }
  nodes
    .filter((node) => node.kind !== "boundary" && !owner.has(node.id))
    .forEach((node, position) =>
      result.push({
        id: node.id,
        x: position * (width + gapX),
        y: tallest + gapY * 2,
        width,
        height,
        container: false,
      }),
    );
  return result;
}
