import { branchAppliesToFlow } from "./explorer-model";
import type { CanvasGraph } from "./graph-model";
import { lineageNodeDetail } from "./lineage";
import type {
  BusinessBranchLineage,
  BusinessLineageEdge,
  BusinessLineageNode,
} from "./lineage-types";
import type { Snapshot } from "./types";

export interface DependencyMapMember {
  branch: string;
  node: string;
  source_ref: string;
  declaration: BusinessLineageNode;
}
export interface DependencyMapEdgeMember {
  branch: string;
  edge: string;
  source_ref: string;
  declaration: BusinessLineageEdge;
}
export interface DependencyMapGroup {
  id: string;
  label: string;
  members: string[];
}
export interface DependencyMap {
  graph: CanvasGraph & { groups: DependencyMapGroup[] };
  members: Map<string, DependencyMapMember[]>;
  edgeMembers: Map<string, DependencyMapEdgeMember[]>;
  status: "READY" | "EMPTY" | "LIMIT" | "INVALID";
  reason: string;
  warnings: string[];
}

const maxNodes = 65_536;
const maxEdges = 131_072;
const maxKeyCharacters = 8 * 1024 * 1024;
const maxSingleKey = 65_536;

class ProjectionLimit extends Error {}

function emptyMap(status: DependencyMap["status"], reason: string): DependencyMap {
  return {
    graph: { nodes: [], edges: [], groups: [], empty: reason, caption: reason },
    members: new Map(),
    edgeMembers: new Map(),
    status,
    reason,
    warnings: [],
  };
}

function keyWriter() {
  let total = 0;
  return (value: unknown) => {
    let size = 0;
    const active = new Set<object>();
    const encode = (item: unknown, depth: number): string => {
      if (++size > maxSingleKey) throw new ProjectionLimit("单个依赖声明过大，请缩小总览范围。");
      if (depth > 64) throw new ProjectionLimit("条件结构过深，请缩小依赖总览范围。");
      if (typeof item === "object" && item !== null) {
        if (active.has(item)) throw new Error("依赖声明含有循环对象。");
        active.add(item);
        const result = Array.isArray(item)
          ? `[${item.map((entry) => encode(entry, depth + 1)).join(",")}]`
          : `{${Object.keys(item)
              .filter((key) => Reflect.get(item, key) !== undefined)
              .sort()
              .map(
                (key) => `${encode(key, depth + 1)}:${encode(Reflect.get(item, key), depth + 1)}`,
              )
              .join(",")}}`;
        active.delete(item);
        if (result.length > maxSingleKey)
          throw new ProjectionLimit("单个依赖声明过大，请缩小总览范围。");
        return result;
      }
      const result = JSON.stringify(item) ?? "null";
      size += result.length;
      if (size > maxSingleKey) throw new ProjectionLimit("单个依赖声明过大，请缩小总览范围。");
      return result;
    };
    const key = encode(value, 0);
    total += key.length;
    if (total > maxKeyCharacters)
      throw new ProjectionLimit("依赖声明比较超出预算，请缩小总览范围。");
    return key;
  };
}

function topologicalIDs(
  nodes: readonly { id: string }[],
  edges: readonly { from: string; to: string }[],
) {
  const degree = new Map(nodes.map((node) => [node.id, 0]));
  const outgoing = new Map<string, string[]>();
  for (const edge of edges) {
    if (!degree.has(edge.from) || !degree.has(edge.to))
      throw new Error("依赖关系引用了未导出的节点。");
    degree.set(edge.to, degree.get(edge.to)! + 1);
    const list = outgoing.get(edge.from) ?? [];
    list.push(edge.to);
    outgoing.set(edge.from, list);
  }
  const queue = [...degree].filter(([, count]) => count === 0).map(([id]) => id);
  for (let index = 0; index < queue.length; index++) {
    for (const target of outgoing.get(queue[index]) ?? []) {
      const next = degree.get(target)! - 1;
      degree.set(target, next);
      if (next === 0) queue.push(target);
    }
  }
  return queue;
}

function nodeDeclaration(node: BusinessLineageNode) {
  return {
    kind: node.kind,
    action: node.action,
    preparation: node.preparation,
    predicate: node.predicate,
    from_states: node.from_states,
    references: node.references,
    effects: node.effects,
    source_ref: node.source_ref,
    confirmed: node.confirmed,
  };
}

