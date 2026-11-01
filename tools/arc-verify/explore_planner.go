package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type ExplorePlanner interface {
	Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error)
}

type PlannerContext struct {
	GraphID             string                  `json:"graph_id"`
	Capability          string                  `json:"capability"`
	Role                string                  `json:"role"`
	State               string                  `json:"state"`
	ConfirmedRules      []string                `json:"confirmed_rules"`
	Entries             []ExploreEntry          `json:"entries"`
	Recent              []ExploreStep           `json:"recent"`
	Unspecified         []UnspecifiedItem       `json:"unspecified"`
	RuleFails           []ExploreFinding        `json:"rule_fails"`
	Frontier            []CandidateAction       `json:"frontier"`
	BudgetLeft          int                     `json:"budget_left"`
	Facts               []EvidenceFact          `json:"facts,omitempty"`
	Hypotheses          []BoundaryHypothesis    `json:"hypotheses,omitempty"`
	Explanations        []CompetingExplanation  `json:"explanations,omitempty"`
	Experiments         []ExperimentCandidate   `json:"experiments,omitempty"`
	Revisions           []UnderstandingRevision `json:"revisions,omitempty"`
	Authorization       []string                `json:"authorization,omitempty"`
	RefutedHypotheses   []string                `json:"refuted_hypotheses,omitempty"`
	UnresolvedQuestions []string                `json:"unresolved_questions,omitempty"`
}

type PlannerSuggestion struct {
	HypothesisID           string          `json:"hypothesis_id,omitempty"`
	ExperimentID           string          `json:"experiment_id,omitempty"`
	Capability             string          `json:"capability"`
	BusinessIntent         string          `json:"business_intent,omitempty"`
	BoundaryHypothesis     string          `json:"boundary_hypothesis,omitempty"`
	CandidateAction        string          `json:"candidate_action,omitempty"`
	Action                 string          `json:"action"`
	Entry                  string          `json:"entry"`
	Role                   string          `json:"role"`
	Reason                 string          `json:"reason"`
	Evidence               []string        `json:"evidence"`
	SourceEvidence         []string        `json:"source_evidence,omitempty"`
	Observe                string          `json:"observe"`
	ExpectedObservation    string          `json:"expected_observation,omitempty"`
	PossibleCounterexample string          `json:"possible_counterexample,omitempty"`
	Risk                   string          `json:"risk"`
	ExecutionCost          int             `json:"execution_cost,omitempty"`
	Preconditions          map[string]bool `json:"preconditions"`
	NeedsRecovery          bool            `json:"needs_recovery"`
	RecoveryRequirement    string          `json:"recovery_requirement,omitempty"`
	NeedObserve            string          `json:"need_observe"`
	InformationGap         string          `json:"information_gap,omitempty"`
}

type PlannerProposal struct {
	Suggestions []PlannerSuggestion `json:"suggestions"`
	Tokens      int                 `json:"tokens,omitempty"`
	Model       string              `json:"model,omitempty"`
	Rejected    []string            `json:"rejected,omitempty"`
}

type ScriptedExplorePlanner struct {
	Suggestions []PlannerSuggestion
	Calls       int
}

func (p *ScriptedExplorePlanner) Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error) {
	if err := ctx.Err(); err != nil {
		return PlannerProposal{}, err
	}
	p.Calls++
	limit := 4
	if in.BudgetLeft < limit {
		limit = in.BudgetLeft
	}
	if limit < 0 {
		limit = 0
	}
	out := PlannerProposal{Model: "scripted", Suggestions: []PlannerSuggestion{}}
	if limit > len(p.Suggestions) {
		limit = len(p.Suggestions)
	}
	out.Suggestions = append(out.Suggestions, p.Suggestions[:limit]...)
	return out, nil
}

type FileExplorePlanner struct {
	Path string
}

func (p FileExplorePlanner) Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error) {
	if err := ctx.Err(); err != nil {
		return PlannerProposal{}, err
	}
	if p.Path == "" {
		return PlannerProposal{}, errors.New("planner file is empty")
	}
	body, err := os.ReadFile(p.Path)
	if err != nil {
		return PlannerProposal{}, err
	}
	var proposal PlannerProposal
	if err := decodeStrictJSON(body, &proposal); err != nil {
		return PlannerProposal{}, err
	}
	if len(proposal.Suggestions) > 8 {
		proposal.Suggestions = proposal.Suggestions[:8]
	}
	// User-supplied fixture data is not a real model call, regardless of
	// any model or token values embedded in the JSON.
	proposal.Model = "scripted"
	proposal.Tokens = 0
	return proposal, nil
}

