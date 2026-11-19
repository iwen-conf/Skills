package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkeletonExtractsGoAndSQLWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	goSrc := `package wallet

type Canceller interface {
	Precheck(userID int64) error
}

type WalletService struct {
	repo string
}

func (s *WalletService) FreezeBalance(userID int64) error {
	secretBody := "must not appear in skeleton"
	_ = secretBody
	return nil
}

func hiddenHelper() {}
`
	sqlSrc := `CREATE TABLE wallet_accounts (
	id BIGINT PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id),
	available_cents BIGINT NOT NULL DEFAULT 0,
	frozen_cents BIGINT NOT NULL DEFAULT 0
);
`
	goPath := filepath.Join(dir, "wallet.go")
	sqlPath := filepath.Join(dir, "001_wallet.sql")
	if err := os.WriteFile(goPath, []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlPath, []byte(sqlSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	syms, err := extractSkeleton(goPath, false)
	if err != nil {
		t.Fatalf("extract Go skeleton: %v", err)
	}
	if len(syms) != 3 {
		t.Fatalf("expected 3 exported Go symbols (interface, struct, method), got %d: %+v", len(syms), syms)
	}
	for _, s := range syms {
		if strings.Contains(s.Signature, "secretBody") {
			t.Fatalf("skeleton leaked function body: %+v", s)
		}
		if s.Name == "hiddenHelper" {
			t.Fatalf("unexported symbol leaked when includeAll=false")
		}
	}

	sqlSyms, err := extractSkeleton(sqlPath, false)
	if err != nil || len(sqlSyms) != 1 || sqlSyms[0].Name != "wallet_accounts" {
		t.Fatalf("unexpected SQL skeleton: %+v err=%v", sqlSyms, err)
	}
}

func TestImpactScansCrossLayerEcommerceCouplings(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("db/migrations/001_init.sql", `
CREATE TABLE users (
	id BIGINT PRIMARY KEY,
	status TEXT NOT NULL
);
CREATE TABLE wallet_accounts (
	id BIGINT PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id),
	available_cents BIGINT NOT NULL,
	frozen_cents BIGINT NOT NULL
);
CREATE TABLE fulfillment_orders (
	id BIGINT PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id),
	delivery_status TEXT NOT NULL
);
`)
	mustWrite("internal/settlement/model.go", `package settlement
type SettlementOrder struct {
	ID           int64
	UserID       int64  `+"`gorm:\"column:user_id\"`"+`
	PendingCents int64  `+"`db:\"pending_cents\"`"+`
	Status       string `+"`db:\"status\"`"+`
}
func Notify(bus Bus, userID int64) {
	bus.Publish("user.settlement.callback", userID)
}
`)
	mustWrite("internal/account/router.go", `package account
func Register(r Router) {
	r.POST("/api/v1/users/cancel", HandleCancel)
}
`)
	mustWrite("docs/notes.md", "user_id mentioned in docs should be excluded when prodOnly=true")

	repo, err := findRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := scanImpact(repo, "users.id", true)
	if err != nil {
		t.Fatalf("scanImpact failed: %v", err)
	}
	if len(report.SQLTables) < 3 {
		t.Fatalf("expected >=3 SQL tables (users, wallet_accounts, fulfillment_orders), got %+v", report.SQLTables)
	}
	if len(report.ORMModels) != 1 || report.ORMModels[0].Model != "SettlementOrder" {
		t.Fatalf("expected SettlementOrder ORM model, got %+v", report.ORMModels)
	}
	if len(report.Routes) != 1 {
		t.Fatalf("expected 1 route hit, got %+v", report.Routes)
	}
	if len(report.AsyncTopics) != 1 || report.AsyncTopics[0].Topic != "user.settlement.callback" {
		t.Fatalf("expected async topic user.settlement.callback, got %+v", report.AsyncTopics)
	}
	if len(report.WorldFlagHints) < 4 {
		t.Fatalf("expected >=4 world_flag_hints, got %+v", report.WorldFlagHints)
	}
	for _, ref := range report.ProdReferences {
		if ref.File == "docs/notes.md" {
			t.Fatalf("prodOnly=true leaked doc file into ProdReferences")
		}
	}
}

func TestSearchAuthorityRankingPrefersProdOverDocs(t *testing.T) {
	res := &searchResult{
		Files: []fileResult{
			{Path: "docs/cancel.md", Score: 10, Matches: []lineMatch{{Line: 1, Text: "cancel"}}},
			{Path: "internal/account/cancel_test.go", Score: 10, Matches: []lineMatch{{Line: 1, Text: "cancel"}}},
			{Path: "internal/account/cancel.go", Score: 10, Matches: []lineMatch{{Line: 1, Text: "cancel"}}},
			{Path: "db/migrations/004_cancel.sql", Score: 10, Matches: []lineMatch{{Line: 1, Text: "cancel"}}},
		},
	}
	rankAndFilterSearch(res, false, 10)
	if res.Files[0].Path != "internal/account/cancel.go" {
		t.Fatalf("expected prod file first, got %s", res.Files[0].Path)
	}
	if res.Files[1].Path != "db/migrations/004_cancel.sql" {
		t.Fatalf("expected contract file second, got %s", res.Files[1].Path)
	}
	if res.Files[len(res.Files)-1].Path != "docs/cancel.md" {
		t.Fatalf("expected doc file last, got %s", res.Files[len(res.Files)-1].Path)
	}

	rankAndFilterSearch(res, true, 10)
	if len(res.Files) != 2 {
		t.Fatalf("prodOnly should drop test and doc files, got %+v", res.Files)
	}
}

func TestDiffManifestDetectsChangesAndRemovals(t *testing.T) {
	prev := map[string]string{"a.go": "git:1", "b.go": "git:2", "c.go": "git:3"}
	curr := map[string]string{"a.go": "git:1", "b.go": "git:9", "d.go": "git:4"}
	changed, removed := diffManifest(prev, curr)
	if len(changed) != 2 || changed[0] != "b.go" || changed[1] != "d.go" {
		t.Fatalf("unexpected changed: %v", changed)
	}
	if len(removed) != 1 || removed[0] != "c.go" {
		t.Fatalf("unexpected removed: %v", removed)
	}
}
