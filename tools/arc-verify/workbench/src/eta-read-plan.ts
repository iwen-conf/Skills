import { analyzeDirected } from "./layout";
import type { CanvasGraph, CanvasNode } from "./graph-model";

/** ETA-style read-only plan. Independent observation is NOT task completion or business PASS. */
export type ProofState = "OBSERVED" | "UNOBSERVED" | "MISMATCH" | "BLOCKED";
export interface EtaReadPlan {
  levels: ReadonlyMap<string, number>;
  critical: string[];
  attention?: string;
  next?: string;
  waiting: ReadonlyMap<string, string[]>;
  releases: ReadonlyMap<string, string[]>;
  state: ReadonlyMap<string, ProofState>;
  counts: Record<ProofState, number>;
  valid: boolean;
  reason: string;
}
const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
export function proofState(node: CanvasNode): ProofState {
  const value = node.reading?.status ?? node.status ?? "NOT_OBSERVED";
  if (value === "BLOCKED" || value === "INVALID" || value === "FAIL") return "BLOCKED";
  if (value === "UNSATISFIED" || value === "FALSE") return "MISMATCH";
  if (value === "SATISFIED" || value === "READY") return "OBSERVED";
  return "UNOBSERVED";
}
const urgency: Record<ProofState, number> = {
  OBSERVED: 0, UNOBSERVED: 1, MISMATCH: 2, BLOCKED: 3,
};
export function etaReadPlan(graph: CanvasGraph): EtaReadPlan {
  const known = new Set(graph.nodes.map((node) => node.id));
  const required = graph.edges.filter((edge) =>
    known.has(edge.from) && known.has(edge.to) &&
    !edge.alternative && edge.kind !== "candidate_provider");
  const analysis = analyzeDirected(graph.nodes, required);
  const state = new Map(graph.nodes.map((node) => [node.id, proofState(node)]));
  const counts: Record<ProofState, number> = {
    OBSERVED: 0, UNOBSERVED: 0, MISMATCH: 0, BLOCKED: 0,
  };
  for (const status of state.values()) counts[status]++;
  const waiting = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
  const releases = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
  const before = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
  for (const edge of required) {
    before.get(edge.to)!.push(edge.from);
    releases.get(edge.from)!.push(edge.to);
    if (state.get(edge.from) !== "OBSERVED") waiting.get(edge.to)!.push(edge.from);
  }
  for (const map of [waiting, releases, before])
    for (const [id, entries] of map) map.set(id, [...new Set(entries)].sort(compare));
  const empty = {
    levels: analysis.levels, critical: [] as string[],
    waiting, releases, state, counts,
    attention: undefined as string | undefined,
    next: undefined as string | undefined,
  };
  if (analysis.cycles.length)
    return { ...empty, valid: false,
      reason: "准备依赖包含环，不能按照 ETA 的无环计划推导执行顺序；状态机循环应留在状态视图。" };
  const ordered = [...graph.nodes].sort((a, b) =>
    (analysis.levels.get(a.id) ?? 0) - (analysis.levels.get(b.id) ?? 0) || compare(a.id, b.id));
  const rank = new Map<string, { length: number; unresolved: number; via?: string }>();
  for (const node of ordered) {
    let best: { length: number; unresolved: number; via?: string } = {
      length: 1, unresolved: urgency[state.get(node.id)!],
    };
    for (const parent of before.get(node.id) ?? []) {
      const previous = rank.get(parent);
      if (!previous) continue;
      const candidate = {
        length: previous.length + 1,
        unresolved: previous.unresolved + urgency[state.get(node.id)!],
        via: parent,
      };
      if (candidate.length > best.length ||
        (candidate.length === best.length &&
          (candidate.unresolved > best.unresolved ||
            (candidate.unresolved === best.unresolved &&
              compare(parent, best.via ?? "~~~~") < 0)))) best = candidate;
    }
    rank.set(node.id, best);
  }
  let end: string | undefined;
  for (const node of ordered) {
    const candidate = rank.get(node.id)!;
    const chosen = end ? rank.get(end)! : undefined;
    if (!chosen || candidate.length > chosen.length ||
      (candidate.length === chosen.length &&
        (candidate.unresolved > chosen.unresolved ||
          (candidate.unresolved === chosen.unresolved && compare(node.id, end!) < 0)))) end = node.id;
  }
  const critical: string[] = [];
  for (let cursor = end; cursor; cursor = rank.get(cursor)?.via) critical.push(cursor);
  critical.reverse();
  if (critical.length < 2) critical.length = 0;
  const attentionIndex = critical.findIndex((id) => state.get(id) !== "OBSERVED");
  return {
    ...empty,
    critical,
    attention: attentionIndex < 0 ? undefined : critical[attentionIndex],
    next: attentionIndex < 0 ? undefined : critical[attentionIndex + 1],
    valid: true,
    reason: "ETA 式最长声明依赖链；等长时优先尚未满足或尚未观察的节点，不代表自动执行授权。",
  };
}
