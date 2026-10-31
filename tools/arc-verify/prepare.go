package main

import (
	"errors"
	"fmt"
	"path/filepath"
)

type PrepareResult struct {
	Graph        string `json:"graph"`
	Verdict      string `json:"verdict"`
	Reason       string `json:"reason"`
	Reopened     int    `json:"reopened"`
	RetainedPass int    `json:"retained_pass"`
	Blocked      int    `json:"blocked"`
}

// Requalification changes task readiness, never historical attempt verdicts.
func (d db) prepare(graph, environment, commit, root, config string) (PrepareResult, error) {
	result := PrepareResult{Graph: graph, Verdict: "NOT_RUN", Reason: "task readiness prepared; no business action executed"}
	target, configHash, err := readApprovedEnvironment(config, environment)
	if err != nil {
		return result, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return result, err
	}
	config, err = filepath.Abs(config)
	if err != nil {
		return result, err
	}
	head, sourceHash, err := sourceFingerprint(root)
	if err != nil {
		return result, err
	}
	if head != commit {
		return result, errors.New("prepare --commit is not the current candidate HEAD")
	}
	graphHash, err := d.graphFingerprint(graph)
	if err != nil {
		return result, err
	}
	approved, err := d.query("SELECT approved FROM graphs WHERE id="+quote(graph), 1)
	if err != nil {
		return result, err
	}
	if len(approved) != 1 || approved[0][0] != "1" {
		return result, errors.New("prepare requires an approved graph")
	}
	rows, err := d.query("SELECT t.id,t.status,COALESCE(r.id,''),COALESCE(b.body,'') FROM tasks t LEFT JOIN runs r ON r.rowid=(SELECT MAX(rr.rowid) FROM runs rr WHERE rr.task=t.id) LEFT JOIN run_bindings b ON b.run=r.id WHERE t.graph="+quote(graph)+" ORDER BY t.id", 4)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		if row[1] == "RUNNING" {
			return result, errors.New("cannot prepare while a graph attempt is RUNNING")
		}
	}
	cache := map[string]string{"source:" + root: head + ":" + sourceHash, "target:" + config + ":" + environment: target, "config:" + config: configHash, "graph:" + graph: graphHash}
	if _, err = d.fixtureSnapshot(cache); err != nil {
		return result, err
	}
	statements := []string{}
	for _, row := range rows {
		switch row[1] {
		case "FAIL", "BLOCKED":
			result.Blocked++
		case "READY":
		case "PASS":
			var binding RunBinding
			current := decodeUniqueJSON([]byte(row[3]), &binding) == nil && binding.SourceRoot == root && binding.ConfigPath == config && binding.Identity.Environment == environment && binding.Identity.SourceCommit == head && binding.Identity.SourceSHA256 == sourceHash && binding.Identity.ConfigSHA256 == configHash && binding.Identity.Target == target && binding.Identity.GraphSHA256 == graphHash
			if current && d.validateBinding(binding, cache) == nil {
				result.RetainedPass++
				continue
			}
			statements = append(statements,
				fmt.Sprintf("UPDATE tasks SET status='READY' WHERE id=%s AND status='PASS' AND (SELECT id FROM runs WHERE task=%s ORDER BY rowid DESC LIMIT 1)=%s AND NOT EXISTS (SELECT 1 FROM tasks WHERE graph=%s AND status='RUNNING');", quote(row[0]), quote(row[0]), quote(row[2]), quote(graph)),
				fmt.Sprintf("INSERT INTO task_events(task,run,event,reason) VALUES(%s,%s,'PREPARE','candidate, environment or fixture changed');", quote(row[0]), quote(row[2])),
			)
			result.Reopened++
		default:
			return result, fmt.Errorf("unknown task status %s", row[1])
		}
	}
	if len(statements) != 0 {
		if err = d.transition(statements); err != nil {
			return result, err
		}
	}
	return result, nil
}
