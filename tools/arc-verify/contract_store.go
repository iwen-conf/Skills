package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

// The authored immutable model remains the source. These normalized projections
// make branch identity/guards/preparation queryable in the same transaction, and
// are checked on every model read so a damaged projection is never a second truth.
func businessContractStatements(g Graph) ([]string, error) {
	if g.BusinessContract == nil {
		return nil, nil
	}
	contract := g.BusinessContract
	body, err := json.Marshal(contract)
	if err != nil {
		return nil, err
	}
	hash, err := businessJSONHash(contract)
	if err != nil {
		return nil, err
	}
	statements := []string{fmt.Sprintf("INSERT INTO business_contracts(graph,version,sha256,body) VALUES(%s,%d,%s,%s)", quote(g.ID), contract.Version, quote(hash), quote(string(body)))}
	for _, branch := range contract.Branches {
		body, err := json.Marshal(branch)
		if err != nil {
			return nil, err
		}
		guard, err := json.Marshal(branch.When)
		if err != nil {
			return nil, err
		}
		expected, err := json.Marshal(branch.Expected)
		if err != nil {
			return nil, err
		}
		confirmed := 0
		if branch.Confirmed {
			confirmed = 1
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO business_branches(graph,id,flow,action,transition_id,rule_id,capability,confirmed,source_ref,guard_json,expected_json,body) VALUES(%s,%s,%s,%s,%s,%s,%s,%d,%s,%s,%s,%s)", quote(g.ID), quote(branch.ID), quote(branch.Flow), quote(branch.Action), quote(branch.Transition), quote(branch.Rule), quote(branch.Capability), confirmed, quote(branch.SourceRef), quote(string(guard)), quote(string(expected)), quote(string(body))))
	}
	for _, action := range contract.Preparation {
		body, err := json.Marshal(action)
		if err != nil {
			return nil, err
		}
		dependencies, err := json.Marshal(action.DependsOn)
		if err != nil {
			return nil, err
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO preparation_actions(graph,id,action,source_ref,dependencies_json,body) VALUES(%s,%s,%s,%s,%s,%s)", quote(g.ID), quote(action.ID), quote(action.Action), quote(action.SourceRef), quote(string(dependencies)), quote(string(body))))
	}
	return statements, nil
}

func (d db) validateBusinessContractProjection(g Graph) error {
	if g.BusinessContract == nil {
		return nil
	}
	rows, err := d.query("SELECT version,sha256,body FROM business_contracts WHERE graph="+quote(g.ID), 3)
	if err != nil {
		return fmt.Errorf("business contract projection unavailable; initialize the additive schema: %w", err)
	}
	body, _ := json.Marshal(g.BusinessContract)
	hash, _ := businessJSONHash(g.BusinessContract)
	if len(rows) != 1 || rows[0][0] != fmt.Sprint(g.BusinessContract.Version) || rows[0][1] != hash || rows[0][2] != string(body) {
		return errors.New("business contract projection contradicts immutable model")
	}
	branches, err := d.query("SELECT id,flow,action,transition_id,rule_id,capability,confirmed,source_ref,guard_json,expected_json,body FROM business_branches WHERE graph="+quote(g.ID), 11)
	if err != nil {
		return err
	}
	if len(branches) != len(g.BusinessContract.Branches) {
		return errors.New("business branch projection missing or unexpected rows")
	}
	wanted := map[string]BusinessBranch{}
	for _, branch := range g.BusinessContract.Branches {
		wanted[branch.ID] = branch
	}
	for _, row := range branches {
		branch, ok := wanted[row[0]]
		if !ok {
			return errors.New("unknown projected business branch")
		}
		guard, _ := json.Marshal(branch.When)
		expected, _ := json.Marshal(branch.Expected)
		body, _ := json.Marshal(branch)
		confirmed := "0"
		if branch.Confirmed {
			confirmed = "1"
		}
		want := []string{branch.ID, branch.Flow, branch.Action, branch.Transition, branch.Rule, branch.Capability, confirmed, branch.SourceRef, string(guard), string(expected), string(body)}
		for i := range want {
			if row[i] != want[i] {
				return fmt.Errorf("business branch %s projection contradicts immutable contract", branch.ID)
			}
		}
	}
	preparations, err := d.query("SELECT id,action,source_ref,dependencies_json,body FROM preparation_actions WHERE graph="+quote(g.ID), 5)
	if err != nil {
		return err
	}
	if len(preparations) != len(g.BusinessContract.Preparation) {
		return errors.New("preparation projection missing or unexpected rows")
	}
	byID := map[string]PreparationAction{}
	for _, action := range g.BusinessContract.Preparation {
		byID[action.ID] = action
	}
	for _, row := range preparations {
		action, ok := byID[row[0]]
		if !ok {
			return errors.New("unknown projected preparation")
		}
		body, _ := json.Marshal(action)
		dependencies, _ := json.Marshal(action.DependsOn)
		if row[1] != action.Action || row[2] != action.SourceRef || row[3] != string(dependencies) || row[4] != string(body) {
			return fmt.Errorf("preparation %s projection contradicts immutable contract", action.ID)
		}
	}
	return nil
}
