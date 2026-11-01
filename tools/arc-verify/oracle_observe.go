package main

import (
	"time"
)

const (
	oracleObserved  = "OBSERVED"
	oracleDerived   = "DERIVED"
	oracleSynthetic = "SYNTHETIC"
	oracleUnknown   = "UNKNOWN"
)

type OracleField struct {
	Name      string `json:"name"`
	Class     string `json:"class"`
	Source    string `json:"source"`
	Query     string `json:"query,omitempty"`
	Target    string `json:"target,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Collected string `json:"collected"`
	Digest    string `json:"digest,omitempty"`
	Entity    string `json:"entity,omitempty"`
	Synthetic bool   `json:"synthetic,omitempty"`
	Inferred  bool   `json:"inferred,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
}

func classifyOracleFacts(facts IndependentFacts, adapter OracleAdapter, target, actor string, synthetic bool) []OracleField {
	collected := time.Now().UTC().Format(time.RFC3339Nano)
	class := oracleObserved
	if synthetic {
		class = oracleSynthetic
	}
	fields := []OracleField{
		{Name: "state", Class: class, Source: adapter.ID, Query: adapter.StatePath, Target: target, Actor: actor, Collected: collected, Entity: facts.State},
		{Name: "role", Class: class, Source: adapter.ID, Query: adapter.RolePath, Target: target, Actor: actor, Collected: collected, Entity: facts.Role},
		{Name: "principal", Class: class, Source: adapter.ID, Query: adapter.PrincipalPath, Target: target, Actor: actor, Collected: collected, Entity: facts.Principal},
	}
	for name := range facts.Conditions {
		fields = append(fields, OracleField{Name: "condition." + name, Class: class, Source: adapter.ID, Query: adapter.Conditions[name], Target: target, Actor: actor, Collected: collected})
	}
	for name := range facts.Values {
		fields = append(fields, OracleField{Name: "measurement." + name, Class: class, Source: adapter.ID, Query: adapter.Measurements[name], Target: target, Actor: actor, Collected: collected})
	}
	return fields
}

func oracleAllowsBusinessPass(fields []OracleField) bool {
	observed := 0
	for _, field := range fields {
		if field.Class == oracleSynthetic || field.Synthetic {
			return false
		}
		if field.Class == oracleUnknown {
			return false
		}
		if field.Class == oracleObserved {
			observed++
		}
	}
	return observed > 0
}
