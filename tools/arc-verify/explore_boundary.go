package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	factKnown       = "KNOWN"
	factInferred    = "INFERRED"
	factConflicting = "CONFLICTING"
	factUnknown     = "UNKNOWN"
	factBusiness    = "BUSINESS_DECISION"

	hypProposed       = "PROPOSED"
	hypEvidenceNeeded = "EVIDENCE_NEEDED"
	hypReady          = "READY_TO_TEST"
	hypTested         = "TESTED"
	hypSupported      = "SUPPORTED"
	hypRefuted        = "REFUTED"
	hypInconclusive   = "INCONCLUSIVE"
	hypNeedsDecision  = "NEEDS_BUSINESS_DECISION"
)

type EvidenceReference struct {
	ID, Source, Version, Scope, Trust string
}

type EvidenceFact struct {
	ID, Class, Statement                                         string
	Capability, SourceType, Location, SourceCommit, Scope, Trust string
	Evidence                                                     []string
	Observed, Confirmed                                          bool
	Refs                                                         []EvidenceReference
}

type HypothesisIdentity struct {
	Role, Action, Resource, State, Invariant string
}

type BoundaryHypothesis struct {
	ID, Capability                    string
	Identity                          HypothesisIdentity `json:"identity,omitempty"`
	KnownFacts                        []string
	Assumption, Counterfactual        string
	CandidateActions                  []string
	RequiredObservation, EvidenceRefs []string
	Roles, Resources                  []string
	Preconditions                     []string
	Observable, Falsifier             []string
	Risk, Cost                        string
	Status                            string
	Version                           int
}

type CompetingExplanation struct {
	ID, Observation          string
	Facts, Supports, Refutes []string
	Experiment, Oracle       string
	Safe                     bool
	Status                   string
}

type ExperimentCandidate struct {
	ID, Hypothesis, SideEffect, Recovery, Oracle string
	Goal, Actor, Resource                        string
	Actions                                      []string
	Preconditions                                []string
	Observations                                 []string
	Stop                                         string
	Cost, Roles                                  int
	Discriminating                               bool
}

type UnderstandingRevision struct {
	Version                                 int
	Campaign, AfterStep, Believed, Observed string
	Supported, Refuted, StillValidRules     []string
	Open, BusinessDecisions, EvidenceRefs   []string
}

type DiscoveryContext struct {
	Facts         []EvidenceFact
	Hypotheses    []BoundaryHypothesis
	Explanations  []CompetingExplanation
	Experiments   []ExperimentCandidate
	Revisions     []UnderstandingRevision
	Questions     []string
	Authorization []string
}

type AmbiguousObservation struct {
	HTTPStatus   int
	Owners       []string
	Resources    []string
	SameResource bool
}

