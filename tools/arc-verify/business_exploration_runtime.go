package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// This authorization is private to the campaign engine. Public execute cannot
// nominate a run ID or reopen a PASS task. The durable one-shot intent is checked
// again inside startBoundRun's transaction before a fixture can be claimed.
type businessCampaignExecution struct {
	Intent businessCampaignDispatch
	SHA256 string
	Lease  businessCampaignLease
}

type businessCampaignDispatch struct {
	Campaign        string                    `json:"campaign"`
	LedgerRevision  int                       `json:"ledger_revision"`
	Sequence        int                       `json:"sequence"`
	Run             string                    `json:"run"`
	VariantID       string                    `json:"variant_id"`
	Branch          string                    `json:"branch"`
	Runner          string                    `json:"runner"`
	Task            string                    `json:"task"`
	Flow            string                    `json:"flow"`
	Round           string                    `json:"round"`
	Binding         RunBinding                `json:"binding"`
	Participants    []ParticipantBinding      `json:"participants"`
	RunnerSHA256    string                    `json:"runner_sha256"`
	CodeSHA256      string                    `json:"code_sha256"`
	Candidate       BusinessCampaignCandidate `json:"candidate"`
	HistorySHA256   string                    `json:"history_sha256"`
	ReservedActions int                       `json:"reserved_actions"`
	Owner           string                    `json:"owner"`
	Fence           int64                     `json:"fence"`
	Created         string                    `json:"created"`
}

type businessCampaignVariantRuntime struct {
	Variant      BusinessCampaignVariant
	Runner       RegisteredRunner
	Obligation   ModelObligation
	Binding      RunBinding
	Participants []ParticipantBinding
	Accounts     map[string]Fixture
	UpperActions int
}

func validateBusinessCampaignRequest(request BusinessCampaignRequest) error {
	if request.Graph == "" || request.Flow == "" || request.Environment == "" || request.Round == "" || request.Commit == "" || request.Root == "" || request.Config == "" || request.Account == "" || request.Fixture == "" {
		return errors.New("campaign needs graph, flow, environment, round, commit, source root, config, account and fixture")
	}
	if len(request.Variants) < 1 || len(request.Variants) > 64 {
		return errors.New("campaign requires 1..64 explicitly registered branch/input variants")
	}
	if request.Budget.MaxActions < 1 || request.Budget.MaxActions > 1024 || request.Budget.MaxStates < 1 || request.Budget.MaxStates > 10000 || request.Budget.DeadlineMS < 1 || request.Budget.DeadlineMS > 3600000 {
		return errors.New("campaign requires max_actions 1..1024, max_states 1..10000 and deadline_ms 1..3600000")
	}
	seen := map[string]bool{}
	for _, variant := range request.Variants {
		if variant.ID == "" || len(variant.ID) > 128 || variant.Branch == "" || variant.Runner == "" || seen[variant.ID] {
			return errors.New("campaign variant identities must be unique and bind an explicit branch and registered runner")
		}
		seen[variant.ID] = true
		if variant.Task != "" || variant.Action != "" || variant.Actor != "" || variant.RunnerSHA256 != "" || variant.CodeSHA256 != "" || variant.Visits != 0 || variant.Passes != 0 || variant.LastRun != "" {
			return errors.New("campaign input cannot supply computed task, runner hashes, visits, passes or child-run history")
		}
	}
	return nil
}

