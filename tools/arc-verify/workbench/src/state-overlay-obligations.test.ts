import { describe, expect, it } from "vite-plus/test";
import type { BusinessComparison } from "./contract";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";

function obligatedFixture() {
  const fixture = recordedFixture();
  const fact = fixture.run.facts![0];
  const item = fact.business_expectations![0];
  const check = {
    id: "persisted",
    kind: "persistence",
    oracle: "isolated-db",
    source_ref: "confirmed-rule",
    predicate: { op: "true" },
  };
  item.decision.frozen!.expected.outcomes[0].checks = [check];
  item.decision.frozen!.expected.common_invariants = [
    { ...check, id: "identity", kind: "invariant" },
  ];
  item.comparison!.checks[0].assertions = [{ ...check, evaluation: { value: "TRUE" } }];
  item.comparison!.invariant_checks = [
    { ...check, id: "identity", kind: "invariant", evaluation: { value: "TRUE" } },
  ];
  mirrorRaw(fixture.snapshot, fixture.run, fact);
  return { ...fixture, fact, item };
}

describe("completeness of recorded Actual obligations", () => {
  it.each([
    [
      "state result",
      (comparison: BusinessComparison): unknown => delete comparison.checks[0].state_passed,
    ],
    [
      "denial result",
      (comparison: BusinessComparison): unknown => delete comparison.checks[0].denied_passed,
    ],
    [
      "outcome assertion",
      (comparison: BusinessComparison): unknown => delete comparison.checks[0].assertions,
    ],
    [
      "common invariant",
      (comparison: BusinessComparison): unknown => delete comparison.invariant_checks,
    ],
    [
      "assertion provenance",
      (comparison: BusinessComparison): unknown =>
        (comparison.checks[0].assertions![0].source_ref = "other-rule"),
    ],
    [
      "unknown invariant",
      (comparison: BusinessComparison): unknown =>
        (comparison.invariant_checks![0].evaluation.value = "UNKNOWN"),
    ],
  ] as const)("cannot display PASS without the required %s", (_, change) => {
    const { snapshot, run, selection, fact, item } = obligatedFixture();
    const project = () => graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(project().edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    change(item.comparison!);
    mirrorRaw(snapshot, run, fact);
    expect(project().edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).not.toBe(
      "PASS",
    );
  });

  it("rejects a claimed passing denial check contradicted by the recorded response", () => {
    const { snapshot, run, selection, fact } = obligatedFixture();
    const project = () => graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(project().edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    fact.actions![0].denied = true;
    mirrorRaw(snapshot, run, fact);
    expect(project().edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).not.toBe(
      "PASS",
    );
  });

  it.each(["FALSE", "UNKNOWN"])(
    "preserves an allowed set with one proven outcome and a %s alternative",
    (value) => {
      const { snapshot, run, selection, fact, item } = obligatedFixture();
      const expected = item.decision.frozen!.expected;
      expected.mode = "allowed_outcomes";
      expected.outcomes.push({ id: "another-permitted-outcome", predicate: { op: "false" } });
      item.comparison!.checks.push({ outcome: "another-permitted-outcome", predicate: { value } });
      delete item.comparison!.outcome;
      mirrorRaw(snapshot, run, fact);
      const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
      expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    },
  );

  it("retains independently observed states for an explicit early comparison failure", () => {
    const { snapshot, run, selection, fact, item } = obligatedFixture();
    item.comparison = {
      branch: item.decision.branch!,
      status: "FAIL",
      reason: "independently recorded identity change",
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
  });
});
