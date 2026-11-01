package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	exploreNew           = "new"
	exploreResume        = "resume"
	exploreAbort         = "abort"
	exploreStatus        = "status"
	exploreRunning       = "running"
	exploreInterrupted   = "interrupted"
	exploreCompleted     = "completed"
	exploreAborted       = "aborted"
	exploreBlocked       = "blocked"
	actionPlanned        = "planned"
	actionClaimed        = "claimed"
	actionDispatching    = "dispatching"
	actionOutcomeUnknown = "outcome_unknown"
	actionObserved       = "observed"
	actionRecorded       = "recorded"
	actionBlocked        = "blocked"
	actionExecuting      = actionDispatching
	actionUnknown        = actionOutcomeUnknown
	actionCompleted      = actionRecorded
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
	Candidate          CandidateAction  `json:"candidate"`
	Status             string           `json:"status"`
	SideEffecting      bool             `json:"side_effecting"`
	Before             IndependentFacts `json:"before"`
	After              IndependentFacts `json:"after,omitempty"`
	Step               *ExploreStep     `json:"step,omitempty"`
	AttemptID          string           `json:"attempt_id,omitempty"`
	StepID             string           `json:"step_id,omitempty"`
	RequestID          string           `json:"request_id,omitempty"`
	IdempotencyKey     string           `json:"idempotency_key,omitempty"`
	Resource           string           `json:"resource,omitempty"`
	Actor              string           `json:"actor,omitempty"`
	Worker             string           `json:"worker,omitempty"`
	Fence              int64            `json:"fence,omitempty"`
	Dispatch           string           `json:"dispatch,omitempty"`
	Response           json.RawMessage  `json:"response,omitempty"`
	RAW                string           `json:"raw,omitempty"`
	RAWSHA             string           `json:"raw_sha256,omitempty"`
	Recorded           string           `json:"recorded,omitempty"`
	ReplayForbidden    bool             `json:"replay_forbidden,omitempty"`
	EvidenceIncomplete bool             `json:"evidence_incomplete,omitempty"`
	FirstObservation   string           `json:"first_observation,omitempty"`
	ErrorReason        string           `json:"error_reason,omitempty"`
}

type ExploreCheckpoint struct {
	Campaign         string                `json:"campaign"`
	Graph            string                `json:"graph"`
	Flow             string                `json:"flow"`
	Environment      string                `json:"environment"`
	Round            string                `json:"round"`
	ModelSHA         string                `json:"model_sha"`
	SourceCommit     string                `json:"source_commit"`
	Status           string                `json:"status"`
	World            ExplorePersistedWorld `json:"world"`
	Seen             []string              `json:"seen"`
	Tried            []string              `json:"tried"`
	Frontier         []CandidateAction     `json:"frontier"`
	Actions          []ExploreActionRecord `json:"actions"`
	Budget           ExploreRuntimeBudget  `json:"budget"`
	Spent            ExploreSpent          `json:"spent"`
	Recovery         RecoveryOutcome       `json:"recovery"`
	RecoveryClass    RecoveryClass         `json:"recovery_class"`
	Account          string                `json:"account"`
	Fixture          string                `json:"fixture"`
	Checkpoint       string                `json:"checkpoint"`
	ClaimedBy        string                `json:"claimed_by,omitempty"`
	Fence            int64                 `json:"fence,omitempty"`
	Mode             string                `json:"mode,omitempty"`
	Report           ExploreReport         `json:"report"`
	Updated          string                `json:"updated,omitempty"`
	Binding          RunBinding            `json:"binding,omitempty"`
	UnsafeResume     bool                  `json:"unsafe_resume,omitempty"`
	ResumeHoldReason string                `json:"resume_hold_reason,omitempty"`
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

func (d db) insertExploreCampaign(state ExploreCheckpoint) error {
	if state.Campaign == "" {
		return errors.New("exploration checkpoint needs a campaign")
	}
	if err := d.migrateExploreLeaseColumns(); err != nil {
		return err
	}
	if state.Status == "" {
		state.Status = exploreNew
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
		"INSERT INTO exploration_state(campaign,graph,status,model_sha,source_commit,claimed_by,claimed_at,fence,lease_until,checkpoint,body,updated) VALUES(%s,%s,%s,%s,%s,%s,%s,%d,%s,%s,%s,%s);",
		quote(state.Campaign), quote(state.Graph), quote(state.Status), quote(state.ModelSHA), quote(state.SourceCommit),
		quote(state.ClaimedBy), quote(state.Updated), state.Fence, quote(state.Updated), quote(state.Checkpoint), quote(string(body)), quote(state.Updated),
	)})
}

