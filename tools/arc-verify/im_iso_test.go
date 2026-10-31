package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIMMongoFixturesMustBindIsolationNotSharedLan50(t *testing.T) {
	path := imMongoFixturesPath()
	if path == "" {
		t.Skip("IM-mongo verify fixtures are not present on this machine")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures Fixtures
	if err := json.Unmarshal(body, &fixtures); err != nil {
		t.Fatal(err)
	}
	if err := fixturesBindIsolationOnly(fixtures, imIsolationEnv); err != nil {
		t.Fatal(err)
	}
}

func TestRefuseSharedIMWriteTarget(t *testing.T) {
	if err := refuseSharedIMTarget("http://192.168.1.50:28886"); err != nil {
		t.Fatalf("isolation writer must be allowed: %v", err)
	}
	for _, raw := range []string{
		"http://192.168.1.50:28885",
		"http://tx.nulltech.cn:28885",
		"http://192.168.1.50:28895",
		"http://121.43.161.42:28885",
	} {
		if err := refuseSharedIMTarget(raw); err == nil {
			t.Fatalf("shared or production target was accepted: %s", raw)
		}
	}
}

func TestIMMongoConfigKeepsSharedStacksUnapproved(t *testing.T) {
	path := imMongoConfigPath()
	if path == "" {
		t.Skip("IM-mongo verify config is not present on this machine")
	}
	cfg, _, err := readEnvironmentConfiguration(path)
	if err != nil {
		t.Fatal(err)
	}
	if env := cfg.Environments["lan50"]; env.Approved {
		t.Fatal("lan50 / 28885 must stay unapproved for writes")
	}
	if env := cfg.Environments["tx"]; env.Approved {
		t.Fatal("tx must stay unapproved for writes")
	}
	iso := cfg.Environments["iso28886"]
	if !iso.Approved || !strings.Contains(iso.URL, ":28886") {
		t.Fatalf("iso28886 must be the approved isolation writer: %+v", iso)
	}
}

func imMongoRoot() string {
	if override := strings.TrimSpace(os.Getenv("IM_MONGO_ROOT")); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	root := filepath.Join(home, "Documents", "Code", "chensi", "IM-mongo")
	if _, err := os.Stat(filepath.Join(root, ".arc", "verify", "config.jsonc")); err != nil {
		return ""
	}
	return root
}

func imMongoFixturesPath() string {
	root := imMongoRoot()
	if root == "" {
		return ""
	}
	path := filepath.Join(root, ".arc", "verify", "fixtures.json")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func imMongoConfigPath() string {
	root := imMongoRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, ".arc", "verify", "config.jsonc")
}
