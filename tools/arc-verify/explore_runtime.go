package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ExploreSource string

const (
	sourceRule  ExploreSource = "confirmed_rule"
	sourceImpl  ExploreSource = "implementation"
	sourceInfer ExploreSource = "inference"
)

type ExploreKind string

const (
	kindDeclared   ExploreKind = "declared_transition"
	kindSkipPre    ExploreKind = "skip_precondition"
	kindRole       ExploreKind = "switch_role"
	kindRepeat     ExploreKind = "repeat"
	kindReorder    ExploreKind = "reorder"
	kindHistory    ExploreKind = "history_variant"
	kindTimeout    ExploreKind = "timeout_retry"
	kindConcurrent ExploreKind = "concurrent"
	kindRecover    ExploreKind = "recover_continue"
	kindUndeclared ExploreKind = "undeclared_entry"
)

type ExploreEntry struct {
	Action string          `json:"action"`
	Entry  string          `json:"entry"`
	Method string          `json:"method"`
	Roles  []string        `json:"roles"`
	Body   json.RawMessage `json:"body,omitempty"`
}

type ExploreAdapter struct {
	SchemaVersion  int               `json:"schema_version"`
	Graph          string            `json:"graph"`
	IsolationScope string            `json:"isolation_scope"`
	TimeoutMS      int               `json:"timeout_ms"`
	StepTimeoutMS  int               `json:"step_timeout_ms"`
	Parallelism    int               `json:"parallelism"`
	Oracle         OracleAdapter     `json:"oracle"`
	Recovery       *FixtureRecovery  `json:"recovery,omitempty"`
	Entries        []ExploreEntry    `json:"entries"`
	Accounts       map[string]string `json:"accounts"`
}

type ExploreRuntimeBudget struct {
	MaxStates  int           `json:"max_states"`
	MaxPaths   int           `json:"max_paths"`
	MaxActions int           `json:"max_actions"`
	Deadline   time.Duration `json:"deadline"`
	MaxReplans int           `json:"max_replans"`
	Seed       int           `json:"seed,omitempty"`
}

type ExploreRequest struct {
	Graph, Flow, Environment, Round, Root, Config, Commit string
	Account, Fixture                                      string
	Accounts                                              map[string]string
	Adapter                                               ExploreAdapter
	Budget                                                ExploreRuntimeBudget
	Campaign                                              string
	Mode                                                  string
	Planner                                               ExplorePlanner
}

type CandidateAction struct {
	ID            string          `json:"id"`
	Kind          ExploreKind     `json:"kind"`
	Source        ExploreSource   `json:"source"`
	Action        string          `json:"action"`
	Entry         string          `json:"entry"`
	Role          string          `json:"role"`
	Account       string          `json:"account"`
	Transition    string          `json:"transition,omitempty"`
	Preconditions map[string]bool `json:"preconditions,omitempty"`
	Scope         string          `json:"scope"`
	Risk          string          `json:"risk"`
	Cost          int             `json:"cost"`
	RequestID     string          `json:"request_id,omitempty"`
	PartnerRole   string          `json:"partner_role,omitempty"`
}

type ExploreFinding struct {
	Kind     string `json:"kind"`
	Rule     string `json:"rule,omitempty"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence,omitempty"`
}

type ExploreCallRecord struct {
	Role           string `json:"role"`
	Account        string `json:"account,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	Denied         bool   `json:"denied"`
	StatusCode     int    `json:"status_code,omitempty"`
	Error          string `json:"error,omitempty"`
	Started        string `json:"started,omitempty"`
	Completed      string `json:"completed,omitempty"`
	RequestSHA256  string `json:"request_sha256,omitempty"`
	ResponseSHA256 string `json:"response_sha256,omitempty"`
}

type ExploreStep struct {
	Candidate     CandidateAction     `json:"candidate"`
	Before        IndependentFacts    `json:"before"`
	After         IndependentFacts    `json:"after"`
	Denied        bool                `json:"denied"`
	RAW           string              `json:"raw"`
	SHA256        string              `json:"sha256"`
	Findings      []ExploreFinding    `json:"findings"`
	Discovery     *DiscoveryReport    `json:"discovery,omitempty"`
	Calls         []ExploreCallRecord `json:"calls,omitempty"`
	ClassifyError string              `json:"classify_error,omitempty"`
}

type ExploreReport struct {
	Campaign       string            `json:"campaign"`
	Graph          string            `json:"graph"`
	Flow           string            `json:"flow"`
	Environment    string            `json:"environment,omitempty"`
	Round          string            `json:"round,omitempty"`
	StopReason     string            `json:"stop_reason"`
	Remaining      []CandidateAction `json:"remaining"`
	Steps          []ExploreStep     `json:"steps"`
	WithinContract int               `json:"within_contract"`
	Unspecified    []UnspecifiedItem `json:"unspecified"`
	RuleFails      []ExploreFinding  `json:"rule_fails"`
	Risks          []ExploreFinding  `json:"risks"`
	Blocked        []ExploreFinding  `json:"blocked,omitempty"`
	States         int               `json:"states"`
	Actions        int               `json:"actions"`
	Truncated      bool              `json:"truncated"`
	Recovery       RecoveryOutcome   `json:"recovery"`
	RecoveryClass  RecoveryClass     `json:"recovery_class"`
	Spent          ExploreSpent      `json:"spent,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	Checkpoint     string            `json:"checkpoint,omitempty"`
}

type exploreWorld struct {
	state      string
	role       string
	principal  string
	account    string
	conditions map[string]bool
	history    map[string]json.RawMessage
	lastAction string
	lastID     string
	lastDenied bool
	lastTO     bool
}

func (b ExploreRuntimeBudget) normalized() ExploreRuntimeBudget {
	if b.MaxStates < 1 {
		b.MaxStates = 16
	}
	if b.MaxStates > 10000 {
		b.MaxStates = 10000
	}
	if b.MaxPaths < 1 {
		b.MaxPaths = 16
	}
	if b.MaxPaths > 256 {
		b.MaxPaths = 256
	}
	if b.MaxActions < 1 {
		b.MaxActions = 8
	}
	if b.MaxActions > 1024 {
		b.MaxActions = 1024
	}
	if b.Deadline < 100*time.Millisecond {
		b.Deadline = 30 * time.Second
	}
	if b.Deadline > 24*time.Hour {
		b.Deadline = 24 * time.Hour
	}
	if b.MaxReplans < 0 {
		b.MaxReplans = 0
	}
	if b.MaxReplans > 64 {
		b.MaxReplans = 64
	}
	return b
}

func validateExploreAdapter(adapter ExploreAdapter, graph string) error {
	if adapter.SchemaVersion != 1 || adapter.Graph == "" || adapter.Graph != graph || adapter.IsolationScope == "" {
		return errors.New("explore adapter needs schema_version 1, the same graph and an isolation scope")
	}
	if adapter.TimeoutMS < 100 || adapter.StepTimeoutMS < 50 || adapter.Parallelism < 1 || adapter.Parallelism > 16 {
		return errors.New("explore adapter timeouts and parallelism are out of range")
	}
	if len(adapter.Entries) == 0 || len(adapter.Entries) > 64 {
		return errors.New("explore adapter needs 1..64 entries")
	}
	if err := validateOracle(adapter.Oracle); err != nil {
		return err
	}
	if adapter.Recovery != nil {
		if err := validateOracle(adapter.Recovery.Oracle); err != nil {
			return err
		}
	}
	return nil
}

