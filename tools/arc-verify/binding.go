package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ExecutionIdentity struct {
	Graph         string `json:"graph"`
	Task          string `json:"task"`
	Environment   string `json:"environment"`
	Target        string `json:"target"`
	SourceCommit  string `json:"source_commit"`
	SourceSHA256  string `json:"source_sha256"`
	ConfigSHA256  string `json:"config_sha256"`
	GraphSHA256   string `json:"graph_sha256"`
	Actor         string `json:"actor"`
	Account       string `json:"account"`
	Principal     string `json:"principal"`
	AccountSHA256 string `json:"account_sha256"`
	Fixture       string `json:"fixture"`
	FixtureSHA256 string `json:"fixture_sha256"`
}

type RunBinding struct {
	Identity   ExecutionIdentity `json:"identity"`
	Oracle     string            `json:"oracle"`
	SourceRoot string            `json:"source_root"`
	ConfigPath string            `json:"config_path"`
}

type ProofContext struct {
	Graph        string `json:"graph"`
	Environment  string `json:"environment"`
	Target       string `json:"target"`
	SourceCommit string `json:"source_commit"`
	SourceSHA256 string `json:"source_sha256"`
	ConfigSHA256 string `json:"config_sha256"`
	GraphSHA256  string `json:"graph_sha256"`
	SourceRoot   string `json:"source_root"`
	ConfigPath   string `json:"config_path"`
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// The candidate includes HEAD, staged/unstaged changes and every nonignored new file.
func sourceFingerprint(root string) (string, string, error) {
	git := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "GIT_") {
				cmd.Env = append(cmd.Env, value)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("candidate git %s: %w", args[0], err)
		}
		return out, nil
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}
	root = strings.TrimSpace(string(top))
	diff, err := git("diff", "--binary", "--no-ext-diff", "--no-textconv", "HEAD", "--", ".")
	if err != nil {
		return "", "", err
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", "", err
	}
	hash := sha256.New()
	for _, raw := range [][]byte{head, diff} {
		if _, err = hash.Write(raw); err != nil {
			return "", "", err
		}
	}
	names := strings.Split(string(untracked), "\x00")
	sort.Strings(names)
	for _, name := range names {
		if name == "" {
			continue
		}
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if err != nil {
			return "", "", err
		}
		contentHash := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return "", "", err
			}
			contentHash = "link:" + digestBytes([]byte(link))
		} else {
			contentHash, err = digestFile(path)
			if err != nil {
				return "", "", err
			}
		}
		if _, err = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00", name, info.Mode(), contentHash); err != nil {
			return "", "", err
		}
	}
	return strings.TrimSpace(string(head)), hex.EncodeToString(hash.Sum(nil)), nil
}