func buildEvidenceContext(g Graph, approvals map[string]bool, _ ExploreAdapter, report ExploreReport, adapter ExploreAdapter, entries []DiscoveredEntry) DiscoveryContext {
	out := DiscoveryContext{Facts: []EvidenceFact{}, Questions: []string{}, Authorization: []string{}}
	businessIndex := indexModel(g)
	graphRef := EvidenceReference{ID: "graph:" + g.ID, Source: "graph", Version: fmt.Sprintf("v%d", g.SchemaVersion), Scope: g.ID, Trust: "VERIFIED"}
	for _, rule := range g.Rules {
		approved := approvals[rule.ID]
		class := factInferred
		statement := "unconfirmed rule " + rule.ID + " describes " + rule.Action
		if approved {
			class = factKnown
			statement = "confirmed RULE " + rule.ID + " covers " + rule.Actor + " " + rule.Action + " via " + rule.Entry
			out.Authorization = appendUnique(out.Authorization, "rule:"+rule.ID)
		}
		out.Facts = append(out.Facts, EvidenceFact{
			ID: discoveryFactID("rule", rule.ID), Class: class, Statement: statement,
			Capability: confirmedCapabilityForAction(businessIndex, rule.Action), SourceType: "rule", Location: rule.ID, SourceCommit: report.Identity.SourceCommit, Scope: g.ID, Trust: trustForClass(class),
			Confirmed: approved, Evidence: []string{"rule:" + rule.ID},
			Refs: []EvidenceReference{graphRef, {ID: "rule:" + rule.ID, Source: "rule", Version: rule.Source, Scope: g.ID, Trust: trustForClass(class)}},
		})
	}
	for _, flow := range g.Flows {
		out.Facts = append(out.Facts, EvidenceFact{
			ID: discoveryFactID("flow", flow.ID), Class: factKnown,
			Statement:  "flow " + flow.ID + " starts at " + flow.Initial + " for " + flow.Capability,
			Capability: flow.Capability, SourceType: "flow", Location: flow.ID, Scope: g.ID, Trust: "VERIFIED", Confirmed: true,
			Refs: []EvidenceReference{graphRef, {ID: "flow:" + flow.ID, Source: "flow", Scope: g.ID, Trust: "VERIFIED"}},
		})
	}
	for _, entry := range adapter.Entries {
		out.Facts = append(out.Facts, EvidenceFact{
			ID: discoveryFactID("entry", entry.Action+"|"+entry.Entry), Class: factKnown,
			Statement:  "authorized adapter entry " + entry.Action + " " + entry.Entry,
			Capability: confirmedCapabilityForAction(businessIndex, entry.Action), SourceType: "entry", Location: entry.Entry, Scope: adapter.IsolationScope, Trust: "VERIFIED", Confirmed: true, Observed: true,
			Refs: []EvidenceReference{{ID: "adapter:" + entry.Entry, Source: "adapter", Scope: adapter.IsolationScope, Trust: "VERIFIED"}},
		})
		out.Authorization = appendUnique(out.Authorization, "entry:"+entry.Entry)
	}
	for _, item := range entries {
		class := factUnknown
		if item.Class == entryConfirmed {
			class = factKnown
		}
		if item.Class == entryForbidden {
			class = factInferred
		}
		out.Facts = append(out.Facts, EvidenceFact{
			ID: discoveryFactID("src", item.Path), Class: class,
			Statement: item.Class + " " + item.Path + " from " + item.File,
			Capability: confirmedCapabilityForAction(businessIndex, item.Action), SourceType: "source",
			Refs:      []EvidenceReference{{ID: item.File, Source: "source", Scope: item.Path, Trust: trustForClass(class)}},
		})
	}
	for _, item := range report.Unspecified {
		out.Facts = append(out.Facts, EvidenceFact{
			ID: discoveryFactID("unspec", item.ID), Class: factBusiness,
			Statement: "UNSPECIFIED " + item.Kind + " on " + item.Action + " is not a confirmed rule",
			Refs:      []EvidenceReference{{ID: item.ID, Source: "unspecified", Scope: g.ID, Trust: "UNVERIFIED"}},
		})
		out.Questions = appendUnique(out.Questions, "BUSINESS_DECISION: is "+item.Action+" "+item.Kind+" intended?")
	}
	for _, finding := range report.Risks {
		out.Facts = append(out.Facts, EvidenceFact{
			ID: discoveryFactID("risk", finding.Reason), Class: factInferred,
			Statement: "RISK " + finding.Reason,
			Refs:      []EvidenceReference{{ID: firstNonEmpty(finding.Evidence, finding.Rule), Source: "explore", Trust: "UNVERIFIED"}},
		})
	}
	markConflictingFacts(&out)
	for _, fact := range out.Facts {
		if fact.Class == factUnknown && !observableFact(fact, adapter) {
			out.Questions = appendUnique(out.Questions, "UNKNOWN: "+fact.Statement)
		}
	}
	return out
}

// Only a unique declared "capability implements action" mapping is a
// capability fact. An API path or action name alone is not a business capability.
func confirmedCapabilityForAction(x modelIndex, action string) string {
	if linked := x.capsByAction[action]; len(linked) == 1 {
		return linked[0]
	}
	return ""
}

func markConflictingFacts(ctx *DiscoveryContext) {
	byAction := map[string][]int{}
	for i, fact := range ctx.Facts {
		action := factActionHint(fact.Statement)
		if action != "" {
			byAction[action] = append(byAction[action], i)
		}
	}
	for _, idxs := range byAction {
		classes := map[string]bool{}
		for _, i := range idxs {
			classes[ctx.Facts[i].Class] = true
		}
		if classes[factKnown] && classes[factBusiness] {
			for _, i := range idxs {
				if ctx.Facts[i].Class == factBusiness {
					ctx.Facts[i].Class = factConflicting
				}
			}
		}
	}
}

