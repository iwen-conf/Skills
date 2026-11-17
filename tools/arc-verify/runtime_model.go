package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

// Version 2 binds observations to an action within a batch. Version 0 remains
// readable as historical evidence, but is never relabeled as identity-complete.
const runtimeEvidenceVersion = 2

type ParticipantBinding struct {
	Alias     string `json:"alias"`
	Role      string `json:"role"`
	Account   string `json:"account"`
	Principal string `json:"principal"`
	SHA256    string `json:"sha256"`
}

type VerificationRound struct {
	ID             string       `json:"id"`
	Context        ProofContext `json:"context"`
	FixtureSHA256  string       `json:"fixture_sha256"`
	RunnersSHA256  string       `json:"runners_sha256"`
	RequiredSHA256 string       `json:"required_sha256"`
	Created        string       `json:"created"`
}

type IndependentFacts struct {
	State      string                     `json:"state"`
	Principal  string                     `json:"principal"`
	Role       string                     `json:"role"`
	Scope      string                     `json:"scope"`
	Conditions ObservedConditions         `json:"conditions"`
	Values     map[string]json.RawMessage `json:"values"`
}

// ObservedConditions are an oracle's readings. null means the oracle was not
// read (an action that was never attempted has no "after"); a present object
// still needs an explicit boolean for every condition.
type ObservedConditions map[string]bool

func (conditions *ObservedConditions) UnmarshalJSON(raw []byte) error {
	if string(bytes.TrimSpace(raw)) == "null" {
		*conditions = nil
		return nil
	}
	var strict ConditionAssignment
	if err := strict.UnmarshalJSON(raw); err != nil {
		return err
	}
	*conditions = ObservedConditions(strict)
	return nil
}

type OracleObservation struct {
	ActionIndex  int              `json:"action_index,omitempty"`
	Actor        string           `json:"actor"`
	ID           string           `json:"id"`
	Boundary     string           `json:"boundary"`
	Before       IndependentFacts `json:"before"`
	After        IndependentFacts `json:"after"`
	BeforeSHA256 string           `json:"before_sha256"`
	AfterSHA256  string           `json:"after_sha256"`
}

type ExtractedBinding struct {
	Name             string `json:"name"`
	Source           string `json:"source"`
	Path             string `json:"path"`
	Value            string `json:"value,omitempty"`
	SHA256           string `json:"sha256"`
	Redacted         bool   `json:"redacted,omitempty"`
	EntityKind       string `json:"entity_kind,omitempty"`
	EntityAlias      string `json:"entity_alias,omitempty"`
	OwnerActor       string `json:"owner_actor,omitempty"`
	OwnerPrincipal   string `json:"owner_principal,omitempty"`
	ChannelAccountID string `json:"channel_account_id,omitempty"`
	Step             int    `json:"step"`
	ActionIndex      int    `json:"action_index,omitempty"`
	SourceActor      string `json:"source_actor,omitempty"`
	OracleID         string `json:"oracle_id,omitempty"`
}

type EntityBinding struct {
	Alias            string            `json:"alias"`
	Kind             string            `json:"kind"`
	ID               string            `json:"id"`
	OwnerActor       string            `json:"owner_actor"`
	OwnerPrincipal   string            `json:"owner_principal"`
	ChannelAccountID string            `json:"channel_account_id,omitempty"`
	SourceStep       int               `json:"source_step"`
	SourceField      string            `json:"source_field"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type SessionBinding struct {
	Scope        string `json:"scope"`
	Actor        string `json:"actor"`
	Principal    string `json:"principal"`
	AcquiredStep int    `json:"acquired_step"`
	Digest       string `json:"digest"`
	Active       bool   `json:"active"`
}

type WaitObservation struct {
	Actor                string   `json:"actor,omitempty"`
	Step                 int      `json:"step,omitempty"`
	ActionIndex          int      `json:"action_index,omitempty"`
	Node                 string   `json:"node,omitempty"`
	Transition           string   `json:"transition,omitempty"`
	OracleID             string   `json:"oracle_id"`
	RuleSourceRef        string   `json:"rule_source_ref"`
	Started              string   `json:"started"`
	Completed            string   `json:"completed"`
	Attempts             int      `json:"attempts"`
	MaxWaitMS            int      `json:"max_wait_ms"`
	PollIntervalMS       int      `json:"poll_interval_ms"`
	StableWindowMS       int      `json:"stable_window_ms,omitempty"`
	StableSince          string   `json:"stable_since,omitempty"`
	TimedOut             bool     `json:"timed_out"`
	ViolatedIntermediate bool     `json:"violated_intermediate,omitempty"`
	FinalState           string   `json:"final_state"`
	ObservedStates       []string `json:"observed_states"`
}

type ConditionReading struct {
	Condition      string `json:"condition"`
	ObserverActor  string `json:"observer_actor,omitempty"`
	SubjectKind    string `json:"subject_kind,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Attribute      string `json:"attribute,omitempty"`
	Operator       string `json:"operator,omitempty"`
	Expected       string `json:"expected,omitempty"`
	AnnotationOnly bool   `json:"annotation_only,omitempty"`
	ExpectedBool   bool   `json:"expected_bool"`
	ActualBool     *bool  `json:"actual_bool,omitempty"`
	OracleID       string `json:"oracle_id,omitempty"`
	Path           string `json:"path,omitempty"`
	Passed         bool   `json:"passed"`
}

