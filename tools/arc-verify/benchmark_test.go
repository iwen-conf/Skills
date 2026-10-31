package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Synthetic microbenchmarks only. These measure graph/ledger CPU and storage
// overhead; they do NOT prove browser E2E, business correctness or trusted RAW.
func benchmarkReadyGraph(b *testing.B, rules int) (db, string) {
	b.Helper()
	d, e := openDB(filepath.Join(b.TempDir(), "data.db"), true)
	if e != nil {
		b.Fatal(e)
	}
	for _, statement := range schema {
		if _, e = d.sql(statement); e != nil {
			b.Fatal(e)
		}
	}
	g := sample()
	for i := 1; i < rules; i++ {
		r := g.Rules[0]
		r.ID = fmt.Sprintf("R-%05d", i)
		g.Rules = append(g.Rules, r)
	}
	if e = validateGraph(g); e != nil {
		b.Fatal(e)
	}
	if e = d.importGraph(g); e != nil {
		b.Fatal(e)
	}
	if e = d.approve(g.ID, ""); e != nil {
		b.Fatal(e)
	}
	stmts := make([]string, 0, rules)
	for _, r := range g.Rules {
		stmts = append(stmts, fmt.Sprintf("UPDATE rules SET approved=1 WHERE graph=%s AND id=%s;", quote(g.ID), quote(r.ID)))
	}
	if e = d.tx(stmts); e != nil {
		b.Fatal(e)
	}
	if _, e = d.plan(g.ID); e != nil {
		b.Fatal(e)
	}
	return d, g.ID
}
func BenchmarkTaskCardPagination(b *testing.B) {
	d, id := benchmarkReadyGraph(b, 300)
	defer d.close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cards, e := d.next(id)
		if e != nil || len(cards) != 8 {
			b.Fatalf("cards %d: %v", len(cards), e)
		}
	}
}
func BenchmarkGateWithRAW(b *testing.B) {
	d, id := benchmarkReadyGraph(b, 10)
	defer d.close()
	cards, e := d.query("SELECT "+fieldsSQL("id", "expected", "kind")+" FROM tasks WHERE graph="+quote(id)+";", 3)
	if e != nil {
		b.Fatal(e)
	}
	for _, card := range cards {
		run, e := startTestRun(b, d, card[0], "bench-isolated", "test-commit")
		if e != nil {
			b.Fatal(e)
		}
		binding := observedBinding(b, d, run)
		raw := StepRAW{Identity: binding.Identity, Run: run, Step: 1, Expected: card[1], Observed: card[1],
			Oracle: binding.Oracle, Runner: "benchmark-not-e2e", ExitCode: 0,
			Preconditions: map[string]bool{"login": card[2] == "POSITIVE"}}
		if card[1] == "UNCHANGED" {
			raw.Denied = true
			raw.Before = "READY"
			raw.After = "READY"
		}
		content, _ := json.Marshal(raw)
		path := filepath.Join(b.TempDir(), "proof.json")
		// b.TempDir is unique per iteration; data remains for the whole benchmark.
		if e = os.WriteFile(path, content, 0600); e != nil {
			b.Fatal(e)
		}
		if _, e = d.step(run, "", card[1], path); e != nil {
			b.Fatal(e)
		}
		if _, e = d.finish(run); e != nil {
			b.Fatal(e)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, e := d.gate(id)
		if e != nil || result["verdict"] != "PASS" {
			b.Fatalf("gate %+v: %v", result, e)
		}
	}
}