func factActionHint(statement string) string {
	for _, token := range []string{"claim", "enroll", "confirm", "close", "send"} {
		if strings.Contains(statement, token) {
			return token
		}
	}
	return ""
}

func observableFact(fact EvidenceFact, adapter ExploreAdapter) bool {
	for _, entry := range adapter.Entries {
		if strings.Contains(fact.Statement, entry.Entry) || strings.Contains(fact.Statement, entry.Action) {
			return true
		}
	}
	return adapter.Oracle.Call.Path != ""
}

func trustForClass(class string) string {
	if class == factKnown {
		return "VERIFIED"
	}
	return "UNVERIFIED"
}

func proposeCounterfactuals(ctx DiscoveryContext, g Graph, adapter ExploreAdapter) []BoundaryHypothesis {
	capability := ""
	if len(g.Flows) > 0 {
		capability = g.Flows[0].Capability
	}
	factIDs := []string{}
	refs := []string{}
	for _, fact := range ctx.Facts {
		if fact.Class == factKnown {
			factIDs = append(factIDs, fact.ID)
			if len(fact.Refs) > 0 {
				refs = append(refs, fact.Refs[0].ID)
			}
		}
	}
	if len(factIDs) == 0 {
		return nil
	}
	roles := map[string]bool{}
	actions := []string{}
	for _, entry := range adapter.Entries {
		actions = appendUnique(actions, entry.Action)
		for _, role := range entry.Roles {
			roles[role] = true
		}
	}
	type seed struct{ assumption, counter, action, observe, status string }
	seeds := []seed{}
	if len(roles) >= 2 {
		seeds = append(seeds, seed{"only the declared actor may perform the action", "if the other authorized role performs it", firstNonEmpty(actions...), "owner", hypProposed})
	}
	if len(actions) >= 2 {
		seeds = append(seeds, seed{"the declared order of actions is required", "if the later action runs first", actions[len(actions)-1], "state", hypProposed})
	}
	if len(actions) >= 1 {
		seeds = append(seeds, seed{"repeating a successful action is a no-op", "if the same actor repeats the action", actions[0], "version", hypProposed})
	}
	if len(adapter.Accounts) >= 2 || len(roles) >= 2 {
		seeds = append(seeds, seed{"one resource has a single owner", "if two subjects act at the same time", firstNonEmpty(actions...), "owner", hypProposed})
	}
	if adapter.Recovery != nil {
		seeds = append(seeds, seed{"recovery and the happy path do not interleave", "if restore runs during a live action", "recover", "state", hypProposed})
	}
	for _, item := range ctx.Facts {
		if item.Class == factBusiness || item.Class == factConflicting {
			seeds = append(seeds, seed{"the observed extra HTTP success is specified", "whether a second HTTP 200 means a second owner", "claim", "owner", hypNeedsDecision})
			break
		}
	}
	out := []BoundaryHypothesis{}
	seen := map[string]bool{}
	for i, item := range seeds {
		key := item.assumption + "|" + item.counter
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, BoundaryHypothesis{
			ID: fmt.Sprintf("BH-%03d", i+1), Capability: capability,
			Identity:   HypothesisIdentity{Action: item.action, Invariant: item.assumption, State: "current", Resource: capability},
			KnownFacts: factIDs, Assumption: item.assumption, Counterfactual: item.counter,
			CandidateActions: []string{item.action}, RequiredObservation: []string{item.observe},
			Observable: []string{item.observe}, Falsifier: []string{item.observe + " unchanged"},
			EvidenceRefs: refs, Status: item.status, Version: 1, Risk: "medium", Cost: "1",
		})
	}
	return out
}

