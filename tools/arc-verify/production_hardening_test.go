package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestProductionRAWIdentityFieldsMustMatch(t *testing.T) {
	for _, field := range []string{"graph", "task", "environment", "target", "source_commit", "source_sha256", "config_sha256", "graph_sha256", "actor", "account", "principal", "account_sha256", "fixture", "fixture_sha256"} {
		t.Run(field, func(t *testing.T) {
			d := isolatedLedger(t, sample())
			runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "candidate")
			if err != nil {
				t.Fatal(err)
			}
			raw := observedRAW(t, d, runID, "ready", true)
			body, err := json.Marshal(raw["identity"])
			if err != nil {
				t.Fatal(err)
			}
			identity := map[string]string{}
			if err = json.Unmarshal(body, &identity); err != nil {
				t.Fatal(err)
			}
			identity[field] = "different-observed-identity"
			raw["identity"] = identity
			got, err := d.record(runID, "ready", writeRAW(t, raw))
			if err == nil && got["verdict"] == "PASS" {
				t.Fatalf("mismatched nested identity %s passed: %+v", field, got)
			}
		})
	}
}

func failedProductionRun(t *testing.T) (db, string, RunBinding) {
	t.Helper()
	d := isolatedLedger(t, sample())
	runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "candidate")
	if err != nil {
		t.Fatal(err)
	}
	raw := observedRAW(t, d, runID, "ready", true)
	raw["observed"] = "BROKEN"
	result, err := d.step(runID, "", "BROKEN", writeRAW(t, raw))
	if err != nil || result["verdict"] != "FAIL" {
		t.Fatalf("failure fixture: %+v %v", result, err)
	}
	return d, runID, observedBinding(t, d, runID)
}

func productionCheckpoint(t *testing.T, d db, runID string, old RunBinding) (RunBinding, map[string]any) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(old.SourceRoot, "business.txt"), []byte("repaired candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	binding, err := d.startBinding(old.Identity.Task, old.Identity.Environment, old.Identity.SourceCommit, old.Identity.Account, old.Identity.Fixture, old.SourceRoot, old.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return binding, map[string]any{
		"identity": binding.Identity, "run": runID, "step": 0, "phase": "fixture",
		"expected": "READY", "observed": "READY", "preconditions": map[string]bool{"login": true},
		"safe_to_continue": true, "oracle": binding.Oracle, "runner": "independent-reset-observer", "exit_code": 0,
	}
}

func retryProductionTask(d db, binding RunBinding, path string) error {
	args := []string{"retry", "--db", d.file, "--task", binding.Identity.Task, "--reason", "fixture reconstructed and read back", "--root", binding.SourceRoot, "--config", binding.ConfigPath, "--commit", binding.Identity.SourceCommit}
	if path != "" {
		args = append(args, "--evidence", path)
	}
	return run(args)
}

func TestProductionRetryRequiresCheckpoint(t *testing.T) {
	d, _, binding := failedProductionRun(t)
	if err := retryProductionTask(d, binding, ""); err == nil {
		t.Fatal("a reason alone released a failed task for retry")
	}
	rows, err := d.query("SELECT status FROM tasks WHERE id="+quote(binding.Identity.Task), 1)
	if err != nil || len(rows) != 1 || rows[0][0] != "FAIL" {
		t.Fatalf("missing checkpoint changed the failed task: %+v %v", rows, err)
	}
}

func TestProductionRetryRestoresLocksAndKeepsFailure(t *testing.T) {
	d, oldRun, old := failedProductionRun(t)
	binding, raw := productionCheckpoint(t, d, oldRun, old)
	path := writeRAW(t, raw)
	if err := retryProductionTask(d, binding, path); err != nil {
		t.Fatal(err)
	}
	newRun, err := d.start(binding.Identity.Task, binding)
	if err != nil {
		t.Fatalf("verified reconstruction could not start a fresh attempt: %v", err)
	}
	rows, err := d.query("SELECT status FROM runs WHERE id="+quote(oldRun), 1)
	if err != nil || len(rows) != 1 || rows[0][0] != "FAIL" || newRun == oldRun {
		t.Fatalf("recovery rewrote historical failure: %+v new=%s old=%s err=%v", rows, newRun, oldRun, err)
	}
	if _, err = d.record(newRun, "ready", writeRAW(t, observedRAW(t, d, newRun, "ready", true))); err != nil {
		t.Fatal(err)
	}
}