func (d db) graphFingerprint(graph string) (string, error) {
	hash := sha256.New()
	queries := []struct {
		statement string
		columns   int
	}{
		{"SELECT id,scope,approved FROM graphs WHERE id=" + quote(graph), 3},
		{"SELECT id,kind,name FROM nodes WHERE graph=" + quote(graph) + " ORDER BY id", 3},
		{"SELECT id,source,target,kind FROM edges WHERE graph=" + quote(graph) + " ORDER BY id", 4},
		{"SELECT id,action,actor,entry,oracle,conditions,expected,rejected,source,description,approved FROM rules WHERE graph=" + quote(graph) + " ORDER BY id", 11},
	}
	for i, query := range queries {
		if _, err := fmt.Fprintf(hash, "%d\x00", i); err != nil {
			return "", err
		}
		err := d.each(query.statement, query.columns, func(row []string) error {
			raw, err := json.Marshal(row)
			if err != nil {
				return err
			}
			_, err = hash.Write(raw)
			return err
		})
		if err != nil {
			return "", err
		}
	}
	models, err := d.query("SELECT schema_version,body FROM business_models WHERE graph="+quote(graph), 2)
	if err != nil {
		return "", fmt.Errorf("model schema unavailable; run arc-verify init: %w", err)
	}
	if len(models) != 0 {
		if len(models) != 1 || models[0][0] != "2" {
			return "", errors.New("unsupported stored model version")
		}
		if _, err := hash.Write([]byte("business-model-v2\x00" + models[0][1])); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (d db) bindingFixture(id string) (Fixture, string, error) {
	rows, err := d.fixture(id)
	if err != nil {
		return Fixture{}, "", err
	}
	if len(rows) != 1 {
		return Fixture{}, "", fmt.Errorf("registered fixture %s not found", id)
	}
	var fixture Fixture
	if err = decodeUniqueJSON([]byte(rows[0][1]), &fixture); err != nil {
		return fixture, "", err
	}
	if err = validateFixtures(Fixtures{Fixtures: []Fixture{fixture}}); err != nil {
		return fixture, "", err
	}
	return fixture, digestBytes([]byte(rows[0][1])), nil
}

func (d db) fixtureSnapshot(cache map[string]string) (string, error) {
	rows, err := d.query("SELECT id,body FROM fixtures ORDER BY id", 2)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		var fixture Fixture
		if err = decodeUniqueJSON([]byte(row[1]), &fixture); err != nil {
			return "", err
		}
		if fixture.ID != row[0] {
			return "", errors.New("fixture registry identity does not match its body")
		}
		if err = validateFixtures(Fixtures{Fixtures: []Fixture{fixture}}); err != nil {
			return "", err
		}
		cache["fixture:"+row[0]] = digestBytes([]byte(row[1]))
		cache["account:"+row[0]] = fixture.Kind + "\x00" + fixture.Refs["role"] + "\x00" + fixture.Refs["principal"] + "\x00" + fixture.Refs["environment"]
		cache["fixture-environment:"+row[0]] = fixture.Refs["environment"]
	}
	body, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return digestBytes(body), nil
}

func (d db) startBinding(task, environment, commit, accountID, fixtureID, root, config string) (RunBinding, error) {
	rows, err := d.query("SELECT t.graph,q.actor,q.oracle FROM tasks t JOIN rules q ON q.graph=t.graph AND q.id=t.rule WHERE t.id="+quote(task), 3)
	if err != nil {
		return RunBinding{}, err
	}
	if len(rows) != 1 {
		return RunBinding{}, errors.New("task or rule not found")
	}
	target, configHash, err := readApprovedEnvironment(config, environment)
	if err != nil {
		return RunBinding{}, err
	}
	sourceRoot, err := filepath.Abs(root)
	if err != nil {
		return RunBinding{}, err
	}
	configPath, err := filepath.Abs(config)
	if err != nil {
		return RunBinding{}, err
	}
	head, sourceHash, err := sourceFingerprint(sourceRoot)
	if err != nil {
		return RunBinding{}, err
	}
	if head != commit {
		return RunBinding{}, errors.New("source commit is not the current candidate HEAD")
	}
	account, accountHash, err := d.bindingFixture(accountID)
	if err != nil {
		return RunBinding{}, err
	}
	fixture, fixtureHash, err := d.bindingFixture(fixtureID)
	if err != nil {
		return RunBinding{}, err
	}
	if account.Kind != "account" || account.Refs["role"] != rows[0][1] || strings.TrimSpace(account.Refs["principal"]) == "" {
		return RunBinding{}, errors.New("account fixture requires the frozen actor role and an explicit principal")
	}
	if account.Refs["environment"] != environment || fixture.Refs["environment"] != environment {
		return RunBinding{}, errors.New("account and fixture must belong to the selected environment")
	}
	graphHash, err := d.graphFingerprint(rows[0][0])
	if err != nil {
		return RunBinding{}, err
	}
	return RunBinding{Identity: ExecutionIdentity{
		Graph: rows[0][0], Task: task, Environment: environment, Target: target,
		SourceCommit: commit, SourceSHA256: sourceHash, ConfigSHA256: configHash, GraphSHA256: graphHash,
		Actor: rows[0][1], Account: accountID, Principal: account.Refs["principal"], AccountSHA256: accountHash,
		Fixture: fixtureID, FixtureSHA256: fixtureHash,
	}, Oracle: rows[0][2], SourceRoot: sourceRoot, ConfigPath: configPath}, nil
}

func (d db) validateBinding(binding RunBinding, cache map[string]string) error {
	actual := func(key string, read func() (string, error)) (string, error) {
		if value, found := cache[key]; found {
			return value, nil
		}
		value, err := read()
		if err == nil && cache != nil {
			cache[key] = value
		}
		return value, err
	}
	v := binding.Identity
	if v.Graph == "" || v.Task == "" || v.Environment == "" || v.Target == "" || v.Actor == "" || v.Account == "" || v.Principal == "" || v.Fixture == "" || binding.Oracle == "" || binding.SourceRoot == "" || binding.ConfigPath == "" {
		return errors.New("run has no complete execution binding; retain it as history and start a new attempt")
	}
	source, err := actual("source:"+binding.SourceRoot, func() (string, error) {
		head, hash, err := sourceFingerprint(binding.SourceRoot)
		return head + ":" + hash, err
	})
	if err != nil {
		return err
	}
	target, err := actual("target:"+binding.ConfigPath+":"+v.Environment, func() (string, error) {
		target, hash, err := readApprovedEnvironment(binding.ConfigPath, v.Environment)
		if err == nil && cache != nil {
			cache["config:"+binding.ConfigPath] = hash
		}
		return target, err
	})
	if err != nil {
		return err
	}
	config, err := actual("config:"+binding.ConfigPath, func() (string, error) {
		_, hash, err := readApprovedEnvironment(binding.ConfigPath, v.Environment)
		return hash, err
	})
	if err != nil {
		return err
	}
	graph, err := actual("graph:"+v.Graph, func() (string, error) { return d.graphFingerprint(v.Graph) })
	if err != nil {
		return err
	}
	if source != v.SourceCommit+":"+v.SourceSHA256 || config != v.ConfigSHA256 || graph != v.GraphSHA256 || target != v.Target {
		return errors.New("candidate, configuration or graph changed since run start")
	}
	for _, fixture := range []struct{ id, expected string }{{v.Account, v.AccountSHA256}, {v.Fixture, v.FixtureSHA256}} {
		hash, err := actual("fixture:"+fixture.id, func() (string, error) {
			_, hash, err := d.bindingFixture(fixture.id)
			return hash, err
		})
		if err != nil {
			return err
		}
		if hash != fixture.expected {
			return errors.New("registered account or fixture changed since run start")
		}
	}
	accountFacts, err := actual("account:"+v.Account, func() (string, error) {
		fixture, _, err := d.bindingFixture(v.Account)
		return fixture.Kind + "\x00" + fixture.Refs["role"] + "\x00" + fixture.Refs["principal"] + "\x00" + fixture.Refs["environment"], err
	})
	if err != nil {
		return err
	}
	environment, err := actual("fixture-environment:"+v.Fixture, func() (string, error) {
		fixture, _, err := d.bindingFixture(v.Fixture)
		return fixture.Refs["environment"], err
	})
	if err != nil {
		return err
	}
	if accountFacts != "account\x00"+v.Actor+"\x00"+v.Principal+"\x00"+v.Environment || environment != v.Environment {
		return errors.New("execution account or fixture belongs to another actor, principal or environment")
	}
	return nil
}

func (binding RunBinding) matchesRAW(raw StepRAW) error {
	if raw.Identity != binding.Identity || raw.Oracle != binding.Oracle {
		return errors.New("RAW candidate, target, account, actor, fixture or oracle does not match this run")
	}
	return nil
}

func bindingFixtureIDs(binding RunBinding) []string {
	if binding.Identity.Account == binding.Identity.Fixture {
		return []string{binding.Identity.Account}
	}
	return []string{binding.Identity.Account, binding.Identity.Fixture}
}
