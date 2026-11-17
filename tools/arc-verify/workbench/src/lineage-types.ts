import type { Predicate, Preparation } from "./contract";
import type { EntityBinding } from "./facts";

export type LineageKind = "target" | "state" | "condition" | "logic" | "preparation" | "input_data" | "expected" | "assertion" | "invariant";
export type LineageStatus = "SATISFIED" | "UNSATISFIED" | "NOT_OBSERVED" | "BLOCKED" | "READY";
export interface BusinessLineageReference {
  subject: string;
  attribute: string;
  subject_kind: string;
  source_ref?: string;
}
export interface BusinessLineageNode {
  id: string;
  kind: LineageKind;
  name: string;
  branch: string;
  action?: string;
  preparation?: string;
  predicate?: Predicate;
  from_states?: string[];
  references?: BusinessLineageReference[];
  effects?: Preparation["effects"];
  source_ref: string;
  confirmed?: boolean;
}
export interface BusinessLineageEdge {
  id: string;
  from: string;
  to: string;
  kind: "requires" | "logical_operand" | "depends_on" | "candidate_provider" | "reads" | "expects" | "verifies";
  source_ref: string;
  alternative?: boolean;
}
export interface BusinessBranchLineage {
  branch: string;
  action: string;
  flow?: string;
  target: string;
  nodes: BusinessLineageNode[];
  edges: BusinessLineageEdge[];
}
export interface BusinessLineage {
  version: number;
  contract_sha256: string;
  branches: BusinessBranchLineage[];
}
export interface BusinessLineageSource {
  oracle: string;
  kind: "oracle" | "request_capture" | "preparation_readback";
  boundary?: string;
  sha256?: string;
}
export interface BusinessLineageValue {
  subject: string;
  subject_kind: string;
  attribute: string;
  source_ref?: string;
  binding?: string;
  actual?: unknown;
  sources: BusinessLineageSource[];
  entity_ref?: EntityBinding;
}
export interface BusinessLineageReading {
  node: string;
  status: LineageStatus;
  evaluation?: LineageEvaluation;
  values?: BusinessLineageValue[];
  observed_state?: string;
  reason?: string;
}
export interface LineageEvaluation {
  value: string;
  unknown?: string[];
  reason?: string;
}
export interface BusinessLineageOverlay {
  branch: string;
  run: string;
  step: number;
  action_index: number;
  actor: string;
  phase: "before" | "preparation";
  evidence_status: string;
  nodes: BusinessLineageReading[];
  readback_index?: number;
  readback_phase?: string;
  evidence_sha256?: string;
  diagnostic?: boolean;
  reason?: string;
}
