package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type HTTPExplorePlanner struct {
	Endpoint  string
	Model     string
	KeyEnv    string
	Timeout   time.Duration
	MaxTokens int
	Calls     int
	LastModel string
	LastUsage PlannerUsage
}

type PlannerUsage struct {
	PromptTokens     int   `json:"prompt_tokens,omitempty"`
	CompletionTokens int   `json:"completion_tokens,omitempty"`
	TotalTokens      int   `json:"total_tokens,omitempty"`
	ElapsedMS        int64 `json:"elapsed_ms,omitempty"`
}

func NewHTTPExplorePlannerFromEnv() (*HTTPExplorePlanner, error) {
	endpoint := firstNonEmpty(os.Getenv("ARC_VERIFY_PLANNER_URL"), os.Getenv("OPENAI_BASE_URL"))
	model := firstNonEmpty(os.Getenv("ARC_VERIFY_PLANNER_MODEL"), os.Getenv("OPENAI_MODEL"))
	keyEnv := firstNonEmpty(os.Getenv("ARC_VERIFY_PLANNER_KEY_ENV"), "OPENAI_API_KEY")
	if endpoint == "" || model == "" || os.Getenv(keyEnv) == "" {
		return nil, errors.New("planner unavailable: set ARC_VERIFY_PLANNER_URL, ARC_VERIFY_PLANNER_MODEL and a key env without writing the key to the ledger")
	}
	timeout := 20 * time.Second
	if raw := os.Getenv("ARC_VERIFY_PLANNER_TIMEOUT_MS"); raw != "" {
		if ms, err := time.ParseDuration(raw + "ms"); err == nil && ms >= time.Second && ms <= 2*time.Minute {
			timeout = ms
		}
	}
	return &HTTPExplorePlanner{Endpoint: strings.TrimRight(endpoint, "/"), Model: model, KeyEnv: keyEnv, Timeout: timeout, MaxTokens: 800}, nil
}

func (p *HTTPExplorePlanner) Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error) {
	if p == nil {
		return PlannerProposal{}, errors.New("planner unavailable")
	}
	if err := ctx.Err(); err != nil {
		return PlannerProposal{}, err
	}
	key := os.Getenv(p.KeyEnv)
	if key == "" {
		return PlannerProposal{}, errors.New("planner key env is empty")
	}
	body, err := json.Marshal(map[string]any{
		"model":           p.Model,
		"max_tokens":      p.MaxTokens,
		"response_format": map[string]any{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": "Return JSON {suggestions:[...]} only. Each suggestion needs hypothesis_id, capability, business_intent, boundary_hypothesis, action, entry, role, reason, observe, risk, preconditions, needs_recovery, source_evidence. Never decide PASS or FAIL or change RULE."},
			{"role": "user", "content": string(mustJSON(in))},
		},
	})
	if err != nil {
		return PlannerProposal{}, err
	}
	url := p.Endpoint
	if !strings.Contains(url, "/chat/completions") {
		url += "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return PlannerProposal{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: p.Timeout}
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return PlannerProposal{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return PlannerProposal{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return PlannerProposal{}, fmt.Errorf("planner HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Model   string `json:"model"`
		Usage   PlannerUsage
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := decodeStrictJSON(raw, &envelope); err != nil {
		return PlannerProposal{}, err
	}
	if len(envelope.Choices) == 0 {
		return PlannerProposal{}, errors.New("planner returned no choices")
	}
	var proposal PlannerProposal
	if err := decodeStrictJSON([]byte(envelope.Choices[0].Message.Content), &proposal); err != nil {
		return PlannerProposal{}, err
	}
	if len(proposal.Suggestions) > 8 {
		proposal.Suggestions = proposal.Suggestions[:8]
	}
	proposal.Model = firstNonEmpty(envelope.Model, p.Model)
	proposal.Tokens = envelope.Usage.TotalTokens
	p.Calls++
	p.LastModel = proposal.Model
	p.LastUsage = envelope.Usage
	p.LastUsage.ElapsedMS = time.Since(started).Milliseconds()
	return proposal, nil
}

func mustJSON(value any) []byte {
	body, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func plannerFromFlags(mode, path string) (ExplorePlanner, error) {
	if path != "" {
		return FileExplorePlanner{Path: path}, nil
	}
	if mode != "ai" {
		return nil, nil
	}
	return NewHTTPExplorePlannerFromEnv()
}
