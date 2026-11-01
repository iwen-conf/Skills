package main

type ModeContrastRow struct {
	Mode             string `json:"mode"`
	ConfirmedDefects int    `json:"confirmed_defects"`
	UniqueToMode     int    `json:"unique_to_mode"`
	Missed           int    `json:"missed"`
	FalsePositives   int    `json:"false_positives"`
	Unspecified      int    `json:"unspecified"`
	PlannerCalls     int    `json:"planner_calls"`
	Actions          int    `json:"actions"`
	ElapsedMS        int64  `json:"elapsed_ms"`
	Tokens           int    `json:"tokens,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

func contrastExploreModes(sealed []SealedMutation, reports map[string]ExploreReport, plannerCalls map[string]int) []ModeContrastRow {
	seen := map[string]map[string]bool{}
	for mode, report := range reports {
		seen[mode] = map[string]bool{}
		attr := AttributeBlindCampaign(report, sealed, report.Spent.asBudget())
		for _, item := range attr.Attributions {
			if item.Verdict == FaultDetected {
				seen[mode][item.MutationID] = true
			}
		}
	}
	out := []ModeContrastRow{}
	for mode, report := range reports {
		row := ModeContrastRow{Mode: mode, Unspecified: len(report.Unspecified), PlannerCalls: plannerCalls[mode], Actions: report.Actions, ElapsedMS: report.Spent.ElapsedMS}
		attr := AttributeBlindCampaign(report, sealed, report.Spent.asBudget())
		for _, item := range attr.Attributions {
			switch item.Verdict {
			case FaultDetected:
				row.ConfirmedDefects++
				unique := true
				for other, found := range seen {
					if other == mode {
						continue
					}
					if found[item.MutationID] {
						unique = false
					}
				}
				if unique {
					row.UniqueToMode++
				}
			case FaultSurvived, FaultFalseNegative:
				row.Missed++
			case FaultFalsePositive:
				row.FalsePositives++
			}
		}
		if mode == "ai" && plannerCalls[mode] < 1 {
			row.Notes = "planner_calls=0; AI incremental value is BLOCKED"
		}
		out = append(out, row)
	}
	return out
}

func (s ExploreSpent) asBudget() ExploreRuntimeBudget {
	return ExploreRuntimeBudget{MaxActions: atLeast(s.Actions, 1), MaxStates: atLeast(s.States, 1), Deadline: 1}
}

func atLeast(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}
