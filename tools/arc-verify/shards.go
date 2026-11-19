package main

import (
	"fmt"
	"strings"
)

// planShards compiles a validated Storyline into a parallel ShardPlan.
func planShards(s *Storyline) (*ShardPlan, error) {
	lint := lintStory(s, nil, 10)
	if lint.Status != "PASS" {
		return nil, fmt.Errorf("storyline lint failed (%d errors); run `arc verify lint-story` first: %s",
			lint.TotalErrors, lint.Errors[0].Message)
	}

	frozen := append([]string{}, s.FrozenReadonly...)
	if s.SourcePath != "" {
		found := false
		for _, f := range frozen {
			if f == s.SourcePath {
				found = true
				break
			}
		}
		if !found {
			frozen = append(frozen, s.SourcePath)
		}
	}

	plan := &ShardPlan{
		StoryID:         s.StoryID,
		FocalEntity:     s.FocalEntity,
		StorySHA256:     s.SHA256,
		FrozenReadonly:  frozen,
		BlindMask:       append([]string{}, s.BlindMask...),
		CheckpointOrder: lint.CheckpointOrder,
	}

	for _, r := range s.Routes {
		layer, hint := inferLayerAndHint(r.RunnerType, r.AgentHint)
		writeScope := append([]string{}, r.WriteScope...)
		if len(writeScope) == 0 {
			writeScope = []string{fmt.Sprintf("tests/stories/%s/%s/**", sanitizeNS(s.StoryID), sanitizeNS(r.RouteID))}
		}
		verifyCmd := strings.TrimSpace(r.VerifyCmd)
		if verifyCmd == "" {
			verifyCmd = fmt.Sprintf("SHARD_NS=%s arc verify play %s --route %s", r.RuntimeNS, s.SourcePath, r.RouteID)
		}
		plan.ConcurrentShards = append(plan.ConcurrentShards, VerifyShard{
			ShardID:       r.RouteID,
			LoadSave:      r.LoadSave,
			Layer:         layer,
			AgentHint:     hint,
			WriteScope:    writeScope,
			RuntimeNS:     r.RuntimeNS,
			UnlocksEnding: r.UnlocksEnding,
			VerifyCmd:     verifyCmd,
		})
	}
	return plan, nil
}

func inferLayerAndHint(runnerType, explicitHint string) (layer, hint string) {
	low := strings.ToLower(strings.TrimSpace(runnerType))
	switch {
	case strings.Contains(low, "e2e") || strings.Contains(low, "playwright") || strings.Contains(low, "grok"):
		layer = "fullstack_e2e"
		hint = "grok"
	case strings.Contains(low, "surface") || strings.Contains(low, "claude"):
		layer = "spiderweb_surface"
		hint = "claude"
	default:
		layer = "regular_unit"
		hint = "codex"
	}
	if strings.TrimSpace(explicitHint) != "" {
		hint = explicitHint
	}
	return layer, hint
}
