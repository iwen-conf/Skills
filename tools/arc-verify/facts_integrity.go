package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type VerifiedRunFacts struct {
	Facts           []UnifiedStepFact
	Steps           []ExplainStep
	EvidenceStatus  string
	OfflineComplete bool
}

// Snapshot and explain share this checked projection. No browser should infer
// evidence validity from the presence of a JSON blob or a saved PASS string.
func (d db) verifiedRunFacts(run string) (VerifiedRunFacts, error) {
	out, err := d.explainFacts("", run, "", explainAllSteps)
	if err != nil {
		return VerifiedRunFacts{EvidenceStatus: "BLOCKED"}, err
	}
	facts, _ := out["facts"].([]UnifiedStepFact)
	steps, _ := out["steps"].([]ExplainStep)
	complete, _ := out["offline_db_complete"].(bool)
	status := "PASS"
	if len(facts) == 0 {
		status = "MISSING"
	}
	if len(facts) > 0 {
		stepsByIndex := map[int]bool{}
		for _, fact := range facts {
			if fact.SourceKind == "execute" || fact.SourceKind == "sequence" {
				if stepsByIndex[fact.Seq] {
					return VerifiedRunFacts{EvidenceStatus: "BLOCKED"}, fmt.Errorf("duplicate fact projection for recorded step %d", fact.Seq)
				}
				stepsByIndex[fact.Seq] = true
			}
		}
		original, err := d.query("SELECT CAST(idx AS TEXT) FROM steps WHERE run="+quote(run)+" ORDER BY idx", 1)
		if err != nil {
			return VerifiedRunFacts{EvidenceStatus: "BLOCKED"}, err
		}
		for _, row := range original {
			var index int
			if _, err := fmt.Sscan(row[0], &index); err != nil || !stepsByIndex[index] {
				return VerifiedRunFacts{EvidenceStatus: "BLOCKED"}, fmt.Errorf("facts omit original recorded step %s", row[0])
			}
		}
	}
	for _, fact := range facts {
		if fact.EvidenceStatus == "MISSING" {
			status = "MISSING"
		} else if fact.EvidenceStatus == "LEGACY" && status == "PASS" {
			status = "LEGACY"
		}
	}
	return VerifiedRunFacts{Facts: facts, Steps: steps, EvidenceStatus: status, OfflineComplete: complete}, nil
}

func sameFactSlice[T any](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		a, errA := json.Marshal(left[i])
		b, errB := json.Marshal(right[i])
		if errA != nil || errB != nil || !sameJSONValue(a, b) {
			return false
		}
	}
	return true
}

func validateFactColumns(fact UnifiedStepFact, row []string, seq int) error {
	if fact.SourceKind != row[0] || fact.Run != row[1] || fact.Seq != seq || fact.Graph != row[3] || fact.Task != row[4] || fact.Node != row[5] || fact.Transition != row[6] || fact.RuleID != row[7] || fact.Actor != row[8] || fact.Account != row[9] || fact.Principal != row[10] || fact.ExpectedState != row[11] || fact.ObservedState != row[12] || fact.Verdict != row[13] || fact.Decision != row[14] || fact.Reason != row[15] || fact.EvidencePath != row[18] || fact.EvidenceSHA256 != row[19] {
		return errors.New("facts JSON disagrees with its indexed columns")
	}
	return nil
}

func validateFactObservation(fact UnifiedStepFact, raw StepRAW) error {
	if raw.Run != fact.Run || raw.Step != fact.Seq || raw.Identity.Graph != fact.Graph || raw.Observed != fact.ObservedState {
		return errors.New("facts identity or observed state disagrees with RAW")
	}
	if fact.SourceKind != "explore" && (raw.Expected != fact.ExpectedState || raw.Identity.Task != fact.Task) {
		return errors.New("facts expected state or task disagrees with RAW")
	}
	if raw.Model == nil {
		if fact.EvidenceVersion != 0 || len(fact.Actions)+len(fact.Oracles)+len(fact.BusinessExpectations) > 0 {
			return errors.New("facts claim observations absent from legacy RAW")
		}
		return nil
	}
	model := raw.Model
	if model.EvidenceVersion != fact.EvidenceVersion || model.Node != fact.Node || model.Transition != fact.Transition || model.RecoveryMode != fact.RecoveryMode {
		return errors.New("facts model identity disagrees with RAW")
	}
	if !sameFactSlice(fact.Requests, model.Requests) {
		return errors.New("facts request projection disagrees with captured pre-action input")
	}
	if fact.SourceKind == "explore" {
		// Historic discover projections normalize the boundary and omit action
		// details. Validate the independent before/after pair without inventing
		// the newer execution identity that those captures never recorded.
		if len(fact.Oracles) != len(model.Oracles) {
			return errors.New("exploration facts omit independent observations")
		}
		for i, item := range fact.Oracles {
			original := model.Oracles[i]
			a, _ := json.Marshal([]IndependentFacts{item.Before, item.After})
			b, _ := json.Marshal([]IndependentFacts{original.Before, original.After})
			if item.ID != original.ID || item.Actor != original.Actor || !sameJSONValue(a, b) {
				return errors.New("exploration facts disagree with independent RAW")
			}
		}
		return nil
	}
	preparation, _ := json.Marshal(fact.Preparation)
	rawPreparation, _ := json.Marshal(model.Preparation)
	if !sameFactSlice(fact.Participants, model.Participants) || !sameFactSlice(fact.BeforeEntities, model.BeforeEntities) || !sameFactSlice(fact.BeforeSessions, model.BeforeSessions) || !sameFactSlice(fact.Entities, model.Entities) || !sameFactSlice(fact.Sessions, model.Sessions) || !sameFactSlice(fact.Extractions, model.Extractions) || !sameFactSlice(fact.Conditions, model.Conditions) || !sameFactSlice(fact.Waits, model.Waits) || !sameFactSlice(fact.Oracles, model.Oracles) || !sameFactSlice(fact.Actions, model.Actions) || !sameFactSlice(fact.BusinessExpectations, model.BusinessExpectations) || !sameJSONValue(preparation, rawPreparation) {
		return errors.New("facts projection disagrees with captured RAW observations")
	}
	if fact.DeniedObserved != nil && *fact.DeniedObserved != raw.Denied {
		return errors.New("facts denial differs from captured RAW")
	}
	return nil
}

