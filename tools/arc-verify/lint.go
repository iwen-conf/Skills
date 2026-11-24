package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type LintViolation struct {
	Code    string `json:"code"`
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type LintReport struct {
	StoryID         string          `json:"story_id"`
	Status          string          `json:"status"` // PASS | FAIL
	TotalErrors     int             `json:"total_errors"`
	Truncated       bool            `json:"truncated"`
	CheckpointOrder []string        `json:"checkpoint_order,omitempty"`
	Errors          []LintViolation `json:"errors"`
}

var intermediateSoftlockTokens = []string{
	"COOLING", "SUSPENSE", "PENDING", "FROZEN", "LOCKED", "HOLD", "WAITING", "PAUSED", "SUSPENDED",
}

var validExpectedBehaviors = map[string]bool{
	"ALLOW_TRANSITION":      true,
	"REJECT_INVALID_INPUT":  true,
	"HARD_BLOCK":            true,
	"ASYNC_HOLD_ESCAPE":     true,
	"IDEMPOTENT_REPLAY":     true,
	"CONCURRENT_CONTENTION": true,
	"FORCE_CASCADE":         true,
}

// lintStory validates storyline structure, checkpoint DAG, world_flag coverage,
// 6-dimension expected_behavior taxonomy, anti-softlock invariants, ending reachability,
// and concurrent write_scope isolation.
func lintStory(s *Storyline, worldScan *WorldScanResult, maxErrors int) *LintReport {
	if maxErrors <= 0 {
		maxErrors = 5
	}
	var all []LintViolation
	addErr := func(code, scope, msg string) {
		all = append(all, LintViolation{Code: code, Scope: scope, Message: msg})
	}

	if strings.TrimSpace(s.StoryID) == "" {
		addErr("MISSING_STORY_ID", "story", "story_id is required")
	}
	if strings.TrimSpace(s.FocalEntity) == "" {
		addErr("MISSING_FOCAL_ENTITY", "story", "focal_entity is required (e.g. users.id)")
	}
	if len(s.WorldFlags) == 0 {
		addErr("EMPTY_WORLD_FLAGS", "world_flags", "world_flags must declare coupled entity states")
	}
	if len(s.Checkpoints) == 0 {
		addErr("EMPTY_CHECKPOINTS", "checkpoints", "at least one checkpoint is required")
	}
	if len(s.Routes) == 0 {
		addErr("EMPTY_ROUTES", "routes", "at least one route is required")
	}
	if len(s.RequiredEndings) == 0 {
		addErr("EMPTY_REQUIRED_ENDINGS", "required_endings", "required_endings must enumerate all required business endings")
	}

	// 1. World flag completeness check against reverse entity scan (when provided).
	if worldScan != nil && len(worldScan.Candidates) > 0 {
		boundSources := map[string]bool{}
		for k, v := range s.WorldFlags {
			boundSources[strings.ToLower(strings.TrimSpace(k))] = true
			boundSources[strings.ToLower(strings.TrimSpace(v))] = true
			if dot := strings.IndexByte(v, '.'); dot > 0 {
				boundSources[strings.ToLower(strings.TrimSpace(v[:dot]))] = true
			}
		}
		for _, cand := range worldScan.Candidates {
			srcLow := strings.ToLower(cand.Source)
			flagLow := strings.ToLower(cand.Flag)
			tblLow := srcLow
			if dot := strings.IndexByte(srcLow, '.'); dot > 0 {
				tblLow = srcLow[:dot]
			}
			if !boundSources[flagLow] && !boundSources[srcLow] && !boundSources[tblLow] {
				addErr("UNBOUND_WORLD_FLAG", cand.Source,
					fmt.Sprintf("coupled entity %q (%s:%d, %s) is missing from world_flags", cand.Source, cand.File, cand.Line, cand.Kind))
			}
		}
	}

	// 2. Checkpoint DAG validation (duplicate IDs, unknown parents, cycles).
	cpMap := map[string]Checkpoint{}
	for _, cp := range s.Checkpoints {
		if strings.TrimSpace(cp.SaveID) == "" {
			addErr("MISSING_SAVE_ID", "checkpoints", "checkpoint save_id must not be empty")
			continue
		}
		if _, dup := cpMap[cp.SaveID]; dup {
			addErr("DUPLICATE_CHECKPOINT", cp.SaveID, "duplicate checkpoint save_id")
			continue
		}
		cpMap[cp.SaveID] = cp
	}
	cpOrder, cycleErr := topoSortCheckpoints(s.Checkpoints, cpMap)
	if cycleErr != nil {
		addErr("CHECKPOINT_DAG_INVALID", "checkpoints", cycleErr.Error())
	}

	// 3. Route validation: load_save, sequence, expected_behavior, anti-softlock, endings, runtime_ns, write_scope.
	requiredSet := map[string]bool{}
	for _, e := range s.RequiredEndings {
		requiredSet[e] = true
	}
	unlockedSet := map[string]bool{}
	routeIDs := map[string]bool{}
	nsOwner := map[string]string{}
	scopeOwners := map[string]string{}

	for _, r := range s.Routes {
		if strings.TrimSpace(r.RouteID) == "" {
			addErr("MISSING_ROUTE_ID", "routes", "route_id is required")
			continue
		}
		if routeIDs[r.RouteID] {
			addErr("DUPLICATE_ROUTE_ID", r.RouteID, "duplicate route_id")
		}
		routeIDs[r.RouteID] = true

		if _, ok := cpMap[r.LoadSave]; !ok {
			addErr("UNKNOWN_CHECKPOINT", r.RouteID, fmt.Sprintf("route references unknown load_save %q", r.LoadSave))
		}
		if len(r.Sequence) == 0 {
			addErr("EMPTY_SEQUENCE", r.RouteID, "route must have at least one step in sequence")
		}
		if strings.TrimSpace(r.UnlocksEnding) == "" {
			addErr("MISSING_ENDING", r.RouteID, "route must declare unlocks_ending")
		} else {
			unlockedSet[r.UnlocksEnding] = true
			if !requiredSet[r.UnlocksEnding] {
				addErr("UNDECLARED_ENDING", r.RouteID, fmt.Sprintf("route unlocks %q which is not in required_endings", r.UnlocksEnding))
			}
		}

		// Unique runtime namespace across concurrent routes.
		if prev, exists := nsOwner[r.RuntimeNS]; exists {
			addErr("DUPLICATE_RUNTIME_NS", r.RouteID, fmt.Sprintf("runtime_ns %q collides with route %q", r.RuntimeNS, prev))
		} else {
			nsOwner[r.RuntimeNS] = r.RouteID
		}

		// Disjoint write_scope across concurrent routes & no overlap with frozen_readonly.
		for _, ws := range r.WriteScope {
			clean := filepath.ToSlash(filepath.Clean(ws))
			for _, frozen := range s.FrozenReadonly {
				if scopesOverlap(clean, filepath.ToSlash(filepath.Clean(frozen))) {
					addErr("FROZEN_WRITE_VIOLATION", r.RouteID, fmt.Sprintf("write_scope %q overlaps frozen_readonly %q", ws, frozen))
				}
			}
			for prevScope, owner := range scopeOwners {
				if owner != r.RouteID && scopesOverlap(clean, prevScope) {
					addErr("OVERLAPPING_WRITE_SCOPE", r.RouteID, fmt.Sprintf("write_scope %q overlaps route %q scope %q", ws, owner, prevScope))
				}
			}
			scopeOwners[clean] = r.RouteID
		}

		// Step checks: choice, expected_behavior enum, and anti-softlock guard.
		for idx, st := range r.Sequence {
			if strings.TrimSpace(st.Choice) == "" {
				addErr("MISSING_STEP_CHOICE", r.RouteID, fmt.Sprintf("step %d missing choice", st.Step))
			}
			if beh := strings.TrimSpace(st.ExpectedBehavior); beh != "" && !validExpectedBehaviors[beh] {
				addErr("INVALID_EXPECTED_BEHAVIOR", r.RouteID,
					fmt.Sprintf("step %d has unknown expected_behavior %q (allowed: ALLOW_TRANSITION, REJECT_INVALID_INPUT, HARD_BLOCK, ASYNC_HOLD_ESCAPE, IDEMPOTENT_REPLAY, CONCURRENT_CONTENTION, FORCE_CASCADE)", st.Step, beh))
			}
			if isIntermediateSuspendedStatus(st.ExpectStatus) {
				hasEscapeStep := false
				for next := idx + 1; next < len(r.Sequence); next++ {
					if !isIntermediateSuspendedStatus(r.Sequence[next].ExpectStatus) || strings.TrimSpace(r.Sequence[next].AntiSoftlock) != "" {
						hasEscapeStep = true
						break
					}
				}
				if !hasEscapeStep && strings.TrimSpace(st.AntiSoftlock) == "" {
					addErr("SOFTLOCK_RISK", r.RouteID,
						fmt.Sprintf("step %d enters intermediate status %q without a subsequent escape transition or anti_softlock guard", st.Step, st.ExpectStatus))
				}
			}
		}
	}

	// 4. Every required_ending must be reachable by at least one route.
	for _, req := range s.RequiredEndings {
		if !unlockedSet[req] {
			addErr("UNREACHABLE_ENDING", req, fmt.Sprintf("required ending %q is not unlocked by any route", req))
		}
	}

	status := "PASS"
	if len(all) > 0 {
		status = "FAIL"
	}
	truncated := len(all) > maxErrors
	shown := all
	if truncated {
		shown = all[:maxErrors]
	}
	return &LintReport{
		StoryID:         s.StoryID,
		Status:          status,
		TotalErrors:     len(all),
		Truncated:       truncated,
		CheckpointOrder: cpOrder,
		Errors:          shown,
	}
}

func isIntermediateSuspendedStatus(status string) bool {
	up := strings.ToUpper(strings.TrimSpace(status))
	if up == "" {
		return false
	}
	parts := strings.FieldsFunc(up, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == ':' || r == '/'
	})
	for _, part := range parts {
		for _, tok := range intermediateSoftlockTokens {
			if part == tok {
				return true
			}
		}
	}
	return false
}

