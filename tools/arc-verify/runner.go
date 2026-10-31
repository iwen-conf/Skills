package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

type AdapterCall struct {
	Kind              AdapterKind       `json:"kind"`
	Method            string            `json:"method,omitempty"`
	Path              string            `json:"path,omitempty"`
	Body              json.RawMessage   `json:"body,omitempty"`
	HeaderRefs        map[string]string `json:"header_refs,omitempty"`
	CredentialHeader  string            `json:"credential_header,omitempty"`
	CredentialPrefix  string            `json:"credential_prefix,omitempty"`
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
	Node          string          `json:"node"`
	Transition    string          `json:"transition,omitempty"`
	Actor         string          `json:"actor"`
	ParallelGroup string          `json:"parallel_group,omitempty"`
	Call          AdapterCall     `json:"call"`
	Oracles       []OracleAdapter `json:"oracles"`
}

type RecoveryStep struct {
	Actor string      `json:"actor"`
	Call  AdapterCall `json:"call"`
}

type FixtureRecovery struct {
	IsolationScope string              `json:"isolation_scope"`
	Steps          []RecoveryStep      `json:"steps"`
	Oracle         OracleAdapter       `json:"oracle"`
	State          string              `json:"state"`
	Conditions     ConditionAssignment `json:"conditions"`
	Checks         []PropertyCheck     `json:"checks"`
}

type RunnerManifest struct {
	SchemaVersion  int              `json:"schema_version"`
	ID             string           `json:"id"`
	Graph          string           `json:"graph"`
	Test           string           `json:"test"`
	IsolationScope string           `json:"isolation_scope"`
	TimeoutMS      int              `json:"timeout_ms"`
	StepTimeoutMS  int              `json:"step_timeout_ms"`
	Parallelism    int              `json:"parallelism"`
	InputFiles     []string         `json:"input_files,omitempty"`
	Setup          []RecoveryStep   `json:"setup,omitempty"`
	Steps          []RunnerStep     `json:"steps"`
	Recovery       *FixtureRecovery `json:"recovery,omitempty"`
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
	return nil
}

