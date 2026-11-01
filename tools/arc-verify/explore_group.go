package main

import (
	"errors"
)

var errOutcomeUnknown = errors.New("business action outcome is unknown")

type ExploreOperationGroup struct {
	ID        string              `json:"id"`
	Kind      string              `json:"kind"`
	Members   []ExploreCallRecord `json:"members"`
	Partial   bool                `json:"partial"`
	Unknown   bool                `json:"unknown"`
	AllDenied bool                `json:"all_denied"`
}

func aggregateExploreGroup(kind string, partners []CandidateAction, results []struct {
	out AdapterOutput
	err error
}) (AdapterOutput, []ExploreCallRecord, ExploreOperationGroup, error) {
	records := make([]ExploreCallRecord, len(partners))
	ok, denied, unknown := 0, 0, 0
	for i := range partners {
		records[i] = callRecord(partners[i], results[i].out, results[i].err)
		records[i].Resource = firstNonEmpty(partners[i].Scope, partners[i].Entry)
		records[i].Scope = partners[i].Scope
		records[i].IdempotencyKey = partners[i].RequestID
		timedOut := isTimeout(results[i].err)
		records[i].SideEffect = !timedOut && results[i].err == nil && !results[i].out.Denied
		if timedOut {
			unknown++
			continue
		}
		if results[i].err != nil || results[i].out.Denied {
			denied++
			continue
		}
		ok++
	}
	group := ExploreOperationGroup{
		ID:        firstNonEmpty(partners[0].RequestID, "group"),
		Kind:      kind,
		Members:   records,
		Partial:   ok > 0 && (denied > 0 || unknown > 0),
		Unknown:   unknown > 0,
		AllDenied: ok == 0 && unknown == 0,
	}
	output := AdapterOutput{Denied: ok == 0}
	if len(results) == 1 {
		output = results[0].out
	}
	if unknown == len(results) {
		return output, records, group, errOutcomeUnknown
	}
	if unknown > 0 {
		return output, records, group, errOutcomeUnknown
	}
	return output, records, group, nil
}

func groupFindings(group ExploreOperationGroup) []ExploreFinding {
	findings := []ExploreFinding{}
	if group.Unknown {
		findings = append(findings, ExploreFinding{Kind: "BLOCKED", Reason: "concurrent group has an unknown outcome; replay is forbidden"})
	}
	if group.Partial {
		findings = append(findings, ExploreFinding{Kind: "PARTIAL", Reason: "concurrent group had mixed success and rejection; one member must not stand for the group"})
	}
	for _, member := range group.Members {
		if member.Error != "" {
			findings = append(findings, ExploreFinding{Kind: "RISK", Reason: "member " + member.Role + " " + member.RequestID + ": " + member.Error, Evidence: member.ResponseSHA256})
		}
	}
	return findings
}
