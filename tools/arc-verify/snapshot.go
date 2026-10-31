package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

const snapshotDefaultMaxBytes = 64 << 20

// snapshotCSP forbids every network request; scripts and styles come only from
// the snapshot directory, and the embedded JSON is data, not script.
const snapshotCSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; base-uri 'none'; form-action 'none'">`

// SnapshotMeta records when and from which ledger context a snapshot was exported.
type SnapshotMeta struct {
	Generated   string   `json:"generated"`
	Version     string   `json:"version"`
	Argv        []string `json:"argv"`
	Graph       string   `json:"graph"`
	GraphSHA256 string   `json:"graph_sha256"`
	Environment string   `json:"environment,omitempty"`
	Round       string   `json:"round,omitempty"`
}

// SnapshotModel is the versioned model with its approval state.
type SnapshotModel struct {
	Data     Graph           `json:"data"`
	Approved bool            `json:"approved"`
	Rules    map[string]bool `json:"rules"`
	SHA256   string          `json:"sha256"`
}

// SnapshotEvidence is one re-verified RAW, or the reason it failed verification.
type SnapshotEvidence struct {
	Run    string   `json:"run"`
	Phase  string   `json:"phase"`
	Step   int      `json:"step"`
	SHA256 string   `json:"sha256,omitempty"`
	Data   *StepRAW `json:"data,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// SnapshotRun is a selected run's replay with its evidence.
type SnapshotRun struct {
	Trace    TraceReport        `json:"trace"`
	Evidence []SnapshotEvidence `json:"evidence"`
}

// Snapshot is the read-only document embedded in the static workbench page.
type Snapshot struct {
	Meta    SnapshotMeta                        `json:"meta"`
	Catalog WorkbenchCatalog                    `json:"catalog"`
	Model   SnapshotModel                       `json:"model"`
	Views   map[BusinessView]BusinessProjection `json:"views"`
	Impact  *ImpactReport                       `json:"impact,omitempty"`
	Runs    []SnapshotRun                       `json:"runs"`
}

// SnapshotRequest selects an existing ledger context to export.
type SnapshotRequest struct {
	Graph, Environment, Round, Config, Assets, Out string
	Runs                                           []string
	Argv                                           []string
	MaxBytes                                       int
}

// writeSnapshot exports one ledger context as snapshot.json plus a static page
// that embeds the same JSON. Nothing is written unless every check passes.
func (d db) writeSnapshot(request SnapshotRequest) (string, error) {
	if request.Graph == "" || request.Assets == "" {
		return "", errors.New("snapshot requires --graph and --assets (the built static workbench)")
	}
	limit := request.MaxBytes
	if limit == 0 {
		limit = snapshotDefaultMaxBytes
	}
	out := request.Out
	if out == "" {
		out = filepath.Join(filepath.Dir(d.file), "snapshots", request.Graph, time.Now().UTC().Format("20060102T150405Z"))
	}
	if err := requireEmptyOutput(out); err != nil {
		return "", err
	}
	snapshot, err := d.buildSnapshot(request)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	if len(body) > limit {
		return "", snapshotSizeError(snapshot, len(body), limit)
	}
	page, err := snapshotPage(request.Assets, body)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(out)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(parent, ".snapshot-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	if err := copySnapshotAssets(request.Assets, staging); err != nil {
		return "", err
	}
	for name, content := range map[string][]byte{"snapshot.json": body, "index.html": page} {
		if err := os.WriteFile(filepath.Join(staging, name), content, 0o600); err != nil {
			return "", err
		}
	}
	if err := requireEmptyOutput(out); err != nil {
		return "", err
	}
	if err := os.Remove(out); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(staging, out); err != nil {
		return "", err
	}
	return out, nil
}

func requireEmptyOutput(out string) error {
	info, err := os.Lstat(out)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("snapshot output %s must not exist or be an empty directory", out)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("snapshot output %s must not exist or be an empty directory; snapshots are never overwritten", out)
	}
	return nil
}

func (d db) buildSnapshot(request SnapshotRequest) (Snapshot, error) {
	g, err := d.model(request.Graph)
	if err != nil {
		return Snapshot{}, fmt.Errorf("graph %s: %w", request.Graph, err)
	}
	if g.SchemaVersion != 2 {
		return Snapshot{}, fmt.Errorf("snapshot requires a v2 business model; %s has schema version %d", g.ID, g.SchemaVersion)
	}
	if request.Round != "" {
		round, err := d.verificationRound(request.Round)
		if err != nil {
			return Snapshot{}, fmt.Errorf("round %s: %w", request.Round, err)
		}
		if round.Context.Graph != g.ID || request.Environment != "" && round.Context.Environment != request.Environment {
			return Snapshot{}, fmt.Errorf("round %s belongs to graph %s and environment %s", round.ID, round.Context.Graph, round.Context.Environment)
		}
	} else if request.Environment != "" {
		rows, err := d.query("SELECT id FROM verification_rounds WHERE graph="+quote(g.ID)+" AND environment="+quote(request.Environment)+" LIMIT 1", 1)
		if err != nil {
			return Snapshot{}, err
		}
		if len(rows) == 0 {
			return Snapshot{}, fmt.Errorf("environment %s has no verification round for %s", request.Environment, g.ID)
		}
	}
	snapshot := Snapshot{Views: map[BusinessView]BusinessProjection{}, Runs: []SnapshotRun{}}
	snapshot.Model, err = d.snapshotModel(g)
	if err != nil {
		return Snapshot{}, err
	}
	for _, view := range []BusinessView{viewOverview, viewCapabilities, viewBoundaries, viewFlows, viewVerification, viewImpact} {
		projection, err := d.viewModel(ViewQuery{Graph: g.ID, View: view, Environment: request.Environment, Round: request.Round, Limit: 1024})
		if err != nil {
			return Snapshot{}, fmt.Errorf("view %s: %w", view, err)
		}
		snapshot.Views[view] = projection
	}
	overview := snapshot.Views[viewOverview]
	snapshot.Meta = SnapshotMeta{Generated: time.Now().UTC().Format(time.RFC3339), Version: cliVersion(), Argv: append([]string{}, request.Argv...), Graph: g.ID, GraphSHA256: snapshot.Model.SHA256, Environment: overview.Environment, Round: overview.Round}
	if g.Previous != "" {
		impact, err := d.diffModel(g.ID, g.Previous, nil)
		if err != nil {
			return Snapshot{}, fmt.Errorf("difference from %s: %w", g.Previous, err)
		}
		snapshot.Impact = &impact
	}
	catalog, err := d.workbenchCatalog(request.Config)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Catalog = catalogForGraph(catalog, g)
	for _, run := range request.Runs {
		exported, err := d.snapshotRun(g.ID, run)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Runs = append(snapshot.Runs, exported)
	}
	return snapshot, nil
}

func (d db) snapshotModel(g Graph) (SnapshotModel, error) {
	result := SnapshotModel{Data: g, Rules: map[string]bool{}}
	rows, err := d.query("SELECT id,approved FROM rules WHERE graph="+quote(g.ID), 2)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Rules[row[0]] = row[1] == "1"
	}
	approved, err := d.query("SELECT approved FROM graphs WHERE id="+quote(g.ID), 1)
	if err != nil {
		return result, err
	}
	result.Approved = len(approved) == 1 && approved[0][0] == "1"
	result.SHA256, err = d.graphFingerprint(g.ID)
	return result, err
}

// snapshotRun re-verifies every recorded RAW of a run; a RAW that fails
// verification is exported as a failure entry, never dropped or shown as valid.
func (d db) snapshotRun(graph, run string) (SnapshotRun, error) {
	rows, err := d.query("SELECT t.graph FROM runs r JOIN tasks t ON t.id=r.task WHERE r.id="+quote(run), 1)
	if err != nil {
		return SnapshotRun{}, err
	}
	if len(rows) != 1 {
		return SnapshotRun{}, fmt.Errorf("run %s not found", run)
	}
	if rows[0][0] != graph {
		return SnapshotRun{}, fmt.Errorf("run %s belongs to graph %s", run, rows[0][0])
	}
	trace, err := d.trace(run)
	if err != nil {
		return SnapshotRun{}, err
	}
	result := SnapshotRun{Trace: trace, Evidence: []SnapshotEvidence{}}
	add := func(phase string, step int) {
		entry := SnapshotEvidence{Run: run, Phase: phase, Step: step}
		raw, hash, err := d.workbenchEvidence(run, phase, step)
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Data, entry.SHA256 = &raw, hash
		}
		result.Evidence = append(result.Evidence, entry)
	}
	if trace.Round != "" {
		add("setup", 0)
	}
	for _, step := range trace.Steps {
		index, err := strconv.Atoi(step.Index)
		if err != nil {
			return SnapshotRun{}, fmt.Errorf("run %s has a non-numeric step %q", run, step.Index)
		}
		add("action", index)
	}
	if trace.Round != "" {
		add("fixture", 0)
	}
	return result, nil
}

func catalogForGraph(catalog WorkbenchCatalog, g Graph) WorkbenchCatalog {
	result := catalog
	result.Models, result.Rounds, result.Runners, result.Runs = []ModelCatalogEntry{}, []RoundCatalogEntry{}, []RunnerCatalogEntry{}, []RunCatalogEntry{}
	for _, model := range catalog.Models {
		if model.ID == g.ID || model.ID == g.Previous {
			result.Models = append(result.Models, model)
		}
	}
	for _, round := range catalog.Rounds {
		if round.Graph == g.ID {
			result.Rounds = append(result.Rounds, round)
		}
	}
	for _, runner := range catalog.Runners {
		if runner.Graph == g.ID {
			result.Runners = append(result.Runners, runner)
		}
	}
	for _, run := range catalog.Runs {
		if run.Graph == g.ID {
			result.Runs = append(result.Runs, run)
		}
	}
	return result
}

func snapshotSizeError(snapshot Snapshot, total, limit int) error {
	type part struct {
		name string
		size int
	}
	measure := func(value any) int {
		body, err := json.Marshal(value)
		if err != nil {
			return 0
		}
		return len(body)
	}
	parts := []part{{"meta", measure(snapshot.Meta)}, {"catalog", measure(snapshot.Catalog)}, {"model", measure(snapshot.Model)}, {"views", measure(snapshot.Views)}, {"impact", measure(snapshot.Impact)}}
	for _, run := range snapshot.Runs {
		parts = append(parts, part{"run " + run.Trace.Run, measure(run)})
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].size > parts[j].size })
	listed := []string{}
	for _, item := range parts {
		listed = append(listed, fmt.Sprintf("%s=%d", item.name, item.size))
	}
	return fmt.Errorf("snapshot JSON is %d bytes, over the %d byte limit; parts by size: %s", total, limit, strings.Join(listed, ", "))
}

// snapshotPage embeds the JSON (json.Marshal escapes <, > and &) and the CSP
// right after the opening <head> so they apply before any script loads.
func snapshotPage(assets string, body []byte) ([]byte, error) {
	_, index, err := loadEvidence(filepath.Join(assets, "index.html"))
	if err != nil {
		return nil, fmt.Errorf("workbench build index.html: %w", err)
	}
	page := string(index)
	open := strings.Index(page, "<head")
	end := -1
	if open >= 0 {
		end = strings.Index(page[open:], ">")
	}
	if end < 0 {
		return nil, errors.New("workbench build index.html has no <head> element")
	}
	at := open + end + 1
	inject := snapshotCSP + `<script type="application/json" id="arc-snapshot">` + string(body) + `</script>`
	return []byte(page[:at] + inject + page[at:]), nil
}

// copySnapshotAssets copies the built workbench except index.html; links and
// builds that still call the removed local API are refused.
func copySnapshotAssets(assets, target string) error {
	root, err := filepath.Abs(assets)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." || relative == "index.html" {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("workbench build entry %s is not a regular file", relative)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "/api/") {
			return fmt.Errorf("workbench build file %s still references the removed local API; rebuild the static workbench", relative)
		}
		return os.WriteFile(destination, body, 0o600)
	})
}

func cliVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	version := info.Main.Version
	for _, setting := range info.Settings {
		switch {
		case setting.Key == "vcs.revision":
			version += "+" + setting.Value
		case setting.Key == "vcs.modified" && setting.Value == "true":
			version += "-dirty"
		}
	}
	return version
}

func (d db) workbenchCatalog(config string) (WorkbenchCatalog, error) {
	result := WorkbenchCatalog{Models: []ModelCatalogEntry{}, Rounds: []RoundCatalogEntry{}, Runners: []RunnerCatalogEntry{}, Fixtures: []AccountCatalogEntry{}, Runs: []RunCatalogEntry{}, Environments: []EnvironmentCatalogEntry{}}
	configured, _, configErr := readEnvironmentConfiguration(config)
	if configErr != nil {
		result.EnvironmentError = configErr.Error()
	} else {
		for name, environment := range configured.Environments {
			entry := EnvironmentCatalogEntry{Name: name, Approved: environment.Approved}
			target, _, err := readApprovedEnvironment(config, name)
			if err != nil {
				entry.Reason = err.Error()
			} else {
				entry.Target = target
			}
			result.Environments = append(result.Environments, entry)
		}
		sort.Slice(result.Environments, func(i, j int) bool { return result.Environments[i].Name < result.Environments[j].Name })
	}
	models, err := d.query("SELECT g.id,g.scope,g.approved,COALESCE(b.schema_version,1),COALESCE(b.body,'') FROM graphs g LEFT JOIN business_models b ON b.graph=g.id ORDER BY g.rowid DESC LIMIT 513", 5)
	if err != nil {
		return result, err
	}
	if len(models) > 512 {
		result.Truncated = true
		models = models[:512]
	}
	for _, row := range models {
		version, err := strconv.Atoi(row[3])
		if err != nil {
			return result, err
		}
		system := ""
		if version == 2 {
			var graph Graph
			if err := decodeStrictJSON([]byte(row[4]), &graph); err != nil {
				return result, err
			}
			system = graph.System
		}
		result.Models = append(result.Models, ModelCatalogEntry{ID: row[0], Scope: row[1], Approved: row[2] == "1", SchemaVersion: version, System: system})
	}
	rounds, err := d.query("SELECT id FROM verification_rounds ORDER BY rowid DESC LIMIT 257", 1)
	if err != nil {
		return result, err
	}
	if len(rounds) > 256 {
		result.Truncated = true
		rounds = rounds[:256]
	}
	for _, row := range rounds {
		round, err := d.verificationRound(row[0])
		if err != nil {
			return result, err
		}
		validity := "CURRENT"
		if d.validateRound(round) != nil {
			validity = "STALE"
		}
		result.Rounds = append(result.Rounds, RoundCatalogEntry{ID: round.ID, Graph: round.Context.Graph, Environment: round.Context.Environment, Created: round.Created, Validity: validity})
	}
	runners, err := d.query("SELECT id,graph,test,sha256 FROM registered_runners ORDER BY id LIMIT 1025", 4)
	if err != nil {
		return result, err
	}
	if len(runners) > 1024 {
		result.Truncated = true
		runners = runners[:1024]
	}
	for _, row := range runners {
		validity := "CURRENT"
		runner, err := d.registeredRunner(row[0])
		if err != nil {
			validity = "STALE"
		}
		result.Runners = append(result.Runners, RunnerCatalogEntry{ID: row[0], Graph: row[1], Test: row[2], SHA256: row[3], Validity: validity, Scope: runner.Manifest.IsolationScope})
	}
	fixtures, err := d.query("SELECT id FROM fixtures ORDER BY id LIMIT 1025", 1)
	if err != nil {
		return result, err
	}
	if len(fixtures) > 1024 {
		result.Truncated = true
		fixtures = fixtures[:1024]
	}
	for _, row := range fixtures {
		fixture, _, err := d.bindingFixture(row[0])
		if err != nil {
			return result, err
		}
		result.Fixtures = append(result.Fixtures, AccountCatalogEntry{ID: fixture.ID, Kind: fixture.Kind, Environment: fixture.Refs["environment"], Role: fixture.Refs["role"], Scope: fixture.Refs["scope"]})
	}
	runs, err := d.query("SELECT r.id,t.graph,r.task,r.environment,COALESCE(m.round,''),r.status,r.created FROM runs r JOIN tasks t ON t.id=r.task LEFT JOIN model_runs m ON m.run=r.id ORDER BY r.rowid DESC LIMIT 257", 7)
	if err != nil {
		return result, err
	}
	if len(runs) > 256 {
		result.Truncated = true
		runs = runs[:256]
	}
	for _, row := range runs {
		result.Runs = append(result.Runs, RunCatalogEntry{ID: row[0], Graph: row[1], Task: row[2], Environment: row[3], Round: row[4], Status: row[5], Created: row[6]})
	}
	sort.Slice(result.Models, func(i, j int) bool { return result.Models[i].ID < result.Models[j].ID })
	return result, nil
}
