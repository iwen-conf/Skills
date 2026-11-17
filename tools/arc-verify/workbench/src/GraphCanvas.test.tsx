import { renderToStaticMarkup } from "react-dom/server";
import type { Edge, Node } from "@xyflow/react";
import { describe, expect, it, vi } from "vite-plus/test";
import GraphCanvas from "./GraphCanvas";
import type { CanvasEdge, CanvasNode } from "./graph-model";
import { initialSelection } from "./selection";

interface FlowCapture {
  nodes: Node[];
  edges: Edge[];
  onNodeClick?: (event: unknown, node: Node) => void;
  onEdgeClick?: (event: unknown, edge: Edge) => void;
}
const drawing = vi.hoisted(() => ({ props: undefined as FlowCapture | undefined }));
vi.mock("@xyflow/react", () => ({
  Background: () => null,
  Controls: () => null,
  MarkerType: { ArrowClosed: "closed" },
  Position: { Left: "left", Right: "right" },
  ReactFlow: (props: FlowCapture) => {
    drawing.props = props;
    return <span data-testid="drawing-host" />;
  },
}));

const actual = {
  id: "R/2/merchant/1",
  run: "R",
  step: "2",
  actor: "merchant",
  action_index: 1,
  branch: "accept",
  action: "enroll",
  from_state: "pending",
  to_state: "unexpected",
  verdict: "FAIL" as const,
  reason: "independent readback violated frozen expectation",
  entity_keys: ["full-entity-identity"],
  evidence_sha256: "synthetic-digest",
  occurrence: "synthetic-exact-occurrence",
};

function stateGraph() {
  return {
    nodes: [
      { id: "pending", subject: "pending", kind: "state", name: "等待" },
      { id: "ready", subject: "ready", kind: "state", name: "完成" },
      {
        id: "unexpected",
        subject: "unexpected",
        kind: "observed_state",
        name: "模型外观察",
        actual,
      },
    ],
    edges: [
      {
        id: "expected",
        from: "pending",
        to: "ready",
        kind: "expected_outcome",
        label: "契约预期",
        branch: "accept",
      },
      {
        id: "actual",
        from: "pending",
        to: "unexpected",
        kind: "actual_transition",
        label: "实际偏离",
        actual,
      },
    ],
    stateOverlay: { run: "R", occurrences: [actual], diagnostics: [] },
    empty: "无状态",
    caption: "合成的显示边界测试",
  };
}

describe("evidence-aware canvas presentation", () => {
  it("does not promote a candidate provider into the mandatory critical chain of a cross-branch map", () => {
    const graph = {
      nodes: ["prepare", "condition", "target"].map((id) => ({
        id,
        subject: id,
        kind: "condition",
        name: id,
      })),
      edges: [
        {
          id: "p",
          from: "prepare",
          to: "condition",
          kind: "candidate_provider",
          label: "候选来源",
          alternative: true,
        },
        { id: "c", from: "condition", to: "target", kind: "requires", label: "前置要求" },
      ],
      groups: [{ id: "shared", label: "共享条件", members: ["prepare", "condition", "target"] }],
      empty: "无依赖",
      caption: "合成的跨分支结构测试",
    };
    const selection = {
      ...initialSelection("test"),
      canvas: "dependencies" as const,
      focus: "critical" as const,
    };
    const html = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(html).toContain('data-plan-node="condition"');
    expect(html).toContain('data-plan-node="target"');
    expect(html).not.toContain('data-plan-node="prepare"');
  });

  it("filters the exact supplied blocker path rather than treating blocked as upstream", () => {
    const graph = {
      nodes: ["prepare", "condition", "target"].map((id) => ({
        id,
        subject: id,
        kind: "condition",
        name: id,
      })),
      edges: [
        {
          id: "p",
          from: "prepare",
          to: "condition",
          kind: "candidate_provider",
          label: "候选来源",
          alternative: true,
        },
        { id: "c", from: "condition", to: "target", kind: "requires", label: "前置要求" },
      ],
      empty: "无依赖",
      caption: "合成结构测试",
    };
    const selection = {
      ...initialSelection("test"),
      canvas: "dependencies" as const,
      focus: "blocked" as const,
      node: "target",
    };
    const html = renderToStaticMarkup(
      <GraphCanvas
        graph={graph}
        selection={selection}
        dispatch={() => undefined}
        blockerNodes={["condition", "target"]}
      />,
    );
    expect(html).toContain('data-plan-node="condition"');
    expect(html).toContain('data-plan-node="target"');
    expect(html).not.toContain('data-plan-node="prepare"');
    const missing = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(missing).not.toContain('data-plan-node="target"');
    expect(missing).toContain("没有可显示的阻断路径");
  });

  it("visually distinguishes a failing actual transition from its frozen expected edge", () => {
    const graph = stateGraph();
    const html = renderToStaticMarkup(
      <GraphCanvas
        graph={graph}
        selection={{ ...initialSelection("test"), canvas: "states" }}
        dispatch={() => undefined}
      />,
    );
    const edges = drawing.props!.edges;
    expect(edges.find((edge) => edge.id === "actual")?.style?.stroke).toBe("var(--fail)");
    expect(edges.find((edge) => edge.id === "actual")?.type).toBe("actual");
    expect(edges.find((edge) => edge.id === "expected")?.type).toBe("smoothstep");
    expect(html).toContain("Expected");
    expect(html).toContain("Actual");
    expect(graph.edges[0]).not.toHaveProperty("actual");
  });

  it("keeps observed self-transitions visible instead of losing them inside a folded state cycle", () => {
    const graph = stateGraph();
    graph.edges[1].to = "pending";
    graph.edges[1].actual = { ...actual, to_state: "pending" };
    renderToStaticMarkup(
      <GraphCanvas
        graph={graph}
        selection={{ ...initialSelection("test"), canvas: "states" }}
        dispatch={() => undefined}
      />,
    );
    expect(drawing.props!.edges.find((edge) => edge.id === "actual")).toMatchObject({
      source: "pending",
      target: "pending",
    });
  });

  it("forwards the exact occurrence-bearing node and edge to the owning selection controller", () => {
    const graph = stateGraph();
    const nodeSelected = vi.fn<(node: CanvasNode) => void>();
    const edgeSelected = vi.fn<(edge: CanvasEdge) => void>();
    const dispatch = vi.fn();
    renderToStaticMarkup(
      <GraphCanvas
        graph={graph}
        selection={{ ...initialSelection("test"), canvas: "states" }}
        dispatch={dispatch}
        onSelectNode={nodeSelected}
        onSelectEdge={edgeSelected}
      />,
    );
    const props = drawing.props!;
    props.onNodeClick!(
      undefined,
      props.nodes.find((node) => node.id === "unexpected")!,
    );
    props.onEdgeClick!(
      undefined,
      props.edges.find((edge) => edge.id === "actual")!,
    );
    expect(nodeSelected).toHaveBeenCalledWith(graph.nodes[2]);
    expect(edgeSelected).toHaveBeenCalledWith(graph.edges[1]);
    expect(dispatch).not.toHaveBeenCalled();
  });
});