func validatePlannerSuggestion(item PlannerSuggestion, adapter ExploreAdapter, accounts map[string]string, budgetLeft int) (CandidateAction, string) {
	if budgetLeft < 1 {
		return CandidateAction{}, "budget exhausted"
	}
	if item.Action == "" || item.Entry == "" || item.Reason == "" {
		return CandidateAction{}, "suggestion needs action, entry and reason"
	}
	blob := strings.ToLower(item.Reason + item.BusinessIntent + item.BoundaryHypothesis)
	if strings.Contains(blob, " pass") || strings.HasSuffix(blob, "pass") || strings.Contains(blob, "fail") && strings.Contains(blob, "verdict") {
		return CandidateAction{}, "planner must not decide a business PASS or FAIL"
	}
	if strings.ContainsAny(item.Action, " \t\n;&|") || strings.Contains(item.Entry, "://") {
		return CandidateAction{}, "suggestion action or entry is not on the allow-list"
	}
	matched := false
	for _, entry := range adapter.Entries {
		if entry.Action == item.Action && entry.Entry == item.Entry {
			matched = true
			if len(entry.Roles) > 0 && item.Role != "" && !hasToken(entry.Roles, item.Role) && !hasToken(entry.Roles, strings.ToUpper(item.Role)) {
				return CandidateAction{}, "role is not listed on the approved entry"
			}
			break
		}
	}
	if !matched {
		return CandidateAction{}, "entry is not an approved adapter action"
	}
	role := item.Role
	account := accounts[role]
	if account == "" {
		account = adapter.Accounts[role]
	}
	if account == "" {
		return CandidateAction{}, "role has no authorized account"
	}
	if item.NeedsRecovery && adapter.Recovery == nil {
		return CandidateAction{}, "suggestion requires recovery that the adapter cannot perform"
	}
	risk := item.Risk
	if risk == "" {
		risk = "medium"
	}
	if risk != "low" && risk != "medium" && risk != "high" {
		return CandidateAction{}, "risk is outside the allowed set"
	}
	return CandidateAction{
		Kind:          kindHistory,
		Source:        sourceInfer,
		Action:        item.Action,
		Entry:         item.Entry,
		Role:          role,
		Account:       account,
		Preconditions: item.Preconditions,
		Scope:         item.Capability,
		Risk:          risk,
		Cost:          3,
		RequestID:     "ai-" + shortDigest(item.Action + "|" + item.Role + "|" + item.Reason)[:8],
	}, ""
}

func (d db) applyExplorePlannerTurn(ctx context.Context, request ExploreRequest, flow BehaviorFlow, approvals map[string]bool, world exploreWorld, report *ExploreReport, candidates []CandidateAction, tried map[string]bool, disc DiscoveryContext, campaign string, plannerCalls int, budget ExploreRuntimeBudget) ([]CandidateAction, int, bool, error) {
	if report == nil || !shouldAskPlanner(request.Mode, candidates, *report, world) || request.Planner == nil {
		return candidates, plannerCalls, false, nil
	}
	if budget.MaxPlannerCalls > 0 && plannerCalls >= budget.MaxPlannerCalls {
		report.StopReason, report.Truncated = "budget_planner", true
		report.Remaining = remainingCandidates(candidates, tried, world.state)
		return candidates, plannerCalls, true, nil
	}
	planIn := attachDiscoveryToPlanner(plannerContext(request, flow, approvals, world, *report, candidates, budget.MaxActions-report.Actions), disc)
	proposal, planErr := request.Planner.Suggest(ctx, planIn)
	plannerCalls++
	report.Spent.PlannerCalls = plannerCalls
	if planErr != nil {
		report.Blocked = append(report.Blocked, ExploreFinding{Kind: "BLOCKED", Reason: "planner unavailable: " + planErr.Error()})
		return candidates, plannerCalls, false, nil
	}
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
		return candidates, plannerCalls, false, nil
	}
	return append(candidates, accepted...), plannerCalls, false, nil
}

func shouldAskPlanner(mode string, candidates []CandidateAction, report ExploreReport, world exploreWorld) bool {
	if mode != "ai" {
		return false
	}
	if report.Spent.PlannerCalls == 0 && len(report.Steps) == 0 {
		return true
	}
	if len(report.Unspecified) > 0 || len(report.RuleFails) > 0 || len(report.Blocked) > 0 {
		return true
	}
	if len(candidates) == 0 {
		return true
	}
	onlyRepeat := true
	for _, item := range candidates {
		if item.Kind != kindRepeat && item.Kind != kindTimeout {
			onlyRepeat = false
			break
		}
	}
	if onlyRepeat || world.state == "" || world.lastDenied || world.lastTO {
		return true
	}
	if len(report.Steps) > 0 {
		last := report.Steps[len(report.Steps)-1]
		if last.Group != nil && (last.Group.Partial || last.Group.Unknown) {
			return true
		}
		if last.After.State != last.Before.State {
			return true
		}
	}
	return false
}