func competeExplanations(obs AmbiguousObservation, facts []EvidenceFact) []CompetingExplanation {
	if obs.HTTPStatus == 0 {
		return nil
	}
	factIDs := []string{}
	for _, fact := range facts {
		factIDs = append(factIDs, fact.ID)
	}
	same := "both calls returned HTTP 200"
	items := []CompetingExplanation{
		{ID: "EX-dual", Observation: same, Facts: factIDs, Supports: []string{"two live owners"}, Refutes: []string{"oracle owner is a single principal"}, Experiment: "read owner and version after both claims", Oracle: "owner+version", Safe: true},
		{ID: "EX-last", Observation: same, Facts: factIDs, Supports: []string{"oracle owner equals the last actor"}, Refutes: []string{"two owners remain"}, Experiment: "read owner after the second 200", Oracle: "owner+version", Safe: true},
		{ID: "EX-idem", Observation: same, Facts: factIDs, Supports: []string{"version does not grow"}, Refutes: []string{"version or owner changes"}, Experiment: "compare version before and after the second call", Oracle: "owner+version", Safe: true},
		{ID: "EX-async", Observation: same, Facts: factIDs, Supports: []string{"owner still empty after 200"}, Refutes: []string{"owner is already assigned"}, Experiment: "re-read owner after a bounded wait", Oracle: "owner+version", Safe: true},
	}
	if !obs.SameResource {
		items = append(items, CompetingExplanation{ID: "EX-other", Observation: same, Facts: factIDs, Supports: []string{"owners map to different resources"}, Refutes: []string{"both IDs are the same work item"}, Experiment: "compare resource identifiers", Oracle: "resource-id", Safe: true})
	}
	if len(obs.Owners) >= 2 && obs.SameResource {
		return items
	}
	if obs.HTTPStatus == 200 {
		return items[:3]
	}
	return items[:2]
}

func planMinimalExperiment(explanations []CompetingExplanation, adapter ExploreAdapter) ExperimentCandidate {
	oraclePath := "/oracle"
	if adapter.Oracle.Call.Path != "" {
		oraclePath = adapter.Oracle.Call.Path
	}
	return ExperimentCandidate{
		ID: "EXP-min-owner", Hypothesis: firstExplanationID(explanations),
		Goal: "distinguish competing owners after identical HTTP 200", Actor: "oracle", Resource: "work-item",
		Actions: []string{"GET " + oraclePath}, Cost: 1, Roles: 1,
		SideEffect: "none", Recovery: "not required", Oracle: "owner+version",
		Observations: []string{"owner", "version"}, Stop: "owner and version observed once",
		Discriminating: len(explanations) >= 2,
	}
}

func sprayExperiment(explanations []CompetingExplanation) ExperimentCandidate {
	cost := 3
	if len(explanations) > 0 {
		cost = 3 * len(explanations)
	}
	if cost < 8 {
		cost = 8
	}
	return ExperimentCandidate{ID: "EXP-spray", Actions: []string{"claim", "claim", "close", "open", "claim"}, Cost: cost, Discriminating: false, Oracle: "http"}
}

func firstExplanationID(items []CompetingExplanation) string {
	if len(items) == 0 {
		return ""
	}
	return items[0].ID
}

func reviseUnderstanding(ctx DiscoveryContext, step ExploreStep, approvals map[string]bool) UnderstandingRevision {
	rev := UnderstandingRevision{Version: len(ctx.Revisions) + 1, AfterStep: step.Candidate.Action, Observed: step.After.State}
	if len(ctx.Hypotheses) > 0 {
		rev.Believed = ctx.Hypotheses[0].Assumption
	}
	if len(ctx.Revisions) > 0 && rev.Believed == "" {
		rev.Believed = ctx.Revisions[0].Believed
	}
	for id, ok := range approvals {
		if ok {
			rev.StillValidRules = append(rev.StillValidRules, id)
		}
	}
	sortStrings(rev.StillValidRules)
	if step.After.State != "" && step.After.State != step.Before.State {
		if strings.Contains(strings.ToLower(rev.Believed), "online") || strings.Contains(strings.ToLower(rev.Believed), "required") {
			rev.Refuted = append(rev.Refuted, firstHypothesisID(ctx.Hypotheses, "BH-online"))
		}
	}
	if len(rev.Refuted) == 0 && rev.Believed != "" && step.After.State != "" {
		rev.Refuted = append(rev.Refuted, firstHypothesisID(ctx.Hypotheses, rev.Believed))
	}
	rev.Open = append(rev.Open, "whether the new state is specified")
	for _, fact := range ctx.Facts {
		if fact.Class == factBusiness {
			rev.BusinessDecisions = append(rev.BusinessDecisions, fact.ID)
		}
		if len(fact.Refs) > 0 {
			rev.EvidenceRefs = appendUnique(rev.EvidenceRefs, fact.Refs[0].ID)
		}
	}
	return rev
}

