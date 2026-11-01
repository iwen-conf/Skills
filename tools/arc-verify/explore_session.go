package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

type exploreDrive struct {
	lease     ExploreLease
	state     *ExploreCheckpoint
	fault     ExploreFault
	candidate CandidateAction
	attemptID string
}

func errExploreCrash(window string) error {
	return fmt.Errorf("explore crash: %s", window)
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
			if !legalExploreActionTransition(actions[i].Status, record.Status) {
				return actions
			}
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
	if fault.Window == window {
		return errExploreCrash(window)
	}
	return nil
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
