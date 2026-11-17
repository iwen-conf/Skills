import sample from "../dev/sample-snapshot.json";
import type { BusinessExpectation, ContractBranch, Predicate } from "./contract";
import type { UnifiedStepFact } from "./facts";
import type { Snapshot } from "./types";

const eq = (subject: string, attribute: string, value: unknown): Predicate => ({
  op: "eq",
  left: { ref: { subject, attribute } },
  right: {
    literal: {
      type:
        typeof value === "boolean" ? "boolean" : typeof value === "number" ? "integer" : "string",
      value,
    },
  },
});
const expectation = (state: string, predicate: Predicate): BusinessExpectation => ({
  mode: "deterministic",
  outcomes: [{ id: state, state, denied: false, predicate }],
});

/** Synthetic UI regression data only. No pay_gateway execution, approvals or real accounts. */
export function contractFixture(): Snapshot {
  const snapshot = structuredClone(sample) as unknown as Snapshot;
  const graph = "tool-business-chain-synthetic-v1";
  snapshot.meta = {
    ...snapshot.meta,
    graph,
    graph_sha256: "synthetic-ui-fixture-not-a-production-fingerprint",
    environment: "isolated-ui-fixture",
    round: "synthetic-round",
  };
  const nodes = [
    { id: "system", kind: "system", name: "工具自测 · 支付模型【fixture】" },
    { id: "domain", kind: "domain", name: "账号与在途业务【fixture】" },
    { id: "capability", kind: "capability", name: "在途回调处理【fixture】" },
    { id: "create", kind: "action", name: "创建隔离订单【fixture】" },
    { id: "disable", kind: "action", name: "停用测试账号【fixture】" },
    { id: "callback", kind: "action", name: "处理在途回调【fixture】" },
    { id: "start", kind: "state", name: "尚未创建" },
    { id: "pending", kind: "state", name: "订单在途" },
    { id: "disabled", kind: "state", name: "账号已停用 · 订单在途" },
    { id: "retry", kind: "state", name: "回调待重试" },
    { id: "success", kind: "state", name: "回调入账完成" },
  ];
  const branches: ContractBranch[] = [
    {
      id: "B-CREATE",
      action: "create",
      rule: "R-CREATE",
      flow: "F-PAY",
      transition: "T-CREATE",
      from_states: ["start"],
      when: eq("merchant", "enabled", true),
      expected: expectation("pending", eq("order", "inflight", true)),
      source: "tool-self-test",
      source_ref: "src/test-fixtures.ts",
      confirmed: true,
    },
    {
      id: "B-DISABLE",
      action: "disable",
      rule: "R-DISABLE",
      flow: "F-PAY",
      transition: "T-DISABLE",
      from_states: ["pending"],
      when: eq("order", "inflight", true),
      expected: expectation("disabled", eq("merchant", "enabled", false)),
      source: "tool-self-test",
      source_ref: "src/test-fixtures.ts",
      confirmed: true,
    },
    {
      id: "B-CALLBACK",
      action: "callback",
      rule: "R-CALLBACK",
      flow: "F-PAY",
      transition: "T-CALLBACK",
      from_states: ["disabled"],
      when: { op: "and", args: [eq("merchant", "enabled", false), eq("order", "inflight", true)] },
      expected: expectation("success", eq("order", "settlements", 1)),
      source: "tool-self-test",
      source_ref: "src/test-fixtures.ts",
      confirmed: true,
    },
  ];
  const stateEdges = [
    { id: "T-CREATE", from: "start", to: "pending", kind: "transitions_to" },
    { id: "T-DISABLE", from: "pending", to: "disabled", kind: "transitions_to" },
    { id: "T-CALLBACK", from: "disabled", to: "success", kind: "transitions_to" },
    { id: "T-RETRY", from: "disabled", to: "retry", kind: "transitions_to" },
    { id: "T-RETURN", from: "retry", to: "disabled", kind: "transitions_to" },
  ];
  snapshot.model = {
    approved: true,
    sha256: snapshot.meta.graph_sha256,
    rules: Object.fromEntries(branches.map((branch) => [branch.rule, true])),
    data: {
      id: graph,
      schema_version: 2,
      scope: "SYNTHETIC UI REGRESSION ONLY — 不代表 pay_gateway 业务规则或运行结果",
      nodes,
      edges: [
        { id: "contain-domain", from: "system", to: "domain", kind: "contains" },
        { id: "contain-capability", from: "domain", to: "capability", kind: "contains" },
        ...["create", "disable", "callback"].map((id) => ({
          id: `implements-${id}`,
          from: "capability",
          to: id,
          kind: "implements",
        })),
        ...stateEdges,
      ],
      rules: branches.map((branch) => ({
        id: branch.rule,
        action: branch.action,
        actor: "MERCHANT",
        entry: "synthetic",
        oracle: "isolated-db",
        conditions: [],
        expected: branch.expected.outcomes[0].state!,
        rejected: "denied",
        source: "tool-self-test",
        description: branch.id,
      })),
      risks: [],
      tests: [],
      flows: [{ id: "F-PAY", capability: "capability", initial: "start" }],
      transitions: stateEdges.map((edge) => ({
        edge: edge.id,
        action: branches.find((branch) => branch.transition === edge.id)?.action ?? "callback",
        rule: branches.find((branch) => branch.transition === edge.id)?.rule ?? "R-CALLBACK",
        preconditions: {},
        outcome: "success",
      })),
      business_contract: {
        version: 1,
        subjects: [
          {
            id: "merchant",
            kind: "account",
            requires_binding: true,
            attributes: [{ name: "enabled", type: "boolean" }],
          },
          {
            id: "order",
            kind: "order",
            requires_binding: true,
            attributes: [
              { name: "inflight", type: "boolean" },
              { name: "settlements", type: "integer" },
            ],
          },
        ],
        branches,
        preparation: ["login", "account", "order", "disabled"].map((id, index, all) => ({
          id: `P-${id}`,
          action: index < 2 ? "create" : index === 2 ? "disable" : "callback",
          requires: { op: "true" },
          ensures: { op: "true" },
          depends_on: index ? [...(index === 3 ? ["P-login"] : []), `P-${all[index - 1]}`] : [],
          effects: [],
          source: "tool-self-test",
          source_ref: "src/test-fixtures.ts",
        })),
      },
    },
  };
  for (const projection of Object.values(snapshot.views)) {
    projection.graph = graph;
    projection.environment = snapshot.meta.environment;
    projection.round = snapshot.meta.round;
    projection.context.graph = graph;
    projection.context.graph_sha256 = snapshot.meta.graph_sha256;
    projection.context.environment = snapshot.meta.environment!;
    projection.nodes = nodes.map((node) => ({
      ...node,
      status: "NOT_RUN",
      reason: "工具自测模型",
      required: 1,
      passed: 0,
      excluded: 0,
      history_failures: 0,
      readiness: {
        described: true,
        implemented: false,
        verified: false,
        live: false,
        implementation_status: "synthetic",
        deployment_status: "not-deployed",
      },
    }));
    projection.edges = snapshot.model.data.edges;
    projection.obligations = [];
    projection.diagnostics = [];
    projection.total = nodes.length;
    projection.truncated = false;
  }
  const makeFact = (run: string, seq: number, principal: string): UnifiedStepFact => {
    const branch = branches[seq - 1];
    const before = {
      flow: "F-PAY",
      state: seq === 1 ? "start" : seq === 2 ? "pending" : "disabled",
      values: { "merchant.enabled": seq < 3, "order.inflight": seq > 1, "order.settlements": 0 },
      bindings: { merchant: principal, order: `order-${principal}` },
    };
    const after = {
      flow: "F-PAY",
      state: branch.expected.outcomes[0].state,
      values: {
        "merchant.enabled": seq < 2,
        "order.inflight": seq < 3,
        "order.settlements": seq === 3 ? 1 : 0,
      },
    };
    return {
      source_kind: "action",
      run,
      seq,
      graph,
      node: branch.action,
      transition: branch.transition,
      rule_id: branch.rule,
      actor: "merchant",
      account: `account-${principal}`,
      principal,
      expected_state: branch.expected.outcomes[0].state,
      observed_state: branch.expected.outcomes[0].state,
      verdict: seq === 3 ? "FAIL" : "PASS",
      decision: "synthetic recorded decision",
      evidence_status: "PASS",
      evidence_version: 2,
      evidence_source: "synthetic-ui-fixture",
      participants: [
        {
          alias: "merchant",
          role: "MERCHANT",
          account: `account-${principal}`,
          principal,
          sha256: "synthetic",
        },
      ],
      entities: [
        {
          alias: "order",
          kind: "payment",
          id: `order-${principal}`,
          owner_actor: "merchant",
          owner_principal: principal,
          source_step: 1,
          source_field: "response.order_id",
        },
      ],
      sessions: [
        {
          scope: "isolated",
          actor: "merchant",
          principal,
          acquired_step: 1,
          digest: "synthetic-digest",
          active: seq < 2,
        },
      ],
      conditions: [
        {
          condition: "merchant-enabled",
          subject_kind: "account",
          subject: "merchant",
          attribute: "enabled",
          expected_bool: seq < 3,
          actual_bool: seq < 3,
          passed: true,
          observer_actor: "merchant",
          oracle_id: "isolated-db",
          operator: "eq",
        },
      ],
      waits:
        seq === 3
          ? [
              {
                actor: "merchant",
                step: seq,
                action_index: 0,
                node: "callback",
                oracle_id: "isolated-db",
                rule_source_ref: "src/test-fixtures.ts",
                started: "2026-11-17T00:00:00Z",
                completed: "2026-11-17T00:00:01Z",
                attempts: 3,
                max_wait_ms: 5000,
                poll_interval_ms: 250,
                stable_window_ms: 500,
                timed_out: false,
                final_state: "success",
                observed_states: ["disabled", "success"],
              },
            ]
          : [],
      oracles: [
        {
          actor: "merchant",
          action_index: 0,
          id: "isolated-db",
          boundary: "synthetic-isolated-database",
          before: {
            state: before.state,
            principal,
            role: "MERCHANT",
            scope: "isolated",
            conditions: { "merchant-enabled": seq < 3 },
            values: before.values,
          },
          after: {
            state: after.state!,
            principal,
            role: "MERCHANT",
            scope: "isolated",
            conditions: { "merchant-enabled": seq < 2 },
            values: after.values,
          },
          before_sha256: "synthetic-before",
          after_sha256: "synthetic-after",
        },
      ],
      assertions: [
        {
          kind: "invariant",
          name: "isolated-settlement-count",
          oracle_id: "isolated-db",
          subject: "order",
          expected: "1",
          actual: seq === 3 ? "2" : "1",
          operator: "eq",
          verdict: seq === 3 ? "FAIL" : "PASS",
          reason: seq === 3 ? "synthetic invariant divergence retained" : "synthetic assertion",
        },
      ],
      business_expectations: [
        {
          actor: "merchant",
          action_index: 0,
          decision: {
            status: "READY",
            reason: "synthetic unique branch",
            action: branch.action,
            branch: branch.id,
            matches: [branch.id],
            frozen: {
              version: 1,
              contract_sha256: "synthetic-contract",
              branch: branch.id,
              action: branch.action,
              rule: branch.rule,
              flow: branch.flow,
              transition: branch.transition,
              before_sha256: "synthetic-before",
              before,
              expected: branch.expected,
              sha256: "synthetic-frozen",
            },
          },
          after,
          comparison: {
            status: "PASS",
            reason: "synthetic expected result matches; independent invariant is separate",
            branch: branch.id,
            outcome: branch.expected.outcomes[0].id,
            checks: [
              {
                outcome: branch.expected.outcomes[0].id,
                state_passed: true,
                denied_passed: true,
                predicate: { value: "TRUE" },
              },
            ],
          },
        },
      ],
    };
  };
  snapshot.runs = [
    {
      trace: {
        run: "synthetic-run-A",
        task: "synthetic-continuous-chain",
        status: "FAIL",
        first_divergence: "3",
        continue_decision: "stop",
        round: "synthetic-round",
        steps: [1, 2, 3].map((seq) => ({
          step: String(seq),
          expected: branches[seq - 1].expected.outcomes[0].state!,
          observed: branches[seq - 1].expected.outcomes[0].state!,
          node: branches[seq - 1].action,
          verdict: seq === 3 ? "FAIL" : "PASS",
          decision: "synthetic",
          raw: "synthetic",
          sha256: "synthetic",
        })),
      },
      evidence: [],
      facts: [1, 2, 3].map((seq) => makeFact("synthetic-run-A", seq, "principal-A")),
      environment: "isolated-ui-fixture",
      flow_id: "F-PAY",
      evidence_status: "PASS",
      evidence_errors: [],
    },
    {
      trace: {
        run: "synthetic-run-B",
        task: "synthetic-other-account",
        status: "PASS",
        continue_decision: "stop",
        round: "synthetic-round",
        steps: [
          {
            step: "1",
            node: "create",
            expected: "pending",
            observed: "pending",
            verdict: "PASS",
            decision: "synthetic",
            raw: "synthetic",
            sha256: "synthetic",
          },
        ],
      },
      evidence: [],
      facts: [makeFact("synthetic-run-B", 1, "principal-B-private-marker")],
      branch_id: "B-CREATE",
      flow_id: "F-PAY",
      environment: "isolated-ui-fixture",
      evidence_status: "PASS",
      evidence_errors: [],
    },
  ];
  snapshot.catalog.runs = snapshot.runs.map((run) => ({
    id: run.trace.run,
    graph,
    task: run.trace.task,
    environment: "isolated-ui-fixture",
    round: "synthetic-round",
    status: run.trace.status,
    created: "2026-11-17T00:00:00Z",
  }));
  snapshot.catalog.rounds = [
    {
      id: "synthetic-round",
      graph,
      environment: "isolated-ui-fixture",
      created: "2026-11-17T00:00:00Z",
      validity: "synthetic",
    },
  ];
  snapshot.catalog.models = [
    {
      id: graph,
      scope: "synthetic UI regression only",
      schema_version: 2,
      system: "fixture",
      approved: true,
    },
  ];
  snapshot.available_runs = 2;
  snapshot.runs_truncated = false;
  snapshot.contract_analysis = {
    status: "INCOMPLETE",
    scope: "synthetic fixture, not business completeness",
    proof_kind: "finite_domain",
    budget: 16,
    required_cases: 8,
    checked_cases: 4,
    complete_in_scope: false,
    issues: [
      { status: "UNSPECIFIED", action: "callback", reason: "synthetic missing branch retained" },
    ],
    reason: "工具自测，未完成全域覆盖",
  };
  delete snapshot.exploration;
  delete snapshot.impact;
  return snapshot;
}
