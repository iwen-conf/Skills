package main

import (
	"encoding/json"
	"fmt"
)

func expectationBeforeBytes(entry BusinessExpectationObservation) ([]byte, string, error) {
	entry.After, entry.Comparison = nil, nil
	hash := ""
	if entry.Decision.Frozen != nil {
		hash = entry.Decision.Frozen.SHA256
	}
	body, err := json.Marshal(entry)
	return body, hash, err
}

func (d db) persistBusinessExpectationCaptures(graph, run string, seq int, entries []BusinessExpectationObservation) error {
	if len(entries) == 0 {
		return nil
	}
	statements := make([]string, 0, len(entries))
	for _, entry := range entries {
		body, hash, err := expectationBeforeBytes(entry)
		if err != nil {
			return err
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO business_expectation_captures(run,seq,action_index,actor,graph,branch,status,expectation_sha256,body_json) SELECT %s,%d,%d,%s,%s,%s,%s,%s,%s FROM runs WHERE id=%s AND status='RUNNING';", quote(run), seq, entry.ActionIndex, quote(entry.Actor), quote(graph), quote(entry.Decision.Branch), quote(entry.Decision.Status), quote(hash), quote(string(body)), quote(run)))
	}
	if err := d.transition(statements); err != nil {
		return err
	}
	return d.verifyBusinessExpectationCaptures(graph, run, seq, entries)
}

func (d db) verifyBusinessExpectationCaptures(graph, run string, seq int, entries []BusinessExpectationObservation) error {
	rows, err := d.query(fmt.Sprintf("SELECT action_index,actor,graph,branch,status,expectation_sha256,body_json FROM business_expectation_captures WHERE run=%s AND seq=%d ORDER BY action_index", quote(run), seq), 7)
	if err != nil {
		return err
	}
	if len(rows) != len(entries) {
		return fmt.Errorf("pre-action expectation receipts are missing or incomplete")
	}
	for i, entry := range entries {
		body, hash, err := expectationBeforeBytes(entry)
		if err != nil {
			return err
		}
		row := rows[i]
		if row[0] != fmt.Sprint(entry.ActionIndex) || row[1] != entry.Actor || row[2] != graph || row[3] != entry.Decision.Branch || row[4] != entry.Decision.Status || row[5] != hash || !sameJSONValue([]byte(row[6]), body) {
			return fmt.Errorf("pre-action expectation receipt differs from captured decision for actor %s action %d", entry.Actor, entry.ActionIndex)
		}
	}
	return nil
}
