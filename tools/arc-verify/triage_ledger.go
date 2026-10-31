package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// TriageSummary counts the user's decisions on AI-produced candidates of one
// origin. It is for improving the generators, never a gate or a quality score.
type TriageSummary struct {
	Origin             string `json:"origin"`
	Accepted           int    `json:"accepted"`
	Rejected           int    `json:"rejected"`
	Pending            int    `json:"pending"`
	Sample             int    `json:"sample"`
	InsufficientSample bool   `json:"insufficient_sample"`
}

// rejectRisk records the user's judgment that a candidate is not a problem. A
// declared risk gets a REJECTED event (W12 state machine); a peer reference
// risk is remembered with its bindings so the same match stays suppressed.
func (d db) rejectRisk(graph, risk, words string) error {
	words = strings.TrimSpace(words)
	if graph == "" || risk == "" || words == "" {
		return errors.New("rejecting a risk needs graph, risk and the user's own words")
	}
	if !strings.HasPrefix(risk, "ref/") {
		return d.decideRisk(graph, risk, "REJECTED", "", words, "")
	}
	entry := strings.TrimPrefix(risk, "ref/")
	current, err := d.query("SELECT entry FROM reference_current WHERE graph="+quote(graph)+" AND risk="+quote(risk), 1)
	if err != nil {
		return err
	}
	if len(current) != 1 {
		return fmt.Errorf("reference risk %s is not a current candidate of %s", risk, graph)
	}
	bindings, err := d.query("SELECT detail FROM reference_matches WHERE graph="+quote(graph)+" AND entry="+quote(entry)+" AND outcome='RISK' ORDER BY seq DESC LIMIT 1", 1)
	if err != nil {
		return err
	}
	if len(bindings) != 1 {
		return fmt.Errorf("reference risk %s has no recorded binding", risk)
	}
	return d.transition([]string{
		fmt.Sprintf("INSERT INTO reference_rejections(graph,entry,bindings,words) SELECT %s,%s,%s,%s WHERE NOT EXISTS (SELECT 1 FROM reference_rejections WHERE graph=%s AND entry=%s AND bindings=%s);", quote(graph), quote(entry), quote(bindings[0][0]), quote(words), quote(graph), quote(entry), quote(bindings[0][0])),
		fmt.Sprintf("DELETE FROM reference_current WHERE graph=%s AND risk=%s;", quote(graph), quote(risk)),
	})
}

func (d db) referenceRejections(graph string) (map[string]string, error) {
	rows, err := d.query("SELECT entry,bindings,words FROM reference_rejections WHERE graph="+quote(graph), 3)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, row := range rows {
		result[row[0]+"|"+row[1]] = row[2]
	}
	return result, nil
}

// triageSummary groups decisions by where the candidate came from; fewer
// than ten candidates are flagged so a ratio is never read as a trend.
func (d db) triageSummary(graph string, risks []RiskEntry) ([]TriageSummary, error) {
	rows := map[string]*TriageSummary{}
	count := func(origin, outcome string) {
		row := rows[origin]
		if row == nil {
			row = &TriageSummary{Origin: origin}
			rows[origin] = row
		}
		switch outcome {
		case "accepted":
			row.Accepted++
		case "rejected":
			row.Rejected++
		default:
			row.Pending++
		}
		row.Sample++
	}
	for _, risk := range risks {
		switch risk.Status {
		case "ACCEPTED_RISK", "PROMOTED":
			count(risk.Origin, "accepted")
		case "REJECTED":
			count(risk.Origin, "rejected")
		default:
			count(risk.Origin, "pending")
		}
	}
	references, err := d.referenceRisks(graph)
	if err != nil {
		return nil, err
	}
	for range references {
		count("peer_reference", "pending")
	}
	rejected, err := d.referenceRejections(graph)
	if err != nil {
		return nil, err
	}
	for range rejected {
		count("peer_reference", "rejected")
	}
	items, err := d.unspecified(graph)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		switch item.Triage {
		case "missing_requirement", "should_not_exist":
			count("discovery", "accepted")
		case "allowed_extension":
			count("discovery", "rejected")
		default:
			count("discovery", "pending")
		}
	}
	derived, err := d.inferredDerivations(graph)
	if err != nil {
		return nil, err
	}
	for range derived {
		count("derived_inference", "pending")
	}
	result := []TriageSummary{}
	for _, row := range rows {
		row.InsufficientSample = row.Sample < 10
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Origin < result[j].Origin })
	return result, nil
}
