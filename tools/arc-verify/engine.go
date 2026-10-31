package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (d db) importGraph(g Graph) error {
	if e := validateGraph(g); e != nil {
		return e
	}
	rows, e := d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(g.ID)+";", 1)
	if e != nil {
		return e
	}
	if len(rows) > 0 {
		return errors.New("graph version already exists; publish a new graph ID")
	}
	if g.Previous != "" {
		previous, err := d.model(g.Previous)
		if err != nil {
			return fmt.Errorf("previous graph: %w", err)
		}
		if previous.SchemaVersion == 2 && previous.System != g.System {
			return errors.New("previous graph belongs to another system")
		}
	}
	stmts := []string{fmt.Sprintf("INSERT INTO graphs(id,scope) VALUES(%s,%s);", quote(g.ID), quote(g.Scope))}
	if g.SchemaVersion == 2 {
		body, err := json.Marshal(g)
		if err != nil {
			return err
		}
		stmts = append(stmts, fmt.Sprintf("INSERT INTO business_models(graph,schema_version,body) VALUES(%s,2,%s);", quote(g.ID), quote(string(body))))
		stmts = append(stmts, fmt.Sprintf("INSERT INTO tasks(id,graph,rule,kind,condition,expected,status) VALUES(%s,%s,'','MODEL_SCHEMA_GUARD','','','BLOCKED')", quote(g.ID+"/_schema/v2"), quote(g.ID)))
	}
	for _, n := range g.Nodes {
		stmts = append(stmts, fmt.Sprintf("INSERT INTO nodes VALUES(%s,%s,%s,%s);", quote(g.ID), quote(n.ID), quote(n.Kind), quote(n.Name)))
	}
	for _, e := range g.Edges {
		stmts = append(stmts, fmt.Sprintf("INSERT INTO edges VALUES(%s,%s,%s,%s,%s);", quote(g.ID), quote(e.ID), quote(e.From), quote(e.To), quote(e.Kind)))
	}
	for _, r := range g.Rules {
		cond, _ := json.Marshal(r.Conditions)
		stmts = append(stmts, fmt.Sprintf("INSERT INTO rules(graph,id,action,actor,entry,oracle,conditions,expected,rejected,source,description) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);", quote(g.ID), quote(r.ID), quote(r.Action), quote(r.Actor), quote(r.Entry), quote(r.Oracle), quote(string(cond)), quote(r.Expected), quote(r.Rejected), quote(r.Source), quote(r.Description)))
	}
	for _, r := range g.Risks {
		stmts = append(stmts, fmt.Sprintf("INSERT INTO risks(graph,id,action,description) VALUES(%s,%s,%s,%s);", quote(g.ID), quote(r.ID), quote(r.Action), quote(r.Description)))
		if r.Origin != "" {
			stmts = append(stmts, fmt.Sprintf("INSERT INTO risk_meta(graph,id,origin,reference) VALUES(%s,%s,%s,%s);", quote(g.ID), quote(r.ID), quote(r.Origin), quote(r.Reference)))
		}
	}
	return d.tx(stmts)
}
func (d db) approve(graph, rule string) error {
	if graph == "" {
		return errors.New("graph ID required")
	}
	exists, e := d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(graph)+";", 1)
	if e != nil {
		return e
	}
	if len(exists) != 1 {
		return errors.New("graph not found")
	}
	if rule == "" {
		return d.tx([]string{
			fmt.Sprintf("UPDATE graphs SET approved=1 WHERE id=%s;", quote(graph)),
			fmt.Sprintf("DELETE FROM plan_cursor WHERE graph=%s;", quote(graph)),
			fmt.Sprintf("DELETE FROM model_plan_cursor WHERE graph=%s;", quote(graph)),
		})
	}
	rows, e := d.query("SELECT "+fieldsSQL("source")+" FROM rules WHERE graph="+quote(graph)+" AND id="+quote(rule)+";", 1)
	if e != nil {
		return e
	}
	if len(rows) != 1 {
		return errors.New("rule not found")
	}
	return d.tx([]string{
		fmt.Sprintf("UPDATE rules SET approved=1 WHERE graph=%s AND id=%s;", quote(graph), quote(rule)),
		fmt.Sprintf("DELETE FROM plan_cursor WHERE graph=%s;", quote(graph)),
		fmt.Sprintf("DELETE FROM model_plan_cursor WHERE graph=%s;", quote(graph)),
	})
}

