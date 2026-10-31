package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (d db) planModel(g Graph, limit int) (int, error) {
	x := indexModel(g)
	obligations := modelObligations(g)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	position := 0
	err = tx.QueryRowContext(ctx, "SELECT position FROM model_plan_cursor WHERE graph=?", g.ID).Scan(&position)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if position < 0 || position > len(obligations) {
		return 0, errors.New("invalid model planner cursor")
	}
	approvals := map[string]bool{}
	rows, err := tx.QueryContext(ctx, "SELECT id,approved FROM rules WHERE graph=?", g.ID)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id string
		var approved bool
		if err = rows.Scan(&id, &approved); err != nil {
			rows.Close()
			return 0, err
		}
		approvals[id] = approved
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	count := 0
	for position < len(obligations) && count < limit {
		obligation := obligations[position]
		position++
		if obligation.Excluded {
			continue
		}
		if obligation.Definition.PendingReason == "" && !rulesApproved(approvals, obligationRules(x, obligation)) {
			continue
		}
		status := "READY"
		if obligation.Definition.PendingReason != "" {
			status = "BLOCKED"
		}
		task := obligation.Task
		result, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO tasks(id,graph,rule,kind,condition,expected,status) VALUES(?,?,?,?,?,?,?)", task.ID, g.ID, task.Rule, task.Kind, task.Condition, task.Expected, status)
		if err != nil {
			return 0, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if changed == 0 {
			var stored taskSpec
			var graph string
			if err = tx.QueryRowContext(ctx, "SELECT id,graph,rule,kind,condition,expected FROM tasks WHERE id=?", task.ID).Scan(&stored.ID, &graph, &stored.Rule, &stored.Kind, &stored.Condition, &stored.Expected); err != nil {
				return 0, err
			}
			if stored != task || graph != g.ID {
				return 0, fmt.Errorf("task %s contradicts its complete obligation", task.ID)
			}
		}
		count++
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO model_plan_cursor(graph,position) VALUES(?,?) ON CONFLICT(graph) DO UPDATE SET position=excluded.position", g.ID, position); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}

func (d db) nextModel(g Graph) ([]TaskCard, error) {
	x := indexModel(g)
	byID := map[string]ModelObligation{}
	for _, obligation := range modelObligations(g) {
		byID[obligation.Task.ID] = obligation
	}
	rows, err := d.query("SELECT t.id,t.rule,t.kind,t.condition,t.expected FROM tasks t JOIN rules q ON q.graph=t.graph AND q.id=t.rule WHERE t.graph="+quote(g.ID)+" AND t.status='READY' AND q.approved=1 ORDER BY t.id LIMIT 8", 5)
	if err != nil {
		return nil, err
	}
	result := []TaskCard{}
	for _, row := range rows {
		obligation, found := byID[row[0]]
		if !found || obligation.Task != (taskSpec{ID: row[0], Rule: row[1], Kind: row[2], Condition: row[3], Expected: row[4]}) {
			return nil, fmt.Errorf("task %s is not a complete frozen obligation", row[0])
		}
		rule := x.rules[obligation.Task.Rule]
		values := map[string]string{}
		for id, value := range obligation.Definition.Preconditions {
			values[id] = "UNSATISFIED"
			if value {
				values[id] = "SATISFIED"
			}
		}
		invariants := []string{}
		for _, invariant := range x.invariants[rule.ID] {
			invariants = append(invariants, invariant.ID)
		}
		instruction := "Execute the registered adapter, observe the declared conditions and independent state, then verify fixture restoration."
		if obligation.Definition.Denied {
			instruction = "Observe the failed conditions, attempt the declared action, and independently verify rejection and the declared rejected state."
		}
		switch obligation.Definition.Kind {
		case testExistence:
			instruction = "Search the candidate for the rule's entry and record it with `arc-verify scan` including the candidate commit and an evidence file; NOT_FOUND stays a candidate gap until a widened search is confirmed by the user."
			invariants = nil
		case testSequence:
			instruction = "Register one runner batch per declared step in order, each observed by its step rule's independent oracle; execution stops at the first divergent step."
		case testConsistency:
			instruction = "Observe the action's result through every declared layer's independent oracle in the final step; a missing layer blocks and any disagreeing layer fails."
		}
		result = append(result, TaskCard{ID: obligation.Task.ID, Rule: rule.ID, Type: string(obligation.Definition.Kind), Actor: rule.Actor, Entry: rule.Entry, Action: rule.Action, Preconditions: values, Expected: obligation.Definition.Expected, Oracle: rule.Oracle, Instruction: instruction, Steps: obligation.Definition.Steps, Layers: obligation.Definition.Layers, DependsOn: obligation.Definition.DependsOn, Invariants: invariants})
	}
	return result, nil
}
