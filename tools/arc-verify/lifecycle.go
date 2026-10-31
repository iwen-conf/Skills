package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func validReason(reason string) error {
	if strings.TrimSpace(reason) == "" || len(reason) > 512 {
		return errors.New("a nonempty reason <=512 bytes is required")
	}
	return nil
}

// abort closes only a RUNNING attempt and preserves all previously captured
// steps and raw evidence. It never turns a test into PASS.
func (d db) abort(run, reason string) (map[string]string, error) {
	if e := validReason(reason); e != nil {
		return nil, e
	}
	rows, e := d.query("SELECT "+fieldsSQL("task", "status")+" FROM runs WHERE id="+quote(run)+";", 2)
	if e != nil {
		return nil, e
	}
	if len(rows) != 1 || rows[0][1] != "RUNNING" {
		return nil, errors.New("run not RUNNING")
	}
	task := rows[0][0]
	if e = d.transition([]string{
		fmt.Sprintf("UPDATE runs SET status='BLOCKED' WHERE id=%s AND status='RUNNING';", quote(run)),
		fmt.Sprintf("UPDATE tasks SET status='BLOCKED' WHERE id=%s AND status='RUNNING';", quote(task)),
		fmt.Sprintf("INSERT INTO task_events(task,run,event,reason) VALUES(%s,%s,'ABORT',%s);", quote(task), quote(run), quote(reason)),
	}); e != nil {
		return nil, e
	}
	return map[string]string{"run": run, "status": "BLOCKED", "next": "retry after environment/data restoration"}, nil
}

// A checkpoint certifies reconstruction; it never performs an external reset.
func (d db) validateCheckpoint(run string, contract runContract, binding RunBinding, evidence, expectedHash string) (string, string, error) {
	fixture, _, err := d.bindingFixture(binding.Identity.Fixture)
	if err != nil {
		return "", "", err
	}
	path, raw, err := loadEvidence(evidence)
	if err != nil {
		return "", "", err
	}
	hash := digestBytes(raw)
	if expectedHash != "" && hash != expectedHash {
		return "", "", errors.New("fixture checkpoint changed after recovery")
	}
	observation, err := parseStepRAW(raw, run, fixture.State, fixture.State, 0)
	if err != nil {
		return "", "", err
	}
	if err = binding.matchesRAW(observation); err != nil {
		return "", "", err
	}
	if contract.Model != nil {
		if binding != contract.Binding || path != contract.Receipt.Recovery.Evidence || hash != contract.Receipt.Recovery.SHA256 {
			return "", "", errors.New("MODEL retry requires its captured recovery under the same candidate context")
		}
		if err := d.validateModelRecovery(contract); err != nil {
			return "", "", err
		}
		return path, hash, nil
	}
	if observation.ExitCode != 0 || !observation.SafeToContinue || !conditionsMatch(observation, contract.Task.Kind, contract.Task.Condition, contract.Rule.Conditions) {
		return "", "", errors.New("fixture state or required preconditions have not been independently verified safe")
	}
	return path, hash, nil
}

func (d db) retry(task, reason, evidence, root, config, commit string) (map[string]string, error) {
	if e := validReason(reason); e != nil {
		return nil, e
	}
	rows, e := d.query("SELECT t.status,r.id,r.status FROM tasks t LEFT JOIN runs r ON r.rowid=(SELECT MAX(rr.rowid) FROM runs rr WHERE rr.task=t.id) WHERE t.id="+quote(task), 3)
	if e != nil {
		return nil, e
	}
	if len(rows) != 1 || rows[0][0] != "BLOCKED" && rows[0][0] != "FAIL" || rows[0][1] == "" || rows[0][2] != rows[0][0] {
		return nil, errors.New("only BLOCKED or FAIL tasks may be retried; PASS is immutable")
	}
	oldRun := rows[0][1]
	contract, e := d.runTaskContract(oldRun)
	if e != nil {
		return nil, e
	}
	old := contract.Binding.Identity
	binding, e := d.startBinding(task, old.Environment, commit, old.Account, old.Fixture, root, config)
	if e != nil {
		return nil, e
	}
	path, hash, e := d.validateCheckpoint(oldRun, contract, binding, evidence, "")
	if e != nil {
		return nil, e
	}
	body, e := json.Marshal(binding)
	if e != nil {
		return nil, e
	}
	statements := []string{
		fmt.Sprintf("UPDATE tasks SET status='READY' WHERE id=%s AND status=%s AND (SELECT id FROM runs WHERE task=%s ORDER BY rowid DESC LIMIT 1)=%s;", quote(task), quote(rows[0][0]), quote(task), quote(oldRun)),
		fmt.Sprintf("INSERT INTO fixture_checkpoints(run,task,binding,evidence_path,evidence_sha256) VALUES(%s,%s,%s,%s,%s);", quote(oldRun), quote(task), quote(string(body)), quote(path), quote(hash)),
	}
	fixtures, e := d.runFixtureIDs(oldRun, contract.Binding)
	if e != nil {
		return nil, e
	}
	held, e := d.query("SELECT fixture FROM fixture_locks WHERE run="+quote(oldRun), 1)
	if e != nil {
		return nil, e
	}
	allowed := map[string]bool{}
	for _, fixture := range fixtures {
		allowed[fixture] = true
	}
	for _, row := range held {
		if !allowed[row[0]] {
			return nil, errors.New("stopped run holds a fixture outside its frozen participants")
		}
		statements = append(statements, fmt.Sprintf("DELETE FROM fixture_locks WHERE fixture=%s AND run=%s;", quote(row[0]), quote(oldRun)))
	}
	statements = append(statements, fmt.Sprintf("INSERT INTO task_events(task,run,event,reason) VALUES(%s,%s,'RETRY',%s);", quote(task), quote(oldRun), quote(reason)))
	if e = d.transition(statements); e != nil {
		return nil, e
	}
	return map[string]string{"task": task, "status": "READY", "next": "start a new run with verified preconditions"}, nil
}

