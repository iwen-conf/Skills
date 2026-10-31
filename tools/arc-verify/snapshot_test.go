package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func snapshotAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	index := `<!doctype html><html><head><meta charset="UTF-8"><title>arc</title><script defer src="./assets/app.js"></script></head><body><div id="root"></div></body></html>`
	for name, body := range map[string]string{"index.html": index, "assets/app.js": "console.log('static');"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readSnapshot(t *testing.T, out string) Snapshot {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(out, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot Snapshot
	if err := decodeStrictJSON(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestSnapshotExportsSixViewsAndContext(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	run := latestRun(t, fixture)
	out := filepath.Join(t.TempDir(), "snapshot")
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", fixture.round, "--run", run, "--config", fixture.config, "--assets", snapshotAssets(t), "--out", out)
	snapshot := readSnapshot(t, out)
	if snapshot.Meta.Graph != "course-v2" || snapshot.Meta.Environment != "independent" || snapshot.Meta.Round != fixture.round || len(snapshot.Meta.GraphSHA256) != 64 || snapshot.Meta.Generated == "" || len(snapshot.Meta.Argv) == 0 {
		t.Fatalf("snapshot context is incomplete: %+v", snapshot.Meta)
	}
	if len(snapshot.Views) != 6 {
		t.Fatalf("snapshot needs all six views: %d", len(snapshot.Views))
	}
	for view, projection := range snapshot.Views {
		if projection.Graph != "course-v2" || projection.Round != fixture.round || projection.Coverage.Obligations.Total == 0 {
			t.Fatalf("view %s is disconnected from the selected ledger context: %+v", view, projection.Coverage)
		}
	}
	if snapshot.Model.Data.ID != "course-v2" || !snapshot.Model.Approved || len(snapshot.Model.SHA256) != 64 {
		t.Fatalf("snapshot model is incomplete: %+v", snapshot.Model.SHA256)
	}
	if len(snapshot.Catalog.Environments) != 1 || snapshot.Catalog.Environments[0].Name != "independent" {
		t.Fatalf("catalog lost the approved configuration facts: %+v", snapshot.Catalog.Environments)
	}
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].Trace.Run != run || len(snapshot.Runs[0].Evidence) == 0 {
		t.Fatalf("selected run was not exported: %+v", snapshot.Runs)
	}
	for _, entry := range snapshot.Runs[0].Evidence {
		if entry.Error != "" || entry.Data == nil || len(entry.SHA256) != 64 {
			t.Fatalf("intact evidence was not re-verified into the snapshot: %+v", entry)
		}
	}
}

func latestRun(t *testing.T, fixture executionFixture) string {
	t.Helper()
	rows, err := fixture.d.query("SELECT id FROM runs ORDER BY rowid DESC LIMIT 1", 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("no run recorded: %v", err)
	}
	return rows[0][0]
}

func TestSnapshotUnknownContextErrors(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	assets := snapshotAssets(t)
	for _, scenario := range []struct {
		name string
		args []string
	}{
		{"unknown graph", []string{"--graph", "missing-v9"}},
		{"unknown environment", []string{"--graph", "course-v2", "--env", "nowhere"}},
		{"unknown round", []string{"--graph", "course-v2", "--round", "Q-missing"}},
		{"round of another environment", []string{"--graph", "course-v2", "--env", "other", "--round", fixture.round}},
		{"unknown run", []string{"--graph", "course-v2", "--run", "R-missing"}},
		{"run of another graph", []string{"--graph", "course-v2-other", "--run", latestRun(t, fixture)}},
		{"legacy graph", []string{"--graph", "legacy-v1"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.name == "run of another graph" {
				other := referenceModel(t)
				other.ID = "course-v2-other"
				if err := fixture.d.importGraph(other); err != nil {
					t.Fatal(err)
				}
			}
			if scenario.name == "legacy graph" {
				legacy, err := loadGraph("examples/academic-v1.json")
				if err != nil {
					t.Fatal(err)
				}
				legacy.ID = "legacy-v1"
				if err := fixture.d.importGraph(legacy); err != nil {
					t.Fatal(err)
				}
			}
			out := filepath.Join(t.TempDir(), "snapshot")
			args := append([]string{"snapshot", "--db", fixture.d.file, "--assets", assets, "--out", out}, scenario.args...)
			if err := run(args); err == nil {
				t.Fatal("snapshot guessed a context that is not in the ledger")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("a refused snapshot left output behind: %v", err)
			}
		})
	}
}

func TestSnapshotRefusesNonEmptyOut(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	out := t.TempDir()
	previous := filepath.Join(out, "snapshot.json")
	if err := os.WriteFile(previous, []byte(`{"previous":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--assets", snapshotAssets(t), "--out", out})
	if err == nil || !strings.Contains(err.Error(), "never overwritten") {
		t.Fatalf("an existing snapshot was overwritten: %v", err)
	}
	if body, _ := os.ReadFile(previous); string(body) != `{"previous":true}` {
		t.Fatal("previous snapshot content changed")
	}
}

func TestSnapshotTamperedEvidenceWrittenAsFailure(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	run := latestRun(t, fixture)
	record, err := fixture.d.modelRun(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.Recovery.Evidence, []byte(`{"tampered":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "snapshot")
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--round", fixture.round, "--env", "independent", "--run", run, "--assets", snapshotAssets(t), "--out", out)
	snapshot := readSnapshot(t, out)
	failed, verified := 0, 0
	for _, entry := range snapshot.Runs[0].Evidence {
		switch {
		case entry.Phase == "fixture" && entry.Error != "" && entry.Data == nil:
			failed++
		case entry.Phase == "action" && entry.Error == "" && entry.Data != nil:
			verified++
		}
	}
	if failed != 1 || verified != 1 {
		t.Fatalf("tampered evidence must be exported as a failure entry next to intact evidence: %+v", snapshot.Runs[0].Evidence)
	}
}

func TestSnapshotSizeLimitFails(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	out := filepath.Join(t.TempDir(), "snapshot")
	_, err := fixture.d.writeSnapshot(SnapshotRequest{Graph: "course-v2", Assets: snapshotAssets(t), Out: out, Argv: []string{"snapshot"}, MaxBytes: 2048})
	if err == nil || !strings.Contains(err.Error(), "views") {
		t.Fatalf("oversized snapshot must fail and name its large parts: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("an oversized snapshot left output behind: %v", err)
	}
}

func TestSnapshotHasNoAPIReferences(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	out := filepath.Join(t.TempDir(), "snapshot")
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--assets", snapshotAssets(t), "--out", out)
	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(index)
	if !strings.Contains(page, `http-equiv="Content-Security-Policy"`) || !strings.Contains(page, "default-src 'none'") {
		t.Fatal("snapshot page does not forbid network access")
	}
	match := regexp.MustCompile(`(?s)<script type="application/json" id="arc-snapshot">(.*?)</script>`).FindStringSubmatch(page)
	if match == nil {
		t.Fatal("snapshot JSON is not embedded")
	}
	var embedded, written any
	file, err := os.ReadFile(filepath.Join(out, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal([]byte(match[1]), &embedded) != nil || json.Unmarshal(file, &written) != nil || !reflect.DeepEqual(embedded, written) {
		t.Fatal("embedded snapshot differs from snapshot.json")
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "app.js")); err != nil {
		t.Fatalf("built assets were not copied: %v", err)
	}
	err = filepath.Walk(out, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(body), "/api/") {
			t.Errorf("%s references the removed local API", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotRefusesBuildCallingRemovedAPI(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	assets := snapshotAssets(t)
	if err := os.WriteFile(filepath.Join(assets, "assets", "app.js"), []byte(`fetch("/api/catalog")`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "snapshot")
	err := run([]string{"snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--assets", assets, "--out", out})
	if err == nil || !strings.Contains(err.Error(), "removed local API") {
		t.Fatalf("a build that still calls the local API was packaged: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a refused snapshot left output behind: %v", err)
	}
}

func TestWorkbenchCommandRemoved(t *testing.T) {
	if err := run([]string{"workbench"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("the local workbench service must not exist: %v", err)
	}
}

// tsInterfaces reads `export interface Name [extends Base] { field?: type; ... }` declarations.
func tsInterfaces(t *testing.T) map[string][]string {
	t.Helper()
	body, err := os.ReadFile("workbench/src/types.ts")
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string][]string{}
	bases := map[string][]string{}
	for _, match := range regexp.MustCompile(`(?s)export interface (\w+)(?: extends ([\w, ]+))? \{(.*?)\}\n`).FindAllStringSubmatch(string(body), -1) {
		fields := []string{}
		for _, field := range regexp.MustCompile(`(\w+)\??:`).FindAllStringSubmatch(match[3], -1) {
			fields = append(fields, field[1])
		}
		declared[match[1]] = fields
		if match[2] != "" {
			for _, base := range strings.Split(match[2], ",") {
				bases[match[1]] = append(bases[match[1]], strings.TrimSpace(base))
			}
		}
	}
	var resolve func(string) []string
	resolve = func(name string) []string {
		fields := append([]string{}, declared[name]...)
		for _, base := range bases[name] {
			fields = append(fields, resolve(base)...)
		}
		return fields
	}
	result := map[string][]string{}
	for name := range declared {
		result[name] = uniqueIDs(resolve(name))
	}
	return result
}

func goJSONFields(t reflect.Type) []string {
	fields := []string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.Anonymous && tag == "" {
			fields = append(fields, goJSONFields(field.Type)...)
			continue
		}
		if tag == "-" || !field.IsExported() {
			continue
		}
		if tag == "" {
			tag = field.Name
		}
		fields = append(fields, tag)
	}
	return fields
}

func TestSnapshotExplorationCoverageFromLedger(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	report := ExploreReport{
		Campaign: "X-coverage", Graph: "course-v2", Flow: "flow.enrollment",
		Environment: "independent", Round: fixture.round,
		StopReason: "budget_actions", Truncated: true,
		Steps: []ExploreStep{{
			Candidate: CandidateAction{ID: "c-repeat", Kind: kindRepeat, Action: "enroll", Scope: "enrollment.submit"},
		}},
		Remaining: []CandidateAction{{ID: "c-skip", Kind: kindSkipPre, Action: "enroll", Scope: "enrollment.submit"}},
	}
	if err := fixture.d.persistExploration(report); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "snapshot")
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", fixture.round, "--config", fixture.config, "--assets", snapshotAssets(t), "--out", out)
	snapshot := readSnapshot(t, out)
	cover, ok := snapshot.Exploration["enrollment.submit"]
	if !ok {
		t.Fatal("capability node lost its exploration coverage")
	}
	if len(cover.Explored) == 0 || len(cover.Unexplored) == 0 {
		t.Fatalf("explored/unexplored branches were not exported: %+v", cover)
	}
	if !cover.Tested {
		t.Fatal("ledger execution must mark the capability tested; explore count must not be the signal")
	}
	if len(cover.CurrentPass) != 0 {
		t.Fatalf("exploring a capability that is not fully verified must not invent a current PASS: %+v", cover)
	}
	if !cover.BudgetExhausted || len(cover.StopReasons) == 0 {
		t.Fatalf("budget stop was dropped: %+v", cover)
	}
	if site := os.Getenv("ARC_VERIFY_EXPLORE_SITE"); site != "" {
		assets := os.Getenv("ARC_VERIFY_SAMPLE_ASSETS")
		if assets == "" {
			t.Fatal("ARC_VERIFY_EXPLORE_SITE needs ARC_VERIFY_SAMPLE_ASSETS")
		}
		cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", fixture.round, "--config", fixture.config, "--assets", assets, "--out", site)
	}
	frontier, err := fixture.d.query("SELECT COUNT(*) FROM exploration_frontier WHERE campaign='X-coverage'", 1)
	if err != nil || frontier[0][0] == "0" {
		t.Fatalf("remaining candidates were not persisted as frontier: %v %v", frontier, err)
	}
	campaign, err := fixture.d.query("SELECT environment,round FROM exploration_campaigns WHERE id='X-coverage'", 2)
	if err != nil || campaign[0][0] != "independent" || campaign[0][1] != fixture.round {
		t.Fatalf("campaign lost environment/round: %v %v", campaign, err)
	}
}

func TestSnapshotTypesMatchFrontend(t *testing.T) {
	interfaces := tsInterfaces(t)
	pairs := map[string]any{
		"Snapshot": Snapshot{}, "SnapshotMeta": SnapshotMeta{}, "SnapshotModel": SnapshotModel{}, "SnapshotRun": SnapshotRun{}, "SnapshotEvidence": SnapshotEvidence{}, "ExplorationCoverage": ExplorationCoverage{},
		"Model": Graph{}, "BusinessNode": Node{}, "Relation": Edge{}, "Rule": Rule{}, "TestDefinition": TestDefinition{},
		"Projection": BusinessProjection{}, "ProjectedNode": ProjectedNode{}, "Readiness": NodeReadiness{}, "Obligation": ObligationStatus{},
		"Coverage": BusinessCoverage{}, "Measure": CoverageMeasure{}, "Diagnostic": BusinessDiagnostic{}, "ProofContext": ProofContext{},
		"Catalog": WorkbenchCatalog{}, "ModelEntry": ModelCatalogEntry{}, "RoundEntry": RoundCatalogEntry{}, "RunnerEntry": RunnerCatalogEntry{},
		"FixtureEntry": AccountCatalogEntry{}, "RunEntry": RunCatalogEntry{}, "EnvironmentEntry": EnvironmentCatalogEntry{},
		"Trace": TraceReport{}, "TraceStep": StepTrace{}, "Recovery": RecoveryOutcome{}, "Impact": ImpactReport{}, "StepRaw": StepRAW{},
	}
	names := []string{}
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ts, found := interfaces[name]
		if !found {
			t.Errorf("types.ts has no interface %s", name)
			continue
		}
		goFields := uniqueIDs(goJSONFields(reflect.TypeOf(pairs[name])))
		if strings.Join(ts, ",") != strings.Join(goFields, ",") {
			t.Errorf("%s fields differ:\n  types.ts: %v\n  Go DTO:   %v", name, ts, goFields)
		}
	}
}

// TestSnapshotSampleForWorkbench exports, through the CLI, the development sample the static
// workbench reads (a failed attempt and its passing retry). Set ARC_VERIFY_SAMPLE_OUT to keep it.
func TestSnapshotSampleForWorkbench(t *testing.T) {
	enrolled, broken := 0, true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/enroll":
			enrolled = 1
			fmt.Fprint(w, `{}`)
		case "/reset":
			enrolled = 0
			fmt.Fprint(w, `{}`)
		default:
			state, capacity := "pending", 1
			if enrolled > 0 {
				state = "ready"
				if broken {
					capacity = 0
				}
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":%d}`, state, enrolled, capacity)
		}
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "sample-runner", "R1/POSITIVE"))
	var failed, passed ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "sample-runner", "R1/POSITIVE"), &failed); err != nil || failed.Verdict != "FAIL" {
		t.Fatalf("sample needs a failed attempt: %+v %v", failed, err)
	}
	if _, err := fixture.d.retry("course-v2/R1/POSITIVE", "fixture independently restored", failed.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	broken = false
	if err := json.Unmarshal(fixture.execute(t, "sample-runner", "R1/POSITIVE"), &passed); err != nil || passed.Verdict != "PASS" {
		t.Fatalf("sample needs a passing retry: %+v %v", passed, err)
	}
	out := filepath.Join(t.TempDir(), "sample")
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", fixture.round, "--run", failed.Run+","+passed.Run, "--config", fixture.config, "--assets", snapshotAssets(t), "--out", out)
	snapshot := readSnapshot(t, out)
	if len(snapshot.Runs) != 2 || snapshot.Runs[0].Trace.FirstDivergence == "" || snapshot.Runs[1].Trace.Status != "PASS" {
		t.Fatalf("sample lost the failed replay or its retry: %+v", snapshot.Runs)
	}
	statuses := map[string]bool{}
	for _, obligation := range snapshot.Views[viewVerification].Obligations {
		statuses[obligation.Status] = true
	}
	if !statuses["FLAKY"] || !statuses["BLOCKED"] {
		t.Fatalf("sample must show a flaky obligation next to blocked ones: %v", statuses)
	}
	if site := os.Getenv("ARC_VERIFY_SAMPLE_SITE"); site != "" {
		// Exports the same ledger with the real build for browser acceptance.
		cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", fixture.round, "--run", failed.Run+","+passed.Run, "--config", fixture.config, "--assets", os.Getenv("ARC_VERIFY_SAMPLE_ASSETS"), "--out", site)
	}
	if target := os.Getenv("ARC_VERIFY_SAMPLE_OUT"); target != "" {
		body, err := os.ReadFile(filepath.Join(out, "snapshot.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestWorkbenchSourceHasNoNetworkLayer keeps the static page free of any request or write path.
func TestWorkbenchSourceHasNoNetworkLayer(t *testing.T) {
	entries, err := os.ReadDir("workbench/src")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) || strings.Contains(name, ".test.") {
			continue
		}
		body, err := os.ReadFile(filepath.Join("workbench/src", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"fetch(", "XMLHttpRequest", "/api/", "X-Arc", "EventSource", "WebSocket"} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("%s contains %s", name, forbidden)
			}
		}
	}
}