func (d db) saveExploreCheckpoint(state ExploreCheckpoint) error {
	return d.insertExploreCampaign(state)
}

func (d db) loadExploreCheckpoint(campaign string) (ExploreCheckpoint, error) {
	state := ExploreCheckpoint{}
	if campaign == "" {
		return state, errors.New("discover status needs --id campaign")
	}
	rows, err := d.query("SELECT body, COALESCE(fence,0), COALESCE(checkpoint,''), COALESCE(status,''), COALESCE(claimed_by,''), COALESCE(lease_until,'') FROM exploration_state WHERE campaign="+quote(campaign), 6)
	if err != nil {
		return state, err
	}
	if len(rows) != 1 {
		return state, fmt.Errorf("exploration campaign %s was not found", campaign)
	}
	if err := json.Unmarshal([]byte(rows[0][0]), &state); err != nil {
		return state, err
	}
	fence, err := strconv.ParseInt(rows[0][1], 10, 64)
	if err != nil {
		return state, err
	}
	state.Fence = fence
	if rows[0][2] != "" {
		state.Checkpoint = rows[0][2]
	}
	if rows[0][3] != "" {
		state.Status = rows[0][3]
	}
	state.ClaimedBy = rows[0][4]
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

func (d db) abortExploreCampaign(campaign, reason string) (ExploreCheckpoint, error) {
	state, err := d.loadExploreCheckpoint(campaign)
	if err != nil {
		return state, err
	}
	if state.Status == exploreCompleted || state.Status == exploreAborted {
		return state, fmt.Errorf("campaign %s is %s", campaign, state.Status)
	}
	// An aborted external request is not necessarily rolled back. Preserve a
	// permanent hold for any dispatched action without a recorded outcome.
	priorFence, priorCheckpoint := state.Fence, state.Checkpoint
	for _, item := range state.Actions {
		if item.SideEffecting && item.Status != actionRecorded && item.Status != actionPlanned {
			state.UnsafeResume = true
			state.ResumeHoldReason = "aborted with unresolved side-effecting attempt"
			state.RecoveryClass = RecoveryFailedLocked
			state.Report.UnsafeResume = true
			state.Report.RecoveryClass = RecoveryFailedLocked
			state.Report.Truncated = true
			state.Report.Blocked = append(state.Report.Blocked, ExploreFinding{
				Kind: "BLOCKED", Reason: state.ResumeHoldReason, Evidence: item.AttemptID,
			})
			break
		}
	}
	state.Status = exploreAborted
	state.Report.StopReason = "aborted"
	if reason != "" {
		state.Report.StopReason = "aborted:" + reason
	}
	state.ClaimedBy = ""
	state.Fence++
	state.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	if state.Checkpoint == "" {
		state.Checkpoint = newID("CK")
	}
	body, err := json.Marshal(state)
	if err != nil {
		return state, err
	}
	if err := d.migrateExploreLeaseColumns(); err != nil {
		return state, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return state, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, fmt.Sprintf(
		`UPDATE exploration_state SET status=%s, claimed_by='', lease_until='', fence=fence+1, checkpoint=%s, body=%s, updated=%s
WHERE campaign=%s AND status NOT IN (%s,%s) AND fence=%d AND checkpoint=%s`,
		quote(exploreAborted), quote(state.Checkpoint), quote(string(body)), quote(state.Updated),
		quote(campaign), quote(exploreCompleted), quote(exploreAborted), priorFence, quote(priorCheckpoint),
	))
	if err != nil {
		return state, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return state, err
	}
	if changed != 1 {
		return state, errors.New("abort lost the race with another owner or terminal status")
	}
	if err := tx.Commit(); err != nil {
		return state, err
	}
	return state, nil
}

var errUnreplayableSideEffects = errors.New("campaign holds unreplayed side effects; resume is blocked until evidence is completed")

const exploreIdentityContract = "explore-identity-v1"

func exploreWorkerID() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), newID("W"))
}