func (d db) runExploration(parent context.Context, request ExploreRequest) (ExploreReport, error) {
	request.Mode = decodePlannerMode(request.Mode)
	if request.Campaign != "" {
		return d.resumeExploration(parent, request)
	}
	report := ExploreReport{Graph: request.Graph, Flow: request.Flow, Remaining: []CandidateAction{}, Steps: []ExploreStep{}, Unspecified: []UnspecifiedItem{}, RuleFails: []ExploreFinding{}, Risks: []ExploreFinding{}, Mode: request.Mode}
	if request.Graph == "" || request.Flow == "" || request.Environment == "" || request.Round == "" || request.Root == "" || request.Config == "" || request.Commit == "" || request.Account == "" || request.Fixture == "" {
		return report, errors.New("discover run needs graph, flow, approved environment, round, candidate, account and fixture")
	}
	if err := validateExploreAdapter(request.Adapter, request.Graph); err != nil {
		return report, err
	}
	budget := request.Budget.normalized()
	g, err := d.model(request.Graph)
	if err != nil {
		return report, err
	}
	if g.SchemaVersion != 2 {
		return report, errors.New("discover run requires a v2 business model")
	}
	x := indexModel(g)
	flow, found := x.flows[request.Flow]
	if !found {
		return report, fmt.Errorf("flow %s not found", request.Flow)
	}
	approvals, err := d.ruleApprovals(request.Graph)
	if err != nil {
		return report, err
	}
	campaign := newID("X")
	report.Campaign = campaign
	report.Environment = request.Environment
	report.Round = request.Round
	binding, err := d.exploreBinding(request.Graph, request.Environment, request.Commit, request.Account, request.Fixture, request.Root, request.Config, request.Adapter.Oracle.ID, "explore/"+campaign)
	if err != nil {
		return report, err
	}
	accounts, actors, ids, err := d.exploreAccounts(request.Adapter, binding, request.Account, request.Fixture)
	if err != nil {
		return report, err
	}
	if err := d.lockExploreFixtures(campaign, ids); err != nil {
		return report, err
	}
	if err := d.attachExploreReceipt(campaign, request.Round); err != nil {
		return report, err
	}
	ctx, cancel := context.WithTimeout(parent, budget.Deadline)
	defer cancel()
	fixture, _, err := d.bindingFixture(request.Fixture)
	if err != nil {
		return report, err
	}
	world, err := d.observeWorld(ctx, request.Adapter, binding, accounts["primary"], flow.Initial)
	if err != nil {
		report.StopReason = "blocked"
		return d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, campaign, err)
	}
	modelSHA, _ := d.graphFingerprint(request.Graph)
	seen := map[string]bool{}
	tried := map[string]bool{}
	replans := 0
	recordWorld := func(current exploreWorld) {
		key, ok := exploreEquivalenceKey(g, x, flow, current)
		if ok {
			if seen[key] {
				return
			}
			seen[key] = true
		}
		report.States++
	}
	recordWorld(world)
	for {
		if ctx.Err() != nil {
			report.StopReason, report.Truncated = "budget_time", true
			break
		}
		if report.Actions >= budget.MaxActions {
			leftover := remainingCandidates(proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried), tried, world.state)
			if len(leftover) == 0 {
				report.StopReason = "complete"
			} else {
				report.StopReason, report.Truncated = "budget_actions", true
				report.Remaining = leftover
			}
			break
		}
		if report.States >= budget.MaxStates {
			report.StopReason, report.Truncated = "budget_states", true
			break
		}
		candidates := proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried)
		if request.Mode == "deterministic" {
			candidates = filterDeterministic(candidates)
		}
		if shouldAskPlanner(request.Mode, candidates, report, world) && request.Planner != nil {
			proposal, planErr := request.Planner.Suggest(ctx, plannerContext(request, flow, approvals, world, report, candidates, budget.MaxActions-report.Actions))
			if planErr == nil {
				rejected := []string{}
				for _, suggestion := range proposal.Suggestions {
					item, reason := validatePlannerSuggestion(suggestion, request.Adapter, request.Adapter.Accounts, budget.MaxActions-report.Actions)
					if reason != "" {
						rejected = append(rejected, reason)
						continue
					}
					item.ID = shortDigest(string(item.Kind) + "|" + item.Action + "|" + item.Role + "|" + item.RequestID + "|" + world.state)
					if !tried[candidateKey(item, world.state)] {
						candidates = append(candidates, item)
					}
				}
				_ = marshalPlannerAudit(proposal, candidates, rejected)
			}
		}
		candidates = rotateCandidates(candidates, budget.Seed)
		if len(candidates) == 0 {
			report.StopReason = "no_candidates"
			break
		}
		chosen, ok := selectCandidate(candidates, tried, world.state)
		if !ok {
			report.StopReason = "complete"
			report.Remaining = candidates
			break
		}
		tried[candidateKey(chosen, world.state)] = true
		tried[chosen.ID] = true
		needsReplan := chosen.Kind == kindUndeclared || chosen.Kind == kindSkipPre || chosen.Source == sourceInfer && (world.state == "" || !modelState(g, world.state))
		if needsReplan {
			replans++
			if replans > budget.MaxReplans {
				report.StopReason, report.Truncated = "budget_replans", true
				report.Remaining = remainingCandidates(candidates, tried, world.state)
				break
			}
		}
		step, live, execErr := d.exploreOnce(ctx, binding, request.Adapter, accounts, actors, chosen, campaign, report.Actions+1, world)
		if execErr != nil {
			if step.RAW != "" || step.ClassifyError != "" || len(step.Calls) > 0 || len(step.Findings) > 0 {
				report.Steps = append(report.Steps, step)
				report.Actions++
				for _, finding := range step.Findings {
					if finding.Kind == "BLOCKED" {
						report.Blocked = append(report.Blocked, finding)
					}
					if finding.Kind == "FAIL" {
						report.RuleFails = append(report.RuleFails, finding)
					}
				}
			}
			if ctx.Err() != nil {
				report.StopReason, report.Truncated = "budget_time", true
				report.Remaining = remainingCandidates(candidates, tried, world.state)
				break
			}
			if step.ClassifyError != "" {
				report.StopReason = "classify_failed"
			} else {
				report.StopReason = "blocked"
			}
			return d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, campaign, execErr)
		}
		report.Steps = append(report.Steps, step)
		report.Actions++
		for _, finding := range step.Findings {
			switch finding.Kind {
			case "FAIL":
				report.RuleFails = append(report.RuleFails, finding)
			case "RISK":
				report.Risks = append(report.Risks, finding)
			case "BLOCKED":
				report.Blocked = append(report.Blocked, finding)
			}
		}
		if step.Discovery != nil {
			report.WithinContract += step.Discovery.WithinContract
			report.Unspecified = append(report.Unspecified, step.Discovery.Unspecified...)
		}
		if chosen.Kind == kindRecover {
			if !step.Denied && step.After.State == flow.Initial {
				world = exploreWorld{state: step.After.State, role: step.After.Role, principal: step.After.Principal, account: request.Account, conditions: map[string]bool(step.After.Conditions), history: step.After.Values}
				recordWorld(world)
				continue
			}
			report.StopReason = "recovery_blocked"
			break
		}
		world = live
		recordWorld(world)
		report.Checkpoint = newID("CK")
		_ = d.saveExploreCheckpoint(ExploreCheckpoint{
			Campaign: campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round,
			ModelSHA: modelSHA, SourceCommit: request.Commit, Status: exploreRunning, World: persistWorld(world),
			Tried: mapKeys(tried), Budget: budget, Spent: ExploreSpent{Actions: report.Actions, States: report.States, Replans: replans},
			Account: request.Account, Fixture: request.Fixture, Checkpoint: report.Checkpoint, Mode: request.Mode, Report: report,
		})
	}
	if report.StopReason == "" {
		report.StopReason = "complete"
	}
	if report.Remaining == nil {
		report.Remaining = remainingCandidates(proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried), tried, world.state)
	}
	if len(report.Remaining) > 0 && (report.StopReason == "budget_actions" || report.StopReason == "budget_states" || report.StopReason == "budget_time" || report.StopReason == "budget_replans") {
		report.Truncated = true
	}
	final, err := d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, campaign, nil)
	status := exploreCompleted
	if final.StopReason == "blocked" || final.StopReason == "recovery_blocked" || final.StopReason == "classify_failed" {
		status = exploreBlocked
	}
	if final.Truncated {
		status = exploreInterrupted
	}
	_ = d.saveExploreCheckpoint(ExploreCheckpoint{
		Campaign: campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round,
		ModelSHA: modelSHA, SourceCommit: request.Commit, Status: status, World: persistWorld(world),
		Tried: mapKeys(tried), Frontier: final.Remaining, Budget: budget, Spent: final.Spent,
		Recovery: final.Recovery, RecoveryClass: final.RecoveryClass, Account: request.Account, Fixture: request.Fixture,
		Checkpoint: final.Checkpoint, Mode: request.Mode, Report: final,
	})
	return final, err
}

