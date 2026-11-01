package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEvidenceFirstDoesNotAskKnownFacts(t *testing.T) {
	g := referenceModel(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	ctx := buildEvidenceContext(g, map[string]bool{"R1": true}, ExploreAdapter{}, ExploreReport{}, adapter, nil)
	if len(ctx.Facts) == 0 {
		t.Fatal("evidence-first must extract known rules, flows and entries")
	}
	for _, fact := range ctx.Facts {
		if fact.Class == "" || fact.ID == "" || fact.Statement == "" || len(fact.Refs) == 0 {
			t.Fatalf("each fact needs class, id, statement and source: %+v", fact)
		}
		if fact.Class == factInferred && strings.Contains(strings.ToLower(fact.Statement), "approved rule") {
			t.Fatal("INFERRED must not be labeled as a confirmed rule")
		}
	}
	for _, question := range ctx.Questions {
		if strings.Contains(question, "R1") || strings.Contains(question, "/enroll") {
			t.Fatalf("must not re-ask a fact the ledger already has: %s", question)
		}
	}
}

func TestEvidenceFirstDoesNotInventRULE(t *testing.T) {
	g := referenceModel(t)
	ctx := buildEvidenceContext(g, map[string]bool{}, ExploreAdapter{}, ExploreReport{
		Unspecified: []UnspecifiedItem{{ID: "U-1", Kind: "UNDECLARED_TRANSITION", Action: "claim", Reason: "second agent also received HTTP 200"}},
	}, seatExploreAdapter(t, seatIsolationScope), nil)
	for _, fact := range ctx.Facts {
		if fact.Class == factKnown && strings.Contains(fact.Statement, "only one owner") {
			t.Fatal("an unconfirmed exclusivity claim is not KNOWN")
		}
	}
	found := false
	for _, hyp := range proposeCounterfactuals(ctx, g, seatExploreAdapter(t, seatIsolationScope)) {
		if hyp.Status == hypNeedsDecision || hyp.Status == hypProposed {
			found = true
		}
		if hyp.Status == "RULE_APPROVED" {
			t.Fatal("discovery must not mint a RULE")
		}
	}
	if !found {
		t.Fatal("true ambiguity must stay a hypothesis or business decision, not a rule")
	}
}

func TestCounterfactualsAreBusinessBound(t *testing.T) {
	g := referenceModel(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	ctx := buildEvidenceContext(g, map[string]bool{"R1": true}, ExploreAdapter{}, ExploreReport{}, adapter, nil)
	hyps := proposeCounterfactuals(ctx, g, adapter)
	if len(hyps) < 3 {
		t.Fatalf("one capability should yield several distinct hypotheses, got %d", len(hyps))
	}
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, hyp := range hyps {
		if hyp.ID == "" || hyp.Capability == "" || hyp.Assumption == "" || hyp.Counterfactual == "" {
			t.Fatalf("hypothesis is incomplete: %+v", hyp)
		}
		if len(hyp.KnownFacts) == 0 || len(hyp.EvidenceRefs) == 0 {
			t.Fatalf("hypothesis must cite facts: %+v", hyp)
		}
		key := hyp.Assumption + "|" + hyp.Counterfactual
		if seen[key] {
			t.Fatalf("duplicate hypothesis: %s", key)
		}
		seen[key] = true
		kinds[hyp.Counterfactual] = true
		if strings.Contains(hyp.Counterfactual, "rename variable") {
			t.Fatal("a renamed parameter is not a new boundary")
		}
	}
	if len(kinds) < 3 {
		t.Fatalf("counterfactuals must differ in role, order or concurrency, got %v", kinds)
	}
}

func TestCompetingExplanationsNeedOracleNotHTTP(t *testing.T) {
	obs := AmbiguousObservation{HTTPStatus: 200, Owners: []string{"agent-a", "agent-b"}, Resources: []string{"work-1", "work-1"}, SameResource: true}
	explanations := competeExplanations(obs, []EvidenceFact{{ID: "FACT-http", Class: factKnown, Statement: "both claim calls returned 200", Refs: []EvidenceReference{{ID: "EV-1", Source: "http", Trust: "VERIFIED"}}}})
	if len(explanations) < 2 {
		t.Fatalf("same HTTP needs at least two different business readings, got %d", len(explanations))
	}
	for _, item := range explanations {
		if item.Oracle == "" || item.Experiment == "" {
			t.Fatalf("each explanation must name an oracle and a distinguishing experiment: %+v", item)
		}
		if strings.EqualFold(item.Oracle, "http") || strings.Contains(strings.ToLower(item.Oracle), "status") && !strings.Contains(strings.ToLower(item.Oracle), "owner") {
			t.Fatalf("HTTP status cannot be the independent oracle: %+v", item)
		}
	}
	exp := planMinimalExperiment(explanations, seatExploreAdapter(t, seatIsolationScope))
	if !exp.Discriminating || exp.Cost < 1 || len(exp.Actions) == 0 {
		t.Fatalf("minimal experiment must discriminate: %+v", exp)
	}
	if containsHTTPOnlyJudge(exp) {
		t.Fatal("the distinguishing step cannot be HTTP 200 alone")
	}
}

func TestUnderstandingRevisionKeepsHistory(t *testing.T) {
	approvals := map[string]bool{"R1": true}
	first := UnderstandingRevision{Version: 1, Believed: "online presence is required to claim", Observed: "offline agent claimed", Refuted: []string{"BH-online"}, StillValidRules: []string{"R1"}}
	second := reviseUnderstanding(DiscoveryContext{Revisions: []UnderstandingRevision{first}, Hypotheses: []BoundaryHypothesis{{ID: "BH-online", Assumption: "online presence is required to claim", Status: hypProposed}}}, ExploreStep{
		Before: IndependentFacts{State: "ready"}, After: IndependentFacts{State: "assigned"},
	}, approvals)
	if second.Version != 2 {
		t.Fatalf("revision must append, got %+v", second)
	}
	if len(second.Refuted) == 0 || second.Believed == "" || second.Observed == "" {
		t.Fatalf("new observation must record what changed: %+v", second)
	}
	if approvals["R1"] != true {
		t.Fatal("revision must not rewrite a confirmed RULE")
	}
	if first.Believed == "" || first.Version != 1 {
		t.Fatal("history of the first understanding was overwritten")
	}
}

func TestHypothesisNeverBecomesApprovedRule(t *testing.T) {
	approvals := map[string]bool{"R1": true}
	if err := applyHypothesisVerdict(approvals, BoundaryHypothesis{ID: "BH-1", Status: hypSupported}); err != nil {
		t.Fatal(err)
	}
	if _, ok := approvals["BH-1"]; ok {
		t.Fatal("SUPPORTED must not insert a RULE")
	}
	if err := applyHypothesisVerdict(approvals, BoundaryHypothesis{ID: "BH-1", Status: hypRefuted}); err != nil {
		t.Fatal(err)
	}
	if !approvals["R1"] {
		t.Fatal("REFUTED must not clear an approved RULE")
	}
}

func TestMinimalExperimentBeatsSpray(t *testing.T) {
	explanations := competeExplanations(AmbiguousObservation{HTTPStatus: 200, Owners: []string{"a", "b"}, Resources: []string{"w", "w"}, SameResource: true}, nil)
	min := planMinimalExperiment(explanations, seatExploreAdapter(t, seatIsolationScope))
	spray := sprayExperiment(explanations)
	if min.Cost == 0 || min.Cost >= spray.Cost {
		t.Fatalf("minimal experiment should cost less than untargeted spray: min=%+v spray=%+v", min, spray)
	}
	if !min.Discriminating {
		t.Fatal("cheap experiment that cannot distinguish is not a discovery")
	}
}

func TestPreferDiscoveryPicksOtherRoleAfterFirstClaim(t *testing.T) {
	disc := DiscoveryContext{Hypotheses: []BoundaryHypothesis{{
		ID: "BH-owner", Assumption: "one resource has a single owner", Counterfactual: "if two subjects act at the same time", Status: hypProposed,
	}}}
	world := exploreWorld{state: "assigned", role: "AGENT", lastAction: "claim"}
	out := preferDiscoveryCandidates([]CandidateAction{
		{Action: "claim", Role: "AGENT", Kind: kindRepeat},
		{Action: "claim", Role: "CONTENDER", Kind: kindHistory},
	}, disc, world)
	if len(out) != 2 || out[0].Role != "CONTENDER" {
		t.Fatalf("after one claim the next probe should be the other subject: %+v", out)
	}
}

func TestPlannerContextCarriesEvidenceAndHypotheses(t *testing.T) {
	g := referenceModel(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	disc := buildEvidenceContext(g, map[string]bool{"R1": true}, ExploreAdapter{}, ExploreReport{}, adapter, nil)
	disc.Hypotheses = proposeCounterfactuals(disc, g, adapter)
	in := plannerContext(ExploreRequest{Graph: "course-v2", Adapter: adapter}, BehaviorFlow{Capability: "enrollment.submit"}, map[string]bool{"R1": true}, exploreWorld{state: "pending", role: "STUDENT"}, ExploreReport{}, nil, 4)
	in = attachDiscoveryToPlanner(in, disc)
	if len(in.Facts) == 0 || len(in.Hypotheses) == 0 {
		t.Fatalf("planner must see facts and open hypotheses: %+v", in)
	}
	if len(in.Authorization) == 0 {
		t.Fatal("planner context must include the current authorization scope")
	}
}

func TestBoundaryBlindContrastRecordsHonestABC(t *testing.T) {
	hidden := newHiddenOwnerServer(t)
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{
		{Capability: "work.claim", Action: "claim", Entry: "/claim", Role: "AGENT", Reason: "establish an owner with the confirmed claim entry", Observe: "owner", Risk: "low"},
		{Capability: "work.claim", Action: "claim", Entry: "/claim", Role: "CONTENDER", Reason: "second subject after HTTP 200; read owner, do not treat status as identity", Observe: "owner", Risk: "medium", BoundaryHypothesis: "last writer vs dual owner"},
	}}
	run := func(mode string, used ExplorePlanner) ExploreReport {
		t.Helper()
		hidden.reset()
		fixture := prepareHiddenOwnerFixture(t, hidden.URL)
		fixture.openHiddenRound(t)
		report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
			Graph: "hidden-owner-v1", Flow: "flow.claim", Environment: "independent", Round: fixture.round,
			Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
			Account: fixture.account, Fixture: fixture.fixture, Adapter: hiddenOwnerAdapter(),
			Mode: mode, Planner: used, Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 3, MaxPlannerCalls: 3, Deadline: 12 * time.Second},
		})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	det := run("deterministic", nil)
	heur := run("heuristic", nil)
	ai := run("ai", planner)
	if det.Mode != "deterministic" || heur.Mode != "heuristic" || ai.Mode != "ai" {
		t.Fatalf("mode labels %s %s %s", det.Mode, heur.Mode, ai.Mode)
	}
	if det.Actions == 0 || heur.Actions == 0 || ai.Actions == 0 {
		t.Fatalf("A/B/C must execute live claims: A=%d B=%d C=%d", det.Actions, heur.Actions, ai.Actions)
	}
	a := scoreBoundaryContrast("deterministic", det)
	b := scoreBoundaryContrast("heuristic", heur)
	c := scoreBoundaryContrast("ai", ai)
	if c.PlannerCalls < 1 {
		t.Fatal("C group must record a planner call")
	}
	if c.HTTPOnlyJudgements != 0 && c.DistinctActors >= 2 {
		t.Fatal("C must not attribute ownership from HTTP 200 after two actors claimed")
	}
	if c.FalsePositives != 0 {
		t.Fatalf("C treated dual HTTP 200 as two owners: %+v", c)
	}
	if ai.Boundary != nil {
		for _, hyp := range ai.Boundary.Hypotheses {
			if hyp.Status == "RULE_APPROVED" {
				t.Fatal("C minted a RULE from a hypothesis")
			}
		}
	}
	uniqueC := 0
	if c.UniqueDiscoveries > 0 && b.UniqueDiscoveries == 0 && a.UniqueDiscoveries == 0 {
		uniqueC = c.UniqueDiscoveries
	}
	verdict := "PARTIAL"
	note := "scripted planner; C unique last-writer discovery is recorded only when A/B did not observe two actors and one owner"
	if uniqueC == 0 && c.UniqueDiscoveries > 0 {
		note = "C and another mode both observed last-writer-wins; C is not claimed as a unique finder"
	}
	if uniqueC == 0 && c.UniqueDiscoveries == 0 {
		note = "C did not obtain a unique last-writer observation under the shared action budget; scripted comparison cannot establish host Agent reasoning"
	}
	writeBoundaryArtifact(t, "reports/boundary-abc.json", map[string]any{
		"verdict":     verdict,
		"note":        note,
		"rows":        []BoundaryContrastRow{a, b, c},
		"unique_to_c": uniqueC,
		"planner":     "scripted",
	})
}

