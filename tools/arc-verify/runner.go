package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AdapterKind string

const (
	adapterHTTP    AdapterKind = "http"
	adapterCommand AdapterKind = "command"
)

type ExtractRule struct {
	Name         string `json:"name"`
	Source       string `json:"source"`
	OracleID     string `json:"oracle_id,omitempty"`
	Path         string `json:"path"`
	EntityKind   string `json:"entity_kind,omitempty"`
	EntityAlias  string `json:"entity_alias,omitempty"`
	OwnerActor   string `json:"owner_actor,omitempty"`
	SessionScope string `json:"session_scope,omitempty"`
	Required     bool   `json:"required,omitempty"`
	Redact       bool   `json:"redact,omitempty"`
}

type WaitContract struct {
	OracleID                  string              `json:"oracle_id"`
	UntilState                string              `json:"until_state,omitempty"`
	UntilConditions           ConditionAssignment `json:"until_conditions,omitempty"`
	UntilChecks               []PropertyCheck     `json:"until_checks,omitempty"`
	AllowedIntermediateStates []string            `json:"allowed_intermediate_states,omitempty"`
	MaxWaitMS                 int                 `json:"max_wait_ms"`
	PollIntervalMS            int                 `json:"poll_interval_ms"`
	StableWindowMS            int                 `json:"stable_window_ms,omitempty"`
	RuleSourceRef             string              `json:"rule_source_ref"`
}

type AdapterCall struct {
	Kind              AdapterKind       `json:"kind"`
	Method            string            `json:"method,omitempty"`
	Path              string            `json:"path,omitempty"`
	Body              json.RawMessage   `json:"body,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	HeaderRefs        map[string]string `json:"header_refs,omitempty"`
	CredentialHeader  string            `json:"credential_header,omitempty"`
	CredentialPrefix  string            `json:"credential_prefix,omitempty"`
	SessionScope      string            `json:"session_scope,omitempty"`
	SessionHeader     string            `json:"session_header,omitempty"`
	SessionPrefix     string            `json:"session_prefix,omitempty"`
	RetainSession     bool              `json:"retain_session,omitempty"`
	Extract           []ExtractRule     `json:"extract,omitempty"`
	SuccessStatuses   []int             `json:"success_statuses,omitempty"`
	RejectionStatuses []int             `json:"rejection_statuses,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Directory         string            `json:"directory,omitempty"`
	EnvRefs           map[string]string `json:"env_refs,omitempty"`
	SuccessExitCodes  []int             `json:"success_exit_codes,omitempty"`
}

type OracleAdapter struct {
	ID            string            `json:"id"`
	Boundary      string            `json:"boundary"`
	Call          AdapterCall       `json:"call"`
	StatePath     string            `json:"state_path"`
	PrincipalPath string            `json:"principal_path"`
	RolePath      string            `json:"role_path"`
	ScopePath     string            `json:"scope_path"`
	Conditions    map[string]string `json:"conditions"`
	Measurements  map[string]string `json:"measurements"`
}

type RunnerStep struct {
	InputMeasurements    map[string]string `json:"input_measurements,omitempty"`
	BusinessBranch       string            `json:"business_branch,omitempty"`
	SubjectBindings      map[string]string `json:"subject_bindings,omitempty"`
	ResponseMeasurements map[string]string `json:"response_measurements,omitempty"`
	Node                 string            `json:"node"`
	Transition           string            `json:"transition,omitempty"`
	Actor                string            `json:"actor"`
	ParallelGroup        string            `json:"parallel_group,omitempty"`
	Call                 AdapterCall       `json:"call"`
	Oracles              []OracleAdapter   `json:"oracles"`
	AsyncWait            *WaitContract     `json:"async_wait,omitempty"`
	Checks               []PropertyCheck   `json:"checks,omitempty"`
}

type RecoveryStep struct {
	Actor string      `json:"actor"`
	Call  AdapterCall `json:"call"`
}

type FixtureRecovery struct {
	Mode           string              `json:"mode,omitempty"`
	IsolationScope string              `json:"isolation_scope"`
	Steps          []RecoveryStep      `json:"steps"`
	Oracle         OracleAdapter       `json:"oracle"`
	State          string              `json:"state"`
	Conditions     ConditionAssignment `json:"conditions"`
	Checks         []PropertyCheck     `json:"checks"`
}

type RunnerManifest struct {
	SchemaVersion  int                `json:"schema_version"`
	ID             string             `json:"id"`
	Graph          string             `json:"graph"`
	Test           string             `json:"test"`
	IsolationScope string             `json:"isolation_scope"`
	TimeoutMS      int                `json:"timeout_ms"`
	StepTimeoutMS  int                `json:"step_timeout_ms"`
	Parallelism    int                `json:"parallelism"`
	InputFiles     []string           `json:"input_files,omitempty"`
	Setup          []RecoveryStep     `json:"setup,omitempty"`
	Preparation    *RunnerPreparation `json:"preparation,omitempty"`
	Steps          []RunnerStep       `json:"steps"`
	Recovery       *FixtureRecovery   `json:"recovery,omitempty"`
}