func (d db) businessCampaignRuntimes(request BusinessCampaignRequest, plan bool) (Graph, []businessCampaignVariantRuntime, error) {
	if err := validateBusinessCampaignRequest(request); err != nil {
		return Graph{}, nil, err
	}
	g, err := d.model(request.Graph)
	if err != nil {
		return g, nil, err
	}
	if g.BusinessContract == nil {
		return g, nil, errors.New("typed campaigns require business_contract; legacy discovery remains a separate command")
	}
	if err := ValidateBusinessContract(g); err != nil {
		return g, nil, err
	}
	if plan {
		if _, err := d.plan(g.ID, 1024); err != nil {
			return g, nil, err
		}
	}
	branches := map[string]BusinessBranch{}
	for _, branch := range g.BusinessContract.Branches {
		branches[branch.ID] = branch
	}
	runtimes := make([]businessCampaignVariantRuntime, 0, len(request.Variants))
	var scope ExecutionIdentity
	for _, variant := range request.Variants {
		runner, err := d.registeredRunner(variant.Runner)
		if err != nil {
			return g, nil, err
		}
		m := runner.Manifest
		obligation, err := validateManifest(g, m)
		if err != nil {
			return g, nil, err
		}
		if m.Graph != g.ID || (obligation.Definition.Flow != "" && obligation.Definition.Flow != request.Flow) || len(m.Steps) != 1 || m.Steps[0].ParallelGroup != "" || len(m.Setup) != 0 {
			return g, nil, errors.New("campaign variants must be single-target registered runners in the selected flow, without static setup or parallel batches; bounded automatic preparation is supported")
		}
		step := m.Steps[0]
		x := indexModel(g)
		flow, flowExists := x.flows[request.Flow]
		inFlow := false
		for _, capability := range x.capsByAction[step.Node] {
			inFlow = inFlow || flowHasCapability(flow, capability)
		}
		if !flowExists || !inFlow {
			return g, nil, errors.New("campaign action does not belong to its explicitly selected declared flow")
		}
		branch, exists := branches[variant.Branch]
		if !exists || !branch.Confirmed || branch.Action != step.Node || (branch.Flow != "" && branch.Flow != request.Flow) || step.BusinessBranch != variant.Branch {
			return g, nil, fmt.Errorf("variant %s must bind its runner's explicit, confirmed business_branch in this flow", variant.ID)
		}
		binding, err := d.startBinding(obligation.Task.ID, request.Environment, request.Commit, request.Account, request.Fixture, request.Root, request.Config)
		if err != nil {
			return g, nil, err
		}
		participants, err := d.participantBindings(obligation.Definition, binding, request.Accounts)
		if err != nil {
			return g, nil, err
		}
		record := ModelRunRecord{Round: request.Round, Runner: variant.Runner, RunnerSHA256: runner.SHA256, CodeSHA256: runner.CodeSHA256, Participants: participants}
		if err := d.validateModelRecord(g, obligation, binding, record); err != nil {
			return g, nil, err
		}
		fixture, _, err := d.bindingFixture(request.Fixture)
		if err != nil {
			return g, nil, err
		}
		if fixture.Refs["scope"] != m.IsolationScope || m.Recovery != nil && fixtureRecoveryMode(m.Recovery) == recoveryRestore && fixture.State != m.Recovery.State {
			return g, nil, errors.New("campaign runner recovery/isolation differs from its registered fixture")
		}
		identity := binding.Identity
		identity.Task, identity.Actor = "", ""
		if len(runtimes) > 0 && identity != scope {
			return g, nil, errors.New("campaign variants cannot cross source, environment, account or fixture identities")
		}
		scope = identity
		variant.Task, variant.Action, variant.Actor = obligation.Task.ID, step.Node, step.Actor
		variant.RunnerSHA256, variant.CodeSHA256 = runner.SHA256, runner.CodeSHA256
		runtime := businessCampaignVariantRuntime{Variant: variant, Runner: runner, Obligation: obligation, Binding: binding, Participants: participants, Accounts: map[string]Fixture{}, UpperActions: 1}
		for _, participant := range participants {
			account, _, err := d.bindingFixture(participant.Account)
			if err != nil {
				return g, nil, err
			}
			runtime.Accounts[participant.Alias] = account
		}
		if m.Preparation != nil {
			runtime.UpperActions += m.Preparation.MaxSteps
		}
		if m.Recovery != nil {
			runtime.UpperActions += len(m.Recovery.Steps)
		}
		runtimes = append(runtimes, runtime)
	}
	return g, runtimes, nil
}

func businessCampaignCoverage(g Graph, variants []BusinessCampaignVariant, flow string) BusinessCampaignCoverage {
	coverage := BusinessCampaignCoverage{SelectedVariants: len(variants), UnmappedBranches: []string{}}
	bound := map[string]bool{}
	for _, variant := range variants {
		bound[variant.Branch] = true
		if variant.Visits > 0 {
			coverage.AttemptedVariants++
		}
		if variant.Passes > 0 {
			coverage.PassedVariants++
		}
	}
	for _, branch := range g.BusinessContract.Branches {
		if branch.Flow != "" && branch.Flow != flow {
			continue
		}
		coverage.DeclaredBranches++
		if bound[branch.ID] {
			coverage.BoundBranches++
		} else {
			coverage.UnmappedBranches = append(coverage.UnmappedBranches, branch.ID)
		}
	}
	sort.Strings(coverage.UnmappedBranches)
	coverage.SelectedVariantsCovered = len(variants) > 0 && coverage.PassedVariants == len(variants)
	// A finite selected input set is never proof of complete business coverage.
	coverage.FullBusinessCoverage = false
	return coverage
}