func mapKeys(values map[string]bool) []string {
	out := []string{}
	for key, ok := range values {
		if ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func (d db) resumeExploration(parent context.Context, request ExploreRequest) (ExploreReport, error) {
	state, err := d.loadExploreCheckpoint(request.Campaign)
	if err != nil {
		return ExploreReport{}, err
	}
	report := state.Report
	if report.Campaign == "" {
		report.Campaign = state.Campaign
		report.Graph, report.Flow, report.Environment, report.Round = state.Graph, state.Flow, state.Environment, state.Round
	}
	if request.Graph != "" && request.Graph != state.Graph {
		return report, errors.New("resume graph does not match the checkpoint")
	}
	modelSHA, err := d.graphFingerprint(state.Graph)
	if err != nil {
		return report, err
	}
	if err := versionsMatch(state, request.Commit, modelSHA); err != nil {
		return report, err
	}
	if state.RecoveryClass == RecoveryFailedLocked {
		return report, errors.New("refusing to continue after failed fixture recovery")
	}
	if state.Status == exploreAborted || state.Status == exploreCompleted {
		return report, fmt.Errorf("campaign %s is %s", state.Campaign, state.Status)
	}
	worker := exploreWorkerID()
	if err := d.claimExploreCampaign(state.Campaign, worker); err != nil {
		return report, err
	}
	request.Graph = firstNonEmpty(request.Graph, state.Graph)
	request.Flow = firstNonEmpty(request.Flow, state.Flow)
	request.Environment = firstNonEmpty(request.Environment, state.Environment)
	request.Round = firstNonEmpty(request.Round, state.Round)
	request.Account = firstNonEmpty(request.Account, state.Account)
	request.Fixture = firstNonEmpty(request.Fixture, state.Fixture)
	if request.Adapter.Graph == "" {
		request.Adapter = ExploreAdapter{}
	}
	g, err := d.model(request.Graph)
	if err != nil {
		return report, err
	}
	x := indexModel(g)
	flow, found := x.flows[request.Flow]
	if !found {
		return report, fmt.Errorf("flow %s not found", request.Flow)
	}
	approvals, err := d.ruleApprovals(request.Graph)
	if err != nil {
		return report, err
	}
	binding, err := d.exploreBinding(request.Graph, request.Environment, request.Commit, request.Account, request.Fixture, request.Root, request.Config, request.Adapter.Oracle.ID, "explore/"+state.Campaign)
	if err != nil {
		return report, err
	}
	accounts, actors, ids, err := d.exploreAccounts(request.Adapter, binding, request.Account, request.Fixture)
	if err != nil {
		return report, err
	}
	_ = ids
	if err := d.reconcileUnknownActions(parent, binding, request.Adapter, accounts, &state); err != nil {
		return report, err
	}
	budget := request.Budget.normalized()
	if budget.MaxActions < state.Spent.Actions {
		budget.MaxActions = state.Spent.Actions
	}
	ctx, cancel := context.WithTimeout(parent, budget.Deadline)
	defer cancel()
	fixture, _, err := d.bindingFixture(request.Fixture)
	if err != nil {
		return report, err
	}
	world := restoreWorld(state.World)
	tried := map[string]bool{}
	for _, key := range state.Tried {
		tried[key] = true
	}
	seen := map[string]bool{}
	for _, key := range state.Seen {
		seen[key] = true
	}
	replans := state.Spent.Replans
	report = state.Report
	report.Mode = firstNonEmpty(request.Mode, state.Mode)
	for {
		if ctx.Err() != nil {
			report.StopReason, report.Truncated = "budget_time", true
			break
		}
		if report.Actions >= budget.MaxActions {
			leftover := remainingCandidates(proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried), tried, world.state)
			if len(leftover) == 0 {
				report.StopReason = "complete"
			} else {
				report.StopReason, report.Truncated = "budget_actions", true
				report.Remaining = leftover
			}
			break
		}
		candidates := proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried)
		if request.Mode == "deterministic" {
			candidates = filterDeterministic(candidates)
		}
		candidates = rotateCandidates(candidates, budget.Seed)
		chosen, ok := selectCandidate(candidates, tried, world.state)
		if !ok {
			if len(state.Frontier) > 0 {
				chosen, ok = selectCandidate(state.Frontier, tried, world.state)
			}
			if !ok {
				report.StopReason = "complete"
				report.Remaining = candidates
				break
			}
		}
		tried[candidateKey(chosen, world.state)] = true
		tried[chosen.ID] = true
		step, live, execErr := d.exploreOnce(ctx, binding, request.Adapter, accounts, actors, chosen, state.Campaign, report.Actions+1, world)
		if execErr != nil {
			if step.RAW != "" || step.ClassifyError != "" || len(step.Calls) > 0 {
				report.Steps = append(report.Steps, step)
				report.Actions++
			}
			if ctx.Err() != nil {
				report.StopReason, report.Truncated = "budget_time", true
				break
			}
			report.StopReason = "blocked"
			return d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, state.Campaign, execErr)
		}
		report.Steps = append(report.Steps, step)
		report.Actions++
		for _, finding := range step.Findings {
			if finding.Kind == "FAIL" {
				report.RuleFails = append(report.RuleFails, finding)
			}
		}
		if step.Discovery != nil {
			report.WithinContract += step.Discovery.WithinContract
			report.Unspecified = append(report.Unspecified, step.Discovery.Unspecified...)
		}
		world = live
		_ = replans
	}
	if report.StopReason == "" {
		report.StopReason = "complete"
	}
	final, err := d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, state.Campaign, nil)
	status := exploreCompleted
	if final.Truncated {
		status = exploreInterrupted
	}
	_ = d.saveExploreCheckpoint(ExploreCheckpoint{
		Campaign: state.Campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round,
		ModelSHA: modelSHA, SourceCommit: request.Commit, Status: status, World: persistWorld(world),
		Tried: mapKeys(tried), Frontier: final.Remaining, Budget: budget, Spent: final.Spent,
		Recovery: final.Recovery, RecoveryClass: final.RecoveryClass, Account: request.Account, Fixture: request.Fixture,
		Checkpoint: final.Checkpoint, Mode: request.Mode, Report: final,
	})
	return final, err
}

