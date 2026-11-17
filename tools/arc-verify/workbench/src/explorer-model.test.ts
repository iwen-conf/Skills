import { describe, expect, it } from "vite-plus/test";
import { branchOptions, executionCoverageScope, selectionContext } from "./explorer-model";
import { graphFor } from "./graph-model";
import { initialSelection, matchingRuns, reduceSelection } from "./selection";
import { contractFixture } from "./test-fixtures";

describe("snapshot-to-workbench identity", () => {
  it("uses captured participant aliases instead of exposing the aggregate role as another account", () => {
    const snapshot = contractFixture();
    snapshot.runs[0].facts![0].actor = "MERCHANT_ROLE";
    const context = selectionContext(snapshot);
    expect(context.runs[0].steps[0].actors).toEqual(["merchant"]);
  });
  it("keeps aggregate branch coverage bound to its recorded environment and round", () => {
    const snapshot = contractFixture();
    snapshot.contract_execution = {
      status: "PASS",
      round: "synthetic-round",
      total: 1,
      passed: 1,
      failed: 0,
      blocked: 0,
      not_run: 0,
      branches: [],
      reason: "synthetic branch-only coverage",
    };
    const state = {
      ...initialSelection(snapshot.meta.graph, "isolated-ui-fixture"),
      run: "synthetic-run-A",
    };
    expect(executionCoverageScope(snapshot, state).appliesToSelection).toBe(true);
    expect(
      executionCoverageScope(snapshot, { ...state, environment: "different-environment" })
        .appliesToSelection,
    ).toBe(false);
    snapshot.runs[0].trace.round = "older-round";
    expect(executionCoverageScope(snapshot, state).reason).toContain("所选运行不属于此覆盖轮次");
  });
  it("attaches a continuous run only through persisted branch decisions", () => {
    const snapshot = contractFixture();
    const context = selectionContext(snapshot);
    const state = reduceSelection(
      initialSelection(snapshot.meta.graph),
      { type: "branch", value: "B-CALLBACK" },
      context,
    );
    expect(matchingRuns(context, state).map((run) => run.id)).toEqual(["synthetic-run-A"]);
    expect(context.runs[0].steps.map((step) => step.branches)).toEqual([
      ["B-CREATE"],
      ["B-DISABLE"],
      ["B-CALLBACK"],
    ]);
  });
  it("keeps an UNKNOWN or unassociated run visible as diagnostics without inferring a branch from its node", () => {
    const snapshot = contractFixture();
    snapshot.runs[1].branch_id = undefined;
    snapshot.runs[1].facts![0].business_expectations = [
      {
        actor: "merchant",
        action_index: 0,
        decision: {
          status: "UNKNOWN",
          action: "create",
          reason: "missing subject observation",
          matches: [],
          unknown: ["merchant.enabled"],
        },
      },
    ];
    const branches = branchOptions(snapshot);
    const context = selectionContext(snapshot, branches);
    expect(branches.some((branch) => branch.unresolved)).toBe(true);
    const state = reduceSelection(
      initialSelection(snapshot.meta.graph),
      { type: "branch", value: "B-CREATE" },
      context,
    );
    expect(matchingRuns(context, state).map((run) => run.id)).toEqual(["synthetic-run-A"]);
    const diagnostic = reduceSelection(
      state,
      { type: "branch", value: "unresolved:records" },
      context,
    );
    expect(matchingRuns(context, diagnostic).map((run) => run.id)).toEqual(["synthetic-run-B"]);
  });
  it("labels gaps in a filtered run rather than drawing a fabricated adjacent step", () => {
    const snapshot = contractFixture();
    const context = selectionContext(snapshot);
    const state = { ...initialSelection(snapshot.meta.graph), canvas: "run" as const };
    const graph = graphFor(
      snapshot,
      state,
      branchOptions(snapshot),
      snapshot.runs[0],
      context.runs[0].steps.filter((step) => step.id !== "2"),
    );
    expect(graph.edges[0].label).toContain("省略 1");
  });
  it("does not attach a predicate to a transition from a different declared state", () => {
    const snapshot = contractFixture();
    snapshot.model.data.business_contract!.branches[2].transition = undefined;
    const state = { ...initialSelection(snapshot.meta.graph), flow: "F-PAY" };
    const graph = graphFor(snapshot, state, branchOptions(snapshot));
    expect(graph.edges.find((edge) => edge.id === "T-CALLBACK")?.label).toContain(
      "merchant.enabled",
    );
    expect(graph.edges.find((edge) => edge.id === "T-RETURN")?.label).not.toContain(
      "merchant.enabled",
    );
  });
  it("does not color a stored PASS as verified when its evidence is blocked", () => {
    const snapshot = contractFixture();
    const run = snapshot.runs[0];
    run.facts![0].evidence_status = "BLOCKED";
    const context = selectionContext(snapshot);
    const graph = graphFor(
      snapshot,
      { ...initialSelection(snapshot.meta.graph), canvas: "run" },
      branchOptions(snapshot),
      run,
      context.runs[0].steps,
    );
    expect(graph.nodes[0].status).toBe("BLOCKED");
    expect(graph.nodes[0].detail).toContain("记载 PASS / 证据 BLOCKED");
  });
});
