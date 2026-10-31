package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	exploreNew         = "new"
	exploreResume      = "resume"
	exploreAbort       = "abort"
	exploreStatus      = "status"
	exploreRunning     = "running"
	exploreInterrupted = "interrupted"
	exploreCompleted   = "completed"
	exploreAborted     = "aborted"
	exploreBlocked     = "blocked"
	actionPlanned      = "planned"
	actionExecuting    = "executing"
	actionUnknown      = "executed_unknown"
	actionCompleted    = "completed"
)

type ExplorePersistedWorld struct {
	State      string                     `json:"state"`
	Role       string                     `json:"role"`
	Principal  string                     `json:"principal"`
	Account    string                     `json:"account"`
	LastAction string                     `json:"last_action"`
	LastID     string                     `json:"last_id"`
	Conditions map[string]bool            `json:"conditions"`
	History    map[string]json.RawMessage `json:"history"`
	LastDenied bool                       `json:"last_denied"`
	LastTO     bool                       `json:"last_to"`
}

type ExploreActionRecord struct {
	Candidate     CandidateAction  `json:"candidate"`
	Status        string           `json:"status"`
	SideEffecting bool             `json:"side_effecting"`
	Before        IndependentFacts `json:"before"`
	Step          *ExploreStep     `json:"step,omitempty"`
}

type ExploreCheckpoint struct {
	Campaign      string                `json:"campaign"`
	Graph         string                `json:"graph"`
	Flow          string                `json:"flow"`
	Environment   string                `json:"environment"`
	Round         string                `json:"round"`
	ModelSHA      string                `json:"model_sha"`
	SourceCommit  string                `json:"source_commit"`
	Status        string                `json:"status"`
	World         ExplorePersistedWorld `json:"world"`
	Seen          []string              `json:"seen"`
	Tried         []string              `json:"tried"`
	Frontier      []CandidateAction     `json:"frontier"`
	Actions       []ExploreActionRecord `json:"actions"`
	Budget        ExploreRuntimeBudget  `json:"budget"`
	Spent         ExploreSpent          `json:"spent"`
	Recovery      RecoveryOutcome       `json:"recovery"`
	RecoveryClass RecoveryClass         `json:"recovery_class"`
	Account       string                `json:"account"`
	Fixture       string                `json:"fixture"`
	Checkpoint    string                `json:"checkpoint"`
	ClaimedBy     string                `json:"claimed_by,omitempty"`
	Mode          string                `json:"mode,omitempty"`
	Report        ExploreReport         `json:"report"`
	Updated       string                `json:"updated,omitempty"`
}

func persistWorld(world exploreWorld) ExplorePersistedWorld {
	return ExplorePersistedWorld{
		State: world.state, Role: world.role, Principal: world.principal, Account: world.account,
		LastAction: world.lastAction, LastID: world.lastID, Conditions: world.conditions,
		History: world.history, LastDenied: world.lastDenied, LastTO: world.lastTO,
	}
}

func restoreWorld(persisted ExplorePersistedWorld) exploreWorld {
	return exploreWorld{
		state: persisted.State, role: persisted.Role, principal: persisted.Principal, account: persisted.Account,
		conditions: persisted.Conditions, history: persisted.History, lastAction: persisted.LastAction,
		lastID: persisted.LastID, lastDenied: persisted.LastDenied, lastTO: persisted.LastTO,
	}
}

func (d db) saveExploreCheckpoint(state ExploreCheckpoint) error {
	if state.Campaign == "" {
		return errors.New("exploration checkpoint needs a campaign")
	}
	state.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	if state.Checkpoint == "" {
		state.Checkpoint = newID("CK")
	}
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return d.tx([]string{fmt.Sprintf(
		"INSERT INTO exploration_state(campaign,graph,status,model_sha,source_commit,claimed_by,claimed_at,checkpoint,body,updated) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s) ON CONFLICT(campaign) DO UPDATE SET graph=excluded.graph,status=excluded.status,model_sha=excluded.model_sha,source_commit=excluded.source_commit,claimed_by=excluded.claimed_by,claimed_at=excluded.claimed_at,checkpoint=excluded.checkpoint,body=excluded.body,updated=excluded.updated;",
		quote(state.Campaign), quote(state.Graph), quote(state.Status), quote(state.ModelSHA), quote(state.SourceCommit),
		quote(state.ClaimedBy), quote(state.Updated), quote(state.Checkpoint), quote(string(body)), quote(state.Updated),
	)})
}

