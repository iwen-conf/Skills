import type { BusinessCampaign, BusinessCampaignVariant } from "./campaign";
import { contractFixture } from "./test-fixtures";

/** Synthetic UI regression only; these are not executed campaigns or product E2E evidence. */
export function campaignFixture() {
  const snapshot = contractFixture();
  snapshot.runs[0].trace.steps = snapshot.runs[0].trace.steps.slice(0, 1);
  snapshot.runs[0].facts = snapshot.runs[0].facts!.slice(0, 1);
  snapshot.runs[0].runner_id = "synthetic-create-100";
  snapshot.runs[0].runner_sha256 = "synthetic-create-100-sha";
  snapshot.runs[1].runner_id = "synthetic-create-200";
  snapshot.runs[1].runner_sha256 = "synthetic-create-200-sha";
  const variants: BusinessCampaignVariant[] = [
    {
      id: "V-100",
      branch: "B-CREATE",
      runner: "synthetic-create-100",
      runner_sha256: "synthetic-create-100-sha",
      actor: "merchant",
      visits: 1,
      passes: 1,
      last_run: "synthetic-run-A",
    },
    {
      id: "V-200",
      branch: "B-CREATE",
      runner: "synthetic-create-200",
      runner_sha256: "synthetic-create-200-sha",
      actor: "merchant",
      visits: 1,
      passes: 0,
      last_run: "synthetic-run-B",
    },
    {
      id: "V-CALLBACK",
      branch: "B-CALLBACK",
      runner: "synthetic-callback",
      actor: "merchant",
      visits: 0,
      passes: 0,
    },
  ];
  const budget = { max_actions: 8, max_states: 6, deadline_ms: 20000 };
  const campaign: BusinessCampaign = {
    version: 1,
    id: "synthetic-campaign-A",
    graph: snapshot.meta.graph,
    flow: "F-PAY",
    environment: snapshot.meta.environment!,
    round: snapshot.meta.round!,
    status: "BUDGET_EXHAUSTED",
    reason: "SYNTHETIC UI REGRESSION ONLY — next candidate cannot reserve its action budget",
    evidence_status: "PASS",
    identity: {
      graph: snapshot.meta.graph,
      task: "synthetic-campaign",
      environment: snapshot.meta.environment!,
      target: "isolated-ui-fixture",
      source_commit: "synthetic-source",
      source_sha256: "synthetic-source-sha",
      config_sha256: "synthetic-config-sha",
      graph_sha256: snapshot.meta.graph_sha256,
      actor: "merchant",
      account: "synthetic-account-A",
      principal: "principal-A",
      account_sha256: "synthetic-account-sha",
      fixture: "synthetic-fixture",
      fixture_sha256: "synthetic-fixture-sha",
    },
    request: {
      graph: snapshot.meta.graph,
      flow: "F-PAY",
      environment: snapshot.meta.environment!,
      round: snapshot.meta.round!,
      commit: "synthetic-source",
      root: "synthetic-root",
      config: "synthetic-config",
      account: "synthetic-account-A",
      fixture: "synthetic-fixture",
      variants: structuredClone(variants),
      budget,
    },
    variants,
    frontier: variants.map((variant, index) => ({
      variant_id: variant.id,
      branch: variant.branch,
      runner: variant.runner,
      run: `synthetic-reserved-${index}`,
      eligibility: index === 2 ? "PREPARABLE" : index === 1 ? "NEXT_DISALLOWED" : "READY",
      reason:
        index === 2
          ? "synthetic preparation needed with independent readback"
          : index === 1
            ? "synthetic next action restriction"
            : "synthetic ready candidate",
      state_key: `synthetic-state-${index}`,
      before: {
        state: "pending",
        flow: "F-PAY",
        values: { "request.amount": index ? 200 : 100, "merchant.enabled": true },
        bindings: { merchant: "candidate-private-marker" },
        sources: { "merchant.enabled": ["synthetic-db-oracle"] },
      },
      input: {
        actor: "merchant",
        action_index: 0,
        node: index === 2 ? "callback" : "create",
        template_sha256: "synthetic-request-template",
        request_sha256: "synthetic-request-sha",
        values: { "request.amount": index ? 200 : 100 },
      },
      decision: {
        status: "MATCHED",
        reason: "synthetic candidate decision",
        action: index === 2 ? "callback" : "create",
        matches: [variant.branch],
      },
      preparation:
        index === 2
          ? {
              status: "PLANNED",
              reason: "synthetic dependency path",
              observed_sha256: "synthetic-before-sha",
              layers: [
                {
                  index: 0,
                  steps: [
                    {
                      id: "P-order",
                      action: "create",
                      requires: { op: "true" },
                      ensures: { op: "true" },
                      predicted_effects: [{ subject: "order", attribute: "inflight", value: true }],
                      source_ref: "synthetic fixture",
                    },
                  ],
                },
              ],
              already_satisfied: [],
              blocked: [],
              gaps: [],
              explored_states: 2,
              max_states: 6,
              max_steps: 2,
              requires_readback: true,
            }
          : undefined,
      preparation_before:
        index === 2
          ? {
              state: "start",
              flow: "F-PAY",
              values: { "order.exists": false },
              sources: { "order.exists": ["synthetic-preparation-db"] },
            }
          : undefined,
      next_allowed: index !== 1,
      selected: index === 0,
    })),
    visits: snapshot.runs.map((run, index) => ({
      sequence: index + 1,
      variant_id: variants[index].id,
      branch: variants[index].branch,
      runner: variants[index].runner,
      run: run.trace.run,
      status: index ? "FAIL" : "PASS",
      reason: index ? "synthetic first divergence retained" : "synthetic observed result",
      dispatch_sha256: `synthetic-dispatch-${index}`,
      before: run.facts![0].business_expectations![0].decision.frozen!.before,
      after: run.facts![0].business_expectations![0].after,
      comparison: run.facts![0].business_expectations![0].comparison,
      actor: "merchant",
      principal: run.facts![0].principal,
      entities: run.facts![0].entities,
      recovery: {
        mode: "checkpoint",
        verdict: "PASS",
        reason: "synthetic checkpoint preserves the scene",
        state: "pending",
        safe: true,
        actions: [],
      },
    })),
    budget,
    spent: {
      actions: 3,
      reserved_actions: 2,
      dispatches: 2,
      target_actions: 2,
      preparation_actions: 1,
      recovery_actions: 0,
      states: 2,
      observations: 4,
      elapsed_ms: 45,
    },
    coverage: {
      declared_branches: 3,
      bound_branches: 2,
      selected_variants: 3,
      attempted_variants: 2,
      passed_variants: 1,
      unmapped_branches: ["B-DISABLE"],
      selected_variants_covered: false,
      full_business_coverage: false,
    },
    last_actual: {
      flow: "F-PAY",
      state: "pending",
      values: { "latest.scope": "latest-not-selected-run" },
    },
    last_run: "synthetic-run-B",
    next_allowed: ["callback"],
    first_divergence: "synthetic-run-B",
    history_sha256: "synthetic-history-sha",
    revision: 3,
    created: "2026-11-17T00:00:00Z",
    updated: "2026-11-17T00:00:01Z",
  };
  const other = structuredClone(campaign);
  other.id = "synthetic-campaign-other";
  other.environment = "other-ui-environment";
  other.identity.environment = other.environment;
  other.request.environment = other.environment;
  other.identity.principal = "other-campaign-private-marker";
  other.visits = [];
  snapshot.business_campaigns = [campaign, other];
  snapshot.available_campaigns = 2;
  snapshot.campaigns_truncated = false;
  return snapshot;
}
