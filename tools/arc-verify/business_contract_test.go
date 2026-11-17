package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func businessRef(subject, attribute string) *BusinessOperand {
	return &BusinessOperand{Ref: &BusinessAttributeRef{Subject: subject, Attribute: attribute}}
}
func businessLiteral(kind, value string) *BusinessOperand {
	return &BusinessOperand{Literal: &BusinessLiteral{Type: kind, Value: json.RawMessage(value)}}
}
func businessPredicate(op, subject, attribute, kind, value string) BusinessPredicate {
	return BusinessPredicate{Op: op, Left: businessRef(subject, attribute), Right: businessLiteral(kind, value)}
}
func businessNot(predicate BusinessPredicate) BusinessPredicate {
	return BusinessPredicate{Op: "not", Args: []BusinessPredicate{predicate}}
}
func businessBool(value bool) *bool { return &value }

func contractModel(t *testing.T) Graph {
	t.Helper()
	graph := referenceModel(t)
	active := businessPredicate("eq", "primary", "active", "boolean", "true")
	graph.BusinessContract = &BusinessContract{
		Version: 1,
		Subjects: []BusinessSubject{
			{ID: "primary", Kind: "actor", RequiresBinding: true, Attributes: []BusinessAttribute{{Name: "active", Type: "boolean"}, {Name: "level", Type: "integer"}, {Name: "tags", Type: "string_set"}, {Name: "id", Type: "string"}}},
			{ID: "other", Kind: "actor", RequiresBinding: true, Attributes: []BusinessAttribute{{Name: "active", Type: "boolean"}, {Name: "id", Type: "string"}}},
			{ID: "order", Kind: "entity", Attributes: []BusinessAttribute{{Name: "owner", Type: "string"}, {Name: "amount", Type: "number"}, {Name: "stored", Type: "boolean"}, {Name: "events", Type: "integer"}, {Name: "balance", Type: "integer"}}},
		},
		Branches: []BusinessBranch{
			{ID: "accept", Action: "enroll", Rule: "R1", Flow: "flow.enrollment", FromStates: []string{"pending"}, When: active, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "created", State: "ready", Denied: businessBool(false), Predicate: BusinessPredicate{Op: "true"}}}}, Source: "document", SourceRef: "test-spec:active", Confirmed: true},
			{ID: "reject", Action: "enroll", Rule: "R1", Flow: "flow.enrollment", FromStates: []string{"pending"}, When: businessNot(active), Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "unchanged", State: "UNCHANGED", Denied: businessBool(true), Predicate: BusinessPredicate{Op: "true"}}}}, Source: "document", SourceRef: "test-spec:inactive", Confirmed: true},
		},
		Coverage: &BusinessCoverageDomain{Scope: "all declared boolean activation values in pending enrollment; no path or SUT claim", Domains: []BusinessFiniteDomain{{Subject: "primary", Attribute: "active", Values: []json.RawMessage{json.RawMessage("false"), json.RawMessage("true")}}}, MaxCases: 100, Scopes: []BusinessCoverageScope{{Action: "enroll", Flow: "flow.enrollment", States: []string{"pending"}}}},
	}
	if err := validateGraph(graph); err != nil {
		t.Fatal(err)
	}
	return graph
}

func contractObservation(active bool) BusinessObservation {
	value := "false"
	if active {
		value = "true"
	}
	return BusinessObservation{Flow: "flow.enrollment", State: "pending", Values: map[string]json.RawMessage{
		"primary.active": json.RawMessage(value), "primary.level": json.RawMessage("3"), "primary.tags": json.RawMessage(`["vip","buyer"]`), "primary.id": json.RawMessage(`"acct-A"`),
		"other.active": json.RawMessage("true"), "other.id": json.RawMessage(`"acct-B"`), "order.owner": json.RawMessage(`"acct-A"`), "order.amount": json.RawMessage("10.25"), "order.stored": json.RawMessage("false"), "order.events": json.RawMessage("0"), "order.balance": json.RawMessage("10"),
	}, Bindings: map[string]string{"primary": "acct-A", "other": "acct-B"}, Sources: map[string][]string{"order.stored": {"db"}, "order.events": {"events"}, "order.balance": {"db"}}}
}