type businessCampaignHooks struct {
	afterIntent  func(BusinessCampaign, *businessCampaignExecution) error
	afterExecute func(BusinessCampaign, *businessCampaignExecution) error
}

func (d db) runBusinessCampaign(ctx context.Context, request BusinessCampaignRequest) (BusinessCampaign, error) {
	return d.runBusinessCampaignWithHooks(ctx, request, nil)
}

// Hooks are private fault-injection seams for real crash-boundary regression;
// neither the CLI request nor stored JSON can supply them.
func (d db) runBusinessCampaignWithHooks(ctx context.Context, request BusinessCampaignRequest, hooks *businessCampaignHooks) (BusinessCampaign, error) {
	g, runtimes, err := d.businessCampaignRuntimes(request, true)
	if err != nil {
		return BusinessCampaign{Status: "BLOCKED", EvidenceStatus: "BLOCKED", Reason: err.Error()}, err
	}
	if request.Campaign == "" {
		request.Campaign = newID("BC")
	}
	c := BusinessCampaign{Version: 1, ID: request.Campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round, Status: "RUNNING", EvidenceStatus: "PASS", Identity: runtimes[0].Binding.Identity, Request: request, Budget: request.Budget, Frontier: []BusinessCampaignCandidate{}, Visits: []BusinessCampaignVisit{}}
	for _, runtime := range runtimes {
		c.Variants = append(c.Variants, runtime.Variant)
	}
	c.Coverage = businessCampaignCoverage(g, c.Variants, c.Flow)
	c.HistorySHA256, _ = businessJSONHash(struct {
		Identity ExecutionIdentity
		Request  BusinessCampaignRequest
	}{c.Identity, request})
	deadline := time.Now().Add(time.Duration(c.Budget.DeadlineMS) * time.Millisecond)
	lease, err := d.createBusinessCampaign(&c, deadline.Add(5*time.Second))
	if err != nil {
		return c, err
	}
	defer d.releaseBusinessCampaign(c.ID, lease)
	bounded, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	return d.driveBusinessCampaign(bounded, c, g, runtimes, lease, hooks)
}

func mergeBusinessCampaignResume(frozen, supplied BusinessCampaignRequest) error {
	if supplied.Campaign != frozen.Campaign {
		return errors.New("resume campaign identity differs from its frozen request")
	}
	left, right := reflect.ValueOf(frozen), reflect.ValueOf(supplied)
	for i := 0; i < left.NumField(); i++ {
		if right.Field(i).IsZero() {
			continue
		}
		if !reflect.DeepEqual(left.Field(i).Interface(), right.Field(i).Interface()) {
			return fmt.Errorf("resume cannot change frozen campaign field %s", left.Type().Field(i).Name)
		}
	}
	return nil
}

func (d db) resumeBusinessCampaign(ctx context.Context, request BusinessCampaignRequest) (BusinessCampaign, error) {
	c, err := d.readBusinessCampaign(request.Campaign)
	if err != nil {
		return c, err
	}
	if err := mergeBusinessCampaignResume(c.Request, request); err != nil {
		return c, err
	}
	if c.Status == "ABORTED" || c.Status == "COMPLETED_IN_SCOPE" || c.Status == "MODEL_ERROR" || c.Status == "SPECIFICATION_GAP" {
		return d.loadBusinessCampaign(c.ID)
	}
	checked, verifyErr := d.verifyBusinessCampaignEvidence(c)
	if verifyErr != nil {
		return checked, verifyErr
	}
	if checked.EvidenceStatus == "BLOCKED" && len(checked.Variants) == 0 {
		return checked, errors.New(checked.Reason)
	}
	// Source, runner registry, round, account and fixture changes invalidate a
	// resume before any read or write reaches the SUT.
	g, runtimes, err := d.businessCampaignRuntimes(c.Request, false)
	if err != nil {
		return blockedBusinessCampaign(c, err.Error()), err
	}
	for i := range runtimes {
		if businessCampaignStaticVariant(c.Variants[i]) != runtimes[i].Variant {
			return blockedBusinessCampaign(c, "frozen runner variant changed"), errors.New("frozen runner variant changed")
		}
	}
	created, err := time.Parse(time.RFC3339Nano, c.Created)
	if err != nil {
		return c, err
	}
	deadline := created.Add(time.Duration(c.Budget.DeadlineMS) * time.Millisecond)
	// A short lease allows recording the expired stop without extending its
	// execution deadline. A resume never resets action/state/time budgets.
	leaseUntil := deadline.Add(5 * time.Second)
	if !leaseUntil.After(time.Now()) {
		leaseUntil = time.Now().Add(5 * time.Second)
	}
	lease, err := d.claimBusinessCampaign(c, leaseUntil)
	if err != nil {
		return c, err
	}
	defer d.releaseBusinessCampaign(c.ID, lease)
	if err := d.reconcileBusinessCampaign(&c, g, lease); err != nil {
		return c, err
	}
	if c.Status == "UNKNOWN" || c.Status == "DIVERGED" || c.Status == "BLOCKED" && c.FirstDivergence != "" {
		return c, nil
	}
	c.Status, c.Reason = "RUNNING", "resuming from fresh independent observations; no captured session is inherited"
	if err := d.saveBusinessCampaign(&c, lease, "RESUME"); err != nil {
		return c, err
	}
	bounded, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	return d.driveBusinessCampaign(bounded, c, g, runtimes, lease)
}

