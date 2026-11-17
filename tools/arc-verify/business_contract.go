package main

import "encoding/json"

// BusinessContract is the executable, explicitly sourced business meaning of
// a graph. The capability tree and cyclic state graph stay in Graph; only
// preparation dependencies below are a DAG. Its absence is legacy scope,
// never a declaration of complete deterministic business verification.
type BusinessContract struct {
	Version     int                     `json:"version"`
	Subjects    []BusinessSubject       `json:"subjects"`
	Branches    []BusinessBranch        `json:"branches"`
	Preparation []PreparationAction     `json:"preparation,omitempty"`
	Coverage    *BusinessCoverageDomain `json:"coverage,omitempty"`
}

type BusinessSubject struct {
	ID              string              `json:"id"`
	Kind            string              `json:"kind"`
	RequiresBinding bool                `json:"requires_binding,omitempty"`
	Attributes      []BusinessAttribute `json:"attributes"`
}

type BusinessAttribute struct {
	Name string `json:"name"`
	// boolean, string, integer, number, string_set, integer_set, number_set.
	Type string `json:"type"`
	// Encoding is an optional source-backed transport constraint. json_i64
	// requires a JSON integer token in the signed 64-bit range, without decimal
	// or exponent notation. An unqualified integer retains mathematical value
	// semantics, so existing contracts do not silently change meaning.
	Encoding  string `json:"encoding,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
}

type BusinessAttributeRef struct {
	Subject   string `json:"subject"`
	Attribute string `json:"attribute"`
}

func (ref BusinessAttributeRef) Key() string { return ref.Subject + "." + ref.Attribute }

type BusinessLiteral struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type BusinessOperand struct {
	Ref     *BusinessAttributeRef `json:"ref,omitempty"`
	Literal *BusinessLiteral      `json:"literal,omitempty"`
}

// Predicate operands are typed references or typed JSON literals. Logical
// operators use Args; comparisons use Left/Right. Missing observations use
// three-valued logic, never false-by-default. Null is an unknown observation.
type BusinessPredicate struct {
	Op    string              `json:"op"`
	Args  []BusinessPredicate `json:"args,omitempty"`
	Left  *BusinessOperand    `json:"left,omitempty"`
	Right *BusinessOperand    `json:"right,omitempty"`
}

type BusinessBranch struct {
	ID          string              `json:"id"`
	Action      string              `json:"action"`
	Rule        string              `json:"rule"`
	Flow        string              `json:"flow,omitempty"`
	Transition  string              `json:"transition,omitempty"`
	Capability  string              `json:"capability,omitempty"`
	FromStates  []string            `json:"from_states"`
	Obligations []string            `json:"obligations,omitempty"`
	When        BusinessPredicate   `json:"when"`
	Expected    BusinessExpectation `json:"expected"`
	Source      string              `json:"source"`
	SourceRef   string              `json:"source_ref"`
	Confirmed   bool                `json:"confirmed"`
}

type BusinessExpectation struct {
	// deterministic requires exactly one outcome; allowed_outcomes explicitly
	// declares a bounded acceptable set and requires a source-backed reason.
	Mode             string                    `json:"mode"`
	Outcomes         []BusinessExpectedOutcome `json:"outcomes"`
	Reason           string                    `json:"reason,omitempty"`
	CommonInvariants []BusinessExpectedCheck   `json:"common_invariants,omitempty"`
}

type BusinessExpectedOutcome struct {
	ID          string                  `json:"id"`
	State       string                  `json:"state,omitempty"`
	Denied      *bool                   `json:"denied,omitempty"`
	Predicate   BusinessPredicate       `json:"predicate"`
	Checks      []BusinessExpectedCheck `json:"checks,omitempty"`
	NextAllowed BusinessSuccessorPolicy `json:"next_allowed,omitzero"`
}

type BusinessExpectedCheck struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Predicate BusinessPredicate `json:"predicate"`
	Oracle    string            `json:"oracle"`
	SourceRef string            `json:"source_ref"`
}

type BusinessAssertionResult struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Oracle     string          `json:"oracle"`
	SourceRef  string          `json:"source_ref"`
	Evaluation TruthEvaluation `json:"evaluation"`
}

// Values are independent observations keyed by declared subject.attribute.
// Bindings associate stable model aliases with actual account/object IDs.
// They are identity facts, not credentials. Empty or missing values are unknown.
type BusinessObservation struct {
	Flow     string                     `json:"flow,omitempty"`
	State    string                     `json:"state,omitempty"`
	Values   map[string]json.RawMessage `json:"values"`
	Bindings map[string]string          `json:"bindings,omitempty"`
	Sources  map[string][]string        `json:"sources,omitempty"`
}

type BusinessTruth string

const (
	businessTrue    BusinessTruth = "TRUE"
	businessFalse   BusinessTruth = "FALSE"
	businessUnknown BusinessTruth = "UNKNOWN"
)

type TruthEvaluation struct {
	Value   BusinessTruth `json:"value"`
	Unknown []string      `json:"unknown,omitempty"`
	Reason  string        `json:"reason,omitempty"`
}

type FrozenBusinessExpectation struct {
	Version        int                 `json:"version"`
	ContractSHA256 string              `json:"contract_sha256"`
	Branch         string              `json:"branch"`
	Action         string              `json:"action"`
	Rule           string              `json:"rule"`
	Flow           string              `json:"flow,omitempty"`
	Transition     string              `json:"transition,omitempty"`
	BeforeSHA256   string              `json:"before_sha256"`
	Before         BusinessObservation `json:"before"`
	Expected       BusinessExpectation `json:"expected"`
	SHA256         string              `json:"sha256"`
}

type BusinessDecision struct {
	Status  string                     `json:"status"`
	Reason  string                     `json:"reason"`
	Action  string                     `json:"action"`
	Branch  string                     `json:"branch,omitempty"`
	Matches []string                   `json:"matches"`
	Unknown []string                   `json:"unknown,omitempty"`
	Frozen  *FrozenBusinessExpectation `json:"frozen,omitempty"`
}

type BusinessCheckResult struct {
	Outcome      string                    `json:"outcome"`
	StatePassed  *bool                     `json:"state_passed,omitempty"`
	DeniedPassed *bool                     `json:"denied_passed,omitempty"`
	Predicate    TruthEvaluation           `json:"predicate"`
	Assertions   []BusinessAssertionResult `json:"assertions,omitempty"`
}

type BusinessComparison struct {
	Status          string                    `json:"status"`
	Reason          string                    `json:"reason"`
	Branch          string                    `json:"branch"`
	Outcome         string                    `json:"outcome,omitempty"`
	Checks          []BusinessCheckResult     `json:"checks"`
	Unknown         []string                  `json:"unknown,omitempty"`
	InvariantChecks []BusinessAssertionResult `json:"invariant_checks,omitempty"`
	NextAllowed     BusinessSuccessorPolicy   `json:"next_allowed,omitzero"`
}

type BusinessCoverageDomain struct {
	Scope    string                  `json:"scope"`
	Domains  []BusinessFiniteDomain  `json:"domains"`
	MaxCases int                     `json:"max_cases"`
	Scopes   []BusinessCoverageScope `json:"scopes"`
}

type BusinessCoverageScope struct {
	Action string   `json:"action"`
	Flow   string   `json:"flow,omitempty"`
	States []string `json:"states"`
}

type BusinessFiniteDomain struct {
	Subject   string            `json:"subject"`
	Attribute string            `json:"attribute"`
	Values    []json.RawMessage `json:"values"`
}

type BusinessCoverageIssue struct {
	Status   string                     `json:"status"`
	Action   string                     `json:"action"`
	Flow     string                     `json:"flow,omitempty"`
	State    string                     `json:"state,omitempty"`
	Values   map[string]json.RawMessage `json:"values,omitempty"`
	Branches []string                   `json:"branches,omitempty"`
	Reason   string                     `json:"reason"`
}

type BusinessCoverageProof struct {
	Status              string                  `json:"status"`
	Scope               string                  `json:"scope"`
	ContractSHA256      string                  `json:"contract_sha256,omitempty"`
	ProofKind           string                  `json:"proof_kind"`
	Budget              int                     `json:"budget"`
	RequiredCases       uint64                  `json:"required_cases"`
	CheckedCases        int                     `json:"checked_cases"`
	CompleteInScope     bool                    `json:"complete_in_scope"`
	EnumerationComplete bool                    `json:"enumeration_complete"`
	Domains             []BusinessFiniteDomain  `json:"domains,omitempty"`
	Scopes              []BusinessCoverageScope `json:"scopes,omitempty"`
	Issues              []BusinessCoverageIssue `json:"issues"`
	IssueCounts         map[string]int          `json:"issue_counts,omitempty"`
	IssuesTruncated     bool                    `json:"issues_truncated,omitempty"`
	Reason              string                  `json:"reason"`
}

type PreparationAction struct {
	ID        string              `json:"id"`
	Action    string              `json:"action"`
	Requires  BusinessPredicate   `json:"requires"`
	Ensures   BusinessPredicate   `json:"ensures"`
	DependsOn []string            `json:"depends_on,omitempty"`
	Effects   []PreparationEffect `json:"effects"`
	Source    string              `json:"source"`
	SourceRef string              `json:"source_ref"`
}

type PreparationEffect struct {
	Subject   string          `json:"subject"`
	Attribute string          `json:"attribute"`
	Value     json.RawMessage `json:"value"`
}

type PreparationRequest struct {
	Observed  BusinessObservation `json:"observed"`
	Goal      BusinessPredicate   `json:"goal"`
	MaxSteps  int                 `json:"max_steps"`
	MaxStates int                 `json:"max_states"`
}

type PlannedPreparationStep struct {
	ID        string              `json:"id"`
	Action    string              `json:"action"`
	Requires  BusinessPredicate   `json:"requires"`
	Ensures   BusinessPredicate   `json:"ensures"`
	Effects   []PreparationEffect `json:"predicted_effects"`
	SourceRef string              `json:"source_ref"`
}

type PreparationLayer struct {
	Index int                      `json:"index"`
	Steps []PlannedPreparationStep `json:"steps"`
}

type PreparationPlan struct {
	Status           string             `json:"status"`
	Reason           string             `json:"reason"`
	ObservedSHA256   string             `json:"observed_sha256"`
	ObservedReadback int                `json:"observed_readback,omitempty"`
	Layers           []PreparationLayer `json:"layers"`
	AlreadySatisfied []string           `json:"already_satisfied"`
	Blocked          []string           `json:"blocked"`
	Gaps             []string           `json:"gaps"`
	ExploredStates   int                `json:"explored_states"`
	MaxStates        int                `json:"max_states"`
	MaxSteps         int                `json:"max_steps"`
	RequiresReadback bool               `json:"requires_readback"`
}