func TestBusinessContractD01SameCompleteInputCannotSelectDifferentExpectations(t *testing.T) {
	graph := contractModel(t)
	duplicate, _ := cloneBusinessJSON(graph.BusinessContract.Branches[0])
	duplicate.ID, duplicate.Expected.Outcomes[0].State = "overlap", "pending"
	graph.BusinessContract.Branches = append(graph.BusinessContract.Branches, duplicate)
	decision := FreezeBusinessExpectation(graph, "enroll", contractObservation(true))
	if decision.Status != "MODEL_ERROR" || len(decision.Matches) != 2 || decision.Frozen != nil {
		t.Fatalf("conflicting complete input was executable: %+v", decision)
	}
}

func TestBusinessContractD02SameActionDifferentPrestateSelectsDifferentUniqueBranch(t *testing.T) {
	graph := contractModel(t)
	again, _ := cloneBusinessJSON(graph.BusinessContract.Branches[0])
	again.ID, again.FromStates, again.When = "already-enrolled", []string{"ready"}, BusinessPredicate{Op: "true"}
	again.Expected.Outcomes[0].ID = "idempotent"
	graph.BusinessContract.Branches = append(graph.BusinessContract.Branches, again)
	before := contractObservation(true)
	first := FreezeBusinessExpectation(graph, "enroll", before)
	before.State = "ready"
	second := FreezeBusinessExpectation(graph, "enroll", before)
	if first.Status != "READY" || first.Branch != "accept" || second.Status != "READY" || second.Branch != "already-enrolled" {
		t.Fatalf("prestate was not part of branch identity: first=%+v second=%+v", first, second)
	}
}

func TestBusinessContractD03PartialOverlapProducesFiniteCounterexample(t *testing.T) {
	graph := contractModel(t)
	graph.BusinessContract.Subjects[0].RequiresBinding = false
	positive := businessPredicate("gte", "primary", "level", "integer", "1")
	negative := businessPredicate("lte", "primary", "level", "integer", "2")
	graph.BusinessContract.Branches[0].When, graph.BusinessContract.Branches[1].When = positive, negative
	graph.BusinessContract.Coverage.Domains = []BusinessFiniteDomain{{Subject: "primary", Attribute: "level", Values: []json.RawMessage{json.RawMessage("0"), json.RawMessage("1"), json.RawMessage("2"), json.RawMessage("3")}}}
	proof := ProveBusinessCoverage(graph)
	if proof.Status != "MODEL_ERROR" || proof.CompleteInScope || !proof.EnumerationComplete || proof.CheckedCases != 4 || proof.IssueCounts["MODEL_ERROR"] != 2 {
		t.Fatalf("partial overlap disappeared: %+v", proof)
	}
	if string(proof.Issues[0].Values["primary.level"]) != "1" {
		t.Fatalf("missing reproducible counterexample: %+v", proof.Issues)
	}
}

func TestBusinessContractD04FiniteGapCannotDefaultToSuccess(t *testing.T) {
	graph := contractModel(t)
	graph.BusinessContract.Branches = graph.BusinessContract.Branches[:1]
	decision := FreezeBusinessExpectation(graph, "enroll", contractObservation(false))
	proof := ProveBusinessCoverage(graph)
	if decision.Status != "SPECIFICATION_GAP" || decision.Frozen != nil || proof.Status != "SPECIFICATION_GAP" || proof.CompleteInScope {
		t.Fatalf("unmatched state defaulted to success: decision=%+v proof=%+v", decision, proof)
	}
}

func TestBusinessContractD05ObservedPrestateCannotBeReplacedByPlannedTarget(t *testing.T) {
	graph := contractModel(t)
	decision := FreezeBusinessBranch(graph, "accept", contractObservation(false))
	if decision.Status != "BLOCKED" || decision.Branch != "reject" || decision.Frozen != nil {
		t.Fatalf("planned branch displaced observed prestate: %+v", decision)
	}
}

func TestBusinessContractD06ActualCannotReselectExpectedBranch(t *testing.T) {
	graph := contractModel(t)
	before := contractObservation(true)
	decision := FreezeBusinessExpectation(graph, "enroll", before)
	if decision.Status != "READY" {
		t.Fatal(decision)
	}
	after := contractObservation(false) // Would match rejection if illegally reselected.
	comparison := CompareBusinessExpectation(graph, *decision.Frozen, after, true)
	if comparison.Status != "FAIL" || comparison.Branch != "accept" || comparison.Checks[0].Outcome != "created" {
		t.Fatalf("actual result rewrote the frozen expectation: %+v", comparison)
	}
	if decision.Frozen.Before.Values["primary.active"][0] != 't' {
		t.Fatal("comparison mutated pre-action evidence")
	}
}

