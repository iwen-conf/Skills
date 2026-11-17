import { describe, expect, it } from "vite-plus/test";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { lineageFixture } from "./lineage-fixtures";
import { buildDependencyMap } from "./lineage-map";
import { initialSelection } from "./selection";

function sharedFixture() {
  const snapshot = lineageFixture();
  const first = snapshot.business_lineage.branches[0];
  const other = structuredClone(first);
  other.branch = "B-CALLBACK-OTHER";
  other.flow = "F-OTHER";
  other.nodes.forEach((node) => {
    node.branch = other.branch;
  });
  snapshot.business_lineage.branches = [first, other];
  return snapshot;
}

describe("shared business dependency overview", () => {
  it("shows every applicable branch target at the existing dependency entry before branch selection", () => {
    const snapshot = lineageFixture();
    const selection = {
      ...initialSelection(snapshot.meta.graph, snapshot.meta.environment),
      canvas: "dependencies" as const,
      flow: "F-PAY",
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(graph.nodes.filter((node) => node.kind === "target")).toHaveLength(3);
    expect(graph.nodes.some((node) => node.kind === "preparation")).toBe(true);
    expect(graph.nodes.every((node) => !node.reading)).toBe(true);
  });

  it("shares declared conditions and preparation across flows without sharing branch state or actual readings", () => {
    const snapshot = sharedFixture();
    const result = buildDependencyMap(snapshot);
    expect(result.status).toBe("READY");
    expect(result.graph.nodes.filter((node) => node.kind === "target")).toHaveLength(2);
    expect(result.graph.nodes.filter((node) => node.kind === "state")).toHaveLength(2);
    expect(result.graph.nodes.filter((node) => node.kind === "preparation")).toHaveLength(2);
    const account = [...result.members.values()].find((members) =>
      members.some((member) => member.node === "account-condition"),
    );
    expect(account?.map((member) => member.branch).sort()).toEqual([
      "B-CALLBACK",
      "B-CALLBACK-OTHER",
    ]);
    expect(account?.every((member) => member.source_ref && member.declaration)).toBe(true);
    expect(
      result.graph.nodes.every((node) => !node.reading && node.status === "NOT_OBSERVED"),
    ).toBe(true);
    expect(
      result.graph.groups.find((group) => group.id === "dependency:preparation")?.members,
    ).toHaveLength(2);
    expect([...result.edgeMembers.values()].some((members) => members.length === 2)).toBe(true);
  });

  it("does not merge equal labels or IDs with different predicates, sources or preparation effects", () => {
    const snapshot = sharedFixture();
    const other = snapshot.business_lineage.branches[1];
    other.nodes.find((node) => node.id === "account-condition")!.source_ref =
      "different-approved-rule";
    other.nodes.find((node) => node.id === "order-condition")!.predicate = {
      op: "eq",
      left: { ref: { subject: "other-order", attribute: "inflight" } },
      right: { literal: { type: "bool", value: true } },
    };
    other.nodes.find((node) => node.id === "provider-a")!.effects![0].value = true;
    const result = buildDependencyMap(snapshot);
    for (const id of ["account-condition", "order-condition", "provider-a"])
      expect(
        [...result.members.values()].filter((members) =>
          members.some((member) => member.node === id),
        ),
      ).toHaveLength(2);
  });

  it("preserves global branch applicability and an explicitly empty capability scope", () => {
    const snapshot = sharedFixture();
    snapshot.business_lineage.branches[1].flow = undefined;
    expect(
      buildDependencyMap(snapshot, "F-PAY").graph.nodes.filter((node) => node.kind === "target"),
    ).toHaveLength(2);
    expect(
      buildDependencyMap(snapshot, "F-OTHER").graph.nodes.filter((node) => node.kind === "target"),
    ).toHaveLength(1);
    expect(buildDependencyMap(snapshot, "", []).status).toBe("EMPTY");
    const one = buildDependencyMap(snapshot, "", ["B-CALLBACK-OTHER"]);
    expect(one.graph.nodes.filter((node) => node.kind === "target")).toHaveLength(1);
    expect(
      [...one.members.values()].flat().every((member) => member.branch === "B-CALLBACK-OTHER"),
    ).toBe(true);
  });

  it("does not share a logical group when its operand has different rule provenance", () => {
    const snapshot = sharedFixture();
    snapshot.business_lineage.branches[1].nodes.find(
      (node) => node.id === "account-condition",
    )!.source_ref = "independently-confirmed-other-rule";
    const result = buildDependencyMap(snapshot);
    expect(result.graph.nodes.filter((node) => node.kind === "logic")).toHaveLength(2);
  });

  it("uses an explicit branch capability before a shared action's broader implementation mapping", () => {
    const snapshot = sharedFixture();
    const branch = snapshot.model.data.business_contract!.branches.find(
      (item) => item.id === "B-CALLBACK",
    )!;
    branch.capability = "specific-callback-capability";
    snapshot.model.data.nodes.push({
      id: branch.capability,
      kind: "capability",
      name: "在途回调处理",
    });
    const result = buildDependencyMap(snapshot, "", [branch.id]);
    expect(result.graph.groups.some((group) => group.label === "在途回调处理")).toBe(true);
  });

  it("keeps separate branch graphs when a shared projection would invent a dependency cycle", () => {
    const snapshot = sharedFixture();
    for (const branch of snapshot.business_lineage.branches) {
      branch.edges.push({
        id: "cross-condition-order",
        kind: "depends_on",
        source_ref: "explicit-local-order",
        from: branch.branch === "B-CALLBACK" ? "account-condition" : "order-condition",
        to: branch.branch === "B-CALLBACK" ? "order-condition" : "account-condition",
      });
    }
    const result = buildDependencyMap(snapshot);
    expect(result.status).toBe("READY");
    expect(result.warnings.join(" ")).toContain("循环");
    expect([...result.members.values()].every((members) => members.length === 1)).toBe(true);
    expect(result.graph.nodes.filter((node) => node.kind === "preparation")).toHaveLength(4);
  });

  it("reports a bounded projection failure without silently dropping the remainder of a model", () => {
    const snapshot = sharedFixture();
    const branch = snapshot.business_lineage.branches[0];
    branch.nodes = Array.from({ length: 65_537 }, (_, index) => ({
      ...branch.nodes[0],
      id: `large-${index}`,
    }));
    const result = buildDependencyMap(snapshot);
    expect(result.status).toBe("LIMIT");
    expect(result.graph.nodes).toHaveLength(0);
    expect(result.reason).toContain("范围");
  });

  it("bounds nested semantic keys and rejects an original preparation cycle", () => {
    const snapshot = sharedFixture();
    const branch = snapshot.business_lineage.branches[0];
    const condition = branch.nodes.find((node) => node.id === "account-condition")!;
    for (let depth = 0; depth < 70; depth++)
      condition.predicate = { op: "not", args: [condition.predicate!] };
    expect(buildDependencyMap(snapshot).status).toBe("LIMIT");
    const cyclic = sharedFixture();
    cyclic.business_lineage.branches[0].edges.push({
      id: "cycle",
      from: "target",
      to: "pre-state",
      kind: "depends_on",
      source_ref: "invalid-preparation-cycle",
    });
    const result = buildDependencyMap(cyclic);
    expect(result.status).toBe("INVALID");
    expect(result.graph.nodes).toEqual([]);
  });
});
