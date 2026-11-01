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
	MaxStates        int           `json:"max_states"`
	MaxPaths         int           `json:"max_paths"`
	MaxActions       int           `json:"max_actions"`
	MaxDepth         int           `json:"max_depth,omitempty"`
	Deadline         time.Duration `json:"deadline"`
	MaxReplans       int           `json:"max_replans"`
	MaxPlannerCalls  int           `json:"max_planner_calls,omitempty"`
	MaxEvidenceBytes int64         `json:"max_evidence_bytes,omitempty"`
	MaxMemoryBytes   int64         `json:"max_memory_bytes,omitempty"`
	Seed             int           `json:"seed,omitempty"`
}

type ExploreFault struct {
	Window     string        `json:"window,omitempty"`
	Notify     string        `json:"notify,omitempty"`
	HangWindow time.Duration `json:"hang_window,omitempty"`
	Message    string        `json:"message,omitempty"`
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
	Worker                                                string
	Fault                                                 ExploreFault
}

func unknownActionStatus(status string) bool {
	return status == actionDispatching || status == actionExecuting || status == actionOutcomeUnknown || status == actionUnknown
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
	Resource       string `json:"resource,omitempty"`
	Scope          string `json:"scope,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	SideEffect     bool   `json:"side_effect,omitempty"`
	OracleRef      string `json:"oracle_ref,omitempty"`
}

type ExploreStep struct {
	Candidate     CandidateAction        `json:"candidate"`
	Before        IndependentFacts       `json:"before"`
	After         IndependentFacts       `json:"after"`
	Denied        bool                   `json:"denied"`
	RAW           string                 `json:"raw"`
	SHA256        string                 `json:"sha256"`
	Findings      []ExploreFinding       `json:"findings"`
	Discovery     *DiscoveryReport       `json:"discovery,omitempty"`
	Calls         []ExploreCallRecord    `json:"calls,omitempty"`
	Group         *ExploreOperationGroup `json:"group,omitempty"`
	ClassifyError string                 `json:"classify_error,omitempty"`
	OracleFields  []OracleField          `json:"oracle_fields,omitempty"`
}

type ExploreReport struct {
	Campaign         string            `json:"campaign"`
	Graph            string            `json:"graph"`
	Flow             string            `json:"flow"`
	Environment      string            `json:"environment,omitempty"`
	Round            string            `json:"round,omitempty"`
	StopReason       string            `json:"stop_reason"`
	Remaining        []CandidateAction `json:"remaining"`
	Steps            []ExploreStep     `json:"steps"`
	WithinContract   int               `json:"within_contract"`
	Unspecified      []UnspecifiedItem `json:"unspecified"`
	RuleFails        []ExploreFinding  `json:"rule_fails"`
	Risks            []ExploreFinding  `json:"risks"`
	Blocked          []ExploreFinding  `json:"blocked,omitempty"`
	States           int               `json:"states"`
	Actions          int               `json:"actions"`
	Truncated        bool              `json:"truncated"`
	Recovery         RecoveryOutcome   `json:"recovery"`
	RecoveryClass    RecoveryClass     `json:"recovery_class"`
	Spent            ExploreSpent      `json:"spent,omitempty"`
	Mode             string            `json:"mode,omitempty"`
	Checkpoint       string            `json:"checkpoint,omitempty"`
	Boundary         *DiscoveryContext `json:"boundary_discovery,omitempty"`
	Identity         ExecutionIdentity `json:"identity,omitempty"`
	Oracle           string            `json:"oracle,omitempty"`
	EvidenceContract string            `json:"evidence_contract,omitempty"`
	OracleFields     []OracleField     `json:"oracle_fields,omitempty"`
	UnsafeResume     bool              `json:"unsafe_resume,omitempty"`
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
	if b.MaxDepth < 1 {
		b.MaxDepth = b.MaxActions
	}
	if b.MaxDepth > 1024 {
		b.MaxDepth = 1024
	}
	if b.MaxPlannerCalls < 0 {
		b.MaxPlannerCalls = 0
	}
	if b.MaxPlannerCalls > 32 {
		b.MaxPlannerCalls = 32
	}
	if b.MaxEvidenceBytes < 0 {
		b.MaxEvidenceBytes = 0
	}
	if b.MaxMemoryBytes < 0 {
		b.MaxMemoryBytes = 0
	}
	return b
}

func exploreBudgetExceeded(budget ExploreRuntimeBudget, report ExploreReport) (bool, string) {
	if budget.MaxDepth > 0 && report.Actions >= budget.MaxDepth {
		return true, "budget_depth"
	}
	if budget.MaxEvidenceBytes > 0 && report.Spent.EvidenceBytes >= budget.MaxEvidenceBytes {
		return true, "budget_evidence"
	}
	if budget.MaxMemoryBytes > 0 && report.Spent.PeakRSSBytes >= budget.MaxMemoryBytes {
		return true, "budget_memory"
	}
	return false, ""
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
	report.Identity = binding.Identity
	report.Oracle = binding.Oracle
	report.EvidenceContract = exploreIdentityContract
	if err := d.persistExploreBinding(campaign, binding); err != nil {
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
	modelSHA, _ := d.graphFingerprint(request.Graph)
	seed := ExploreCheckpoint{
		Campaign: campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round,
		ModelSHA: modelSHA, SourceCommit: request.Commit, Status: exploreNew, Account: request.Account, Fixture: request.Fixture,
		Mode: request.Mode, Report: report, Budget: budget, Binding: binding,
	}
	if err := d.insertExploreCampaign(seed); err != nil {
		return report, err
	}
	lease, err := d.claimExploreCampaign(campaign, firstNonEmpty(request.Worker, exploreWorkerID()))
	if err != nil {
		return report, err
	}
	drive := &exploreDrive{lease: lease, state: &seed, fault: request.Fault}
	ctx, cancel := context.WithTimeout(parent, budget.Deadline)
	defer cancel()
	fixture, _, err := d.bindingFixture(request.Fixture)
	if err != nil {
		return report, err
	}
	world, err := d.observeWorld(ctx, request.Adapter, binding, accounts["primary"], flow.Initial)
	if err != nil {
		report.StopReason = "blocked"
		return d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, campaign, err, request.Fault)
	}
	seen := map[string]bool{}
	tried := map[string]bool{}
	replans := 0
	plannerCalls := 0
	discovered := []DiscoveredEntry{}
	disc := DiscoveryContext{}
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
	if foundEntries, discErr := discoverSourceEntries(request.Root, request.Adapter); discErr == nil {
		discovered = foundEntries
		for _, item := range foundEntries {
			item = authorizeDiscoveredEntry(item, request.Adapter)
			if item.Class == entryUndeclared || item.Class == entryForbidden {
				report.Risks = append(report.Risks, ExploreFinding{Kind: "RISK", Reason: item.Class + " " + item.Path + " from " + item.File, Evidence: item.Evidence})
			}
		}
	}
	disc = refreshBoundaryDiscovery(disc, g, approvals, request.Adapter, report, discovered)
	report.Boundary = &disc
	if request.Mode == "agent-native" {
		report.StopReason, report.Truncated = "awaiting_agent", true
		report.Checkpoint = newID("CK")
		seed.World, seed.Seen = persistWorld(world), mapKeys(seen)
		seed.Report, seed.Status, seed.Checkpoint = report, exploreInterrupted, report.Checkpoint
		if err := d.saveExploreCheckpointOwned(seed, lease); err != nil { return report, err }
		if err := d.persistExploration(report); err != nil { return report, err }
		return report, nil
	}
	for {
		if ctx.Err() != nil {
			report.StopReason, report.Truncated = "budget_time", true
			break
		}
		if stop, reason := exploreBudgetExceeded(budget, report); stop {
			report.StopReason, report.Truncated = reason, true
			report.Remaining = remainingCandidates(proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried), tried, world.state)
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
			if budget.MaxPlannerCalls > 0 && plannerCalls >= budget.MaxPlannerCalls {
				report.StopReason, report.Truncated = "budget_planner", true
				report.Remaining = remainingCandidates(candidates, tried, world.state)
				break
			}
			planIn := attachDiscoveryToPlanner(plannerContext(request, flow, approvals, world, report, candidates, budget.MaxActions-report.Actions), disc)
			proposal, planErr := request.Planner.Suggest(ctx, planIn)
			plannerCalls++
			report.Spent.PlannerCalls = plannerCalls
			if planErr != nil {
				report.Blocked = append(report.Blocked, ExploreFinding{Kind: "BLOCKED", Reason: "planner unavailable: " + planErr.Error()})
			} else {
				rejected := []string{}
				accepted := []CandidateAction{}
				for _, suggestion := range proposal.Suggestions {
					item, reason := validatePlannerSuggestion(suggestion, request.Adapter, request.Adapter.Accounts, budget.MaxActions-report.Actions)
					if reason != "" {
						rejected = append(rejected, reason)
						continue
					}
					item.ID = shortDigest(string(item.Kind) + "|" + item.Action + "|" + item.Role + "|" + item.RequestID + "|" + world.state)
					if !tried[candidateKey(item, world.state)] {
						accepted = append(accepted, item)
					}
				}
				if err := d.persistPlannerAudit(campaign, planIn, proposal, accepted, rejected, plannerCalls); err != nil {
					report.Blocked = append(report.Blocked, ExploreFinding{Kind: "BLOCKED", Reason: "planner audit persist failed: " + err.Error()})
				} else {
					candidates = append(candidates, accepted...)
				}
			}
		}
		candidates = rotateCandidates(candidates, budget.Seed)
		if request.Mode == "ai" {
			candidates = preferDiscoveryCandidates(candidates, disc, world)
		}
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
		drive.candidate = chosen
		drive.attemptID = ""
		if err := d.ensureExploreLease(drive); err != nil {
			report.StopReason, report.Truncated = "lease_lost", true
			break
		}
		needsReplan := chosen.Kind == kindUndeclared || chosen.Kind == kindSkipPre || chosen.Source == sourceInfer && (world.state == "" || !modelState(g, world.state))
		if needsReplan {
			replans++
			if replans > budget.MaxReplans {
				report.StopReason, report.Truncated = "budget_replans", true
				report.Remaining = remainingCandidates(candidates, tried, world.state)
				break
			}
		}
		drive.candidate = chosen
		step, live, execErr := d.exploreOnce(ctx, binding, request.Adapter, accounts, actors, chosen, campaign, report.Actions+1, world, drive)
		if execErr != nil && errors.Is(execErr, errExploreCrashSentinel) {
			report.UnsafeResume = true
			if drive != nil && drive.state != nil {
				drive.state.UnsafeResume = true
				drive.state.Report.UnsafeResume = true
			}
		}
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
			return d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, campaign, execErr, request.Fault)
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
		disc = refreshBoundaryDiscovery(disc, g, approvals, request.Adapter, report, discovered)
		report.Boundary = &disc
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
		report.Spent = measureExploreSpent(d, report, replans, plannerCalls)
		if err := d.persistExploreDrive(drive, report, world, tried, budget, exploreRunning); err != nil {
			return report, err
		}
	}
	if report.StopReason == "" {
		if len(report.Remaining) > 0 {
			report.StopReason, report.Truncated = "budget_actions", true
		} else {
			report.StopReason = "complete"
		}
	}
	if report.Remaining == nil {
		report.Remaining = remainingCandidates(proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried), tried, world.state)
	}
	if len(report.Remaining) > 0 && report.StopReason == "complete" {
		report.StopReason, report.Truncated = "budget_actions", true
	}
	if len(report.Remaining) > 0 && (report.StopReason == "budget_actions" || report.StopReason == "budget_states" || report.StopReason == "budget_time" || report.StopReason == "budget_replans" || report.StopReason == "budget_planner") {
		report.Truncated = true
	}
	report.Spent = measureExploreSpent(d, report, replans, plannerCalls)
	final, err := d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, campaign, nil, request.Fault)
	if persistErr := d.persistExploreDrive(drive, final, world, tried, budget, exploreStatusFor(final)); persistErr != nil {
		if err != nil {
			return final, errors.Join(err, persistErr)
		}
		return final, persistErr
	}
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
	if state.UnsafeResume {
		return report, errUnreplayableSideEffects
	}
	if state.RecoveryClass == RecoveryFailedLocked {
		return report, errors.New("refusing to continue after failed fixture recovery")
	}
	if state.Status == exploreAborted || state.Status == exploreCompleted {
		return report, fmt.Errorf("campaign %s is %s", state.Campaign, state.Status)
	}
	worker := exploreWorkerID()
	lease, err := d.claimExploreCampaign(state.Campaign, worker)
	if err != nil {
		return report, err
	}
	state.ClaimedBy = lease.Worker
	state.Fence = lease.Fence
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
	if err := d.reconcileUnknownActions(parent, binding, request.Adapter, accounts, &state, lease); err != nil {
		return report, err
	}
	if campaignHoldsUnreplayableSideEffects(state) {
		if err := d.saveExploreCheckpointOwned(state, lease); err != nil {
			return state.Report, err
		}
		return state.Report, errUnreplayableSideEffects
	}
	drive := &exploreDrive{lease: lease, state: &state, fault: request.Fault}
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
	for _, rec := range state.Actions {
		if rec.ReplayForbidden || rec.EvidenceIncomplete || rec.Status == actionBlocked || rec.Status == actionObserved || rec.Status == actionRecorded {
			tried[candidateKey(rec.Candidate, rec.Before.State)] = true
			if rec.Candidate.ID != "" {
				tried[rec.Candidate.ID] = true
			}
			if rec.RequestID != "" {
				tried["request:"+rec.RequestID] = true
			}
		}
	}
	seen := map[string]bool{}
	for _, key := range state.Seen {
		seen[key] = true
	}
	replans := state.Spent.Replans
	plannerCalls := report.Spent.PlannerCalls
	if plannerCalls == 0 {
		plannerCalls = state.Spent.PlannerCalls
	}
	disc := DiscoveryContext{}
	if loaded, loadErr := d.loadDiscoveryContext(state.Campaign); loadErr == nil {
		disc = loaded
	} else if state.Report.Boundary != nil {
		disc = *state.Report.Boundary
	}
	discovered := []DiscoveredEntry{}
	if foundEntries, discErr := discoverSourceEntries(request.Root, request.Adapter); discErr == nil {
		discovered = foundEntries
	}
	report = state.Report
	report.Mode = firstNonEmpty(request.Mode, state.Mode)
	disc = refreshBoundaryDiscovery(disc, g, approvals, request.Adapter, report, discovered)
	report.Boundary = &disc
	for {
		if ctx.Err() != nil {
			report.StopReason, report.Truncated = "budget_time", true
			break
		}
		if stop, reason := exploreBudgetExceeded(budget, report); stop {
			report.StopReason, report.Truncated = reason, true
			report.Remaining = remainingCandidates(proposeCandidates(g, x, flow, request.Adapter, approvals, world, tried), tried, world.state)
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
		if request.Mode == "agent-native" {
			if len(state.Frontier) != 1 { return report, errors.New("no single approved agent proposal; use discover propose") }
			candidates = append([]CandidateAction(nil), state.Frontier...)
		}
		if request.Mode == "deterministic" {
			candidates = filterDeterministic(candidates)
		}
		if shouldAskPlanner(request.Mode, candidates, report, world) && request.Planner != nil {
			if budget.MaxPlannerCalls > 0 && plannerCalls >= budget.MaxPlannerCalls {
				report.StopReason, report.Truncated = "budget_planner", true
				report.Remaining = remainingCandidates(candidates, tried, world.state)
				break
			}
			planIn := attachDiscoveryToPlanner(plannerContext(request, flow, approvals, world, report, candidates, budget.MaxActions-report.Actions), disc)
			proposal, planErr := request.Planner.Suggest(ctx, planIn)
			plannerCalls++
			report.Spent.PlannerCalls = plannerCalls
			if planErr != nil {
				report.Blocked = append(report.Blocked, ExploreFinding{Kind: "BLOCKED", Reason: "planner unavailable: " + planErr.Error()})
			} else {
				rejected := []string{}
				accepted := []CandidateAction{}
				for _, suggestion := range proposal.Suggestions {
					item, reason := validatePlannerSuggestion(suggestion, request.Adapter, request.Adapter.Accounts, budget.MaxActions-report.Actions)
					if reason != "" {
						rejected = append(rejected, reason)
						continue
					}
					item.ID = shortDigest(string(item.Kind) + "|" + item.Action + "|" + item.Role + "|" + item.RequestID + "|" + world.state)
					if !tried[candidateKey(item, world.state)] {
						accepted = append(accepted, item)
					}
				}
				if err := d.persistPlannerAudit(state.Campaign, planIn, proposal, accepted, rejected, plannerCalls); err != nil {
					report.Blocked = append(report.Blocked, ExploreFinding{Kind: "BLOCKED", Reason: "planner audit persist failed: " + err.Error()})
				} else {
					candidates = append(candidates, accepted...)
				}
			}
		}
		candidates = rotateCandidates(candidates, budget.Seed)
		if request.Mode == "ai" {
			candidates = preferDiscoveryCandidates(candidates, disc, world)
		}
		chosen, ok := selectCandidate(candidates, tried, world.state)
		if !ok {
			if request.Mode != "agent-native" && len(state.Frontier) > 0 {
				chosen, ok = selectCandidate(state.Frontier, tried, world.state)
			}
			if !ok {
				if request.Mode == "agent-native" { return report, errors.New("proposal already tried or stale; no replay") }
				report.StopReason = "complete"
				report.Remaining = candidates
				break
			}
		}
		tried[candidateKey(chosen, world.state)] = true
		tried[chosen.ID] = true
		drive.candidate = chosen
		drive.attemptID = ""
		if err := d.ensureExploreLease(drive); err != nil {
			report.StopReason, report.Truncated = "lease_lost", true
			break
		}
		step, live, execErr := d.exploreOnce(ctx, binding, request.Adapter, accounts, actors, chosen, state.Campaign, report.Actions+1, world, drive)
		if execErr != nil && errors.Is(execErr, errExploreCrashSentinel) {
			report.UnsafeResume = true
			state.UnsafeResume = true
		}
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
			if request.Mode == "agent-native" {
				report.StopReason, report.Truncated, report.UnsafeResume = "blocked", true, true
				report.Remaining = nil
				report.Checkpoint = newID("CK")
				state.UnsafeResume = true
				if saveErr := d.persistExploreDrive(drive, report, world, tried, budget, exploreBlocked); saveErr != nil { return report, errors.Join(execErr, saveErr) }
				if saveErr := d.persistExploration(report); saveErr != nil { return report, errors.Join(execErr, saveErr) }
				return report, execErr
			}
			return d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, state.Campaign, execErr, request.Fault)
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
		disc = refreshBoundaryDiscovery(disc, g, approvals, request.Adapter, report, discovered)
		report.Boundary = &disc
		_ = replans
		if request.Mode == "agent-native" {
			report.StopReason, report.Truncated = "awaiting_agent", true
			report.Remaining = nil
			report.Checkpoint = newID("CK")
			if err := d.persistExploreDrive(drive, report, world, tried, budget, exploreInterrupted); err != nil { return report, err }
			if err := d.persistExploration(report); err != nil { return report, err }
			return report, nil
		}
		if err := d.persistExploreDrive(drive, report, world, tried, budget, exploreRunning); err != nil {
			return report, err
		}
	}
	if report.StopReason == "" {
		if len(report.Remaining) > 0 {
			report.StopReason, report.Truncated = "budget_actions", true
		} else {
			report.StopReason = "complete"
		}
	}
	final, err := d.finishExplore(ctx, report, binding, request.Adapter, accounts, actors, fixture, state.Campaign, nil, request.Fault)
	if persistErr := d.persistExploreDrive(drive, final, world, tried, budget, exploreStatusFor(final)); persistErr != nil {
		if err != nil {
			return final, errors.Join(err, persistErr)
		}
		return final, persistErr
	}
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

func (d db) persistExploreBinding(run string, binding RunBinding) error {
	if run == "" {
		return errors.New("explore binding needs a campaign")
	}
	body, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	return d.tx([]string{fmt.Sprintf("INSERT OR REPLACE INTO run_bindings(run,body) VALUES(%s,%s);", quote(run), quote(string(body)))})
}

func (d db) loadExploreBinding(run string) (RunBinding, error) {
	var binding RunBinding
	rows, err := d.query("SELECT body FROM run_bindings WHERE run="+quote(run), 1)
	if err != nil {
		return binding, err
	}
	if len(rows) != 1 {
		return binding, errors.New("explore campaign has no recorded execution binding")
	}
	if err := json.Unmarshal([]byte(rows[0][0]), &binding); err != nil {
		return binding, err
	}
	return binding, nil
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
	history := map[string]bool{}
	for id, raw := range world.history {
		if string(raw) == "true" || string(raw) == "1" {
			history[id] = true
		} else if string(raw) == "false" || string(raw) == "0" || string(raw) == "null" || string(raw) == "" {
			history[id] = false
		} else {
			history[id] = true
		}
	}
	key, basis, reason := equivalenceKey(g, EquivalenceInput{
		Flow: flow.ID, State: world.state, Role: world.role, Principal: world.principal,
		Conditions: world.conditions, History: history, Measurements: world.history,
	})
	if reason != "" {
		return "", false
	}
	if world.lastTO || world.lastDenied {
		basis = append(basis, fmt.Sprintf("last_denied=%t", world.lastDenied), fmt.Sprintf("last_to=%t", world.lastTO))
		key = key + "|denied=" + fmt.Sprint(world.lastDenied) + "|timeout=" + fmt.Sprint(world.lastTO)
	}
	if world.lastAction != "" {
		key += "|last=" + world.lastAction
	}
	return key, basis != nil
}

func (d db) exploreOnce(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, actors map[string]ParticipantBinding, candidate CandidateAction, run string, index int, world exploreWorld, drive *exploreDrive) (ExploreStep, exploreWorld, error) {
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
	if err := d.markExploreAction(drive, actionPlanned, before); err != nil {
		return step, live, err
	}
	if err := crashAt(faultOf(drive), "before_dispatch"); err != nil {
		_ = d.markExploreAction(drive, actionClaimed, before)
		return step, live, err
	}
	if candidate.Kind == kindRecover {
		if err := crashAt(faultOf(drive), "during_recovery"); err != nil {
			return step, live, err
		}
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
	candidate, authorized := authorizeExploreCandidate(adapter, candidate)
	step.Candidate = candidate
	if drive != nil {
		drive.candidate = candidate
	}
	if !authorized {
		step.Findings = append(step.Findings, ExploreFinding{Kind: "BLOCKED", Reason: "undeclared entry is not authorized: " + candidate.Entry})
		if err := d.markExploreAction(drive, actionBlocked, before); err != nil {
			return step, live, err
		}
		return step, live, errors.New("undeclared entry is not authorized")
	}
	if err := d.markExploreAction(drive, actionDispatching, before); err != nil {
		return step, live, err
	}
	if err := crashAt(faultOf(drive), "during_dispatch"); err != nil {
		_ = d.markExploreAction(drive, actionOutcomeUnknown, before)
		return step, live, err
	}
	output, calls, group, callErr := d.performExplore(ctx, binding, adapter, accounts, candidate)
	step.Calls = calls
	if group.ID != "" {
		step.Group = &group
		step.Findings = append(step.Findings, groupFindings(group)...)
	}
	if callErr != nil && !isTimeout(callErr) && !errors.Is(callErr, errOutcomeUnknown) {
		return step, live, callErr
	}
	if err := crashAt(faultOf(drive), "after_command_observed"); err != nil {
		_ = d.markExploreAction(drive, actionOutcomeUnknown, before)
		return step, live, err
	}
	if isTimeout(callErr) || errors.Is(callErr, errOutcomeUnknown) || faultOf(drive).Window == "after_send_timeout" {
		live.lastTO = true
		if err := d.markExploreAction(drive, actionOutcomeUnknown, before); err != nil {
			return step, live, err
		}
		step.Findings = append(step.Findings, ExploreFinding{Kind: "BLOCKED", Reason: "request completed with an unknown outcome; side-effecting replay is forbidden"})
		if faultOf(drive).Window == "after_send_timeout" {
			return step, live, errExploreCrash("after_send_timeout")
		}
	}
	if err := crashAt(faultOf(drive), "before_oracle"); err != nil {
		_ = d.markExploreAction(drive, actionOutcomeUnknown, before)
		return step, live, err
	}
	step.Denied = output.Denied
	afterCtx, cancelAfter := context.WithTimeout(ctx, stepTimeout)
	after, afterHash, err := readOracle(afterCtx, adapter.Oracle, binding, oracleAccount, adapter.IsolationScope)
	cancelAfter()
	if err != nil {
		_ = d.markExploreAction(drive, actionOutcomeUnknown, before)
		return step, live, err
	}
	step.After = after
	step.OracleFields = classifyOracleFacts(after, adapter.Oracle, binding.Identity.Target, candidate.Role, false)
	if drive != nil {
		drive.after = after
		copied := step
		drive.step = &copied
	}
	if err := d.markExploreAction(drive, actionObserved, before); err != nil {
		return step, live, err
	}
	if err := crashAt(faultOf(drive), "after_observed_before_raw"); err != nil {
		return step, live, err
	}
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
	if err := crashAt(faultOf(drive), "after_raw_before_db"); err != nil {
		_ = d.markExploreAction(drive, actionOutcomeUnknown, before)
		return step, live, err
	}
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
	if err := d.markExploreAction(drive, actionRecorded, before); err != nil {
		return step, live, err
	}
	return step, live, nil
}

func faultOf(drive *exploreDrive) ExploreFault {
	if drive == nil {
		return ExploreFault{}
	}
	return drive.fault
}

func (d db) performExplore(ctx context.Context, binding RunBinding, adapter ExploreAdapter, accounts map[string]Fixture, candidate CandidateAction) (AdapterOutput, []ExploreCallRecord, ExploreOperationGroup, error) {
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
		return aggregateExploreGroup("concurrent", partners, []struct {
			out AdapterOutput
			err error
		}{results[0], results[1]})
	}
	callCtx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	output, err := performCall(callCtx, exploreCall(adapter, candidate), binding, accountFor(accounts, candidate))
	return aggregateExploreGroup("single", []CandidateAction{candidate}, []struct {
		out AdapterOutput
		err error
	}{{output, err}})
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

func authorizeExploreCandidate(adapter ExploreAdapter, candidate CandidateAction) (CandidateAction, bool) {
	if candidate.Kind == kindRecover {
		return candidate, true
	}
	for _, entry := range adapter.Entries {
		if entry.Entry == candidate.Entry && (entry.Action == "" || entry.Action == candidate.Action) {
			return candidate, true
		}
	}
	for _, entry := range adapter.Entries {
		if entry.Action == candidate.Action && entry.Entry != "" {
			candidate.Entry = entry.Entry
			return candidate, true
		}
	}
	return candidate, false
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
	stampRunnerIdentity(&value)
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
	if err := d.tx(statements); err != nil {
		return err
	}
	if report.Boundary != nil {
		return d.persistDiscoveryContext(report.Campaign, *report.Boundary)
	}
	return nil
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
			for _, finding := range step.Findings {
				if finding.Kind == "PASS" && finding.Rule != "" {
					cover.ConfirmedRules = appendUnique(cover.ConfirmedRules, finding.Rule)
					cover.HumanConfirmed = appendUnique(cover.HumanConfirmed, finding.Rule)
				}
				if finding.Kind == "RISK" {
					cover.Hypotheses = appendUnique(cover.Hypotheses, finding.Reason)
					if step.Candidate.Source == sourceInfer || strings.HasPrefix(step.Candidate.RequestID, "ai-") {
						cover.AIInferred = appendUnique(cover.AIInferred, finding.Reason)
					}
				}
			}
			if cover.FirstDivergence == "" {
				for _, finding := range step.Findings {
					if finding.Kind == "FAIL" {
						cover.FirstDivergence = firstNonEmpty(finding.Rule, finding.Reason)
						break
					}
				}
			}
			verified := d.verifyExploreEvidence(ExploreCheckpoint{Campaign: report.Campaign, Graph: report.Graph, Report: report, SourceCommit: report.Identity.SourceCommit, ModelSHA: report.Identity.GraphSHA256, Fixture: report.Identity.Fixture, Environment: report.Environment, Mode: report.Mode, Binding: RunBinding{Identity: report.Identity, Oracle: report.Oracle}})
			cover.BusinessVerdict = verified.BusinessVerdict
			cover.EvidenceTrust = verified.EvidenceTrust
			cover.Trusted = cover.EvidenceTrust == trustVerified
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
		if report.Boundary != nil {
			for capability, cover := range out {
				out[capability] = attachBoundaryCoverage(cover, *report.Boundary)
			}
		}
	}
	return out, nil
}

func attachBoundaryCoverage(cover ExplorationCoverage, disc DiscoveryContext) ExplorationCoverage {
	for _, fact := range disc.Facts {
		if fact.Class == factKnown {
			cover.KnownFacts = appendUnique(cover.KnownFacts, fact.ID)
		}
		if fact.Class == factBusiness || fact.Class == factConflicting {
			cover.NeedsDecision = appendUnique(cover.NeedsDecision, fact.ID)
		}
	}
	for _, hyp := range disc.Hypotheses {
		cover.HypothesisStatus = appendUnique(cover.HypothesisStatus, hyp.ID+":"+hyp.Status)
		cover.Hypotheses = appendUnique(cover.Hypotheses, hyp.Counterfactual)
		cover.CounterfactualPaths = appendUnique(cover.CounterfactualPaths, hyp.Identity.Action+":"+hyp.Counterfactual)
		cover.HypothesisHistory = appendUnique(cover.HypothesisHistory, hyp.ID+":v"+fmt.Sprint(hyp.Version)+":"+hyp.Status)
	}
	for _, item := range disc.Explanations {
		cover.Explanations = appendUnique(cover.Explanations, item.ID+":"+firstNonEmpty(item.Status, "UNTESTED")+":"+item.Oracle)
	}
	for _, item := range disc.Experiments {
		cover.Experiments = appendUnique(cover.Experiments, item.ID)
	}
	for _, item := range disc.Revisions {
		cover.Revisions = appendUnique(cover.Revisions, fmt.Sprintf("v%d", item.Version))
	}
	return cover
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
	Explored            []string `json:"explored_branches"`
	Unexplored          []string `json:"unexplored_branches"`
	Unspecified         []string `json:"unspecified"`
	ConfirmedFails      []string `json:"confirmed_fails"`
	CurrentPass         []string `json:"current_pass"`
	Tested              bool     `json:"tested"`
	PendingRecovery     []string `json:"pending_recovery"`
	StopReasons         []string `json:"stop_reasons"`
	BudgetExhausted     bool     `json:"budget_exhausted"`
	EvidenceMissing     string   `json:"evidence_missing,omitempty"`
	ExecutedPaths       []string `json:"executed_paths,omitempty"`
	Frontier            []string `json:"frontier,omitempty"`
	AICandidates        []string `json:"ai_candidates,omitempty"`
	FailEvidence        []string `json:"fail_evidence,omitempty"`
	RecoveryStatus      []string `json:"recovery_status,omitempty"`
	UnknownStates       []string `json:"unknown_states,omitempty"`
	ModeContrast        []string `json:"mode_contrast,omitempty"`
	ConfirmedRules      []string `json:"confirmed_rules,omitempty"`
	Hypotheses          []string `json:"hypotheses,omitempty"`
	FirstDivergence     string   `json:"first_divergence,omitempty"`
	Trusted             bool     `json:"trusted,omitempty"`
	BusinessVerdict     string   `json:"business_verdict,omitempty"`
	EvidenceTrust       string   `json:"evidence_trust,omitempty"`
	HumanConfirmed      []string `json:"human_confirmed,omitempty"`
	AIInferred          []string `json:"ai_inferred,omitempty"`
	KnownFacts          []string `json:"known_facts,omitempty"`
	HypothesisStatus    []string `json:"hypothesis_status,omitempty"`
	Explanations        []string `json:"explanations,omitempty"`
	Experiments         []string `json:"experiments,omitempty"`
	Revisions           []string `json:"revisions,omitempty"`
	NeedsDecision       []string `json:"needs_decision,omitempty"`
	CounterfactualPaths []string `json:"counterfactual_paths,omitempty"`
	HypothesisHistory   []string `json:"hypothesis_history,omitempty"`
}
