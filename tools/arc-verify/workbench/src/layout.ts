import type { BusinessNode, BusinessView, PositionedNode, Relation } from "./types";

const width = 220;
const height = 110;
const gapX = 80;
const gapY = 28;
const padding = 24;
const header = 100;

const byID = <T extends { id: string }>(left: T, right: T) =>
  left.id < right.id ? -1 : left.id > right.id ? 1 : 0;

/**
 * Deterministic positions for one projection. Boundaries contain only what they `owns`;
 * `contains` never implies ownership. Flow layering visits each state once, so cycles terminate.
 */
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
      : new Set(["contains", "implements", "serves", "owns"]);
  return layeredLayout(
    sorted,
    relations.filter((edge) => kinds.has(edge.kind)),
  );
}

function layeredLayout(nodes: BusinessNode[], edges: Relation[]): PositionedNode[] {
  const outgoing = new Map<string, string[]>();
  const incoming = new Set<string>();
  for (const edge of edges) {
    outgoing.set(edge.from, [...(outgoing.get(edge.from) ?? []), edge.to]);
    if (edge.from !== edge.to) incoming.add(edge.to);
  }
  const level = new Map<string, number>();
  const visit = (start: string) => {
    level.set(start, 0);
    const queue = [start];
    while (queue.length) {
      const current = queue.shift()!;
      for (const next of outgoing.get(current) ?? []) {
        if (!level.has(next)) {
          level.set(next, level.get(current)! + 1);
          queue.push(next);
        }
      }
    }
  };
  for (const node of nodes) if (!incoming.has(node.id) && !level.has(node.id)) visit(node.id);
  for (const node of nodes) if (!level.has(node.id)) visit(node.id);
  const rows = new Map<number, number>();
  return nodes.map((node) => {
    const column = level.get(node.id)!;
    const row = rows.get(column) ?? 0;
    rows.set(column, row + 1);
    return {
      id: node.id,
      x: column * (width + gapX),
      y: row * (height + gapY),
      width,
      height,
      container: false,
    };
  });
}

function boundaryLayout(nodes: BusinessNode[], edges: Relation[]): PositionedNode[] {
  const kinds = new Map(nodes.map((node) => [node.id, node.kind]));
  const owner = new Map<string, string>();
  for (const edge of edges) {
    if (
      edge.kind === "owns" &&
      kinds.get(edge.from) === "boundary" &&
      edge.to !== edge.from &&
      !owner.has(edge.to)
    )
      owner.set(edge.to, edge.from);
  }
  const result: PositionedNode[] = [];
  let x = 0;
  let tallest = 0;
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
    children.forEach((child, index) => {
      result.push({
        id: child.id,
        parentId: boundary.id,
        x: padding,
        y: header + index * (height + gapY),
        width,
        height,
        container: false,
      });
    });
    x += containerWidth + gapX;
    tallest = Math.max(tallest, containerHeight);
  }
  nodes
    .filter((node) => node.kind !== "boundary" && !owner.has(node.id))
    .forEach((node, index) => {
      result.push({
        id: node.id,
        x: index * (width + gapX),
        y: tallest + gapY * 2,
        width,
        height,
        container: false,
      });
    });
  return result;
}