func firstHypothesisID(items []BoundaryHypothesis, hint string) string {
	for _, item := range items {
		if item.ID == hint || strings.Contains(item.Assumption, hint) {
			return item.ID
		}
	}
	if hint != "" {
		return hint
	}
	if len(items) > 0 {
		return items[0].ID
	}
	return "BH-unknown"
}

func applyHypothesisVerdict(approvals map[string]bool, hyp BoundaryHypothesis) error {
	if approvals == nil {
		return errors.New("approvals required")
	}
	_ = hyp
	return nil
}

func attachDiscoveryToPlanner(in PlannerContext, disc DiscoveryContext) PlannerContext {
	in.Facts = disc.Facts
	in.Hypotheses = disc.Hypotheses
	in.Explanations = disc.Explanations
	in.Experiments = disc.Experiments
	in.Revisions = disc.Revisions
	in.Authorization = disc.Authorization
	in.UnresolvedQuestions = disc.Questions
	for _, hyp := range disc.Hypotheses {
		if hyp.Status == hypRefuted {
			in.RefutedHypotheses = appendUnique(in.RefutedHypotheses, hyp.ID)
		}
	}
	if len(in.Authorization) == 0 {
		in.Authorization = append(in.Authorization, in.ConfirmedRules...)
	}
	return in
}

func refreshBoundaryDiscovery(prev DiscoveryContext, g Graph, approvals map[string]bool, adapter ExploreAdapter, report ExploreReport, entries []DiscoveredEntry) DiscoveryContext {
	next := buildEvidenceContext(g, approvals, ExploreAdapter{}, report, adapter, entries)
	next.Hypotheses = mergeHypotheses(prev.Hypotheses, proposeCounterfactuals(next, g, adapter))
	next.Revisions = prev.Revisions
	next.Explanations = prev.Explanations
	next.Experiments = prev.Experiments
	if obs, ok := ambiguousObservation(report); ok {
		next.Explanations = competeExplanations(obs, next.Facts)
		next.Experiments = []ExperimentCandidate{planMinimalExperiment(next.Explanations, adapter)}
		applyObservations(&next, report, obs)
	}
	if len(report.Steps) > 0 {
		next.Revisions = append(next.Revisions, reviseUnderstanding(next, report.Steps[len(report.Steps)-1], approvals))
	}
	return next
}

func mergeHypotheses(prev, next []BoundaryHypothesis) []BoundaryHypothesis {
	byKey := map[string]BoundaryHypothesis{}
	for _, item := range prev {
		byKey[hypothesisKey(item)] = item
	}
	out := []BoundaryHypothesis{}
	seen := map[string]bool{}
	for _, item := range next {
		key := hypothesisKey(item)
		if old, ok := byKey[key]; ok {
			item.ID = old.ID
			item.Version = old.Version
			if old.Status != "" && old.Status != hypProposed {
				item.Status = old.Status
			}
		}
		out = append(out, item)
		seen[key] = true
	}
	for key, old := range byKey {
		if !seen[key] {
			out = append(out, old)
		}
	}
	return out
}

func hypothesisKey(item BoundaryHypothesis) string {
	if item.Identity.Action != "" || item.Identity.Invariant != "" {
		return strings.Join([]string{item.Identity.Role, item.Identity.Action, item.Identity.Resource, item.Identity.State, item.Identity.Invariant}, "|")
	}
	return item.Assumption + "|" + item.Counterfactual
}

func ambiguousObservation(report ExploreReport) (AmbiguousObservation, bool) {
	codes := 0
	for _, step := range report.Steps {
		for _, call := range step.Calls {
			if call.StatusCode == 200 {
				codes++
			}
		}
	}
	if codes < 2 {
		return AmbiguousObservation{}, false
	}
	owners := currentOwners(report)
	if len(owners) == 0 {
		owners = []string{"unknown"}
	}
	return AmbiguousObservation{HTTPStatus: 200, Owners: owners, Resources: []string{"same", "same"}, SameResource: true}, true
}

func currentOwners(report ExploreReport) []string {
	if len(report.Steps) == 0 {
		return nil
	}
	owner := workOwner(report.Steps[len(report.Steps)-1].After)
	if owner == "" {
		return nil
	}
	return []string{owner}
}

