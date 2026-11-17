package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const explainAllSteps = -999

// ExplainAssertion is a normalized assertion verdict row stored in step_assertions.
type ExplainAssertion struct {
	Index    int    `json:"idx"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	OracleID string `json:"oracle_id,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
	Operator string `json:"operator,omitempty"`
	Verdict  string `json:"verdict"`
	Reason   string `json:"reason,omitempty"`
}

// ExplainStep answers who ran what business node, under which preconditions and
// entities, what was expected vs observed, and where first divergence occurred.
type ExplainStep struct {
	SourceKind      string               `json:"source_kind"`
	Run             string               `json:"run"`
	Seq             int                  `json:"seq"`
	Graph           string               `json:"graph"`
	Task            string               `json:"task,omitempty"`
	Node            string               `json:"node,omitempty"`
	Transition      string               `json:"transition,omitempty"`
	RuleID          string               `json:"rule_id,omitempty"`
	Who             map[string]any       `json:"who"`
	BusinessObjects map[string]any       `json:"business_objects"`
	Sessions        []SessionBinding     `json:"sessions,omitempty"`
	Preconditions   []ConditionReading   `json:"preconditions,omitempty"`
	Oracles         []OracleObservation  `json:"oracles,omitempty"`
	Actions         []ActionObservation  `json:"actions,omitempty"`
	DeniedObserved  *bool                `json:"denied_observed,omitempty"`
	Waits           []WaitObservation    `json:"waits,omitempty"`
	ExpectedState   string               `json:"expected_state"`
	ObservedState   string               `json:"observed_state"`
	Verdict         string               `json:"verdict"`
	Decision        string               `json:"decision,omitempty"`
	Reason          string               `json:"reason,omitempty"`
	Assertions      []ExplainAssertion   `json:"assertions"`
	FirstDivergence bool                 `json:"first_divergence"`
	RelatedRun      string               `json:"related_run,omitempty"`
	EvidencePath    string               `json:"evidence_path,omitempty"`
	EvidenceSHA256  string               `json:"evidence_sha256,omitempty"`
	RawInDB         bool                 `json:"raw_in_db"`
	Created         string               `json:"created,omitempty"`
}

func parseExplainScalar(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var out any
	if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
		return out
	}
	return raw
}