func (d db) loadExploreCheckpoint(campaign string) (ExploreCheckpoint, error) {
	state := ExploreCheckpoint{}
	if campaign == "" {
		return state, errors.New("discover status needs --id campaign")
	}
	rows, err := d.query("SELECT body FROM exploration_state WHERE campaign="+quote(campaign), 1)
	if err != nil {
		return state, err
	}
	if len(rows) != 1 {
		return state, fmt.Errorf("exploration campaign %s was not found", campaign)
	}
	if err := json.Unmarshal([]byte(rows[0][0]), &state); err != nil {
		return state, err
	}
	return state, nil
}

func (d db) listExploreCheckpoints(graph string) ([]ExploreCheckpoint, error) {
	clause := ""
	if graph != "" {
		clause = " WHERE graph=" + quote(graph)
	}
	rows, err := d.query("SELECT body FROM exploration_state"+clause+" ORDER BY updated DESC LIMIT 32", 1)
	if err != nil {
		return nil, err
	}
	out := []ExploreCheckpoint{}
	for _, row := range rows {
		var item ExploreCheckpoint
		if json.Unmarshal([]byte(row[0]), &item) == nil {
			out = append(out, item)
		}
	}
	return out, nil
}

func (d db) claimExploreCampaign(campaign, worker string) error {
	rows, err := d.query("SELECT claimed_by,status FROM exploration_state WHERE campaign="+quote(campaign), 2)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("exploration campaign %s was not found", campaign)
	}
	if rows[0][0] != "" && rows[0][0] != worker && rows[0][1] == exploreRunning {
		return errors.New("campaign is already claimed by another worker")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return d.tx([]string{fmt.Sprintf("UPDATE exploration_state SET claimed_by=%s, claimed_at=%s, status=%s WHERE campaign=%s", quote(worker), quote(now), quote(exploreRunning), quote(campaign))})
}

func (d db) abortExploreCampaign(campaign, reason string) (ExploreCheckpoint, error) {
	state, err := d.loadExploreCheckpoint(campaign)
	if err != nil {
		return state, err
	}
	state.Status = exploreAborted
	state.Report.StopReason = "aborted"
	state.ClaimedBy = ""
	if reason != "" {
		state.Report.StopReason = "aborted:" + reason
	}
	return state, d.saveExploreCheckpoint(state)
}

func exploreWorkerID() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), newID("W"))
}