func workOwner(facts IndependentFacts) string {
	if raw, ok := facts.Values["owner"]; ok {
		var owner string
		if json.Unmarshal(raw, &owner) == nil && owner != "" {
			return owner
		}
	}
	return ""
}

func applyObservations(disc *DiscoveryContext, report ExploreReport, obs AmbiguousObservation) {
	actors := distinctActors(report)
	owners := uniqueNonEmpty(obs.Owners)
	lastOwner := ""
	if len(owners) == 1 {
		lastOwner = owners[0]
	}
	for i, item := range disc.Explanations {
		switch item.ID {
		case "EX-dual":
			if len(owners) == 1 {
				disc.Explanations[i].Status = hypRefuted
			} else if len(owners) >= 2 {
				disc.Explanations[i].Status = hypSupported
			} else {
				disc.Explanations[i].Status = hypInconclusive
			}
		case "EX-last":
			if len(owners) == 1 && lastOwner != "" && len(actors) >= 2 {
				disc.Explanations[i].Status = hypSupported
			} else if len(owners) >= 2 {
				disc.Explanations[i].Status = hypRefuted
			} else {
				disc.Explanations[i].Status = hypInconclusive
			}
		case "EX-idem":
			if versionGrew(report) {
				disc.Explanations[i].Status = hypRefuted
			} else if len(report.Steps) >= 2 {
				disc.Explanations[i].Status = hypSupported
			}
		case "EX-async":
			if lastOwner != "" {
				disc.Explanations[i].Status = hypRefuted
			}
		}
	}
	for i, hyp := range disc.Hypotheses {
		if !strings.Contains(hyp.Assumption, "single owner") {
			continue
		}
		if len(owners) == 1 && lastOwner != "" && len(actors) >= 2 {
			disc.Hypotheses[i].Status = hypSupported
			disc.Questions = appendUnique(disc.Questions, "BUSINESS_DECISION: is last-writer ownership intended after two HTTP 200 claims?")
		}
	}
}

func distinctActors(report ExploreReport) []string {
	out := []string{}
	for _, step := range report.Steps {
		out = appendUnique(out, step.Candidate.Role)
		for _, call := range step.Calls {
			out = appendUnique(out, call.Role)
		}
	}
	return out
}

func uniqueNonEmpty(values []string) []string {
	out := []string{}
	for _, value := range values {
		if value != "" && value != "unknown" {
			out = appendUnique(out, value)
		}
	}
	return out
}

func versionGrew(report ExploreReport) bool {
	var prev json.RawMessage
	for _, step := range report.Steps {
		cur := step.After.Values["version"]
		if len(prev) > 0 && len(cur) > 0 && string(cur) != string(prev) {
			return true
		}
		if len(cur) > 0 {
			prev = cur
		}
	}
	return false
}

func preferDiscoveryCandidates(candidates []CandidateAction, disc DiscoveryContext, world exploreWorld) []CandidateAction {
	if len(candidates) < 2 || !discoveryWantsOtherSubject(disc, world) {
		return candidates
	}
	preferred := []CandidateAction{}
	rest := []CandidateAction{}
	for _, item := range candidates {
		other := item.Kind == kindRole || item.Kind == kindConcurrent || world.role != "" && item.Role != "" && item.Role != world.role && item.Kind != kindRecover
		if other {
			preferred = append(preferred, item)
			continue
		}
		rest = append(rest, item)
	}
	if len(preferred) == 0 {
		return candidates
	}
	return append(preferred, rest...)
}

func discoveryWantsOtherSubject(disc DiscoveryContext, world exploreWorld) bool {
	if len(disc.Explanations) >= 2 {
		return true
	}
	if world.lastAction == "" && world.state != "" {
		return false
	}
	for _, hyp := range disc.Hypotheses {
		if hyp.Status == hypSupported || hyp.Status == hypRefuted {
			continue
		}
		if strings.Contains(hyp.Counterfactual, "two subjects") || strings.Contains(hyp.Counterfactual, "other authorized") {
			return world.lastAction != ""
		}
	}
	return false
}