func containsHTTPOnlyJudge(exp ExperimentCandidate) bool {
	blob := strings.ToLower(exp.Oracle + exp.ID + strings.Join(exp.Actions, " "))
	return strings.Contains(blob, "http 200") && !strings.Contains(blob, "owner")
}

type hiddenOwnerServer struct {
	URL    string
	mu     sync.Mutex
	owner  string
	claims int
}

func newHiddenOwnerServer(t *testing.T) *hiddenOwnerServer {
	t.Helper()
	target := &hiddenOwnerServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target.mu.Lock()
		defer target.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/claim":
			target.claims++
			actor := r.Header.Get("X-Fixture-Actor")
			if actor == "" {
				actor = "agent-a"
			}
			target.owner = actor
			w.WriteHeader(http.StatusOK)
			fmtJSON(w, map[string]any{"accepted": true, "http": 200})
		case "/oracle":
			fmtJSON(w, target.facts(r.Header.Get("X-Fixture-Actor")))
		case "/reset":
			target.owner, target.claims = "", 0
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	target.URL = server.URL
	return target
}

func (s *hiddenOwnerServer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owner, s.claims = "", 0
}

func (s *hiddenOwnerServer) facts(actor string) map[string]any {
	principal, role := "agent-a", "AGENT"
	if actor == "agent-b" {
		principal, role = "agent-b", "CONTENDER"
	}
	state, owners := "pending", 0
	if s.owner != "" {
		state, owners = "assigned", 1
	}
	return map[string]any{
		"state": state, "principal": principal, "role": role, "scope": "hidden-owner",
		"login": true, "owner": s.owner, "version": s.claims, "owners": owners, "one": 1,
	}
}

