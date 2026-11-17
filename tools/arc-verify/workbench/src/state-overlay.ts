import type { CanvasEdge, CanvasGraph, CanvasNode } from "./graph-model";
import type { Selection } from "./selection";
import type { Snapshot, SnapshotRun } from "./types";
import { occurrenceKey } from "./lineage";
import {
  stateFactDiagnostic,
  stateOccurrenceDiagnostic,
  stateOccurrenceEntityKeys,
  stateOccurrenceVerdict,
} from "./state-overlay-evidence";

export interface StatePathOccurrence {
  id: string;
  run: string;
  step: string;
  actor: string;
  action_index: number;
  branch: string;
  action: string;
  from_state: string;
  to_state: string;
  verdict: "PASS" | "FAIL" | "BLOCKED";
  reason: string;
  entity_keys: string[];
  evidence_sha256: string;
  occurrence: string;
}
export interface StateOverlayDiagnostic {
  run: string;
  step?: string;
  actor?: string;
  action_index?: number;
  branch?: string;
  reason: string;
}
export interface StateOverlay {
  run: string;
  occurrences: StatePathOccurrence[];
  diagnostics: StateOverlayDiagnostic[];
}

/** Overlay captured occurrences on the declared graph without changing Expected or ordering actions. */
export function withObservedStatePath(
  graph: CanvasGraph,
  snapshot: Snapshot,
  selection: Selection,
  run?: SnapshotRun,
): CanvasGraph {
  if (!run || !selection.run) return graph;
  const overlay: StateOverlay = { run: selection.run, occurrences: [], diagnostics: [] };
  const result = {
    ...graph,
    nodes: [...graph.nodes],
    edges: [...graph.edges],
    stateOverlay: overlay,
  };
  const associated =
    run.branch_id === selection.branch ||
    (run.facts ?? []).some((fact) =>
      (fact.business_expectations ?? []).some(
        (item) =>
          item.decision.frozen?.branch === selection.branch ||
          item.decision.branch === selection.branch ||
          (!item.decision.frozen && selection.branch === "unresolved:records"),
      ),
    );
  if (
    !selection.branch ||
    !associated ||
    selection.model !== snapshot.meta.graph ||
    selection.run !== run.trace.run ||
    !run.environment?.trim() ||
    (selection.environment && selection.environment !== run.environment) ||
    run.evidence_status !== "PASS" ||
    (run.trace.evidence_status !== undefined && run.trace.evidence_status !== "PASS") ||
    run.evidence_errors?.length
  ) {
    overlay.diagnostics.push({
      run: selection.run,
      reason: "所选运行、模型、环境或独立证据不匹配，实际路径不可用。",
    });
    return result;
  }
  const declared = new Map(
    snapshot.model.data.nodes
      .filter((node) => node.kind === "state")
      .map((node) => [node.id, node]),
  );
  const names = new Map(snapshot.model.data.nodes.map((node) => [node.id, node.name]));
  const stateNode = (state: string, phase: "before" | "after", actual: StatePathOccurrence) => {
    const known = declared.get(state);
    const id = known ? state : `actual-state:${actual.id}/${phase}/${encodeURIComponent(state)}`;
    if (!result.nodes.some((node) => node.id === id)) {
      const node: CanvasNode = known
        ? { ...known, subject: known.id }
        : {
            id,
            kind: "observed_state",
            subject: state,
            name: `模型外实际状态 · ${state}`,
            detail: `第 ${actual.step} 步 · ${actual.actor} / 动作 ${actual.action_index}`,
            status: actual.verdict,
            actual,
          };
      result.nodes.push(node);
    }
    return id;
  };
  for (const fact of run.facts ?? []) {
    if (fact.seq < 1) continue;
    const candidates = (fact.business_expectations ?? []).filter(
      (item) => !selection.actor || item.actor === selection.actor,
    );
    if (!candidates.length) {
      if (
        !selection.actor ||
        (fact.actions ?? []).some((action) => action.actor === selection.actor)
      )
        overlay.diagnostics.push({
          run: run.trace.run,
          step: String(fact.seq),
          reason: "该步骤没有可关联的冻结动作记录。",
        });
      continue;
    }
    const factError = stateFactDiagnostic(snapshot, run, fact);
    for (const item of candidates) {
      const frozen = item.decision.frozen;
      if (selection.flow && frozen?.flow && frozen.flow !== selection.flow) continue;
      const diagnostic = {
        run: run.trace.run,
        step: String(fact.seq),
        actor: item.actor,
        action_index: item.action_index,
        branch: frozen?.branch,
      };
      const reason = factError || stateOccurrenceDiagnostic(snapshot, fact, item);
      if (reason) {
        overlay.diagnostics.push({ ...diagnostic, reason });
        continue;
      }
      const entities = stateOccurrenceEntityKeys(run, fact, item);
      if (selection.entity && !entities.includes(selection.entity)) {
        overlay.diagnostics.push({
          ...diagnostic,
          reason: "该动作没有所选完整实体身份的前态绑定或创建捕获，不借用同名 ID。",
        });
        continue;
      }
      const { verdict, reason: reasonText } = stateOccurrenceVerdict(fact, item);
      const id = `actual:${[run.trace.run, fact.seq, item.actor, item.action_index, frozen!.branch].map((value) => encodeURIComponent(String(value))).join("/")}`;
      const actual: StatePathOccurrence = {
        id,
        run: run.trace.run,
        step: String(fact.seq),
        actor: item.actor,
        action_index: item.action_index,
        branch: frozen!.branch,
        action: frozen!.action,
        from_state: frozen!.before.state!,
        to_state: item.after!.state!,
        verdict,
        reason: reasonText,
        entity_keys: entities,
        evidence_sha256: fact.evidence_sha256!,
        occurrence: occurrenceKey({
          run: run.trace.run,
          step: fact.seq,
          actor: item.actor,
          action_index: item.action_index,
          phase: "before",
          branch: frozen!.branch,
          evidence_status: "PASS",
          nodes: [],
        }),
      };
      const edge: CanvasEdge = {
        id,
        from: stateNode(actual.from_state, "before", actual),
        to: stateNode(actual.to_state, "after", actual),
        kind: "actual_transition",
        branch: actual.branch,
        action: actual.action,
        label: `Actual · 第 ${actual.step} 步 · ${actual.actor} / 动作 ${actual.action_index} · ${names.get(actual.action) ?? actual.action} · ${actual.verdict}`,
        actual,
      };
      result.edges.push(edge);
      overlay.occurrences.push(actual);
    }
  }
  if (!overlay.occurrences.length && !overlay.diagnostics.length)
    overlay.diagnostics.push({
      run: run.trace.run,
      reason: "当前运行和筛选范围没有完整的实际状态观察。",
    });
  result.caption += ` 已叠加 ${overlay.occurrences.length} 次独立观察的状态转移；${overlay.diagnostics.length} 项路径诊断。连线属于各自动作发生记录，不声明并发提交顺序。`;
  return result;
}
