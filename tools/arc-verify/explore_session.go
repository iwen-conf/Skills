package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

type exploreDrive struct {
	lease     ExploreLease
	state     *ExploreCheckpoint
	fault     ExploreFault
	candidate CandidateAction
}

func errExploreCrash(window string) error {
	return fmt.Errorf("explore crash: %s", window)
}

func upsertExploreAction(actions []ExploreActionRecord, record ExploreActionRecord) []ExploreActionRecord {
	key := record.Candidate.RequestID
	if key == "" {
		key = candidateKey(record.Candidate, record.Before.State)
	}
	for i := range actions {
		existing := actions[i].Candidate.RequestID
		if existing == "" {
			existing = candidateKey(actions[i].Candidate, actions[i].Before.State)
		}
		if existing == key {
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
	if drive.fault.Window == "db_write_fail" && (status == actionRecorded || status == actionObserved) {
		return errExploreCrash("db_write_fail")
	}
	record := ExploreActionRecord{
		Candidate:     drive.candidate,
		Status:        status,
		SideEffecting: sideEffecting(drive.candidate),
		Before:        before,
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

func measureExploreSpent(d db, report ExploreReport, replans, plannerCalls int) ExploreSpent {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	spent := report.Spent
	spent.Actions = report.Actions
	spent.States = report.States
	spent.Replans = replans
	spent.PlannerCalls = plannerCalls
	spent.Depth = report.Actions
	spent.PeakRSSBytes = int64(mem.Sys)
	if info, err := os.Stat(d.file); err == nil {
		spent.DBBytes = info.Size()
	}
	return spent
}
