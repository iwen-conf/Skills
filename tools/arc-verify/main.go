// arc-verify: short-lived Go CLI; local Turso file, no server or background worker.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

func emit(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if e := runContext(ctx, os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "arc-verify:", e)
		os.Exit(exitCode(e))
	}
}

// exitCode lets a caller tell why run-chain stopped without parsing output.
func exitCode(err error) int {
	var stop chainStop
	if errors.As(err, &stop) {
		return stop.code
	}
	return 1
}
func run(args []string) error {
	return runContext(context.Background(), args)
}

// commands is the complete CLI surface; unknown names fail before any ledger is opened.
var commands = []string{"init", "selftest", "validate", "import", "approve", "model", "view", "diff", "explore", "plan", "prepare", "next", "runner", "round", "execute", "run-chain", "derive", "discover", "reference-import", "reference-match", "mutation-report", "snapshot", "recover", "context", "start", "step", "finish", "record", "trace", "abort", "retry", "gate", "report", "risk", "accept-risk", "reject-risk", "promote-risk", "scan", "gaps", "fixtures", "fixture", "handoff"}

func runContext(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: arc-verify <" + strings.Join(commands, "|") + "> [options]")
	}
	name := args[0]
	subcommand := ""
	if name == "risk" && len(args) > 1 && args[1] == "refute" {
		subcommand, args = args[1], append([]string{name}, args[2:]...)
	}
	if name == "discover" {
		if len(args) < 2 || !slices.Contains([]string{"record", "list", "triage"}, args[1]) {
			return errors.New("usage: arc-verify discover <record|list|triage> [options]")
		}
		subcommand, args = args[1], append([]string{name}, args[2:]...)
	}
	if !slices.Contains(commands, name) {
		return fmt.Errorf("unknown command %q; usage: arc-verify <%s> [options]", name, strings.Join(commands, "|"))
	}
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	dbfile := f.String("db", ".arc/verify/data.db", "local Turso database path")
	file := f.String("file", "", "business graph JSON file")
	graph := f.String("graph", "", "graph id")
	rule := f.String("rule", "", "rule id for approval")
	to := f.String("to", "", "handoff recipient Arc skill")
	reason := f.String("reason", "", "required human-readable reason for abort/retry")
	condition := f.String("condition", "", "condition node id")
	maxDepth := f.Int("max-depth", 6, "recovery graph traversal depth, 1..8")
	limit := f.Int("limit", 128, "bounded number of task cards to materialize, 1..1024")
	task := f.String("task", "", "task id")
	runID := f.String("run", "", "run id")
	env := f.String("env", "", "approved environment alias")
	commit := f.String("commit", "", "source commit")
	root := f.String("root", ".", "candidate repository root")
	account := f.String("account", "", "registered account fixture id")
	fixture := f.String("fixture", "", "registered business fixture id")
	phase := f.String("phase", "action", "context phase: action or fixture")
	expected := f.String("expected", "", "expected graph state for this step (default final task state)")
	observed := f.String("observed", "", "observed business state")
	evidence := f.String("evidence", "", "local runner evidence path")
	id := f.String("id", "", "fixture id")
	config := f.String("config", ".arc/verify/config.jsonc", "test data and environment config")
	view := f.String("view", "capabilities", "business projection")
	round := f.String("round", "", "existing verification round")
	runner := f.String("runner", "", "registered execution adapter")
	assignment := f.String("assignment", "{}", "explicit observed-condition model assignment JSON")
	flow := f.String("flow", "", "declared behavior flow")
	maxStates := f.Int("max-states", 10000, "bounded exploration states")
	maxSteps := f.Int("max-steps", 64, "bounded exploration steps")
	maxPaths := f.Int("max-paths", 256, "bounded exploration paths")
	timeoutMS := f.Int("timeout-ms", 1000, "model exploration time budget")
	previous := f.String("previous", "", "previous model version")
	changed := f.String("changed", "", "comma-separated stable business subjects")
	role := f.String("role", "", "declared role filter")
	status := f.String("status", "", "verification status filter")
	readiness := f.String("readiness", "", "described, implemented, verified or live filter")
	offset := f.Int("offset", 0, "projection offset")
	participantAccounts := f.String("accounts", "{}", "declared participant alias to registered account JSON")
	assets := f.String("assets", "", "built static workbench directory")
	out := f.String("out", "", "snapshot output directory; must not exist or be empty")
	deadline := f.Duration("deadline", 0, "run-chain total deadline, 100ms..24h")
	budget := f.Int("budget", 16, "items per derivation (1..256) or exploration (1..1024)")
	refutation := f.String("result", "", "refutation result: supported, refuted or not_found")
	triage := f.String("as", "", "triage decision: missing_requirement, allowed_extension or should_not_exist")
	riskID := f.String("risk", "", "risk id")
	riskScope := f.String("scope", "", "explicit scope of an accepted risk")
	reportKind := f.String("kind", "", "report kind: acceptance or risk")
	riskReview := f.Bool("risk-review", false, "fail the gate while user-undecided risks remain")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if len(f.Args()) != 0 {
		return fmt.Errorf("unexpected arguments %v", f.Args())
	}
	if name == "selftest" {
		if e := selftestLocalTurso(); e != nil {
			return fmt.Errorf("local Turso self-test failed: %w", e)
		}
		emit(map[string]string{"status": "PASS", "database": "temporary local Turso", "network": "none"})
		return nil
	}
	if name == "validate" {
		if *file == "" {
			return errors.New("--file required")
		}
		g, e := loadGraph(*file)
		if e == nil {
			emit(map[string]any{"valid": true, "graph": g.ID, "nodes": len(g.Nodes), "edges": len(g.Edges)})
		}
		return e
	}
	if name == "explore" && *file != "" {
		g, err := loadGraph(*file)
		if err != nil {
			return err
		}
		budget := ExplorationBudget{MaxStates: *maxStates, MaxSteps: *maxSteps, MaxPaths: *maxPaths, TimeoutMS: *timeoutMS}
		if *flow != "" {
			result, err := exploreFlow(g, *flow, budget)
			if err == nil {
				emit(result)
			}
			return err
		}
		var values ConditionAssignment
		if err := decodeStrictJSON([]byte(*assignment), &values); err != nil {
			return err
		}
		result, err := solveConditions(g, values, nil, budget)
		if err == nil {
			emit(result)
		}
		return err
	}
	if name == "init" {
		d, e := openDB(*dbfile, true)
		if e != nil {
			return e
		}
		defer d.close()
		// DDL and indexes are committed together; a failed migration never
		// leaves a partially initialized graph ledger behind.
		if e = d.tx(schema); e != nil {
			return fmt.Errorf("initialize local Turso schema: %w", e)
		}
		emit(map[string]string{"database": d.file, "status": "initialized"})
		return nil
	}
	if (name == "approve" || name == "plan" || name == "prepare" || name == "next" || name == "gate" || name == "risk" || name == "gaps" || name == "recover") && *graph == "" {
		return errors.New("--graph required")
	}
	d, e := openDB(*dbfile, false)
	if e != nil {
		return e
	}
	defer d.close()
	switch name {
	case "run-chain":
		accounts := map[string]string{}
		if err := decodeStrictJSON([]byte(*participantAccounts), &accounts); err != nil {
			return err
		}
		packet, err := d.runChain(ctx, ChainRequest{Graph: *graph, Environment: *env, Round: *round, Commit: *commit, Root: *root, Config: *config, Account: *account, Fixture: *fixture, Accounts: accounts, MaxSteps: *maxSteps, Deadline: *deadline})
		var stop chainStop
		if err == nil || errors.As(err, &stop) {
			emit(packet)
		}
		return err
	case "derive":
		report, err := d.derive(*runID, *budget)
		if err == nil {
			emit(report)
		}
		return err
	case "discover":
		switch subcommand {
		case "record":
			if *file == "" {
				return errors.New("--file required")
			}
			body, err := os.ReadFile(*file)
			if err != nil {
				return err
			}
			var exploration Exploration
			if err := decodeStrictJSON(body, &exploration); err != nil {
				return err
			}
			report, err := d.recordExploration(exploration, *budget)
			if err == nil {
				emit(report)
			}
			return err
		case "list":
			items, err := d.unspecified(*graph)
			if err == nil {
				emit(items)
			}
			return err
		default:
			answers, err := interactiveAnswers(fmt.Sprintf("User triage of %s as %s. Type TRIAGE %s %s: ", *id, *triage, *id, *triage), "User's own words for this decision: ")
			if err != nil {
				return err
			}
			if answers[0] != "TRIAGE "+*id+" "+*triage {
				return errors.New("triage declined")
			}
			if err := d.triageUnspecified(*graph, *id, *triage, answers[1]); err != nil {
				return err
			}
			emit(map[string]string{"item": *id, "triage": *triage})
			return nil
		}
	case "reference-import":
		result, err := d.importReferences(*file)
		if err == nil {
			emit(result)
		}
		return err
	case "reference-match":
		report, err := d.matchReferences(*graph)
		if err == nil {
			emit(report)
		}
		return err
	case "mutation-report":
		report, err := d.mutationReport(*file)
		if err == nil {
			emit(report)
		}
		return err
	case "snapshot":
		path, err := d.writeSnapshot(SnapshotRequest{Graph: *graph, Environment: *env, Round: *round, Config: *config, Assets: *assets, Out: *out, Runs: splitIDs(*runID), Argv: append([]string{}, args...)})
		if err == nil {
			emit(map[string]string{"snapshot": path, "status": "exported"})
		}
		return err
	case "model":
		g, err := d.model(*graph)
		if err == nil {
			emit(g)
		}
		return err
	case "view":
		viewRoot := *root
		if viewRoot == "." {
			viewRoot = ""
		}
		result, err := d.viewModel(ViewQuery{Graph: *graph, View: BusinessView(*view), Root: viewRoot, Environment: *env, Round: *round, Role: *role, Status: *status, Readiness: *readiness, Offset: *offset, Limit: *limit})
		if err == nil {
			emit(result)
		}
		return err
	case "diff":
		result, err := d.diffModel(*graph, *previous, splitIDs(*changed))
		if err == nil {
			emit(result)
		}
		return err
	case "explore":
		g, err := d.model(*graph)
		if err != nil {
			return err
		}
		budget := ExplorationBudget{MaxStates: *maxStates, MaxSteps: *maxSteps, MaxPaths: *maxPaths, TimeoutMS: *timeoutMS}
		if *flow != "" {
			result, err := exploreFlow(g, *flow, budget)
			if err == nil {
				emit(result)
			}
			return err
		}
		var values ConditionAssignment
		if err := decodeStrictJSON([]byte(*assignment), &values); err != nil {
			return err
		}
		result, err := solveConditions(g, values, nil, budget)
		if err == nil {
			emit(result)
		}
		return err
	case "runner":
		if *file == "" {
			return errors.New("--file required")
		}
		_, raw, err := loadEvidence(*file)
		if err != nil {
			return err
		}
		var manifest RunnerManifest
		if err := decodeStrictJSON(raw, &manifest); err != nil {
			return err
		}
		result, err := d.registerRunner(manifest)
		if err == nil {
			emit(result)
		}
		return err
	case "round":
		result, err := d.createRound(*graph, *env, *commit, *root, *config)
		if err == nil {
			emit(result)
		}
		return err
	case "execute":
		var accounts map[string]string
		if err := decodeStrictJSON([]byte(*participantAccounts), &accounts); err != nil {
			return err
		}
		result, err := d.execute(ctx, ExecuteRequest{Task: *task, Runner: *runner, Round: *round, Environment: *env, Commit: *commit, Root: *root, Config: *config, Account: *account, Fixture: *fixture, Accounts: accounts})
		if err == nil {
			emit(result)
		}
		return err
	case "import":
		if *file == "" {
			return errors.New("--file required")
		}
		g, e := loadGraph(*file)
		if e != nil {
			return e
		}
		if e = d.importGraph(g); e == nil {
			emit(map[string]string{"graph": g.ID, "status": "candidate"})
		}
		return e
	case "approve":
		info, e := os.Stdin.Stat()
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeCharDevice == 0 {
			return errors.New("human approval needs an interactive terminal; a pipe is not allowed")
		}
		approved := *graph
		if *rule != "" {
			approved = *rule
		}
		fmt.Fprintf(os.Stderr, "User approval for %s. Type APPROVE %s: ", approved, approved)
		var word, value string
		_, e = fmt.Fscanln(os.Stdin, &word, &value)
		if e != nil || word != "APPROVE" || value != approved {
			return errors.New("approval declined")
		}
		if e = d.approve(*graph, *rule); e == nil {
			emit(map[string]string{"approved": approved})
		}
		return e
	case "plan":
		n, e := d.plan(*graph, *limit)
		if e == nil {
			emit(map[string]any{"graph": *graph, "task_candidates": n})
		}
		return e
	case "prepare":
		if *env == "" || *commit == "" {
			return errors.New("--env --commit required")
		}
		v, e := d.prepare(*graph, *env, *commit, *root, *config)
		if e == nil {
			emit(v)
		}
		return e
	case "context":
		if *runID == "" {
			return errors.New("--run required")
		}
		v, e := d.expectedContext(*runID, *phase, *root, *config, *commit)
		if e == nil {
			emit(v)
		}
		return e
	case "recover":
		if *condition == "" {
			return errors.New("--condition required")
		}
		v, e := d.recover(*graph, *condition, *maxDepth)
		if e == nil {
			emit(v)
		}
		return e
	case "next":
		rows, e := d.next(*graph)
		if e == nil {
			emit(rows)
		}
		return e
	case "start":
		if *task == "" || *env == "" || *commit == "" || *account == "" || *fixture == "" {
			return errors.New("--task --env --commit --account --fixture required")
		}
		binding, e := d.startBinding(*task, *env, *commit, *account, *fixture, *root, *config)
		if e != nil {
			return e
		}
		id, e := d.start(*task, binding)
		if e == nil {
			emit(map[string]any{"run": id, "status": "RUNNING", "identity": binding.Identity, "oracle": binding.Oracle})
		}
		return e
	case "step":
		if *runID == "" || *evidence == "" || *observed == "" {
			return errors.New("--run --observed --evidence required")
		}
		v, e := d.step(*runID, *expected, *observed, *evidence)
		if e == nil {
			emit(v)
		}
		return e
	case "finish":
		if *runID == "" {
			return errors.New("--run required")
		}
		v, e := d.finish(*runID)
		if e == nil {
			emit(v)
		}
		return e
	case "record":
		if *runID == "" || *evidence == "" || *observed == "" {
			return errors.New("--run --observed --evidence required")
		}
		rawPath, e := filepath.Abs(*evidence)
		if e != nil {
			return e
		}
		v, e := d.record(*runID, *observed, rawPath)
		if e == nil {
			emit(v)
		}
		return e
	case "abort":
		if *runID == "" || *reason == "" {
			return errors.New("--run --reason required")
		}
		v, e := d.abort(*runID, *reason)
		if e == nil {
			emit(v)
		}
		return e
	case "retry":
		if *task == "" || *reason == "" || *evidence == "" || *commit == "" {
			return errors.New("--task --reason --evidence --commit required")
		}
		v, e := d.retry(*task, *reason, *evidence, *root, *config, *commit)
		if e == nil {
			emit(v)
		}
		return e
	case "handoff":
		v, e := d.handoff(*to, *graph, *runID)
		if e == nil {
			emit(v)
		}
		return e
	case "trace":
		rows, e := d.trace(*runID)
		if e == nil {
			emit(rows)
		}
		return e
	case "gate":
		v, e := d.gate(*graph)
		if e != nil {
			return e
		}
		applyRiskReview(v, *riskReview)
		emit(v)
		if v["verdict"] != "PASS" {
			return errors.New("graph gate not satisfied")
		}
		return nil
	case "scan":
		if *file == "" {
			return errors.New("--file required")
		}
		b, e := os.ReadFile(*file)
		if e != nil {
			return e
		}
		var doc Implementation
		if e = decodeUniqueJSON(b, &doc); e != nil {
			return e
		}
		n, e := d.scan(doc)
		if e == nil {
			emit(map[string]any{"observations": n, "status": "OBSERVED_NOT_VERIFIED"})
		}
		return e
	case "gaps":
		rows, e := d.gaps(*graph)
		if e == nil {
			emit(rows)
		}
		return e
	case "fixtures":
		if *file == "" {
			return errors.New("--file required")
		}
		b, e := os.ReadFile(*file)
		if e != nil {
			return e
		}
		var fs Fixtures
		if e = decodeUniqueJSON(b, &fs); e != nil {
			return e
		}
		n, e := d.importFixtures(fs)
		if e == nil {
			emit(map[string]int{"fixtures_added": n})
		}
		return e
	case "fixture":
		if *id == "" {
			return errors.New("--id required")
		}
		rows, e := d.fixture(*id)
		if e == nil {
			emit(rows)
		}
		return e
	case "report":
		switch *reportKind {
		case "acceptance":
			v, e := d.acceptanceReport(*graph)
			if e == nil {
				emit(v)
			}
			return e
		case "risk":
			v, e := d.riskReport(*graph)
			if e == nil {
				emit(v)
			}
			return e
		default:
			return errors.New("--kind must be acceptance or risk")
		}
	case "accept-risk":
		answers, e := interactiveAnswers(fmt.Sprintf("User decision for risk %s within scope %q. Type ACCEPT %s: ", *riskID, *riskScope, *riskID), "User's own words for this decision: ")
		if e != nil {
			return e
		}
		if answers[0] != "ACCEPT "+*riskID {
			return errors.New("risk acceptance declined")
		}
		if e = d.acceptRisk(*graph, *riskID, *riskScope, answers[1]); e == nil {
			emit(map[string]string{"risk": *riskID, "status": "ACCEPTED_RISK"})
		}
		return e
	case "reject-risk":
		answers, e := interactiveAnswers(fmt.Sprintf("User decision: risk %s is not a problem. Type REJECT %s: ", *riskID, *riskID), "User's own words for this decision: ")
		if e != nil {
			return e
		}
		if answers[0] != "REJECT "+*riskID {
			return errors.New("risk rejection declined")
		}
		if e = d.rejectRisk(*graph, *riskID, answers[1]); e == nil {
			emit(map[string]string{"risk": *riskID, "status": "REJECTED"})
		}
		return e
	case "promote-risk":
		answers, e := interactiveAnswers(fmt.Sprintf("User decision: risk %s is covered by approved rule %s. Type PROMOTE %s: ", *riskID, *rule, *riskID))
		if e != nil {
			return e
		}
		if answers[0] != "PROMOTE "+*riskID {
			return errors.New("risk promotion declined")
		}
		if e = d.promoteRisk(*graph, *riskID, *rule); e == nil {
			emit(map[string]string{"risk": *riskID, "status": "PROMOTED", "rule": *rule})
		}
		return e
	case "risk":
		if subcommand == "refute" {
			attempt, err := d.refuteRisk(*graph, *riskID, *refutation, *evidence)
			if err == nil {
				emit(map[string]string{"risk": *riskID, "refutation": attempt.Result, "sha256": attempt.SHA256})
			}
			return err
		}
		rows, e := d.riskRows(*graph, 0)
		if e == nil {
			emit(rows)
		}
		return e
	default:
		return fmt.Errorf("unknown command %q", name)
	}
}

