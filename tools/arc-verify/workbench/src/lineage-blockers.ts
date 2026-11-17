import type { LineageSelection } from "./lineage";
import type { BusinessLineageEdge, BusinessLineageReading } from "./lineage-types";

export type LineageTruth = "TRUE" | "FALSE";
export interface LineageBlockerScope {
  branch: string;
  run?: string;
  step?: number;
  actor?: string;
  action_index?: number;
  phase?: "before" | "preparation";
  readback_index?: number;
}
export interface LineageBlockerAlternative {
  node: string;
  required_truth: LineageTruth;
  observed_truth: LineageTruth | "UNKNOWN";
}
export interface LineageBlocker extends LineageBlockerAlternative {
  id: string;
  kind: "CONDITION" | "OBSERVATION" | "EVIDENCE" | "MODEL";
  reason: string;
  path: string[];
  alternatives: LineageBlockerAlternative[];
  providers: string[];
}
export interface LineageBlockerReport {
  status: "CLEAR" | "BLOCKED" | "UNOBSERVED" | "DIAGNOSTIC" | "UNSELECTED" | "LIMIT";
  scope: LineageBlockerScope;
  issues: LineageBlocker[];
  reason: string;
  truncated: boolean;
}

const maxNodes = 65_536;
const maxEdges = 131_072;
const maxVisits = 16_384;
const maxIssues = 1_024;
const maxPath = 256;
const maxAlternatives = 128;

function observedTruth(reading?: BusinessLineageReading): LineageTruth | "UNKNOWN" {
  const value = reading?.evaluation?.value;
  return reading?.status !== "BLOCKED" && (value === "TRUE" || value === "FALSE")
    ? value
    : "UNKNOWN";
}

