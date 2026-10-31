package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StepRAW is an independent runner's observation, never the expected result
// derived from the implementation being tested.
type StepRAW struct {
	Identity      ExecutionIdentity `json:"identity"`
	Phase         string            `json:"phase,omitempty"`
	Run           string            `json:"run"`
	Step          int               `json:"step"`
	Expected      string            `json:"expected"`
	Observed      string            `json:"observed"`
	Preconditions map[string]bool   `json:"preconditions"`
	Before        string            `json:"before,omitempty"`
	After         string            `json:"after,omitempty"`
	// Denied is always written: "observed not denied" must stay distinct from
	// "denial not observed", or an accepted forbidden action cannot be a FAIL.
	Denied         bool              `json:"denied"`
	SafeToContinue bool              `json:"safe_to_continue,omitempty"`
	Oracle         string            `json:"oracle"`
	Runner         string            `json:"runner"`
	ExitCode       int               `json:"exit_code"`
	Model          *ModelObservation `json:"model,omitempty"`
	denialObserved bool
}

func hasSensitiveKey(v any) bool {
	switch data := v.(type) {
	case map[string]any:
		for key, child := range data {
			k := strings.ToLower(key)
			for _, part := range []string{"password", "secret", "token", "authorization", "cookie", "api_key", "apikey"} {
				if strings.Contains(k, part) {
					return true
				}
			}
			if hasSensitiveKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range data {
			if hasSensitiveKey(child) {
				return true
			}
		}
	}
	return false
}

func parseStepRAW(raw []byte, run, expected, observed string, index int) (StepRAW, error) {
	var value any
	if err := decodeUniqueJSON(raw, &value); err != nil {
		return StepRAW{}, fmt.Errorf("invalid runner RAW: %w", err)
	}
	if hasSensitiveKey(value) {
		return StepRAW{}, errors.New("RAW contains a credential-like field; redact it before recording")
	}
	fields, ok := value.(map[string]any)
	if !ok || fields["exit_code"] == nil {
		return StepRAW{}, errors.New("RAW requires an explicit integer exit_code")
	}
	if conditions, exists := fields["preconditions"]; exists {
		observed, ok := conditions.(map[string]any)
		if !ok {
			return StepRAW{}, errors.New("RAW preconditions must be an object of observed booleans")
		}
		for name, actual := range observed {
			if _, ok := actual.(bool); !ok {
				return StepRAW{}, fmt.Errorf("RAW precondition %s must be an observed boolean", name)
			}
		}
	}
	for _, name := range []string{"denied", "safe_to_continue"} {
		if actual, exists := fields[name]; exists {
			if _, ok := actual.(bool); !ok {
				return StepRAW{}, fmt.Errorf("RAW %s must be boolean", name)
			}
		}
	}
	var v StepRAW
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v); err != nil {
		return StepRAW{}, err
	}
	if index < 0 && (index != -1 || v.Phase != "setup") || index == 0 && v.Phase != "fixture" || index > 0 && v.Phase != "" && v.Phase != "action" {
		return StepRAW{}, errors.New("RAW phase does not match the action or fixture checkpoint")
	}
	_, v.denialObserved = fields["denied"].(bool)
	if v.Run != run || v.Step != index || v.Expected != expected || v.Observed != observed || strings.TrimSpace(v.Oracle) == "" || strings.TrimSpace(v.Runner) == "" {
		return StepRAW{}, errors.New("RAW run, step, oracle, expected, observed or runner identity mismatch")
	}
	if expected == "UNCHANGED" {
		if !rejectionEvidencePresent(v) {
			return StepRAW{}, errors.New("UNCHANGED requires before/after readback")
		}
	}
	return v, nil
}

// Validate actual preconditions independently before evaluating the action's
// result. A rejection for the wrong reason is never a PASS.
func conditionsMatch(v StepRAW, kind, failed string, required []string) bool {
	if kind != "POSITIVE" && kind != "NEGATIVE" && kind != "FORCED" {
		return false
	}
	if kind == "POSITIVE" && failed != "" {
		return false
	}
	foundFailed := kind == "POSITIVE"
	if len(v.Preconditions) < len(required) {
		return false
	}
	for _, condition := range required {
		if condition == failed {
			foundFailed = true
		}
		actual, found := v.Preconditions[condition]
		if !found {
			return false
		}
		shouldBeSatisfied := kind == "POSITIVE" || condition != failed
		if actual != shouldBeSatisfied {
			return false
		}
	}
	return foundFailed
}

func rejectionMatches(v StepRAW, kind string) bool {
	if kind == "POSITIVE" && v.Expected != "UNCHANGED" {
		return true
	}
	return rejectionEvidencePresent(v) && v.Denied && v.Before == v.After
}