func fmtJSON(w http.ResponseWriter, value any) {
	body, _ := json.Marshal(value)
	w.Write(body)
}

func hiddenOwnerOracle() OracleAdapter {
	return OracleAdapter{
		ID: "owner-store", Boundary: "after claim and fixture restoration",
		Call:      AdapterCall{Kind: adapterHTTP, Method: http.MethodGet, Path: "/oracle", SuccessStatuses: []int{200}, CredentialHeader: "X-Fixture-Actor"},
		StatePath: "/state", PrincipalPath: "/principal", RolePath: "/role", ScopePath: "/scope",
		Conditions:   map[string]string{"login": "/login"},
		Measurements: map[string]string{"owner": "/owner", "version": "/version", "owners": "/owners", "one": "/one"},
	}
}

func hiddenOwnerAdapter() ExploreAdapter {
	oracle := hiddenOwnerOracle()
	recovery := &FixtureRecovery{
		IsolationScope: "hidden-owner",
		Steps:          []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/reset", SuccessStatuses: []int{200}}}},
		Oracle:         oracle,
		State:          "pending",
		Conditions:     ConditionAssignment{"login": true},
	}
	return ExploreAdapter{
		SchemaVersion: 1, Graph: "hidden-owner-v1", IsolationScope: "hidden-owner",
		TimeoutMS: 8000, StepTimeoutMS: 2000, Parallelism: 2, Oracle: oracle, Recovery: recovery,
		Accounts: map[string]string{"AGENT": "fixture.agent-a", "CONTENDER": "fixture.agent-b"},
		Entries: []ExploreEntry{
			{Action: "claim", Entry: "/claim", Method: "POST", Roles: []string{"AGENT", "CONTENDER"}},
		},
	}
}

