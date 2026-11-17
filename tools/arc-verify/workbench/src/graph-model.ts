import { predicateText } from "./contract";
import type { BranchOption } from "./explorer-model";
import type { Selection, SelectionStep } from "./selection";
import type { BusinessNode, Relation, Snapshot, SnapshotRun } from "./types";

export interface CanvasNode extends BusinessNode {
  subject: string;
  detail?: string;
  step?: string;
  status?: string;
}
export interface CanvasEdge extends Relation {
  label: string;
  branch?: string;
  action?: string;
}
export interface CanvasGraph {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  empty: string;
  caption: string;
}

export function graphFor(
  snapshot: Snapshot,
  state: Selection,
  branches: BranchOption[],
  run?: SnapshotRun,
  steps: SelectionStep[] = [],
): CanvasGraph {
  const model = snapshot.model.data;
  const names = new Map(model.nodes.map((node) => [node.id, node.name]));
  if (state.canvas === "dependencies") {
    const preparations = model.business_contract?.preparation ?? [];
    return {
      nodes: preparations.map((prep) => ({
        id: prep.id,
        subject: prep.action,
        kind: "preparation",
        name: names.get(prep.action) ?? prep.action,
        detail: `需要 ${predicateText(prep.requires)}`,
      })),
      edges: preparations.flatMap((prep) =>
        (prep.depends_on ?? []).map((dependency) => ({
          id: `${dependency}->${prep.id}`,
          from: dependency,
          to: prep.id,
          kind: "prepares",
          label: "准备依赖",
        })),
      ),
      empty: "模型尚未声明准备依赖。状态迁移不会被当作准备步骤。",
      caption: "准备依赖按最长路径分层；预计效果仍需独立回读，不代表已经执行。",
    };
  }
  if (state.canvas === "run") {
    if (!run)
      return {
        nodes: [],
        edges: [],
        empty: "先选择业务分支，再选择该分支已导出的运行。",
        caption: "一次运行中的真实步骤；不会把模型分支当成执行记录。",
      };
    const nodes = steps.map((step) => {
      const actorNodes = state.actor ? step.actorNodes?.[state.actor] : undefined;
      const subject = actorNodes ? (actorNodes.length === 1 ? actorNodes[0] : "") : step.node;
      const trace = run.trace.steps.find((item) => item.step === step.id);
      const fact = run.facts?.find((item) => String(item.seq) === step.id);
      const recorded = fact?.verdict ?? trace?.verdict;
      const evidence = fact?.evidence_status ?? run.evidence_status ?? "LEGACY";
      return {
        id: `step:${step.id}`,
        subject,
        step: step.id,
        kind: "step",
        name: names.get(subject) ?? (subject || `第 ${step.id} 步`),
        detail: `${state.actor || fact?.actor || "参与者见事实"} · ${state.actor && state.actor !== fact?.actor ? "主体观察见事实" : (fact?.observed_state ?? trace?.observed ?? "未观察")} · 记载 ${recorded ?? "NOT_RUN"} / 证据 ${evidence}`,
        status: recorded === "PASS" && evidence !== "PASS" ? evidence : recorded,
      };
    });
    return {
      nodes,
      edges: nodes.slice(1).map((node, i) => ({
        id: `sequence:${i}`,
        from: nodes[i].id,
        to: node.id,
        kind: "recorded_order",
        label:
          Number(node.step) - Number(nodes[i].step) > 1
            ? `省略 ${Number(node.step) - Number(nodes[i].step) - 1} 个筛除步骤`
            : "记录步骤",
      })),
      empty: run.trace.steps.length
        ? "当前主体与实体筛选下没有步骤。清除筛选可查看本次运行的完整轨迹。"
        : "此运行没有记录动作步骤。",
      caption: "连线只表示账本步骤顺序；同一步的并发动作没有人为先后顺序。",
    };
  }
  const flow = model.flows?.find((item) => item.id === state.flow);
  const capabilityIds = new Set(flow ? [flow.capability, ...(flow.capabilities ?? [])] : []);
  const flowActions = new Set(
    model.edges
      .filter((edge) => edge.kind === "implements" && capabilityIds.has(edge.from))
      .map((edge) => edge.to),
  );
  const scopedBranches = branches.filter(
    (branch) => branch.contract && (!state.flow || branch.flow === state.flow),
  );
  const explicitTransitions = new Set(
    scopedBranches.map((branch) => branch.contract?.transition).filter(Boolean),
  );
  const transitions = (model.transitions ?? []).filter(
    (transition) =>
      !state.flow || explicitTransitions.has(transition.edge) || flowActions.has(transition.action),
  );
  const transitionIds = new Set(transitions.map((transition) => transition.edge));
  const edges: CanvasEdge[] = model.edges
    .filter((edge) => edge.kind === "transitions_to" && (!state.flow || transitionIds.has(edge.id)))
    .map((edge) => {
      const transition = transitions.find((item) => item.edge === edge.id);
      const choices = scopedBranches.filter(
        (branch) =>
          branch.contract?.from_states?.includes(edge.from) &&
          (branch.contract?.transition === edge.id ||
            (!branch.contract?.transition && branch.action === transition?.action)),
      );
      const condition = choices.map((branch) => predicateText(branch.contract?.when)).join(" / ");
      const rule = model.rules.find((item) => item.id === transition?.rule);
      const fallback = Object.entries(transition?.preconditions ?? {})
        .map(([id, value]) => `${names.get(id) ?? id}=${value}`)
        .join(" 且 ");
      return {
        ...edge,
        label: `${names.get(transition?.action ?? "") ?? transition?.action ?? rule?.description ?? "状态转换"}${condition || fallback ? ` · ${condition || fallback}` : ""}`,
        branch: choices.length === 1 ? choices[0].id : undefined,
        action: transition?.action,
      };
    });
  const ids = new Set(edges.flatMap((edge) => [edge.from, edge.to]));
  if (flow?.initial) ids.add(flow.initial);
  const nodes: CanvasNode[] = model.nodes
    .filter((node) => node.kind === "state" && (ids.has(node.id) || !state.flow))
    .map((node) => ({ ...node, subject: node.id, detail: node.id }));
  return {
    nodes,
    edges,
    empty: "当前范围没有已声明的状态迁移。先选择流程，或补充有来源的状态与条件。",
    caption: "状态图允许重试和恢复环；线上的条件来自模型，不代表本次运行已满足。",
  };
}
