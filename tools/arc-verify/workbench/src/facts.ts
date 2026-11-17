import type {
  BusinessComparison,
  BusinessDecision,
  BusinessObservation,
  Predicate,
} from "./contract";
/** Read-only mirrors of the persisted observation contract; absence is never a positive fact. */
export interface ParticipantBinding {
  alias: string;
  role: string;
  account: string;
  principal: string;
  sha256: string;
}
export interface EntityBinding {
  alias: string;
  kind: string;
  id: string;
  owner_actor: string;
  owner_principal: string;
  channel_account_id?: string;
  source_step: number;
  source_field: string;
  metadata?: Record<string, string>;
}
export interface SessionBinding {
  scope: string;
  actor: string;
  principal: string;
  acquired_step: number;
  digest: string;
  active: boolean;
}
export interface ConditionReading {
  condition: string;
  subject_kind?: string;
  subject?: string;
  attribute?: string;
  expected_bool: boolean;
  actual_bool?: boolean;
  oracle_id?: string;
  path?: string;
  passed: boolean;
  observer_actor?: string;
  operator?: string;
  expected?: unknown;
  annotation_only?: boolean;
}
export interface WaitObservation {
  oracle_id: string;
  rule_source_ref: string;
  started: string;
  completed: string;
  attempts: number;
  max_wait_ms: number;
  poll_interval_ms: number;
  stable_window_ms?: number;
  timed_out: boolean;
  violated_intermediate?: boolean;
  final_state: string;
  observed_states: string[];
  actor?: string;
  step?: number;
  action_index?: number;
  node?: string;
  transition?: string;
  stable_since?: string;
}
export interface StepAssertion {
  kind: string;
  name: string;
  oracle_id?: string;
  subject?: string;
  expected?: string;
  actual?: string;
  operator?: string;
  verdict: string;
  reason: string;
}
export interface IndependentFacts {
  state: string;
  principal: string;
  role: string;
  scope: string;
  conditions: Record<string, boolean> | null;
  values: Record<string, unknown>;
}
export interface OracleObservation {
  actor: string;
  id: string;
  boundary: string;
  before: IndependentFacts;
  after: IndependentFacts;
  before_sha256: string;
  after_sha256: string;
  action_index?: number;
}
export interface ExtractedBinding {
  name: string;
  source: string;
  path: string;
  value?: string;
  sha256: string;
  redacted?: boolean;
  entity_kind?: string;
  entity_alias?: string;
  owner_actor?: string;
  owner_principal?: string;
  channel_account_id?: string;
  step: number;
  source_actor?: string;
  oracle_id?: string;
  action_index?: number;
}
export interface BusinessExpectationObservation {
  actor: string;
  action_index: number;
  decision: BusinessDecision;
  after?: BusinessObservation;
  comparison?: BusinessComparison;
  request?: RequestObservation;
}
export interface ModelObservation {
  recovery_mode?: string;
  evidence_version?: number;
  node: string;
  transition?: string;
  round: string;
  runner_sha256: string;
  code_sha256: string;
  input: unknown;
  participants: ParticipantBinding[];
  before_entities?: EntityBinding[];
  before_sessions?: SessionBinding[];
  oracles: OracleObservation[];
  actions: ActionObservation[];
  requests?: RequestObservation[];
  extractions?: ExtractedBinding[];
  entities?: EntityBinding[];
  sessions?: SessionBinding[];
  waits?: WaitObservation[];
  conditions?: ConditionReading[];
  assertions?: StepAssertion[];
  business_expectations?: BusinessExpectationObservation[];
  preparation?: PreparationExecution;
  error?: string;
}
export interface UnifiedStepFact {
  recovery_mode?: string;
  source_kind: string;
  run: string;
  seq: number;
  graph: string;
  task?: string;
  node?: string;
  transition?: string;
  rule_id?: string;
  actor?: string;
  account?: string;
  principal?: string;
  expected_state?: string;
  observed_state?: string;
  denied_expected?: boolean;
  denied_observed?: boolean;
  verdict: string;
  decision?: string;
  reason?: string;
  participants?: ParticipantBinding[];
  entities?: EntityBinding[];
  sessions?: SessionBinding[];
  before_entities?: EntityBinding[];
  before_sessions?: SessionBinding[];
  extractions?: ExtractedBinding[];
  conditions?: ConditionReading[];
  waits?: WaitObservation[];
  oracles?: OracleObservation[];
  actions?: ActionObservation[];
  requests?: RequestObservation[];
  assertions?: StepAssertion[];
  business_expectations?: BusinessExpectationObservation[];
  evidence_path?: string;
  evidence_sha256?: string;
  evidence_version?: number;
  evidence_status?: string;
  evidence_source?: string;
  preparation?: PreparationExecution;
}

export interface RequestObservation {
  actor: string;
  action_index: number;
  node: string;
  template_sha256: string;
  request_sha256: string;
  resolved_path?: string;
  resolved_input?: unknown;
  substitutions?: Record<string, string>;
  values: Record<string, unknown> | null;
}

export interface ActionObservation {
  action_index?: number;
  node: string;
  transition?: string;
  actor: string;
  principal: string;
  template_sha256?: string;
  resolved_path?: string;
  resolved_input?: unknown;
  substitutions?: Record<string, string>;
  request_sha256: string;
  response_sha256: string;
  response_values?: Record<string, unknown>;
  status_code?: number;
  exit_code: number;
  denied: boolean;
  started: string;
  completed: string;
}
interface PreparationStep {
  id: string;
  action: string;
  requires: Predicate;
  ensures: Predicate;
  predicted_effects: { subject: string; attribute: string; value: unknown }[];
  source_ref: string;
}
interface TruthReading {
  value: string;
  unknown?: string[];
  reason?: string;
}
export interface PreparationExecution {
  status: string;
  reason: string;
  contract_sha256: string;
  initial_observed: BusinessObservation;
  final_observed: BusinessObservation;
  plans: {
    status: string;
    reason: string;
    layers: { index: number; steps: PreparationStep[] }[];
    requires_readback: boolean;
    blocked: string[];
    gaps: string[];
  }[];
  steps: {
    index: number;
    before_read: number;
    after_read: number;
    planned: PreparationStep;
    before: BusinessObservation;
    requirements: TruthReading;
    dependencies: Record<string, TruthReading>;
    receipt: { action: ActionObservation; artifacts?: string[] };
    after?: BusinessObservation;
    ensures: TruthReading;
    status: string;
    reason: string;
  }[];
  readbacks: { index: number; phase: string; observation?: BusinessObservation; error?: string }[];
  observation_reads: number;
  explored_states: number;
  parallel_executed: boolean;
  target_ready: boolean;
  first_failure_index?: number;
}

/** Alias + stable ID prevents two distinct orders with the same display alias being merged. */
export const entityKey = (entity: EntityBinding) =>
  JSON.stringify([entity.alias, entity.id, entity.owner_principal]);
