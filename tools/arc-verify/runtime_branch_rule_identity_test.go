package main

import (
	"encoding/json"
	"os"
	"testing"
)

func branchRuleIdentityGraph(t *testing.T, actor string) Graph {
	t.Helper()
	g := runtimeBusinessGraph(t)
	other := g.Rules[0]
	other.ID, other.Actor = "R-OTHER", actor
	other.Description = "Synthetic independently sourced rule for the same operation"
	g.Rules = append(g.Rules, other)
	if actor != "STUDENT" {
		g.Nodes = append(g.Nodes, Node{ID: actor, Kind: "role", Name: actor})
		g.Edges = append(g.Edges, Edge{ID: "other-role", From: actor, To: "enrollment", Kind: "serves"})
	}
	g.BusinessContract.Branches[0].Rule = other.ID
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestRuntimeBusinessCorrectRuleExecutesButCannotRequalifyAnotherRule(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	g := branchRuleIdentityGraph(t, "STUDENT")
	var manifest RunnerManifest
	f := setupRuntimeBusiness(t, g, target, server, "accept", func(m *RunnerManifest) {
		m.Test = "R-OTHER/POSITIVE"
		manifest = *m
	})
	var result ExecuteResult
	if err := json.Unmarshal(f.execute(t, manifest.ID, manifest.Test), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "PASS" || target.requests != 1 {
		t.Fatalf("properly bound alternate rule failed to execute: %+v requests=%d", result, target.requests)
	}
	if err := f.d.qualifyModelRun(result.Run); err != nil {
		t.Fatalf("properly bound alternate rule evidence did not qualify: %v", err)
	}
	raw, _, err := f.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	var original *ModelObligation
	for _, obligation := range modelObligations(g) {
		if obligation.Task.ID == g.ID+"/R1/POSITIVE" {
			copy := obligation
			original = &copy
		}
	}
	if original == nil {
		t.Fatal("missing original rule obligation")
	}
	verdict, reason, _ := verifyRuntimeBusiness(g, *original, manifest.Steps, raw.Model)
	if verdict != "BLOCKED" {
		t.Fatalf("real alternate-rule facts fulfilled another rule during offline verification: verdict=%s reason=%s", verdict, reason)
	}
}

func TestRuntimeBusinessSelectedBranchCannotBorrowAnotherRulesExpectation(t *testing.T) {
	for _, role := range []string{"STUDENT", "ADMIN"} {
		t.Run(role, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			g := branchRuleIdentityGraph(t, role)
			f := setupRuntimeBusiness(t, g, target, server, "", nil)
			result := executeRuntimeBusiness(t, f)
			t.Logf("branch_rule=%s runner_rule=R1 role=%s requests=%d verdict=%s", g.BusinessContract.Branches[0].Rule, role, target.requests, result.Verdict)
			if result.Verdict != "BLOCKED" || target.requests != 0 {
				t.Fatalf("another rule's expectation authorized this runner: result=%+v requests=%d", result, target.requests)
			}
			if facts, err := f.d.verifiedRunFacts(result.Run); err != nil || facts.EvidenceStatus != "PASS" {
				t.Fatalf("wrong-rule block lost its verifiable facts: %+v %v", facts, err)
			}
		})
	}
}

func TestRuntimeBusinessNamedBranchMustBelongToTheRegisteredRule(t *testing.T) {
	g := branchRuleIdentityGraph(t, "STUDENT")
	manifest := runtimeBranchRuleManifest(t)
	if _, err := validateManifest(g, manifest); err == nil {
		t.Fatal("registered R1 runner accepted an explicit branch whose frozen rule is R-OTHER")
	}
	manifest.Test = "R-OTHER/POSITIVE"
	if _, err := validateManifest(g, manifest); err != nil {
		t.Fatalf("same action bound to the correct declared rule must remain valid: %v", err)
	}
}

func runtimeBranchRuleManifest(t *testing.T) RunnerManifest {
	t.Helper()
	var m RunnerManifest
	file := rewriteFixtureManifest(t, "branch-rule", "R1/POSITIVE", func(m *RunnerManifest) {
		m.Steps[0].BusinessBranch = "accept"
	})
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictJSON(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