func validateAdapter(call AdapterCall, readOnly bool) error {
	if len(call.Body) > 1<<20 {
		return errors.New("adapter input exceeds 1 MiB")
	}
	if len(call.Body) > 0 {
		var input any
		if err := decodeUniqueJSON(call.Body, &input); err != nil {
			return fmt.Errorf("adapter input: %w", err)
		}
		if hasSensitiveKey(input) {
			return errors.New("adapter input contains inline credentials; use credential references")
		}
	}
	switch call.Kind {
	case adapterHTTP:
		path, err := url.Parse(call.Path)
		if err != nil || path.IsAbs() || path.Host != "" || !strings.HasPrefix(call.Path, "/") || strings.HasPrefix(call.Path, "//") || path.Fragment != "" || len(call.Args) != 0 || len(call.SuccessStatuses) == 0 {
			return errors.New("HTTP adapter requires a relative absolute path, explicit success statuses and no command argv")
		}
		switch call.Method {
		case http.MethodGet, http.MethodHead:
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if readOnly {
				return errors.New("independent HTTP oracle must use GET or HEAD")
			}
		default:
			return errors.New("unsupported adapter HTTP method")
		}
		if readOnly && len(call.RejectionStatuses) > 0 {
			return errors.New("oracle rejection is an observation failure")
		}
		for _, code := range call.SuccessStatuses {
			if code < 200 || code > 299 {
				return errors.New("HTTP success statuses must be 2xx")
			}
		}
		for _, code := range call.RejectionStatuses {
			if code < 400 || code > 499 {
				return errors.New("business rejection statuses must be 4xx")
			}
		}
	case adapterCommand:
		if len(call.Args) == 0 || len(call.Args) > 128 || call.Args[0] == "" || len(call.SuccessExitCodes) == 0 || call.Path != "" || call.Method != "" {
			return errors.New("command adapter requires argv and explicit successful exit codes")
		}
		for _, arg := range call.Args {
			if len(arg) > 65536 || strings.ContainsRune(arg, 0) {
				return errors.New("invalid command argument")
			}
		}
	default:
		return fmt.Errorf("unsupported adapter kind %q", call.Kind)
	}
	for name := range call.Headers {
		if hasSensitiveKey(map[string]any{name: ""}) {
			return fmt.Errorf("header %s looks credential-like; use header_refs or session_scope", name)
		}
	}
	for _, ref := range call.HeaderRefs {
		if !strings.HasPrefix(ref, "env://") {
			return errors.New("header credentials require env:// references")
		}
	}
	for _, ref := range call.EnvRefs {
		if !strings.HasPrefix(ref, "env://") {
			return errors.New("command environment values require env:// references")
		}
	}
	if len(call.Extract) > 32 {
		return errors.New("adapter extract rules exceed 32")
	}
	seenExtract := map[string]bool{}
	for _, ext := range call.Extract {
		if strings.TrimSpace(ext.Name) == "" || seenExtract[ext.Name] {
			return fmt.Errorf("invalid or duplicate extract name %q", ext.Name)
		}
		if hasSensitiveKey(map[string]any{ext.Name: ""}) {
			return fmt.Errorf("extract name %q contains a credential-like keyword; use a neutral alias such as session_handle and set redact:true", ext.Name)
		}
		seenExtract[ext.Name] = true
		switch ext.Source {
		case "response_body", "oracle":
			if !strings.HasPrefix(ext.Path, "/") {
				return fmt.Errorf("extract %s needs a JSON pointer path", ext.Name)
			}
			if ext.Source == "oracle" && strings.TrimSpace(ext.OracleID) == "" {
				return fmt.Errorf("extract %s from oracle requires oracle_id", ext.Name)
			}
		case "response_header", "state_cookie":
			if strings.TrimSpace(ext.Path) == "" {
				return fmt.Errorf("extract %s needs a header or cookie name", ext.Name)
			}
		default:
			return fmt.Errorf("extract %s has unsupported source %q", ext.Name, ext.Source)
		}
	}
	return nil
}

func validateOracle(oracle OracleAdapter) error {
	if oracle.ID == "request-capture" || oracle.ID == "response-capture" {
		return errors.New("oracle identity cannot impersonate a runtime request or response capture")
	}
	if oracle.ID == "" || oracle.Boundary == "" || !strings.HasPrefix(oracle.StatePath, "/") || !strings.HasPrefix(oracle.PrincipalPath, "/") || !strings.HasPrefix(oracle.RolePath, "/") || !strings.HasPrefix(oracle.ScopePath, "/") || len(oracle.Conditions) > 64 || len(oracle.Measurements) > 64 {
		return errors.New("oracle needs identity, boundary, state/principal/role/scope paths and bounded measurements")
	}
	if err := validateAdapter(oracle.Call, true); err != nil {
		return err
	}
	for _, path := range oracle.Conditions {
		if !strings.HasPrefix(path, "/") {
			return errors.New("condition observation needs a JSON pointer")
		}
	}
	for _, path := range oracle.Measurements {
		if !strings.HasPrefix(path, "/") {
			return errors.New("property measurement needs a JSON pointer")
		}
	}
	return nil
}

