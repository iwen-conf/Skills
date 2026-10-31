package main

import (
	"errors"
	"fmt"
)

var refutationResults = map[string]bool{"supported": true, "refuted": true, "not_found": true}

// refuteRisk registers one bounded attempt to disprove a risk. The attempt is
// done elsewhere (AI or runner); the CLI binds its evidence by SHA-256 and
// never closes, accepts or promotes the risk because of it.
func (d db) refuteRisk(graph, risk, result, evidence string) (RefutationAttempt, error) {
	if !refutationResults[result] {
		return RefutationAttempt{}, errors.New("refutation result must be supported, refuted or not_found")
	}
	if evidence == "" {
		return RefutationAttempt{}, errors.New("a refutation needs its evidence file")
	}
	path, raw, err := loadEvidence(evidence)
	if err != nil {
		return RefutationAttempt{}, fmt.Errorf("refutation evidence: %w", err)
	}
	report, err := d.riskReport(graph)
	if err != nil {
		return RefutationAttempt{}, err
	}
	known := false
	for _, entry := range append(report.Risks, report.ReferenceRisks...) {
		known = known || entry.ID == risk
	}
	if !known {
		return RefutationAttempt{}, fmt.Errorf("risk %s is not in the risk report of %s", risk, graph)
	}
	attempt := RefutationAttempt{Result: result, Evidence: path, SHA256: digestBytes(raw)}
	return attempt, d.tx([]string{fmt.Sprintf("INSERT INTO risk_refutations(graph,risk,result,evidence,sha256) VALUES(%s,%s,%s,%s,%s);", quote(graph), quote(risk), quote(result), quote(path), quote(attempt.SHA256))})
}

// annotateRefutations shows the latest attempt per risk, re-verifying its
// evidence; a changed file is reported as evidence_changed, never as a result.
func (d db) annotateRefutations(graph string, entries []RiskEntry) error {
	rows, err := d.query("SELECT risk,result,evidence,sha256,created FROM risk_refutations WHERE graph="+quote(graph)+" ORDER BY seq", 5)
	if err != nil {
		return err
	}
	history := map[string][]RefutationAttempt{}
	for _, row := range rows {
		history[row[0]] = append(history[row[0]], RefutationAttempt{Result: row[1], Evidence: row[2], SHA256: row[3], Created: row[4]})
	}
	for index := range entries {
		entry := &entries[index]
		entry.Refutation = "not_attempted"
		attempts := history[entry.ID]
		if len(attempts) == 0 {
			continue
		}
		latest := attempts[len(attempts)-1]
		entry.RefutationHistory, entry.RefutationEvidence, entry.RefutationSHA256 = attempts, latest.Evidence, latest.SHA256
		entry.Refutation = latest.Result
		if _, raw, err := loadEvidence(latest.Evidence); err != nil || digestBytes(raw) != latest.SHA256 {
			entry.Refutation = "evidence_changed"
		}
	}
	return nil
}
