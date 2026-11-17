import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vite-plus/test";
import GraphCanvas from "./GraphCanvas";
import type { CanvasGraph } from "./graph-model";
import { initialSelection } from "./selection";

const drawing = vi.hoisted(() => ({ props: undefined as Record<string, unknown> | undefined }));
vi.mock("@xyflow/react", () => ({
  Background: () => null,
  Controls: () => null,
  MarkerType: { ArrowClosed: "closed" },
  Position: { Left: "left", Right: "right" },
  ReactFlow: (props: Record<string, unknown>) => {
    drawing.props = props;
    return <span data-testid="drawing-host" />;
  },
}));

function dependencyGraph() {
  return {
    nodes: [
      { id: "prepare", subject: "prepare", kind: "preparation", name: "建立订单" },
      {
        id: "paid",
        subject: "paid",
        kind: "condition",
        name: "订单已支付",
        status: "NOT_OBSERVED",
      },
      { id: "refund", subject: "refund", kind: "target", name: "发起退款" },
      { id: "query", subject: "query", kind: "target", name: "查询订单" },
    ],
    edges: [
      {
        id: "provider",
        from: "prepare",
        to: "paid",
        kind: "candidate_provider",
        label: "候选来源",
        alternative: true,
      },
      { id: "refund-pre", from: "paid", to: "refund", kind: "requires", label: "前置要求" },
      { id: "query-pre", from: "paid", to: "query", kind: "requires", label: "前置要求" },
    ],
    groups: [
      { id: "shared", label: "共享业务条件", members: ["prepare", "paid"] },
      { id: "refund-capability", label: "退款业务", members: ["refund"] },
      { id: "query-capability", label: "查询业务", members: ["query"] },
    ],
    empty: "暂无依赖",
    caption: "合成结构测试；没有真实业务运行",
  } satisfies CanvasGraph & { groups: { id: string; label: string; members: string[] }[] };
}

describe("grouped dependency reading through GraphCanvas", () => {
  it("opens a compact plan with independently collapsible business groups", () => {
    const graph = dependencyGraph();
    const selection = { ...initialSelection("test"), canvas: "dependencies" as const };
    const html = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(html).toContain('aria-label="业务分组系谱"');
    for (const group of graph.groups) {
      expect(html).toContain(`data-plan-group="${group.id}"`);
      expect(html).toContain(group.label);
    }
    expect(html.match(/<details[^>]*data-plan-group=/g)).toHaveLength(3);
    expect(html).toContain("关系画布");
  });

  it("keeps global dependency levels and explicit cross-group or alternative navigation in the narrow plan", () => {
    const graph = dependencyGraph();
    const selection = {
      ...initialSelection("test"),
      canvas: "dependencies" as const,
      node: "paid",
    };
    const html = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(html).toContain('aria-label="依赖系谱分层目录"');
    expect(html).toContain('data-plan-node="paid"');
    expect(html).toContain('data-plan-level="2"');
    expect(html).toContain('data-plan-level="3"');
    expect(html).toContain('data-relation="provider"');
    expect(html).toContain('data-relation-kind="candidate_provider"');
    expect(html).toContain('data-relation="refund-pre"');
    expect(html).toContain('data-relation="query-pre"');
    expect(html).toContain("候选来源");
    expect(html).not.toContain("业务通过");
    expect(html).not.toContain("可并行执行");
  });

  it("retains nodes omitted from the group declaration instead of silently hiding them", () => {
    const graph = dependencyGraph();
    graph.groups = graph.groups.slice(0, 1);
    const selection = { ...initialSelection("test"), canvas: "dependencies" as const };
    const html = renderToStaticMarkup(
      <GraphCanvas graph={graph} selection={selection} dispatch={() => undefined} />,
    );
    expect(html).toContain('data-plan-node="refund"');
    expect(html).toContain('data-plan-node="query"');
    expect(html).toContain("未分组节点");
  });
});