func validateManifest(g Graph, manifest RunnerManifest) (ModelObligation, error) {
	if manifest.SchemaVersion != 1 || manifest.ID == "" || len(manifest.ID) > 256 || manifest.Graph != g.ID || manifest.Test == "" || manifest.IsolationScope == "" || manifest.TimeoutMS < 100 || manifest.TimeoutMS > 600000 || manifest.StepTimeoutMS < 1 || manifest.StepTimeoutMS > manifest.TimeoutMS/2 || manifest.Parallelism < 1 || manifest.Parallelism > 16 || len(manifest.Steps) == 0 || len(manifest.Steps) > 128 || len(manifest.Setup) > 16 || len(manifest.InputFiles) > 64 {
		return ModelObligation{}, errors.New("runner identity, isolation or execution bounds are invalid")
	}
	var obligation *ModelObligation
	for _, item := range modelObligations(g) {
		if item.Definition.ID == manifest.Test {
			copy := item
			obligation = &copy
			break
		}
	}
	if obligation == nil || obligation.Excluded || obligation.Definition.PendingReason != "" {
		return ModelObligation{}, errors.New("runner must bind an executable frozen obligation")
	}
	if obligation.Definition.Kind == testExistence {
		return ModelObligation{}, errors.New("EXISTENCE obligations are satisfied by implementation scan evidence; a runner cannot bind them")
	}
	x := indexModel(g)
	rule := x.rules[obligation.Task.Rule]
	actors := map[string]bool{"primary": true}
	for _, actor := range obligation.Definition.Actors {
		actors[actor.Alias] = true
	}
	if obligation.Definition.ReadOnly && len(manifest.Setup) != 0 {
		return ModelObligation{}, errors.New("read-only verification cannot prepare a fixture with mutating operations")
	}
	for _, step := range manifest.Setup {
		if !actors[step.Actor] {
			return ModelObligation{}, errors.New("setup actor is not declared")
		}
		if err := validateAdapter(step.Call, obligation.Definition.ReadOnly); err != nil {
			return ModelObligation{}, err
		}
	}
	for _, step := range manifest.Steps {
		if err := validateInputMeasurements(step); err != nil {
			return ModelObligation{}, err
		}
		if err := validateResponseMeasurements(step); err != nil {
			return ModelObligation{}, err
		}
		if err := validateRunnerBusinessBindings(g, step, actors); err != nil {
			return ModelObligation{}, err
		}
		if x.kinds[step.Node] != "action" || !actors[step.Actor] || len(step.Oracles) == 0 || len(step.Oracles) > 8 {
			return ModelObligation{}, errors.New("runner step needs a declared action, participant and independent oracle")
		}
		if err := validateAdapter(step.Call, obligation.Definition.ReadOnly); err != nil {
			return ModelObligation{}, err
		}
		if step.Transition != "" {
			found := false
			for _, transition := range g.Transitions {
				if transition.Edge == step.Transition && transition.Action == step.Node {
					found = true
				}
			}
			if !found {
				return ModelObligation{}, errors.New("runner transition does not belong to its action")
			}
		}
		seen := map[string]bool{}
		for _, oracle := range step.Oracles {
			if seen[oracle.ID] {
				return ModelObligation{}, errors.New("duplicate oracle identity")
			}
			seen[oracle.ID] = true
			if err := validateOracle(oracle); err != nil {
				return ModelObligation{}, err
			}
			if oracle.Call.Kind == step.Call.Kind && oracle.Call.Path == step.Call.Path && equalArgs(oracle.Call.Args, step.Call.Args) {
				return ModelObligation{}, errors.New("action response cannot serve as the independent oracle")
			}
		}
		judged := stepRule(x, *obligation, step.Transition)
		if !seen[judged.Oracle] {
			return ModelObligation{}, errors.New("frozen rule oracle is missing")
		}
		for _, invariant := range x.invariants[judged.ID] {
			found := false
			for _, oracle := range step.Oracles {
				if oracle.ID == invariant.Oracle && oracle.Boundary == invariant.ObservationBoundary {
					found = true
					for _, check := range invariant.Checks {
						if oracle.Measurements[check.Left] == "" || check.Right != "" && oracle.Measurements[check.Right] == "" {
							return ModelObligation{}, errors.New("invariant measurement path is missing")
						}
					}
				}
			}
			if !found {
				return ModelObligation{}, fmt.Errorf("invariant %s requires its independent oracle and boundary", invariant.ID)
			}
		}
		for _, ext := range step.Call.Extract {
			if ext.Source == "oracle" && !seen[ext.OracleID] {
				return ModelObligation{}, fmt.Errorf("extract %s references unknown oracle %s", ext.Name, ext.OracleID)
			}
		}
		for _, check := range step.Checks {
			if err := validatePropertyCheck(check, step.Oracles, ""); err != nil {
				return ModelObligation{}, err
			}
		}
		if step.AsyncWait != nil {
			wait := step.AsyncWait
			if !seen[wait.OracleID] || strings.TrimSpace(wait.RuleSourceRef) == "" {
				return ModelObligation{}, errors.New("async_wait requires a step oracle_id and rule_source_ref")
			}
			if wait.MaxWaitMS < 50 || wait.MaxWaitMS > 120000 || wait.MaxWaitMS > manifest.StepTimeoutMS || wait.PollIntervalMS < 25 || wait.PollIntervalMS > 10000 || wait.StableWindowMS < 0 || wait.StableWindowMS > wait.MaxWaitMS {
				return ModelObligation{}, errors.New("async_wait polling bounds exceed step_timeout_ms or allowed limits")
			}
			if wait.UntilState != "" && x.kinds[wait.UntilState] != "state" {
				return ModelObligation{}, fmt.Errorf("async_wait until_state %s is not in model", wait.UntilState)
			}
			for _, st := range wait.AllowedIntermediateStates {
				if x.kinds[st] != "state" {
					return ModelObligation{}, fmt.Errorf("async_wait allowed_intermediate_states %s is not in model", st)
				}
			}
			if wait.UntilState == "" && len(wait.UntilConditions) == 0 && len(wait.UntilChecks) == 0 {
				return ModelObligation{}, errors.New("async_wait requires at least one convergence condition")
			}
			for _, check := range wait.UntilChecks {
				if err := validatePropertyCheck(check, step.Oracles, wait.OracleID); err != nil {
					return ModelObligation{}, err
				}
			}
		}
	}
	if err := validateRunnerPreparation(g, manifest, actors, obligation.Definition.ReadOnly); err != nil {
		return ModelObligation{}, err
	}
	if obligation.Definition.Kind == testSequence {
		batches := runnerBatches(manifest)
		if len(batches) != len(obligation.Definition.Steps) {
			return ModelObligation{}, errors.New("SEQUENCE runner must execute every declared step as one batch, in order")
		}
		for position, batch := range batches {
			if batch[len(batch)-1].Transition != obligation.Definition.Steps[position] {
				return ModelObligation{}, fmt.Errorf("SEQUENCE runner batch %d must perform declared step %s", position+1, obligation.Definition.Steps[position])
			}
		}
	}
	if manifest.Steps[len(manifest.Steps)-1].Node != rule.Action {
		return ModelObligation{}, errors.New("final runner action does not match the frozen rule")
	}
	if obligation.Transition != "" && manifest.Steps[len(manifest.Steps)-1].Transition != obligation.Transition {
		return ModelObligation{}, errors.New("declared transition must be recorded by the runner")
	}
	if obligation.Definition.Kind == testConsistency {
		final := manifest.Steps[len(manifest.Steps)-1]
		for _, layer := range obligation.Definition.Layers {
			found := false
			for _, oracle := range final.Oracles {
				found = found || oracle.ID == layer.Oracle
			}
			if !found {
				return ModelObligation{}, fmt.Errorf("CONSISTENCY runner final step must observe declared layer %s through oracle %s", layer.Layer, layer.Oracle)
			}
		}
	}
	if !obligation.Definition.ReadOnly && manifest.Recovery == nil {
		return ModelObligation{}, errors.New("mutating test needs a real fixture recovery protocol")
	}
	if manifest.Recovery != nil {
		recovery := manifest.Recovery
		mode := fixtureRecoveryMode(recovery)
		if mode != recoveryRestore && mode != recoveryCheckpoint {
			return ModelObligation{}, errors.New("recovery mode must be restore or checkpoint")
		}
		if recovery.IsolationScope != manifest.IsolationScope || recovery.State == "" || mode == recoveryRestore && !obligation.Definition.ReadOnly && len(recovery.Steps) == 0 || len(recovery.Steps) > 16 || len(recovery.Checks) == 0 {
			return ModelObligation{}, errors.New("recovery needs matching isolation, operations, target state and independent checks")
		}
		if mode == recoveryCheckpoint && (len(recovery.Steps) != 0 || x.kinds[recovery.State] != "state") {
			return ModelObligation{}, errors.New("checkpoint requires no recovery writes and an explicit declared final state")
		}
		if err := validateOracle(recovery.Oracle); err != nil {
			return ModelObligation{}, err
		}
		if mode == recoveryCheckpoint {
			for _, check := range recovery.Checks {
				if err := validatePropertyCheck(check, []OracleAdapter{recovery.Oracle}, recovery.Oracle.ID); err != nil {
					return ModelObligation{}, fmt.Errorf("checkpoint property: %w", err)
				}
			}
		}
		for _, step := range recovery.Steps {
			if !actors[step.Actor] {
				return ModelObligation{}, errors.New("recovery actor is not declared")
			}
			if err := validateAdapter(step.Call, obligation.Definition.ReadOnly); err != nil {
				return ModelObligation{}, err
			}
		}
	}
	return *obligation, nil
}

