import { contractFixture } from "./test-fixtures";
import type {
  BusinessLineageNode,
  BusinessLineageEdge,
  BusinessLineageOverlay,
} from "./lineage-types";

/** Synthetic wire-contract regressions only; never a product execution example. */
export function lineageFixture() {
  const snapshot = contractFixture();
  const branch = snapshot.model.data.business_contract!.branches[2];
  const source_ref = "src/lineage-fixtures.ts";
  const declarations: Omit<BusinessLineageNode, "branch" | "source_ref">[] = [
    { id: "target", kind: "target", name: "处理在途回调", action: branch.action, confirmed: true },
    { id: "pre-state", kind: "state", name: "前态", from_states: ["disabled"] },
    { id: "guard", kind: "logic", name: "AND", predicate: branch.when },
    {
      id: "account-condition",
      kind: "condition",
      name: "merchant.enabled = false",
      predicate: branch.when.args![0],
      references: [{ subject: "merchant", attribute: "enabled", subject_kind: "account" }],
    },
    {
      id: "order-condition",
      kind: "condition",
      name: "order.inflight = true",
      predicate: branch.when.args![1],
      references: [{ subject: "order", attribute: "inflight", subject_kind: "order" }],
    },
    {
      id: "provider-a",
      kind: "preparation",
      name: "停用账号 · 入口 A",
      preparation: "P-disable-a",
      action: "disable",
      predicate: branch.when.args![0],
      effects: [{ subject: "merchant", attribute: "enabled", value: false }],
    },
    {
      id: "provider-b",
      kind: "preparation",
      name: "停用账号 · 入口 B",
      preparation: "P-disable-b",
      action: "disable",
      predicate: branch.when.args![0],
      effects: [{ subject: "merchant", attribute: "enabled", value: false }],
    },
  ];
  const nodes = declarations.map((node) => ({ ...node, branch: branch.id, source_ref }));
  const accountRef = { subject: "merchant", attribute: "enabled", subject_kind: "account" };
  const orderRef = { subject: "order", attribute: "inflight", subject_kind: "order" };
  for (const node of nodes) {
    if (node.kind === "target" || node.id === "guard") node.references = [accountRef, orderRef];
    if (node.kind === "preparation") node.references = [accountRef];
  }
  const relations: Omit<BusinessLineageEdge, "source_ref">[] = [
    { id: "state-required", from: "pre-state", to: "target", kind: "requires" },
    { id: "guard-required", from: "guard", to: "target", kind: "requires" },
    { id: "account-and", from: "account-condition", to: "guard", kind: "logical_operand" },
    { id: "order-and", from: "order-condition", to: "guard", kind: "logical_operand" },
    {
      id: "provider-a-candidate",
      from: "provider-a",
      to: "account-condition",
      kind: "candidate_provider",
      alternative: true,
    },
    {
      id: "provider-b-candidate",
      from: "provider-b",
      to: "account-condition",
      kind: "candidate_provider",
      alternative: true,
    },
  ];
  const edges = relations.map((edge) => ({ ...edge, source_ref }));
  const overlay: BusinessLineageOverlay = {
    branch: branch.id,
    run: snapshot.runs[0].trace.run,
    step: 3,
    action_index: 0,
    actor: "merchant",
    phase: "before",
    evidence_status: "PASS",
    nodes: [
      {
        node: "target",
        status: "READY",
        reason: "synthetic preconditions observed",
        evaluation: { value: "TRUE" },
      },
      {
        node: "pre-state",
        status: "SATISFIED",
        observed_state: "disabled",
        evaluation: { value: "TRUE" },
      },
      { node: "guard", status: "SATISFIED", evaluation: { value: "TRUE" } },
      {
        node: "account-condition",
        status: "SATISFIED",
        evaluation: { value: "TRUE" },
        values: [
          {
            subject: "merchant",
            subject_kind: "account",
            attribute: "enabled",
            binding: "principal-A",
            actual: false,
            sources: [{ oracle: "isolated-db", kind: "oracle", sha256: "synthetic-before" }],
          },
        ],
      },
      {
        node: "order-condition",
        status: "SATISFIED",
        evaluation: { value: "TRUE" },
        values: [
          {
            subject: "order",
            subject_kind: "order",
            attribute: "inflight",
            binding: "order-principal-A",
            actual: true,
            sources: [{ oracle: "isolated-db", kind: "oracle", sha256: "synthetic-before" }],
          },
        ],
      },
      { node: "provider-a", status: "NOT_OBSERVED" },
      { node: "provider-b", status: "NOT_OBSERVED" },
    ],
  };
  const values = overlay.nodes.flatMap((reading) => reading.values ?? []);
  for (const value of values) value.sources[0].boundary = "synthetic-isolated-database";
  for (const reading of overlay.nodes) {
    const refs = nodes.find((node) => node.id === reading.node)?.references ?? [];
    if (refs.length && !reading.values)
      reading.values = refs.map((ref) =>
        structuredClone(
          values.find(
            (value) => value.subject === ref.subject && value.attribute === ref.attribute,
          )!,
        ),
      );
  }
  snapshot.runs[0].facts![2].business_expectations![0].decision.frozen!.before.sources = {
    "merchant.enabled": ["isolated-db"],
    "order.inflight": ["isolated-db"],
  };
  const run = Object.assign(snapshot.runs[0], { business_lineage: [overlay] });
  return Object.assign(snapshot, {
    business_lineage: {
      version: 1,
      contract_sha256: "synthetic-contract",
      branches: [
        {
          branch: branch.id,
          action: branch.action,
          flow: branch.flow,
          target: "target",
          nodes,
          edges,
        },
        ...snapshot.model.data.business_contract!.branches.slice(0, 2).map((other) => ({
          branch: other.id,
          action: other.action,
          flow: other.flow,
          target: `${other.id}-target`,
          nodes: [
            {
              id: `${other.id}-target`,
              kind: "target",
              name: other.id,
              branch: other.id,
              action: other.action,
              source_ref,
              confirmed: true,
              references: other.when.left?.ref
                ? [
                    {
                      ...other.when.left.ref,
                      subject_kind:
                        other.when.left.ref.subject === "merchant" ? "account" : "order",
                    },
                  ]
                : [],
            },
            {
              id: `${other.id}-state`,
              kind: "state",
              name: "前态",
              branch: other.id,
              source_ref,
              from_states: other.from_states,
            },
            {
              id: `${other.id}-guard`,
              kind: "condition",
              name: "条件",
              branch: other.id,
              source_ref,
              predicate: other.when,
              references: other.when.left?.ref
                ? [
                    {
                      ...other.when.left.ref,
                      subject_kind:
                        other.when.left.ref.subject === "merchant" ? "account" : "order",
                    },
                  ]
                : [],
            },
          ],
          edges: [
            {
              id: `${other.id}-state-required`,
              from: `${other.id}-state`,
              to: `${other.id}-target`,
              kind: "requires",
              source_ref,
            },
            {
              id: `${other.id}-guard-required`,
              from: `${other.id}-guard`,
              to: `${other.id}-target`,
              kind: "requires",
              source_ref,
            },
          ],
        })),
      ],
    },
    runs: [run, Object.assign(snapshot.runs[1], { business_lineage: [] as (typeof overlay)[] })],
  });
}
