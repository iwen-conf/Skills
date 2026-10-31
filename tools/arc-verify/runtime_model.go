package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

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
	Actor        string           `json:"actor"`
	ID           string           `json:"id"`
	Boundary     string           `json:"boundary"`
	Before       IndependentFacts `json:"before"`
	After        IndependentFacts `json:"after"`
	BeforeSHA256 string           `json:"before_sha256"`
	AfterSHA256  string           `json:"after_sha256"`
}

type ActionObservation struct {
	Node           string `json:"node"`
	Transition     string `json:"transition,omitempty"`
	Actor          string `json:"actor"`
	Principal      string `json:"principal"`
	RequestSHA256  string `json:"request_sha256"`
	ResponseSHA256 string `json:"response_sha256"`
	StatusCode     int    `json:"status_code,omitempty"`
	ExitCode       int    `json:"exit_code"`
	Denied         bool   `json:"denied"`
	Started        string `json:"started"`
	Completed      string `json:"completed"`
}

type ModelObservation struct {
	Node         string               `json:"node"`
	Transition   string               `json:"transition,omitempty"`
	Round        string               `json:"round"`
	RunnerSHA256 string               `json:"runner_sha256"`
	CodeSHA256   string               `json:"code_sha256"`
	Input        json.RawMessage      `json:"input"`
	Participants []ParticipantBinding `json:"participants"`
	Oracles      []OracleObservation  `json:"oracles"`
	Actions      []ActionObservation  `json:"actions"`
	Error        string               `json:"error,omitempty"`
}

type RecoveryOutcome struct {
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