func (d db) reconcileUnknownActions(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, state *ExploreCheckpoint, lease ExploreLease) error {
	if state == nil {
		return errors.New("reconcile needs a campaign state")
	}
	if lease.Worker == "" || lease.Fence < 1 || lease.Campaign != "" && lease.Campaign != state.Campaign {
		return errors.New("reconcile needs the current worker fencing token")
	}
	stepTimeout := time.Duration(adapter.StepTimeoutMS) * time.Millisecond
	changed := false
	for i := range state.Actions {
		record := &state.Actions[i]
		switch record.Status {
		case actionObserved:
			// A persisted RAW path alone is not durable evidence. A crash may
			// have occurred during the file write, or the file may have been
			// replaced. Never resume side effects until the recorded bytes,
			// campaign and execution identity are all verified.
			if record.Step == nil || record.Step.RAW == "" {
				if record.After.State == "" && adapter.Oracle.ID != "" {
					oracleAccount := accountFor(accounts, record.Candidate)
					readCtx, cancel := context.WithTimeout(ctx, stepTimeout)
					after, _, err := readOracle(readCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
					cancel()
					if err == nil {
						record.After = after
						if record.Step != nil {
							record.Step.After = after
						}
					}
				}
				sealUnreplayableAction(state, record, "observed without RAW; replay is forbidden")
			} else {
				body, err := os.ReadFile(record.Step.RAW)
				if err != nil || record.Step.SHA256 == "" || digestBytes(body) != record.Step.SHA256 {
					sealUnreplayableAction(state, record, "observed RAW is missing, unreadable or fails its SHA256; replay is forbidden")
				} else if err := rawMatchesBinding(body, binding); err != nil {
					sealUnreplayableAction(state, record, "observed RAW identity mismatch; replay is forbidden: "+err.Error())
				} else if err := rawMatchesCampaign(body, EvidenceGateInput{
					Report: state.Report, Binding: binding, ExpectedCampaign: state.Campaign,
					ExpectedFixture: binding.Identity.Fixture,
					ExpectedActor:   firstNonEmpty(binding.Identity.Actor, binding.Identity.Principal),
				}); err != nil {
					sealUnreplayableAction(state, record, "observed RAW campaign mismatch; replay is forbidden: "+err.Error())
				} else {
					record.Status = actionRecorded
					record.RAW = record.Step.RAW
					record.RAWSHA = record.Step.SHA256
				}
			}
			changed = true
			continue
		case actionRecorded, actionBlocked, actionPlanned:
			if record.ReplayForbidden || record.EvidenceIncomplete {
				sealUnreplayableAction(state, record, firstNonEmpty(record.ErrorReason, "side-effecting action remains unreplayable"))
				changed = true
			}
			continue
		case actionDispatching, actionOutcomeUnknown, actionClaimed:
		default:
			continue
		}
		if record.SideEffecting {
			record.Status = actionOutcomeUnknown
			oracleAccount := accountFor(accounts, record.Candidate)
			readCtx, cancel := context.WithTimeout(ctx, stepTimeout)
			after, _, err := readOracle(readCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
			cancel()
			if err != nil {
				return fmt.Errorf("cannot judge an in-flight action without an independent oracle: %w", err)
			}
			record.After = after
			if record.Before.State != "" && after.State == record.Before.State && !measurementGrew(record.Before, after, "records") {
				sealUnreplayableAction(state, record, "action was in-flight; side-effecting replay is forbidden")
				changed = true
				continue
			}
			sealUnreplayableAction(state, record, "oracle observed a completed side effect without RAW; replay is forbidden")
			changed = true
			continue
		}
		record.Status = actionBlocked
		changed = true
	}
	if changed {
		return d.saveExploreCheckpointOwned(*state, lease)
	}
	return nil
}

func sealUnreplayableAction(state *ExploreCheckpoint, record *ExploreActionRecord, reason string) {
	if state == nil || record == nil {
		return
	}
	if record.AttemptID == "" {
		record.AttemptID = newID("AT")
	}
	if record.FirstObservation == "" {
		record.FirstObservation = time.Now().UTC().Format(time.RFC3339Nano)
	}
	record.Status = actionBlocked
	record.ReplayForbidden = true
	record.EvidenceIncomplete = true
	record.ErrorReason = reason
	if record.Step == nil {
		record.Step = &ExploreStep{Candidate: record.Candidate, Before: record.Before, After: record.After}
	}
	record.Step.Findings = append(record.Step.Findings, ExploreFinding{Kind: "BLOCKED", Reason: reason, Evidence: record.AttemptID})
	state.Tried = appendUnique(state.Tried, candidateKey(record.Candidate, record.Before.State))
	if record.Candidate.ID != "" {
		state.Tried = appendUnique(state.Tried, record.Candidate.ID)
	}
	if record.RequestID != "" {
		state.Tried = appendUnique(state.Tried, "request:"+record.RequestID)
	}
	state.Frontier = excludeFrontierAction(state.Frontier, record.Candidate, record.RequestID)
	state.Report.Remaining = excludeFrontierAction(state.Report.Remaining, record.Candidate, record.RequestID)
	state.UnsafeResume = true
	state.ResumeHoldReason = reason
	state.Status = exploreBlocked
	state.Report.UnsafeResume = true
	state.Report.StopReason = "blocked"
	state.Report.Truncated = true
	state.RecoveryClass = RecoveryFailedLocked
	state.Report.RecoveryClass = RecoveryFailedLocked
	state.Report.Blocked = append(state.Report.Blocked, ExploreFinding{Kind: "BLOCKED", Reason: reason, Evidence: record.AttemptID})
}

func excludeFrontierAction(frontier []CandidateAction, item CandidateAction, requestID string) []CandidateAction {
	out := []CandidateAction{}
	for _, cand := range frontier {
		if item.ID != "" && cand.ID == item.ID {
			continue
		}
		if requestID != "" && cand.RequestID == requestID {
			continue
		}
		if cand.Action == item.Action && cand.Role == item.Role && cand.Entry == item.Entry && cand.Kind == item.Kind {
			continue
		}
		out = append(out, cand)
	}
	return out
}

func campaignHoldsUnreplayableSideEffects(state ExploreCheckpoint) bool {
	if state.UnsafeResume || state.Report.UnsafeResume {
		return true
	}
	for _, record := range state.Actions {
		if record.ReplayForbidden || record.EvidenceIncomplete {
			return true
		}
		if record.Status == actionObserved && (record.Step == nil || record.Step.RAW == "") {
			return true
		}
	}
	return false
}

func keepsExploreFixture(reason string) bool {
	switch reason {
	case "budget_actions", "budget_states", "budget_time", "budget_replans", "budget_planner", "planner_wait":
		return true
	default:
		return false
	}
}

func sideEffecting(candidate CandidateAction) bool {
	return candidate.Kind != kindRecover && candidate.Action != "" && candidate.Action != "fixture.restore"
}

func (d db) finishExplore(ctx context.Context, report ExploreReport, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, actors map[string]ParticipantBinding, fixture Fixture, campaign string, execErr error, fault ExploreFault) (ExploreReport, error) {
	started := time.Now()
	if report.UnsafeResume || errors.Is(execErr, errUnreplayableSideEffects) || (execErr != nil && errors.Is(execErr, errExploreCrashSentinel) && strings.Contains(execErr.Error(), "after_observed_before_raw")) {
		report.UnsafeResume = true
		report.StopReason = firstNonEmpty(report.StopReason, "blocked")
		report.Truncated = true
		report.Recovery = RecoveryOutcome{Verdict: "BLOCKED", Reason: "unreplayable side effect; fixture remains locked", Safe: false, Actions: []ActionObservation{}}
		report.RecoveryClass = RecoveryFailedLocked
		if persistErr := d.persistExploration(report); persistErr != nil {
			if execErr != nil {
				return report, errors.Join(execErr, persistErr)
			}
			return report, persistErr
		}
		if execErr != nil {
			return report, execErr
		}
		return report, errUnreplayableSideEffects
	}
	if fault.Window == "during_recovery" {
		report.Recovery = RecoveryOutcome{Verdict: "BLOCKED", Reason: "explore crash: during_recovery", Safe: false, Actions: []ActionObservation{}}
		report.RecoveryClass = RecoveryFailedLocked
		if persistErr := d.persistExploration(report); persistErr != nil {
			return report, errors.Join(errExploreCrash("during_recovery"), persistErr)
		}
		return report, errExploreCrash("during_recovery")
	}
	if report.Truncated && keepsExploreFixture(report.StopReason) {
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