function project(
  snapshot: Snapshot,
  branches: BusinessBranchLineage[],
  share: boolean,
): DependencyMap {
  const result = emptyMap(
    "READY",
    "声明依赖总览；共享节点保留各分支来源，不汇总不同运行的实际状态。",
  );
  const key = keyWriter();
  const byDeclaration = new Map<string, string>();
  const aliases = new Map<string, Map<string, string>>();
  const preparations = new Map(
    (snapshot.model.data.business_contract?.preparation ?? []).map((item) => [item.id, item]),
  );
  for (const branch of branches) {
    if (aliases.has(branch.branch)) throw new Error("依赖声明包含重复分支。");
    const nodeAliases = new Map<string, string>();
    aliases.set(branch.branch, nodeAliases);
    const originals = new Map(branch.nodes.map((node) => [node.id, node]));
    if (originals.size !== branch.nodes.length) throw new Error("分支依赖包含重复节点。");
    if (new Set(branch.edges.map((edge) => edge.id)).size !== branch.edges.length)
      throw new Error("分支依赖包含重复关系标识。");
    const incoming = new Map<string, BusinessLineageEdge[]>();
    for (const edge of branch.edges) {
      if (!originals.has(edge.from) || !originals.has(edge.to))
        throw new Error("依赖关系引用了未导出的节点。");
      const list = incoming.get(edge.to) ?? [];
      list.push(edge);
      incoming.set(edge.to, list);
    }
    const ordered = topologicalIDs(
      [...branch.nodes].sort((left, right) => left.id.localeCompare(right.id)),
      branch.edges,
    );
    if (ordered.length !== branch.nodes.length)
      throw new Error("声明的准备依赖含循环，业务状态循环应在状态视图查看。");
    for (const originalID of ordered) {
      const node = originals.get(originalID)!;
      const identity = key([branch.branch, node.id]);
      const declared = nodeDeclaration(node);
      const semantic =
        share && node.kind !== "target" && node.kind !== "state"
          ? key({
              node: declared,
              preparation:
                node.kind === "preparation" ? preparations.get(node.preparation ?? "") : undefined,
              prerequisites:
                node.kind === "preparation" || node.kind === "logic"
                  ? (incoming.get(node.id) ?? [])
                      .filter((edge) =>
                        node.kind === "logic"
                          ? edge.kind === "logical_operand"
                          : edge.kind === "requires" || edge.kind === "depends_on",
                      )
                      .map((edge) => ({
                        kind: edge.kind,
                        source_ref: edge.source_ref,
                        alternative: Boolean(edge.alternative),
                        declaration: nodeAliases.get(edge.from),
                      }))
                      .sort(
                        (left, right) =>
                          (left.declaration ?? "").localeCompare(right.declaration ?? "") ||
                          left.kind.localeCompare(right.kind) ||
                          left.source_ref.localeCompare(right.source_ref),
                      )
                  : undefined,
            })
          : identity;
      let id = byDeclaration.get(semantic);
      if (!id) {
        id = `dependency:${encodeURIComponent(branch.branch)}:${encodeURIComponent(node.id)}`;
        byDeclaration.set(semantic, id);
        result.graph.nodes.push({
          id,
          subject: id,
          kind: node.kind,
          name: node.name,
          branch: node.kind === "target" ? branch.branch : undefined,
          detail: lineageNodeDetail(node),
          status: "NOT_OBSERVED",
        });
        result.members.set(id, []);
      }
      nodeAliases.set(node.id, id);
      result.members.get(id)!.push({
        branch: branch.branch,
        node: node.id,
        source_ref: node.source_ref,
        declaration: node,
      });
    }
  }
  const edges = new Map<string, string>();
  for (const branch of branches) {
    const nodeAliases = aliases.get(branch.branch)!;
    const originals = new Map(branch.nodes.map((node) => [node.id, node]));
    for (const edge of branch.edges) {
      const from = nodeAliases.get(edge.from)!;
      const to = nodeAliases.get(edge.to)!;
      const parent = originals.get(edge.to)!;
      const alternative = Boolean(
        edge.alternative || (edge.kind === "logical_operand" && parent.predicate?.op === "or"),
      );
      const semantic = key([from, to, edge.kind, edge.source_ref, alternative]);
      let id = edges.get(semantic);
      if (!id) {
        id = `dependency-edge:${encodeURIComponent(branch.branch)}:${encodeURIComponent(edge.id)}`;
        edges.set(semantic, id);
        result.graph.edges.push({
          id,
          from,
          to,
          kind: edge.kind,
          alternative,
          label:
            edge.kind === "candidate_provider"
              ? "候选来源 · 可替代"
              : edge.kind === "reads"
                ? "读取前置数据"
                : edge.kind === "expects"
                  ? "声明预期"
              : edge.kind === "depends_on"
                ? "声明先后依赖"
                : edge.kind === "logical_operand"
                  ? (parent.predicate?.op.toUpperCase() ?? "逻辑条件")
                  : "前置要求",
        });
        result.edgeMembers.set(id, []);
      }
      result.edgeMembers.get(id)!.push({
        branch: branch.branch,
        edge: edge.id,
        source_ref: edge.source_ref,
        declaration: edge,
      });
    }
  }
  result.graph.groups = dependencyGroups(snapshot, branches, result.members);
  result.graph.caption = result.reason;
  return result;
}