export function buildLineageBlockers(lineage: LineageSelection): LineageBlockerReport {
  const { graph, overlay, readings } = lineage;
  const report: LineageBlockerReport = {
    status: "UNSELECTED",
    scope: {
      branch: graph?.branch ?? "",
      ...(overlay
        ? {
            run: overlay.run,
            step: overlay.step,
            actor: overlay.actor,
            action_index: overlay.action_index,
            phase: overlay.phase,
            readback_index: overlay.readback_index,
          }
        : {}),
    },
    issues: [],
    reason: lineage.reason,
    truncated: false,
  };
  if (!graph) return report;
  const standalone = (kind: LineageBlocker["kind"], reason: string, node = graph.target) => {
    report.issues.push({
      id: JSON.stringify([node, kind]),
      node,
      kind,
      required_truth: "TRUE",
      observed_truth: "UNKNOWN",
      reason,
      path: [node],
      alternatives: [],
      providers: [],
    });
    report.reason = reason;
  };
  const limit = (reason: string) => {
    report.status = "LIMIT";
    report.truncated = true;
    report.reason = reason;
    return report;
  };
  if (graph.nodes.length > maxNodes || graph.edges.length > maxEdges)
    return limit("阻断分析超出图谱预算；请缩小分支范围，当前结果不证明条件已齐备。");
  if (!overlay) {
    report.status = "UNOBSERVED";
    standalone(
      "OBSERVATION",
      lineage.reason || "尚未选定唯一实际观察，不能从声明推断条件已经满足。",
    );
    return report;
  }
  if (overlay.branch !== graph.branch || !overlay.run || !overlay.actor) {
    report.status = "BLOCKED";
    standalone("EVIDENCE", "观察记录的分支、运行或主体与当前系谱作用域不一致。");
    return report;
  }
  if (overlay.diagnostic || overlay.evidence_status !== "PASS") {
    report.status = overlay.diagnostic ? "DIAGNOSTIC" : "BLOCKED";
    standalone(
      "EVIDENCE",
      overlay.reason || lineage.reason || "证据尚未通过校验，捕获值仅用于诊断。",
    );
    return report;
  }
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
  if (nodes.size !== graph.nodes.length || !nodes.has(graph.target)) {
    report.status = "BLOCKED";
    standalone("MODEL", "依赖声明缺少唯一目标或包含重复节点。");
    return report;
  }
  const incoming = new Map<string, BusinessLineageEdge[]>();
  const outgoing = new Map<string, string[]>();
  const degree = new Map(graph.nodes.map((node) => [node.id, 0]));
  for (const edge of graph.edges) {
    if (!nodes.has(edge.from) || !nodes.has(edge.to)) {
      report.status = "BLOCKED";
      standalone("MODEL", "依赖关系引用了未导出的节点。");
      return report;
    }
    const from = outgoing.get(edge.from) ?? [];
    from.push(edge.to);
    outgoing.set(edge.from, from);
    const to = incoming.get(edge.to) ?? [];
    to.push(edge);
    incoming.set(edge.to, to);
    degree.set(edge.to, degree.get(edge.to)! + 1);
  }
  const ordered = [...degree].filter(([, count]) => count === 0).map(([node]) => node);
  for (let index = 0; index < ordered.length; index++) {
    for (const node of outgoing.get(ordered[index]) ?? []) {
      degree.set(node, degree.get(node)! - 1);
      if (degree.get(node) === 0) ordered.push(node);
    }
  }
  if (ordered.length !== nodes.size) {
    report.status = "BLOCKED";
    standalone("MODEL", "准备依赖含循环，无法形成可靠阻断路径；状态循环应在状态视图查看。");
    return report;
  }
  const untrusted = [...readings.values()].find((reading) => reading.status === "BLOCKED");
  if (untrusted) {
    report.status = "BLOCKED";
    standalone(
      "EVIDENCE",
      untrusted.reason || lineage.reason || "当前发生记录存在未通过校验的观察。",
    );
    return report;
  }
  interface Visit {
    node: string;
    required: LineageTruth;
    parent: number;
  }
  const visits: Visit[] = [{ node: graph.target, required: "TRUE", parent: -1 }];
  const seen = new Set<string>();
  const pathFor = (index: number) => {
    const path: string[] = [];
    while (index >= 0 && path.length < maxPath) {
      path.push(visits[index].node);
      index = visits[index].parent;
    }
    if (index >= 0) limit("阻断路径超出展示预算；已保留局部证据，不能据此确认完整前态。");
    return path.reverse();
  };
  const add = (
    index: number,
    kind: LineageBlocker["kind"],
    reason: string,
    alternatives: LineageBlockerAlternative[] = [],
  ) => {
    if (report.issues.length >= maxIssues) {
      limit("阻断项目超出展示预算，请缩小分支范围；当前结果不完整。");
      return;
    }
    const visit = visits[index];
    const providers =
      visit.required === "TRUE"
        ? [
            ...new Set(
              (incoming.get(visit.node) ?? [])
                .filter((edge) => edge.kind === "candidate_provider")
                .map((edge) => edge.from),
            ),
          ]
        : [];
    if (providers.length > maxAlternatives || alternatives.length > maxAlternatives) {
      limit("候选条件或准备来源超出展示预算，请查看更小的依赖范围。");
      return;
    }
    report.issues.push({
      id: JSON.stringify([visit.node, visit.required, kind]),
      node: visit.node,
      kind,
      required_truth: visit.required,
      observed_truth: observedTruth(readings.get(visit.node)),
      reason,
      path: pathFor(index),
      alternatives,
      providers,
    });
  };
  const enqueue = (node: string, required: LineageTruth, parent: number) => {
    if (visits.length >= maxVisits) {
      limit("阻断路径遍历超出预算，请缩小业务范围；当前结果不证明条件已齐备。");
      return;
    }
    visits.push({ node, required, parent });
  };
  for (let index = 0; index < visits.length && !report.truncated; index++) {
    const visit = visits[index];
    const identity = JSON.stringify([visit.node, visit.required]);
    if (seen.has(identity)) continue;
    seen.add(identity);
    const node = nodes.get(visit.node)!;
    const reading = readings.get(visit.node);
    const observed = observedTruth(reading);
    if (node.kind === "target") {
      const requirements = (incoming.get(node.id) ?? []).filter(
        (edge) => edge.kind === "requires" && !edge.alternative,
      );
      if (!requirements.length) add(index, "MODEL", "目标分支没有已导出的前态与条件依赖。");
      for (const edge of requirements) enqueue(edge.from, "TRUE", index);
      continue;
    }
    if (reading?.node !== node.id) {
      add(index, "OBSERVATION", "此依赖没有当前发生记录的独立观察。");
      continue;
    }
    if (observed === visit.required) continue;
    const kind = observed === "UNKNOWN" ? "OBSERVATION" : "CONDITION";
    if (node.kind !== "logic") {
      add(
        index,
        kind,
        reading.reason ||
          reading.evaluation?.reason ||
          (kind === "OBSERVATION"
            ? "当前观察尚未确定此条件。"
            : `当前观察为 ${observed}，此依赖要求 ${visit.required}。`),
      );
      continue;
    }
    const op = node.predicate?.op;
    const children = (incoming.get(node.id) ?? [])
      .filter((edge) => edge.kind === "logical_operand")
      .map((edge) => edge.from);
    if (
      !children.length ||
      (op === "not" && children.length !== 1) ||
      !["and", "or", "not"].includes(op ?? "")
    ) {
      add(index, "MODEL", "逻辑组合缺少完整、可追溯的操作数。");
      continue;
    }
    const required = op === "not" ? (visit.required === "TRUE" ? "FALSE" : "TRUE") : visit.required;
    const choice = (op === "or" && required === "TRUE") || (op === "and" && required === "FALSE");
    const options = children.map((child) => ({
      node: child,
      required_truth: required,
      observed_truth: observedTruth(readings.get(child)),
    }));
    if (choice) {
      if (options.some((option) => option.observed_truth === required)) {
        add(
          index,
          observed === "UNKNOWN" ? "OBSERVATION" : "EVIDENCE",
          "组合条件的独立评价尚未确认或与子条件冲突，不能在浏览器重算后宣称满足。",
        );
      } else {
        add(
          index,
          kind,
          `此组合要求至少一个子条件为 ${required}；以下为替代条件，不是全部必经要求。`,
          options,
        );
      }
      continue;
    }
    const unmet = options.filter((option) => option.observed_truth !== required);
    if (!unmet.length) {
      add(
        index,
        observed === "UNKNOWN" ? "OBSERVATION" : "EVIDENCE",
        "组合条件的独立评价尚未确认或与子条件冲突，不能从子条件推造通过结果。",
      );
    } else {
      for (const child of unmet) enqueue(child.node, required, index);
    }
  }
  if (report.truncated) return report;
  report.status = report.issues.some((issue) => issue.kind !== "OBSERVATION")
    ? "BLOCKED"
    : report.issues.length
      ? "UNOBSERVED"
      : "CLEAR";
  report.reason =
    report.status === "CLEAR"
      ? overlay.phase === "preparation"
        ? "本次准备回读中的目标条件未发现阻断；尚未冻结目标动作，不代表执行或业务通过。"
        : "本次历史前态观察未发现条件阻断；不代表当前环境就绪或业务执行通过。"
      : report.status === "UNOBSERVED"
        ? "目标依赖仍缺少本次发生记录的独立观察。"
        : "已沿目标依赖定位本次观察的阻断；候选准备来源需要另行规划与验证。";
  return report;
}
