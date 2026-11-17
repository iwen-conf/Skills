import { describe, expect, it } from "vite-plus/test";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";
import { readSnapshotText, SnapshotError } from "./snapshot";

describe("Actual state path scope and malformed evidence", () => {
  it.each(["environment", "branch", "flow"] as const)("rejects an unbound %s selection", (key) => {
    const { snapshot, run, selection } = recordedFixture();
    if (key === "environment") {
      run.environment = undefined;
      selection.environment = "";
      for (const fact of run.facts!) mirrorRaw(snapshot, run, fact);
    } else selection[key] = "not-in-this-run";
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(graph.stateOverlay?.occurrences).toEqual([]);
    expect(graph.stateOverlay?.diagnostics.length).toBeGreaterThan(0);
  });

  it.each(["participants", "before_entities", "entities"] as const)(
    "rejects malformed %s at the snapshot boundary before any renderer can dereference it",
    (key) => {
      const { snapshot, run } = recordedFixture();
      (run.facts![0][key] as unknown[])[0] = null;
      mirrorRaw(snapshot, run, run.facts![0]);
      expect(() => readSnapshotText(JSON.stringify(snapshot))).toThrow(SnapshotError);
    },
  );

  it("does not allow a comparison labelled PASS to contradict its own frozen state check", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    fact.business_expectations![0].after!.state = "start";
    fact.oracles![0].after.state = "start";
    mirrorRaw(snapshot, run, fact);
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(graph.stateOverlay?.diagnostics.some((item) => item.step === "1")).toBe(true);
    expect(graph.edges.some((edge) => edge.actual?.step === "1")).toBe(false);
  });
});
