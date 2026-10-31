package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type ExplorePlanner interface {
	Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error)
}

type PlannerContext struct {
	GraphID        string            `json:"graph_id"`
	Capability     string            `json:"capability"`
	Role           string            `json:"role"`
	State          string            `json:"state"`
	ConfirmedRules []string          `json:"confirmed_rules"`
	Entries        []ExploreEntry    `json:"entries"`
	Recent         []ExploreStep     `json:"recent"`
	Unspecified    []UnspecifiedItem `json:"unspecified"`
	RuleFails      []ExploreFinding  `json:"rule_fails"`
	Frontier       []CandidateAction `json:"frontier"`
	BudgetLeft     int               `json:"budget_left"`
}

type PlannerSuggestion struct {
	Capability    string          `json:"capability"`
	Action        string          `json:"action"`
	Entry         string          `json:"entry"`
	Role          string          `json:"role"`
	Reason        string          `json:"reason"`
	Evidence      []string        `json:"evidence"`
	Observe       string          `json:"observe"`
	Risk          string          `json:"risk"`
	Preconditions map[string]bool `json:"preconditions"`
	NeedsRecovery bool            `json:"needs_recovery"`
	NeedObserve   string          `json:"need_observe"`
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
	return proposal, nil
}

func validatePlannerSuggestion(item PlannerSuggestion, adapter ExploreAdapter, accounts map[string]string, budgetLeft int) (CandidateAction, string) {
	if budgetLeft < 1 {
		return CandidateAction{}, "budget exhausted"
	}
	if item.Action == "" || item.Entry == "" || item.Reason == "" {
		return CandidateAction{}, "suggestion needs action, entry and reason"
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

func shouldAskPlanner(mode string, candidates []CandidateAction, report ExploreReport, world exploreWorld) bool {
	if mode != "ai" {
		return false
	}
	if len(report.Unspecified) > 0 || len(report.RuleFails) > 0 {
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
	return onlyRepeat || world.state == ""
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

func marshalPlannerAudit(proposal PlannerProposal, accepted []CandidateAction, rejected []string) json.RawMessage {
	body, err := json.Marshal(map[string]any{"proposal": proposal, "accepted": accepted, "rejected": rejected})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return body
}