func (d db) verifyExplainFact(fact *UnifiedStepFact, embedded string) error {
	if embedded == "" || fact.EvidenceSHA256 == "" {
		fact.EvidenceStatus, fact.EvidenceSource = "MISSING", "missing"
		return nil
	}
	if digestBytes([]byte(embedded)) != fact.EvidenceSHA256 {
		return fmt.Errorf("embedded DB RAW hash mismatch: expected %s, got %s (%d bytes)", fact.EvidenceSHA256, digestBytes([]byte(embedded)), len(embedded))
	}
	var raw StepRAW
	var source string
	exploration := fact.SourceKind == "explore"
	if fact.SourceKind == "recovery" {
		if record, err := d.modelRun(fact.Run); err == nil && record.Runner == "discover-run" {
			exploration = true
		}
	}
	if exploration {
		if err := decodeStrictJSON([]byte(embedded), &raw); err != nil {
			return fmt.Errorf("invalid exploration RAW: %w", err)
		}
		source = "db_embedded"
		if fact.EvidencePath != "" {
			if _, body, err := loadEvidence(fact.EvidencePath); err == nil {
				if digestBytes(body) != fact.EvidenceSHA256 {
					return errors.New("RAW changed after being recorded")
				}
				source = "disk_raw"
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if fact.SourceKind == "recovery" {
			if err := d.modelCaptureExists(fact.Run, fact.Seq, fact.EvidenceSHA256); err != nil {
				return err
			}
		}
	} else {
		var err error
		// This checks disk/DB bytes, the frozen execution binding and, for
		// model runs, the engine's capture receipt. Never trust this view's
		// redundant hash in place of the original steps/model_runs receipt.
		phase, index := "action", fact.Seq
		if fact.SourceKind == "setup" {
			phase, index = "setup", 0
		} else if fact.SourceKind == "recovery" {
			phase, index = "fixture", 0
		}
		var canonicalHash string
		raw, canonicalHash, err = d.workbenchEvidence(fact.Run, phase, index)
		if err != nil {
			return err
		}
		if canonicalHash != fact.EvidenceSHA256 {
			return errors.New("fact RAW hash differs from the original capture receipt")
		}
		_, source, err = d.readRecordedRAWWithFallback(fact.EvidencePath, canonicalHash, fact.Run, fact.ExpectedState, fact.ObservedState, fact.Seq)
		if err != nil {
			return err
		}
	}
	if err := validateFactObservation(*fact, raw); err != nil {
		return err
	}
	fact.EvidenceSource = source
	fact.EvidenceStatus = "LEGACY"
	if !exploration && raw.Model != nil && raw.Model.EvidenceVersion >= runtimeEvidenceVersion {
		fact.EvidenceStatus = "PASS"
		if fact.SourceKind == "recovery" && fact.Verdict == "PASS" {
			contract, err := d.runTaskContract(fact.Run)
			if err != nil {
				return err
			}
			if err := d.validateModelRecovery(contract); err != nil {
				return fmt.Errorf("fixture finalization proof: %w", err)
			}
		}
		if fact.SourceKind == "setup" && fact.Verdict == "PASS" && raw.Model.Preparation != nil {
			contract, err := d.runTaskContract(fact.Run)
			if err != nil {
				return err
			}
			runner, err := d.registeredRunner(contract.Receipt.Runner)
			if err != nil || runner.Manifest.Preparation == nil {
				return fmt.Errorf("setup preparation has no frozen manifest")
			}
			p := runner.Manifest.Preparation
			if err := ValidatePreparationExecution(*contract.Model, PreparationRequest{Goal: p.Goal, MaxSteps: p.MaxSteps, MaxStates: p.MaxStates}, *raw.Model.Preparation); err != nil {
				return fmt.Errorf("setup preparation proof: %w", err)
			}
		}
		if fact.SourceKind == "execute" || fact.SourceKind == "sequence" {
			contract, err := d.runTaskContract(fact.Run)
			if err != nil {
				return err
			}
			verdict, reason, assertions, err := d.evaluateModelObservation(contract, raw, fact.Seq, fact.EvidenceSHA256)
			if err != nil {
				return err
			}
			if verdict != fact.Verdict || reason != fact.Reason || !sameFactSlice(assertions, fact.Assertions) {
				return errors.New("recorded verdict or assertions differ from verified RAW evaluation")
			}
		}
	}
	return nil
}