func TestBusinessContractD07ObservedEventStateControlsNextTransition(t *testing.T) {
	graph := contractModel(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "awaiting-event", Kind: "state", Name: "等待外部事件"}, Node{ID: "settled", Kind: "state", Name: "已结算"})
	graph.BusinessContract.Branches = []BusinessBranch{
		{ID: "wait", Action: "enroll", Rule: "R1", Flow: "flow.enrollment", FromStates: []string{"awaiting-event"}, When: BusinessPredicate{Op: "true"}, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "still-waiting", State: "awaiting-event", Predicate: BusinessPredicate{Op: "true"}}}}, Source: "document", SourceRef: "test:event-before", Confirmed: true},
		{ID: "after-event", Action: "enroll", Rule: "R1", Flow: "flow.enrollment", FromStates: []string{"settled"}, When: BusinessPredicate{Op: "true"}, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "settled-action", State: "ready", Predicate: BusinessPredicate{Op: "true"}}}}, Source: "document", SourceRef: "test:event-after", Confirmed: true},
	}
	before := contractObservation(true)
	before.State = "awaiting-event"
	first := FreezeBusinessExpectation(graph, "enroll", before)
	before.State = "settled" // Independent event readback supplied to the next step.
	next := FreezeBusinessExpectation(graph, "enroll", before)
	if first.Branch != "wait" || next.Branch != "after-event" || first.Frozen.Expected.Outcomes[0].State != "awaiting-event" {
		t.Fatalf("terminal state was assumed before the observed event: first=%+v next=%+v", first, next)
	}
}

func TestBusinessContractMultiSubjectANDORNOTAndRelations(t *testing.T) {
	graph := contractModel(t)
	predicate := BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		businessPredicate("eq", "primary", "active", "boolean", "true"),
		{Op: "or", Args: []BusinessPredicate{businessPredicate("gt", "primary", "level", "integer", "5"), businessPredicate("contains", "primary", "tags", "string", `"vip"`)}},
		businessNot(businessPredicate("eq", "other", "active", "boolean", "false")),
		{Op: "eq", Left: businessRef("primary", "id"), Right: businessRef("order", "owner")},
	}}
	observed := contractObservation(true)
	if got := EvaluateBusinessPredicate(graph.BusinessContract, predicate, observed); got.Value != businessTrue {
		t.Fatal(got)
	}
	observed.Values["other.active"] = json.RawMessage("false")
	if got := EvaluateBusinessPredicate(graph.BusinessContract, predicate, observed); got.Value != businessFalse {
		t.Fatalf("other actor collapsed into primary: %+v", got)
	}
	delete(observed.Values, "other.active")
	if got := EvaluateBusinessPredicate(graph.BusinessContract, predicate, observed); got.Value != businessUnknown || !containsBusinessID(got.Unknown, "other.active") {
		t.Fatalf("unknown was coerced to false: %+v", got)
	}
	observed.Values["primary.active"] = json.RawMessage("false")
	if got := EvaluateBusinessPredicate(graph.BusinessContract, predicate, observed); got.Value != businessFalse {
		t.Fatalf("known false AND unknown should remain false: %+v", got)
	}
}

func TestBusinessContractTypedRangesSetsAndExactNumbers(t *testing.T) {
	graph := contractModel(t)
	observed := contractObservation(true)
	tests := []struct {
		name      string
		predicate BusinessPredicate
		want      BusinessTruth
	}{
		{"inclusive-range", businessPredicate("between", "order", "amount", "number_set", "[10.25,20]"), businessTrue},
		{"outside-range", businessPredicate("between", "order", "amount", "number_set", "[10.26,20]"), businessFalse},
		{"membership", businessPredicate("in", "primary", "level", "integer_set", "[1,3,5]"), businessTrue},
		{"subset", businessPredicate("subset", "primary", "tags", "string_set", `["buyer","vip","seller"]`), businessTrue},
		{"intersection", businessPredicate("intersects", "primary", "tags", "string_set", `["staff","vip"]`), businessTrue},
		{"set-order-equality", businessPredicate("eq", "primary", "tags", "string_set", `["buyer","vip"]`), businessTrue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateBusinessPredicate(graph.BusinessContract, test.predicate, observed); got.Value != test.want {
				t.Fatalf("want %s got %+v", test.want, got)
			}
		})
	}
	observed.Values["primary.level"] = json.RawMessage("9007199254740993")
	if got := EvaluateBusinessPredicate(graph.BusinessContract, businessPredicate("gt", "primary", "level", "integer", "9007199254740992"), observed); got.Value != businessTrue {
		t.Fatalf("integer precision lost through float64: %+v", got)
	}
	observed.Values["primary.level"] = json.RawMessage(`"3"`)
	if got := EvaluateBusinessPredicate(graph.BusinessContract, businessPredicate("eq", "primary", "level", "integer", "3"), observed); got.Value != businessUnknown {
		t.Fatalf("string silently coerced to integer: %+v", got)
	}
}

