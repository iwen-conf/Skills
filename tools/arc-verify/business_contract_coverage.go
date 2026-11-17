package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

type businessCoveragePartition struct {
	action string
	flow   string
	state  string
	keys   []string
	cases  uint64
}

// ProveBusinessCoverage proves a branch partition only for the caller's
// explicitly declared finite domains and action/flow/state scopes. It neither
// executes the SUT nor claims reachability, infinite-domain or path coverage.
func ProveBusinessCoverage(g Graph) BusinessCoverageProof {
	proof := BusinessCoverageProof{Status: "BLOCKED", ProofKind: "finite_branch_partition", Issues: []BusinessCoverageIssue{}, IssueCounts: map[string]int{}}
	if g.BusinessContract == nil {
		proof.Status, proof.Reason = "LEGACY_UNDECLARED", "no deterministic contract is declared"
		return proof
	}
	if err := ValidateBusinessContract(g); err != nil {
		proof.Status, proof.Reason = "MODEL_ERROR", err.Error()
		return proof
	}
	coverage := g.BusinessContract.Coverage
	if coverage == nil {
		proof.Reason = "no finite coverage domain and explicit action/flow/state scope were declared"
		return proof
	}
	proof.Scope, proof.Budget = coverage.Scope, coverage.MaxCases
	proof.Domains, _ = cloneBusinessJSON(coverage.Domains)
	proof.Scopes, _ = cloneBusinessJSON(coverage.Scopes)
	proof.ContractSHA256, _ = businessJSONHash(g)
	domains := map[string][]json.RawMessage{}
	for _, domain := range coverage.Domains {
		domains[domain.Subject+"."+domain.Attribute] = domain.Values
	}
	attributes := businessAttributeIndex(g.BusinessContract)
	partitions := []businessCoveragePartition{}
	missing := false
	addIssue := func(issue BusinessCoverageIssue) {
		proof.IssueCounts[issue.Status]++
		if len(proof.Issues) < 64 {
			proof.Issues = append(proof.Issues, issue)
		} else {
			proof.IssuesTruncated = true
		}
	}
	for _, scope := range coverage.Scopes {
		for _, state := range scope.States {
			keys := []string{}
			for _, branch := range g.BusinessContract.Branches {
				if branch.Action != scope.Action || branch.Flow != "" && scope.Flow != "" && branch.Flow != scope.Flow || !containsBusinessID(branch.FromStates, state) {
					continue
				}
				keys = append(keys, businessPredicateRefs(branch.When)...)
			}
			keys = uniqueSortedBusiness(keys)
			partition := businessCoveragePartition{action: scope.Action, flow: scope.Flow, state: state, keys: keys, cases: 1}
			for _, key := range keys {
				if len(domains[key]) == 0 {
					missing = true
					addIssue(BusinessCoverageIssue{Status: "BLOCKED", Action: scope.Action, Flow: scope.Flow, State: state, Reason: "finite domain not declared for " + key})
					continue
				}
				if partition.cases > math.MaxUint64/uint64(len(domains[key])) {
					partition.cases = math.MaxUint64
				} else {
					partition.cases *= uint64(len(domains[key]))
				}
			}
			if proof.RequiredCases > math.MaxUint64-partition.cases {
				proof.RequiredCases = math.MaxUint64
			} else {
				proof.RequiredCases += partition.cases
			}
			partitions = append(partitions, partition)
		}
	}
	if missing {
		proof.Reason = "one or more relevant guard domains are undeclared; no completeness claim is possible"
		return proof
	}
	// Stable ordering makes counterexamples and bounded prefixes reproducible.
	sort.Slice(partitions, func(i, j int) bool {
		left, right := partitions[i], partitions[j]
		return left.action+"\x00"+left.flow+"\x00"+left.state < right.action+"\x00"+right.flow+"\x00"+right.state
	})
	for _, partition := range partitions {
		if proof.CheckedCases >= proof.Budget {
			break
		}
		observed := BusinessObservation{Flow: partition.flow, State: partition.state, Values: map[string]json.RawMessage{}, Bindings: map[string]string{}}
		for _, subject := range g.BusinessContract.Subjects {
			if subject.RequiresBinding {
				observed.Bindings[subject.ID] = "symbolic-domain:" + subject.ID
			}
		}
		var visit func(int)
		visit = func(index int) {
			if proof.CheckedCases >= proof.Budget {
				return
			}
			if index < len(partition.keys) {
				key := partition.keys[index]
				for _, value := range domains[key] {
					observed.Values[key] = value
					visit(index + 1)
					if proof.CheckedCases >= proof.Budget {
						break
					}
				}
				return
			}
			proof.CheckedCases++
			decision := selectBusinessBranchIndexed(g.BusinessContract, partition.action, observed, attributes)
			if decision.Status != "READY" {
				values, _ := cloneBusinessJSON(observed.Values)
				addIssue(BusinessCoverageIssue{Status: decision.Status, Action: partition.action, Flow: partition.flow, State: partition.state, Values: values, Branches: decision.Matches, Reason: decision.Reason})
			}
		}
		visit(0)
	}
	proof.EnumerationComplete = uint64(proof.CheckedCases) == proof.RequiredCases
	switch {
	case proof.IssueCounts["MODEL_ERROR"] > 0:
		proof.Status, proof.Reason = "MODEL_ERROR", "finite-domain counterexamples match multiple confirmed branches"
	case proof.IssueCounts["SPECIFICATION_GAP"] > 0:
		proof.Status, proof.Reason = "SPECIFICATION_GAP", "finite-domain counterexamples have no matching branch"
	case proof.IssueCounts["BLOCKED"] > 0:
		proof.Status, proof.Reason = "BLOCKED", "some scoped cases lack confirmed, known observations"
	case !proof.EnumerationComplete:
		proof.Status, proof.Reason = "BUDGET_EXHAUSTED", fmt.Sprintf("checked %d of %d declared finite cases; no completeness proof", proof.CheckedCases, proof.RequiredCases)
	default:
		proof.Status, proof.CompleteInScope = "PROVEN_IN_SCOPE", true
		proof.Reason = "exactly one confirmed branch matches every declared finite case; this proves neither SUT behavior, reachability nor inputs or paths outside the explicit scope"
	}
	return proof
}
