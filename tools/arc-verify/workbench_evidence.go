package main

import (
	"errors"
	"fmt"
)

func (d db) workbenchEvidence(run, phase string, index int) (StepRAW, string, error) {
	if run == "" {
		return StepRAW{}, "", errors.New("evidence requires an existing run")
	}
	bindings, err := d.query("SELECT body FROM run_bindings WHERE run="+quote(run), 1)
	if err != nil {
		return StepRAW{}, "", err
	}
	if len(bindings) != 1 {
		return StepRAW{}, "", errors.New("run has no recorded execution binding")
	}
	var binding RunBinding
	if err := decodeStrictJSON([]byte(bindings[0][0]), &binding); err != nil {
		return StepRAW{}, "", err
	}
	var path, hash, expected, observed string
	switch phase {
	case "action":
		if index < 1 || index > 128 {
			return StepRAW{}, "", errors.New("action evidence requires a bounded positive step")
		}
		rows, err := d.query(fmt.Sprintf("SELECT evidence_path,evidence_sha256,expected,observed FROM steps WHERE run=%s AND idx=%d", quote(run), index), 4)
		if err != nil {
			return StepRAW{}, "", err
		}
		if len(rows) != 1 {
			return StepRAW{}, "", errors.New("recorded step not found")
		}
		path, hash, expected, observed = rows[0][0], rows[0][1], rows[0][2], rows[0][3]
	case "setup", "fixture":
		if index != 0 {
			return StepRAW{}, "", errors.New("setup and fixture evidence do not accept an action step")
		}
		record, err := d.modelRun(run)
		if err != nil {
			return StepRAW{}, "", err
		}
		if phase == "setup" {
			path, hash, expected, observed, index = record.SetupEvidence, record.SetupSHA256, "ISOLATED", "ISOLATED", -1
		} else {
			fixture, _, err := d.bindingFixture(binding.Identity.Fixture)
			if err != nil {
				return StepRAW{}, "", err
			}
			path, hash, expected, observed = record.Recovery.Evidence, record.Recovery.SHA256, fixture.State, record.Recovery.State
			if observed == "" {
				observed = "UNKNOWN"
			}
		}
	default:
		return StepRAW{}, "", errors.New("evidence phase must be action, setup or fixture")
	}
	if phase == "setup" {
		_, body, err := loadEvidence(path)
		if err != nil {
			return StepRAW{}, "", err
		}
		var captured StepRAW
		if err := decodeStrictJSON(body, &captured); err != nil {
			return StepRAW{}, "", err
		}
		observed = captured.Observed
	}
	raw, err := readRecordedRAW(path, hash, run, expected, observed, index)
	if err != nil {
		return StepRAW{}, "", err
	}
	if err := binding.matchesRAW(raw); err != nil {
		return StepRAW{}, "", err
	}
	if raw.Model != nil {
		record, err := d.modelRun(run)
		if err != nil {
			return StepRAW{}, "", err
		}
		if raw.Model.Round != record.Round || raw.Model.RunnerSHA256 != record.RunnerSHA256 || raw.Model.CodeSHA256 != record.CodeSHA256 {
			return StepRAW{}, "", errors.New("evidence execution metadata differs from its frozen receipt")
		}
		if err := d.modelCaptureExists(run, index, hash); err != nil {
			return StepRAW{}, "", err
		}
	}
	return raw, hash, nil
}
