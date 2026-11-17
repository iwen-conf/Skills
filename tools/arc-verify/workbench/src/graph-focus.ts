import type { CanvasGraph } from "./graph-model";
import type { Selection } from "./selection";

/** A branch can have several prestates and allowed outcomes; none is a default. */
export function canvasSelectionAnchors(graph: CanvasGraph, selection: Selection): Set<string> {
  const direct = graph.nodes.filter((node) =>
    selection.canvas === "run"
      ? node.step === selection.step
      : node.id === selection.node || node.subject === selection.node,
  );
  if (direct.length) return new Set(direct.map((node) => node.id));
  if (selection.canvas !== "states") return new Set();

  const exact = selection.branch
    ? graph.edges.filter((edge) => !edge.legacy && edge.branch === selection.branch)
    : [];
  // A selected branch with no visible edges stays unavailable. Falling back to
  // its action would silently select a different branch or an old transition.
  // Without a branch identity, action navigation keeps every applicable edge.
  const candidates = selection.branch
    ? exact
    : graph.edges.filter((edge) => edge.action === selection.node);
  return new Set(candidates.map((edge) => (selection.focus === "upstream" ? edge.from : edge.to)));
}
