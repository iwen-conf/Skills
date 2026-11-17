import { predicateText } from "./contract";
import { branchAppliesToFlow } from "./explorer-model";
import type { BranchOption } from "./explorer-model";
import type { Selection, SelectionStep } from "./selection";
import type { BusinessNode, Relation, Snapshot, SnapshotRun } from "./types";

export interface CanvasNode extends BusinessNode {
  subject: string;
  detail?: string;
  step?: string;
  status?: string;
  branch?: string;
}
export interface CanvasEdge extends Relation {
  label: string;
  branch?: string;
  action?: string;
  outcome?: string;
  expectationMode?: "deterministic" | "allowed_outcomes";
  expectedState?: string;
  denied?: boolean;
  legacy?: boolean;
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
    (branch) => branch.contract && branchAppliesToFlow(branch, state.flow),
  );
  const declaredEdges: CanvasEdge[] = [];
  const resultNodes = new Map<string, CanvasNode>();
  for (const { contract: branch } of scopedBranches) {
    if (!branch) continue;
    for (const from of branch.from_states) {
      for (const outcome of branch.expected.outcomes) {
        const resultID = `expected-result:${[branch.id, outcome.id].map(encodeURIComponent).join("/")}`;
        const to = outcome.state === "UNCHANGED" ? from : outcome.state || resultID;
        if (!outcome.state)
          resultNodes.set(resultID, {
            id: resultID,
            subject: resultID,
            branch: branch.id,
            kind: "expected_result",
            name: outcome.id,
            detail: "预期结果约束占位 · 未声明后态，不是生命周期状态或实际观察",
          });
        const label = [
          names.get(branch.action) ?? branch.action,
          `${branch.id} / ${outcome.id}`,
          branch.expected.mode === "allowed_outcomes" ? "允许结果集合成员" : "确定性结果",
          outcome.denied === undefined ? "" : outcome.denied ? "应拒绝" : "应允许",
          outcome.state === "UNCHANGED" ? "保持前态" : outcome.state ? "" : "后态未声明",
          `当 ${predicateText(branch.when)}`,
        ].filter(Boolean);
        declaredEdges.push({
          id: `expected:${[branch.id, from, outcome.id].map(encodeURIComponent).join("/")}`,
          from,
          to,
          kind: "expected_outcome",
          label: label.join(" · "),
          branch: branch.id,
          action: branch.action,
          outcome: outcome.id,
          expectationMode: branch.expected.mode,
          expectedState: outcome.state,
          denied: outcome.denied,
        });
      }
    }
  }
  const explicitTransitions = new Set(
    scopedBranches.map((branch) => branch.contract?.transition).filter(Boolean),
  );
  const transitions = (model.transitions ?? []).filter(
    (transition) =>
      !state.flow || explicitTransitions.has(transition.edge) || flowActions.has(transition.action),
  );
  const transitionIds = new Set(transitions.map((transition) => transition.edge));
  const legacyEdges: CanvasEdge[] = model.edges
    .filter((edge) => edge.kind === "transitions_to" && (!state.flow || transitionIds.has(edge.id)))
    .filter((edge) => {
      const transition = transitions.find((item) => item.edge === edge.id);
      // Expected owns every covered action/prestate pair. An old edge cannot add
      // an undeclared outcome or merge allow/deny guards into its former target.
      return !scopedBranches.some(
        ({ contract: branch }) =>
          branch?.action === transition?.action && branch?.from_states.includes(edge.from),
      );
    })
    .map((edge) => {
      const transition = transitions.find((item) => item.edge === edge.id);
      const rule = model.rules.find((item) => item.id === transition?.rule);
      const fallback = Object.entries(transition?.preconditions ?? {})
        .map(([id, value]) => `${names.get(id) ?? id}=${value}`)
        .join(" 且 ");
      return {
        ...edge,
        label: `历史迁移 · ${names.get(transition?.action ?? "") ?? transition?.action ?? rule?.description ?? "状态转换"}${fallback ? ` · ${fallback}` : ""}`,
        action: transition?.action,
        legacy: true,
      };
    });
  const edges = [...declaredEdges, ...legacyEdges].sort((a, b) => a.id.localeCompare(b.id));
  const ids = new Set(edges.flatMap((edge) => [edge.from, edge.to]));
  if (flow?.initial) ids.add(flow.initial);
  const nodes: CanvasNode[] = model.nodes
    .filter((node) => node.kind === "state" && (ids.has(node.id) || !state.flow))
    .map((node) => ({ ...node, subject: node.id, detail: node.id }));
  nodes.push(...resultNodes.values());
  return {
    nodes,
    edges,
    empty: "当前范围没有已声明的状态迁移。先选择流程，或补充有来源的状态与条件。",
    caption: model.business_contract
      ? "契约边按分支、前态和预期结果分别展示；允许结果集合仍属于同一分支。历史迁移以虚线单独标识，不补写契约。图不代表实际执行。"
      : "历史状态迁移允许重试和恢复环；尚未声明确定性业务契约，不代表本次运行已满足。",
  };
}