func TestBusinessContractUnknownCompetingGuardBlocksUniqueness(t *testing.T) {
	graph := contractModel(t)
	graph.BusinessContract.Branches[1].When = businessPredicate("eq", "other", "active", "boolean", "true")
	before := contractObservation(true)
	delete(before.Values, "other.active")
	decision := FreezeBusinessExpectation(graph, "enroll", before)
	if decision.Status != "BLOCKED" || len(decision.Matches) != 1 || decision.Frozen != nil {
		t.Fatalf("one true branch hid an unknown competing guard: %+v", decision)
	}
	graph.BusinessContract.Branches[1].Confirmed = false
	before.Values["other.active"] = json.RawMessage("true")
	decision = FreezeBusinessExpectation(graph, "enroll", before)
	if decision.Status != "BLOCKED" {
		t.Fatalf("unconfirmed competing branch ignored: %+v", decision)
	}
}

func TestBusinessContractExpectedIsDetachedAndTamperingRejected(t *testing.T) {
	graph := contractModel(t)
	before := contractObservation(true)
	decision := FreezeBusinessExpectation(graph, "enroll", before)
	before.Values["primary.active"][0] = 'f'
	graph.BusinessContract.Branches[0].Expected.Outcomes[0].State = "pending"
	if string(decision.Frozen.Before.Values["primary.active"]) != "true" || decision.Frozen.Expected.Outcomes[0].State != "ready" {
		t.Fatal("freeze aliases mutable source memory")
	}
	graph = contractModel(t)
	after := contractObservation(true)
	after.State = "ready"
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "PASS" {
		t.Fatal(got)
	}
	decision.Frozen.Expected.Outcomes[0].State = "pending"
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "MODEL_ERROR" {
		t.Fatalf("tampered expectation accepted: %+v", got)
	}
}

func TestBusinessContractAllowedOutcomeSetStillRequiresCommonInvariantAndOracle(t *testing.T) {
	graph := contractModel(t)
	expected := &graph.BusinessContract.Branches[0].Expected
	expected.Mode, expected.Reason = "allowed_outcomes", "test spec explicitly permits asynchronous pending or committed result"
	expected.Outcomes = append(expected.Outcomes, BusinessExpectedOutcome{ID: "queued", State: "pending", Denied: businessBool(false), Predicate: BusinessPredicate{Op: "true"}})
	expected.CommonInvariants = []BusinessExpectedCheck{{ID: "funds-preserved", Kind: "invariant", Oracle: "db", SourceRef: "test:funds", Predicate: businessPredicate("gte", "order", "balance", "integer", "0")}}
	decision := FreezeBusinessExpectation(graph, "enroll", contractObservation(true))
	if decision.Status != "READY" {
		t.Fatal(decision)
	}
	after := contractObservation(true)
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "PASS" || got.Outcome != "queued" {
		t.Fatal(got)
	}
	after.Values["order.balance"] = json.RawMessage("-1")
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "FAIL" {
		t.Fatalf("allowed outcome bypassed invariant: %+v", got)
	}
	after.Values["order.balance"] = json.RawMessage("10")
	delete(after.Sources, "order.balance")
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "BLOCKED" {
		t.Fatalf("oracle provenance missing but check passed: %+v", got)
	}
}

