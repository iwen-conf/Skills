import { describe, expect, it } from "vite-plus/test";
import { layoutNodes } from "./layout";
import type { BusinessNode, Relation } from "./types";

describe("typed visual projection", () => {
  const nodes: BusinessNode[] = [
    { id: "left", kind: "boundary", name: "职责甲" },
    { id: "right", kind: "boundary", name: "职责乙" },
    { id: "cap", kind: "capability", name: "能力" },
  ];
  const edges: Relation[] = [
    { id: "own", kind: "owns", from: "left", to: "cap" },
    { id: "depend", kind: "depends_on", from: "cap", to: "right" },
  ];
  it("contains owned nodes and preserves cross-boundary identities", () => {
    const result = layoutNodes(nodes, edges, "boundaries");
    expect(result.map((n) => n.id).sort()).toEqual(["cap", "left", "right"]);
    const cap = result.find((n) => n.id === "cap");
    const owner = result.find((n) => n.id === "left");
    expect(cap?.parentId).toBe("left");
    expect(cap && owner && cap.x + cap.width <= owner.width).toBe(true);
    expect(cap && owner && cap.y + cap.height <= owner.height).toBe(true);
    expect(edges[1]).toEqual({ id: "depend", kind: "depends_on", from: "cap", to: "right" });
  });
  it("is deterministic and never uses contains as boundary ownership", () => {
    expect(layoutNodes([...nodes].reverse(), edges, "boundaries")).toEqual(
      layoutNodes(nodes, edges, "boundaries"),
    );
    expect(
      layoutNodes(nodes, [{ ...edges[0], kind: "contains" }], "boundaries").find(
        (n) => n.id === "cap",
      )?.parentId,
    ).toBeUndefined();
  });
  it("terminates for a declared retry cycle", () => {
    const states = [
      { id: "s0", kind: "state", name: "初始" },
      { id: "s1", kind: "state", name: "完成" },
    ];
    const transitions = [
      { id: "t0", kind: "transitions_to", from: "s0", to: "s1" },
      { id: "t1", kind: "transitions_to", from: "s1", to: "s0" },
    ];
    expect(
      layoutNodes(states, transitions, "flows").every(
        (n) => Number.isFinite(n.x) && Number.isFinite(n.y),
      ),
    ).toBe(true);
  });
});
