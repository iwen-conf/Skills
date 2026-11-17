import { describe, expect, it } from "vite-plus/test";
import { buildLineageBlockers } from "./lineage-blockers";
import type { LineageSelection } from "./lineage";
import { lineageFixture } from "./lineage-fixtures";

function observation(): LineageSelection {
  const snapshot = lineageFixture();
  const overlay = snapshot.runs[0].business_lineage[0];
  return {
    graph: snapshot.business_lineage.branches[0],
    overlay,
    candidates: [overlay],
    readings: new Map(overlay.nodes.map((reading) => [reading.node, reading])),
    reason: "synthetic isolated observation",
  };
}

function truth(lineage: LineageSelection, id: string, value: "TRUE" | "FALSE" | "UNKNOWN") {
  const reading = lineage.readings.get(id)!;
  reading.evaluation = { value };
  reading.status =
    value === "TRUE" ? "SATISFIED" : value === "FALSE" ? "UNSATISFIED" : "NOT_OBSERVED";
}

describe("observed dependency blockers", () => {
  it("traces an unmet AND operand to its branch with optional providers rather than mandatory preparation failures", () => {
    const lineage = observation();
    truth(lineage, "guard", "FALSE");
    truth(lineage, "account-condition", "FALSE");
    const report = buildLineageBlockers(lineage);
    expect(report.status).toBe("BLOCKED");
    expect(report.scope).toMatchObject({
      branch: "B-CALLBACK",
      run: lineage.overlay!.run,
      actor: "merchant",
      step: 3,
      action_index: 0,
    });
    expect(report.issues).toHaveLength(1);
    expect(report.issues[0]).toMatchObject({
      node: "account-condition",
      kind: "CONDITION",
      path: ["target", "guard", "account-condition"],
      required_truth: "TRUE",
      observed_truth: "FALSE",
    });
    expect(report.issues[0].providers.sort()).toEqual(["provider-a", "provider-b"]);
  });

  it("does not require a false sibling or an unexecuted provider when OR is already observed true", () => {
    const lineage = observation();
    lineage.graph!.nodes.find((node) => node.id === "guard")!.predicate!.op = "or";
    truth(lineage, "account-condition", "FALSE");
    const report = buildLineageBlockers(lineage);
    expect(report.status).toBe("CLEAR");
    expect(report.issues).toEqual([]);
    expect(report.reason).not.toContain("PASS");
  });

  it("presents an unmet OR as a choice rather than reporting every alternative as mandatory", () => {
    const lineage = observation();
    lineage.graph!.nodes.find((node) => node.id === "guard")!.predicate!.op = "or";
    truth(lineage, "guard", "FALSE");
    truth(lineage, "account-condition", "FALSE");
    truth(lineage, "order-condition", "FALSE");
    const report = buildLineageBlockers(lineage);
    expect(report.issues).toHaveLength(1);
    expect(report.issues[0].node).toBe("guard");
    expect(report.issues[0].alternatives.map((item) => item.node).sort()).toEqual([
      "account-condition",
      "order-condition",
    ]);
  });

  it("explains NOT using the required false child and never suggests a provider that only establishes true", () => {
    const lineage = observation();
    const guard = lineage.graph!.nodes.find((node) => node.id === "guard")!;
    guard.predicate = { op: "not", args: [guard.predicate!.args![0]] };
    lineage.graph!.edges = lineage.graph!.edges.filter((edge) => edge.id !== "order-and");
    truth(lineage, "guard", "FALSE");
    const report = buildLineageBlockers(lineage);
    expect(report.issues).toHaveLength(1);
    expect(report.issues[0]).toMatchObject({
      node: "account-condition",
      required_truth: "FALSE",
      observed_truth: "TRUE",
      providers: [],
    });
    truth(lineage, "guard", "TRUE");
    truth(lineage, "account-condition", "FALSE");
    expect(buildLineageBlockers(lineage).issues).toEqual([]);
  });

  it("keeps unknown observations separate from false business conditions", () => {
    const lineage = observation();
    truth(lineage, "guard", "UNKNOWN");
    truth(lineage, "order-condition", "UNKNOWN");
    const report = buildLineageBlockers(lineage);
    expect(report.status).toBe("UNOBSERVED");
    expect(report.issues).toHaveLength(1);
    expect(report.issues[0]).toMatchObject({
      node: "order-condition",
      kind: "OBSERVATION",
      observed_truth: "UNKNOWN",
    });
  });

  it("preserves failed preparation diagnostics without promoting captured values into readiness", () => {
    const lineage = observation();
    Object.assign(lineage.overlay!, {
      diagnostic: true,
      phase: "preparation",
      step: -1,
      readback_index: 2,
      evidence_status: "BLOCKED",
      reason: "preparation reported success but actual state remained false",
    });
    const report = buildLineageBlockers(lineage);
    expect(report.status).toBe("DIAGNOSTIC");
    expect(report.reason).toContain("actual state remained false");
    expect(report.scope).toMatchObject({ phase: "preparation", readback_index: 2, step: -1 });
    expect(report.issues[0].kind).toBe("EVIDENCE");
  });

  it("does not evaluate business predicates or request values in the browser", () => {
    const lineage = observation();
    lineage.readings.get("account-condition")!.values![0].actual = true;
    expect(buildLineageBlockers(lineage).status).toBe("CLEAR");
  });

  it("refuses a reading scope belonging to another branch and explains missing selection", () => {
    const lineage = observation();
    lineage.overlay!.branch = "other-branch";
    expect(buildLineageBlockers(lineage).issues[0].kind).toBe("EVIDENCE");
    lineage.overlay = undefined;
    lineage.readings.clear();
    expect(buildLineageBlockers(lineage).status).toBe("UNOBSERVED");
  });

  it("reports a conflict between a recorded logical truth and its operands without recalculating success", () => {
    const lineage = observation();
    truth(lineage, "guard", "FALSE");
    const report = buildLineageBlockers(lineage);
    expect(report.status).toBe("BLOCKED");
    expect(report.issues[0]).toMatchObject({ node: "guard", kind: "EVIDENCE" });
  });

  it("bounds a long diagnostic path without falsely reporting that all dependencies are clear", () => {
    const lineage = observation();
    const graph = lineage.graph!;
    const original = graph.nodes.find((node) => node.id === "account-condition")!;
    graph.edges = [
      {
        id: "root",
        from: "nested-0",
        to: graph.target,
        kind: "requires",
        source_ref: original.source_ref,
      },
    ];
    for (let index = 0; index < 280; index++) {
      const id = `nested-${index}`;
      graph.nodes.push({
        ...original,
        id,
        kind: "logic",
        predicate: { op: "not", args: [{ op: "true" }] },
      });
      const next = index === 279 ? original.id : `nested-${index + 1}`;
      graph.edges.push({
        id: `operand-${index}`,
        from: next,
        to: id,
        kind: "logical_operand",
        source_ref: original.source_ref,
      });
      lineage.readings.set(id, {
        node: id,
        status: index % 2 === 0 ? "UNSATISFIED" : "SATISFIED",
        evaluation: { value: index % 2 === 0 ? "FALSE" : "TRUE" },
      });
    }
    truth(lineage, original.id, "FALSE");
    const report = buildLineageBlockers(lineage);
    expect(report.status).toBe("LIMIT");
    expect(report.truncated).toBe(true);
    expect(report.issues[0].path.length).toBeLessThanOrEqual(256);
  });
});
