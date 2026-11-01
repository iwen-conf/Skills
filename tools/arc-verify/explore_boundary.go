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
	// Preserve already-recorded hypotheses for compatibility. This CLI does
	// NOT invent domain hypotheses, interpretations or conclusions from HTTP
	// status codes. The current coding Agent documents questions in Markdown;
	// the runtime reports only source/model facts and independent observations.
	next.Hypotheses = prev.Hypotheses
	next.Revisions = prev.Revisions
	next.Explanations = prev.Explanations
	next.Experiments = prev.Experiments
	return next
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
