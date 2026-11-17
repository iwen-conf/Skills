import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vite-plus/test";
import App from "./App";
import GraphCanvas from "./GraphCanvas";
import { branchOptions } from "./explorer-model";
import { canvasSelectionAnchors } from "./graph-focus";
import { graphFor } from "./graph-model";
import type { CanvasNode } from "./graph-model";
import { initialSelection } from "./selection";
import { paymentStateSnapshot, typedBranchStateSnapshot } from "./state-graph-fixtures";

// Observe only the drawing host's input. Production selection/projection and the
// actual GraphCanvas component run unchanged. SSR is not browser visual acceptance.
vi.mock("@xyflow/react", () => ({
  Background: () => null,
  Controls: () => null,
  MarkerType: { ArrowClosed: "closed" },
  Position: { Left: "left", Right: "right" },
  ReactFlow: ({ nodes }: { nodes: { data: { source: CanvasNode } }[] }) => (
    <pre data-testid="projected-node-identities">
      {JSON.stringify(nodes.map((node) => node.data.source))}
    </pre>
  ),
}));

describe("typed business state graph", () => {
  it.each(["FLOW_PAYMENT_REFERENCE", "FLOW_REFUND_DUPLICATE"])(
    "keeps the shipped flow-optional branches visible in %s",
    (flow) => {
      const snapshot = paymentStateSnapshot();
      const branches = branchOptions(snapshot).filter((branch) => branch.contract);
      expect(branches).toHaveLength(4);
      expect(branches.every((branch) => branch.flow === "")).toBe(true);
      const html = renderToStaticMarkup(<App state={{ snapshot }} initialContext={{ flow }} />);
      const nav = html.match(/<nav class="capability-rail"[\s\S]*?<\/nav>/)![0];
      for (const branch of branches) expect(nav).toContain(branch.id);
      expect(nav).not.toContain("当前范围没有声明分支");
      const graph = graphFor(
        snapshot,
        { ...initialSelection(snapshot.meta.graph), flow },
        branchOptions(snapshot),
      );
      expect(new Set(graph.edges.map((edge) => edge.branch))).toEqual(
        new Set(branches.map((branch) => branch.id)),
      );
    },
  );

  it("draws reject/UNCHANGED separately instead of attaching its guard to the success edge", () => {
    const snapshot = typedBranchStateSnapshot();
    const graph = graphFor(
      snapshot,
      { ...initialSelection(snapshot.meta.graph), flow: "flow.enrollment" },
      branchOptions(snapshot),
    );
    expect(graph.edges.find((edge) => edge.branch === "accept")).toMatchObject({
      from: "pending",
      to: "ready",
    });
    expect(graph.edges.find((edge) => edge.branch === "reject")).toMatchObject({
      from: "pending",
      to: "pending",
    });
    expect(graph.edges.find((edge) => edge.branch === "reject")?.label).toContain("应拒绝");
    expect(graph.edges.find((edge) => edge.branch === "accept")?.label).not.toContain("非 (");
  });

  it("focuses the exact missing-parent branch before another refund action's earlier edge", () => {
    const snapshot = paymentStateSnapshot();
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      branch: "BR_MISSING_PARENT",
      node: "SUBMIT_REFUND",
      focus: "downstream" as const,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(graph.edges.find((edge) => edge.branch === selection.branch)).toMatchObject({
      from: "EMPTY",
      to: "EMPTY",
    });
    for (const edges of [graph.edges, [...graph.edges].reverse()]) {
      const html = renderToStaticMarkup(
        <GraphCanvas
          graph={{ ...graph, edges }}
          selection={selection}
          dispatch={() => undefined}
        />,
      );
      expect(html).toContain("EMPTY");
      expect(html).toContain("PAYMENT_INDEXED");
      expect(html).toContain("REFUND_INDEXED");
    }
  });

  it("projects all declared allowed outcomes without legacy edges or inventing independent branches", () => {
    const snapshot = typedBranchStateSnapshot();
    const contract = snapshot.model.data.business_contract!;
    const branch = contract.branches[0];
    branch.expected = {
      mode: "allowed_outcomes",
      reason: "Synthetic contract allows a recorded acceptance or unchanged rejection",
      outcomes: [
        ...branch.expected.outcomes,
        ...contract.branches[1].expected.outcomes,
        { ...branch.expected.outcomes[0], id: "created-with-event" },
      ],
    };
    contract.branches = [branch];
    snapshot.model.data.edges = snapshot.model.data.edges.filter(
      (edge) => edge.kind !== "transitions_to",
    );
    snapshot.model.data.transitions = [];
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      branch: branch.id,
      node: branch.action,
      focus: "downstream" as const,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(graph.edges).toHaveLength(3);
    expect(new Set(graph.edges.map((edge) => edge.id)).size).toBe(3);
    expect(new Set(graph.edges.map((edge) => edge.branch))).toEqual(new Set([branch.id]));
    expect(new Set(graph.edges.map((edge) => edge.outcome))).toEqual(
      new Set(["created", "unchanged", "created-with-event"]),
    );
    expect(graph.edges.every((edge) => edge.expectationMode === "allowed_outcomes")).toBe(true);
    expect(graph.edges.every((edge) => edge.label.includes("允许结果集合成员"))).toBe(true);
    expect(canvasSelectionAnchors(graph, selection)).toEqual(new Set(["pending", "ready"]));
    const html = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(html).toContain("pending");
    expect(html).toContain("ready");
  });

  it("isolates the chosen branch without reviving other branches as legacy states", () => {
    const snapshot = typedBranchStateSnapshot();
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      branch: "accept",
      flow: "flow.enrollment",
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(graph.edges.every((edge) => edge.branch === "accept")).toBe(true);
    expect(graph.edges.some((edge) => edge.legacy)).toBe(false);
    expect(graph.nodes.some((node) => node.id === "pending")).toBe(true);
    expect(graph.nodes.some((node) => node.id === "ready")).toBe(true);
  });

  it("keeps each declared prestate for UNCHANGED and does not choose a default prestate", () => {
    const snapshot = typedBranchStateSnapshot();
    const branch = snapshot.model.data.business_contract!.branches[1];
    branch.from_states = ["pending", "ready"];
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      branch: branch.id,
      node: branch.action,
      focus: "upstream" as const,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(
      graph.edges.filter((edge) => edge.branch === "reject").map((edge) => [edge.from, edge.to]),
    ).toEqual([
      ["pending", "pending"],
      ["ready", "ready"],
    ]);
    expect(canvasSelectionAnchors(graph, selection)).toEqual(new Set(["pending", "ready"]));
    expect(
      canvasSelectionAnchors({ ...graph, edges: [...graph.edges].reverse() }, selection),
    ).toEqual(new Set(["pending", "ready"]));
  });

  it("keeps action-level focus on every matching result, while exact branch focus excludes other branches", () => {
    const snapshot = typedBranchStateSnapshot();
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      node: "enroll",
      focus: "downstream" as const,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(canvasSelectionAnchors(graph, selection)).toEqual(new Set(["pending", "ready"]));
    expect(canvasSelectionAnchors(graph, { ...selection, branch: "accept" })).toEqual(
      new Set(["ready"]),
    );
    expect(canvasSelectionAnchors(graph, { ...selection, branch: "reject" })).toEqual(
      new Set(["pending"]),
    );
  });

  it("does not fall back to another flow's legacy action when the selected typed branch has no edge", () => {
    const snapshot = typedBranchStateSnapshot();
    snapshot.model.data.flows!.push({
      ...snapshot.model.data.flows![0],
      id: "flow.other",
    });
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      flow: "flow.other",
      branch: "accept",
      node: "enroll",
      focus: "downstream" as const,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(graph.edges.some((edge) => edge.action === "enroll" && edge.legacy)).toBe(true);
    expect(graph.edges.some((edge) => edge.branch === "accept")).toBe(false);
    expect(canvasSelectionAnchors(graph, selection)).toEqual(new Set());
    const html = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(html).not.toContain('data-testid="projected-node-identities"');
  });

  it("labels stateless expected constraints without inventing a lifecycle or actual state", () => {
    const snapshot = typedBranchStateSnapshot();
    const branch = snapshot.model.data.business_contract!.branches[0];
    branch.expected.outcomes[0].state = undefined;
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      branch: branch.id,
      node: branch.action,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    const edge = graph.edges.find((item) => item.branch === branch.id)!;
    const result = graph.nodes.find((node) => node.id === edge.to)!;
    expect(result).toMatchObject({ kind: "expected_result", branch: branch.id });
    expect(result.detail).toContain("不是生命周期状态或实际观察");
    expect(result.status).toBeUndefined();
    expect(edge.expectedState).toBeUndefined();
    expect(edge.to).not.toBe("ready");
    expect(edge.to).not.toBe("pending");
    expect(edge.label).toContain("后态未声明");
  });

  it("preserves legacy-only state edges without attaching typed identity or strong results", () => {
    const snapshot = typedBranchStateSnapshot();
    snapshot.model.data.business_contract = undefined;
    const selection = {
      ...initialSelection(snapshot.meta.graph),
      node: "enroll",
      focus: "downstream" as const,
    };
    const graph = graphFor(snapshot, selection, branchOptions(snapshot));
    expect(graph.edges).toHaveLength(1);
    expect(graph.edges[0]).toMatchObject({
      id: "submit-flow",
      from: "pending",
      to: "ready",
      legacy: true,
    });
    expect(graph.edges[0].branch).toBeUndefined();
    expect(graph.edges[0].outcome).toBeUndefined();
    expect(graph.edges[0].label).toContain("历史迁移");
    expect(graph.nodes.every((node) => node.status === undefined)).toBe(true);
    expect(canvasSelectionAnchors(graph, selection)).toEqual(new Set(["ready"]));
    expect(
      canvasSelectionAnchors(graph, { ...selection, branch: "legacy:test.idempotency" }),
    ).toEqual(new Set());
  });
});