func TestProductionRetryRejectsUnverifiedOrChangedCheckpoint(t *testing.T) {
	for _, damage := range []string{"state", "conditions", "identity", "exit", "unsafe", "phase", "after-retry"} {
		t.Run(damage, func(t *testing.T) {
			d, oldRun, old := failedProductionRun(t)
			binding, raw := productionCheckpoint(t, d, oldRun, old)
			switch damage {
			case "state":
				raw["observed"] = "UNKNOWN"
			case "conditions":
				raw["preconditions"] = map[string]bool{"login": false}
			case "identity":
				identity := binding.Identity
				identity.Principal = "different-account"
				raw["identity"] = identity
			case "exit":
				raw["exit_code"] = 124
			case "unsafe":
				raw["safe_to_continue"] = false
			case "phase":
				raw["phase"] = "action"
			}
			path := writeRAW(t, raw)
			err := retryProductionTask(d, binding, path)
			if damage == "after-retry" {
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(path, []byte("replaced checkpoint"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err = d.start(binding.Identity.Task, binding); err == nil {
					t.Fatal("start accepted a replaced checkpoint")
				}
				return
			}
			if err == nil {
				t.Fatalf("%s checkpoint was accepted", damage)
			}
			locks, err := d.query("SELECT fixture FROM fixture_locks WHERE run="+quote(oldRun), 1)
			if err != nil || len(locks) != 2 {
				t.Fatalf("invalid recovery released original locks: %+v %v", locks, err)
			}
		})
	}
}

func TestProductionRequalifyChangedCandidatePreservesHistory(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	rows, err := d.query("SELECT body FROM run_bindings ORDER BY run LIMIT 1", 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("bindings: %+v %v", rows, err)
	}
	var binding RunBinding
	if err = decodeUniqueJSON([]byte(rows[0][0]), &binding); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(binding.SourceRoot, "business.txt"), []byte("new release candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"prepare", "--db", d.file, "--graph", "academic-v1", "--env", binding.Identity.Environment, "--commit", binding.Identity.SourceCommit, "--root", binding.SourceRoot, "--config", binding.ConfigPath}); err != nil {
		t.Fatalf("cannot requalify a new candidate: %v", err)
	}
	ready, err := d.query("SELECT id FROM tasks WHERE status='READY'", 1)
	if err != nil || len(ready) != 3 {
		t.Fatalf("new candidate retained old PASS tasks: %+v %v", ready, err)
	}
	history, err := d.query("SELECT id FROM runs WHERE status='PASS'", 1)
	if err != nil || len(history) != 3 {
		t.Fatalf("requalification rewrote historic PASS attempts: %+v %v", history, err)
	}
	gate, err := d.gate("academic-v1")
	if err != nil || gate["verdict"] == "PASS" {
		t.Fatalf("old evidence qualified the new candidate: %+v %v", gate, err)
	}
}

func TestProductionFixtureImportCollisionIsAtomic(t *testing.T) {
	d := isolatedLedger(t, sample())
	fixture := Fixture{ID: "case", Kind: "business", State: "READY"}
	if n, err := d.importFixtures(Fixtures{Fixtures: []Fixture{fixture}}); err != nil || n != 1 {
		t.Fatalf("initial import: %d %v", n, err)
	}
	if n, err := d.importFixtures(Fixtures{Fixtures: []Fixture{fixture}}); err != nil || n != 0 {
		t.Fatalf("same fixture reported an actual insertion: %d %v", n, err)
	}
	changed := fixture
	changed.State = "DIFFERENT"
	if _, err := d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "new-case", Kind: "business", State: "READY"}, changed}}); err == nil {
		t.Fatal("same fixture ID silently ignored a different recipe")
	}
	rows, err := d.fixture("new-case")
	if err != nil || len(rows) != 0 {
		t.Fatalf("collision left a partially imported registry: %+v %v", rows, err)
	}
}

type productionGateMutationDriver struct {
	inner           driver.Driver
	afterLedgerRead func()
}

func (d productionGateMutationDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return productionGateMutationConn{Conn: conn, afterLedgerRead: d.afterLedgerRead}, nil
}

type productionGateMutationConn struct {
	driver.Conn
	afterLedgerRead func()
}

func (c productionGateMutationConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.Conn.Prepare(query)
	if err != nil || !strings.Contains(query, "LEFT JOIN run_bindings b") {
		return stmt, err
	}
	return productionGateMutationStmt{Stmt: stmt, afterLedgerRead: c.afterLedgerRead}, nil
}

type productionGateMutationStmt struct {
	driver.Stmt
	afterLedgerRead func()
}

func (s productionGateMutationStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := s.Stmt.Query(args)
	if err != nil {
		return nil, err
	}
	return &productionGateMutationRows{Rows: rows, afterLedgerRead: s.afterLedgerRead}, nil
}

