import { displayValue, predicateText } from "./contract";
import { entityKey, factEntities } from "./facts";
import type { CanvasGraph } from "./graph-model";
import type {
  BusinessBranchLineage,
  BusinessLineageNode,
  BusinessLineageOverlay,
  BusinessLineageReading,
} from "./lineage-types";
import type { Selection } from "./selection";
import type { Snapshot, SnapshotRun } from "./types";

export interface LineageSelection {
  graph?: BusinessBranchLineage;
  candidates: BusinessLineageOverlay[];
  overlay?: BusinessLineageOverlay;
  readings: Map<string, BusinessLineageReading>;
  reason: string;
}

export const occurrenceKey = (overlay: BusinessLineageOverlay) =>
  JSON.stringify([
    overlay.run,
    overlay.step,
    overlay.actor,
    overlay.action_index,
    overlay.phase,
    overlay.branch,
    overlay.readback_index,
  ]);
export const occurrenceLabel = (overlay: BusinessLineageOverlay) =>
  overlay.phase === "preparation"
    ? `准备回读 ${overlay.readback_index} · ${{ initial: "初始观察", pre_action: "准备动作前", post_action: "准备动作后" }[overlay.readback_phase ?? ""] ?? overlay.readback_phase} · ${overlay.actor}`
    : `第 ${overlay.step} 步 · 动作 ${overlay.action_index} · ${overlay.actor} · before`;

export function lineageSelection(
  snapshot: Snapshot,
  selection: Selection,
  run?: SnapshotRun,
): LineageSelection {
  const graph = snapshot.business_lineage?.branches.find(
    (branch) => branch.branch === selection.branch,
  );
  const empty = (reason: string, candidates: BusinessLineageOverlay[] = []): LineageSelection => ({
    graph,
    candidates,
    readings: new Map(),
    reason,
  });
  if (!graph)
    return empty(
      selection.branch ? "当前分支未导出依赖系谱。" : "选择业务分支，追溯它的前态和条件来源。",
    );
  if (!run || run.trace.run !== selection.run || !selection.step)
    return empty("尚无实际前态观察；图中展示声明条件与候选准备来源。");
  let candidates = (run.business_lineage ?? []).filter(
    (overlay) =>
      overlay.branch === graph.branch &&
      overlay.run === selection.run &&
      String(overlay.step) === selection.step &&
      overlay.phase === (selection.step === "-1" ? "preparation" : "before") &&
      (!selection.actor || overlay.actor === selection.actor),
  );
  if (selection.entity) {
    const fact = run.facts?.find((item) => String(item.seq) === selection.step);
    const entity = factEntities(fact).find((item) => entityKey(item) === selection.entity);
    candidates = candidates.filter(
      (overlay) =>
        entity &&
        overlay.nodes.some((node) =>
          node.values?.some(
            (value) => value.entity_ref && entityKey(value.entity_ref) === entityKey(entity),
          ),
        ),
    );
  }
  if (!candidates.length)
    return empty("当前运行、步骤、主体或实体没有对应的前态观察；不会借用其他发生记录。");
  const overlay = selection.occurrence
    ? candidates.find((item) => occurrenceKey(item) === selection.occurrence)
    : candidates.length === 1
      ? candidates[0]
      : undefined;
  if (!overlay)
    return empty("此步骤有多个观察作用域，请选择动作发生记录；尚未合并任何读取。", candidates);
  const fact = run.facts?.find((item) => item.seq === overlay.step);
  const trust = [
    run.evidence_status,
    fact?.evidence_status ?? run.evidence_status,
    overlay.evidence_status,
  ];
  const qualified = !overlay.diagnostic && trust.every((status) => status === "PASS");
  const reason = qualified
    ? `${occurrenceLabel(overlay)}；${overlay.phase === "preparation" ? "准备目标尚未执行" : "不代表业务结果通过"}。`
    : `${overlay.reason || "证据未通过校验"}（${trust.map((status) => status ?? "LEGACY").join(" / ")}）；保留观察，不能据此确认条件。`;
  return {
    graph,
    candidates,
    overlay,
    reason,
    readings: new Map(
      overlay.nodes.map((reading) => [
        reading.node,
        qualified
          ? reading
          : {
              ...reading,
              status: "BLOCKED",
              reason: `${reason}${reading.reason ? ` ${reading.reason}` : ""}`,
            },
      ]),
    ),
  };
}

export const lineageKindLabel: Record<BusinessLineageNode["kind"], string> = {
  target: "目标业务分支",
  state: "前置状态",
  condition: "属性条件",
  logic: "逻辑组合",
  preparation: "准备来源",
};

export function lineageStatusLabel(
  node: BusinessLineageNode,
  reading?: BusinessLineageReading,
): string {
  if (!reading || reading.status === "NOT_OBSERVED") return "未观察";
  if (reading.status === "BLOCKED") return "证据阻断";
  if (node.kind === "target") return reading.status === "READY" ? "前态已具备" : "前态不满足";
  if (node.kind === "preparation")
    return reading.status === "SATISFIED" ? "效果已观察" : "效果不符合";
  if (node.kind === "state") return reading.status === "SATISFIED" ? "前态吻合" : "前态不符";
  return reading.status === "SATISFIED" ? "谓词为真" : "谓词为假";
}

export function lineageNodeDetail(node: BusinessLineageNode): string {
  if (node.kind === "state")
    return (node.from_states ?? []).join(" 或 ") || "未声明具体生命周期状态";
  if (node.kind === "preparation")
    return node.effects?.length
      ? node.effects
          .map((effect) => `${effect.subject}.${effect.attribute} ← ${displayValue(effect.value)}`)
          .join("；")
      : "声明准备动作；是否已执行请查看准备账本";
  if (node.predicate) return predicateText(node.predicate);
  return `${node.branch} · ${node.confirmed ? "已确认规则" : "规则待确认"}`;
}

export function lineageGraphFor(
  snapshot: Snapshot,
  selection: Selection,
  run?: SnapshotRun,
): CanvasGraph {
  const lineage = lineageSelection(snapshot, selection, run);
  const graph = lineage.graph;
  if (!graph)
    return { nodes: [], edges: [], empty: lineage.reason, caption: lineage.reason, lineage };
  return {
    nodes: graph.nodes.map((node) => ({
      id: node.id,
      subject: node.kind === "target" ? graph.action : node.id,
      kind: node.kind,
      name: node.name,
      branch: graph.branch,
      detail: lineageNodeDetail(node),
      status: lineage.readings.get(node.id)?.status ?? "NOT_OBSERVED",
      lineage: node,
      reading: lineage.readings.get(node.id),
    })),
    edges: graph.edges.map((edge) => {
      const parent = graph.nodes.find((node) => node.id === edge.to);
      const alternative =
        edge.alternative || (edge.kind === "logical_operand" && parent?.predicate?.op === "or");
      return {
        ...edge,
        alternative,
        label:
          edge.kind === "candidate_provider"
            ? "候选来源 · 可替代"
            : edge.kind === "depends_on"
              ? "声明先后依赖"
              : edge.kind === "logical_operand"
                ? (parent?.predicate?.op.toUpperCase() ?? "逻辑条件")
                : "前置要求",
      };
    }),
    empty: "当前分支没有可显示的系谱节点。",
    caption: `${lineage.reason} 候选来源表示可替代的声明路径；逻辑子条件的真假不单独决定目标就绪。`,
    lineage,
  };
}
