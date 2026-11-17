import { describe, expect, it } from "vite-plus/test";
import { branchOptions } from "./explorer-model";
import { entityKey } from "./facts";
import type { UnifiedStepFact } from "./facts";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";

describe("observed transitions on the contract state graph", () => {
  it("adds scoped Actual edges while preserving every declared Expected edge", () => {
    const { snapshot, run, selection } = recordedFixture();
    const declared = graphFor(snapshot, { ...selection, run: "" }, branchOptions(snapshot));
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    const actual = graph.edges.filter((edge) => edge.kind === "actual_transition");
    expect(actual).toHaveLength(3);
    expect(graph.edges.filter((edge) => edge.kind !== "actual_transition")).toEqual(declared.edges);
    expect(actual.map((edge) => [edge.from, edge.to])).toEqual([
      ["start", "pending"],
      ["pending", "disabled"],
      ["disabled", "success"],
    ]);
    expect(actual[0].actual).toMatchObject({
      run: "synthetic-run-A",
      step: "1",
      actor: "merchant",
      action_index: 0,
      branch: "B-CREATE",
      verdict: "PASS",
    });
    expect(actual[2].actual?.verdict).toBe("FAIL");
    expect(graph.stateOverlay?.diagnostics).toEqual([]);
    expect(graph.edges.some((edge) => edge.kind === "recorded_order")).toBe(false);
  });

  it("retains an independently observed deviation outside the model without changing the frozen branch", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    fact.business_expectations![0].after!.state = "unmodeled-provider-state";
    fact.oracles![0].after.state = "unmodeled-provider-state";
    fact.business_expectations![0].comparison!.status = "FAIL";
    fact.business_expectations![0].comparison!.checks[0].state_passed = false;
    fact.verdict = "FAIL";
    mirrorRaw(snapshot, run, fact);
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    const actual = graph.edges.find((edge) => edge.actual?.step === "1")!;
    expect(actual).toBeDefined();
    expect(actual.actual).toMatchObject({
      branch: "B-CREATE",
      from_state: "start",
      to_state: "unmodeled-provider-state",
      verdict: "FAIL",
    });
    expect(graph.nodes.find((node) => node.id === actual.to)).toMatchObject({
      kind: "observed_state",
      actual: { branch: "B-CREATE", step: "1" },
    });
    expect(
      graph.edges.find((edge) => edge.kind === "expected_outcome" && edge.branch === "B-CREATE"),
    ).toMatchObject({ from: "start", to: "pending" });
  });

  it.each([
    [
      "missing poststate",
      (fact: UnifiedStepFact) => {
        delete fact.business_expectations![0].after;
      },
    ],
    [
      "missing frozen decision",
      (fact: UnifiedStepFact) => {
        delete fact.business_expectations![0].decision.frozen;
      },
    ],
    [
      "another actor's Oracle",
      (fact: UnifiedStepFact) => {
        fact.oracles![0].actor = "other";
      },
    ],
    [
      "another action's Oracle",
      (fact: UnifiedStepFact) => {
        fact.oracles![0].action_index = 9;
      },
    ],
    [
      "missing action receipt",
      (fact: UnifiedStepFact) => {
        fact.actions = [];
      },
    ],
    [
      "conflicting observed state",
      (fact: UnifiedStepFact) => {
        fact.oracles![0].after.state = "other-state";
      },
    ],
    [
      "damaged evidence",
      (fact: UnifiedStepFact) => {
        fact.evidence_status = "BLOCKED";
      },
    ],
    [
      "duplicated occurrence",
      (fact: UnifiedStepFact) => {
        fact.business_expectations!.push(structuredClone(fact.business_expectations![0]));
      },
    ],
  ] as const)("keeps %s diagnostic rather than inventing an Actual transition", (_, corrupt) => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    corrupt(fact);
    mirrorRaw(snapshot, run, fact);
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(graph.stateOverlay?.diagnostics.some((item) => item.step === "1")).toBe(true);
    expect(graph.edges.some((edge) => edge.actual?.step === "1")).toBe(false);
    expect(
      graph.edges.some((edge) => edge.kind === "expected_outcome" && edge.branch === "B-CREATE"),
    ).toBe(true);
  });

  it("rejects a fact poststate edited independently of its captured RAW mirror", () => {
    const { snapshot, run, selection } = recordedFixture();
    run.facts![0].business_expectations![0].after!.state = "borrowed-state";
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    expect(graph.stateOverlay?.diagnostics.some((item) => item.step === "1")).toBe(true);
    expect(graph.edges.some((edge) => edge.actual?.step === "1")).toBe(false);
  });

  it("does not borrow a different run or environment even when the business branch is equal", () => {
    const { snapshot, run, selection } = recordedFixture();
    for (const selected of [
      { ...selection, run: "synthetic-run-B" },
      { ...selection, environment: "another-environment" },
    ]) {
      const graph = graphFor(snapshot, selected, branchOptions(snapshot), run);
      expect(graph.edges.filter((edge) => edge.kind === "actual_transition")).toEqual([]);
      expect(graph.stateOverlay?.diagnostics.length).toBeGreaterThan(0);
    }
  });

  it("scopes the Actual path by complete before-entity identity, not the reused display ID", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const original = fact.before_entities![0];
    const other = { ...original, owner_principal: "another-owner" };
    fact.entities = [other];
    mirrorRaw(snapshot, run, fact);
    const matching = graphFor(
      snapshot,
      { ...selection, entity: entityKey(original) },
      branchOptions(snapshot),
      run,
    );
    expect(matching.edges.some((edge) => edge.actual?.step === "1")).toBe(true);
    const excluded = graphFor(
      snapshot,
      { ...selection, entity: entityKey(other) },
      branchOptions(snapshot),
      run,
    );
    expect(excluded.edges.some((edge) => edge.actual?.step === "1")).toBe(false);
    expect(excluded.stateOverlay?.diagnostics.length).toBeGreaterThan(0);
  });

  it("preserves same-actor concurrent occurrences and self-loops without ordering their actions", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const second = structuredClone(fact.business_expectations![0]);
    second.action_index = 7;
    second.request!.action_index = 7;
    second.request!.request_sha256 = "synthetic-request-second";
    second.after!.state = "start";
    second.comparison!.status = "FAIL";
    second.comparison!.checks[0].state_passed = false;
    fact.business_expectations!.push(second);
    fact.requests!.push(second.request!);
    fact.actions!.push({
      ...fact.actions![0],
      action_index: 7,
      request_sha256: "synthetic-request-second",
    });
    fact.oracles!.push({ ...structuredClone(fact.oracles![0]), action_index: 7 });
    fact.oracles![1].after.state = "start";
    mirrorRaw(snapshot, run, fact);
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), run);
    const edges = graph.edges.filter((edge) => edge.actual?.step === "1");
    expect(edges).toHaveLength(2);
    expect(new Set(edges.map((edge) => edge.id)).size).toBe(2);
    expect(new Set(edges.map((edge) => edge.actual!.occurrence)).size).toBe(2);
    expect(edges.find((edge) => edge.actual!.action_index === 7)).toMatchObject({
      from: "start",
      to: "start",
    });
    expect(graph.edges.some((edge) => edge.kind === "recorded_order")).toBe(false);
  });
});
