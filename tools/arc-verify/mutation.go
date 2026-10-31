package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

var mutationCategories = map[string]bool{"unauthorized_access": true, "race_oversell": true, "illegal_transition": true, "idempotency": true, "missing_server_validation": true, "other": true}

// MutationInput lists defects injected into a disposable copy and the runs
// executed against each injected build.
type MutationInput struct {
	Graph     string           `json:"graph"`
	Mutations []InjectedDefect `json:"mutations"`
}

type InjectedDefect struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	Location        string   `json:"location"`
	ExpectedFailing []string `json:"expected_failing"`
	Runs            []string `json:"runs"`
}

// MutationOutcome is one injected defect's result.
type MutationOutcome struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Location    string   `json:"location"`
	Outcome     string   `json:"outcome"`
	FailingRuns []string `json:"failing_runs"`
	Reason      string   `json:"reason"`
}

type MutationCategory struct {
	Detected     int `json:"detected"`
	Survived     int `json:"survived"`
	Inconclusive int `json:"inconclusive"`
}

// MutationReport lists detected and surviving injected defects; it sets no threshold.
type MutationReport struct {
	Graph              string                      `json:"graph"`
	Mutations          []MutationOutcome           `json:"mutations"`
	ByCategory         map[string]MutationCategory `json:"by_category"`
	Detected           int                         `json:"detected"`
	Survived           int                         `json:"survived"`
	Inconclusive       int                         `json:"inconclusive"`
	Survivors          []string                    `json:"survivors"`
	InsufficientSample bool                        `json:"insufficient_sample"`
}

// mutationReport judges each injected defect only from independent run
// verdicts: detected when an expected obligation FAILs with intact evidence,
// survived when every expected obligation PASSes, inconclusive otherwise.
func (d db) mutationReport(path string) (MutationReport, error) {
	report := MutationReport{Mutations: []MutationOutcome{}, ByCategory: map[string]MutationCategory{}, Survivors: []string{}}
	body, err := os.ReadFile(path)
	if err != nil {
		return report, err
	}
	var input MutationInput
	if err := decodeStrictJSON(body, &input); err != nil {
		return report, err
	}
	g, err := d.model(input.Graph)
	if err != nil {
		return report, err
	}
	report.Graph = g.ID
	if len(input.Mutations) == 0 || len(input.Mutations) > 1000 {
		return report, errors.New("injection list needs 1..1000 mutations")
	}
	obligations := map[string]bool{}
	for _, obligation := range modelObligations(g) {
		obligations[obligation.Definition.ID] = true
	}
	seen := map[string]bool{}
	for _, mutation := range input.Mutations {
		if mutation.ID == "" || seen[mutation.ID] || !mutationCategories[mutation.Category] || strings.TrimSpace(mutation.Location) == "" || len(mutation.ExpectedFailing) == 0 {
			return report, fmt.Errorf("mutation %q needs a unique id, a known category, a location and expected failing obligations", mutation.ID)
		}
		seen[mutation.ID] = true
		expected := map[string]bool{}
		for _, id := range mutation.ExpectedFailing {
			if !obligations[id] {
				return report, fmt.Errorf("mutation %s expects unknown obligation %s", mutation.ID, id)
			}
			expected[g.ID+"/"+id] = true
		}
		outcome := MutationOutcome{ID: mutation.ID, Category: mutation.Category, Location: mutation.Location, FailingRuns: []string{}}
		passed, failed, unclear := 0, 0, 0
		for _, run := range mutation.Runs {
			rows, err := d.query("SELECT r.task,r.status,t.graph FROM runs r JOIN tasks t ON t.id=r.task WHERE r.id="+quote(run), 3)
			if err != nil {
				return report, err
			}
			if len(rows) != 1 || rows[0][2] != g.ID {
				return report, fmt.Errorf("mutation %s run %s is not a run of %s", mutation.ID, run, g.ID)
			}
			if !expected[rows[0][0]] {
				continue
			}
			trace, err := d.trace(run)
			if err != nil {
				return report, err
			}
			switch {
			case rows[0][1] == "FAIL" && trace.EvidenceStatus == "PASS":
				failed++
				outcome.FailingRuns = append(outcome.FailingRuns, run)
			case rows[0][1] == "PASS" && d.qualifyModelRun(run) == nil:
				passed++
			default:
				unclear++
			}
		}
		category := report.ByCategory[mutation.Category]
		switch {
		case failed > 0:
			outcome.Outcome, outcome.Reason = "detected", fmt.Sprintf("%d expected obligation run(s) failed with intact evidence", failed)
			report.Detected++
			category.Detected++
		case passed > 0 && unclear == 0:
			outcome.Outcome, outcome.Reason = "survived", fmt.Sprintf("every expected obligation passed on the injected build (%d qualified run(s)); a new obligation is needed", passed)
			report.Survived++
			category.Survived++
			report.Survivors = append(report.Survivors, mutation.ID)
		default:
			outcome.Outcome, outcome.Reason = "inconclusive", fmt.Sprintf("no failing run and %d run(s) without a qualified verdict", unclear)
			report.Inconclusive++
			category.Inconclusive++
		}
		report.ByCategory[mutation.Category] = category
		report.Mutations = append(report.Mutations, outcome)
	}
	sort.Strings(report.Survivors)
	report.InsufficientSample = report.Detected+report.Survived < 10
	return report, nil
}