func (d db) validateRetryCheckpoint(task string, binding RunBinding) error {
	rows, err := d.query("SELECT r.id,r.status,COALESCE(c.binding,''),COALESCE(c.evidence_path,''),COALESCE(c.evidence_sha256,'') FROM runs r LEFT JOIN fixture_checkpoints c ON c.run=r.id WHERE r.task="+quote(task)+" ORDER BY r.rowid DESC LIMIT 1", 5)
	if err != nil || len(rows) == 0 {
		return err
	}
	if rows[0][1] == "PASS" {
		return nil
	}
	if rows[0][1] != "FAIL" && rows[0][1] != "BLOCKED" {
		return errors.New("previous attempt has not stopped")
	}
	var frozen RunBinding
	if err = decodeUniqueJSON([]byte(rows[0][2]), &frozen); err != nil || frozen != binding {
		return errors.New("new attempt requires the verified recovery context")
	}
	contract, err := d.runTaskContract(rows[0][0])
	if err != nil {
		return err
	}
	_, _, err = d.validateCheckpoint(rows[0][0], contract, binding, rows[0][3], rows[0][4])
	return err
}

type ExpectedRunContext struct {
	Status                string          `json:"status"`
	Run                   string          `json:"run"`
	Phase                 string          `json:"phase"`
	Expected              string          `json:"expected"`
	RequiredPreconditions map[string]bool `json:"required_preconditions"`
	Binding               RunBinding      `json:"binding"`
}

func (d db) expectedContext(run, phase, root, config, commit string) (ExpectedRunContext, error) {
	contract, err := d.runTaskContract(run)
	if err != nil {
		return ExpectedRunContext{}, err
	}
	result := ExpectedRunContext{Status: "EXPECTED_CONTEXT_NOT_EVIDENCE", Run: run, Phase: phase, Expected: contract.Task.Expected, Binding: contract.Binding, RequiredPreconditions: map[string]bool{}}
	for _, condition := range contract.Rule.Conditions {
		result.RequiredPreconditions[condition] = contract.Task.Kind == "POSITIVE" || condition != contract.Task.Condition
	}
	if contract.Obligation != nil {
		result.RequiredPreconditions = map[string]bool(contract.Obligation.Definition.Preconditions)
	}
	switch phase {
	case "action":
		return result, nil
	case "fixture":
		if commit == "" || contract.Status != "FAIL" && contract.Status != "BLOCKED" {
			return ExpectedRunContext{}, errors.New("fixture context requires a stopped FAIL/BLOCKED run and current --commit")
		}
		old := contract.Binding.Identity
		result.Binding, err = d.startBinding(old.Task, old.Environment, commit, old.Account, old.Fixture, root, config)
		if err != nil {
			return ExpectedRunContext{}, err
		}
		fixture, _, err := d.bindingFixture(old.Fixture)
		if err != nil {
			return ExpectedRunContext{}, err
		}
		result.Expected = fixture.State
		if contract.Model != nil {
			runner, err := d.registeredRunner(contract.Receipt.Runner)
			if err != nil {
				return ExpectedRunContext{}, err
			}
			if runner.Manifest.Recovery != nil {
				result.RequiredPreconditions = map[string]bool(runner.Manifest.Recovery.Conditions)
			}
		}
		return result, nil
	default:
		return ExpectedRunContext{}, errors.New("context --phase must be action or fixture")
	}
}