func equalArgs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func runnerCodeFingerprint(manifest RunnerManifest) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	paths := []string{executable}
	inputs := map[string]bool{}
	for _, input := range manifest.InputFiles {
		absolute, err := filepath.Abs(input)
		if err != nil {
			return "", err
		}
		inputs[absolute] = true
	}
	collect := func(call AdapterCall) error {
		if call.Kind != adapterCommand {
			return nil
		}
		path, err := exec.LookPath(call.Args[0])
		if err != nil {
			return err
		}
		paths = append(paths, path)
		for _, arg := range call.Args[1:] {
			if strings.HasPrefix(arg, "-") {
				continue
			}
			candidate := arg
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(call.Directory, candidate)
			}
			stat, err := os.Stat(candidate)
			if err != nil || !stat.Mode().IsRegular() {
				continue
			}
			if !filepath.IsAbs(arg) && !filepath.IsAbs(call.Directory) {
				return errors.New("command file inputs require an absolute path or execution directory")
			}
			absolute, err := filepath.Abs(candidate)
			if err != nil {
				return err
			}
			if !inputs[absolute] {
				return fmt.Errorf("command file input %s is missing from frozen input_files", absolute)
			}
		}
		return nil
	}
	for _, step := range manifest.Setup {
		if err := collect(step.Call); err != nil {
			return "", err
		}
	}
	for _, step := range manifest.Steps {
		if err := collect(step.Call); err != nil {
			return "", err
		}
		for _, oracle := range step.Oracles {
			if err := collect(oracle.Call); err != nil {
				return "", err
			}
		}
	}
	if manifest.Preparation != nil {
		for _, oracle := range manifest.Preparation.Observer.Oracles {
			if err := collect(oracle.Call); err != nil {
				return "", err
			}
		}
		for _, step := range manifest.Preparation.Adapters {
			if err := collect(step.Call); err != nil {
				return "", err
			}
			for _, oracle := range step.Oracles {
				if err := collect(oracle.Call); err != nil {
					return "", err
				}
			}
		}
	}
	if manifest.Recovery != nil {
		for _, step := range manifest.Recovery.Steps {
			if err := collect(step.Call); err != nil {
				return "", err
			}
		}
		if err := collect(manifest.Recovery.Oracle.Call); err != nil {
			return "", err
		}
	}
	paths = append(paths, manifest.InputFiles...)
	sort.Strings(paths)
	hashes := []string{}
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !stat.Mode().IsRegular() {
			return "", errors.New("runner input is not a regular file")
		}
		hash, err := digestFile(path)
		if err != nil {
			return "", err
		}
		hashes = append(hashes, hash)
	}
	body, err := json.Marshal(hashes)
	if err != nil {
		return "", err
	}
	return digestBytes(body), nil
}

type boundedOutput struct {
	mutex sync.Mutex
	body  bytes.Buffer
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.body.Len()+len(p) > 4<<20 {
		return 0, errors.New("adapter output exceeds 4 MiB")
	}
	return b.body.Write(p)
}
func (b *boundedOutput) Bytes() []byte {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return append([]byte(nil), b.body.Bytes()...)
}

func credential(ref string) (string, error) {
	if !strings.HasPrefix(ref, "env://") {
		return "", errors.New("credential provider is unavailable; use a configured env:// reference")
	}
	name := strings.TrimPrefix(ref, "env://")
	if name == "" || strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) >= 0 {
		return "", errors.New("invalid credential environment reference")
	}
	value, found := os.LookupEnv(name)
	if !found || value == "" {
		return "", fmt.Errorf("credential environment %s is unavailable", name)
	}
	return value, nil
}

func validatePropertyCheck(check PropertyCheck, oracles []OracleAdapter, requiredOracle string) error {
	if check.Left == "" || (check.Right == "") == (len(check.Value) == 0) {
		return errors.New("property check requires left and exactly one right/value")
	}
	switch check.Operator {
	case compareEqual, compareNotEqual, compareLess, compareLessEqual, compareGreater, compareGreaterEqual:
	default:
		return fmt.Errorf("invalid property operator %q", check.Operator)
	}
	if len(check.Value) != 0 && !json.Valid(check.Value) {
		return errors.New("property check has invalid comparison value")
	}
	checkMeasurement := func(name string) bool {
		if name == "" {
			return true
		}
		for _, oracle := range oracles {
			if requiredOracle != "" && oracle.ID != requiredOracle {
				continue
			}
			if oracle.Measurements[name] != "" {
				return true
			}
		}
		return false
	}
	if !checkMeasurement(check.Left) || !checkMeasurement(check.Right) {
		return fmt.Errorf("property check measurement %s/%s is missing from step oracle", check.Left, check.Right)
	}
	return nil
}

type AdapterOutput struct {
	Body                          []byte
	StatusCode, ExitCode          int
	Denied                        bool
	RequestSHA256, ResponseSHA256 string
	TemplateSHA256                string
	ResolvedPath                  string
	ResolvedInput                 json.RawMessage
	Substitutions                 map[string]string
	Started, Completed            string
	ResponseHeaders               http.Header
	ResponseCookies               map[string]string
}

type execSession struct {
	cookieMu     sync.Mutex
	vars         map[string]ExtractedBinding
	redactedVals map[string]string
	cookies      map[string]http.CookieJar
	entities     map[string]EntityBinding
	sessions     map[string]SessionBinding
	actors       map[string]ParticipantBinding
	runID        string
	nonce        string
}