// plan persists an exact rule/condition cursor in the same transaction as
// each bounded task batch. Repeated planning is O(new obligations), not O(all
// rules × number of batches). The CLI process can exit after every call.
func (d db) plan(graph string, limits ...int) (int, error) {
	limit := 128
	if len(limits) > 0 {
		limit = limits[0]
	}
	if limit < 1 || limit > 1024 {
		return 0, errors.New("plan limit must be 1..1024")
	}
	approved, e := d.query("SELECT "+fieldsSQL("CAST(approved AS TEXT)")+" FROM graphs WHERE id="+quote(graph)+";", 1)
	if e != nil {
		return 0, e
	}
	if len(approved) != 1 || approved[0][0] != "1" {
		return 0, errors.New("business catalog requires explicit user approval")
	}
	model, e := d.extendedModel(graph)
	if e != nil {
		return 0, e
	}
	if model != nil {
		return d.planModel(*model, limit)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, e := d.handle.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	cursor := ""
	offset := 0
	err := tx.QueryRowContext(ctx,
		"SELECT rule_id,offset FROM plan_cursor WHERE graph=?", graph,
	).Scan(&cursor, &offset)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	rows, e := tx.QueryContext(ctx,
		"SELECT id,conditions,expected,rejected FROM rules "+
			"WHERE graph=? AND approved=1 AND id>=? ORDER BY id LIMIT ?",
		graph, cursor, limit+2)
	if e != nil {
		return 0, e
	}
	pending := make([]taskSpec, 0, limit)
	nextRule, nextOffset := cursor, offset
	for rows.Next() {
		var id, conditionsJSON, success, rejected string
		if e = rows.Scan(&id, &conditionsJSON, &success, &rejected); e != nil {
			rows.Close()
			return 0, e
		}
		var conditions []string
		if e = json.Unmarshal([]byte(conditionsJSON), &conditions); e != nil {
			rows.Close()
			return 0, e
		}
		if len(conditions) > 64 {
			rows.Close()
			return 0, fmt.Errorf("rule %s has too many conditions", id)
		}
		items := ruleTasks(graph, Rule{ID: id, Conditions: conditions, Expected: success, Rejected: rejected})
		start := 0
		if id == cursor {
			start = offset
		}
		if start < 0 || start > len(items) {
			rows.Close()
			return 0, errors.New("planner cursor is invalid; refusing to skip tasks")
		}
		for pos := start; pos < len(items) && len(pending) < limit; pos++ {
			pending = append(pending, items[pos])
			nextRule, nextOffset = id, pos+1
		}
		if len(pending) == limit {
			break
		}
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return 0, e
	}
	if e = rows.Close(); e != nil {
		return 0, e
	}
	if len(pending) == 0 {
		return 0, tx.Commit()
	}
	statement, e := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO tasks(id,graph,rule,kind,condition,expected) VALUES(?,?,?,?,?,?)")
	if e != nil {
		return 0, e
	}
	for _, task := range pending {
		result, err := statement.ExecContext(ctx, task.ID, graph, task.Rule, task.Kind, task.Condition, task.Expected)
		if err != nil {
			statement.Close()
			return 0, err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			statement.Close()
			return 0, err
		}
		if inserted == 0 {
			var existing taskSpec
			var existingGraph string
			err = tx.QueryRowContext(ctx, "SELECT id,graph,rule,kind,condition,expected FROM tasks WHERE id=?", task.ID).
				Scan(&existing.ID, &existingGraph, &existing.Rule, &existing.Kind, &existing.Condition, &existing.Expected)
			if err != nil {
				statement.Close()
				return 0, err
			}
			if existingGraph != graph || existing != task {
				statement.Close()
				return 0, fmt.Errorf("task %s conflicts with a stored obligation", task.ID)
			}
		}
	}
	if e = statement.Close(); e != nil {
		return 0, e
	}
	_, e = tx.ExecContext(ctx,
		"INSERT INTO plan_cursor(graph,rule_id,offset) VALUES(?,?,?) "+
			"ON CONFLICT(graph) DO UPDATE SET rule_id=excluded.rule_id,offset=excluded.offset",
		graph, nextRule, nextOffset)
	if e != nil {
		return 0, e
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return len(pending), nil
}

type taskSpec struct{ ID, Rule, Kind, Condition, Expected string }

func ruleTasks(graph string, rule Rule) []taskSpec {
	prefix := graph + "/" + rule.ID
	tasks := make([]taskSpec, 0, 1+len(rule.Conditions)*2)
	tasks = append(tasks, taskSpec{ID: prefix + "/POSITIVE", Rule: rule.ID, Kind: "POSITIVE", Expected: rule.Expected})
	for _, condition := range rule.Conditions {
		tasks = append(tasks, taskSpec{ID: prefix + "/NEGATIVE/" + condition, Rule: rule.ID, Kind: "NEGATIVE", Condition: condition, Expected: rule.Rejected})
		tasks = append(tasks, taskSpec{ID: prefix + "/FORCED/" + condition, Rule: rule.ID, Kind: "FORCED", Condition: condition, Expected: rule.Rejected})
	}
	return tasks
}

func validateTaskSpec(graph string, task taskSpec, rule Rule) error {
	for _, expected := range ruleTasks(graph, rule) {
		if task == expected {
			return nil
		}
	}
	return fmt.Errorf("task %s does not match its frozen rule", task.ID)
}

type runContract struct {
	Status, Graph string
	Task          taskSpec
	Rule          Rule
	Binding       RunBinding
	Model         *Graph
	Obligation    *ModelObligation
	Receipt       *ModelRunRecord
}

func (d db) runTaskContract(run string) (runContract, error) {
	rows, e := d.query("SELECT "+fieldsSQL("r.status", "t.graph", "t.id", "t.rule", "t.kind", "t.condition", "t.expected", "q.conditions", "q.expected", "q.rejected", "CAST(q.approved AS TEXT)", "CAST(g.approved AS TEXT)", "q.actor", "q.oracle", "COALESCE(b.body,'')")+" FROM runs r JOIN tasks t ON t.id=r.task JOIN rules q ON q.graph=t.graph AND q.id=t.rule JOIN graphs g ON g.id=t.graph LEFT JOIN run_bindings b ON b.run=r.id WHERE r.id="+quote(run)+";", 15)
	if e != nil {
		return runContract{}, e
	}
	if len(rows) != 1 {
		return runContract{}, errors.New("run or frozen rule not found")
	}
	row := rows[0]
	if row[10] != "1" || row[11] != "1" {
		return runContract{}, errors.New("run requires an approved catalog and rule")
	}
	contract := runContract{Status: row[0], Graph: row[1], Task: taskSpec{ID: row[2], Rule: row[3], Kind: row[4], Condition: row[5], Expected: row[6]}, Rule: Rule{ID: row[3], Expected: row[8], Rejected: row[9]}}
	if e = json.Unmarshal([]byte(row[7]), &contract.Rule.Conditions); e != nil {
		return runContract{}, e
	}
	if contract.Task.Kind != "MODEL" {
		if e = validateTaskSpec(contract.Graph, contract.Task, contract.Rule); e != nil {
			return runContract{}, e
		}
	} else {
		model, e := d.extendedModel(contract.Graph)
		if e != nil {
			return runContract{}, e
		}
		if model == nil {
			return runContract{}, errors.New("model task has no frozen model")
		}
		for _, item := range modelObligations(*model) {
			if item.Task == contract.Task && item.Definition.PendingReason == "" && !item.Excluded {
				copy := item
				contract.Obligation = &copy
				break
			}
		}
		if contract.Obligation == nil {
			return runContract{}, errors.New("run task is not an executable model obligation")
		}
		contract.Model = model
		contract.Rule = indexModel(*model).rules[contract.Task.Rule]
	}
	if e = decodeUniqueJSON([]byte(row[14]), &contract.Binding); e != nil {
		return runContract{}, fmt.Errorf("run is unbound: %w", e)
	}
	if contract.Binding.Identity.Task != contract.Task.ID || contract.Binding.Identity.Graph != contract.Graph || contract.Binding.Identity.Actor != row[12] || contract.Binding.Oracle != row[13] {
		return runContract{}, errors.New("execution binding does not match the frozen task")
	}
	if contract.Model != nil {
		record, e := d.modelRun(run)
		if e != nil {
			return runContract{}, e
		}
		if e = d.validateModelRecord(*contract.Model, *contract.Obligation, contract.Binding, record); e != nil {
			return runContract{}, e
		}
		contract.Receipt = &record
	}
	return contract, nil
}

type TaskCard struct {
	ID            string             `json:"id"`
	Rule          string             `json:"rule"`
	Type          string             `json:"type"`
	Actor         string             `json:"actor"`
	Entry         string             `json:"entry"`
	Action        string             `json:"action"`
	Preconditions map[string]string  `json:"preconditions"`
	Expected      string             `json:"expected"`
	Oracle        string             `json:"oracle"`
	Instruction   string             `json:"instruction"`
	Steps         []string           `json:"steps,omitempty"`
	Layers        []ObservationLayer `json:"layers,omitempty"`
	DependsOn     []string           `json:"depends_on,omitempty"`
	Invariants    []string           `json:"invariants,omitempty"`
}

func (d db) next(graph string) ([]TaskCard, error) {
	model, e := d.extendedModel(graph)
	if e != nil {
		return nil, e
	}
	if model != nil {
		return d.nextModel(*model)
	}
	rows, e := d.query("SELECT "+fieldsSQL("t.id", "t.rule", "t.kind", "t.condition", "t.expected", "r.action", "r.actor", "r.entry", "r.oracle", "r.conditions", "r.expected", "r.rejected")+" FROM tasks t JOIN rules r ON t.graph=r.graph AND t.rule=r.id WHERE t.graph="+quote(graph)+" AND t.status='READY' AND r.approved=1 ORDER BY t.id LIMIT 8;", 12)
	if e != nil {
		return nil, e
	}
	cards := make([]TaskCard, 0, len(rows))
	for _, r := range rows {
		var conditions []string
		if e = json.Unmarshal([]byte(r[9]), &conditions); e != nil {
			return nil, e
		}
		if e = validateTaskSpec(graph, taskSpec{ID: r[0], Rule: r[1], Kind: r[2], Condition: r[3], Expected: r[4]}, Rule{ID: r[1], Conditions: conditions, Expected: r[10], Rejected: r[11]}); e != nil {
			return nil, e
		}
		conditionsMap := map[string]string{}
		for _, c := range conditions {
			conditionsMap[c] = "SATISFIED"
			if (r[2] == "NEGATIVE" || r[2] == "FORCED") && c == r[3] {
				conditionsMap[c] = "UNSATISFIED"
			}
		}
		instruction := "From a real client, perform the action and verify independent readback."
		if r[2] == "NEGATIVE" {
			instruction = "Perform the normal UI operation with the failed condition, verify rejection and no state change."
		}
		if r[2] == "FORCED" {
			instruction = "In an authorized isolated environment, attempt the operation via the legitimate API with the failed condition; expect server-side rejection and no state change."
		}
		cards = append(cards, TaskCard{ID: r[0], Rule: r[1], Type: r[2], Actor: r[6], Entry: r[7], Action: r[5], Preconditions: conditionsMap, Expected: r[4], Oracle: r[8], Instruction: instruction})
	}
	return cards, nil
}
func newID(prefix string) string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return prefix + "-" + hex.EncodeToString(b)
}
func (d db) start(task string, binding RunBinding, receipts ...ModelRunRecord) (string, error) {
	rows, e := d.query("SELECT t.status,t.graph,q.actor,q.oracle,q.approved,g.approved,t.kind FROM tasks t JOIN rules q ON q.graph=t.graph AND q.id=t.rule JOIN graphs g ON g.id=t.graph WHERE t.id="+quote(task)+";", 7)
	if e != nil {
		return "", e
	}
	if len(rows) != 1 || rows[0][0] != "READY" {
		return "", errors.New("task not READY")
	}
	if binding.Identity.Task != task || binding.Identity.Graph != rows[0][1] || binding.Identity.Actor != rows[0][2] || binding.Oracle != rows[0][3] || rows[0][4] != "1" || rows[0][5] != "1" {
		return "", errors.New("start requires an approved task and its exact execution binding")
	}
	if e = d.validateBinding(binding, nil); e != nil {
		return "", e
	}
	if e = d.validateRetryCheckpoint(task, binding); e != nil {
		return "", e
	}
	body, e := json.Marshal(binding)
	if e != nil {
		return "", e
	}
	id := newID("R")
	var modelRecord *ModelRunRecord
	if rows[0][6] == "MODEL" {
		if len(receipts) != 1 {
			return "", errors.New("model tasks require the registered execute entry point and an explicit round")
		}
		g, e := d.model(rows[0][1])
		if e != nil {
			return "", e
		}
		var obligation *ModelObligation
		for _, item := range modelObligations(g) {
			if item.Task.ID == task && item.Definition.PendingReason == "" && !item.Excluded {
				copy := item
				obligation = &copy
				break
			}
		}
		if obligation == nil {
			return "", errors.New("model obligation is unbound")
		}
		record := receipts[0]
		record.Run = id
		if e = d.validateModelRecord(g, *obligation, binding, record); e != nil {
			return "", e
		}
		if e = d.validateModelDependencies(g, *obligation, record.Round); e != nil {
			return "", e
		}
		modelRecord = &record
	} else if len(receipts) != 0 {
		return "", errors.New("legacy tasks do not accept model execution context")
	}
	statements := []string{
		fmt.Sprintf("INSERT INTO runs(id,task,environment,commit_sha) SELECT %s,id,%s,%s FROM tasks WHERE id=%s AND status='READY';", quote(id), quote(binding.Identity.Environment), quote(binding.Identity.SourceCommit), quote(task)),
		fmt.Sprintf("UPDATE tasks SET status='RUNNING' WHERE id=%s AND status='READY';", quote(task)),
		fmt.Sprintf("INSERT INTO run_bindings(run,body) VALUES(%s,%s);", quote(id), quote(string(body))),
	}
	fixtures := bindingFixtureIDs(binding)
	if modelRecord != nil {
		body, e := json.Marshal(modelRecord)
		if e != nil {
			return "", e
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO model_runs(run,round,runner,body) VALUES(%s,%s,%s,%s)", quote(id), quote(modelRecord.Round), quote(modelRecord.Runner), quote(string(body))))
		for _, participant := range modelRecord.Participants {
			fixtures = append(fixtures, participant.Account)
		}
	}
	for _, fixture := range uniqueIDs(fixtures) {
		statements = append(statements, fmt.Sprintf("INSERT INTO fixture_locks(fixture,run) VALUES(%s,%s);", quote(fixture), quote(id)))
	}
	err := d.transition(statements)
	if err != nil {
		return "", err
	}
	check, err := d.query("SELECT "+fieldsSQL("id")+" FROM runs WHERE id="+quote(id)+";", 1)
	if err != nil {
		return "", err
	}
	if len(check) != 1 {
		return "", errors.New("task claimed by another process")
	}
	return id, nil
}
func (d db) record(run, observed, evidence string) (map[string]string, error) {
	detail, e := d.step(run, "", observed, evidence)
	if e != nil {
		return nil, e
	}
	if detail["verdict"] != "PASS" {
		return map[string]string{"run": run, "verdict": fmt.Sprint(detail["verdict"]), "decision": fmt.Sprint(detail["decision"])}, nil
	}
	finished, e := d.finish(run)
	if e != nil {
		return nil, e
	}
	return map[string]string{"run": run, "verdict": fmt.Sprint(finished["verdict"])}, nil
}

// gate scans the execution ledger once, in order, and re-reads each RAW
// exactly once. The previous implementation used 3+ SQL queries per
// PASS task (N+1), and loaded every step set repeatedly into memory.
func (d db) gate(graph string) (map[string]any, error) {
	model, err := d.extendedModel(graph)
	if err != nil {
		return nil, err
	}
	if model != nil {
		return d.modelGate(*model)
	}
	graphHash, e := d.graphFingerprint(graph)
	if e != nil {
		return nil, e
	}
	cache := map[string]string{"graph:" + graph: graphHash}
	fixtureHash, e := d.fixtureSnapshot(cache)
	if e != nil {
		return nil, e
	}
	contexts := map[string]RunBinding{}
	cohorts := map[string]bool{}
	versions, e := d.query("SELECT "+fieldsSQL("CAST(approved AS TEXT)")+" FROM graphs WHERE id="+quote(graph)+";", 1)
	if e != nil {
		return nil, e
	}
	if len(versions) != 1 {
		return nil, errors.New("graph not found")
	}
	rules, e := d.query("SELECT "+fieldsSQL("id", "conditions", "CAST(approved AS TEXT)", "expected", "rejected")+" FROM rules WHERE graph="+quote(graph)+";", 5)
	if e != nil {
		return nil, e
	}
	required := map[string]taskSpec{}
	unconfirmed := 0
	for _, r := range rules {
		if r[2] != "1" {
			unconfirmed++
			continue
		}
		var conditions []string
		if e = json.Unmarshal([]byte(r[1]), &conditions); e != nil {
			return nil, e
		}
		for _, task := range ruleTasks(graph, Rule{ID: r[0], Conditions: conditions, Expected: r[3], Rejected: r[4]}) {
			if _, exists := required[task.ID]; exists {
				return nil, fmt.Errorf("rules generate conflicting task identity %q", task.ID)
			}
			required[task.ID] = task
		}
	}
	// The task, latest attempt, rule and every step are fetched in one
	// stable ordered query. Evidence bytes remain on disk until checked.
	statement := "SELECT " + fieldsSQL(
		"t.id", "t.status", "t.kind", "t.condition", "t.expected",
		"COALESCE(q.conditions,'[]')", "COALESCE(r.id,'')", "COALESCE(r.status,'')",
		"COALESCE(CAST(s.idx AS TEXT),'')", "COALESCE(s.expected,'')",
		"COALESCE(s.observed,'')", "COALESCE(s.verdict,'')",
		"COALESCE(s.evidence_path,'')", "COALESCE(s.evidence_sha256,'')",
		"t.rule",
		"COALESCE(b.body,'')", "COALESCE(q.actor,'')", "COALESCE(q.oracle,'')",
		"COALESCE(r.environment,'')", "COALESCE(r.commit_sha,'')",
	) + " FROM tasks t LEFT JOIN rules q ON q.graph=t.graph AND q.id=t.rule " +
		"LEFT JOIN runs r ON r.rowid=(SELECT MAX(rr.rowid) FROM runs rr WHERE rr.task=t.id) " +
		"LEFT JOIN run_bindings b ON b.run=r.id " +
		"LEFT JOIN steps s ON s.run=r.id WHERE t.graph=" + quote(graph) +
		" ORDER BY t.id,s.idx;"
	expectedCount := len(required)
	total, passed, broken := 0, 0, 0
	current := ""
	currentPass := false
	currentExpected := ""
	stepCount := 0
	lastExpected, lastObserved := "", ""
	lastRAW := StepRAW{}
	var lastRequired []string
	var lastKind, lastFailed string
	currentBinding := RunBinding{}
	finishTask := func() {
		if current == "" {
			return
		}
		if stepCount == 0 || lastExpected != currentExpected || lastObserved != currentExpected {
			currentPass = false
		}
		if currentPass && (!conditionsMatch(lastRAW, lastKind, lastFailed, lastRequired) || !rejectionMatches(lastRAW, lastKind)) {
			currentPass = false
		}
		if currentPass {
			passed++
		} else {
			broken++
		}
	}
	e = d.each(statement, 20, func(row []string) error {
		id := row[0]
		if id != current {
			finishTask()
			current = id
			total++
			stepCount = 0
			lastExpected = ""
			lastObserved = ""
			lastRAW = StepRAW{}
			frozen, found := required[id]
			currentPass = found && frozen == (taskSpec{ID: id, Rule: row[14], Kind: row[2], Condition: row[3], Expected: row[4]}) && row[1] == "PASS" && row[7] == "PASS" && row[6] != ""
			currentBinding = RunBinding{}
			if err := decodeUniqueJSON([]byte(row[15]), &currentBinding); err != nil {
				currentPass = false
			}
			binding := currentBinding.Identity
			if binding.Task != id || binding.Graph != graph || binding.Actor != row[16] || currentBinding.Oracle != row[17] || binding.Environment != row[18] || binding.SourceCommit != row[19] {
				currentPass = false
			}
			_, accountKnown := cache["fixture:"+binding.Account]
			_, fixtureKnown := cache["fixture:"+binding.Fixture]
			if !accountKnown || !fixtureKnown {
				currentPass = false
			}
			if currentPass {
				if err := d.validateBinding(currentBinding, cache); err != nil {
					currentPass = false
				} else {
					key := strings.Join([]string{binding.SourceCommit, binding.SourceSHA256, binding.ConfigSHA256, binding.Environment, binding.Target, binding.GraphSHA256, currentBinding.SourceRoot, currentBinding.ConfigPath}, "\x00")
					cohorts[key] = true
					contextKey := strings.Join([]string{key, currentBinding.SourceRoot, currentBinding.ConfigPath, binding.Account, binding.Fixture}, "\x00")
					contexts[contextKey] = currentBinding
				}
			}
			lastKind, lastFailed, currentExpected = row[2], row[3], row[4]
			delete(required, id)
			if e = json.Unmarshal([]byte(row[5]), &lastRequired); e != nil {
				return e
			}
		}
		if row[8] == "" {
			return nil
		}
		stepCount++
		expectedIndex := fmt.Sprint(stepCount)
		if row[8] != expectedIndex || row[11] != "PASS" || row[9] != row[10] {
			currentPass = false
		}
		lastExpected, lastObserved = row[9], row[10]
		if !currentPass {
			return nil
		}
		// A mutable RAW cannot be trusted from a stored verdict. Rehash it.
		parsed, e := readRecordedRAW(row[12], row[13], row[6], row[9], row[10], stepCount)
		if e != nil || parsed.ExitCode != 0 {
			currentPass = false
			return nil
		}
		if e = currentBinding.matchesRAW(parsed); e != nil {
			currentPass = false
			return nil
		}
		if parsed.Expected == "UNCHANGED" && !rejectionMatches(parsed, "NEGATIVE") {
			currentPass = false
			return nil
		}
		lastRAW = parsed
		return nil
	})
	if e != nil {
		return nil, e
	}
	finishTask()
	openRisks, e := d.openRisks(graph)
	if e != nil {
		return nil, e
	}
	missing := len(required)
	fresh := map[string]string{}
	latestGraphHash, e := d.graphFingerprint(graph)
	if e != nil {
		return nil, e
	}
	fresh["graph:"+graph] = latestGraphHash
	latestFixtureHash, fixtureErr := d.fixtureSnapshot(fresh)
	inputsUnchanged := fixtureErr == nil && fixtureHash == latestFixtureHash
	if inputsUnchanged {
		for _, binding := range contexts {
			if e = d.validateBinding(binding, fresh); e != nil {
				inputsUnchanged = false
			}
		}
	}
	ok := versions[0][0] == "1" && len(rules) > 0 && unconfirmed == 0 &&
		total > 0 && total == passed && missing == 0 && broken == 0 && len(cohorts) == 1 && inputsUnchanged
	verdict := "FAIL"
	context := ProofContext{}
	if ok {
		verdict = "PASS"
		for _, binding := range contexts {
			v := binding.Identity
			context = ProofContext{Graph: graph, Environment: v.Environment, Target: v.Target, SourceCommit: v.SourceCommit, SourceSHA256: v.SourceSHA256, ConfigSHA256: v.ConfigSHA256, GraphSHA256: v.GraphSHA256, SourceRoot: binding.SourceRoot, ConfigPath: binding.ConfigPath}
			break
		}
	}
	return map[string]any{
		"verdict": verdict, "catalog_approved": versions[0][0] == "1",
		"confirmed_rules": len(rules) - unconfirmed, "unconfirmed_rules": unconfirmed,
		"required_tasks": expectedCount, "actual_tasks": total,
		"passed_with_valid_raw": passed, "missing_tasks": missing,
		"invalid_tasks_or_evidence": broken, "open_risks": openRisks,
		"proof_scope": "planned_rule_tasks", "execution_contexts": len(cohorts), "inputs_unchanged": inputsUnchanged,
		"context": context,
	}, nil
}

type StepTrace struct {
	Index          string            `json:"step"`
	Expected       string            `json:"expected"`
	Observed       string            `json:"observed"`
	Verdict        string            `json:"verdict"`
	Decision       string            `json:"decision"`
	Evidence       string            `json:"raw"`
	SHA256         string            `json:"sha256"`
	Node           string            `json:"node,omitempty"`
	Transition     string            `json:"transition,omitempty"`
	Observation    *ModelObservation `json:"observation,omitempty"`
	EvidenceStatus string            `json:"evidence_status,omitempty"`
}
type TraceReport struct {
	Run              string           `json:"run"`
	Task             string           `json:"task"`
	Status           string           `json:"status"`
	FirstDivergence  string           `json:"first_divergence,omitempty"`
	LatestObserved   string           `json:"latest_observed,omitempty"`
	ContinueDecision string           `json:"continue_decision"`
	Steps            []StepTrace      `json:"steps"`
	Round            string           `json:"round,omitempty"`
	Recovery         *RecoveryOutcome `json:"recovery,omitempty"`
	EvidenceStatus   string           `json:"evidence_status,omitempty"`
}

func (d db) trace(run string) (TraceReport, error) {
	rows, e := d.query("SELECT "+fieldsSQL("r.id", "r.task", "r.status", "COALESCE(CAST(s.idx AS TEXT),'')", "COALESCE(s.expected,'')", "COALESCE(s.observed,'')", "COALESCE(s.verdict,'')", "COALESCE(s.decision,'')", "COALESCE(s.evidence_path,'')", "COALESCE(s.evidence_sha256,'')")+" FROM runs r LEFT JOIN steps s ON r.id=s.run WHERE r.id="+quote(run)+" ORDER BY s.idx;", 10)
	if e != nil {
		return TraceReport{}, e
	}
	if len(rows) == 0 {
		return TraceReport{}, errors.New("run not found")
	}
	out := TraceReport{Run: run, Task: rows[0][1], Status: rows[0][2], ContinueDecision: "NONE", Steps: []StepTrace{}}
	for _, r := range rows {
		if r[3] == "" {
			continue
		}
		step := StepTrace{Index: r[3], Expected: r[4], Observed: r[5], Verdict: r[6], Decision: r[7], Evidence: r[8], SHA256: r[9], EvidenceStatus: "NOT_RUN"}
		var index int
		if _, err := fmt.Sscan(r[3], &index); err != nil {
			return out, err
		}
		if raw, err := readRecordedRAW(r[8], r[9], run, r[4], r[5], index); err == nil {
			step.EvidenceStatus = "PASS"
			step.Observation = raw.Model
			if raw.Model != nil {
				step.Node, step.Transition = raw.Model.Node, raw.Model.Transition
			}
		} else {
			step.EvidenceStatus = "STALE"
		}
		out.Steps = append(out.Steps, step)
		out.LatestObserved = r[5]
		if r[6] == "FAIL" && out.FirstDivergence == "" {
			out.FirstDivergence = r[3]
			out.ContinueDecision = r[7]
		}
	}
	if out.Status == "RUNNING" {
		out.ContinueDecision = "CONTINUE"
	}
	if out.Status == "BLOCKED" {
		out.ContinueDecision = "RESET"
	}
	modelRows, e := d.query("SELECT run FROM model_runs WHERE run="+quote(run), 1)
	if e != nil {
		return out, e
	}
	if len(modelRows) == 1 {
		record, err := d.modelRun(run)
		if err != nil {
			return out, err
		}
		out.Round = record.Round
		out.Recovery = &record.Recovery
		out.EvidenceStatus = "PASS"
		round, err := d.verificationRound(record.Round)
		if err != nil || d.validateRound(round) != nil {
			out.EvidenceStatus = "STALE"
		}
		for _, step := range out.Steps {
			if step.EvidenceStatus != "PASS" {
				out.EvidenceStatus = "STALE"
			}
		}
	}
	return out, nil
}