type productionGateMutationRows struct {
	driver.Rows
	afterLedgerRead func()
	once            sync.Once
}

func (r *productionGateMutationRows) Close() error {
	err := r.Rows.Close()
	if err == nil {
		r.once.Do(r.afterLedgerRead)
	}
	return err
}

func TestProductionGateRejectsLateFixtureRegistryChange(t *testing.T) {
	native := isolatedLedger(t, sample())
	completeIsolatedGraph(t, native)
	called := false
	var mutationErr error
	name := newID("production-late-fixture")
	sql.Register(name, productionGateMutationDriver{inner: native.handle.Driver(), afterLedgerRead: func() {
		called = true
		_, mutationErr = native.sql("UPDATE fixtures SET body='{}' WHERE id='isolated-test/case'")
	}})
	conn, err := sql.Open(name, native.file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1)
	scheduled := db{file: native.file, handle: conn}
	gate, err := scheduled.gate("academic-v1")
	if !called || mutationErr != nil {
		t.Fatalf("late registry schedule failed: called=%v err=%v", called, mutationErr)
	}
	if err == nil && gate["verdict"] == "PASS" {
		t.Fatalf("registry changed after the first snapshot but gate passed: %+v", gate)
	}
}

func TestProductionRunEvidenceBindingsAbsent(t *testing.T) {
	for _, field := range []string{"environment", "source_commit", "account", "actor", "fixture", "oracle"} {
		t.Run(field, func(t *testing.T) {
			d := isolatedLedger(t, sample())
			runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "0123456789012345678901234567890123456789")
			if err != nil {
				t.Fatal(err)
			}
			raw := observedRAW(t, d, runID, "ready", true)
			raw[field] = "unrelated-execution"
			got, err := d.record(runID, "ready", writeRAW(t, raw))
			if err == nil && got["verdict"] == "PASS" {
				t.Fatalf("mismatched %s still completed PASS: %+v", field, got)
			}
		})
	}
}

func TestProductionGraphCannotMixCandidates(t *testing.T) {
	d := isolatedLedger(t, sample())
	cards, err := d.next("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	for i, card := range cards {
		commit := fmt.Sprintf("%040x", i+1)
		runID, err := startTestRun(t, d, card.ID, "isolated-test", commit)
		if err != nil {
			t.Fatal(err)
		}
		raw := observedRAW(t, d, runID, card.Expected, card.Type == "POSITIVE")
		got, err := d.record(runID, card.Expected, writeRAW(t, raw))
		if err != nil || got["verdict"] != "PASS" {
			t.Fatalf("valid individual candidate execution failed: %+v %v", got, err)
		}
	}
	result, err := d.gate("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	if result["verdict"] == "PASS" {
		t.Fatalf("different source candidates were combined into one PASS: %+v", result)
	}
}

type productionScheduleDriver struct {
	inner       driver.Driver
	beforeBegin func()
}

func (d productionScheduleDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &productionScheduleConn{Conn: c, beforeBegin: d.beforeBegin}, nil
}

type productionScheduleConn struct {
	driver.Conn
	beforeBegin func()
	once        sync.Once
}

func (c *productionScheduleConn) Begin() (driver.Tx, error) {
	c.once.Do(c.beforeBegin)
	return c.Conn.Begin()
}

func TestProductionFinishAfterConcurrentAbortDoesNotPass(t *testing.T) {
	native := isolatedLedger(t, sample())
	runID, err := startTestRun(t, native, "academic-v1/R1/POSITIVE", "isolated-test", "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = native.step(runID, "", "ready", writeRAW(t, observedRAW(t, native, runID, "ready", true))); err != nil {
		t.Fatal(err)
	}
	var abortErr error
	name := "production-schedule-" + runID
	sql.Register(name, productionScheduleDriver{inner: native.handle.Driver(), beforeBegin: func() {
		_, abortErr = native.abort(runID, "runner interruption committed before stale finish")
	}})
	conn, err := sql.Open(name, native.file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	scheduled := db{file: native.file, handle: conn}
	got, err := scheduled.finish(runID)
	if abortErr != nil {
		t.Fatalf("schedule could not commit abort: %v", abortErr)
	}
	rows, readErr := native.query("SELECT r.status,t.status FROM runs r JOIN tasks t ON t.id=r.task WHERE r.id="+quote(runID), 2)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil && got["verdict"] == "PASS" {
		t.Fatalf("finish claimed PASS after committed abort: run/task=%+v", rows)
	}
	if len(rows) != 1 || rows[0][0] != "BLOCKED" || rows[0][1] != "BLOCKED" {
		t.Fatalf("abort state was overwritten: %+v", rows)
	}
}
