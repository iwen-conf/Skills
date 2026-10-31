package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

var errRoundNotFound = errors.New("verification round not found")
var errRunnerNotFound = errors.New("registered runner not found")

type RegisteredRunner struct {
	Manifest   RunnerManifest `json:"manifest"`
	SHA256     string         `json:"sha256"`
	CodeSHA256 string         `json:"code_sha256"`
}

func (d db) registerRunner(manifest RunnerManifest) (RegisteredRunner, error) {
	model, err := d.model(manifest.Graph)
	if err != nil {
		return RegisteredRunner{}, err
	}
	if model.SchemaVersion != 2 {
		return RegisteredRunner{}, errors.New("registered execution requires model v2")
	}
	if _, err := validateManifest(model, manifest); err != nil {
		return RegisteredRunner{}, err
	}
	for i, path := range manifest.InputFiles {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return RegisteredRunner{}, err
		}
		manifest.InputFiles[i] = absolute
	}
	code, err := runnerCodeFingerprint(manifest)
	if err != nil {
		return RegisteredRunner{}, err
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return RegisteredRunner{}, err
	}
	registered := RegisteredRunner{Manifest: manifest, SHA256: digestBytes(body), CodeSHA256: code}
	rows, err := d.query("SELECT body,sha256,code_sha256 FROM registered_runners WHERE id="+quote(manifest.ID), 3)
	if err != nil {
		return registered, err
	}
	if len(rows) > 0 {
		if rows[0][0] != string(body) || rows[0][1] != registered.SHA256 || rows[0][2] != code {
			return registered, errors.New("runner identity is immutable; register a new ID for changed inputs or code")
		}
		return registered, nil
	}
	return registered, d.transition([]string{fmt.Sprintf("INSERT INTO registered_runners(id,graph,test,body,sha256,code_sha256) VALUES(%s,%s,%s,%s,%s,%s)", quote(manifest.ID), quote(manifest.Graph), quote(manifest.Test), quote(string(body)), quote(registered.SHA256), quote(code))})
}

func (d db) registeredRunner(id string) (RegisteredRunner, error) {
	rows, err := d.query("SELECT graph,test,body,sha256,code_sha256 FROM registered_runners WHERE id="+quote(id), 5)
	if err != nil {
		return RegisteredRunner{}, err
	}
	if len(rows) != 1 {
		return RegisteredRunner{}, errRunnerNotFound
	}
	var manifest RunnerManifest
	if err := decodeStrictJSON([]byte(rows[0][2]), &manifest); err != nil {
		return RegisteredRunner{}, err
	}
	if manifest.ID != id || manifest.Graph != rows[0][0] || manifest.Test != rows[0][1] || digestBytes([]byte(rows[0][2])) != rows[0][3] {
		return RegisteredRunner{}, errors.New("registered runner identity or hash changed")
	}
	code, err := runnerCodeFingerprint(manifest)
	if err != nil {
		return RegisteredRunner{}, err
	}
	if code != rows[0][4] {
		return RegisteredRunner{}, errors.New("registered runner code or input changed")
	}
	return RegisteredRunner{Manifest: manifest, SHA256: rows[0][3], CodeSHA256: code}, nil
}

func (d db) runnerSnapshot(graph string) (string, error) {
	rows, err := d.query("SELECT id,sha256,code_sha256 FROM registered_runners WHERE graph="+quote(graph)+" ORDER BY id LIMIT 20001", 3)
	if err != nil {
		return "", err
	}
	if len(rows) > 20000 {
		return "", errors.New("runner registry exceeds 20000 entries")
	}
	for _, row := range rows {
		runner, err := d.registeredRunner(row[0])
		if err != nil {
			return "", err
		}
		if runner.SHA256 != row[1] || runner.CodeSHA256 != row[2] {
			return "", errors.New("runner registry changed during snapshot")
		}
	}
	body, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return digestBytes(body), nil
}

