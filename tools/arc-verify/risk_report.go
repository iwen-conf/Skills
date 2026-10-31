package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Risk decisions are append-only events beside the unchanged risks table:
// a risk without an event is OPEN; ACCEPTED_RISK and PROMOTED are terminal.
type RiskDecision struct {
	Status  string `json:"status"`
	Scope   string `json:"scope,omitempty"`
	Quote   string `json:"quote,omitempty"`
	Rule    string `json:"rule,omitempty"`
	Created string `json:"created"`
}

type RiskEntry struct {
	ID           string         `json:"id"`
	Description  string         `json:"description"`
	Action       string         `json:"action"`
	Origin       string         `json:"origin"`
	Reference    string         `json:"reference,omitempty"`
	Status       string         `json:"status"`
	Scope        string         `json:"scope,omitempty"`
	Quote        string         `json:"quote,omitempty"`
	Rule         string         `json:"rule,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	History      []RiskDecision `json:"history"`
	// Refutation is the latest bounded attempt to disprove the risk:
	// not_attempted, supported, refuted, not_found, or evidence_changed.
	Refutation         string              `json:"refutation"`
	RefutationEvidence string              `json:"refutation_evidence,omitempty"`
	RefutationSHA256   string              `json:"refutation_sha256,omitempty"`
	RefutationHistory  []RefutationAttempt `json:"refutation_history,omitempty"`
}

// RefutationAttempt is one registered attempt with its evidence binding.
type RefutationAttempt struct {
	Result   string `json:"result"`
	Evidence string `json:"evidence"`
	SHA256   string `json:"sha256"`
	Created  string `json:"created"`
}

type RiskReport struct {
	Graph            string              `json:"graph"`
	Risks            []RiskEntry         `json:"risks"`
	PendingDecisions []string            `json:"pending_decisions"`
	DerivedRisks     []DerivedObligation `json:"derived_risks"`
	UnspecifiedRisks []UnspecifiedItem   `json:"unspecified_risks"`
	ReferenceRisks   []RiskEntry         `json:"reference_risks"`
	ThreeWay         ThreeWayReport      `json:"three_way"`
	TriageSummary    []TriageSummary     `json:"triage_summary"`
}

// ThreeWayEntry is one difference between two of the three sources.
type ThreeWayEntry struct {
	Source   string `json:"source"`
	Subject  string `json:"subject"`
	Detail   string `json:"detail"`
	Evidence string `json:"evidence"`
	SHA256   string `json:"sha256,omitempty"`
}

// ThreeWayReport keeps requirements, implementation evidence and peer
// references in separate columns; it neither scores nor judges them.
type ThreeWayReport struct {
	RequiredNotImplemented []ThreeWayEntry `json:"required_not_implemented"`
	ImplementedNotRequired []ThreeWayEntry `json:"implemented_not_required"`
	ReferencedNotRequired  []ThreeWayEntry `json:"referenced_not_required"`
}

const effectiveRiskStatusSQL = "COALESCE((SELECT e.status FROM risk_events e WHERE e.graph=r.graph AND e.risk=r.id ORDER BY e.seq DESC LIMIT 1),'OPEN')"

func validateRiskOrigin(r Risk) error {
	switch r.Origin {
	case "", "ai_inference", "industry_practice":
		if r.Reference != "" {
			return fmt.Errorf("risk %s reference requires origin peer_reference", r.ID)
		}
	case "peer_reference":
		if strings.TrimSpace(r.Reference) == "" {
			return fmt.Errorf("risk %s origin peer_reference needs a reference entry", r.ID)
		}
	default:
		return fmt.Errorf("risk %s has unknown origin %q", r.ID, r.Origin)
	}
	return nil
}

// riskRows lists id, description and effective status for read-only packets.
func (d db) riskRows(graph string, limit int) ([][]string, error) {
	statement := "SELECT " + fieldsSQL("r.id", "r.description", effectiveRiskStatusSQL) + " FROM risks r WHERE r.graph=" + quote(graph) + " ORDER BY r.id"
	if limit > 0 {
		statement += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := d.query(statement+";", 3)
	if err != nil {
		return nil, fmt.Errorf("risk decision schema unavailable; run arc-verify init: %w", err)
	}
	return rows, nil
}

func (d db) openRisks(graph string) (int, error) {
	rows, err := d.query("SELECT "+fieldsSQL("CAST(COUNT(*) AS TEXT)")+" FROM risks r WHERE r.graph="+quote(graph)+" AND NOT EXISTS (SELECT 1 FROM risk_events e WHERE e.graph=r.graph AND e.risk=r.id);", 1)
	if err != nil {
		return 0, fmt.Errorf("risk decision schema unavailable; run arc-verify init: %w", err)
	}
	open := 0
	if len(rows) == 1 {
		if _, err = fmt.Sscan(rows[0][0], &open); err != nil {
			return 0, err
		}
	}
	return open, nil
}

func (d db) riskReport(graph string) (RiskReport, error) {
	if graph == "" {
		return RiskReport{}, errors.New("graph id required")
	}
	g, err := d.model(graph)
	if err != nil {
		return RiskReport{}, err
	}
	rows, err := d.query("SELECT risk,status,scope,quote,rule,created FROM risk_events WHERE graph="+quote(graph)+" ORDER BY seq;", 6)
	if err != nil {
		return RiskReport{}, fmt.Errorf("risk decision schema unavailable; run arc-verify init: %w", err)
	}
	history := map[string][]RiskDecision{}
	for _, row := range rows {
		history[row[0]] = append(history[row[0]], RiskDecision{Status: row[1], Scope: row[2], Quote: row[3], Rule: row[4], Created: row[5]})
	}
	report := RiskReport{Graph: graph, Risks: []RiskEntry{}, PendingDecisions: []string{}}
	if report.DerivedRisks, err = d.inferredDerivations(graph); err != nil {
		return RiskReport{}, err
	}
	if report.UnspecifiedRisks, err = d.triagedUnspecified(graph, "should_not_exist"); err != nil {
		return RiskReport{}, err
	}
	if report.ReferenceRisks, err = d.referenceRisks(graph); err != nil {
		return RiskReport{}, err
	}

	for _, risk := range g.Risks {
		entry := RiskEntry{ID: risk.ID, Description: risk.Description, Action: risk.Action, Origin: risk.Origin, Reference: risk.Reference,
			Status: "OPEN", Capabilities: actionCapabilities(g, risk.Action), History: history[risk.ID]}
		if entry.Origin == "" {
			entry.Origin = "undeclared"
		}
		if entry.History == nil {
			entry.History = []RiskDecision{}
		}
		if n := len(entry.History); n > 0 {
			last := entry.History[n-1]
			entry.Status, entry.Scope, entry.Quote, entry.Rule = last.Status, last.Scope, last.Quote, last.Rule
		}
		if entry.Status == "OPEN" {
			report.PendingDecisions = append(report.PendingDecisions, risk.ID)
		}
		report.Risks = append(report.Risks, entry)
	}
	for _, entries := range [][]RiskEntry{report.Risks, report.ReferenceRisks} {
		if err := d.annotateRefutations(graph, entries); err != nil {
			return RiskReport{}, err
		}
	}
	if report.ThreeWay, err = d.threeWay(g, report.ReferenceRisks); err != nil {
		return RiskReport{}, err
	}
	if report.TriageSummary, err = d.triageSummary(graph, report.Risks); err != nil {
		return RiskReport{}, err
	}
	return report, nil
}

func (d db) acceptRisk(graph, risk, scope, words string) error {
	scope, words = strings.TrimSpace(scope), strings.TrimSpace(words)
	if scope == "" || words == "" {
		return errors.New("accepting a risk needs an explicit scope and the user's own words")
	}
	return d.decideRisk(graph, risk, "ACCEPTED_RISK", scope, words, "")
}

func (d db) promoteRisk(graph, risk, rule string) error {
	if rule == "" {
		return errors.New("promotion needs the approved rule id")
	}
	rows, err := d.query("SELECT "+fieldsSQL("CAST(approved AS TEXT)")+" FROM rules WHERE graph="+quote(graph)+" AND id="+quote(rule)+";", 1)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("rule %s not found in graph %s", rule, graph)
	}
	if rows[0][0] != "1" {
		return fmt.Errorf("rule %s is not approved by the user; a risk cannot be promoted to it", rule)
	}
	return d.decideRisk(graph, risk, "PROMOTED", "", "", rule)
}

func (d db) decideRisk(graph, risk, status, scope, words, rule string) error {
	if graph == "" || risk == "" {
		return errors.New("graph and risk ids required")
	}
	rows, err := d.query("SELECT "+fieldsSQL(effectiveRiskStatusSQL)+" FROM risks r WHERE r.graph="+quote(graph)+" AND r.id="+quote(risk)+";", 1)
	if err != nil {
		return fmt.Errorf("risk decision schema unavailable; run arc-verify init: %w", err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("risk %s not found in graph %s", risk, graph)
	}
	if rows[0][0] != "OPEN" {
		return fmt.Errorf("risk %s already decided as %s", risk, rows[0][0])
	}
	return d.transition([]string{fmt.Sprintf(
		"INSERT INTO risk_events(graph,risk,status,scope,quote,rule) SELECT %s,%s,%s,%s,%s,%s WHERE NOT EXISTS (SELECT 1 FROM risk_events WHERE graph=%s AND risk=%s);",
		quote(graph), quote(risk), quote(status), quote(scope), quote(words), quote(rule), quote(graph), quote(risk))})
}

// acceptanceReport is the confirmed-rule view: gate results plus candidate
// implementation gaps, with every risk figure removed.
func (d db) acceptanceReport(graph string) (map[string]any, error) {
	result, err := d.gate(graph)
	if err != nil {
		return nil, err
	}
	delete(result, "open_risks")
	gaps, err := d.gaps(graph)
	if err != nil {
		return nil, err
	}
	candidates := []map[string]string{}
	for _, row := range gaps {
		if row[2] != "PRESENT" {
			candidates = append(candidates, map[string]string{"rule": row[0], "action": row[1], "observation": row[2]})
		}
	}
	result["candidate_gaps"] = candidates
	return result, nil
}

// applyRiskReview blocks the gate on undecided risks only when the project asks for it.
func applyRiskReview(result map[string]any, enabled bool) {
	if !enabled {
		return
	}
	if open, _ := result["open_risks"].(int); open > 0 {
		result["verdict"] = "FAIL"
		result["risk_review_blocked"] = true
	}
}

// interactiveAnswers reads one line per prompt from a human terminal; pipes are refused.
func interactiveAnswers(prompts ...string) ([]string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return nil, errors.New("this decision needs an interactive terminal; a pipe is not allowed")
	}
	reader := bufio.NewReader(os.Stdin)
	answers := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		fmt.Fprint(os.Stderr, prompt)
		line, err := reader.ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			return nil, errors.New("decision input incomplete")
		}
		answers = append(answers, strings.TrimSpace(line))
	}
	return answers, nil
}

// threeWay compares the three sources without merging them: requirements
// (rules) against implementation evidence (scans, exploration) and peer
// references. NOT_FOUND stays a candidate gap; nothing here becomes a rule.
func (d db) threeWay(g Graph, references []RiskEntry) (ThreeWayReport, error) {
	report := ThreeWayReport{RequiredNotImplemented: []ThreeWayEntry{}, ImplementedNotRequired: []ThreeWayEntry{}, ReferencedNotRequired: []ThreeWayEntry{}}
	scans, err := d.latestScans(g.ID)
	if err != nil {
		return report, err
	}
	required := map[string]bool{}
	rules := append([]Rule{}, g.Rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	for _, rule := range rules {
		required[rule.Action] = true
		if scan, found := scans[rule.Action]; found && scan.Status == "NOT_FOUND" {
			report.RequiredNotImplemented = append(report.RequiredNotImplemented, ThreeWayEntry{Source: "requirement", Subject: rule.ID,
				Detail: "action " + rule.Action + " has no implementation entry (candidate gap; MISSING needs a widened search and user confirmation)", Evidence: scan.Evidence, SHA256: scan.SHA256})
		}
	}
	actions := make([]string, 0, len(scans))
	for action := range scans {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	for _, action := range actions {
		if scan := scans[action]; scan.Status == "PRESENT" && !required[action] {
			report.ImplementedNotRequired = append(report.ImplementedNotRequired, ThreeWayEntry{Source: "implementation", Subject: action, Detail: "implementation entry found that no rule requires", Evidence: scan.Evidence, SHA256: scan.SHA256})
		}
	}
	items, err := d.unspecified(g.ID)
	if err != nil {
		return report, err
	}
	for _, item := range items {
		report.ImplementedNotRequired = append(report.ImplementedNotRequired, ThreeWayEntry{Source: "implementation", Subject: item.Action, Detail: item.Kind + ": " + item.Reason + " · triage " + item.Triage, Evidence: item.Evidence, SHA256: item.EvidenceSHA256})
	}
	for _, risk := range references {
		report.ReferencedNotRequired = append(report.ReferencedNotRequired, ThreeWayEntry{Source: "peer_reference", Subject: risk.ID, Detail: risk.Reference + " · " + risk.Description, Evidence: "reference catalog " + risk.Reference})
	}
	return report, nil
}