type StepAssertion struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	OracleID string `json:"oracle_id,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Operator string `json:"operator,omitempty"`
	Verdict  string `json:"verdict"`
	Reason   string `json:"reason"`
}

type UnifiedStepFact struct {
	RecoveryMode         string                           `json:"recovery_mode,omitempty"`
	EvidenceVersion      int                              `json:"evidence_version,omitempty"`
	EvidenceStatus       string                           `json:"evidence_status,omitempty"`
	EvidenceSource       string                           `json:"evidence_source,omitempty"`
	SourceKind           string                           `json:"source_kind"`
	Run                  string                           `json:"run"`
	Seq                  int                              `json:"seq"`
	Graph                string                           `json:"graph"`
	Task                 string                           `json:"task,omitempty"`
	Node                 string                           `json:"node,omitempty"`
	Transition           string                           `json:"transition,omitempty"`
	RuleID               string                           `json:"rule_id,omitempty"`
	Actor                string                           `json:"actor,omitempty"`
	Account              string                           `json:"account,omitempty"`
	Principal            string                           `json:"principal,omitempty"`
	ExpectedState        string                           `json:"expected_state,omitempty"`
	ObservedState        string                           `json:"observed_state,omitempty"`
	DeniedExpected       *bool                            `json:"denied_expected,omitempty"`
	DeniedObserved       *bool                            `json:"denied_observed,omitempty"`
	Verdict              string                           `json:"verdict"`
	Decision             string                           `json:"decision,omitempty"`
	Reason               string                           `json:"reason,omitempty"`
	Participants         []ParticipantBinding             `json:"participants,omitempty"`
	BeforeEntities       []EntityBinding                  `json:"before_entities,omitempty"`
	BeforeSessions       []SessionBinding                 `json:"before_sessions,omitempty"`
	Entities             []EntityBinding                  `json:"entities,omitempty"`
	Sessions             []SessionBinding                 `json:"sessions,omitempty"`
	Extractions          []ExtractedBinding               `json:"extractions,omitempty"`
	Conditions           []ConditionReading               `json:"conditions,omitempty"`
	Waits                []WaitObservation                `json:"waits,omitempty"`
	Oracles              []OracleObservation              `json:"oracles,omitempty"`
	Actions              []ActionObservation              `json:"actions,omitempty"`
	Requests             []RequestObservation             `json:"requests,omitempty"`
	Assertions           []StepAssertion                  `json:"assertions,omitempty"`
	BusinessExpectations []BusinessExpectationObservation `json:"business_expectations,omitempty"`
	Preparation          *PreparationExecution            `json:"preparation,omitempty"`
	EvidencePath         string                           `json:"evidence_path,omitempty"`
	EvidenceSHA256       string                           `json:"evidence_sha256,omitempty"`
}

type ActionObservation struct {
	ActionIndex    int                        `json:"action_index,omitempty"`
	Node           string                     `json:"node"`
	Transition     string                     `json:"transition,omitempty"`
	Actor          string                     `json:"actor"`
	Principal      string                     `json:"principal"`
	TemplateSHA256 string                     `json:"template_sha256,omitempty"`
	ResolvedPath   string                     `json:"resolved_path,omitempty"`
	ResolvedInput  json.RawMessage            `json:"resolved_input,omitempty"`
	Substitutions  map[string]string          `json:"substitutions,omitempty"`
	RequestSHA256  string                     `json:"request_sha256"`
	ResponseSHA256 string                     `json:"response_sha256"`
	ResponseValues map[string]json.RawMessage `json:"response_values,omitempty"`
	StatusCode     int                        `json:"status_code,omitempty"`
	ExitCode       int                        `json:"exit_code"`
	Denied         bool                       `json:"denied"`
	Started        string                     `json:"started"`
	Completed      string                     `json:"completed"`
}

type ModelObservation struct {
	RecoveryMode         string                           `json:"recovery_mode,omitempty"`
	EvidenceVersion      int                              `json:"evidence_version,omitempty"`
	Node                 string                           `json:"node"`
	Transition           string                           `json:"transition,omitempty"`
	Round                string                           `json:"round"`
	RunnerSHA256         string                           `json:"runner_sha256"`
	CodeSHA256           string                           `json:"code_sha256"`
	Input                json.RawMessage                  `json:"input"`
	Participants         []ParticipantBinding             `json:"participants"`
	BeforeEntities       []EntityBinding                  `json:"before_entities,omitempty"`
	BeforeSessions       []SessionBinding                 `json:"before_sessions,omitempty"`
	Oracles              []OracleObservation              `json:"oracles"`
	Actions              []ActionObservation              `json:"actions"`
	Requests             []RequestObservation             `json:"requests,omitempty"`
	Extractions          []ExtractedBinding               `json:"extractions,omitempty"`
	Entities             []EntityBinding                  `json:"entities,omitempty"`
	Sessions             []SessionBinding                 `json:"sessions,omitempty"`
	Waits                []WaitObservation                `json:"waits,omitempty"`
	Conditions           []ConditionReading               `json:"conditions,omitempty"`
	Assertions           []StepAssertion                  `json:"assertions,omitempty"`
	BusinessExpectations []BusinessExpectationObservation `json:"business_expectations,omitempty"`
	Preparation          *PreparationExecution            `json:"preparation,omitempty"`
	Error                string                           `json:"error,omitempty"`
}

// A decision is frozen from independent pre-action readings. The after state
// and comparison are recorded by the runner, then rechecked by the verifier.
type BusinessExpectationObservation struct {
	Request     *RequestObservation  `json:"request,omitempty"`
	Actor       string               `json:"actor"`
	ActionIndex int                  `json:"action_index"`
	Decision    BusinessDecision     `json:"decision"`
	After       *BusinessObservation `json:"after,omitempty"`
	Comparison  *BusinessComparison  `json:"comparison,omitempty"`
}

type RecoveryOutcome struct {
	Mode     string              `json:"mode,omitempty"`
	Verdict  string              `json:"verdict"`
	Reason   string              `json:"reason"`
	State    string              `json:"state,omitempty"`
	Safe     bool                `json:"safe"`
	Evidence string              `json:"raw,omitempty"`
	SHA256   string              `json:"sha256,omitempty"`
	Actions  []ActionObservation `json:"actions"`
}

type ModelRunRecord struct {
	Run           string               `json:"run"`
	Round         string               `json:"round"`
	Runner        string               `json:"runner"`
	RunnerSHA256  string               `json:"runner_sha256"`
	CodeSHA256    string               `json:"code_sha256"`
	Participants  []ParticipantBinding `json:"participants"`
	SetupEvidence string               `json:"setup_raw,omitempty"`
	SetupSHA256   string               `json:"setup_sha256,omitempty"`
	Recovery      RecoveryOutcome      `json:"recovery"`
	Completed     bool                 `json:"completed"`
}

type PropertyOutcome struct {
	Index   int             `json:"index"`
	Left    json.RawMessage `json:"left,omitempty"`
	Right   json.RawMessage `json:"right,omitempty"`
	Verdict string          `json:"verdict"`
	Reason  string          `json:"reason"`
}

// Exact rational comparison avoids accepting counters rounded by float64.
func compareProperty(left, right json.RawMessage, operator ComparisonOperator) (bool, error) {
	decode := func(raw json.RawMessage) (any, error) {
		if !json.Valid(raw) {
			return nil, errors.New("missing or invalid property measurement")
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	}
	l, err := decode(left)
	if err != nil {
		return false, err
	}
	r, err := decode(right)
	if err != nil {
		return false, err
	}
	ln, lNumeric := l.(json.Number)
	rn, rNumeric := r.(json.Number)
	if lNumeric && rNumeric {
		if len(ln.String()) > 1024 || len(rn.String()) > 1024 {
			return false, errors.New("property number exceeds 1024 characters")
		}
		lv, lok := new(big.Rat).SetString(ln.String())
		rv, rok := new(big.Rat).SetString(rn.String())
		if !lok || !rok {
			return false, errors.New("invalid finite property number")
		}
		cmp := lv.Cmp(rv)
		switch operator {
		case compareEqual:
			return cmp == 0, nil
		case compareNotEqual:
			return cmp != 0, nil
		case compareLess:
			return cmp < 0, nil
		case compareLessEqual:
			return cmp <= 0, nil
		case compareGreater:
			return cmp > 0, nil
		case compareGreaterEqual:
			return cmp >= 0, nil
		default:
			return false, errors.New("unknown property comparison")
		}
	}
	if reflect.TypeOf(l) != reflect.TypeOf(r) {
		return false, errors.New("property measurement types disagree")
	}
	switch operator {
	case compareEqual:
		return reflect.DeepEqual(l, r), nil
	case compareNotEqual:
		return !reflect.DeepEqual(l, r), nil
	case compareLess, compareLessEqual, compareGreater, compareGreaterEqual:
		return false, errors.New("ordered property comparisons require two numbers")
	default:
		return false, errors.New("unknown property comparison")
	}
}

func evaluateProperties(checks []PropertyCheck, values map[string]json.RawMessage) []PropertyOutcome {
	result := make([]PropertyOutcome, 0, len(checks))
	for index, check := range checks {
		right := check.Value
		if check.Right != "" {
			right = values[check.Right]
		}
		outcome := PropertyOutcome{Index: index + 1, Left: values[check.Left], Right: right, Verdict: "PASS", Reason: "independent measurement satisfies the declared property"}
		matches, err := compareProperty(outcome.Left, outcome.Right, check.Operator)
		if err != nil {
			outcome.Verdict, outcome.Reason = "BLOCKED", err.Error()
		} else if !matches {
			outcome.Verdict, outcome.Reason = "FAIL", fmt.Sprintf("property %s %s did not hold", check.Left, check.Operator)
		}
		result = append(result, outcome)
	}
	return result
}