func TestBusinessContractNamedResponsePersistenceEventAndForbiddenSideEffects(t *testing.T) {
	graph := contractModel(t)
	outcome := &graph.BusinessContract.Branches[0].Expected.Outcomes[0]
	outcome.Checks = []BusinessExpectedCheck{
		{ID: "record-created", Kind: "persistence", Oracle: "db", SourceRef: "test:stored", Predicate: businessPredicate("eq", "order", "stored", "boolean", "true")},
		{ID: "exactly-one-event", Kind: "event", Oracle: "events", SourceRef: "test:event", Predicate: businessPredicate("eq", "order", "events", "integer", "1")},
		{ID: "no-extra-debit", Kind: "forbidden_side_effect", Oracle: "db", SourceRef: "test:no-extra-debit", Predicate: businessPredicate("eq", "order", "balance", "integer", "10")},
	}
	decision := FreezeBusinessExpectation(graph, "enroll", contractObservation(true))
	after := contractObservation(true)
	after.State, after.Values["order.stored"], after.Values["order.events"] = "ready", json.RawMessage("true"), json.RawMessage("1")
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "PASS" || len(got.Checks[0].Assertions) != 3 {
		t.Fatal(got)
	}
	after.Values["order.events"] = json.RawMessage("2")
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "FAIL" {
		t.Fatalf("duplicate event accepted: %+v", got)
	}
}

func TestBusinessContractBoundIdentityCannotDisappearOrChange(t *testing.T) {
	graph := contractModel(t)
	decision := FreezeBusinessExpectation(graph, "enroll", contractObservation(true))
	after := contractObservation(true)
	after.State = "ready"
	delete(after.Bindings, "other")
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "BLOCKED" {
		t.Fatal(got)
	}
	after.Bindings["other"] = "acct-C"
	if got := CompareBusinessExpectation(graph, *decision.Frozen, after, false); got.Status != "FAIL" {
		t.Fatalf("same shape, different account accepted: %+v", got)
	}
}

func TestBusinessContractFiniteProofUsesRelevantDomainsAndStatesBudget(t *testing.T) {
	graph := contractModel(t)
	graph.BusinessContract.Coverage.Domains = append(graph.BusinessContract.Coverage.Domains, BusinessFiniteDomain{Subject: "other", Attribute: "active", Values: []json.RawMessage{json.RawMessage("false"), json.RawMessage("true")}})
	proof := ProveBusinessCoverage(graph)
	if proof.Status != "PROVEN_IN_SCOPE" || !proof.CompleteInScope || proof.RequiredCases != 2 || proof.CheckedCases != 2 || proof.ProofKind != "finite_branch_partition" {
		t.Fatalf("irrelevant domain multiplied proof scope: %+v", proof)
	}
	graph.BusinessContract.Coverage.MaxCases = 1
	proof = ProveBusinessCoverage(graph)
	if proof.Status != "BUDGET_EXHAUSTED" || proof.CompleteInScope || proof.EnumerationComplete || proof.CheckedCases != 1 {
		t.Fatalf("budget prefix claimed complete: %+v", proof)
	}
	graph.BusinessContract.Coverage.MaxCases = 100
	graph.BusinessContract.Coverage.Scopes[0].States = append(graph.BusinessContract.Coverage.Scopes[0].States, "ready")
	proof = ProveBusinessCoverage(graph)
	if proof.Status != "SPECIFICATION_GAP" || proof.RequiredCases != 3 {
		t.Fatalf("explicit uncovered prestate omitted: %+v", proof)
	}
}

func TestBusinessContractMissingDomainsLegacyAndWrongTypesCannotClaimProof(t *testing.T) {
	graph := contractModel(t)
	graph.BusinessContract.Coverage.Domains = nil
	if got := ProveBusinessCoverage(graph); got.Status != "BLOCKED" || got.CompleteInScope {
		t.Fatal(got)
	}
	graph.BusinessContract.Coverage.Domains = []BusinessFiniteDomain{{Subject: "primary", Attribute: "active", Values: []json.RawMessage{json.RawMessage(`"false"`)}}}
	if err := ValidateBusinessContract(graph); err == nil {
		t.Fatal("ill-typed finite domain accepted")
	}
	graph.BusinessContract = nil
	if got := ProveBusinessCoverage(graph); got.Status != "LEGACY_UNDECLARED" {
		t.Fatal(got)
	}
	if got := FreezeBusinessExpectation(graph, "enroll", contractObservation(true)); got.Status != "LEGACY_UNDECLARED" || got.Frozen != nil {
		t.Fatal(got)
	}
}