func hiddenOwnerModel(t *testing.T) Graph {
	t.Helper()
	graph, err := loadGraph("examples/hidden-owner-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateGraph(graph); err != nil {
		t.Fatal(err)
	}
	return graph
}

func prepareHiddenOwnerFixture(t *testing.T, target string) executionFixture {
	t.Helper()
	d := modelTestDB(t, hiddenOwnerModel(t))
	root := filepath.Join(t.TempDir(), "candidate")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "business.txt"), []byte("hidden last-writer owner fixture; not IM-mongo evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "business.txt"}, {"-c", "user.name=verify-fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "hidden owner"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
		if body, err := command.CombinedOutput(); err != nil {
			t.Fatalf("hidden fixture git: %v %s", err, body)
		}
	}
	commit, _, err := sourceFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(config, []byte(fmt.Sprintf(`{"environments":{"independent":{"approved":true,"url":%q}}}`, target)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARC_HIDDEN_A", "agent-a")
	t.Setenv("ARC_HIDDEN_B", "agent-b")
	if _, err := d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: "fixture.agent-a", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_HIDDEN_A", Refs: map[string]string{"role": "AGENT", "principal": "agent-a", "environment": "independent"}},
		{ID: "fixture.agent-b", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_HIDDEN_B", Refs: map[string]string{"role": "CONTENDER", "principal": "agent-b", "environment": "independent"}},
		{ID: "fixture.work", Kind: "business", State: "pending", Refs: map[string]string{"environment": "independent", "scope": "hidden-owner"}},
	}}); err != nil {
		t.Fatal(err)
	}
	return executionFixture{d: d, root: root, config: config, commit: commit, account: "fixture.agent-a", fixture: "fixture.work"}
}

