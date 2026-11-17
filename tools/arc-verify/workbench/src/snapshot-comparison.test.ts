import { describe, expect, it } from "vite-plus/test";
import { recordedFixture } from "./state-overlay-fixtures";
import { readSnapshotText, SnapshotError } from "./snapshot";

describe("captured comparison structure", () => {
  it.each([
    ["missing predicate", (check: Record<string, unknown>): unknown => delete check.predicate],
    ["null predicate", (check: Record<string, unknown>): unknown => (check.predicate = null)],
    [
      "invalid truth",
      (check: Record<string, unknown>): unknown => (check.predicate = { value: "PASS" }),
    ],
    [
      "invalid unknown identity",
      (check: Record<string, unknown>): unknown =>
        (check.predicate = { value: "UNKNOWN", unknown: [null] }),
    ],
    [
      "invalid reason",
      (check: Record<string, unknown>): unknown =>
        (check.predicate = { value: "TRUE", reason: {} }),
    ],
    [
      "invalid state result",
      (check: Record<string, unknown>): unknown => (check.state_passed = "true"),
    ],
    [
      "invalid denial result",
      (check: Record<string, unknown>): unknown => (check.denied_passed = 1),
    ],
    [
      "missing assertion evaluation",
      (check: Record<string, unknown>): unknown =>
        (check.assertions = [{ id: "A", kind: "invariant", oracle: "db", source_ref: "rule" }]),
    ],
    [
      "non-object assertion",
      (check: Record<string, unknown>): unknown => (check.assertions = [null]),
    ],
  ] as const)("refuses %s before renderers access comparison fields", (_, change) => {
    const { snapshot, run } = recordedFixture();
    expect(() => readSnapshotText(JSON.stringify(snapshot))).not.toThrow();
    change(run.facts![0].business_expectations![0].comparison!.checks[0]);
    expect(() => readSnapshotText(JSON.stringify(snapshot))).toThrow(SnapshotError);
  });

  it("refuses malformed common-invariant evaluations without discarding valid comparisons", () => {
    const { snapshot, run } = recordedFixture();
    const comparison = run.facts![0].business_expectations![0].comparison!;
    comparison.invariant_checks = [
      {
        id: "I",
        kind: "invariant",
        oracle: "db",
        source_ref: "rule",
        evaluation: { value: "TRUE" },
      },
    ];
    expect(() => readSnapshotText(JSON.stringify(snapshot))).not.toThrow();
    Object.assign(comparison.invariant_checks[0], { evaluation: null });
    expect(() => readSnapshotText(JSON.stringify(snapshot))).toThrow(SnapshotError);
  });

  it("preserves a recorded early failure with no outcome checks and optional missing comparisons", () => {
    const { snapshot, run } = recordedFixture();
    const item = run.facts![0].business_expectations![0];
    item.comparison = {
      branch: item.decision.branch!,
      status: "FAIL",
      reason: "identity changed",
      checks: [],
    };
    expect(
      readSnapshotText(JSON.stringify(snapshot)).runs[0].facts![0].business_expectations![0]
        .comparison,
    ).toEqual(item.comparison);
    delete item.comparison;
    expect(() => readSnapshotText(JSON.stringify(snapshot))).not.toThrow();
  });
});
