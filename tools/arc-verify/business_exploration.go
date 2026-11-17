package main

// BusinessCampaignRequest freezes an explicitly selected, bounded set of
// registered input variants. A variant is an obligation on a declared branch,
// never an inferred route or a request synthesized by the explorer.
type BusinessCampaignRequest struct {
	Campaign    string                    `json:"campaign,omitempty"`
	Graph       string                    `json:"graph"`
	Flow        string                    `json:"flow"`
	Environment string                    `json:"environment"`
	Round       string                    `json:"round"`
	Commit      string                    `json:"commit"`
	Root        string                    `json:"root"`
	Config      string                    `json:"config"`
	Account     string                    `json:"account"`
	Fixture     string                    `json:"fixture"`
	Accounts    map[string]string         `json:"accounts,omitempty"`
	Variants    []BusinessCampaignVariant `json:"variants"`
	Budget      BusinessCampaignBudget    `json:"budget"`
}

type BusinessCampaignBudget struct {
	MaxActions int   `json:"max_actions"`
	MaxStates  int   `json:"max_states"`
	DeadlineMS int64 `json:"deadline_ms"`
}

type BusinessCampaignSpent struct {
	Actions            int   `json:"actions"`
	ReservedActions    int   `json:"reserved_actions"`
	Dispatches         int   `json:"dispatches"`
	TargetActions      int   `json:"target_actions"`
	PreparationActions int   `json:"preparation_actions"`
	RecoveryActions    int   `json:"recovery_actions"`
	States             int   `json:"states"`
	Observations       int   `json:"observations"`
	ElapsedMS          int64 `json:"elapsed_ms"`
}

type BusinessCampaignVariant struct {
	ID           string `json:"id"`
	Branch       string `json:"branch"`
	Runner       string `json:"runner"`
	Task         string `json:"task,omitempty"`
	Action       string `json:"action,omitempty"`
	Actor        string `json:"actor,omitempty"`
	RunnerSHA256 string `json:"runner_sha256,omitempty"`
	CodeSHA256   string `json:"code_sha256,omitempty"`
	Visits       int    `json:"visits"`
	Passes       int    `json:"passes"`
	LastRun      string `json:"last_run,omitempty"`
}

type BusinessCampaignCandidate struct {
	VariantID              string               `json:"variant_id"`
	Branch                 string               `json:"branch"`
	Runner                 string               `json:"runner"`
	Run                    string               `json:"run"`
	Eligibility            string               `json:"eligibility"`
	Reason                 string               `json:"reason"`
	StateKey               string               `json:"state_key,omitempty"`
	Before                 *BusinessObservation `json:"before,omitempty"`
	Observation            *ModelObservation    `json:"observation,omitempty"`
	Input                  *RequestObservation  `json:"input,omitempty"`
	Decision               *BusinessDecision    `json:"decision,omitempty"`
	PreparationBefore      *BusinessObservation `json:"preparation_before,omitempty"`
	PreparationObservation *ModelObservation    `json:"preparation_observation,omitempty"`
	Preparation            *PreparationPlan     `json:"preparation,omitempty"`
	NextAllowed            bool                 `json:"next_allowed"`
	Selected               bool                 `json:"selected"`
}

type BusinessCampaignVisit struct {
	Sequence       int                   `json:"sequence"`
	VariantID      string                `json:"variant_id"`
	Branch         string                `json:"branch"`
	Runner         string                `json:"runner"`
	Run            string                `json:"run"`
	Status         string                `json:"status"`
	Reason         string                `json:"reason"`
	BeforeStateKey string                `json:"before_state_key,omitempty"`
	AfterStateKey  string                `json:"after_state_key,omitempty"`
	DispatchSHA256 string                `json:"dispatch_sha256"`
	Before         *BusinessObservation  `json:"before,omitempty"`
	After          *BusinessObservation  `json:"after,omitempty"`
	Comparison     *BusinessComparison   `json:"comparison,omitempty"`
	Actor          string                `json:"actor,omitempty"`
	Principal      string                `json:"principal,omitempty"`
	Entities       []EntityBinding       `json:"entities,omitempty"`
	Preparation    *PreparationExecution `json:"preparation,omitempty"`
	Recovery       *RecoveryOutcome      `json:"recovery,omitempty"`
}

type BusinessCampaignCoverage struct {
	DeclaredBranches        int      `json:"declared_branches"`
	BoundBranches           int      `json:"bound_branches"`
	SelectedVariants        int      `json:"selected_variants"`
	AttemptedVariants       int      `json:"attempted_variants"`
	PassedVariants          int      `json:"passed_variants"`
	UnmappedBranches        []string `json:"unmapped_branches"`
	SelectedVariantsCovered bool     `json:"selected_variants_covered"`
	FullBusinessCoverage    bool     `json:"full_business_coverage"`
}

type BusinessCampaignDispatchReference struct {
	Sequence int    `json:"sequence"`
	Run      string `json:"run"`
	SHA256   string `json:"sha256"`
}

// The snapshot is a checked projection of a campaign ledger. Status is an
// execution stop reason, not a claim of complete business correctness.
type BusinessCampaign struct {
	Version         int                                 `json:"version"`
	ID              string                              `json:"id"`
	Graph           string                              `json:"graph"`
	Flow            string                              `json:"flow"`
	Environment     string                              `json:"environment"`
	Round           string                              `json:"round"`
	Status          string                              `json:"status"`
	Reason          string                              `json:"reason"`
	EvidenceStatus  string                              `json:"evidence_status"`
	Identity        ExecutionIdentity                   `json:"identity"`
	Request         BusinessCampaignRequest             `json:"request"`
	Variants        []BusinessCampaignVariant           `json:"variants"`
	Frontier        []BusinessCampaignCandidate         `json:"frontier"`
	Visits          []BusinessCampaignVisit             `json:"visits"`
	Dispatches      []BusinessCampaignDispatchReference `json:"dispatches"`
	Budget          BusinessCampaignBudget              `json:"budget"`
	Spent           BusinessCampaignSpent               `json:"spent"`
	Coverage        BusinessCampaignCoverage            `json:"coverage"`
	LastActual      *BusinessObservation                `json:"last_actual,omitempty"`
	LastRun         string                              `json:"last_run,omitempty"`
	NextAllowed     BusinessSuccessorPolicy             `json:"next_allowed,omitzero"`
	FirstDivergence string                              `json:"first_divergence,omitempty"`
	HistorySHA256   string                              `json:"history_sha256"`
	Revision        int                                 `json:"revision"`
	Created         string                              `json:"created"`
	Updated         string                              `json:"updated"`
}
