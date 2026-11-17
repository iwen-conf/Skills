import { describe, expect, it } from "vite-plus/test";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";

describe("recorded business identity continuity", () => {
  it("allows a global branch to keep the flow captured by its frozen prestate", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const item = fact.business_expectations![0];
    delete snapshot.model.data.business_contract!.branches.find(
      (branch) => branch.id === item.decision.branch,
    )!.flow;
    mirrorRaw(snapshot, run, fact);
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
  });

  it("refuses a frozen flow identity that contradicts its own before observation", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const project = () => graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(project().edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    const item = fact.business_expectations![0];
    item.decision.frozen!.before.flow = "other-flow";
    item.after!.flow = "other-flow";
    mirrorRaw(snapshot, run, fact);
    expect(project().edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).not.toBe(
      "PASS",
    );
  });

  it.each(["changed", "missing", "flow"] as const)(
    "preserves an explicit failure and independently captured path for %s identity",
    (change) => {
      const { snapshot, run, selection } = recordedFixture();
      const fact = run.facts![0];
      const item = fact.business_expectations![0];
      if (change === "changed") item.after!.bindings!.order = "other-entity";
      else if (change === "missing") delete item.after!.bindings!.order;
      else item.after!.flow = "other-flow";
      item.comparison = {
        status: "FAIL",
        branch: item.decision.branch!,
        reason: "recorded identity failure",
        checks: [],
      };
      fact.verdict = "FAIL";
      mirrorRaw(snapshot, run, fact);
      const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
      expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual).toMatchObject({
        from_state: "start",
        to_state: "pending",
        verdict: "FAIL",
      });
    },
  );

  it("does not require continuity for a subject that had no pre-action binding", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const item = fact.business_expectations![0];
    delete item.decision.frozen!.before.bindings!.order;
    item.after!.bindings!.order = "newly-created-entity";
    mirrorRaw(snapshot, run, fact);
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
  });
});
