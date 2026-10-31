package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func judgeClaimGraph() Graph {
	return Graph{
		Rules: []Rule{
			{ID: "R-CLAIM", Action: "claim", Actor: "STUDENT", Conditions: []string{"login", "course_open"}, Expected: "pending", Rejected: "UNCHANGED"},
			{ID: "R-FORCE", Action: "force", Actor: "ADMIN", Conditions: []string{"login", "is_admin"}, Expected: "enrolled", Rejected: "UNCHANGED"},
			{ID: "R-NO-FORCE", Action: "force", Actor: "STUDENT", Conditions: []string{"login", "is_admin"}, Expected: "enrolled", Rejected: "UNCHANGED"},
		},
		Invariants: []BusinessInvariant{{
			ID:     "invariant.capacity",
			Checks: []PropertyCheck{{Left: "records", Operator: compareLessEqual, Right: "capacity"}},
		}},
	}
}

func judgeFacts(state string, conds map[string]bool, records, capacity int) IndependentFacts {
	return IndependentFacts{
		State:      state,
		Conditions: ObservedConditions(conds),
		Values: map[string]json.RawMessage{
			"records":  json.RawMessage(mustJSONNumber(records)),
			"capacity": json.RawMessage(mustJSONNumber(capacity)),
		},
	}
}

func mustJSONNumber(n int) []byte {
	body, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	return body
}

func findingKinds(findings []ExploreFinding, rule, kind string) []ExploreFinding {
	out := []ExploreFinding{}
	for _, item := range findings {
		if (rule == "" || item.Rule == rule) && (kind == "" || item.Kind == kind) {
			out = append(out, item)
		}
	}
	return out
}

func TestJudgeExplorationAllowedRequestRejectedIsFail(t *testing.T) {
	g := judgeClaimGraph()
	approvals := map[string]bool{"R-CLAIM": true, "R-FORCE": true, "R-NO-FORCE": true}
	before := judgeFacts("idle", map[string]bool{"login": true, "course_open": true}, 0, 1)
	after := judgeFacts("idle", map[string]bool{"login": true, "course_open": true}, 0, 1)
	findings := judgeExploration(g, approvals, "STUDENT", "claim", before, after, true)
	if len(findingKinds(findings, "R-CLAIM", "FAIL")) == 0 {
		t.Fatalf("allowed claim wrongly rejected must FAIL R-CLAIM, got %+v", findings)
	}
}

func TestJudgeExplorationCorrectAllowIsPass(t *testing.T) {
	g := judgeClaimGraph()
	approvals := map[string]bool{"R-CLAIM": true}
	before := judgeFacts("idle", map[string]bool{"login": true, "course_open": true}, 0, 1)
	after := judgeFacts("pending", map[string]bool{"login": true, "course_open": true}, 1, 1)
	findings := judgeExploration(g, approvals, "STUDENT", "claim", before, after, false)
	if len(findingKinds(findings, "R-CLAIM", "PASS")) == 0 {
		t.Fatalf("allowed claim that reached pending must PASS R-CLAIM, got %+v", findings)
	}
	if len(findingKinds(findings, "R-CLAIM", "FAIL")) != 0 {
		t.Fatalf("correct allow must not also FAIL: %+v", findings)
	}
}

func TestJudgeExplorationCorrectRejectIsPass(t *testing.T) {
	g := judgeClaimGraph()
	approvals := map[string]bool{"R-NO-FORCE": true}
	before := judgeFacts("idle", map[string]bool{"login": true, "is_admin": false}, 0, 1)
	after := judgeFacts("idle", map[string]bool{"login": true, "is_admin": false}, 0, 1)
	findings := judgeExploration(g, approvals, "STUDENT", "force", before, after, true)
	if len(findingKinds(findings, "R-NO-FORCE", "PASS")) == 0 {
		t.Fatalf("student force correctly rejected without side effects must PASS, got %+v", findings)
	}
}

func TestJudgeExplorationRejectWithSideEffectIsFail(t *testing.T) {
	g := judgeClaimGraph()
	approvals := map[string]bool{"R-NO-FORCE": true}
	before := judgeFacts("idle", map[string]bool{"login": true, "is_admin": false}, 0, 1)
	after := judgeFacts("enrolled", map[string]bool{"login": true, "is_admin": false}, 1, 1)
	findings := judgeExploration(g, approvals, "STUDENT", "force", before, after, true)
	if len(findingKinds(findings, "R-NO-FORCE", "FAIL")) == 0 {
		t.Fatalf("rejected force that still enrolled must FAIL, got %+v", findings)
	}
}

func TestJudgeExplorationAdminForceDoesNotUseStudentDenyRule(t *testing.T) {
	g := judgeClaimGraph()
	approvals := map[string]bool{"R-FORCE": true, "R-NO-FORCE": true}
	before := judgeFacts("idle", map[string]bool{"login": true, "is_admin": true}, 0, 1)
	after := judgeFacts("enrolled", map[string]bool{"login": true, "is_admin": true}, 1, 1)
	findings := judgeExploration(g, approvals, "ADMIN", "force", before, after, false)
	if len(findingKinds(findings, "R-NO-FORCE", "FAIL")) != 0 {
		t.Fatalf("ADMIN force must not be judged by R-NO-FORCE: %+v", findings)
	}
	if len(findingKinds(findings, "R-FORCE", "PASS")) == 0 {
		t.Fatalf("ADMIN force that reached enrolled must PASS R-FORCE, got %+v", findings)
	}
}

func TestInvariantFindingsKeepsBlockedWhenOracleMissing(t *testing.T) {
	g := judgeClaimGraph()
	after := IndependentFacts{State: "pending", Values: map[string]json.RawMessage{}}
	findings := invariantFindings(g, after)
	if len(findingKinds(findings, "invariant.capacity", "BLOCKED")) == 0 {
		t.Fatalf("missing measurements must stay BLOCKED, not remapped to FAIL: %+v", findings)
	}
	if len(findingKinds(findings, "invariant.capacity", "FAIL")) != 0 {
		t.Fatalf("unobservable invariant must not be a product FAIL: %+v", findings)
	}
}

func TestPerformExploreSourceKeepsBothResults(t *testing.T) {
	body, err := os.ReadFile("explore_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "return results[0].out, nil") {
		t.Fatal("performExplore still discards the concurrent partner result")
	}
}

func TestExploreOnceSourceSurfacesDiscoveryError(t *testing.T) {
	body, err := os.ReadFile("explore_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if strings.Contains(src, "if discErr == nil") && strings.Contains(src, "step.Discovery = &discovery") && !strings.Contains(src, "return step, live, discErr") {
		t.Fatal("recordExploration errors are still dropped on the explore path")
	}
}

func TestDiscoverResumeAndStatusCommandsExist(t *testing.T) {
	err := run([]string{"discover", "resume"})
	if err != nil && strings.Contains(err.Error(), "discover <record|list|triage|run>") && !strings.Contains(err.Error(), "resume") {
		t.Fatal("discover resume is still missing")
	}
	err = run([]string{"discover", "status"})
	if err != nil && strings.Contains(err.Error(), "discover <record|list|triage|run>") && !strings.Contains(err.Error(), "status") {
		t.Fatal("discover status is still missing")
	}
	err = run([]string{"discover", "abort"})
	if err != nil && strings.Contains(err.Error(), "discover <record|list|triage|run>") && !strings.Contains(err.Error(), "abort") {
		t.Fatal("discover abort is still missing")
	}
}