func businessCampaignElapsed(c BusinessCampaign) int64 {
	started, err := time.Parse(time.RFC3339Nano, c.Created)
	if err != nil {
		return 0
	}
	return time.Since(started).Milliseconds()
}

func (d db) stopBusinessCampaign(c *BusinessCampaign, lease businessCampaignLease, status, reason string) (BusinessCampaign, error) {
	c.Status, c.Reason = status, reason
	c.Spent.ElapsedMS = businessCampaignElapsed(*c)
	if status == "UNKNOWN" {
		c.EvidenceStatus = "BLOCKED"
	}
	err := d.saveBusinessCampaign(c, lease, "STOP")
	return *c, err
}

func (d db) driveBusinessCampaign(ctx context.Context, c BusinessCampaign, g Graph, runtimes []businessCampaignVariantRuntime, lease businessCampaignLease, hookOptions ...*businessCampaignHooks) (BusinessCampaign, error) {
	var hooks *businessCampaignHooks
	if len(hookOptions) > 0 {
		hooks = hookOptions[0]
	}
	for {
		c.Coverage = businessCampaignCoverage(g, c.Variants, c.Flow)
		if c.Coverage.SelectedVariantsCovered {
			return d.stopBusinessCampaign(&c, lease, "COMPLETED_IN_SCOPE", "all explicitly selected branch/input variants have verified child-run PASS evidence; this is not full business coverage")
		}
		if ctx.Err() != nil {
			return d.stopBusinessCampaign(&c, lease, "BUDGET", "campaign deadline/cancellation reached before another dispatch")
		}
		if err := d.validateBusinessCampaignLease(c.ID, lease); err != nil {
			return c, err
		}
		if c.Spent.Actions+c.Spent.ReservedActions >= c.Budget.MaxActions || c.Spent.States >= c.Budget.MaxStates {
			return d.stopBusinessCampaign(&c, lease, "BUDGET", "bounded action or observed-context budget exhausted")
		}
		frontier := make([]BusinessCampaignCandidate, 0, len(runtimes))
		selected := -1
		stop := "BLOCKED"
		for i, runtime := range runtimes {
			candidate := BusinessCampaignCandidate{VariantID: runtime.Variant.ID, Branch: runtime.Variant.Branch, Runner: runtime.Variant.Runner, Run: newID("R"), Eligibility: "BUDGET", Reason: "observed-context budget exhausted", NextAllowed: len(c.NextAllowed) == 0 || containsBusinessID(c.NextAllowed, runtime.Variant.Action)}
			if !candidate.NextAllowed {
				candidate.Eligibility, candidate.Reason = "NEXT_DISALLOWED", "previous verified actual outcome restricts the next action"
			} else if c.Spent.States < c.Budget.MaxStates && ctx.Err() == nil {
				candidate = d.probeBusinessCampaign(ctx, c, g, runtime, candidate.Run, lease)
				if candidate.Observation != nil {
					c.Spent.States++
					c.Spent.Observations++
				}
			}
			if candidate.Eligibility == "MODEL_ERROR" {
				stop = "MODEL_ERROR"
			}
			if candidate.Eligibility == "SPECIFICATION_GAP" && stop != "MODEL_ERROR" {
				stop = "SPECIFICATION_GAP"
			}
			if candidate.Eligibility == "READY" || candidate.Eligibility == "PREPARABLE" {
				if c.Spent.Actions+c.Spent.ReservedActions+runtime.UpperActions > c.Budget.MaxActions || c.Spent.States+1 > c.Budget.MaxStates {
					candidate.Eligibility, candidate.Reason = "BUDGET", "remaining budget cannot reserve the target, all bounded preparation/recovery attempts and the actual post-state"
				} else if selected < 0 || businessCampaignPrefer(c.Variants[i], c.Variants[selected]) {
					selected = i
				}
			}
			frontier = append(frontier, candidate)
		}
		// A later probe may consume the last state slot; reserve a post-state
		// only after the whole frontier has been observed.
		if c.Spent.States+1 > c.Budget.MaxStates || ctx.Err() != nil {
			selected = -1
			stop = "BUDGET"
		}
		if stop == "MODEL_ERROR" || stop == "SPECIFICATION_GAP" {
			selected = -1
		}
		c.Frontier = frontier
		c.Spent.ElapsedMS = businessCampaignElapsed(c)
		if selected < 0 {
			if stop == "BLOCKED" {
				for _, candidate := range frontier {
					if candidate.Eligibility == "BUDGET" {
						stop = "BUDGET"
						break
					}
				}
			}
			return d.stopBusinessCampaign(&c, lease, stop, "no safely dispatchable registered branch/input variant in the freshly observed frontier")
		}
		candidate := &c.Frontier[selected]
		candidate.Selected = true
		runtime := runtimes[selected]
		intent := businessCampaignDispatch{Campaign: c.ID, LedgerRevision: c.Revision + 1, Sequence: c.Spent.Dispatches + 1, Run: candidate.Run, VariantID: candidate.VariantID, Branch: candidate.Branch, Runner: candidate.Runner, Task: runtime.Variant.Task, Flow: c.Flow, Round: c.Round, Binding: runtime.Binding, Participants: runtime.Participants, RunnerSHA256: runtime.Runner.SHA256, CodeSHA256: runtime.Runner.CodeSHA256, Candidate: *candidate, HistorySHA256: c.HistorySHA256, ReservedActions: runtime.UpperActions, Owner: lease.Owner, Fence: lease.Fence, Created: time.Now().UTC().Format(time.RFC3339Nano)}
		body, err := json.Marshal(intent)
		if err != nil {
			return c, err
		}
		hash := digestBytes(body)
		c.Dispatches = append(c.Dispatches, BusinessCampaignDispatchReference{Sequence: intent.Sequence, Run: intent.Run, SHA256: hash})
		c.Spent.Dispatches++
		c.Spent.ReservedActions += intent.ReservedActions
		insert := fmt.Sprintf("INSERT INTO business_campaign_dispatches(campaign_id,seq,run_id,variant_id,runner_id,task,status,intent_sha256,body_json,created,updated) VALUES(%s,%d,%s,%s,%s,%s,'INTENT',%s,%s,%s,%s)", quote(c.ID), intent.Sequence, quote(intent.Run), quote(intent.VariantID), quote(intent.Runner), quote(intent.Task), quote(hash), quote(string(body)), quote(intent.Created), quote(intent.Created))
		if err := d.saveBusinessCampaign(&c, lease, "DISPATCH_INTENT", insert); err != nil {
			return c, err
		}
		authorization := &businessCampaignExecution{Intent: intent, SHA256: hash, Lease: lease}
		if hooks != nil && hooks.afterIntent != nil {
			if err := hooks.afterIntent(c, authorization); err != nil {
				return c, err
			}
		}
		_, executeErr := d.execute(ctx, ExecuteRequest{Task: intent.Task, Runner: intent.Runner, Round: c.Round, Environment: c.Environment, Commit: c.Request.Commit, Root: c.Request.Root, Config: c.Request.Config, Account: c.Request.Account, Fixture: c.Request.Fixture, Accounts: c.Request.Accounts, campaign: authorization})
		if hooks != nil && hooks.afterExecute != nil {
			if err := hooks.afterExecute(c, authorization); err != nil {
				return c, err
			}
		}
		if err := d.recordBusinessCampaignChild(&c, g, intent, hash, lease); err != nil {
			reason := "child dispatch is incomplete or cannot be independently verified; it will never be replayed automatically: " + err.Error()
			if executeErr != nil {
				reason += "; execute: " + executeErr.Error()
			}
			return d.stopBusinessCampaign(&c, lease, "UNKNOWN", reason)
		}
		if c.Status != "RUNNING" {
			return c, nil
		}
	}
}