func (d db) ruleApprovals(graph string) (map[string]bool, error) {
	rows, err := d.query("SELECT id,approved FROM rules WHERE graph="+quote(graph), 2)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, row := range rows {
		out[row[0]] = row[1] == "1"
	}
	return out, nil
}

func (d db) exploreBinding(graph, env, commit, accountID, fixtureID, root, config, oracle, task string) (RunBinding, error) {
	target, configHash, err := readApprovedEnvironment(config, env)
	if err != nil {
		return RunBinding{}, err
	}
	sourceRoot, err := absPath(root)
	if err != nil {
		return RunBinding{}, err
	}
	configPath, err := absPath(config)
	if err != nil {
		return RunBinding{}, err
	}
	head, sourceHash, err := sourceFingerprint(sourceRoot)
	if err != nil {
		return RunBinding{}, err
	}
	if head != commit {
		return RunBinding{}, errors.New("source commit is not the current candidate HEAD")
	}
	account, accountHash, err := d.bindingFixture(accountID)
	if err != nil {
		return RunBinding{}, err
	}
	fixture, fixtureHash, err := d.bindingFixture(fixtureID)
	if err != nil {
		return RunBinding{}, err
	}
	if account.Kind != "account" || strings.TrimSpace(account.Refs["principal"]) == "" {
		return RunBinding{}, errors.New("explore account fixture needs a principal")
	}
	if account.Refs["environment"] != env || fixture.Refs["environment"] != env {
		return RunBinding{}, errors.New("account and fixture must belong to the selected environment")
	}
	graphHash, err := d.graphFingerprint(graph)
	if err != nil {
		return RunBinding{}, err
	}
	return RunBinding{Identity: ExecutionIdentity{
		Graph: graph, Task: task, Environment: env, Target: target,
		SourceCommit: commit, SourceSHA256: sourceHash, ConfigSHA256: configHash, GraphSHA256: graphHash,
		Actor: account.Refs["role"], Account: accountID, Principal: account.Refs["principal"], AccountSHA256: accountHash,
		Fixture: fixtureID, FixtureSHA256: fixtureHash,
	}, Oracle: oracle, SourceRoot: sourceRoot, ConfigPath: configPath}, nil
}

func absPath(value string) (string, error) {
	return filepath.Abs(value)
}

func (d db) exploreAccounts(adapter ExploreAdapter, binding RunBinding, primary, fixture string) (map[string]Fixture, map[string]ParticipantBinding, []string, error) {
	accounts := map[string]Fixture{}
	actors := map[string]ParticipantBinding{}
	ids := []string{primary, fixture}
	load := func(alias, id string) error {
		if id == "" {
			return nil
		}
		account, hash, err := d.bindingFixture(id)
		if err != nil {
			return err
		}
		accounts[alias] = account
		actors[alias] = ParticipantBinding{Alias: alias, Role: account.Refs["role"], Account: id, Principal: account.Refs["principal"], SHA256: hash}
		ids = append(ids, id)
		return nil
	}
	if err := load("primary", primary); err != nil {
		return nil, nil, nil, err
	}
	for role, id := range adapter.Accounts {
		if err := load(role, id); err != nil {
			return nil, nil, nil, err
		}
	}
	_ = binding
	return accounts, actors, uniqueIDs(ids), nil
}

func (d db) attachExploreReceipt(run, round string) error {
	record := ModelRunRecord{Run: run, Round: round, Runner: "discover-run", Recovery: RecoveryOutcome{Verdict: "NOT_RUN", Actions: []ActionObservation{}}}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return d.tx([]string{fmt.Sprintf("INSERT INTO model_runs(run,round,runner,body) VALUES(%s,%s,%s,%s);", quote(run), quote(round), quote("discover-run"), quote(string(body)))})
}

func (d db) lockExploreFixtures(run string, ids []string) error {
	statements := []string{}
	for _, id := range ids {
		statements = append(statements, fmt.Sprintf("INSERT INTO fixture_locks(fixture,run) VALUES(%s,%s);", quote(id), quote(run)))
	}
	return d.tx(statements)
}

func (d db) observeWorld(ctx context.Context, adapter ExploreAdapter, binding RunBinding, account Fixture, fallback string) (exploreWorld, error) {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(adapter.StepTimeoutMS)*time.Millisecond)
	defer cancel()
	facts, _, err := readOracle(callCtx, adapter.Oracle, binding, account, adapter.IsolationScope)
	if err != nil {
		return exploreWorld{}, err
	}
	state := facts.State
	if state == "" {
		state = fallback
	}
	return exploreWorld{state: state, role: facts.Role, principal: facts.Principal, account: account.ID, conditions: map[string]bool(facts.Conditions), history: facts.Values}, nil
}

