package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBusinessExactNumbersDoNotDependOnFloat64Range(t *testing.T) {
	for _, tc := range []struct {
		kind, literal string
		valid         bool
	}{
		{"number", "1e400", true},
		{"number", "-1e400", true},
		{"number", "1e1000", true},
		{"number", "1e-1000", true},
		{"integer", "1e400", true},
		{"number_set", "[1e400,2e400]", true},
		{"number", "1e1001", false},
		{"number", "1e-1001", false},
		{"integer", "1e-400", false},
		{"number_set", "[1e400,10e399]", false},
		{"number", "1e400 0", false},
		{"number", "{\"n\":1,\"n\":2}", false},
	} {
		t.Run(tc.kind+"/"+tc.literal, func(t *testing.T) {
			_, err := parseBusinessValue(tc.kind, json.RawMessage(tc.literal))
			if (err == nil) != tc.valid {
				t.Fatalf("exact numeric contract was changed by transport parsing: valid=%t err=%v", tc.valid, err)
			}
		})
	}
}

func TestBusinessExactNumberRuntimePreservesRequestResponseOracleAndLedger(t *testing.T) {
	target, original := newRuntimeBusinessTarget(t, true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded := httptest.NewRecorder()
		original.Config.Handler.ServeHTTP(recorded, r)
		var object map[string]json.RawMessage
		if err := json.Unmarshal(recorded.Body.Bytes(), &object); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		object["amount"] = json.RawMessage("1e400")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorded.Code)
		_ = json.NewEncoder(w).Encode(object)
	}))
	t.Cleanup(server.Close)
	g := runtimeBusinessGraph(t)
	g.BusinessContract.Subjects = append(g.BusinessContract.Subjects, BusinessSubject{ID: "request", Kind: "environment", Attributes: []BusinessAttribute{{Name: "amount", Type: "number"}}})
	for i := range g.BusinessContract.Subjects {
		subject := &g.BusinessContract.Subjects[i]
		if subject.ID == "order" || subject.ID == "reply" {
			subject.Attributes = append(subject.Attributes, BusinessAttribute{Name: "amount", Type: "number"})
		}
	}
	branch := &g.BusinessContract.Branches[0]
	branch.When = BusinessPredicate{Op: "and", Args: []BusinessPredicate{branch.When, businessPredicate("gt", "request", "amount", "number", "1e399"), businessPredicate("gt", "order", "amount", "number", "1e399")}}
	branch.Expected.Outcomes[0].Checks = append(branch.Expected.Outcomes[0].Checks,
		BusinessExpectedCheck{ID: "exact-response", Kind: "response", Oracle: "response-capture", SourceRef: "synthetic-contract:exact-number", Predicate: businessPredicate("eq", "reply", "amount", "number", "1e400")},
		BusinessExpectedCheck{ID: "exact-store", Kind: "persistence", Oracle: "fixture-independent-store", SourceRef: "synthetic-contract:exact-number", Predicate: businessPredicate("eq", "order", "amount", "number", "1e400")})
	f := setupRuntimeBusiness(t, g, target, server, "accept", func(m *RunnerManifest) {
		step := &m.Steps[0]
		step.Call.Body = json.RawMessage(`{"amount":1e400}`)
		step.InputMeasurements = map[string]string{"request.amount": "/amount"}
		step.ResponseMeasurements["reply.amount"] = "/amount"
		step.Oracles[0].Measurements["order.amount"] = "/amount"
	})
	result := executeRuntimeBusiness(t, f)
	if result.Verdict != "PASS" || target.requests != 1 {
		t.Fatalf("exact-number runtime failed: %+v requests=%d", result, target.requests)
	}
	verified, err := f.d.verifiedRunFacts(result.Run)
	if err != nil || verified.EvidenceStatus != "PASS" {
		t.Fatalf("exact-number evidence failed independent revalidation: %+v %v", verified, err)
	}
	raw, _, err := f.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"request.amount", "reply.amount", "order.amount"} {
		if got := string(raw.Model.BusinessExpectations[0].After.Values[key]); got != "1e400" {
			t.Fatalf("%s numeric token changed in persisted evidence: %s", key, got)
		}
	}
}

func TestBusinessExactNumberModelImportFreezeAndFiniteProof(t *testing.T) {
	g := contractModel(t)
	guard := businessPredicate("gt", "order", "amount", "number", "1e399")
	g.BusinessContract.Branches[0].When = guard
	g.BusinessContract.Branches[1].When = businessNot(guard)
	g.BusinessContract.Coverage.Domains = []BusinessFiniteDomain{{Subject: "order", Attribute: "amount", Values: []json.RawMessage{json.RawMessage("1e398"), json.RawMessage("1e400")}}}
	body, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var imported Graph
	if err := decodeStrictJSON(body, &imported); err != nil {
		t.Fatalf("valid exact-number model failed normal import parsing: %v", err)
	}
	if err := validateGraph(imported); err != nil {
		t.Fatal(err)
	}
	if proof := ProveBusinessCoverage(imported); proof.Status != "PROVEN_IN_SCOPE" || proof.CheckedCases != 2 {
		t.Fatalf("finite exact-number domain lost its partition: %+v", proof)
	}
	before := contractObservation(true)
	before.Values["order.amount"] = json.RawMessage("1e400")
	decision := FreezeBusinessExpectation(imported, "enroll", before)
	if decision.Status != "READY" || decision.Branch != "accept" {
		t.Fatalf("independently observed exact value could not select its branch: %+v", decision)
	}
	after := contractObservation(true)
	after.State = "ready"
	if result := CompareBusinessExpectation(imported, *decision.Frozen, after, false); result.Status != "PASS" {
		t.Fatal(result)
	}
}

func TestBusinessExactNumberValidationStillRejectsDuplicateKeys(t *testing.T) {
	var raw json.RawMessage
	err := decodeUniqueJSON([]byte(`{"amount":1e400,"amount":2e400}`), &raw)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("numeric scanning bypassed duplicate-key detection: %v", err)
	}
}
