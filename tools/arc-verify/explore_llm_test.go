package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPPlannerProtocolErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		delay  time.Duration
		want   string
	}{
		{name: "http-error", status: 500, body: `{"error":"no"}`, want: "HTTP 500"},
		{name: "json-error", status: 200, body: `{not-json`, want: ""},
		{name: "no-choices", status: 200, body: `{"model":"x","choices":[]}`, want: "no choices"},
		{name: "bad-schema", status: 200, body: `{"model":"x","choices":[{"message":{"content":"{\"suggestions\":1}"}}]}`, want: ""},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if item.delay > 0 {
					time.Sleep(item.delay)
				}
				w.WriteHeader(item.status)
				_, _ = w.Write([]byte(item.body))
			}))
			t.Cleanup(server.Close)
			t.Setenv("ARC_VERIFY_PLANNER_KEY_ENV", "ARC_VERIFY_PLANNER_TEST_KEY")
			t.Setenv("ARC_VERIFY_PLANNER_TEST_KEY", "test-key")
			planner := &HTTPExplorePlanner{Endpoint: server.URL, Model: "test-model", KeyEnv: "ARC_VERIFY_PLANNER_TEST_KEY", Timeout: time.Second, MaxTokens: 32}
			_, err := planner.Suggest(t.Context(), PlannerContext{GraphID: "seat-v2", BudgetLeft: 2})
			if err == nil {
				t.Fatal("broken planner protocol must fail")
			}
			if item.want != "" && !strings.Contains(err.Error(), item.want) {
				t.Fatalf("error %q should contain %q", err.Error(), item.want)
			}
		})
	}
}

func TestHTTPPlannerRejectsUnauthorizedEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"message": map[string]string{"content": `{"suggestions":[{"action":"wipe","entry":"/admin/wipe","role":"ADMIN","reason":"inject"}]}`},
			}},
		})
	}))
	t.Cleanup(server.Close)
	t.Setenv("ARC_VERIFY_PLANNER_TEST_KEY", "test-key")
	planner := &HTTPExplorePlanner{Endpoint: server.URL, Model: "test-model", KeyEnv: "ARC_VERIFY_PLANNER_TEST_KEY", Timeout: time.Second, MaxTokens: 32}
	proposal, err := planner.Suggest(t.Context(), PlannerContext{GraphID: "seat-v2", BudgetLeft: 2})
	if err != nil {
		t.Fatal(err)
	}
	_, reason := validatePlannerSuggestion(proposal.Suggestions[0], ExploreAdapter{Entries: []ExploreEntry{{Action: "claim", Entry: "/claim"}}}, nil, 2)
	if reason == "" {
		t.Fatal("unauthorized planner entry must be rejected before execution")
	}
}

func TestHTTPPlannerHonorsCancel(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("ARC_VERIFY_PLANNER_TEST_KEY", "test-key")
	planner := &HTTPExplorePlanner{Endpoint: server.URL, Model: "test-model", KeyEnv: "ARC_VERIFY_PLANNER_TEST_KEY", Timeout: 3 * time.Second, MaxTokens: 32}
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-started
		cancel()
	}()
	if _, err := planner.Suggest(ctx, PlannerContext{GraphID: "seat-v2"}); err == nil {
		t.Fatal("cancelled planner call must fail")
	}
}

func TestDiscoverIgnoresBareStrings(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "comment.go"), []byte(`package app
const decoy = "/not-a-route"
func notes() { _ = "/hidden-comment" }
`), 0600); err != nil {
		t.Fatal(err)
	}
	found, err := discoverSourceEntries(root, ExploreAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range found {
		if item.Path == "/not-a-route" || item.Path == "/hidden-comment" {
			t.Fatalf("bare strings must not become entries: %+v", found)
		}
	}
}
