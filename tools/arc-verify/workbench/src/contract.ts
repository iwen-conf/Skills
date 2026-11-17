export interface ValueRef {
  subject: string;
  attribute: string;
}
export interface Operand {
  ref?: ValueRef;
  literal?: { type: string; value: unknown };
}
export interface Predicate {
  op: string;
  args?: Predicate[];
  left?: Operand;
  right?: Operand;
}
export interface BusinessOutcome {
  id: string;
  state?: string;
  denied?: boolean;
  predicate: Predicate;
  checks?: BusinessExpectedCheck[];
  next_allowed?: string[];
}
export interface BusinessExpectedCheck {
  id: string;
  kind: string;
  predicate: Predicate;
  oracle: string;
  source_ref: string;
}
export interface BusinessAssertionResult {
  id: string;
  kind: string;
  oracle: string;
  source_ref: string;
  evaluation: { value: string; unknown?: string[]; reason?: string };
}
export interface BusinessExpectation {
  mode: "deterministic" | "allowed_outcomes";
  outcomes: BusinessOutcome[];
  reason?: string;
  common_invariants?: BusinessExpectedCheck[];
}
export interface ContractBranch {
  id: string;
  action: string;
  rule: string;
  flow?: string;
  transition?: string;
  capability?: string;
  from_states: string[];
  obligations?: string[];
  when: Predicate;
  expected: BusinessExpectation;
  source: string;
  source_ref: string;
  confirmed: boolean;
}
export interface Preparation {
  id: string;
  action: string;
  requires: Predicate;
  ensures: Predicate;
  depends_on?: string[];
  effects: { subject: string; attribute: string; value: unknown }[];
  source: string;
  source_ref: string;
}
export interface BusinessContract {
  version: number;
  subjects: {
    id: string;
    kind: string;
    requires_binding?: boolean;
    attributes: { name: string; type: string }[];
  }[];
  branches: ContractBranch[];
  preparation?: Preparation[];
  coverage?: {
    scope: string;
    domains: { subject: string; attribute: string; values: unknown[] }[];
    max_cases: number;
    scopes?: { action: string; flow?: string; states: string[] }[];
  };
}
export interface BusinessObservation {
  flow?: string;
  state?: string;
  values: Record<string, unknown>;
  bindings?: Record<string, string>;
  sources?: Record<string, string[]>;
}
export interface FrozenBusinessExpectation {
  version: number;
  contract_sha256: string;
  branch: string;
  action: string;
  rule: string;
  flow?: string;
  transition?: string;
  before_sha256: string;
  before: BusinessObservation;
  expected: BusinessExpectation;
  sha256: string;
}
export interface BusinessDecision {
  status: string;
  reason: string;
  action: string;
  branch?: string;
  matches: string[];
  unknown?: string[];
  frozen?: FrozenBusinessExpectation;
}
export interface BusinessComparison {
  status: string;
  reason: string;
  branch: string;
  outcome?: string;
  checks: {
    outcome: string;
    state_passed?: boolean;
    denied_passed?: boolean;
    predicate: { value: string; unknown?: string[]; reason?: string };
    assertions?: BusinessAssertionResult[];
  }[];
  unknown?: string[];
  invariant_checks?: BusinessAssertionResult[];
  next_allowed?: string[];
}
export interface BusinessExecutionCoverage {
  status: string;
  round?: string;
  total: number;
  passed: number;
  failed: number;
  blocked: number;
  not_run: number;
  branches: {
    branch: string;
    status: string;
    reason: string;
    runs: string[];
    steps: string[];
    outcomes?: string[];
  }[];
  evidence_errors?: { run: string; reason: string }[];
  reason: string;
}
export interface BusinessCoverageProof {
  status: string;
  scope: string;
  contract_sha256?: string;
  proof_kind: string;
  budget: number;
  required_cases: number;
  checked_cases: number;
  complete_in_scope: boolean;
  domains?: { subject: string; attribute: string; values: unknown[] }[];
  issues: {
    status: string;
    action: string;
    flow?: string;
    values?: Record<string, unknown>;
    branches?: string[];
    reason: string;
  }[];
  reason: string;
}

const operators: Record<string, string> = {
  eq: "=",
  ne: "≠",
  lt: "<",
  lte: "≤",
  gt: ">",
  gte: "≥",
  in: "属于",
  contains: "包含",
  subset: "是子集",
  intersects: "有交集",
  between: "介于",
};
export function displayValue(value: unknown): string {
  if (value === undefined) return "未观察";
  if (value === null) return "null";
  if (value === true) return "true";
  if (value === false) return "false";
  return typeof value === "string" ? value : JSON.stringify(value);
}
function operandText(operand?: Operand): string {
  if (operand?.ref) return `${operand.ref.subject}.${operand.ref.attribute}`;
  return operand?.literal ? displayValue(operand.literal.value) : "未声明操作数";
}
/** Render the declared predicate. The browser never evaluates it or supplies a verdict. */
export function predicateText(predicate?: Predicate): string {
  if (!predicate) return "未声明";
  const op = predicate.op.toLowerCase();
  if (op === "and" || op === "or")
    return (
      (predicate.args ?? [])
        .map((arg) => `(${predicateText(arg)})`)
        .join(op === "and" ? " 且 " : " 或 ") || "空条件"
    );
  if (op === "not") return `非 (${predicateText(predicate.args?.[0])})`;
  if (op === "true" || op === "false") return op;
  return `${operandText(predicate.left)} ${operators[op] ?? predicate.op} ${operandText(predicate.right)}`;
}