func topoSortCheckpoints(list []Checkpoint, cpMap map[string]Checkpoint) ([]string, error) {
	inDeg := map[string]int{}
	children := map[string][]string{}
	for _, cp := range list {
		if cp.SaveID == "" {
			continue
		}
		inDeg[cp.SaveID] = 0
	}
	for _, cp := range list {
		if cp.SaveID == "" {
			continue
		}
		parent := strings.TrimSpace(cp.Parent)
		if parent == "" || parent == "root" {
			continue
		}
		if _, ok := cpMap[parent]; !ok {
			return nil, fmt.Errorf("checkpoint %q references unknown parent %q", cp.SaveID, parent)
		}
		children[parent] = append(children[parent], cp.SaveID)
		inDeg[cp.SaveID]++
	}
	var queue []string
	for _, cp := range list {
		if cp.SaveID != "" && inDeg[cp.SaveID] == 0 {
			queue = append(queue, cp.SaveID)
		}
	}
	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, ch := range children[cur] {
			inDeg[ch]--
			if inDeg[ch] == 0 {
				queue = append(queue, ch)
			}
		}
	}
	if len(order) != len(cpMap) {
		return nil, fmt.Errorf("checkpoint parent graph contains a cycle")
	}
	return order, nil
}

// scopesOverlap checks whether two glob/path scopes could touch the same file.
func scopesOverlap(a, b string) bool {
	if a == b {
		return true
	}
	pa := globPrefix(a)
	pb := globPrefix(b)
	if pa == "" || pb == "" {
		return true
	}
	return pa == pb || strings.HasPrefix(pa, pb+"/") || strings.HasPrefix(pb, pa+"/")
}

func globPrefix(p string) string {
	p = strings.TrimPrefix(p, "./")
	idx := strings.IndexAny(p, "*?[")
	if idx < 0 {
		return strings.TrimSuffix(p, "/")
	}
	prefix := strings.TrimSuffix(p[:idx], "/")
	return prefix
}
