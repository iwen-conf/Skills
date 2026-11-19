package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Storyline is the canonical Detroit-style branching verification contract.
type Storyline struct {
	StoryID         string            `yaml:"story_id" json:"story_id"`
	FocalEntity     string            `yaml:"focal_entity" json:"focal_entity"`
	FrozenReadonly  []string          `yaml:"frozen_readonly,omitempty" json:"frozen_readonly,omitempty"`
	BlindMask       []string          `yaml:"blind_mask,omitempty" json:"blind_mask,omitempty"`
	WorldFlags      map[string]string `yaml:"world_flags" json:"world_flags"`
	Checkpoints     []Checkpoint      `yaml:"checkpoints" json:"checkpoints"`
	Routes          []Route           `yaml:"routes" json:"routes"`
	RequiredEndings []string          `yaml:"required_endings" json:"required_endings"`

	// Runtime metadata populated by loader (not serialized back to YAML).
	SourcePath string `yaml:"-" json:"source_path,omitempty"`
	SHA256     string `yaml:"-" json:"sha256,omitempty"`
}

type Checkpoint struct {
	SaveID      string          `yaml:"save_id" json:"save_id"`
	Parent      string          `yaml:"parent" json:"parent"`
	Engine      string          `yaml:"engine,omitempty" json:"engine,omitempty"` // dir | sqlite | postgres
	SetupSteps  []SetupStep     `yaml:"setup_steps" json:"setup_steps"`
	AssertFlags map[string]any  `yaml:"assert_flags,omitempty" json:"assert_flags,omitempty"`
	SeedCmd     string          `yaml:"seed_cmd,omitempty" json:"seed_cmd,omitempty"`
}

type SetupStep struct {
	Call string         `yaml:"call" json:"call"`
	With map[string]any `yaml:"with,omitempty" json:"with,omitempty"`
	Cmd  string         `yaml:"cmd,omitempty" json:"cmd,omitempty"`
}

type Route struct {
	RouteID       string      `yaml:"route_id" json:"route_id"`
	LoadSave      string      `yaml:"load_save" json:"load_save"`
	RunnerType    string      `yaml:"runner_type" json:"runner_type"` // regular_unit | spiderweb_surface | fullstack_e2e | codex_integration | claude_surface | grok_e2e
	AgentHint     string      `yaml:"agent_hint,omitempty" json:"agent_hint,omitempty"`
	WriteScope    []string    `yaml:"write_scope,omitempty" json:"write_scope,omitempty"`
	RuntimeNS     string      `yaml:"runtime_ns,omitempty" json:"runtime_ns,omitempty"`
	VerifyCmd     string      `yaml:"verify_cmd,omitempty" json:"verify_cmd,omitempty"`
	Sequence      []RouteStep `yaml:"sequence" json:"sequence"`
	UnlocksEnding string      `yaml:"unlocks_ending" json:"unlocks_ending"`
}

type RouteStep struct {
	Step            int            `yaml:"step" json:"step"`
	Choice          string         `yaml:"choice" json:"choice"`
	With            map[string]any `yaml:"with,omitempty" json:"with,omitempty"`
	Cmd             string         `yaml:"cmd,omitempty" json:"cmd,omitempty"`
	ExpectStatus    string         `yaml:"expect_status" json:"expect_status"`
	ButterflyProbes map[string]any `yaml:"butterfly_probes,omitempty" json:"butterfly_probes,omitempty"`
	ProbeCmd        string         `yaml:"probe_cmd,omitempty" json:"probe_cmd,omitempty"`
	AntiSoftlock    string         `yaml:"anti_softlock,omitempty" json:"anti_softlock,omitempty"`
}

// ShardPlan is the concurrent multi-agent dispatch manifest compiled from a Storyline.
type ShardPlan struct {
	StoryID          string        `yaml:"story_id" json:"story_id"`
	FocalEntity      string        `yaml:"focal_entity" json:"focal_entity"`
	StorySHA256      string        `yaml:"story_sha256" json:"story_sha256"`
	FrozenReadonly   []string      `yaml:"frozen_readonly" json:"frozen_readonly"`
	BlindMask        []string      `yaml:"blind_mask,omitempty" json:"blind_mask,omitempty"`
	CheckpointOrder  []string      `yaml:"checkpoint_order" json:"checkpoint_order"`
	ConcurrentShards []VerifyShard `yaml:"concurrent_shards" json:"concurrent_shards"`
}

type VerifyShard struct {
	ShardID       string   `yaml:"shard_id" json:"shard_id"`
	LoadSave      string   `yaml:"load_save" json:"load_save"`
	Layer         string   `yaml:"layer" json:"layer"`
	AgentHint     string   `yaml:"agent_hint" json:"agent_hint"`
	WriteScope    []string `yaml:"write_scope" json:"write_scope"`
	RuntimeNS     string   `yaml:"runtime_ns" json:"runtime_ns"`
	UnlocksEnding string   `yaml:"unlocks_ending" json:"unlocks_ending"`
	VerifyCmd     string   `yaml:"verify_cmd" json:"verify_cmd"`
}

type CompletionReport struct {
	StoryID              string            `json:"story_id"`
	StorySHA256          string            `json:"story_sha256"`
	Verdict              string            `json:"verdict"` // PASS | FAIL | BLOCKED | NOT_RUN
	WorldFlagBindingPct  float64           `json:"world_flag_binding_pct"`
	RouteCoveragePct     float64           `json:"route_coverage_pct"`
	ButterflyProbeHitPct float64           `json:"butterfly_probe_hit_pct"`
	EndingUnlockPct      float64           `json:"ending_unlock_pct"`
	SoftlockCount        int               `json:"softlock_count"`
	UnresolvedFailCount  int               `json:"unresolved_fail_count"`
	HistoricalFailCount  int               `json:"historical_fail_count"`
	UnlockedEndings      []string          `json:"unlocked_endings"`
	MissingEndings       []string          `json:"missing_endings"`
	RouteStatuses        map[string]string `json:"route_statuses"`
	Reason               string            `json:"reason,omitempty"`
}

func loadStoryline(path string) (*Storyline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read storyline %s: %w", path, err)
	}
	var s Storyline
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse JSON storyline %s: %w", path, err)
		}
	} else {
		if err := yaml.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse YAML storyline %s: %w", path, err)
		}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		s.SourcePath = abs
	} else {
		s.SourcePath = path
	}
	sum := sha256.Sum256(raw)
	s.SHA256 = hex.EncodeToString(sum[:])

	// Normalize default runtime_ns per route if omitted.
	for i := range s.Routes {
		if strings.TrimSpace(s.Routes[i].RuntimeNS) == "" && s.Routes[i].RouteID != "" {
			s.Routes[i].RuntimeNS = "ns_" + sanitizeNS(s.Routes[i].RouteID)
		}
	}
	return &s, nil
}

func sanitizeNS(id string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
