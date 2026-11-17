import { describe, expect, it } from "vite-plus/test";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { entityKey } from "./facts";
import type { UnifiedStepFact } from "./facts";
import { lineageFixture } from "./lineage-fixtures";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";
import { readSnapshotText, SnapshotError } from "./snapshot";

describe("independent Actual path audit", () => {
  it.each([
    [
      "a false state check",
      (fact: UnifiedStepFact) => {
        const item = fact.business_expectations![0];
        item.after!.state = "start";
        fact.oracles![0].after.state = "start";
        item.comparison!.checks[0].state_passed = false;
      },
    ],
    [
      "a false predicate check",
      (fact: UnifiedStepFact) => {
        fact.business_expectations![0].comparison!.checks[0].predicate.value = "FALSE";
      },
    ],
    [
      "an unknown predicate check",
      (fact: UnifiedStepFact) => {
        fact.business_expectations![0].comparison!.checks[0].predicate.value = "UNKNOWN";
      },
    ],
    [
      "missing outcome checks",
      (fact: UnifiedStepFact) => {
        fact.business_expectations![0].comparison!.checks = [];
      },
    ],
  ] as const)("does not display PASS when the captured comparison contains %s", (_, change) => {
    const { snapshot, run, selection } = recordedFixture();
    const before = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(before.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    const fact = run.facts![0];
    change(fact);
    mirrorRaw(snapshot, run, fact);
    try {
      const parsed = readSnapshotText(JSON.stringify(snapshot));
      const graph = graphFor(parsed, selection, branchOptions(parsed), parsed.runs[0]);
      expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).not.toBe(
        "PASS",
      );
    } catch (error) {
      if (!(error instanceof SnapshotError)) throw error;
    }
  });

  it("does not display a blocked action step as an Actual PASS", () => {
    const { snapshot, run, selection } = recordedFixture();
    run.facts![0].verdict = "BLOCKED";
    run.facts![0].reason = "independent completion obligation was not established";
    const parsed = readSnapshotText(JSON.stringify(snapshot));
    const graph = graphFor(parsed, selection, branchOptions(parsed), parsed.runs[0]);
    expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).not.toBe("PASS");
  });

  it.each(["changed", "missing", "flow"] as const)(
    "does not display PASS when the captured after identity is %s despite passing explicit checks",
    (change) => {
      const { snapshot, run, selection } = recordedFixture();
      const fact = run.facts![0];
      const item = fact.business_expectations![0];
      if (change === "changed") item.after!.bindings!.order = "other-business-entity";
      else if (change === "missing") delete item.after!.bindings!.order;
      else item.after!.flow = "different-business-flow";
      mirrorRaw(snapshot, run, fact);
      const parsed = readSnapshotText(JSON.stringify(snapshot));
      const graph = graphFor(parsed, selection, branchOptions(parsed), parsed.runs[0]);
      expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).not.toBe(
        "PASS",
      );
    },
  );

  it("does not use a bare alias collision when a scoped entity reference identifies a different namespace", () => {
    const { snapshot, run, selection } = recordedFixture();
    const declaration = lineageFixture();
    snapshot.business_lineage = declaration.business_lineage;
    run.business_lineage = declaration.runs[0].business_lineage;
    const fact = run.facts![2];
    const mapped = { ...fact.before_entities![0], alias: "mapped-order" };
    const unrelated = {
      ...fact.before_entities![0],
      owner_actor: "other-owner",
      owner_principal: "unrelated-principal",
    };
    fact.before_entities = [mapped, unrelated];
    fact.entities = structuredClone(fact.before_entities);
    for (const overlay of run.business_lineage) {
      overlay.evidence_sha256 = fact.evidence_sha256;
      for (const node of overlay.nodes)
        for (const value of node.values ?? [])
          if (value.subject === "order") value.entity_ref = structuredClone(mapped);
    }
    mirrorRaw(snapshot, run, fact);
    const parsed = readSnapshotText(JSON.stringify(snapshot));
    const selected = { ...selection, branch: "B-CALLBACK", step: "3", entity: entityKey(mapped) };
    const correct = graphFor(parsed, selected, branchOptions(parsed), parsed.runs[0]);
    expect(correct.edges.some((edge) => edge.actual?.step === "3")).toBe(true);
    const wrong = graphFor(
      parsed,
      { ...selected, entity: entityKey(unrelated) },
      branchOptions(parsed),
      parsed.runs[0],
    );
    expect(wrong.edges.some((edge) => edge.actual?.step === "3")).toBe(false);
  });
});
