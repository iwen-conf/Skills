import { describe, expect, it } from "vite-plus/test";
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
  it("includes expected-outcome fields even when they are absent from admission guards", () => {
    const contract = {
      version: 1,
      subjects: [{ id: "ledger", kind: "entity", attributes: [{ name: "balance", type: "integer", source_ref: "db:ledger" }] }],
      branches: [],
    };
    const branch = {
      id: "refund-eligible", action: "refund", rule: "R1", from_states: ["ready"],
      when: { op: "true" },
      expected: { mode: "deterministic" as const, outcomes: [{
        id: "accepted", state: "submitted",
        predicate: { op: "eq", left: { ref: { subject: "ledger", attribute: "balance" } },
          right: { literal: { type: "integer", value: 900 } } },
      }] },
      source: "spec", source_ref: "spec:R1", confirmed: true,
    };
    const rows = extractPreconditions(base, branch, contract);
    const balance = rows.find((item) => item.key === "ledger.balance");
    expect(balance?.reference.source_ref).toBe("db:ledger");
    expect(balance?.conditions).toEqual([]);
    expect(balance?.providers).toEqual([]);
  });
  it("keeps after-action expectations separate from before-action data", () => {
    const branch = {
      id: "refund-eligible", action: "refund", rule: "R1", from_states: ["ready"],
      when: { op: "true" },
      expected: { mode: "deterministic" as const, outcomes: [{
        id: "accepted", predicate: { op: "eq",
          left: { ref: { subject: "payment", attribute: "paid" } },
          right: { literal: { type: "boolean", value: true } } },
      }] },
      source: "spec", source_ref: "spec:R1", confirmed: true,
    };
    const rows = extractPreconditions(base, branch);
    expect(rows.filter((r) => r.key === "payment.paid").map((r) => r.phase)).toEqual(["after", "before"]);
  });
  it("does not fabricate prerequisites for an absent lineage", () => {
    expect(extractPreconditions({ candidates: [], readings: new Map(), reason: "missing" })).toEqual([]);
  });
});