func validateOracle(oracle OracleAdapter) error {
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
		if recovery.IsolationScope != manifest.IsolationScope || recovery.State == "" || !obligation.Definition.ReadOnly && len(recovery.Steps) == 0 || len(recovery.Steps) > 16 || len(recovery.Checks) == 0 {
			return ModelObligation{}, errors.New("recovery needs matching isolation, operations, target state and independent checks")
		}
		if err := validateOracle(recovery.Oracle); err != nil {
			return ModelObligation{}, err
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

type AdapterOutput struct {
	Body                          []byte
	StatusCode, ExitCode          int
	Denied                        bool
	RequestSHA256, ResponseSHA256 string
	Started, Completed            string
}

func performCall(ctx context.Context, call AdapterCall, binding RunBinding, account Fixture) (output AdapterOutput, err error) {
	output = AdapterOutput{Started: time.Now().UTC().Format(time.RFC3339Nano)}
	defer func() { output.Completed = time.Now().UTC().Format(time.RFC3339Nano) }()
	input, err := json.Marshal(call)
	if err != nil {
		return output, err
	}
	output.RequestSHA256 = digestBytes(input)
	switch call.Kind {
	case adapterHTTP:
		base, err := url.Parse(binding.Identity.Target)
		if err != nil {
			return output, err
		}
		ref, err := url.Parse(call.Path)
		if err != nil {
			return output, err
		}
		encodedPath := strings.TrimRight(base.EscapedPath(), "/") + ref.EscapedPath()
		base.Path = strings.TrimRight(base.Path, "/") + ref.Path
		base.RawPath = encodedPath
		base.RawQuery = ref.RawQuery
		request, err := http.NewRequestWithContext(ctx, call.Method, base.String(), bytes.NewReader(call.Body))
		if err != nil {
			return output, err
		}
		if len(call.Body) > 0 {
			request.Header.Set("Content-Type", "application/json")
		}
		for header, ref := range call.HeaderRefs {
			value, err := credential(ref)
			if err != nil {
				return output, err
			}
			request.Header.Set(header, value)
		}
		if call.CredentialHeader != "" {
			if account.SecretRef == "" {
				return output, errors.New("declared account credential is unavailable")
			}
			value, err := credential(account.SecretRef)
			if err != nil {
				return output, err
			}
			request.Header.Set(call.CredentialHeader, call.CredentialPrefix+value)
		}
		client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(request)
		if err != nil {
			return output, fmt.Errorf("HTTP adapter transport: %w", err)
		}
		defer response.Body.Close()
		output.StatusCode = response.StatusCode
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
		for _, status := range call.SuccessStatuses {
			if status == response.StatusCode {
				accepted = true
			}
		}
		for _, status := range call.RejectionStatuses {
			if status == response.StatusCode {
				accepted = true
				output.Denied = true
			}
		}
		if !accepted {
			return output, fmt.Errorf("HTTP adapter status %d is outside the declared contract", response.StatusCode)
		}
	case adapterCommand:
		command := exec.CommandContext(ctx, call.Args[0], call.Args[1:]...)
		command.Dir = call.Directory
		if command.Dir == "" {
			command.Dir = binding.SourceRoot
		} else if !filepath.IsAbs(command.Dir) {
			command.Dir = filepath.Join(binding.SourceRoot, command.Dir)
		}
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "ARC_VERIFY_TARGET=" + binding.Identity.Target, "ARC_VERIFY_PRINCIPAL=" + account.Refs["principal"], "ARC_VERIFY_FIXTURE=" + binding.Identity.Fixture}
		for name, ref := range call.EnvRefs {
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
		for _, code := range call.SuccessExitCodes {
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
	facts := IndependentFacts{Conditions: ObservedConditions{}, Values: map[string]json.RawMessage{}}
	output, err := performCall(ctx, oracle.Call, binding, account)
	if err != nil {
		return facts, "", err
	}
	var document json.RawMessage
	if err := decodeUniqueJSON(output.Body, &document); err != nil {
		return facts, "", fmt.Errorf("oracle response: %w", err)
	}
	state, err := jsonPointer(document, oracle.StatePath)
	if err != nil {
		return facts, "", err
	}
	if err = json.Unmarshal(state, &facts.State); err != nil || facts.State == "" {
		return facts, "", errors.New("oracle state must be a nonempty string")
	}
	principal, err := jsonPointer(document, oracle.PrincipalPath)
	if err != nil {
		return facts, "", err
	}
	if err = json.Unmarshal(principal, &facts.Principal); err != nil || facts.Principal != account.Refs["principal"] {
		return facts, "", errors.New("independent oracle principal differs from the registered execution account")
	}
	role, err := jsonPointer(document, oracle.RolePath)
	if err != nil {
		return facts, "", err
	}
	if err = json.Unmarshal(role, &facts.Role); err != nil || facts.Role != account.Refs["role"] {
		return facts, "", errors.New("independent oracle role differs from the registered execution account")
	}
	observedScope, err := jsonPointer(document, oracle.ScopePath)
	if err != nil {
		return facts, "", err
	}
	if err = json.Unmarshal(observedScope, &facts.Scope); err != nil || facts.Scope != scope {
		return facts, "", errors.New("independent oracle fixture scope differs from the registered isolated resource")
	}
	for condition, path := range oracle.Conditions {
		raw, err := jsonPointer(document, path)
		if err != nil {
			return facts, "", err
		}
		var value *bool
		if err = json.Unmarshal(raw, &value); err != nil || value == nil {
			return facts, "", errors.New("oracle condition must be an observed boolean")
		}
		facts.Conditions[condition] = *value
	}
	for name, path := range oracle.Measurements {
		raw, err := jsonPointer(document, path)
		if err != nil {
			return facts, "", err
		}
		if len(raw) > 65536 {
			return facts, "", errors.New("oracle measurement exceeds 64 KiB")
		}
		facts.Values[name] = append(json.RawMessage(nil), raw...)
	}
	body, err := json.Marshal(facts)
	if err != nil {
		return facts, "", err
	}
	if len(body) > 512<<10 {
		return facts, "", errors.New("normalized oracle facts exceed 512 KiB")
	}
	var value any
	if err := decodeUniqueJSON(body, &value); err != nil {
		return facts, "", err
	}
	if hasSensitiveKey(value) {
		return facts, "", errors.New("oracle facts contain credential-like fields")
	}
	return facts, output.ResponseSHA256, nil
}
