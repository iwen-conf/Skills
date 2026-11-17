package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestRuntimeConsistencyUsesFinalActionIndexRegardlessOfOracleOrder(t *testing.T) {
	for _, finalHealthy := range []bool{false, true} {
		t.Run(fmt.Sprintf("final_healthy=%t", finalHealthy), func(t *testing.T) {
			runRuntimeConsistencyIndexCase(t, finalHealthy)
		})
	}
}

func runRuntimeConsistencyIndexCase(t *testing.T, finalHealthy bool) {
	t.Helper()
	unhealthyPath, want := "/db-final", "FAIL"
	if finalHealthy {
		unhealthyPath, want = "/db-first", "PASS"
	}
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enroll":
			calls++
			fmt.Fprint(w, `{}`)
		case "/reset":
			calls = 0
			fmt.Fprint(w, `{}`)
		default:
			state := "pending"
			if calls > 0 && r.URL.Path != unhealthyPath {
				state = "ready"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d}`, state, calls)
		}
	}))
	t.Cleanup(server.Close)
	g := kindsModel(t)
	f := prepareExecutionFixture(t, g, server.URL)
	file := kindsManifest(t, "consistency-indexed", "test.consistency.enroll",
		kindsStep("enroll", "submit-flow", "/enroll", kindsOracle("fixture-independent-store", "/oracle"), kindsOracle("db-store", "/db-first")),
		kindsStep("enroll", "submit-flow", "/enroll", kindsOracle("fixture-independent-store", "/oracle"), kindsOracle("db-store", "/db-final")))
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var manifest RunnerManifest
	if err := decodeStrictJSON(body, &manifest); err != nil {
		t.Fatal(err)
	}
	for i := range manifest.Steps {
		manifest.Steps[i].ParallelGroup = "registered-same-actor-batch"
	}
	body, _ = json.Marshal(manifest)
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	f.registerAndRound(t, file)
	result := executeKinds(t, f, manifest.ID, manifest.Test)
	if result.Verdict != want {
		t.Fatalf("control must reflect final DB observation: want=%s result=%+v", want, result)
	}
	raw, _, err := f.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil || len(raw.Model.Oracles) != 4 {
		t.Fatalf("real batch oracle evidence missing: %+v %v", raw.Model, err)
	}
	// Facts retain their exact actor/action identity. Reordering this array must
	// not let action 1's healthy DB replace action 2's contradictory DB result.
	for left, right := 0, len(raw.Model.Oracles)-1; left < right; left, right = left+1, right-1 {
		raw.Model.Oracles[left], raw.Model.Oracles[right] = raw.Model.Oracles[right], raw.Model.Oracles[left]
	}
	hash := replaceSuccessorHistoricalCapture(t, f.d, raw)
	contract, err := f.d.runTaskContract(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	verdict, reason, _, err := f.d.evaluateModelObservation(contract, raw, 1, hash)
	if err != nil || verdict != want {
		t.Fatalf("final action borrowed an earlier action's consistency layer: want=%s verdict=%s reason=%s err=%v", want, verdict, reason, err)
	}
	if facts, err := f.d.verifiedRunFacts(result.Run); err != nil || facts.EvidenceStatus != "PASS" {
		t.Fatalf("array reordering must retain correctly scoped evidence: %+v %v", facts, err)
	}
	// Older captures remain readable, but without action indices two readers
	// for the same actor and ID cannot prove which final action they describe.
	raw.Model.EvidenceVersion = 0
	for i := range raw.Model.Oracles {
		raw.Model.Oracles[i].ActionIndex = 0
	}
	_, body, err = writeRunnerRAW(f.d, result.Run, 1, raw)
	if err != nil {
		t.Fatal(err)
	}
	verdict, reason, _, _ = f.d.evaluateModelObservation(contract, raw, 1, digestBytes(body))
	if verdict != "BLOCKED" || !strings.Contains(reason, "legacy oracle") {
		t.Fatalf("ambiguous legacy reader identity must block: verdict=%s reason=%s", verdict, reason)
	}
}