func TestBusinessContractValidationRejectsUndeclaredUnsourcedAndImplicitNondeterminism(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Graph)
	}{
		{"unknown-ref", func(g *Graph) { g.BusinessContract.Branches[0].When.Left.Ref.Attribute = "missing" }},
		{"unsourced", func(g *Graph) { g.BusinessContract.Branches[0].Source = "inferred" }},
		{"missing-prestate", func(g *Graph) { g.BusinessContract.Branches[0].FromStates = nil }},
		{"two-deterministic-outcomes", func(g *Graph) {
			g.BusinessContract.Branches[0].Expected.Outcomes = append(g.BusinessContract.Branches[0].Expected.Outcomes, BusinessExpectedOutcome{ID: "unexpected", State: "pending", Predicate: BusinessPredicate{Op: "true"}})
		}},
		{"implicit-allowed-set", func(g *Graph) { g.BusinessContract.Branches[0].Expected.Mode = "allowed_outcomes" }},
		{"cross-type", func(g *Graph) {
			g.BusinessContract.Branches[0].When.Right.Literal = &BusinessLiteral{Type: "string", Value: json.RawMessage(`"true"`)}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := contractModel(t)
			test.mutate(&g)
			if err := validateGraph(g); err == nil {
				t.Fatal("invalid deterministic extension passed existing graph import gate")
			}
		})
	}
}

func TestBusinessContractStrictJSONRoundTripAndNoV1Silencing(t *testing.T) {
	graph := contractModel(t)
	body, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := loadGraph(writeBusinessModel(t, string(body)))
	if err != nil || decoded.BusinessContract == nil || len(decoded.BusinessContract.Branches) != 2 {
		t.Fatalf("contract extension lost through existing graph loader: %v", err)
	}
	wrong := strings.Replace(string(body), `"confirmed":true`, `"confirmd":true`, 1)
	if _, err := loadGraph(writeBusinessModel(t, wrong)); err == nil {
		t.Fatal("misspelled confirmation silently accepted")
	}
	legacy := sample()
	legacy.BusinessContract = graph.BusinessContract
	if err := validateGraph(legacy); err == nil {
		t.Fatal("v1 graph silently ignored deterministic contract")
	}
}

func TestBusinessContractD02ExistingGraphAcceptsExplicitPerPrestateTargets(t *testing.T) {
	graph := contractModel(t)
	graph.Edges = append(graph.Edges, Edge{ID: "ready-to-pending", From: "ready", To: "pending", Kind: "transitions_to"})
	graph.Transitions = append(graph.Transitions, TransitionContract{Edge: "ready-to-pending", Action: "enroll", Rule: "R1", Preconditions: ConditionAssignment{"login": true}, Outcome: branchSuccess})
	branch, _ := cloneBusinessJSON(graph.BusinessContract.Branches[0])
	branch.ID, branch.Transition, branch.FromStates, branch.Expected.Outcomes[0].State = "reset-after-ready", "ready-to-pending", []string{"ready"}, "pending"
	graph.BusinessContract.Branches = append(graph.BusinessContract.Branches, branch)
	if err := validateGraph(graph); err != nil {
		t.Fatalf("typed per-prestate expected target was forced back to legacy Rule.Expected: %v", err)
	}
	graph.BusinessContract = nil
	if err := validateGraph(graph); err == nil {
		t.Fatal("legacy global expected guard was weakened without an explicit typed transition contract")
	}
}

func TestBusinessContractUnknownPrestateAndFlowCannotBecomeSpecificationGap(t *testing.T) {
	graph := contractModel(t)
	graph.BusinessContract.Branches = graph.BusinessContract.Branches[:1]
	before := contractObservation(false) // The only guard is known false.
	before.State = "UNKNOWN"
	if decision := FreezeBusinessExpectation(graph, "enroll", before); decision.Status != "BLOCKED" || !containsBusinessID(decision.Unknown, "state") {
		t.Fatalf("unknown prestate was treated as a proven uncovered case: %+v", decision)
	}
	before.State, before.Flow = "pending", ""
	if decision := FreezeBusinessExpectation(graph, "enroll", before); decision.Status != "BLOCKED" || !containsBusinessID(decision.Unknown, "flow") {
		t.Fatalf("unknown flow was silently inferred: %+v", decision)
	}
}