func proposeCandidates(g Graph, x modelIndex, flow BehaviorFlow, adapter ExploreAdapter, approvals map[string]bool, world exploreWorld, tried map[string]bool) []CandidateAction {
	entries := map[string]ExploreEntry{}
	modelActions := map[string]bool{}
	for _, entry := range adapter.Entries {
		entries[entry.Action] = entry
	}
	for _, node := range g.Nodes {
		if node.Kind == "action" {
			modelActions[node.ID] = true
		}
	}
	out := []CandidateAction{}
	add := func(item CandidateAction) {
		if item.Account == "" || item.Entry == "" || item.Action == "" {
			return
		}
		item.ID = shortDigest(string(item.Kind) + "|" + item.Action + "|" + item.Role + "|" + item.RequestID + "|" + world.state)
		if tried[candidateKey(item, world.state)] {
			return
		}
		out = append(out, item)
	}
	adjacency := flowAdjacency(x, flow, g.Transitions)
	for _, contract := range adjacency[world.state] {
		entry, ok := entries[contract.Action]
		if !ok {
			continue
		}
		rule := x.rules[contract.Rule]
		source := sourceInfer
		risk := "medium"
		if approvals[contract.Rule] {
			source = sourceRule
			risk = "low"
		}
		role := rule.Actor
		account := adapter.Accounts[role]
		if account == "" {
			for alias, id := range adapter.Accounts {
				if alias == role {
					account = id
					break
				}
			}
		}
		matched := observedConditions(world.conditions, contract.Preconditions, nil)
		if matched && account != "" {
			add(CandidateAction{Kind: kindDeclared, Source: source, Action: contract.Action, Entry: entry.Entry, Role: role, Account: account, Transition: contract.Edge, Preconditions: map[string]bool(contract.Preconditions), Scope: flow.Capability, Risk: risk, Cost: 1, RequestID: "e-" + shortDigest(world.state + contract.Edge)[:8]})
		}
		if !matched && account != "" {
			add(CandidateAction{Kind: kindSkipPre, Source: sourceInfer, Action: contract.Action, Entry: entry.Entry, Role: role, Account: account, Transition: contract.Edge, Preconditions: map[string]bool(contract.Preconditions), Scope: flow.Capability, Risk: "high", Cost: 2, RequestID: "skip-" + shortDigest(world.state + contract.Edge)[:8]})
		}
		for alias, id := range adapter.Accounts {
			if alias == role || id == account {
				continue
			}
			add(CandidateAction{Kind: kindRole, Source: sourceInfer, Action: contract.Action, Entry: entry.Entry, Role: alias, Account: id, Transition: contract.Edge, Preconditions: map[string]bool(contract.Preconditions), Scope: flow.Capability, Risk: "high", Cost: 2, RequestID: "role-" + shortDigest(alias + contract.Edge)[:8]})
		}
	}
	for _, rule := range g.Rules {
		if !approvals[rule.ID] || rule.Actor == world.role || rule.Rejected != "UNCHANGED" {
			continue
		}
		entry, ok := entries[rule.Action]
		if !ok {
			continue
		}
		account := adapter.Accounts[world.role]
		if account == "" {
			account = adapter.Accounts[rule.Actor]
		}
		add(CandidateAction{Kind: kindRole, Source: sourceRule, Action: rule.Action, Entry: firstNonEmpty(rule.Entry, entry.Entry), Role: world.role, Account: account, Scope: flow.Capability, Risk: "high", Cost: 2, RequestID: "deny-" + shortDigest(rule.ID + world.role)[:8]})
	}
	for action, entry := range entries {
		if modelActions[action] {
			continue
		}
		role := world.role
		account := adapter.Accounts[role]
		if account == "" {
			for alias, id := range adapter.Accounts {
				role, account = alias, id
				break
			}
		}
		add(CandidateAction{Kind: kindUndeclared, Source: sourceImpl, Action: action, Entry: entry.Entry, Role: role, Account: account, Scope: flow.Capability, Risk: "medium", Cost: 3, RequestID: "impl-" + shortDigest(action)[:8]})
	}
	if world.lastAction != "" && world.lastID != "" {
		if entry, ok := entries[world.lastAction]; ok {
			add(CandidateAction{Kind: kindRepeat, Source: sourceInfer, Action: world.lastAction, Entry: entry.Entry, Role: world.role, Account: firstNonEmpty(world.account, adapter.Accounts[world.role]), Scope: flow.Capability, Risk: "medium", Cost: 2, RequestID: world.lastID})
		}
	}
	if world.lastTO {
		if entry, ok := entries[world.lastAction]; ok {
			add(CandidateAction{Kind: kindTimeout, Source: sourceInfer, Action: world.lastAction, Entry: entry.Entry, Role: world.role, Account: firstNonEmpty(world.account, adapter.Accounts[world.role]), Scope: flow.Capability, Risk: "medium", Cost: 3, RequestID: "retry-" + world.lastID})
		}
	}
	if partners := concurrentPartners(adapter); world.state == flow.Initial && len(partners) >= 2 {
		if entry, ok := entries["claim"]; ok {
			add(CandidateAction{Kind: kindConcurrent, Source: sourceInfer, Action: "claim", Entry: entry.Entry, Role: partners[0], Account: adapter.Accounts[partners[0]], PartnerRole: partners[1], Scope: flow.Capability, Risk: "high", Cost: 4, RequestID: "pair-" + shortDigest(partners[0] + partners[1])[:8]})
		} else {
			for action, entry := range entries {
				add(CandidateAction{Kind: kindConcurrent, Source: sourceInfer, Action: action, Entry: entry.Entry, Role: partners[0], Account: adapter.Accounts[partners[0]], PartnerRole: partners[1], Scope: flow.Capability, Risk: "high", Cost: 4, RequestID: "pair-" + shortDigest(action)[:8]})
				break
			}
		}
	}
	if adapter.Recovery != nil && world.state != flow.Initial {
		add(CandidateAction{Kind: kindRecover, Source: sourceInfer, Action: "fixture.restore", Entry: "/reset", Role: "primary", Account: firstNonEmpty(adapter.Accounts[world.role], world.account), Scope: flow.Capability, Risk: "low", Cost: 1})
	}
	if world.state != flow.Initial {
		if entry, ok := entries["approve"]; ok {
			if id := adapter.Accounts["ADMIN"]; id != "" {
				add(CandidateAction{Kind: kindReorder, Source: sourceInfer, Action: "approve", Entry: entry.Entry, Role: "ADMIN", Account: id, Scope: flow.Capability, Risk: "medium", Cost: 2, RequestID: "ord-" + shortDigest(world.state)[:8]})
			}
		}
	}
	if len(world.history) > 0 {
		for action, entry := range entries {
			add(CandidateAction{Kind: kindHistory, Source: sourceInfer, Action: action, Entry: entry.Entry, Role: world.role, Account: firstNonEmpty(world.account, adapter.Accounts[world.role]), Scope: flow.Capability, Risk: "medium", Cost: 3, RequestID: "hist-" + shortDigest(action + world.state)[:8]})
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return candidateScore(out[i]) > candidateScore(out[j]) || candidateScore(out[i]) == candidateScore(out[j]) && out[i].ID < out[j].ID
	})
	return out
}

func concurrentPartners(adapter ExploreAdapter) []string {
	names := []string{}
	for alias := range adapter.Accounts {
		names = append(names, alias)
	}
	sort.Strings(names)
	if len(names) > 2 {
		return names[:2]
	}
	return names
}

func candidateScore(item CandidateAction) int {
	score := 10 - item.Cost
	switch item.Source {
	case sourceRule:
		score += 100
	case sourceImpl:
		score += 30
	default:
		score += 15
	}
	switch item.Kind {
	case kindDeclared:
		score += 20
	case kindSkipPre, kindRole:
		score += 25
	case kindConcurrent, kindRepeat:
		score += 18
	case kindRecover:
		score += 5
	}
	if item.Risk == "high" {
		score += 8
	}
	return score
}

func rotateCandidates(candidates []CandidateAction, seed int) []CandidateAction {
	if seed == 0 || len(candidates) < 2 {
		return candidates
	}
	shift := seed % len(candidates)
	if shift == 0 {
		return candidates
	}
	out := make([]CandidateAction, 0, len(candidates))
	out = append(out, candidates[shift:]...)
	return append(out, candidates[:shift]...)
}

func selectCandidate(candidates []CandidateAction, tried map[string]bool, state string) (CandidateAction, bool) {
	for _, item := range candidates {
		if !tried[candidateKey(item, state)] && !tried[item.ID] {
			return item, true
		}
	}
	return CandidateAction{}, false
}

func candidateKey(item CandidateAction, state string) string {
	return item.Kind.String() + "|" + item.Action + "|" + item.Role + "|" + item.RequestID + "|" + state
}

func (k ExploreKind) String() string { return string(k) }

func remainingCandidates(candidates []CandidateAction, tried map[string]bool, state string) []CandidateAction {
	out := []CandidateAction{}
	for _, item := range candidates {
		if !tried[candidateKey(item, state)] && !tried[item.ID] {
			out = append(out, item)
		}
	}
	return out
}

func exploreEquivalenceKey(g Graph, x modelIndex, flow BehaviorFlow, world exploreWorld) (string, bool) {
	required := relevantConditions(g, x, flow, world.state)
	for _, id := range required {
		if _, ok := world.conditions[id]; !ok {
			return "", false
		}
	}
	parts := []string{"state=" + world.state, "role=" + world.role}
	for _, id := range required {
		parts = append(parts, fmt.Sprintf("%s=%t", id, world.conditions[id]))
	}
	keys := []string{}
	for id := range world.history {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		parts = append(parts, id+"="+string(world.history[id]))
	}
	return strings.Join(parts, "|"), true
}