func plannerContext(request ExploreRequest, flow BehaviorFlow, approvals map[string]bool, world exploreWorld, report ExploreReport, leftover []CandidateAction, budgetLeft int) PlannerContext {
	rules := []string{}
	for id, ok := range approvals {
		if ok {
			rules = append(rules, id)
		}
	}
	sortStrings(rules)
	recent := report.Steps
	if len(recent) > 4 {
		recent = recent[len(recent)-4:]
	}
	return PlannerContext{
		GraphID: request.Graph, Capability: flow.Capability, Role: world.role, State: world.state,
		ConfirmedRules: rules, Entries: request.Adapter.Entries, Recent: recent,
		Unspecified: report.Unspecified, RuleFails: report.RuleFails, Frontier: leftover, BudgetLeft: budgetLeft,
	}
}

func hasToken(values []string, want string) bool {
	for _, item := range values {
		if item == want {
			return true
		}
	}
	return false
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func filterDeterministic(candidates []CandidateAction) []CandidateAction {
	out := []CandidateAction{}
	for _, item := range candidates {
		if item.Source == sourceRule && item.Kind == kindDeclared {
			out = append(out, item)
		}
	}
	return out
}

func decodePlannerMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "heuristic":
		return "heuristic"
	case "deterministic", "ai":
		return strings.ToLower(value)
	default:
		return "heuristic"
	}
}

func redactPlannerText(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "sk-") || strings.Contains(lower, "bearer ") || strings.Contains(lower, "api_key") {
		return "[redacted]"
	}
	if len(value) > 400 {
		return value[:400] + "…"
	}
	return value
}

func (d db) persistPlannerAudit(campaign string, in PlannerContext, proposal PlannerProposal, accepted []CandidateAction, rejected []string, callNo int) error {
	if campaign == "" {
		return errors.New("planner audit needs a campaign")
	}
	if _, err := d.handle.Exec("CREATE TABLE IF NOT EXISTS exploration_planner_audit (id TEXT PRIMARY KEY, campaign TEXT NOT NULL, model TEXT NOT NULL, body TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);"); err != nil {
		return err
	}
	id := newID("PC")
	hypotheses := []string{}
	for _, item := range in.Hypotheses {
		hypotheses = append(hypotheses, item.ID+":"+item.Status)
	}
	acceptedSummary := []map[string]string{}
	for _, item := range accepted {
		acceptedSummary = append(acceptedSummary, map[string]string{
			"action": item.Action, "entry": item.Entry, "role": item.Role, "kind": string(item.Kind),
		})
	}
	suggestionIDs := []string{}
	for _, item := range proposal.Suggestions {
		if item.HypothesisID != "" {
			suggestionIDs = appendUnique(suggestionIDs, item.HypothesisID)
		}
		if item.ExperimentID != "" {
			suggestionIDs = appendUnique(suggestionIDs, item.ExperimentID)
		}
	}
	source := "scripted_file"
	if proposal.Model == "scripted" {
		source = "scripted"
	}
	audit := map[string]any{
		"planner_call_id": id,
		"campaign":        campaign,
		"call_no":         callNo,
		"model":           redactPlannerText(firstNonEmpty(proposal.Model, "unknown")),
		"source":          source,
		"input_facts":     factAuditIDs(in.Facts),
		"authorization":   in.Authorization,
		"accepted":        acceptedSummary,
		"rejected":        rejected,
		"hypotheses":      hypotheses,
		"suggestion_ids":  suggestionIDs,
		"tokens":          proposal.Tokens,
	}
	body, err := json.Marshal(audit)
	if err != nil {
		return err
	}
	if sensitivePlannerAudit(string(body)) {
		return errors.New("planner audit contains disallowed sensitive content")
	}
	return d.tx([]string{fmt.Sprintf(
		"INSERT INTO exploration_planner_audit(id,campaign,model,body) VALUES(%s,%s,%s,%s);",
		quote(id), quote(campaign), quote(firstNonEmpty(proposal.Model, "unknown")), quote(string(body)),
	)})
}

func factAuditIDs(facts []EvidenceFact) []string {
	out := []string{}
	for _, fact := range facts {
		out = append(out, fact.ID+":"+fact.Class)
	}
	return out
}

func sensitivePlannerAudit(body string) bool {
	lower := strings.ToLower(body)
	for _, token := range []string{"sk-", "bearer ", "api_key", "authorization:", "password"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// Agent reasoning belongs to the active coding Agent and the written Markdown plan.
// CLI planner files are optional, reproducible fixtures; no model HTTP connection.
func plannerFromFlags(mode, path string) (ExplorePlanner, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "agent" {
		return nil, errors.New("CLI has no Agent planner or file handshake; use heuristic/deterministic, or --mode ai --planner FILE as a scripted comparison fixture")
	}
	if path != "" {
		return FileExplorePlanner{Path: path}, nil
	}
	if mode == "ai" {
		return nil, errors.New("CLI has no AI model: use --planner FILE as a scripted comparison fixture")
	}
	return nil, nil
}
