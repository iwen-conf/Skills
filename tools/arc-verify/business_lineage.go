package main

import (
	"encoding/json"
	"errors"
)

var errBusinessLineageBudget = errors.New("business lineage export work budget exceeded")

// Lineage is a read-only projection of the versioned contract. Candidate
// providers describe declared effects, never execution or observed state.
type BusinessLineage struct {
	Version        int                     `json:"version"`
	ContractSHA256 string                  `json:"contract_sha256"`
	Branches       []BusinessBranchLineage `json:"branches"`
}

type BusinessBranchLineage struct {
	Branch string                `json:"branch"`
	Action string                `json:"action"`
	Flow   string                `json:"flow,omitempty"`
	Target string                `json:"target"`
	Nodes  []BusinessLineageNode `json:"nodes"`
	Edges  []BusinessLineageEdge `json:"edges"`
}

type BusinessLineageReference struct {
	Subject     string `json:"subject"`
	SubjectKind string `json:"subject_kind"`
	Attribute   string `json:"attribute"`
	SourceRef   string `json:"source_ref,omitempty"`
}

type BusinessLineageNode struct {
	ID          string                     `json:"id"`
	Kind        string                     `json:"kind"`
	Name        string                     `json:"name"`
	Branch      string                     `json:"branch"`
	Action      string                     `json:"action,omitempty"`
	Preparation string                     `json:"preparation,omitempty"`
	Predicate   *BusinessPredicate         `json:"predicate,omitempty"`
	FromStates  []string                   `json:"from_states,omitempty"`
	References  []BusinessLineageReference `json:"references,omitempty"`
	Effects     []PreparationEffect        `json:"effects,omitempty"`
	SourceRef   string                     `json:"source_ref"`
	Confirmed   *bool                      `json:"confirmed,omitempty"`
}

type BusinessLineageEdge struct {
	ID          string `json:"id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Kind        string `json:"kind"`
	SourceRef   string `json:"source_ref"`
	Alternative bool   `json:"alternative,omitempty"`
}

// Every source below inherits this exact run/step/action/actor scope. A
// same-named Oracle on another account or action is a different observation.
type BusinessLineageOverlay struct {
	Branch         string                   `json:"branch"`
	Run            string                   `json:"run"`
	Step           int                      `json:"step"`
	ActionIndex    int                      `json:"action_index"`
	Actor          string                   `json:"actor"`
	Phase          string                   `json:"phase"`
	ReadbackIndex  *int                     `json:"readback_index,omitempty"`
	ReadbackPhase  string                   `json:"readback_phase,omitempty"`
	EvidenceSHA256 string                   `json:"evidence_sha256,omitempty"`
	Diagnostic     bool                     `json:"diagnostic,omitempty"`
	Reason         string                   `json:"reason,omitempty"`
	EvidenceStatus string                   `json:"evidence_status"`
	Nodes          []BusinessLineageReading `json:"nodes"`
}

type BusinessLineageReading struct {
	Node          string                 `json:"node"`
	Status        string                 `json:"status"`
	Evaluation    *TruthEvaluation       `json:"evaluation,omitempty"`
	Values        []BusinessLineageValue `json:"values,omitempty"`
	ObservedState string                 `json:"observed_state,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
}

type BusinessLineageValue struct {
	Subject     string                  `json:"subject"`
	SubjectKind string                  `json:"subject_kind"`
	Attribute   string                  `json:"attribute"`
	SourceRef   string                  `json:"source_ref,omitempty"`
	Binding     string                  `json:"binding,omitempty"`
	EntityRef   *EntityBinding          `json:"entity_ref,omitempty"`
	Actual      json.RawMessage         `json:"actual,omitempty"`
	Sources     []BusinessLineageSource `json:"sources"`
}

type BusinessLineageSource struct {
	Oracle   string `json:"oracle"`
	Kind     string `json:"kind"`
	Boundary string `json:"boundary,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}