func (d db) exploreOnce(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, actors map[string]ParticipantBinding, candidate CandidateAction, run string, index int, world exploreWorld) (ExploreStep, exploreWorld, error) {
	step := ExploreStep{Candidate: candidate, Findings: []ExploreFinding{}}
	stepTimeout := time.Duration(adapter.StepTimeoutMS) * time.Millisecond
	oracleAccount := accountFor(accounts, candidate)
	beforeCtx, cancelBefore := context.WithTimeout(ctx, stepTimeout)
	before, beforeHash, err := readOracle(beforeCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
	cancelBefore()
	if err != nil {
		return step, world, err
	}
	step.Before = before
	live := world
	live.state, live.role, live.principal, live.conditions, live.history = before.State, before.Role, before.Principal, map[string]bool(before.Conditions), before.Values
	if candidate.Kind == kindRecover {
		outcome := d.restoreExplore(ctx, binding, adapter, accounts, actors, Fixture{State: adapter.Recovery.State}, run)
		step.Denied = outcome.Verdict != "PASS"
		step.After = IndependentFacts{State: outcome.State, Role: before.Role, Principal: before.Principal, Scope: before.Scope, Conditions: before.Conditions, Values: before.Values}
		if outcome.Verdict == "PASS" {
			afterCtx, cancel := context.WithTimeout(ctx, stepTimeout)
			after, _, readErr := readOracle(afterCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
			cancel()
			if readErr != nil {
				return step, live, readErr
			}
			step.After = after
			live.state, live.role, live.principal, live.conditions, live.history = after.State, after.Role, after.Principal, map[string]bool(after.Conditions), after.Values
		}
		step.RAW, step.SHA256 = outcome.Evidence, outcome.SHA256
		if step.Denied {
			step.Findings = append(step.Findings, ExploreFinding{Kind: "FAIL", Reason: "recovery did not restore the isolated fixture: " + outcome.Reason, Evidence: outcome.Evidence})
		}
		return step, live, nil
	}
	output, calls, callErr := d.performExplore(ctx, binding, adapter, accounts, candidate)
	step.Calls = calls
	if callErr != nil && !isTimeout(callErr) {
		return step, live, callErr
	}
	live.lastTO = isTimeout(callErr)
	step.Denied = output.Denied
	afterCtx, cancelAfter := context.WithTimeout(ctx, stepTimeout)
	after, afterHash, err := readOracle(afterCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
	cancelAfter()
	if err != nil {
		return step, live, err
	}
	step.After = after
	live.state, live.role, live.principal, live.conditions, live.history = after.State, after.Role, after.Principal, map[string]bool(after.Conditions), after.Values
	live.lastAction, live.lastID, live.lastDenied = candidate.Action, candidate.RequestID, output.Denied
	g, err := d.model(binding.Identity.Graph)
	if err != nil {
		return step, live, err
	}
	approvals, err := d.ruleApprovals(binding.Identity.Graph)
	if err != nil {
		return step, live, err
	}
	step.Findings = append(step.Findings, judgeExploration(g, approvals, actorRole(accounts, candidate), candidate.Action, before, after, output.Denied)...)
	step.Findings = append(step.Findings, invariantFindings(g, after)...)
	if candidate.Source == sourceInfer && !hasFinding(step.Findings, "FAIL") {
		step.Findings = append(step.Findings, ExploreFinding{Kind: "RISK", Reason: "inferred candidate " + string(candidate.Kind) + "; not a confirmed business rule"})
	}
	raw := StepRAW{Identity: binding.Identity, Phase: "action", Run: run, Step: index, Expected: expectedExplore(g, candidate, before), Observed: after.State, Preconditions: map[string]bool(before.Conditions), Before: beforeHash, After: afterHash, Oracle: adapter.Oracle.ID, Denied: output.Denied, Model: &ModelObservation{Node: candidate.Action, Transition: candidate.Transition, Round: "", Input: candidateBody(adapter, candidate), Participants: []ParticipantBinding{actors[aliasFor(accounts, candidate)]}, Oracles: []OracleObservation{{Actor: candidate.Role, ID: adapter.Oracle.ID, Boundary: adapter.Oracle.Boundary, Before: before, After: after, BeforeSHA256: beforeHash, AfterSHA256: afterHash}}, Actions: []ActionObservation{actionRecord(RunnerStep{Node: candidate.Action, Transition: candidate.Transition, Actor: candidate.Role}, actors[aliasFor(accounts, candidate)], output)}}}
	path, body, err := writeExploreRAW(d, run, index, raw)
	if err != nil {
		return step, live, err
	}
	step.RAW, step.SHA256 = path, digestBytes(body)
	observation := ExploredObservation{Entry: candidate.Entry, Role: after.Role, Principal: after.Principal, Action: candidate.Action, BeforeState: before.State, AfterState: after.State, Denied: output.Denied, Conditions: map[string]bool(before.Conditions), Measurements: after.Values, Evidence: path}
	if observation.Role == "" {
		observation.Role = candidate.Role
	}
	if observation.Principal == "" {
		observation.Principal = before.Principal
	}
	discovery, discErr := d.recordExploration(Exploration{Graph: binding.Identity.Graph, Exploration: run, Observations: []ExploredObservation{observation}}, 32)
	if discErr != nil {
		step.ClassifyError = discErr.Error()
		step.Findings = append(step.Findings, ExploreFinding{Kind: "BLOCKED", Reason: "discovery classification failed: " + discErr.Error(), Evidence: path})
		return step, live, fmt.Errorf("discovery classification failed: %w", discErr)
	}
	step.Discovery = &discovery
	return step, live, nil
}

func (d db) performExplore(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, candidate CandidateAction) (AdapterOutput, []ExploreCallRecord, error) {
	stepTimeout := time.Duration(adapter.StepTimeoutMS) * time.Millisecond
	if candidate.Kind == kindConcurrent && candidate.PartnerRole != "" {
		type result struct {
			out AdapterOutput
			err error
		}
		results := make([]result, 2)
		var workers sync.WaitGroup
		partners := []CandidateAction{candidate, candidate}
		partners[1].Role = candidate.PartnerRole
		partners[1].Account = adapter.Accounts[candidate.PartnerRole]
		partners[1].RequestID = candidate.RequestID + "-b"
		for i := range partners {
			workers.Add(1)
			go func(i int) {
				defer workers.Done()
				callCtx, cancel := context.WithTimeout(ctx, stepTimeout)
				results[i].out, results[i].err = performCall(callCtx, exploreCall(adapter, partners[i]), binding, accountFor(accounts, partners[i]))
				cancel()
			}(i)
		}
		workers.Wait()
		records := []ExploreCallRecord{callRecord(partners[0], results[0].out, results[0].err), callRecord(partners[1], results[1].out, results[1].err)}
		if results[0].err != nil && results[1].err != nil {
			return results[0].out, records, results[0].err
		}
		primary := results[0].out
		if results[0].err != nil {
			primary = results[1].out
		}
		return primary, records, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	output, err := performCall(callCtx, exploreCall(adapter, candidate), binding, accountFor(accounts, candidate))
	return output, []ExploreCallRecord{callRecord(candidate, output, err)}, err
}

func callRecord(candidate CandidateAction, out AdapterOutput, err error) ExploreCallRecord {
	record := ExploreCallRecord{
		Role: candidate.Role, Account: candidate.Account, RequestID: candidate.RequestID,
		Denied: out.Denied, StatusCode: out.StatusCode, Started: out.Started, Completed: out.Completed,
		RequestSHA256: out.RequestSHA256, ResponseSHA256: out.ResponseSHA256,
	}
	if err != nil {
		record.Error = err.Error()
	}
	return record
}

func exploreCall(adapter ExploreAdapter, candidate CandidateAction) AdapterCall {
	method := httpMethod(adapter, candidate)
	header := adapter.Oracle.Call.CredentialHeader
	if header == "" {
		header = "X-Fixture-Actor"
	}
	return AdapterCall{Kind: adapterHTTP, Method: method, Path: candidate.Entry, Body: candidateBody(adapter, candidate), SuccessStatuses: []int{200}, RejectionStatuses: []int{400, 401, 403, 404, 409, 422}, CredentialHeader: header}
}

func httpMethod(adapter ExploreAdapter, candidate CandidateAction) string {
	for _, entry := range adapter.Entries {
		if entry.Action == candidate.Action && entry.Method != "" {
			return entry.Method
		}
	}
	return "POST"
}

func candidateBody(adapter ExploreAdapter, candidate CandidateAction) json.RawMessage {
	for _, entry := range adapter.Entries {
		if entry.Action == candidate.Action && len(entry.Body) > 0 {
			var obj map[string]any
			if json.Unmarshal(entry.Body, &obj) == nil {
				if _, ok := obj["request_id"]; !ok && candidate.RequestID != "" {
					obj["request_id"] = candidate.RequestID
				}
				if body, err := json.Marshal(obj); err == nil {
					return body
				}
			}
			return entry.Body
		}
	}
	if candidate.RequestID == "" {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(`{"request_id":"` + candidate.RequestID + `"}`)
}

func accountFor(accounts map[string]Fixture, candidate CandidateAction) Fixture {
	if account, ok := accounts[candidate.Role]; ok {
		return account
	}
	if account, ok := accounts["primary"]; ok {
		return account
	}
	return Fixture{}
}

func aliasFor(accounts map[string]Fixture, candidate CandidateAction) string {
	if _, ok := accounts[candidate.Role]; ok {
		return candidate.Role
	}
	return "primary"
}

func actorRole(accounts map[string]Fixture, candidate CandidateAction) string {
	if account, ok := accounts[candidate.Role]; ok && account.Refs["role"] != "" {
		return account.Refs["role"]
	}
	return candidate.Role
}

func judgeExploration(g Graph, approvals map[string]bool, role, action string, before, after IndependentFacts, denied bool) []ExploreFinding {
	findings := []ExploreFinding{}
	changed := after.State != before.State || measurementGrew(before, after, "records")
	hasActorRule := false
	for _, rule := range g.Rules {
		if !approvals[rule.ID] || rule.Action != action || rule.Actor != role {
			continue
		}
		hasActorRule = true
		required := map[string]bool{}
		for _, condition := range rule.Conditions {
			required[condition] = true
		}
		condsMet := observedConditions(map[string]bool(before.Conditions), required, rule.Conditions)
		switch {
		case condsMet && !denied && rule.Expected != "UNCHANGED" && after.State == rule.Expected:
			findings = append(findings, ExploreFinding{Kind: "PASS", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " allowed " + action + " and reached " + after.State})
		case condsMet && !denied && rule.Expected != "UNCHANGED" && after.State != rule.Expected:
			findings = append(findings, ExploreFinding{Kind: "FAIL", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " expected " + rule.Expected + " and observed " + after.State})
		case denied && changed:
			findings = append(findings, ExploreFinding{Kind: "FAIL", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " was rejected but the business state or records changed"})
		case condsMet && denied && !changed && rule.Expected != "UNCHANGED":
			findings = append(findings, ExploreFinding{Kind: "FAIL", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " requires allowing " + action + " but the request was rejected"})
		case !condsMet && rule.Rejected == "UNCHANGED" && denied && !changed:
			findings = append(findings, ExploreFinding{Kind: "PASS", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " correctly rejected " + action + " without side effects"})
		case !condsMet && rule.Rejected == "UNCHANGED" && !denied && changed:
			findings = append(findings, ExploreFinding{Kind: "FAIL", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " must stay unchanged when a guard is false"})
		}
	}
	if !hasActorRule && changed && !denied {
		for _, rule := range g.Rules {
			if approvals[rule.ID] && rule.Action == action && rule.Rejected == "UNCHANGED" {
				findings = append(findings, ExploreFinding{Kind: "FAIL", Rule: rule.ID, Reason: "confirmed rule " + rule.ID + " forbids " + role + " from changing state with " + action})
				break
			}
		}
	}
	return findings
}

func invariantFindings(g Graph, after IndependentFacts) []ExploreFinding {
	findings := []ExploreFinding{}
	for _, invariant := range g.Invariants {
		for _, outcome := range evaluateProperties(invariant.Checks, after.Values) {
			if outcome.Verdict == "PASS" {
				continue
			}
			kind := "FAIL"
			if outcome.Verdict == "BLOCKED" {
				kind = "BLOCKED"
			}
			findings = append(findings, ExploreFinding{Kind: kind, Rule: invariant.ID, Reason: "confirmed invariant " + invariant.ID + ": " + outcome.Reason})
		}
	}
	return findings
}

func expectedExplore(g Graph, candidate CandidateAction, before IndependentFacts) string {
	for _, rule := range g.Rules {
		if rule.Action == candidate.Action && rule.Actor == candidate.Role {
			return rule.Expected
		}
	}
	return before.State
}

func hasFinding(findings []ExploreFinding, kind string) bool {
	for _, item := range findings {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func isTimeout(err error) bool {
	return err != nil && (errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline") || strings.Contains(err.Error(), "canceled"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (d db) restoreExplore(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, actors map[string]ParticipantBinding, fixture Fixture, run string) RecoveryOutcome {
	if adapter.Recovery == nil {
		return RecoveryOutcome{Verdict: "NOT_RUN", Reason: "explore adapter has no recovery protocol", Actions: []ActionObservation{}}
	}
	if fixture.State == "" {
		fixture.State = adapter.Recovery.State
	}
	record := ModelRunRecord{Run: run, Participants: []ParticipantBinding{actors["primary"]}, Recovery: RecoveryOutcome{Verdict: "NOT_RUN", Actions: []ActionObservation{}}}
	manifest := RunnerManifest{IsolationScope: adapter.IsolationScope, StepTimeoutMS: adapter.StepTimeoutMS, Recovery: adapter.Recovery, Steps: []RunnerStep{{Actor: "primary", Oracles: []OracleAdapter{adapter.Oracle}}}}
	return d.restoreFixture(ctx, binding, record, manifest, accounts, actors, fixture, nil, true)
}

func writeExploreRAW(d db, run string, index int, value StepRAW) (string, []byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", nil, err
	}
	var parsed any
	if err := decodeUniqueJSON(body, &parsed); err != nil {
		return "", nil, err
	}
	if hasSensitiveKey(parsed) {
		return "", nil, errors.New("captured evidence contains credential-like data")
	}
	directory := filepath.Join(filepath.Dir(d.file), "raw", run)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", nil, err
	}
	path := filepath.Join(directory, fmt.Sprintf("%03d-%s.json", index, newID("explore")))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", nil, err
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		return "", nil, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		return "", nil, err
	}
	return path, body, nil
}

func (d db) persistExploration(report ExploreReport) error {
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	statements := []string{
		fmt.Sprintf("DELETE FROM exploration_actions WHERE campaign=%s;", quote(report.Campaign)),
		fmt.Sprintf("DELETE FROM exploration_frontier WHERE campaign=%s;", quote(report.Campaign)),
		fmt.Sprintf("INSERT OR REPLACE INTO exploration_campaigns(id,graph,flow,environment,round,stop_reason,body) VALUES(%s,%s,%s,%s,%s,%s,%s);", quote(report.Campaign), quote(report.Graph), quote(report.Flow), quote(report.Environment), quote(report.Round), quote(report.StopReason), quote(string(body))),
	}
	for index, step := range report.Steps {
		stepBody, err := json.Marshal(step)
		if err != nil {
			return err
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO exploration_actions(campaign,candidate,run,verdict,body) VALUES(%s,%s,%s,%s,%s);", quote(report.Campaign), quote(step.Candidate.ID), quote(report.Campaign), quote(strconv.Itoa(index)), quote(string(stepBody))))
	}
	for _, leftover := range report.Remaining {
		item, err := json.Marshal(leftover)
		if err != nil {
			return err
		}
		key := leftover.ID + "|" + leftover.Action + "|" + leftover.Role + "|" + leftover.RequestID
		worldState := leftover.Scope
		if len(report.Steps) > 0 {
			worldState = firstNonEmpty(report.Steps[len(report.Steps)-1].After.State, leftover.Scope)
		}
		statements = append(statements, fmt.Sprintf("INSERT OR REPLACE INTO exploration_frontier(id,campaign,equivalence_key,state,role,body) VALUES(%s,%s,%s,%s,%s,%s);", quote(report.Campaign+"/"+shortDigest(key)), quote(report.Campaign), quote(key), quote(worldState), quote(leftover.Role), quote(string(item))))
	}
	return d.tx(statements)
}

func (d db) explorationCoverage(graph string) (map[string]ExplorationCoverage, error) {
	out := map[string]ExplorationCoverage{}
	g, err := d.model(graph)
	if err != nil {
		return out, err
	}
	x := indexModel(g)
	rows, err := d.query("SELECT body FROM exploration_campaigns WHERE graph="+quote(graph)+" ORDER BY created DESC LIMIT 8", 1)
	if err != nil {
		return out, err
	}
	unspecified, err := d.unspecified(graph)
	if err != nil {
		return out, err
	}
	analysis, err := d.analyzeModel(g, "", "")
	if err != nil {
		analysis = ModelAnalysis{Nodes: map[string]BusinessNodeStatus{}}
	}
	for _, capability := range capabilityIDs(g) {
		cover := ExplorationCoverage{Explored: []string{}, Unexplored: []string{}, Unspecified: []string{}, ConfirmedFails: []string{}, CurrentPass: []string{}, PendingRecovery: []string{}, ExecutedPaths: []string{}, Frontier: []string{}, AICandidates: []string{}, FailEvidence: []string{}, RecoveryStatus: []string{}, UnknownStates: []string{}, ModeContrast: []string{}}
		if status, ok := analysis.Nodes[capability]; ok {
			cover.Tested = status.Required > 0 && status.Status != "" && status.Status != "NOT_RUN"
			if status.Status == "PASS" {
				cover.CurrentPass = append(cover.CurrentPass, capability)
			}
			if status.HistoryFailures > 0 {
				cover.ConfirmedFails = append(cover.ConfirmedFails, capability+" historic FAIL")
			}
		}
		out[capability] = cover
	}
	for _, item := range unspecified {
		for _, capability := range item.Capabilities {
			cover := out[capability]
			cover.Unspecified = append(cover.Unspecified, item.ID)
			out[capability] = cover
		}
	}
	for _, row := range rows {
		var report ExploreReport
		if json.Unmarshal([]byte(row[0]), &report) != nil {
			continue
		}
		path := []string{}
		for _, step := range report.Steps {
			capability := step.Candidate.Scope
			if capability == "" {
				capability = firstCapability(x, step.Candidate.Action)
			}
			cover := out[capability]
			cover.Explored = append(cover.Explored, string(step.Candidate.Kind)+":"+step.Candidate.Action)
			path = append(path, string(step.Candidate.Kind)+":"+step.Candidate.Action+":"+step.Candidate.Role)
			if strings.HasPrefix(step.Candidate.RequestID, "ai-") || step.Candidate.Source == sourceInfer && step.Candidate.Kind == kindHistory {
				cover.AICandidates = appendUnique(cover.AICandidates, step.Candidate.Action+":"+string(step.Candidate.Kind))
			}
			for _, finding := range step.Findings {
				if finding.Kind == "FAIL" {
					cover.FailEvidence = appendUnique(cover.FailEvidence, firstNonEmpty(finding.Evidence, step.RAW, finding.Rule))
					cover.ConfirmedFails = appendUnique(cover.ConfirmedFails, finding.Rule)
				}
			}
			if report.RecoveryClass == RecoveryFailedLocked || report.RecoveryClass == RecoveryUnknown {
				cover.PendingRecovery = append(cover.PendingRecovery, report.Campaign)
			}
			if report.Truncated {
				cover.BudgetExhausted = true
				cover.StopReasons = append(cover.StopReasons, report.StopReason)
			}
			if report.RecoveryClass != "" {
				cover.RecoveryStatus = appendUnique(cover.RecoveryStatus, string(report.RecoveryClass))
			}
			if report.Mode != "" {
				cover.ModeContrast = appendUnique(cover.ModeContrast, report.Mode+":"+report.StopReason)
			}
			out[capability] = cover
		}
		if len(path) > 0 {
			joined := strings.Join(path, " -> ")
			for capability, cover := range out {
				if len(cover.Explored) > 0 {
					cover.ExecutedPaths = appendUnique(cover.ExecutedPaths, joined)
					out[capability] = cover
				}
			}
		}
		for _, leftover := range report.Remaining {
			capability := leftover.Scope
			if capability == "" {
				capability = firstCapability(x, leftover.Action)
			}
			cover := out[capability]
			cover.Unexplored = append(cover.Unexplored, string(leftover.Kind)+":"+leftover.Action)
			cover.Frontier = appendUnique(cover.Frontier, string(leftover.Kind)+":"+leftover.Action+":"+leftover.Role)
			cover.StopReasons = appendUnique(cover.StopReasons, report.StopReason)
			out[capability] = cover
		}
		for _, item := range report.Unspecified {
			for _, capability := range item.Capabilities {
				cover := out[capability]
				cover.UnknownStates = appendUnique(cover.UnknownStates, item.Kind+":"+item.AfterState)
				out[capability] = cover
			}
		}
	}
	return out, nil
}

func capabilityIDs(g Graph) []string {
	ids := []string{}
	for _, node := range g.Nodes {
		if node.Kind == "capability" {
			ids = append(ids, node.ID)
		}
	}
	return ids
}

func firstCapability(x modelIndex, action string) string {
	if caps := x.capsByAction[action]; len(caps) > 0 {
		return caps[0]
	}
	return action
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}

type ExplorationCoverage struct {
	Explored        []string `json:"explored_branches"`
	Unexplored      []string `json:"unexplored_branches"`
	Unspecified     []string `json:"unspecified"`
	ConfirmedFails  []string `json:"confirmed_fails"`
	CurrentPass     []string `json:"current_pass"`
	Tested          bool     `json:"tested"`
	PendingRecovery []string `json:"pending_recovery"`
	StopReasons     []string `json:"stop_reasons"`
	BudgetExhausted bool     `json:"budget_exhausted"`
	EvidenceMissing string   `json:"evidence_missing,omitempty"`
	ExecutedPaths   []string `json:"executed_paths,omitempty"`
	Frontier        []string `json:"frontier,omitempty"`
	AICandidates    []string `json:"ai_candidates,omitempty"`
	FailEvidence    []string `json:"fail_evidence,omitempty"`
	RecoveryStatus  []string `json:"recovery_status,omitempty"`
	UnknownStates   []string `json:"unknown_states,omitempty"`
	ModeContrast    []string `json:"mode_contrast,omitempty"`
}