func newExecSession(runID string, participants []ParticipantBinding) *execSession {
	actors := make(map[string]ParticipantBinding, len(participants))
	for _, p := range participants {
		actors[p.Alias] = p
	}
	return &execSession{
		vars:         map[string]ExtractedBinding{},
		redactedVals: map[string]string{},
		cookies:      map[string]http.CookieJar{},
		entities:     map[string]EntityBinding{},
		sessions:     map[string]SessionBinding{},
		actors:       actors,
		runID:        runID,
		nonce:        digestBytes([]byte(runID))[:12],
	}
}

func (s *execSession) cookieJar(scope string) http.CookieJar {
	if s == nil {
		return nil
	}
	if scope == "" {
		scope = "primary"
	}
	s.cookieMu.Lock()
	defer s.cookieMu.Unlock()
	jar, ok := s.cookies[scope]
	if !ok {
		jar, _ = cookiejar.New(nil)
		s.cookies[scope] = jar
	}
	return jar
}

func (s *execSession) entitySlice() []EntityBinding {
	if s == nil || len(s.entities) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.entities))
	for k := range s.entities {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]EntityBinding, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.entities[k])
	}
	return out
}

func (s *execSession) sessionSlice() []SessionBinding {
	if s == nil || len(s.sessions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.sessions))
	for k := range s.sessions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SessionBinding, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.sessions[k])
	}
	return out
}

var templatePattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_.-]+)\}\}`)

func (s *execSession) resolvePlaceholder(key string) (actual string, redacted string, isRedacted bool, err error) {
	if s == nil {
		return "", "", false, fmt.Errorf("template placeholder {{%s}} requires active execution session", key)
	}
	switch {
	case key == "run.id":
		return s.runID, s.runID, false, nil
	case key == "run.nonce":
		return s.nonce, s.nonce, false, nil
	case strings.HasPrefix(key, "var."):
		name := strings.TrimPrefix(key, "var.")
		binding, found := s.vars[name]
		if !found {
			return "", "", false, fmt.Errorf("unbound variable placeholder {{%s}}", key)
		}
		if binding.Redacted {
			val := s.redactedVals["var:"+name]
			if val == "" {
				return "", "", true, fmt.Errorf("redacted variable {{%s}} is unavailable in memory", key)
			}
			return val, "[REDACTED:" + binding.SHA256[:12] + "]", true, nil
		}
		return binding.Value, binding.Value, false, nil
	case strings.HasPrefix(key, "actor."):
		parts := strings.Split(strings.TrimPrefix(key, "actor."), ".")
		if len(parts) != 2 {
			return "", "", false, fmt.Errorf("invalid actor placeholder {{%s}}", key)
		}
		actor, found := s.actors[parts[0]]
		if !found {
			return "", "", false, fmt.Errorf("unknown participant alias in {{%s}}", key)
		}
		switch parts[1] {
		case "principal":
			return actor.Principal, actor.Principal, false, nil
		case "account":
			return actor.Account, actor.Account, false, nil
		case "role":
			return actor.Role, actor.Role, false, nil
		default:
			return "", "", false, fmt.Errorf("unsupported actor attribute in {{%s}}", key)
		}
	default:
		return "", "", false, fmt.Errorf("unknown template placeholder {{%s}}", key)
	}
}

func jsonStringEscape(s string) string {
	encoded, err := json.Marshal(s)
	if err != nil || len(encoded) < 2 {
		return s
	}
	return string(encoded[1 : len(encoded)-1])
}

func renderTemplateString(input string, sess *execSession, jsonEscape bool, subs map[string]string) (string, string, error) {
	if !strings.Contains(input, "{{") {
		return input, input, nil
	}
	matches := templatePattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("malformed template placeholder in %q", input)
	}
	var actualBuf, redactedBuf strings.Builder
	last := 0
	for _, m := range matches {
		actualBuf.WriteString(input[last:m[0]])
		redactedBuf.WriteString(input[last:m[0]])
		key := input[m[2]:m[3]]
		actualVal, redactedVal, _, err := sess.resolvePlaceholder(key)
		if err != nil {
			return "", "", err
		}
		if subs != nil {
			subs[key] = redactedVal
		}
		if jsonEscape {
			actualVal = jsonStringEscape(actualVal)
			redactedVal = jsonStringEscape(redactedVal)
		}
		actualBuf.WriteString(actualVal)
		redactedBuf.WriteString(redactedVal)
		last = m[1]
	}
	actualBuf.WriteString(input[last:])
	redactedBuf.WriteString(input[last:])
	if strings.Contains(actualBuf.String(), "{{") {
		return "", "", errors.New("unresolved or malformed template placeholder remains after substitution")
	}
	return actualBuf.String(), redactedBuf.String(), nil
}

func applySubstitutionsToCall(call AdapterCall, subs map[string]string) (AdapterCall, error) {
	rendered := call
	replaceStr := func(s string, jsonEscape bool) (string, error) {
		if !strings.Contains(s, "{{") {
			return s, nil
		}
		matches := templatePattern.FindAllStringSubmatchIndex(s, -1)
		if len(matches) == 0 {
			return "", fmt.Errorf("malformed template placeholder in %q", s)
		}
		var b strings.Builder
		last := 0
		for _, m := range matches {
			b.WriteString(s[last:m[0]])
			key := s[m[2]:m[3]]
			val, ok := subs[key]
			if !ok {
				return "", fmt.Errorf("missing recorded substitution for {{%s}}", key)
			}
			if jsonEscape {
				val = jsonStringEscape(val)
			}
			b.WriteString(val)
			last = m[1]
		}
		b.WriteString(s[last:])
		if strings.Contains(b.String(), "{{") {
			return "", errors.New("unresolved placeholder remains in call")
		}
		return b.String(), nil
	}
	var err error
	if rendered.Path, err = replaceStr(call.Path, false); err != nil {
		return rendered, err
	}
	if len(call.Body) > 0 {
		bodyStr, err := replaceStr(string(call.Body), true)
		if err != nil {
			return rendered, err
		}
		if !json.Valid([]byte(bodyStr)) {
			return rendered, errors.New("substituted JSON body is invalid")
		}
		rendered.Body = json.RawMessage(bodyStr)
	}
	if len(call.Headers) > 0 {
		rendered.Headers = make(map[string]string, len(call.Headers))
		for k, v := range call.Headers {
			rv, err := replaceStr(v, false)
			if err != nil {
				return rendered, err
			}
			rendered.Headers[k] = rv
		}
	}
	if len(call.Args) > 0 {
		rendered.Args = make([]string, len(call.Args))
		for i, arg := range call.Args {
			ra, err := replaceStr(arg, false)
			if err != nil {
				return rendered, err
			}
			rendered.Args[i] = ra
		}
	}
	return rendered, nil
}

func renderCallWithSession(call AdapterCall, sess *execSession) (actual AdapterCall, redacted AdapterCall, templateSHA string, subs map[string]string, err error) {
	tmplBytes, err := json.Marshal(call)
	if err != nil {
		return call, call, "", nil, err
	}
	templateSHA = digestBytes(tmplBytes)
	actual = call
	redacted = call
	subs = map[string]string{}
	actual.Path, redacted.Path, err = renderTemplateString(call.Path, sess, false, subs)
	if err != nil {
		return call, call, templateSHA, nil, err
	}
	if len(call.Body) > 0 {
		aBody, rBody, err := renderTemplateString(string(call.Body), sess, true, subs)
		if err != nil {
			return call, call, templateSHA, nil, err
		}
		if !json.Valid([]byte(aBody)) || !json.Valid([]byte(rBody)) {
			return call, call, templateSHA, nil, errors.New("rendered adapter JSON body is invalid")
		}
		actual.Body = json.RawMessage(aBody)
		redacted.Body = json.RawMessage(rBody)
	}
	if len(call.Headers) > 0 {
		actual.Headers = make(map[string]string, len(call.Headers))
		redacted.Headers = make(map[string]string, len(call.Headers))
		for k, v := range call.Headers {
			av, rv, err := renderTemplateString(v, sess, false, subs)
			if err != nil {
				return call, call, templateSHA, nil, err
			}
			actual.Headers[k] = av
			redacted.Headers[k] = rv
		}
	}
	if len(call.Args) > 0 {
		actual.Args = make([]string, len(call.Args))
		redacted.Args = make([]string, len(call.Args))
		for i, arg := range call.Args {
			aa, ra, err := renderTemplateString(arg, sess, false, subs)
			if err != nil {
				return call, call, templateSHA, nil, err
			}
			actual.Args[i] = aa
			redacted.Args[i] = ra
		}
	}
	if len(subs) == 0 {
		subs = nil
	}
	return actual, redacted, templateSHA, subs, nil
}

func performCall(ctx context.Context, call AdapterCall, binding RunBinding, account Fixture) (output AdapterOutput, err error) {
	return performCallWithSession(ctx, call, binding, account, "", nil)
}

func performCallWithSession(ctx context.Context, call AdapterCall, binding RunBinding, account Fixture, actor string, sess *execSession) (output AdapterOutput, err error) {
	return performCallWithRequestCapture(ctx, call, binding, account, actor, sess, nil)
}

func performCallWithRequestCapture(ctx context.Context, call AdapterCall, binding RunBinding, account Fixture, actor string, sess *execSession, captured *RequestObservation) (output AdapterOutput, err error) {
	output = AdapterOutput{Started: time.Now().UTC().Format(time.RFC3339Nano)}
	defer func() { output.Completed = time.Now().UTC().Format(time.RFC3339Nano) }()
	actualCall, redactedCall, templateSHA, subs, err := renderCallWithSession(call, sess)
	if err != nil {
		return output, err
	}
	redactedBytes, err := json.Marshal(redactedCall)
	if err != nil {
		return output, err
	}
	output.TemplateSHA256 = templateSHA
	output.RequestSHA256 = digestBytes(redactedBytes)
	output.ResolvedPath = redactedCall.Path
	output.ResolvedInput = redactedCall.Body
	output.Substitutions = subs
	if captured != nil && (captured.Actor != actor || captured.TemplateSHA256 != output.TemplateSHA256 || captured.RequestSHA256 != output.RequestSHA256 || captured.ResolvedPath != output.ResolvedPath || !sameJSONValue(captured.ResolvedInput, output.ResolvedInput)) {
		return output, errors.New("resolved request changed after the business expectation was frozen; request was not sent")
	}
	switch actualCall.Kind {
	case adapterHTTP:
		base, err := url.Parse(binding.Identity.Target)
		if err != nil {
			return output, err
		}
		ref, err := url.Parse(actualCall.Path)
		if err != nil {
			return output, err
		}
		encodedPath := strings.TrimRight(base.EscapedPath(), "/") + ref.EscapedPath()
		base.Path = strings.TrimRight(base.Path, "/") + ref.Path
		base.RawPath = encodedPath
		base.RawQuery = ref.RawQuery
		request, err := http.NewRequestWithContext(ctx, actualCall.Method, base.String(), bytes.NewReader(actualCall.Body))
		if err != nil {
			return output, err
		}
		if len(actualCall.Body) > 0 {
			request.Header.Set("Content-Type", "application/json")
		}
		for header, value := range actualCall.Headers {
			request.Header.Set(header, value)
		}
		for header, ref := range actualCall.HeaderRefs {
			value, err := credential(ref)
			if err != nil {
				return output, err
			}
			request.Header.Set(header, value)
		}
		if actualCall.CredentialHeader != "" {
			if account.SecretRef == "" {
				return output, errors.New("declared account credential is unavailable")
			}
			value, err := credential(account.SecretRef)
			if err != nil {
				return output, err
			}
			request.Header.Set(actualCall.CredentialHeader, actualCall.CredentialPrefix+value)
		}
		if actualCall.SessionScope != "" && sess != nil {
			if secretVal, ok := sess.redactedVals["session:"+actualCall.SessionScope]; ok && secretVal != "" {
				headerName := firstNonEmpty(actualCall.SessionHeader, "Authorization")
				prefix := actualCall.SessionPrefix
				if actualCall.SessionHeader == "" && prefix == "" {
					prefix = "Bearer "
				}
				request.Header.Set(headerName, prefix+secretVal)
			} else if !actualCall.RetainSession {
				return output, fmt.Errorf("session scope %s has no active session credential", actualCall.SessionScope)
			}
		}
		client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if (actualCall.RetainSession || actualCall.SessionScope != "") && sess != nil {
			client.Jar = sess.cookieJar(firstNonEmpty(actualCall.SessionScope, actor))
		}
		response, err := client.Do(request)
		if err != nil {
			return output, fmt.Errorf("HTTP adapter transport: %w", err)
		}
		defer response.Body.Close()
		output.StatusCode = response.StatusCode
		output.ResponseHeaders = response.Header.Clone()
		if len(response.Cookies()) > 0 {
			output.ResponseCookies = map[string]string{}
			for _, ck := range response.Cookies() {
				output.ResponseCookies[ck.Name] = ck.Value
			}
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
		if err != nil {
			return output, err
		}
		if len(body) > 4<<20 {
			return output, errors.New("HTTP adapter response exceeds 4 MiB")
		}
		output.Body = body
		output.ResponseSHA256 = digestBytes(body)
		accepted := false
		for _, status := range actualCall.SuccessStatuses {
			if status == response.StatusCode {
				accepted = true
			}
		}
		for _, status := range actualCall.RejectionStatuses {
			if status == response.StatusCode {
				accepted = true
				output.Denied = true
			}
		}
		if !accepted {
			return output, fmt.Errorf("HTTP adapter status %d is outside the declared contract", response.StatusCode)
		}
	case adapterCommand:
		command := exec.CommandContext(ctx, actualCall.Args[0], actualCall.Args[1:]...)
		command.Dir = actualCall.Directory
		if command.Dir == "" {
			command.Dir = binding.SourceRoot
		} else if !filepath.IsAbs(command.Dir) {
			command.Dir = filepath.Join(binding.SourceRoot, command.Dir)
		}
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "ARC_VERIFY_TARGET=" + binding.Identity.Target, "ARC_VERIFY_PRINCIPAL=" + account.Refs["principal"], "ARC_VERIFY_FIXTURE=" + binding.Identity.Fixture}
		for name, ref := range actualCall.EnvRefs {
			value, err := credential(ref)
			if err != nil {
				return output, err
			}
			command.Env = append(command.Env, name+"="+value)
		}
		var stdout, stderr boundedOutput
		command.Stdout = &stdout
		command.Stderr = &stderr
		if err := configureRunnerProcess(command); err != nil {
			return output, err
		}
		runErr := command.Run()
		output.Body = stdout.Bytes()
		output.ResponseSHA256 = digestBytes(output.Body)
		if stopErr := terminateRunnerProcess(command); stopErr != nil {
			return output, stopErr
		}
		output.ExitCode = -1
		if command.ProcessState != nil {
			output.ExitCode = command.ProcessState.ExitCode()
		}
		accepted := false
		for _, code := range actualCall.SuccessExitCodes {
			if code == output.ExitCode {
				accepted = true
			}
		}
		if ctx.Err() != nil {
			return output, fmt.Errorf("command adapter interrupted: %w", ctx.Err())
		}
		if runErr != nil {
			var exitError *exec.ExitError
			if !errors.As(runErr, &exitError) || !accepted {
				return output, fmt.Errorf("command adapter execution/capture (exit %d): %w", output.ExitCode, runErr)
			}
		}
		if !accepted {
			return output, fmt.Errorf("command adapter exit %d is outside the declared contract", output.ExitCode)
		}
	default:
		return output, errors.New("unsupported adapter")
	}
	output.ResponseSHA256 = digestBytes(output.Body)
	output.Completed = time.Now().UTC().Format(time.RFC3339Nano)
	return output, nil
}

func jsonPointer(raw json.RawMessage, pointer string) (json.RawMessage, error) {
	if pointer == "" {
		return raw, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("invalid JSON pointer")
	}
	parts := strings.Split(pointer[1:], "/")
	if len(parts) > 32 {
		return nil, errors.New("JSON pointer depth exceeds 32")
	}
	for _, part := range parts {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			return nil, errors.New("JSON pointer selected no value")
		}
		switch trimmed[0] {
		case '{':
			var object map[string]json.RawMessage
			if err := decodeUniqueJSON(raw, &object); err != nil {
				return nil, err
			}
			value, found := object[part]
			if !found {
				return nil, fmt.Errorf("oracle value %s is missing", pointer)
			}
			raw = value
		case '[':
			var array []json.RawMessage
			if err := json.Unmarshal(raw, &array); err != nil {
				return nil, err
			}
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(array) {
				return nil, errors.New("oracle array index is invalid")
			}
			raw = array[index]
		default:
			return nil, errors.New("JSON pointer cannot descend through a scalar")
		}
	}
	return raw, nil
}

func readOracle(ctx context.Context, oracle OracleAdapter, binding RunBinding, account Fixture, scope string) (IndependentFacts, string, error) {
	facts, hash, _, err := readOracleWithSession(ctx, oracle, binding, account, "", scope, nil)
	return facts, hash, err
}

func readOracleWithSession(ctx context.Context, oracle OracleAdapter, binding RunBinding, account Fixture, actor, scope string, sess *execSession) (IndependentFacts, string, json.RawMessage, error) {
	facts := IndependentFacts{Conditions: ObservedConditions{}, Values: map[string]json.RawMessage{}}
	output, err := performCallWithSession(ctx, oracle.Call, binding, account, actor, sess)
	if err != nil {
		return facts, "", nil, err
	}
	var document json.RawMessage
	if err := decodeUniqueJSON(output.Body, &document); err != nil {
		return facts, "", nil, fmt.Errorf("oracle response: %w", err)
	}
	state, err := jsonPointer(document, oracle.StatePath)
	if err != nil {
		return facts, "", document, err
	}
	if err = json.Unmarshal(state, &facts.State); err != nil || facts.State == "" {
		return facts, "", document, errors.New("oracle state must be a nonempty string")
	}
	principal, err := jsonPointer(document, oracle.PrincipalPath)
	if err != nil {
		return facts, "", document, err
	}
	if err = json.Unmarshal(principal, &facts.Principal); err != nil || facts.Principal != account.Refs["principal"] {
		return facts, "", document, errors.New("independent oracle principal differs from the registered execution account")
	}
	role, err := jsonPointer(document, oracle.RolePath)
	if err != nil {
		return facts, "", document, err
	}
	if err = json.Unmarshal(role, &facts.Role); err != nil || facts.Role != account.Refs["role"] {
		return facts, "", document, errors.New("independent oracle role differs from the registered execution account")
	}
	observedScope, err := jsonPointer(document, oracle.ScopePath)
	if err != nil {
		return facts, "", document, err
	}
	if err = json.Unmarshal(observedScope, &facts.Scope); err != nil || facts.Scope != scope {
		return facts, "", document, errors.New("independent oracle fixture scope differs from the registered isolated resource")
	}
	for condition, path := range oracle.Conditions {
		raw, err := jsonPointer(document, path)
		if err != nil {
			return facts, "", document, err
		}
		var value *bool
		if err = json.Unmarshal(raw, &value); err != nil || value == nil {
			return facts, "", document, errors.New("oracle condition must be an observed boolean")
		}
		facts.Conditions[condition] = *value
	}
	for name, path := range oracle.Measurements {
		raw, err := jsonPointer(document, path)
		if err != nil {
			return facts, "", document, err
		}
		if len(raw) > 65536 {
			return facts, "", document, errors.New("oracle measurement exceeds 64 KiB")
		}
		facts.Values[name] = append(json.RawMessage(nil), raw...)
	}
	body, err := json.Marshal(facts)
	if err != nil {
		return facts, "", document, err
	}
	if len(body) > 512<<10 {
		return facts, "", document, errors.New("normalized oracle facts exceed 512 KiB")
	}
	var value any
	if err := decodeUniqueJSON(body, &value); err != nil {
		return facts, "", document, err
	}
	if hasSensitiveKey(value) {
		return facts, "", document, errors.New("oracle facts contain credential-like fields")
	}
	return facts, output.ResponseSHA256, document, nil
}

func scalarJSONString(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return "", errors.New("extracted value is empty or null")
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return "", err
		}
		if s == "" {
			return "", errors.New("extracted string is empty")
		}
		return s, nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return "", errors.New("extracted value must be scalar")
	}
	return string(trimmed), nil
}

// An oracle ID identifies the reader, not the account or operation being read.
// Every batch-local lookup must retain both of those execution coordinates.
type oracleIdentity struct {
	ActionIndex int
	Actor       string
	ID          string
}

func applyExtractions(rules []ExtractRule, stepIdx, actionIndex int, defaultActor string, output *AdapterOutput, oracleDocs map[oracleIdentity]json.RawMessage, sess *execSession) ([]ExtractedBinding, error) {
	if len(rules) == 0 || sess == nil {
		return nil, nil
	}
	out := make([]ExtractedBinding, 0, len(rules))
	var responseDoc json.RawMessage
	responseParsed := false
	for _, ext := range rules {
		var rawValue string
		var extractErr error
		switch ext.Source {
		case "response_body":
			if output == nil {
				extractErr = errors.New("no action response body for extraction")
				break
			}
			if !responseParsed {
				if err := decodeUniqueJSON(output.Body, &responseDoc); err != nil {
					extractErr = fmt.Errorf("extract %s response JSON: %w", ext.Name, err)
					break
				}
				responseParsed = true
			}
			sel, err := jsonPointer(responseDoc, ext.Path)
			if err != nil {
				extractErr = err
				break
			}
			rawValue, extractErr = scalarJSONString(sel)
		case "response_header":
			if output == nil || output.ResponseHeaders == nil {
				extractErr = errors.New("no response headers for extraction")
				break
			}
			rawValue = strings.TrimSpace(output.ResponseHeaders.Get(ext.Path))
			if rawValue == "" {
				extractErr = fmt.Errorf("header %s is missing", ext.Path)
			}
		case "state_cookie":
			if output == nil || output.ResponseCookies == nil {
				extractErr = errors.New("no response cookies for extraction")
				break
			}
			rawValue = strings.TrimSpace(output.ResponseCookies[ext.Path])
			if rawValue == "" {
				extractErr = fmt.Errorf("cookie %s is missing", ext.Path)
			}
		case "oracle":
			doc, ok := oracleDocs[oracleIdentity{ActionIndex: actionIndex, Actor: defaultActor, ID: ext.OracleID}]
			if !ok {
				extractErr = fmt.Errorf("oracle %s response unavailable for extraction", ext.OracleID)
				break
			}
			sel, err := jsonPointer(doc, ext.Path)
			if err != nil {
				extractErr = err
				break
			}
			rawValue, extractErr = scalarJSONString(sel)
		default:
			extractErr = fmt.Errorf("unsupported extract source %q", ext.Source)
		}
		if extractErr != nil {
			if ext.Required {
				return out, fmt.Errorf("extract %s: %w", ext.Name, extractErr)
			}
			continue
		}
		digest := digestBytes([]byte(rawValue))
		ownerActor := firstNonEmpty(ext.OwnerActor, defaultActor)
		ownerPrincipal := sess.actors[ownerActor].Principal
		item := ExtractedBinding{
			Name:           ext.Name,
			Source:         ext.Source,
			Path:           ext.Path,
			SHA256:         digest,
			Redacted:       ext.Redact,
			EntityKind:     ext.EntityKind,
			EntityAlias:    firstNonEmpty(ext.EntityAlias, ext.Name),
			OwnerActor:     ownerActor,
			OwnerPrincipal: ownerPrincipal,
			Step:           stepIdx,
			ActionIndex:    actionIndex,
			SourceActor:    defaultActor,
			OracleID:       ext.OracleID,
		}
		if ext.Redact {
			sess.redactedVals["var:"+ext.Name] = rawValue
			if ext.SessionScope != "" {
				sess.redactedVals["session:"+ext.SessionScope] = rawValue
				sess.sessions[ext.SessionScope] = SessionBinding{
					Scope:        ext.SessionScope,
					Actor:        ownerActor,
					Principal:    ownerPrincipal,
					AcquiredStep: stepIdx,
					Digest:       digest,
					Active:       true,
				}
			}
		} else {
			item.Value = rawValue
			if ext.SessionScope != "" {
				sess.redactedVals["session:"+ext.SessionScope] = rawValue
				sess.sessions[ext.SessionScope] = SessionBinding{
					Scope:        ext.SessionScope,
					Actor:        ownerActor,
					Principal:    ownerPrincipal,
					AcquiredStep: stepIdx,
					Digest:       digest,
					Active:       true,
				}
			}
			if ext.EntityKind != "" {
				alias := firstNonEmpty(ext.EntityAlias, ext.Name)
				sess.entities[alias] = EntityBinding{
					Alias:          alias,
					Kind:           ext.EntityKind,
					ID:             rawValue,
					OwnerActor:     ownerActor,
					OwnerPrincipal: ownerPrincipal,
					SourceStep:     stepIdx,
					SourceField:    ext.Source + ":" + ext.Path,
				}
			}
		}
		sess.vars[ext.Name] = item
		out = append(out, item)
	}
	return out, nil
}