func businessCampaignPrefer(next, current BusinessCampaignVariant) bool {
	if (next.Passes == 0) != (current.Passes == 0) {
		return next.Passes == 0
	}
	if next.Visits != current.Visits {
		return next.Visits < current.Visits
	}
	return next.ID < current.ID
}

func (d db) businessCampaignDispatchGuard(execution *businessCampaignExecution) string {
	i := execution.Intent
	return fmt.Sprintf("EXISTS(SELECT 1 FROM business_campaign_dispatches x JOIN business_campaigns c ON c.id=x.campaign_id WHERE x.campaign_id=%s AND x.seq=%d AND x.run_id=%s AND x.status='INTENT' AND x.intent_sha256=%s AND c.status='RUNNING' AND c.owner=%s AND c.fence=%d AND c.lease_until>%s)", quote(i.Campaign), i.Sequence, quote(i.Run), quote(execution.SHA256), quote(execution.Lease.Owner), execution.Lease.Fence, quote(time.Now().UTC().Format(time.RFC3339Nano)))
}

func (d db) validateBusinessCampaignDispatch(task string, binding RunBinding, record ModelRunRecord, execution *businessCampaignExecution) error {
	i := execution.Intent
	if err := d.validateBusinessCampaignLease(i.Campaign, execution.Lease); err != nil {
		return err
	}
	body, err := json.Marshal(i)
	if err != nil {
		return err
	}
	if digestBytes(body) != execution.SHA256 || i.Run == "" || task != i.Task || binding != i.Binding || record.Round != i.Round || record.Runner != i.Runner || record.RunnerSHA256 != i.RunnerSHA256 || record.CodeSHA256 != i.CodeSHA256 || !reflect.DeepEqual(record.Participants, i.Participants) || execution.Lease.Owner != i.Owner || execution.Lease.Fence != i.Fence {
		return errors.New("campaign execution does not match its frozen dispatch identity")
	}
	rows, err := d.query("SELECT body_json,intent_sha256,status FROM business_campaign_dispatches WHERE run_id="+quote(i.Run), 3)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0][0] != string(body) || rows[0][1] != execution.SHA256 || rows[0][2] != "INTENT" {
		return errors.New("campaign dispatch is absent, changed or already consumed")
	}
	c, err := d.readBusinessCampaign(i.Campaign)
	if err != nil {
		return err
	}
	if c.Round != i.Round || c.Flow != i.Flow || c.Identity.Graph != binding.Identity.Graph || c.Request.Account != binding.Identity.Account || c.Request.Fixture != binding.Identity.Fixture || c.HistorySHA256 != i.HistorySHA256 {
		return errors.New("campaign scope/history changed before dispatch")
	}
	for _, variant := range c.Variants {
		if variant.ID == i.VariantID && variant.Branch == i.Branch && variant.Runner == i.Runner && variant.Task == i.Task && variant.RunnerSHA256 == i.RunnerSHA256 && variant.CodeSHA256 == i.CodeSHA256 {
			return nil
		}
	}
	return errors.New("dispatch does not name a frozen campaign variant")
}

