import { describe, expect, it } from "vitest";
import { extractPreconditions } from "./PreconditionLedger";
import type { LineageSelection } from "./lineage";

const base: LineageSelection = {
  candidates: [],
  readings: new Map(),
  reason: "declaration only",
  graph: {
    branch: "refund-eligible",
    action: "refund",
    target: "target",
    nodes: [
      { id: "target", branch: "refund-eligible", kind: "target", name: "refund", source_ref: "rule:refund", references: [] },
      { id: "condition", branch: "refund-eligible", kind: "condition", name: "eq", source_ref: "rule:refund", references: [
        { subject: "payment", subject_kind: "entity", attribute: "paid", source_ref: "db:payment" },
        { subject: "refund", subject_kind: "entity", attribute: "amount", source_ref: "db:refund" },
      ] },
      { id: "prep", branch: "refund-eligible", kind: "preparation", name: "Confirm payment", source_ref: "rule:prepare" },
      { id: "unrelated", branch: "refund-eligible", kind: "preparation", name: "Similar attribute name", source_ref: "rule:other" },
    ],
    edges: [
      { id: "prep-condition", from: "prep", to: "condition", kind: "candidate_provider", source_ref: "rule:prepare" },
      { id: "condition-target", from: "condition", to: "target", kind: "requires", source_ref: "rule:refund" },
    ],
  },
};

describe("source-first precondition inventory", () => {
  it("enumerates declared subject attributes with their own provenance", () => {
    const rows = extractPreconditions(base);
    expect(rows.map((row) => row.key)).toEqual(["payment.paid", "refund.amount"]);
    expect(rows[0].reference.source_ref).toBe("db:payment");
    expect(rows[1].reference.source_ref).toBe("db:refund");
  });
  it("uses only declared candidate edges, never same-name inference", () => {
    const rows = extractPreconditions(base);
    expect(rows[0].providers.map((node) => node.id)).toEqual(["prep"]);
    expect(rows[0].providers.some((node) => node.id === "unrelated")).toBe(false);
  });
  it("does not fabricate prerequisites for an absent lineage", () => {
    expect(extractPreconditions({ candidates: [], readings: new Map(), reason: "missing" })).toEqual([]);
  });
});