func scoreBoundaryContrast(mode string, report ExploreReport) BoundaryContrastRow {
	row := BoundaryContrastRow{Mode: mode, Actions: report.Actions, PlannerCalls: report.Spent.PlannerCalls, Unspecified: len(report.Unspecified)}
	actors := distinctActors(report)
	for _, step := range report.Steps {
		for _, call := range step.Calls {
			if call.StatusCode == 200 {
				row.HTTP200s++
			}
		}
	}
	owners := currentOwners(report)
	if len(owners) == 1 {
		row.LastOwner = owners[0]
	}
	row.DistinctActors = len(actors)
	row.DistinctOwners = len(owners)
	if report.Boundary != nil {
		row.Explanations = len(report.Boundary.Explanations)
		for _, item := range report.Boundary.Explanations {
			if item.Status == hypSupported {
				row.SupportedExplanations = append(row.SupportedExplanations, item.ID)
			}
			if item.Status == hypSupported && item.ID == "EX-dual" && row.DistinctOwners == 1 {
				row.FalsePositives++
			}
			if item.Status == hypSupported && item.ID == "EX-last" && row.DistinctActors >= 2 && row.DistinctOwners == 1 {
				row.UniqueDiscoveries++
			}
		}
		if row.HTTP200s >= 2 && row.Explanations == 0 {
			row.HTTPOnlyJudgements = 1
		}
		for _, finding := range report.Blocked {
			if strings.Contains(finding.Reason, "not authorized") || strings.Contains(finding.Reason, "allow-list") {
				row.BlockedUnauthorized++
			}
		}
	} else if row.HTTP200s >= 2 {
		row.HTTPOnlyJudgements = 1
	}
	if mode == "ai" && row.PlannerCalls < 1 {
		row.Notes = "planner_calls=0; no scripted planner used; host Agent reasoning is documented separately"
	}
	if mode == "ai" && row.PlannerCalls >= 1 {
		row.Notes = "scripted comparison fixture; not evidence of host Agent reasoning"
	}
	return row
}

type BoundaryContrastRow struct {
	Mode                  string   `json:"mode"`
	Actions               int      `json:"actions"`
	PlannerCalls          int      `json:"planner_calls"`
	HTTP200s              int      `json:"http_200s"`
	DistinctActors        int      `json:"distinct_actors"`
	DistinctOwners        int      `json:"distinct_owners"`
	LastOwner             string   `json:"last_owner,omitempty"`
	Explanations          int      `json:"explanations"`
	SupportedExplanations []string `json:"supported_explanations,omitempty"`
	UniqueDiscoveries     int      `json:"unique_discoveries"`
	FalsePositives        int      `json:"false_positives"`
	Unspecified           int      `json:"unspecified"`
	HTTPOnlyJudgements    int      `json:"http_only_judgements"`
	BlockedUnauthorized   int      `json:"blocked_unauthorized"`
	Notes                 string   `json:"notes,omitempty"`
}

func (d db) persistDiscoveryContext(campaign string, disc DiscoveryContext) error {
	if campaign == "" {
		return errors.New("discovery persist needs a campaign")
	}
	if err := d.ensureDiscoveryTables(); err != nil {
		return err
	}
	body, err := json.Marshal(disc)
	if err != nil {
		return err
	}
	return d.tx([]string{fmt.Sprintf(
		"INSERT OR REPLACE INTO exploration_boundary(campaign,body) VALUES(%s,%s);",
		quote(campaign), quote(string(body)),
	)})
}

func (d db) loadDiscoveryContext(campaign string) (DiscoveryContext, error) {
	var out DiscoveryContext
	if err := d.ensureDiscoveryTables(); err != nil {
		return out, err
	}
	rows, err := d.query("SELECT body FROM exploration_boundary WHERE campaign = "+quote(campaign), 1)
	if err != nil {
		return out, err
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		return out, errors.New("discovery context missing")
	}
	if err := json.Unmarshal([]byte(rows[0][0]), &out); err != nil {
		return out, err
	}
	return out, nil
}

func (d db) ensureDiscoveryTables() error {
	_, err := d.handle.Exec("CREATE TABLE IF NOT EXISTS exploration_boundary (campaign TEXT PRIMARY KEY, body TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);")
	return err
}

func discoveryFactID(kind, key string) string {
	return fmt.Sprintf("FACT-%s-%s", kind, shortDigest(key)[:8])
}