func (d db) reconcileUnknownActions(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, state *ExploreCheckpoint) error {
	stepTimeout := time.Duration(adapter.StepTimeoutMS) * time.Millisecond
	changed := false
	for i := range state.Actions {
		record := &state.Actions[i]
		if record.Status != actionExecuting && record.Status != actionUnknown {
			continue
		}
		if record.SideEffecting {
			record.Status = actionUnknown
			oracleAccount := accountFor(accounts, record.Candidate)
			readCtx, cancel := context.WithTimeout(ctx, stepTimeout)
			after, _, err := readOracle(readCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
			cancel()
			if err != nil {
				return fmt.Errorf("cannot judge an in-flight action without an independent oracle: %w", err)
			}
			if record.Before.State != "" && after.State == record.Before.State && !measurementGrew(record.Before, after, "records") {
				record.Status = actionUnknown
				record.Step = &ExploreStep{Candidate: record.Candidate, Before: record.Before, After: after, Findings: []ExploreFinding{{Kind: "BLOCKED", Reason: "action was in-flight; side-effecting replay is forbidden"}}}
				state.Tried = appendUnique(state.Tried, candidateKey(record.Candidate, record.Before.State))
				changed = true
				continue
			}
			g, err := d.model(binding.Identity.Graph)
			if err != nil {
				return err
			}
			approvals, err := d.ruleApprovals(binding.Identity.Graph)
			if err != nil {
				return err
			}
			step := ExploreStep{Candidate: record.Candidate, Before: record.Before, After: after, Findings: judgeExploration(g, approvals, actorRole(accounts, record.Candidate), record.Candidate.Action, record.Before, after, false)}
			step.Findings = append(step.Findings, invariantFindings(g, after)...)
			record.Status = actionCompleted
			record.Step = &step
			state.Report.Steps = append(state.Report.Steps, step)
			state.Report.Actions++
			changed = true
			continue
		}
		record.Status = actionPlanned
		changed = true
	}
	if changed {
		return d.saveExploreCheckpoint(*state)
	}
	return nil
}

func sideEffecting(candidate CandidateAction) bool {
	return candidate.Kind != kindRecover && candidate.Action != "" && candidate.Action != "fixture.restore"
}

func (d db) finishExplore(ctx context.Context, report ExploreReport, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, actors map[string]ParticipantBinding, fixture Fixture, campaign string, execErr error) (ExploreReport, error) {
	started := time.Now()
	if report.Truncated && (report.StopReason == "budget_actions" || report.StopReason == "budget_states" || report.StopReason == "budget_time" || report.StopReason == "budget_replans") {
		report.Recovery = RecoveryOutcome{Verdict: "NOT_RUN", Reason: "truncated campaign keeps the fixture locked for resume", Actions: []ActionObservation{}}
		report.RecoveryClass = RecoveryNotAttempted
		report.Spent.Actions = report.Actions
		report.Spent.States = report.States
		if report.Spent.ElapsedMS == 0 {
			report.Spent.ElapsedMS = time.Since(started).Milliseconds()
		}
		if err := d.persistExploration(report); err != nil {
			if execErr != nil {
				return report, errors.Join(execErr, err)
			}
			return report, err
		}
		if execErr != nil {
			return report, execErr
		}
		return report, nil
	}
	report.Recovery = d.restoreExplore(ctx, binding, adapter, accounts, actors, fixture, campaign)
	locks, lockErr := d.query("SELECT COUNT(*) FROM fixture_locks WHERE run="+quote(campaign), 1)
	lockCount := 0
	if lockErr != nil {
		report.RecoveryClass = RecoveryUnknown
		if report.Recovery.Reason != "" {
			report.Recovery.Reason += "; "
		}
		report.Recovery.Reason += "lock query failed: " + lockErr.Error()
	} else if len(locks) == 1 {
		lockCount, _ = strconv.Atoi(locks[0][0])
		report.RecoveryClass = classifyRecovery(report.Recovery, lockCount)
	}
	if report.RecoveryClass == RecoveryRestored {
		if err := d.transition([]string{fmt.Sprintf("DELETE FROM fixture_locks WHERE run=%s", quote(campaign))}); err != nil {
			report.RecoveryClass = RecoveryUnknown
			report.Recovery.Safe = false
			report.Recovery.Reason += "; unlock failed: " + err.Error()
		}
	}
	if report.Spent.ElapsedMS == 0 {
		report.Spent.ElapsedMS = time.Since(started).Milliseconds()
	}
	report.Spent.Actions = report.Actions
	report.Spent.States = report.States
	if err := d.persistExploration(report); err != nil {
		if execErr != nil {
			return report, errors.Join(execErr, err)
		}
		return report, err
	}
	if execErr != nil {
		return report, execErr
	}
	return report, nil
}

func versionsMatch(state ExploreCheckpoint, commit, modelSHA string) error {
	if state.SourceCommit != "" && commit != "" && state.SourceCommit != commit {
		return fmt.Errorf("refusing to reuse campaign %s: source commit changed from %s to %s", state.Campaign, state.SourceCommit, commit)
	}
	if state.ModelSHA != "" && modelSHA != "" && state.ModelSHA != modelSHA {
		return fmt.Errorf("refusing to reuse campaign %s: model digest changed", state.Campaign)
	}
	return nil
}
