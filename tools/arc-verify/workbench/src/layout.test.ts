import { describe, expect, it } from "vite-plus/test";
import { analyzeDirected, layoutDirected, layoutNodes, neighborhood } from "./layout";
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
  it("puts a shared dependency after its longest path, regardless of a shortcut", () => {
    const dag = ["A", "B", "C", "D"].map((id) => ({ id, kind: "action", name: id }));
    const links = [
      ["A", "D"],
      ["A", "B"],
      ["B", "C"],
      ["C", "D"],
    ].map(([from, to]) => ({ id: from + to, kind: "prepares", from, to }));
    const result = layoutDirected(dag, links);
    const x = new Map(result.map((node) => [node.id, node.x]));
    for (const edge of links) expect(x.get(edge.from)!).toBeLessThan(x.get(edge.to)!);
    expect([...analyzeDirected(dag, links).criticalEdges].sort()).toEqual(["AB", "BC", "CD"]);
    expect(layoutDirected([...dag].reverse(), [...links].reverse())).toEqual(result);
  });
  it("condenses legal state loops without losing a downstream exit or self-loop", () => {
    const states = ["pending", "retry", "success", "terminal"].map((id) => ({
      id,
      kind: "state",
      name: id,
    }));
    const links = [
      ["pending", "retry"],
      ["retry", "pending"],
      ["retry", "success"],
      ["terminal", "terminal"],
    ].map(([from, to]) => ({ id: from + to, kind: "transitions_to", from, to }));
    const analysis = analyzeDirected(states, links);
    expect(analysis.cycles).toEqual([["pending", "retry"], ["terminal"]]);
    expect(analysis.levels.get("success")).toBeGreaterThan(analysis.levels.get("retry")!);
    expect([...neighborhood(links, "success", "upstream")].sort()).toEqual([
      "pending",
      "retry",
      "success",
    ]);
    expect([...neighborhood(links, "pending", "downstream")].sort()).toEqual([
      "pending",
      "retry",
      "success",
    ]);
  });

  it("handles a dependency chain at the export node budget without recursive stack overflow", () => {
    const count = 65_536;
    const chain = Array.from({ length: count }, (_, index) => ({
      id: `node-${String(index).padStart(5, "0")}`,
      name: `条件 ${index}`,
      kind: "condition",
    }));
    const links = chain.slice(1).map((node, index) => ({
      id: `edge-${index}`,
      kind: "requires",
      from: chain[index].id,
      to: node.id,
    }));
    let analysis: ReturnType<typeof analyzeDirected> | undefined;
    expect(() => {
      analysis = analyzeDirected(chain, links);
    }).not.toThrow();
    expect(analysis?.levels.get(chain[count - 1].id)).toBe(count - 1);
    expect(analysis?.cycles).toEqual([]);
    expect(analysis?.criticalEdges.size).toBe(count - 1);
    expect(neighborhood(links, chain[count - 1].id, "upstream").size).toBe(count);
  });

  it("preserves exactly the mutually reachable state components across branching cycles", () => {
    let seed = 17;
    for (let sample = 0; sample < 96; sample++) {
      const states = Array.from({ length: 7 }, (_, index) => ({
        id: String(index),
        kind: "state",
        name: String(index),
      }));
      const reachable = states.map(() => states.map(() => false));
      const links: Relation[] = [];
      for (let from = 0; from < states.length; from++) {
        for (let to = 0; to < states.length; to++) {
          seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
          if (seed % 5 !== 0) continue;
          reachable[from][to] = true;
          links.push({
            id: `${from}:${to}`,
            kind: "transitions_to",
            from: String(from),
            to: String(to),
          });
        }
      }
      for (let via = 0; via < states.length; via++)
        for (let from = 0; from < states.length; from++)
          for (let to = 0; to < states.length; to++)
            reachable[from][to] ||= reachable[from][via] && reachable[via][to];
      const analysis = analyzeDirected(states, links);
      for (let from = 0; from < states.length; from++) {
        for (let to = 0; to < states.length; to++) {
          expect(
            analysis.componentOf.get(String(from)) === analysis.componentOf.get(String(to)),
          ).toBe(from === to || (reachable[from][to] && reachable[to][from]));
        }
      }
    }
  });
});