func rejectionEvidencePresent(v StepRAW) bool {
	return v.denialObserved && strings.TrimSpace(v.Before) != "" && strings.TrimSpace(v.After) != ""
}

func loadEvidence(path string) (string, []byte, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return "", nil, e
	}
	stat, e := os.Lstat(abs)
	if e != nil {
		return "", nil, e
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 || stat.Size() > 4<<20 {
		return "", nil, errors.New("RAW evidence must be a nonempty regular file <=4 MiB")
	}
	b, e := os.ReadFile(abs)
	return abs, b, e
}

func (d db) step(run, expected, observed, evidence string) (map[string]any, error) {
	contract, e := d.runTaskContract(run)
	if e != nil {
		return nil, e
	}
	if contract.Status != "RUNNING" {
		return nil, errors.New("run not RUNNING")
	}
	if e = d.validateBinding(contract.Binding, nil); e != nil {
		return nil, e
	}
	finalExpected, graph := contract.Task.Expected, contract.Graph
	if expected == "" {
		expected = finalExpected
	}
	if expected != "UNCHANGED" {
		known, e := d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(graph)+" AND id="+quote(expected)+" AND kind='state';", 1)
		if e != nil {
			return nil, e
		}
		if len(known) == 0 {
			return nil, fmt.Errorf("expected state %q is not in the frozen business graph", expected)
		}
	}
	n, e := d.query("SELECT "+fieldsSQL("CAST(COALESCE(MAX(idx),0)+1 AS TEXT)")+" FROM steps WHERE run="+quote(run)+";", 1)
	if e != nil {
		return nil, e
	}
	index := 1
	if len(n) == 1 {
		if _, e = fmt.Sscan(n[0][0], &index); e != nil {
			return nil, e
		}
	}
	path, raw, e := loadEvidence(evidence)
	if e != nil {
		return nil, e
	}
	observation, e := parseStepRAW(raw, run, expected, observed, index)
	if e != nil {
		return nil, e
	}
	if e = contract.Binding.matchesRAW(observation); e != nil {
		return nil, e
	}
	if expected == finalExpected && contract.Model == nil && contract.Task.Kind != "POSITIVE" && !rejectionEvidencePresent(observation) {
		return nil, errors.New("rejected action requires denial observation and before/after readback")
	}
	h := sha256.Sum256(raw)
	sha := hex.EncodeToString(h[:])
	verdict, decision := "PASS", "CONTINUE"
	if contract.Model != nil {
		verdict, e = d.modelObservationVerdict(contract, observation, index, sha)
		if e != nil {
			return nil, e
		}
		if verdict == "BLOCKED" {
			decision = "RESET"
		}
		if verdict == "FAIL" {
			decision = "HALT"
		}
	}
	if observation.ExitCode != 0 {
		verdict, decision = "BLOCKED", "HALT"
	}
	if verdict == "PASS" && contract.Model == nil && expected == finalExpected {
		if !conditionsMatch(observation, contract.Task.Kind, contract.Task.Condition, contract.Rule.Conditions) {
			verdict, decision = "BLOCKED", "RESET"
		}
	}
	rejectedMismatch := contract.Model == nil && (expected == finalExpected || expected == "UNCHANGED") && !rejectionMatches(observation, contract.Task.Kind)
	if verdict == "PASS" && (expected != observed || rejectedMismatch) {
		verdict, decision = "FAIL", "HALT"
		// Replan only when a known state was independently observed and the
		// observer explicitly certifies the state is safe for a new branch.
		if observation.SafeToContinue && observation.ExitCode == 0 {
			known, e := d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(graph)+" AND id="+quote(observed)+" AND kind='state';", 1)
			if e != nil {
				return nil, e
			}
			if len(known) > 0 {
				decision = "REPLAN"
			}
		}
	}
	stmts := []string{
		fmt.Sprintf("INSERT INTO steps(run,idx,expected,observed,verdict,decision,evidence_path,evidence_sha256) SELECT %s,%d,%s,%s,%s,%s,%s,%s FROM runs WHERE id=%s AND status='RUNNING' AND EXISTS (SELECT 1 FROM tasks WHERE tasks.id=runs.task AND tasks.status='RUNNING');",
			quote(run), index, quote(expected), quote(observed), quote(verdict), quote(decision), quote(path), quote(sha), quote(run)),
	}
	if verdict != "PASS" {
		stmts = append(stmts, fmt.Sprintf("UPDATE runs SET status=%s WHERE id=%s AND status='RUNNING';", quote(verdict), quote(run)))
		stmts = append(stmts, fmt.Sprintf("UPDATE tasks SET status=%s WHERE id=(SELECT task FROM runs WHERE id=%s) AND status='RUNNING';", quote(verdict), quote(run)))
	}
	if e = d.transition(stmts); e != nil {
		return nil, e
	}
	check, e := d.query("SELECT "+fieldsSQL("verdict", "evidence_sha256")+" FROM steps WHERE run="+quote(run)+" AND idx="+fmt.Sprint(index)+";", 2)
	if e != nil {
		return nil, e
	}
	if len(check) != 1 || check[0][1] != sha {
		return nil, errors.New("step not durably recorded")
	}
	return map[string]any{"run": run, "step": index, "expected": expected, "observed": observed, "verdict": verdict, "decision": decision, "sha256": sha}, nil
}

