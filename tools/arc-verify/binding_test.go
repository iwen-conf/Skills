package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Synthetic candidate/account fixtures; these helpers never contact a target service.
func startTestRun(t testing.TB, d db, task, environment, candidateLabel string) (string, error) {
	t.Helper()
	root := filepath.Join(filepath.Dir(d.file), "candidate")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) []byte {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=verify-fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		for _, item := range os.Environ() {
			if !strings.HasPrefix(item, "GIT_") {
				cmd.Env = append(cmd.Env, item)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %s: %v %s", args[0], err, raw)
		}
		return raw
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "business.txt"), []byte(candidateLabel), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "business.txt")
	if len(git("diff", "--cached", "--name-only")) > 0 {
		git("commit", "-q", "-m", "synthetic candidate")
	}
	commit := strings.TrimSpace(string(git("rev-parse", "HEAD")))
	config := filepath.Join(filepath.Dir(d.file), "config.jsonc")
	body, err := json.Marshal(map[string]any{"environments": map[string]any{environment: map[string]any{"approved": true, "url": "https://unit-target.invalid"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(config, body, 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := d.query("SELECT q.actor FROM tasks t JOIN rules q ON q.graph=t.graph AND q.id=t.rule WHERE t.id="+quote(task), 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("fixture actor: %+v %v", rows, err)
	}
	role := rows[0][0]
	accountID, fixtureID := environment+"/"+role+"-account", environment+"/case"
	_, err = d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: accountID, Kind: "account", State: "ACTIVE", Refs: map[string]string{"role": role, "principal": "synthetic-" + role, "environment": environment}},
		{ID: fixtureID, Kind: "business", State: "READY", Refs: map[string]string{"environment": environment}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := d.startBinding(task, environment, commit, accountID, fixtureID, root, config)
	if err != nil {
		t.Fatal(err)
	}
	return d.start(task, binding)
}

func observedBinding(t testing.TB, d db, run string) RunBinding {
	t.Helper()
	contract, err := d.runTaskContract(run)
	if err != nil {
		t.Fatal(err)
	}
	return contract.Binding
}