func (d db) createRound(graph, environment, commit, root, config string) (VerificationRound, error) {
	result := VerificationRound{ID: newID("Q"), Created: time.Now().UTC().Format(time.RFC3339Nano)}
	model, err := d.model(graph)
	if err != nil {
		return result, err
	}
	if model.SchemaVersion != 2 {
		return result, errors.New("logical rounds require model v2")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return result, err
	}
	config, err = filepath.Abs(config)
	if err != nil {
		return result, err
	}
	head, source, err := sourceFingerprint(root)
	if err != nil {
		return result, err
	}
	if head != commit {
		return result, errors.New("round commit is not current candidate HEAD")
	}
	target, configuration, err := readApprovedEnvironment(config, environment)
	if err != nil {
		return result, err
	}
	graphHash, err := d.graphFingerprint(graph)
	if err != nil {
		return result, err
	}
	result.Context = ProofContext{Graph: graph, Environment: environment, Target: target, SourceCommit: head, SourceSHA256: source, ConfigSHA256: configuration, GraphSHA256: graphHash, SourceRoot: root, ConfigPath: config}
	result.FixtureSHA256, err = d.fixtureSnapshot(map[string]string{})
	if err != nil {
		return result, err
	}
	result.RunnersSHA256, err = d.runnerSnapshot(graph)
	if err != nil {
		return result, err
	}
	obligations, err := json.Marshal(modelObligations(model))
	if err != nil {
		return result, err
	}
	result.RequiredSHA256 = digestBytes(obligations)
	body, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	return result, d.transition([]string{fmt.Sprintf("INSERT INTO verification_rounds(id,graph,environment,body) VALUES(%s,%s,%s,%s)", quote(result.ID), quote(graph), quote(environment), quote(string(body)))})
}

func (d db) verificationRound(id string) (VerificationRound, error) {
	rows, err := d.query("SELECT graph,environment,body FROM verification_rounds WHERE id="+quote(id), 3)
	if err != nil {
		return VerificationRound{}, err
	}
	if len(rows) != 1 {
		return VerificationRound{}, errRoundNotFound
	}
	var round VerificationRound
	if err := decodeStrictJSON([]byte(rows[0][2]), &round); err != nil {
		return round, err
	}
	if round.ID != id || round.Context.Graph != rows[0][0] || round.Context.Environment != rows[0][1] {
		return round, errors.New("round identity differs from its ledger")
	}
	return round, nil
}

func (d db) validateRound(round VerificationRound) error {
	head, source, err := sourceFingerprint(round.Context.SourceRoot)
	if err != nil {
		return err
	}
	target, config, err := readApprovedEnvironment(round.Context.ConfigPath, round.Context.Environment)
	if err != nil {
		return err
	}
	graph, err := d.graphFingerprint(round.Context.Graph)
	if err != nil {
		return err
	}
	fixtures, err := d.fixtureSnapshot(map[string]string{})
	if err != nil {
		return err
	}
	runners, err := d.runnerSnapshot(round.Context.Graph)
	if err != nil {
		return err
	}
	model, err := d.model(round.Context.Graph)
	if err != nil {
		return err
	}
	obligations, err := json.Marshal(modelObligations(model))
	if err != nil {
		return err
	}
	if head != round.Context.SourceCommit || source != round.Context.SourceSHA256 || target != round.Context.Target || config != round.Context.ConfigSHA256 || graph != round.Context.GraphSHA256 || fixtures != round.FixtureSHA256 || runners != round.RunnersSHA256 || digestBytes(obligations) != round.RequiredSHA256 {
		return errors.New("verification round is stale: candidate, configuration, graph, fixtures or runners changed")
	}
	return nil
}

func (d db) modelRun(run string) (ModelRunRecord, error) {
	rows, err := d.query("SELECT round,runner,body FROM model_runs WHERE run="+quote(run), 3)
	if err != nil {
		return ModelRunRecord{}, err
	}
	if len(rows) != 1 {
		return ModelRunRecord{}, errors.New("model run has no registered execution receipt")
	}
	var record ModelRunRecord
	if err := decodeStrictJSON([]byte(rows[0][2]), &record); err != nil {
		return record, err
	}
	if record.Run != run || record.Round != rows[0][0] || record.Runner != rows[0][1] {
		return record, errors.New("model run receipt identity mismatch")
	}
	return record, nil
}

