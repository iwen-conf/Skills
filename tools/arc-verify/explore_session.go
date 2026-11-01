package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

type exploreDrive struct {
	lease     ExploreLease
	state     *ExploreCheckpoint
	fault     ExploreFault
	candidate CandidateAction
	attemptID string
	before    IndependentFacts
	after     IndependentFacts
	step      *ExploreStep
}

var errExploreCrashSentinel = errors.New("explore crash")

func errExploreCrash(window string) error {
	return fmt.Errorf("%w: %s", errExploreCrashSentinel, window)
}

func legalExploreActionTransition(from, to string) bool {
	if from == to {
		return true
	}
	allowed := map[string][]string{
		"":                   {actionPlanned, actionClaimed, actionDispatching, actionBlocked},
		actionPlanned:        {actionClaimed, actionDispatching, actionBlocked},
		actionClaimed:        {actionDispatching, actionOutcomeUnknown, actionBlocked},
		actionDispatching:    {actionOutcomeUnknown, actionObserved, actionBlocked},
		actionOutcomeUnknown: {actionObserved, actionRecorded, actionBlocked},
		actionObserved:       {actionRecorded, actionBlocked},
	}
	for _, item := range allowed[from] {
		if item == to {
			return true
		}
	}
	return false
}

func upsertExploreAction(actions []ExploreActionRecord, record ExploreActionRecord) []ExploreActionRecord {
	if record.AttemptID == "" {
		record.AttemptID = newID("AT")
	}
	if record.RequestID == "" {
		record.RequestID = record.Candidate.RequestID
	}
	for i := range actions {
		if actions[i].AttemptID != "" && actions[i].AttemptID == record.AttemptID {
			if actions[i].Status == actionRecorded || actions[i].Status == actionBlocked {
				return actions
			}
			if !legalExploreActionTransition(actions[i].Status, record.Status) {
				return actions
			}
			if actions[i].FirstObservation != "" {
				record.FirstObservation = actions[i].FirstObservation
			}
			record.AttemptID = actions[i].AttemptID
			actions[i] = record
			return actions
		}
	}
	return append(actions, record)
}

func (d db) markExploreAction(drive *exploreDrive, status string, before IndependentFacts) error {
	if drive == nil || drive.state == nil {
		return nil
	}
	if err := d.ensureExploreLease(drive); err != nil {
		return err
	}
	if drive.fault.Window == "db_write_fail" && (status == actionRecorded || status == actionObserved) {
		return errExploreCrash("db_write_fail")
	}
	if drive.attemptID == "" {
		drive.attemptID = newID("AT")
	}
	record := ExploreActionRecord{
		Candidate:      drive.candidate,
		Status:         status,
		SideEffecting:  sideEffecting(drive.candidate),
		Before:         before,
		After:          drive.after,
		AttemptID:      drive.attemptID,
		StepID:         firstNonEmpty(drive.candidate.ID, drive.attemptID),
		RequestID:      drive.candidate.RequestID,
		IdempotencyKey: drive.candidate.RequestID,
		Resource:       firstNonEmpty(drive.candidate.Scope, drive.candidate.Entry),
		Actor:          drive.candidate.Role,
		Worker:         drive.lease.Worker,
		Fence:          drive.lease.Fence,
		Dispatch:       status,
	}
	if drive.step != nil && (status == actionObserved || status == actionRecorded) {
		copied := *drive.step
		record.Step = &copied
		if copied.RAW != "" {
			record.RAW = copied.RAW
			record.RAWSHA = copied.SHA256
		}
	}
	if status == actionObserved && record.FirstObservation == "" {
		record.FirstObservation = time.Now().UTC().Format(time.RFC3339Nano)
	}
	drive.state.Actions = upsertExploreAction(drive.state.Actions, record)
	drive.state.ClaimedBy = drive.lease.Worker
	drive.state.Fence = drive.lease.Fence
	drive.state.Status = exploreRunning
	if err := d.saveExploreCheckpointOwned(*drive.state, drive.lease); err != nil {
		return err
	}
	return nil
}

func (d db) persistExploreDrive(drive *exploreDrive, report ExploreReport, world exploreWorld, tried map[string]bool, budget ExploreRuntimeBudget, status string) error {
	if drive == nil || drive.state == nil {
		return errors.New("exploration drive is missing")
	}
	if err := d.ensureExploreLease(drive); err != nil {
		return err
	}
	drive.state.Status = status
	drive.state.World = persistWorld(world)
	drive.state.Tried = mapKeys(tried)
	drive.state.Frontier = report.Remaining
	drive.state.Budget = budget
	drive.state.Spent = report.Spent
	drive.state.Recovery = report.Recovery
	drive.state.RecoveryClass = report.RecoveryClass
	drive.state.Report = report
	drive.state.Checkpoint = firstNonEmpty(report.Checkpoint, drive.state.Checkpoint, newID("CK"))
	drive.state.ClaimedBy = drive.lease.Worker
	drive.state.Fence = drive.lease.Fence
	if err := d.saveExploreCheckpointOwned(*drive.state, drive.lease); err != nil {
		return err
	}
	report.Checkpoint = drive.state.Checkpoint
	return nil
}

func exploreStatusFor(report ExploreReport) string {
	if report.StopReason == "blocked" || report.StopReason == "recovery_blocked" || report.StopReason == "classify_failed" {
		return exploreBlocked
	}
	if report.Truncated {
		return exploreInterrupted
	}
	return exploreCompleted
}

func crashAt(fault ExploreFault, window string) error {
	if fault.Window != window {
		return nil
	}
	if fault.Notify != "" {
		if err := os.WriteFile(fault.Notify, []byte(window+"\n"), 0o600); err != nil {
			return err
		}
	}
	if fault.HangWindow > 0 {
		time.Sleep(fault.HangWindow)
	}
	if fault.Message != "" {
		return errors.New(fault.Message)
	}
	return errExploreCrash(window)
}

func processPeakRSS() (int64, string) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
		rss := int64(usage.Maxrss)
		if runtime.GOOS == "linux" {
			rss *= 1024
		}
		if rss > 0 {
			return rss, "rusage.maxrss"
		}
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return int64(mem.Sys), "runtime.MemStats.Sys (not process RSS)"
}

func measureExploreSpent(d db, report ExploreReport, replans, plannerCalls int) ExploreSpent {
	spent := report.Spent
	spent.Actions = report.Actions
	spent.States = report.States
	spent.Replans = replans
	spent.PlannerCalls = plannerCalls
	spent.Depth = report.Actions
	spent.PeakRSSBytes, spent.RSSKind = processPeakRSS()
	var evidence int64
	for _, step := range report.Steps {
		if step.RAW == "" {
			continue
		}
		if info, err := os.Stat(step.RAW); err == nil {
			evidence += info.Size()
		}
	}
	spent.EvidenceBytes = evidence
	spent.EvidenceKind = "sum of RAW file sizes"
	if info, err := os.Stat(d.file); err == nil {
		spent.DBBytes = info.Size()
	}
	return spent
}
