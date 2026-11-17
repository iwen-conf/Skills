import { describe, expect, it } from "vite-plus/test";
import { etaReadPlan, proofState } from "./eta-read-plan";
import type { CanvasGraph } from "./graph-model";
const graph = (
  nodes: { id: string; status?: string }[], edges: [string, string, boolean?][],
): CanvasGraph => ({
  nodes: nodes.map((node) => ({
    id: node.id, subject: node.id, name: node.id,
    kind: "condition", status: node.status,
  })),
  edges: edges.map(([from, to, alternative], i) => ({
    id: String(i), from, to,
    kind: alternative ? "candidate_provider" : "requires",
    label: alternative ? "候选来源" : "必要前置",
    alternative: Boolean(alternative),
  })),
  caption: "synthetic declarations; no product execution", empty: "none",
});
describe("ETA-style business dependency plan", () => {
  it("uses longest dependency depth rather than shortcut BFS", () => {
    const plan = etaReadPlan(graph(
      ["A", "B", "C", "D"].map((id) => ({ id })),
      [["A","D"],["A","B"],["B","C"],["C","D"]]));
    expect(plan.valid).toBe(true);
    expect(plan.levels.get("D")).toBe(3);
    expect(plan.critical).toEqual(["A","B","C","D"]);
    expect(plan.waiting.get("D")).toEqual(["A","C"]);
  });
  it("prioritizes an unresolved chain on equal depth without assuming business PASS", () => {
    const plan = etaReadPlan(graph(
      [{ id:"A",status:"SATISFIED" }, { id:"B",status:"SATISFIED" },
       { id:"C" }, { id:"D" }],
      [["A","B"],["A","C"],["B","D"],["C","D"]]));
    expect(plan.critical).toEqual(["A","C","D"]);
    expect(plan.attention).toBe("C");
    expect(plan.next).toBe("D");
    expect(plan.waiting.get("D")).toEqual(["C"]);
  });
  it("does not treat candidates and OR alternatives as mandatory dependencies", () => {
    const plan = etaReadPlan(graph(
      [{ id:"provision" },{ id:"ready",status:"SATISFIED" },{ id:"target" }],
      [["provision","ready",true],["ready","target"]]));
    expect(plan.levels.get("ready")).toBe(0);
    expect(plan.critical).toEqual(["ready","target"]);
    expect(plan.waiting.get("ready")).toEqual([]);
    expect(plan.releases.get("provision")).toEqual([]);
  });
  it("detects a preparation cycle without declaring state loops invalid", () => {
    const plan = etaReadPlan(graph([{id:"A"},{id:"B"}], [["A","B"],["B","A"]]));
    expect(plan.valid).toBe(false);
    expect(plan.critical).toEqual([]);
  });
  it("does not equate a generic PASS label with independently observed prerequisite", () => {
    expect(proofState({id:"x",subject:"x",kind:"condition",name:"x",status:"PASS"}))
      .toBe("UNOBSERVED");
    expect(proofState({id:"x",subject:"x",kind:"condition",name:"x",status:"BLOCKED"}))
      .toBe("BLOCKED");
  });
  it("uses linear memory for large dependency chains", () => {
    const n = 2500;
    const label = (i: number) => "N" + String(i).padStart(4,"0");
    const plan = etaReadPlan(graph(
      Array.from({length:n},(_,i)=>({id:label(i)})),
      Array.from({length:n-1},(_,i)=>[label(i),label(i+1)])));
    expect(plan.valid).toBe(true);
    expect(plan.critical).toHaveLength(n);
    expect(plan.levels.get("N2499")).toBe(n-1);
  });
});