// Keep diagnostic error text bounded even if an external adapter returns an
// unusually long message; raw HTTP/command evidence remains in child RAW.
func campaignReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > 4096 {
		return reason[:4096]
	}
	return reason
}

func validateBusinessCampaignTarget(execution *businessCampaignExecution, step RunnerStep, observation *ModelObservation) error {
	selected := execution.Intent.Candidate
	if selected.Input == nil || selected.Before == nil {
		return errors.New("dispatch has no captured target input or actual subject identities")
	}
	actual, err := runtimeRequestFor(step, 1, observation)
	if err != nil {
		return err
	}
	if !equalBusinessCampaignJSON(*actual, *selected.Input) {
		return errors.New("resolved target request differs from the durable selected input")
	}
	if len(observation.BusinessExpectations) != 1 || observation.BusinessExpectations[0].Decision.Status != "READY" || observation.BusinessExpectations[0].Decision.Frozen == nil {
		return errors.New("target has no unique READY expectation")
	}
	frozen := observation.BusinessExpectations[0].Decision.Frozen
	if frozen.Branch != execution.Intent.Branch {
		return errors.New("target branch differs from durable selected branch")
	}
	for subject, identity := range selected.Before.Bindings {
		if identity != "" && frozen.Before.Bindings[subject] != identity {
			return fmt.Errorf("subject %s changed identity after selection", subject)
		}
	}
	return nil
}