func (fixture *executionFixture) openHiddenRound(t *testing.T) {
	t.Helper()
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cliJSON(t, "round", "--db", fixture.d.file, "--graph", "hidden-owner-v1", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &result); err != nil || result.ID == "" {
		t.Fatalf("hidden round: %v %+v", err, result)
	}
	fixture.round = result.ID
}

func writeBoundaryArtifact(t *testing.T, rel string, value any) {
	t.Helper()
	path := filepath.Join("..", "..", ".arc", "artifacts", "verify-boundary-discovery", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestBoundaryDiscoveryPersistsWithoutApprovingRules(t *testing.T) {
	d, err := openDB(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	g := hiddenOwnerModel(t)
	disc := buildEvidenceContext(g, map[string]bool{"R-OWNER": true}, ExploreAdapter{}, ExploreReport{}, hiddenOwnerAdapter(), nil)
	disc.Hypotheses = proposeCounterfactuals(disc, g, hiddenOwnerAdapter())
	if err := d.persistDiscoveryContext("X-hidden", disc); err != nil {
		t.Fatal(err)
	}
	loaded, err := d.loadDiscoveryContext("X-hidden")
	if err != nil || len(loaded.Hypotheses) == 0 {
		t.Fatalf("persisted hypotheses missing: %+v %v", loaded, err)
	}
	for _, hyp := range loaded.Hypotheses {
		if hyp.Status == "RULE_APPROVED" {
			t.Fatal("persisted hypothesis became a rule")
		}
	}
}

func TestHiddenOwnerHTTP200IsNotBusinessIdentity(t *testing.T) {
	hidden := newHiddenOwnerServer(t)
	client := &http.Client{Timeout: 2 * time.Second}
	claim := func(actor string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, hidden.URL+"/claim", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Fixture-Actor", actor)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if claim("agent-a") != 200 || claim("agent-b") != 200 {
		t.Fatal("both claims must return 200")
	}
	resp, err := client.Get(hidden.URL + "/oracle")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var facts map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&facts); err != nil {
		t.Fatal(err)
	}
	if facts["owner"] != "agent-b" {
		t.Fatalf("oracle must show last writer, got %v", facts["owner"])
	}
}
