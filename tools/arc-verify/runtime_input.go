package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RequestObservation records the rendered request before a branch is frozen.
// Credentials retain the existing redaction rules. Selected business values
// must be non-secret and reproducible from this exact recorded request.
type RequestObservation struct {
	Actor          string                     `json:"actor"`
	ActionIndex    int                        `json:"action_index"`
	Node           string                     `json:"node"`
	TemplateSHA256 string                     `json:"template_sha256"`
	RequestSHA256  string                     `json:"request_sha256"`
	ResolvedPath   string                     `json:"resolved_path,omitempty"`
	ResolvedInput  json.RawMessage            `json:"resolved_input,omitempty"`
	Substitutions  map[string]string          `json:"substitutions,omitempty"`
	Values         map[string]json.RawMessage `json:"values"`
}

func validateInputMeasurements(step RunnerStep) error {
	if len(step.InputMeasurements) > 64 {
		return fmt.Errorf("input measurements exceed 64")
	}
	for key, path := range step.InputMeasurements {
		parts := strings.Split(key, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.HasPrefix(key, "transport.") || !strings.HasPrefix(path, "/") || hasSensitiveKey(map[string]any{key: nil, path: nil}) {
			return fmt.Errorf("invalid or sensitive input measurement %s", key)
		}
		if _, exists := step.ResponseMeasurements[key]; exists {
			return fmt.Errorf("input and response measurements share %s", key)
		}
	}
	return nil
}

func captureInputValues(body json.RawMessage, measurements map[string]string) (map[string]json.RawMessage, error) {
	if len(measurements) == 0 {
		return nil, nil
	}
	var doc json.RawMessage
	if err := decodeUniqueJSON(body, &doc); err != nil {
		return nil, fmt.Errorf("request measurement JSON: %w", err)
	}
	values := make(map[string]json.RawMessage, len(measurements))
	for key, path := range measurements {
		value, err := jsonPointer(doc, path)
		if err != nil || len(value) > 65536 {
			return nil, fmt.Errorf("request measurement %s is missing or exceeds 64 KiB", key)
		}
		var parsed any
		if err := decodeUniqueJSON(value, &parsed); err != nil || hasSensitiveKey(parsed) || strings.Contains(string(value), "[REDACTED:") {
			return nil, fmt.Errorf("request measurement %s is invalid or sensitive", key)
		}
		values[key] = append(json.RawMessage(nil), value...)
	}
	return values, nil
}

func captureRuntimeRequest(step RunnerStep, actionIndex int, sess *execSession) (RequestObservation, error) {
	request := RequestObservation{Actor: step.Actor, ActionIndex: actionIndex, Node: step.Node}
	actual, redacted, templateSHA, subs, err := renderCallWithSession(step.Call, sess)
	if err != nil {
		return request, err
	}
	values, err := captureInputValues(actual.Body, step.InputMeasurements)
	if err != nil {
		return request, err
	}
	publicValues, err := captureInputValues(redacted.Body, step.InputMeasurements)
	if err != nil {
		return request, err
	}
	actualValues, _ := json.Marshal(values)
	recordedValues, _ := json.Marshal(publicValues)
	if !sameJSONValue(actualValues, recordedValues) {
		return request, fmt.Errorf("request measurements may not capture redacted credentials")
	}
	body, err := json.Marshal(redacted)
	if err != nil {
		return request, err
	}
	request.TemplateSHA256, request.RequestSHA256 = templateSHA, digestBytes(body)
	request.ResolvedPath, request.ResolvedInput, request.Substitutions, request.Values = redacted.Path, redacted.Body, subs, values
	return request, nil
}

func validateRuntimeRequest(step RunnerStep, actionIndex int, request RequestObservation) error {
	if request.Actor != step.Actor || request.ActionIndex != actionIndex || request.Node != step.Node {
		return fmt.Errorf("request capture belongs to another actor or action")
	}
	template, err := json.Marshal(step.Call)
	if err != nil || request.TemplateSHA256 != digestBytes(template) {
		return fmt.Errorf("request capture template differs from the registered operation")
	}
	rendered, err := applySubstitutionsToCall(step.Call, request.Substitutions)
	if err != nil {
		return fmt.Errorf("request capture substitutions: %w", err)
	}
	body, err := json.Marshal(rendered)
	if err != nil || request.RequestSHA256 != digestBytes(body) || request.ResolvedPath != rendered.Path || !sameJSONValue(request.ResolvedInput, rendered.Body) {
		return fmt.Errorf("request capture differs from its resolved operation")
	}
	values, err := captureInputValues(rendered.Body, step.InputMeasurements)
	if err != nil {
		return err
	}
	want, _ := json.Marshal(values)
	actual, _ := json.Marshal(request.Values)
	if !sameJSONValue(want, actual) {
		return fmt.Errorf("request measurement differs from the captured resolved input")
	}
	return nil
}

func runtimeRequestFor(step RunnerStep, actionIndex int, observed *ModelObservation) (*RequestObservation, error) {
	var request *RequestObservation
	for i := range observed.Requests {
		candidate := &observed.Requests[i]
		if candidate.Actor == step.Actor && candidate.ActionIndex == actionIndex {
			if request != nil {
				return nil, fmt.Errorf("duplicate request capture for actor %s action %d", step.Actor, actionIndex)
			}
			request = candidate
		}
	}
	if request == nil {
		return nil, fmt.Errorf("missing pre-action request capture for actor %s action %d", step.Actor, actionIndex)
	}
	if err := validateRuntimeRequest(step, actionIndex, *request); err != nil {
		return nil, err
	}
	return request, nil
}

func requestMatchesAction(request RequestObservation, action ActionObservation) bool {
	return request.Actor == action.Actor && request.ActionIndex == action.ActionIndex && request.Node == action.Node && request.TemplateSHA256 == action.TemplateSHA256 && request.RequestSHA256 == action.RequestSHA256 && request.ResolvedPath == action.ResolvedPath && sameJSONValue(request.ResolvedInput, action.ResolvedInput)
}