func (d db) finish(run string) (map[string]any, error) {
	contract, e := d.runTaskContract(run)
	if e != nil {
		return nil, e
	}
	if contract.Status != "RUNNING" {
		return nil, errors.New("run not RUNNING")
	}
	if e = d.validateBinding(contract.Binding, nil); e != nil {
		return nil, e
	}
	if contract.Model != nil {
		if !contract.Receipt.Completed || contract.Receipt.Recovery.Verdict != "PASS" || !contract.Receipt.Recovery.Safe {
			return nil, errors.New("model run requires completed registered execution and independently verified fixture restoration")
		}
		if e = d.validateModelCompletion(contract); e != nil {
			return nil, e
		}
	}
	steps, e := d.query("SELECT "+fieldsSQL("expected", "observed", "verdict", "evidence_path", "evidence_sha256", "CAST(idx AS TEXT)")+" FROM steps WHERE run="+quote(run)+" ORDER BY idx;", 6)
	if e != nil {
		return nil, e
	}
	if len(steps) == 0 {
		return nil, errors.New("cannot PASS an unexecuted run")
	}
	for index, s := range steps {
		if s[5] != fmt.Sprint(index+1) || s[2] != "PASS" || s[0] != s[1] {
			return nil, errors.New("a step failed or its sequence changed")
		}
		observation, e := readRecordedRAW(s[3], s[4], run, s[0], s[1], index+1)
		if e != nil {
			return nil, e
		}
		if e = contract.Binding.matchesRAW(observation); e != nil {
			return nil, e
		}
		if observation.ExitCode != 0 {
			return nil, errors.New("runner did not complete successfully")
		}
		if contract.Model != nil {
			verdict, err := d.modelObservationVerdict(contract, observation, index+1, s[4])
			if err != nil {
				return nil, err
			}
			if verdict != "PASS" {
				return nil, errors.New("model observation no longer proves its declared contract")
			}
		} else if s[0] == contract.Task.Expected {
			if !conditionsMatch(observation, contract.Task.Kind, contract.Task.Condition, contract.Rule.Conditions) || !rejectionMatches(observation, contract.Task.Kind) {
				return nil, errors.New("RAW does not satisfy the frozen action contract")
			}
		} else if s[0] == "UNCHANGED" && !rejectionMatches(observation, "NEGATIVE") {
			return nil, errors.New("rejected step changed business state")
		}
	}
	last := steps[len(steps)-1]
	if last[0] != contract.Task.Expected || last[1] != contract.Task.Expected {
		return nil, errors.New("final observed step does not match frozen task expectation")
	}
	statements := []string{
		fmt.Sprintf("UPDATE runs SET status='PASS' WHERE id=%s AND status='RUNNING' AND (SELECT COUNT(*) FROM steps WHERE run=%s)=%d AND NOT EXISTS (SELECT 1 FROM steps WHERE run=%s AND verdict<>'PASS');", quote(run), quote(run), len(steps), quote(run)),
		fmt.Sprintf("UPDATE tasks SET status='PASS' WHERE id=(SELECT task FROM runs WHERE id=%s) AND status='RUNNING';", quote(run)),
	}
	fixtures, e := d.runFixtureIDs(run, contract.Binding)
	if e != nil {
		return nil, e
	}
	for _, fixture := range fixtures {
		statements = append(statements, fmt.Sprintf("DELETE FROM fixture_locks WHERE fixture=%s AND run=%s;", quote(fixture), quote(run)))
	}
	if e = d.transition(statements); e != nil {
		return nil, e
	}
	return map[string]any{"run": run, "verdict": "PASS", "steps": len(steps)}, nil
}

func readRecordedRAW(path, hash, run, expected, observed string, index int) (StepRAW, error) {
	_, raw, e := loadEvidence(path)
	if e != nil {
		return StepRAW{}, e
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != hash {
		return StepRAW{}, errors.New("RAW changed after being recorded")
	}
	return parseStepRAW(raw, run, expected, observed, index)
}
