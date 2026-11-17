package main

import (
	"errors"
	"fmt"
)

// TaskProof is a read-only receipt for one existing linked obligation. It never
// executes tests and never upgrades a partial graph into whole-system proof.
// ReceiptSHA256 hashes the JSON representation with that field empty.
type TaskProof struct {
	Version         int          `json:"version"`
	Graph           string       `json:"graph"`
	Task            string       `json:"task"`
	ObligationID    string       `json:"obligation_id"`
	Run             string       `json:"run"`
	Round           string       `json:"round"`
	Verdict         string       `json:"verdict"`
	Reason          string       `json:"reason"`
	ProofScope      string       `json:"proof_scope"`
	InputsUnchanged bool         `json:"inputs_unchanged"`
	EvidenceStatus  string       `json:"evidence_status"`
	Context         ProofContext `json:"context"`
	RAWSHA256       []string     `json:"raw_sha256"`
	ReceiptSHA256   string       `json:"receipt_sha256"`
}

func (d db) taskProof(graph, task string) (TaskProof, error) {
	result := TaskProof{Version: 1, Graph: graph, Task: task, Verdict: "BLOCKED", ProofScope: "linked_task_evidence", EvidenceStatus: "BLOCKED", RAWSHA256: []string{}}
	blocked := func(err error) (TaskProof, error) { result.Reason = err.Error(); return result, err }
	if graph == "" || task == "" {
		return blocked(errors.New("task-proof requires --graph and --task"))
	}
	g, err := d.model(graph)
	if err != nil {
		return blocked(err)
	}
	if g.SchemaVersion != 2 {
		return blocked(errors.New("native task completion requires a versioned model and current verified runtime facts"))
	}
	rows, err := d.query("SELECT r.id,r.status,r.environment,COALESCE(m.round,'') FROM runs r JOIN tasks t ON t.id=r.task LEFT JOIN model_runs m ON m.run=r.id WHERE t.graph="+quote(graph)+" AND t.id="+quote(task)+" ORDER BY r.rowid DESC LIMIT 1", 4)
	if err != nil {
		return blocked(err)
	}
	if len(rows) != 1 {
		return blocked(errors.New("linked task has no executed run"))
	}
	result.Run, result.Round = rows[0][0], rows[0][3]
	if rows[0][1] != "PASS" || result.Round == "" {
		return blocked(errors.New("latest linked task run has no completed PASS in a verification round"))
	}
	analysis, err := d.analyzeModel(g, rows[0][2], result.Round)
	if err != nil {
		return blocked(err)
	}
	result.Context = analysis.Context
	matched := false
	for _, obligation := range analysis.Obligations {
		if obligation.Task.ID != task {
			continue
		}
		matched = true
		result.ObligationID = obligation.Definition.ID
		if obligation.Excluded || obligation.Status != "PASS" || obligation.Run != result.Run {
			return blocked(fmt.Errorf("linked obligation is %s: %s", obligation.Status, obligation.Reason))
		}
	}
	if !matched {
		return blocked(errors.New("task does not identify a current model obligation"))
	}
	verified, err := d.verifiedRunFacts(result.Run)
	if err != nil {
		return blocked(err)
	}
	if verified.EvidenceStatus != "PASS" || !verified.OfflineComplete {
		return blocked(errors.New("linked run lacks complete validated runtime evidence; historical legacy status is not a new completion receipt"))
	}
	for _, fact := range verified.Facts {
		if len(fact.EvidenceSHA256) != 64 {
			return blocked(errors.New("linked step lacks a SHA-256 evidence binding"))
		}
		result.RAWSHA256 = append(result.RAWSHA256, fact.EvidenceSHA256)
	}
	result.RAWSHA256 = uniqueIDs(result.RAWSHA256)
	if len(result.RAWSHA256) == 0 {
		return blocked(errors.New("linked run has no validated raw evidence"))
	}
	result.Verdict, result.EvidenceStatus, result.InputsUnchanged = "PASS", "PASS", true
	result.Reason = "latest linked obligation, frozen context and independent runtime facts revalidated; no tests rerun"
	result.ReceiptSHA256, err = businessJSONHash(result)
	if err != nil {
		return blocked(err)
	}
	return result, nil
}