function dependencyGroups(
  snapshot: Snapshot,
  branches: BusinessBranchLineage[],
  members: DependencyMap["members"],
): DependencyMapGroup[] {
  const model = snapshot.model.data;
  const names = new Map(model.nodes.map((node) => [node.id, node.name]));
  const actions = new Map<string, string[]>();
  const explicitCapabilities = new Map(
    (model.business_contract?.branches ?? []).map((branch) => [branch.id, branch.capability]),
  );
  for (const edge of model.edges) {
    if (edge.kind !== "implements") continue;
    const capabilities = actions.get(edge.to) ?? [];
    capabilities.push(edge.from);
    actions.set(edge.to, capabilities);
  }
  const capabilities = new Map(
    branches.map((branch) => {
      const flow = model.flows?.find((item) => item.id === branch.flow);
      const explicit = explicitCapabilities.get(branch.branch);
      return [
        branch.branch,
        explicit
          ? [explicit]
          : (actions.get(branch.action) ??
            (flow ? [flow.capability, ...(flow.capabilities ?? [])] : [])),
      ];
    }),
  );
  const groups = new Map<string, DependencyMapGroup>();
  for (const [id, references] of members) {
    const caps = [
      ...new Set(references.flatMap((member) => capabilities.get(member.branch) ?? [])),
    ].sort();
    const preparation = references.every(
      (member) =>
        member.declaration.kind === "preparation" || Boolean(member.declaration.preparation),
    );
    const groupID = preparation
      ? "dependency:preparation"
      : caps.length > 1
        ? "dependency:shared-capabilities"
        : caps.length
          ? `dependency:capability:${encodeURIComponent(caps[0])}`
          : "dependency:unassigned";
    const label = preparation
      ? "状态准备与前置条件"
      : caps.length > 1
        ? "跨能力共享条件"
        : caps.length
          ? (names.get(caps[0]) ?? caps[0])
          : "尚未归属业务能力";
    let group = groups.get(groupID);
    if (!group) {
      group = { id: groupID, label, members: [] };
      groups.set(groupID, group);
    }
    group.members.push(id);
  }
  return [...groups.values()].sort((left, right) => left.id.localeCompare(right.id));
}

export function buildDependencyMap(
  snapshot: Snapshot,
  flow = "",
  branchIDs?: readonly string[],
): DependencyMap {
  const selected = branchIDs === undefined ? undefined : new Set(branchIDs);
  const branches = (snapshot.business_lineage?.branches ?? [])
    .filter(
      (branch) => branchAppliesToFlow(branch, flow) && (!selected || selected.has(branch.branch)),
    )
    .sort((left, right) => left.branch.localeCompare(right.branch));
  if (!branches.length)
    return emptyMap("EMPTY", "当前范围没有已导出的业务依赖；请选择其他能力或流程。");
  if (
    branches.reduce((count, branch) => count + branch.nodes.length, 0) > maxNodes ||
    branches.reduce((count, branch) => count + branch.edges.length, 0) > maxEdges
  )
    return emptyMap("LIMIT", "依赖总览超出节点或关系预算，请缩小业务能力或流程范围。");
  try {
    const shared = project(snapshot, branches, true);
    if (topologicalIDs(shared.graph.nodes, shared.graph.edges).length === shared.graph.nodes.length)
      return shared;
    const separate = project(snapshot, branches, false);
    if (
      topologicalIDs(separate.graph.nodes, separate.graph.edges).length !==
      separate.graph.nodes.length
    )
      return emptyMap(
        "INVALID",
        "声明的准备依赖含循环，无法作为无环准备系谱；业务状态循环应在状态视图查看。",
      );
    separate.warnings.push("跨分支共享会形成循环，已保留各分支独立依赖，未增加执行先后关系。");
    separate.graph.caption = `${separate.reason} ${separate.warnings[0]}`;
    return separate;
  } catch (error) {
    return emptyMap(
      error instanceof ProjectionLimit ? "LIMIT" : "INVALID",
      error instanceof Error ? error.message : "依赖声明无法生成总览。",
    );
  }
}