// JSONC comments removed with a finite-state scanner; URLs and escapes inside strings survive.
func cleanJSONC(s string) (string, error) {
	var b strings.Builder
	inString, escaped, line, block := false, false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		next := byte(0)
		if i+1 < len(s) {
			next = s[i+1]
		}
		if line {
			if c == '\n' {
				line = false
				b.WriteByte('\n')
			}
			continue
		}
		if block {
			if c == '*' && next == '/' {
				block = false
				i++
			} else if c == '\n' {
				b.WriteByte('\n')
			}
			continue
		}
		if inString {
			b.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && next == '/' {
			line = true
			i++
			continue
		}
		if c == '/' && next == '*' {
			block = true
			i++
			continue
		}
		b.WriteByte(c)
	}
	if block || inString {
		return "", errors.New("unterminated JSONC string or comment")
	}
	return b.String(), nil
}
func approvedEnvironment(path, name string) error {
	_, _, err := readApprovedEnvironment(path, name)
	return err
}

type ConfiguredEnvironment struct {
	Approved bool   `json:"approved"`
	URL      string `json:"url"`
}

type EnvironmentConfiguration struct {
	Environments map[string]ConfiguredEnvironment `json:"environments"`
}

func readEnvironmentConfiguration(path string) (EnvironmentConfiguration, []byte, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return EnvironmentConfiguration{}, nil, e
	}
	s, e := cleanJSONC(string(raw))
	if e != nil {
		return EnvironmentConfiguration{}, nil, e
	}
	var c EnvironmentConfiguration
	if e = decodeUniqueJSON([]byte(s), &c); e != nil {
		return c, nil, e
	}
	return c, raw, nil
}

func readApprovedEnvironment(path, name string) (string, string, error) {
	c, raw, e := readEnvironmentConfiguration(path)
	if e != nil {
		return "", "", e
	}
	target, ok := c.Environments[name]
	if !ok || !target.Approved || strings.TrimSpace(target.URL) == "" {
		return "", "", fmt.Errorf("environment %s not explicitly approved in %s", name, path)
	}
	parsed, e := url.Parse(target.URL)
	if e != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", errors.New("environment URL must be an HTTP(S) base without credentials, query or fragment")
	}
	return target.URL, digestBytes(raw), nil
}
