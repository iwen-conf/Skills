// Mirror of the Go snapshot DTOs (tools/arc-verify/snapshot.go and the types it embeds).
// TestSnapshotTypesMatchFrontend compares every interface below with the Go JSON fields.
export type BusinessView =
  | "overview"
  | "capabilities"
  | "boundaries"
  | "flows"
  | "verification"
  | "impact";
export type Verdict =
  | "PASS"
  | "PARTIAL"
  | "FAIL"
  | "NOT_RUN"
  | "BLOCKED"
  | "STALE"
  | "FLAKY"
  | "EXCLUDED";
export type Workspace = "model" | "verify" | "analyze";

export interface BusinessNode {
  id: string;
  kind: string;
  name: string;
}
export interface Relation {
  id: string;
  from: string;
  to: string;
  kind: string;
}
export interface Rule {
  id: string;
  action: string;
  actor: string;
  entry: string;
  oracle: string;
  conditions: string[];
  expected: string;
  rejected: string;
  source: string;
  description: string;
}
export interface Risk {
  id: string;
  description: string;
  action: string;
  origin?: string;
  reference?: string;
}
export interface Transition {
  edge: string;
  action: string;
  rule: string;
  preconditions: Record<string, boolean>;
  outcome: string;
  parallel_group?: string;
}
export interface Flow {
  id: string;
  capability: string;
  initial: string;
}
export interface Actor {
  alias: string;
  role: string;
}
export interface ObservationLayer {
  layer: string;
  oracle: string;
}
export interface TestDefinition {
  id: string;
  name: string;
  rule: string;
  capability?: string;
  kind: string;
  flow?: string;
  steps?: string[];
  layers?: ObservationLayer[];
  preconditions?: Record<string, boolean>;
  expected?: string;
  denied?: boolean;
  depends_on?: string[];
  actors?: Actor[];
  read_only?: boolean;
  source: string;
  source_ref: string;
  pending_reason?: string;
}
export interface Model {
  id: string;
  scope: string;
  nodes: BusinessNode[];
  edges: Relation[];
  rules: Rule[];
  risks: Risk[];
  schema_version?: number;
  system?: string;
  previous?: string;
  contracts?: unknown[];
  guards?: unknown[];
  constraints?: unknown[];
  transitions?: Transition[];
  flows?: Flow[];
  invariants?: unknown[];
  tests?: TestDefinition[];
  shared_owners?: unknown[];
  applicability?: unknown[];
  history?: unknown[];
  equivalence?: unknown;
  reference_bindings?: unknown[];
}

export interface Readiness {
  described: boolean;
  implemented: boolean;
  verified: boolean;
  live: boolean;
  implementation_status: string;
  deployment_status: string;
}
export interface ProjectedNode extends BusinessNode {
  status: Verdict;
  reason: string;
  required: number;
  passed: number;
  excluded: number;
  history_failures: number;
  readiness: Readiness;
  parent?: string;
  domain?: string;
  description?: string;
}
export interface Obligation {
  definition: TestDefinition;
  targets: string[];
  transition?: string;
  reachability?: unknown;
  excluded?: boolean;
  exclusion_reason?: string;
  status: Verdict;
  reason: string;
  run?: string;
  history_failures: number;
  planned: boolean;
}
export interface Measure {
  total: number;
  passed: number;
  excluded: number;
  percent: number;
}
export interface Coverage {
  obligations: Measure;
  capabilities: Measure;
  behaviors: Measure;
  branches: Measure;
  invariants: Measure;
}
export interface Diagnostic {
  code: string;
  severity: string;
  subjects: string[];
  reason: string;
  source_ref?: string;
}
export interface ProofContext {
  graph: string;
  environment: string;
  target: string;
  source_commit: string;
  source_sha256: string;
  config_sha256: string;
  graph_sha256: string;
  source_root: string;
  config_path: string;
}
export interface Projection {
  graph: string;
  view: BusinessView;
  root?: string;
  environment?: string;
  round?: string;
  nodes: ProjectedNode[];
  edges: Relation[];
  coverage: Coverage;
  obligations: Obligation[];
  diagnostics: Diagnostic[];
  verdict: Verdict;
  reason: string;
  context: ProofContext;
  total: number;
  offset: number;
  next_offset: number;
  truncated: boolean;
  history_failures: number;
}

export interface ModelEntry {
  id: string;
  scope: string;
  schema_version: number;
  system: string;
  approved: boolean;
}
export interface RoundEntry {
  id: string;
  graph: string;
  environment: string;
  created: string;
  validity: string;
}
export interface RunnerEntry {
  id: string;
  graph: string;
  test: string;
  sha256: string;
  validity: string;
  scope: string;
}
export interface EnvironmentEntry {
  name: string;
  approved: boolean;
  target?: string;
  reason?: string;
}
export interface FixtureEntry {
  id: string;
  kind: string;
  environment: string;
  role?: string;
  scope?: string;
}
export interface RunEntry {
  id: string;
  graph: string;
  task: string;
  environment: string;
  round: string;
  status: string;
  created: string;
}
export interface Catalog {
  models: ModelEntry[];
  rounds: RoundEntry[];
  runners: RunnerEntry[];
  fixtures: FixtureEntry[];
  runs: RunEntry[];
  environments: EnvironmentEntry[];
  environment_error?: string;
  truncated: boolean;
}

export interface Recovery {
  verdict: string;
  reason: string;
  state?: string;
  safe: boolean;
  raw?: string;
  sha256?: string;
  actions: Record<string, unknown>[];
}
export interface TraceStep {
  step: string;
  expected: string;
  observed: string;
  verdict: string;
  decision: string;
  raw: string;
  sha256: string;
  node?: string;
  transition?: string;
  observation?: Record<string, unknown>;
  evidence_status?: string;
}
export interface Trace {
  run: string;
  task: string;
  status: string;
  first_divergence?: string;
  latest_observed?: string;
  continue_decision: string;
  steps: TraceStep[];
  round?: string;
  recovery?: Recovery;
  evidence_status?: string;
}
export interface StepRaw {
  identity: Record<string, unknown>;
  phase?: string;
  run: string;
  step: number;
  expected: string;
  observed: string;
  preconditions: Record<string, boolean>;
  before?: string;
  after?: string;
  denied: boolean;
  safe_to_continue?: boolean;
  oracle: string;
  runner: string;
  exit_code: number;
  model?: Record<string, unknown>;
}
export interface Impact {
  graph: string;
  previous?: string;
  changed: string[];
  affected: string[];
  tests: string[];
  reasons: Record<string, string[]>;
  scope: string;
}

export interface SnapshotMeta {
  generated: string;
  version: string;
  argv: string[];
  graph: string;
  graph_sha256: string;
  environment?: string;
  round?: string;
}
export interface SnapshotModel {
  data: Model;
  approved: boolean;
  rules: Record<string, boolean>;
  sha256: string;
}
export interface SnapshotEvidence {
  run: string;
  phase: string;
  step: number;
  sha256?: string;
  data?: StepRaw;
  error?: string;
}
export interface SnapshotRun {
  trace: Trace;
  evidence: SnapshotEvidence[];
}
export interface Snapshot {
  meta: SnapshotMeta;
  catalog: Catalog;
  model: SnapshotModel;
  views: Record<BusinessView, Projection>;
  impact?: Impact;
  runs: SnapshotRun[];
}

export interface PositionedNode {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  parentId?: string;
  container: boolean;
}