func (d db) explainFacts(graph, run, node string, seq int) (map[string]any, error) {
	if run == "" && (graph == "" || node == "") {
		return nil, errors.New("explain requires --run or (--graph and --node)")
	}
	if err := d.ensureFactsSchema(); err != nil {
		return nil, err
	}
	clauses := []string{}
	if run != "" {
		clauses = append(clauses, "run="+quote(run))
	}
	if graph != "" {
		clauses = append(clauses, "graph="+quote(graph))
	}
	if node != "" {
		clauses = append(clauses, "(node="+quote(node)+" OR transition="+quote(node)+" OR rule_id="+quote(node)+")")
	}
	if seq != explainAllSteps {
		clauses = append(clauses, fmt.Sprintf("seq=%d", seq))
	}
	where := strings.Join(clauses, " AND ")
	rows, err := d.query(
		"SELECT "+fieldsSQL(
			"source_kind", "run", "CAST(seq AS TEXT)", "graph", "task", "node", "transition", "rule_id",
			"actor", "account", "principal", "expected_state", "observed_state", "verdict", "decision", "reason",
			"CAST(first_divergence AS TEXT)", "related_run", "evidence_path", "evidence_sha256",
			"CASE WHEN raw_json <> '' THEN '1' ELSE '0' END", "facts_json", "created",
		)+" FROM step_facts WHERE "+where+" ORDER BY run ASC, CASE source_kind WHEN 'setup' THEN 0 WHEN 'recovery' THEN 999999 ELSE 1 END ASC, seq ASC, source_kind ASC LIMIT 1000",
		23,
	)
	if err != nil {
		return nil, err
	}
	runIDs := map[string]bool{}
	for _, r := range rows {
		runIDs[r[1]] = true
	}
	if run != "" {
		runIDs[run] = true
	}
	sortedRuns := make([]string, 0, len(runIDs))
	for id := range runIDs {
		if id != "" {
			sortedRuns = append(sortedRuns, id)
		}
	}
	sort.Strings(sortedRuns)

	assertionsByStep := map[string][]ExplainAssertion{}
	entitiesByRun := map[string][]EntityBinding{}
	allEntities := []EntityBinding{}
	runSummaries := map[string]map[string]any{}

	if len(sortedRuns) > 0 {
		quotedRuns := make([]string, 0, len(sortedRuns))
		for _, id := range sortedRuns {
			quotedRuns = append(quotedRuns, quote(id))
		}
		inRuns := strings.Join(quotedRuns, ",")
		aRows, err := d.query(
			"SELECT "+fieldsSQL("source_kind", "run", "CAST(seq AS TEXT)", "CAST(assertion_idx AS TEXT)", "kind", "name", "oracle_id", "subject", "expected_json", "actual_json", "operator", "verdict", "reason")+
				" FROM step_assertions WHERE run IN ("+inRuns+") ORDER BY run,source_kind,seq,assertion_idx LIMIT 10000",
			13,
		)
		if err != nil {
			return nil, err
		}
		for _, a := range aRows {
			sNum, _ := strconv.Atoi(a[2])
			idxNum, _ := strconv.Atoi(a[3])
			key := fmt.Sprintf("%s|%s|%d", a[0], a[1], sNum)
			assertionsByStep[key] = append(assertionsByStep[key], ExplainAssertion{
				Index:    idxNum,
				Kind:     a[4],
				Name:     a[5],
				OracleID: a[6],
				Subject:  a[7],
				Expected: parseExplainScalar(a[8]),
				Actual:   parseExplainScalar(a[9]),
				Operator: a[10],
				Verdict:  a[11],
				Reason:   a[12],
			})
		}

		eRows, err := d.query(
			"SELECT "+fieldsSQL("run", "alias", "entity_kind", "entity_id", "owner_actor", "owner_principal", "channel_account_id", "CAST(source_step AS TEXT)", "source_field", "metadata_json")+
				" FROM run_entity_bindings WHERE run IN ("+inRuns+") ORDER BY run,source_step,alias LIMIT 1000",
			10,
		)
		if err != nil {
			return nil, err
		}
		for _, er := range eRows {
			srcStep, _ := strconv.Atoi(er[7])
			var meta map[string]string
			if er[9] != "" && er[9] != "{}" {
				_ = json.Unmarshal([]byte(er[9]), &meta)
			}
			ent := EntityBinding{
				Alias:            er[1],
				Kind:             er[2],
				ID:               er[3],
				OwnerActor:       er[4],
				OwnerPrincipal:   er[5],
				ChannelAccountID: er[6],
				SourceStep:       srcStep,
				SourceField:      er[8],
				Metadata:         meta,
			}
			entitiesByRun[er[0]] = append(entitiesByRun[er[0]], ent)
			allEntities = append(allEntities, ent)
		}

		rRows, err := d.query(
			"SELECT "+fieldsSQL("id", "task", "environment", "commit_sha", "status")+
				" FROM runs WHERE id IN ("+inRuns+") ORDER BY id LIMIT 1000",
			5,
		)
		if err != nil {
			return nil, err
		}
		for _, rr := range rRows {
			runSummaries[rr[0]] = map[string]any{
				"run":         rr[0],
				"task":        rr[1],
				"environment": rr[2],
				"commit_sha":  rr[3],
				"status":      rr[4],
				"entities":    entitiesByRun[rr[0]],
			}
		}
		bRows, err := d.query(
			"SELECT "+fieldsSQL("run", "body")+" FROM run_bindings WHERE run IN ("+inRuns+") LIMIT 1000",
			2,
		)
		if err != nil {
			return nil, err
		}
		for _, br := range bRows {
			var rb RunBinding
			if err := json.Unmarshal([]byte(br[1]), &rb); err == nil {
				summary := runSummaries[br[0]]
				if summary == nil {
					summary = map[string]any{"run": br[0]}
					runSummaries[br[0]] = summary
				}
				summary["graph"] = rb.Identity.Graph
				summary["target"] = rb.Identity.Target
				summary["oracle"] = rb.Oracle
				summary["actor"] = rb.Identity.Actor
				summary["account"] = rb.Identity.Account
				summary["principal"] = rb.Identity.Principal
				summary["fixture"] = rb.Identity.Fixture
				if rec, mErr := d.modelRun(br[0]); mErr == nil {
					summary["round"] = rec.Round
					summary["runner"] = rec.Runner
					summary["runner_sha256"] = rec.RunnerSHA256
					summary["participants"] = rec.Participants
				}
			}
		}
	}

	steps := make([]ExplainStep, 0, len(rows))
	offlineComplete := len(rows) > 0
	var firstDivStep *ExplainStep
	for _, r := range rows {
		sNum, _ := strconv.Atoi(r[2])
		rawInDB := r[20] == "1"
		if !rawInDB || r[19] == "" {
			offlineComplete = false
		}
		var fact UnifiedStepFact
		if r[21] != "" {
			_ = json.Unmarshal([]byte(r[21]), &fact)
		}
		key := fmt.Sprintf("%s|%s|%d", r[0], r[1], sNum)
		stepAssertions := assertionsByStep[key]
		if len(stepAssertions) == 0 && len(fact.Assertions) > 0 {
			for aIdx, a := range fact.Assertions {
				stepAssertions = append(stepAssertions, ExplainAssertion{
					Index:    aIdx + 1,
					Kind:     a.Kind,
					Name:     a.Name,
					OracleID: a.OracleID,
					Subject:  a.Subject,
					Expected: parseExplainScalar(a.Expected),
					Actual:   parseExplainScalar(a.Actual),
					Operator: a.Operator,
					Verdict:  a.Verdict,
					Reason:   a.Reason,
				})
			}
		}
		if stepAssertions == nil {
			stepAssertions = []ExplainAssertion{}
		}
		stepEntities := fact.Entities
		if len(stepEntities) == 0 {
			stepEntities = entitiesByRun[r[1]]
		}
		step := ExplainStep{
			SourceKind: r[0],
			Run:        r[1],
			Seq:        sNum,
			Graph:      r[3],
			Task:       r[4],
			Node:       r[5],
			Transition: r[6],
			RuleID:     r[7],
			Who: map[string]any{
				"actor":        r[8],
				"account":      r[9],
				"principal":    r[10],
				"participants": fact.Participants,
			},
			BusinessObjects: map[string]any{
				"entities":    stepEntities,
				"extractions": fact.Extractions,
			},
			Sessions:        fact.Sessions,
			Preconditions:   fact.Conditions,
			Oracles:         fact.Oracles,
			Actions:         fact.Actions,
			DeniedObserved:  fact.DeniedObserved,
			Waits:           fact.Waits,
			ExpectedState:   r[11],
			ObservedState:   r[12],
			Verdict:         r[13],
			Decision:        r[14],
			Reason:          r[15],
			Assertions:      stepAssertions,
			FirstDivergence: r[16] == "1",
			RelatedRun:      r[17],
			EvidencePath:    r[18],
			EvidenceSHA256:  r[19],
			RawInDB:         rawInDB,
			Created:         r[22],
		}
		steps = append(steps, step)
		if step.FirstDivergence && firstDivStep == nil {
			copyStep := step
			firstDivStep = &copyStep
		}
	}

	outRuns := make([]map[string]any, 0, len(sortedRuns))
	for _, id := range sortedRuns {
		if s := runSummaries[id]; s != nil {
			outRuns = append(outRuns, s)
		}
	}
	if graph == "" && len(steps) > 0 {
		graph = steps[0].Graph
	}
	return map[string]any{
		"graph":               graph,
		"run":                 run,
		"node":                node,
		"steps_count":         len(steps),
		"offline_db_complete": offlineComplete,
		"runs":                outRuns,
		"entities":            allEntities,
		"first_divergence":    firstDivStep,
		"steps":               steps,
	}, nil
}
