package main

import "sort"

// BusinessSuccessorPolicy scopes the next operation in a registered ordered
// business journey. nil is an undeclared restriction; a non-nil empty set
// forbids every successor. Fields must use json:",omitzero", never omitempty,
// so freezing, RAW and campaign persistence preserve this distinction.
type BusinessSuccessorPolicy []string

func (policy BusinessSuccessorPolicy) Allows(action string) bool {
	return policy == nil || containsBusinessID(policy, action)
}

func (policy BusinessSuccessorPolicy) Clone() BusinessSuccessorPolicy {
	if policy == nil {
		return nil
	}
	result := make(BusinessSuccessorPolicy, len(policy))
	copy(result, policy)
	return result
}

// Intersect treats an undeclared restriction as the universal set. The result
// never widens an explicit restriction, including a disjoint/empty set.
func (policy BusinessSuccessorPolicy) Intersect(other BusinessSuccessorPolicy) BusinessSuccessorPolicy {
	if policy == nil {
		return other.Clone()
	}
	if other == nil {
		return policy.Clone()
	}
	result := BusinessSuccessorPolicy{}
	for _, action := range policy {
		if containsBusinessID(other, action) && !containsBusinessID(result, action) {
			result = append(result, action)
		}
	}
	sort.Strings(result)
	return result
}
