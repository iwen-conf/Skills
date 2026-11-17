import type {
  BusinessComparison,
  BusinessDecision,
  BusinessObservation,
  Predicate,
} from "./contract";
import type {
  ActionObservation,
  EntityBinding,
  ModelObservation,
  PreparationExecution,
  RequestObservation,
} from "./facts";

/** Mirrors business_exploration.go. The browser displays persisted decisions; it does not plan or execute. */
export interface BusinessCampaignBudget {
  max_actions: number;
  max_states: number;
  deadline_ms: number;
}
export interface BusinessCampaignSpent {
  dispatches: number;
  actions: number;
  reserved_actions: number;
  target_actions: number;
  preparation_actions: number;
  recovery_actions: number;
  states: number;
  observations: number;
  elapsed_ms: number;
}
export interface BusinessCampaignVariant {
  id: string;
  branch: string;
  runner: string;
  task?: string;
  action?: string;
  actor?: string;
  runner_sha256?: string;
  code_sha256?: string;
  visits: number;
  passes: number;
  last_run?: string;
}
export interface BusinessCampaignRequest {
  campaign?: string;
  graph: string;
  flow: string;
  environment: string;
  round: string;
  commit: string;
  root: string;
  config: string;
  account: string;
  fixture: string;
  accounts?: Record<string, string>;
  variants: BusinessCampaignVariant[] | null;
  budget: BusinessCampaignBudget;
}
export interface ExecutionIdentity {
  graph: string;
  task: string;
  environment: string;
  target: string;
  source_commit: string;
  source_sha256: string;
  config_sha256: string;
  graph_sha256: string;
  actor: string;
  account: string;
  principal: string;
  account_sha256: string;
  fixture: string;
  fixture_sha256: string;
}
export interface PreparationPlan {
  status: string;
  reason: string;
  observed_sha256: string;
  observed_readback?: number;
  layers:
    | {
        index: number;
        steps: {
          id: string;
          action: string;
          requires: Predicate;
          ensures: Predicate;
          predicted_effects: { subject: string; attribute: string; value: unknown }[];
          source_ref: string;
        }[];
      }[]
    | null;
  already_satisfied: string[] | null;
  blocked: string[] | null;
  gaps: string[] | null;
  explored_states: number;
  max_states: number;
  max_steps: number;
  requires_readback: boolean;
}
export interface BusinessCampaignCandidate {
  variant_id: string;
  branch: string;
  runner: string;
  run: string;
  eligibility: string;
  reason: string;
  state_key?: string;
  before?: BusinessObservation;
  observation?: ModelObservation;
  input?: RequestObservation;
  decision?: BusinessDecision;
  preparation?: PreparationPlan;
  preparation_before?: BusinessObservation;
  preparation_observation?: ModelObservation;
  next_allowed: boolean;
  selected: boolean;
}
export interface BusinessCampaignVisit {
  sequence: number;
  variant_id: string;
  branch: string;
  runner: string;
  run: string;
  status: string;
  reason: string;
  before_state_key?: string;
  after_state_key?: string;
  dispatch_sha256: string;
  before?: BusinessObservation;
  after?: BusinessObservation;
  comparison?: BusinessComparison;
  actor?: string;
  principal?: string;
  entities?: EntityBinding[];
  preparation?: PreparationExecution;
  recovery?: {
    mode?: string;
    verdict: string;
    reason: string;
    state?: string;
    safe: boolean;
    raw?: string;
    sha256?: string;
    actions: ActionObservation[];
  };
}
export interface BusinessCampaignCoverage {
  declared_branches: number;
  bound_branches: number;
  selected_variants: number;
  attempted_variants: number;
  passed_variants: number;
  unmapped_branches: string[] | null;
  selected_variants_covered: boolean;
  full_business_coverage: boolean;
}
export interface BusinessCampaignDispatchReference {
  sequence: number;
  run: string;
  sha256: string;
}
export interface BusinessCampaign {
  version: number;
  id: string;
  graph: string;
  flow: string;
  environment: string;
  round: string;
  status: string;
  reason: string;
  evidence_status: string;
  identity: ExecutionIdentity;
  request: BusinessCampaignRequest;
  variants: BusinessCampaignVariant[] | null;
  frontier: BusinessCampaignCandidate[] | null;
  visits: BusinessCampaignVisit[] | null;
  dispatches?: BusinessCampaignDispatchReference[] | null;
  budget: BusinessCampaignBudget;
  spent: BusinessCampaignSpent;
  coverage: BusinessCampaignCoverage;
  last_actual?: BusinessObservation;
  last_run?: string;
  next_allowed?: string[];
  first_divergence?: string;
  history_sha256: string;
  revision: number;
  created: string;
  updated: string;
}

/** A checked load can retain only trusted row identity when the ledger body is damaged. */
export function isCampaignDiagnostic(campaign: BusinessCampaign): boolean {
  return (
    campaign.status === "BLOCKED" &&
    campaign.evidence_status === "BLOCKED" &&
    !campaign.variants?.length &&
    !campaign.frontier?.length &&
    !campaign.visits?.length
  );
}