func (d db) updateModelRun(record ModelRunRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return d.transition([]string{fmt.Sprintf("UPDATE model_runs SET body=%s WHERE run=%s AND round=%s AND runner=%s", quote(string(body)), quote(record.Run), quote(record.Round), quote(record.Runner))})
}

func (d db) participantBindings(definition TestDefinition, binding RunBinding, accounts map[string]string) ([]ParticipantBinding, error) {
	result := []ParticipantBinding{{Alias: "primary", Role: binding.Identity.Actor, Account: binding.Identity.Account, Principal: binding.Identity.Principal, SHA256: binding.Identity.AccountSHA256}}
	if len(accounts) != len(definition.Actors) {
		return nil, errors.New("all and only declared participant aliases must be bound")
	}
	for _, actor := range definition.Actors {
		id, found := accounts[actor.Alias]
		if !found || actor.Alias == "primary" {
			return nil, errors.New("participant alias is missing or reserved")
		}
		fixture, hash, err := d.bindingFixture(id)
		if err != nil {
			return nil, err
		}
		if fixture.Kind != "account" || fixture.Refs["role"] != actor.Role || fixture.Refs["environment"] != binding.Identity.Environment || fixture.Refs["principal"] == "" {
			return nil, errors.New("participant role, principal or environment mismatch")
		}
		result = append(result, ParticipantBinding{Alias: actor.Alias, Role: actor.Role, Account: id, Principal: fixture.Refs["principal"], SHA256: hash})
	}
	return result, nil
}

func (d db) validateModelRecord(g Graph, obligation ModelObligation, binding RunBinding, record ModelRunRecord) error {
	round, err := d.verificationRound(record.Round)
	if err != nil {
		return err
	}
	if err := d.validateRound(round); err != nil {
		return err
	}
	v := binding.Identity
	context := round.Context
	if v.Graph != context.Graph || v.Environment != context.Environment || v.Target != context.Target || v.SourceCommit != context.SourceCommit || v.SourceSHA256 != context.SourceSHA256 || v.ConfigSHA256 != context.ConfigSHA256 || v.GraphSHA256 != context.GraphSHA256 || binding.SourceRoot != context.SourceRoot || binding.ConfigPath != context.ConfigPath {
		return errors.New("execution binding belongs to another round context")
	}
	runner, err := d.registeredRunner(record.Runner)
	if err != nil {
		return err
	}
	if runner.Manifest.Graph != g.ID || runner.Manifest.Test != obligation.Definition.ID || runner.SHA256 != record.RunnerSHA256 || runner.CodeSHA256 != record.CodeSHA256 {
		return errors.New("execution receipt differs from its registered runner")
	}
	if _, err := validateManifest(g, runner.Manifest); err != nil {
		return err
	}
	aliases := map[string]string{}
	for _, participant := range record.Participants {
		if participant.Alias != "primary" {
			aliases[participant.Alias] = participant.Account
		}
	}
	expected, err := d.participantBindings(obligation.Definition, binding, aliases)
	if err != nil {
		return err
	}
	a, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	b, err := json.Marshal(record.Participants)
	if err != nil {
		return err
	}
	if string(a) != string(b) {
		return errors.New("execution participants differ from their frozen accounts")
	}
	return nil
}

func (d db) runFixtureIDs(run string, binding RunBinding) ([]string, error) {
	ids := bindingFixtureIDs(binding)
	model, err := d.extendedModel(binding.Identity.Graph)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return ids, nil
	}
	record, err := d.modelRun(run)
	if err != nil {
		return nil, err
	}
	for _, participant := range record.Participants {
		ids = append(ids, participant.Account)
	}
	return uniqueIDs(ids), nil
}
