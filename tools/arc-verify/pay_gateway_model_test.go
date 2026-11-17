package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const payGatewayExampleRevision = "e4f054fc7bb06e091be62d6765bb29b4348ad651"

func payGatewayExampleModel(t *testing.T) Graph {
	t.Helper()
	graph, err := loadGraph("examples/pay-gateway/model.json")
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// These observations are synthetic inputs to the model kernel. They verify the
// example's semantics, not pay_gateway, PostgreSQL, the adapter, or real funds.
func payGatewayExampleObservation(state string) BusinessObservation {
	payment, refund := state != "EMPTY", state == "REFUND_INDEXED"
	values := map[string]any{
		"actor.enabled": true, "account.enabled": true, "environment.controlled": true,
		"payment.exists": payment, "refund.exists": refund,
		"payment.rows": 0, "refund.rows": 0, "payment.id": "", "refund.id": "",
		"payment.amount": nil, "payment.currency": nil, "payment.channel_account_id": nil, "payment.provider": nil,
		"refund.amount": nil, "refund.currency": nil, "refund.channel_account_id": nil, "refund.provider": nil,
		"refund.notifications": 0, "refund.callbacks": 0,
		"provider.payment_dispatches": 0, "provider.refund_dispatches": 0,
		"provider.payment_received": 0, "provider.refund_received": 0, "provider.payment_confirmed": false,
		"provider.unexpected_receipts": 0,
		"relation.parent_valid":        false, "relation.same_caller": false, "relation.same_account": false,
		"request.amount": 100, "request.currency": "CNY", "request.channel_account_id": "ca_browser_huiju",
	}
	bindings := map[string]string{"actor": "caller-owned-fixture"}
	if payment {
		values["payment.rows"], values["payment.id"] = 1, "payment-existing-id"
		values["payment.amount"], values["payment.currency"] = 100, "CNY"
		values["payment.channel_account_id"], values["payment.provider"] = "ca_browser_huiju", "huiju"
		values["provider.payment_dispatches"], values["provider.payment_received"] = 1, 1
		values["provider.payment_confirmed"] = true
		bindings["payment"] = "payment-existing-id"
	}
	if refund {
		values["refund.rows"], values["refund.id"] = 1, "refund-existing-id"
		values["refund.amount"], values["refund.currency"] = 100, "CNY"
		values["refund.channel_account_id"], values["refund.provider"] = "ca_browser_huiju", "huiju"
		values["provider.refund_dispatches"], values["provider.refund_received"] = 1, 1
		values["relation.parent_valid"], values["relation.same_caller"], values["relation.same_account"] = true, true, true
		bindings["refund"] = "refund-existing-id"
	}
	observed := BusinessObservation{State: state, Bindings: bindings, Values: map[string]json.RawMessage{}, Sources: map[string][]string{}}
	for key, value := range values {
		observed.Values[key], _ = json.Marshal(value)
		observed.Sources[key] = []string{"pay-store"}
		if strings.HasPrefix(key, "request.") {
			observed.Sources[key] = []string{"request-capture"}
		}
	}
	return observed
}

func payGatewayExampleSet(observed *BusinessObservation, key string, value any, source string) {
	observed.Values[key], _ = json.Marshal(value)
	observed.Sources[key] = []string{source}
}

func payGatewayExampleActual(branch string) (BusinessObservation, bool) {
	state, status, denied := "REFUND_INDEXED", 200, false
	switch branch {
	case "BR_CREATE_PAYMENT_INDEX":
		state = "PAYMENT_INDEXED"
	case "BR_DUPLICATE_REFUND":
		status, denied = 409, true
	case "BR_MISSING_PARENT":
		state, status, denied = "EMPTY", 404, true
	}
	after := payGatewayExampleObservation(state)
	payGatewayExampleSet(&after, "transport.status_code", status, "response-capture")
	switch branch {
	case "BR_CREATE_PAYMENT_INDEX":
		// A reference index and HTTP 200 do not assert payment funding status.
		payGatewayExampleSet(&after, "provider.payment_confirmed", false, "pay-store")
		payGatewayExampleSet(&after, "reply.order_id", "payment-existing-id", "response-capture")
	case "BR_CREATE_REFUND_INDEX":
		payGatewayExampleSet(&after, "reply.status", "processing", "response-capture")
		payGatewayExampleSet(&after, "reply.order_id", "refund-existing-id", "response-capture")
	case "BR_DUPLICATE_REFUND":
		payGatewayExampleSet(&after, "reply.code", "duplicate_order", "response-capture")
		payGatewayExampleSet(&after, "reply.resource_id", "refund-existing-id", "response-capture")
	case "BR_MISSING_PARENT":
		payGatewayExampleSet(&after, "reply.code", "resource_not_found", "response-capture")
	}
	return after, denied
}

func payGatewayExampleFreeze(t *testing.T, graph Graph, branch string) FrozenBusinessExpectation {
	t.Helper()
	state := "EMPTY"
	if branch == "BR_CREATE_REFUND_INDEX" {
		state = "PAYMENT_INDEXED"
	} else if branch == "BR_DUPLICATE_REFUND" {
		state = "REFUND_INDEXED"
	}
	decision := FreezeBusinessBranch(graph, branch, payGatewayExampleObservation(state))
	if decision.Status != "READY" || decision.Frozen == nil {
		t.Fatalf("cannot freeze %s: %+v", branch, decision)
	}
	return *decision.Frozen
}

func TestPayGatewayModelFiniteProofIsFourExplicitFixtureCases(t *testing.T) {
	graph := payGatewayExampleModel(t)
	proof := ProveBusinessCoverage(graph)
	if proof.Status != "PROVEN_IN_SCOPE" || !proof.CompleteInScope || !proof.EnumerationComplete || proof.CheckedCases != 4 || proof.RequiredCases != 4 {
		t.Fatalf("fixture partition was not proven in the declared four-case scope: %+v", proof)
	}
	if !strings.Contains(graph.Scope, payGatewayExampleRevision) || !strings.Contains(proof.Scope, "有限") {
		t.Fatalf("proof lost pinned, finite scope: graph=%s proof=%s", graph.Scope, proof.Scope)
	}
	wantStates := map[string]bool{"EMPTY": true, "PAYMENT_INDEXED": true, "REFUND_INDEXED": true}
	for _, node := range graph.Nodes {
		if node.Kind == "state" {
			if !wantStates[node.ID] {
				t.Fatalf("index fixture introduced a funding state: %+v", node)
			}
			delete(wantStates, node.ID)
		}
	}
	if len(wantStates) != 0 {
		t.Fatal(wantStates)
	}
	for _, rule := range graph.Rules {
		if !reflect.DeepEqual(rule.Conditions, []string{"C_FIXTURE_SCOPE"}) {
			t.Fatalf("gateway rule acquired an invented paid/balance precondition: %+v", rule)
		}
	}
	if len(graph.Risks) != 4 {
		t.Fatalf("out-of-scope funding/callback/recovery/boundary risks disappeared: %+v", graph.Risks)
	}
}

func TestPayGatewayModelSourcesStayPinnedAndPaidPreparationIsFixtureSpecific(t *testing.T) {
	graph := payGatewayExampleModel(t)
	body, _ := json.Marshal(graph)
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	var visit func(any)
	count := 0
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, child := range node {
				if key == "source_ref" {
					count++
					ref, ok := child.(string)
					if !ok || !strings.Contains(ref, "/blob/"+payGatewayExampleRevision+"/") || strings.Contains(ref, "/blob/main/") {
						t.Errorf("nonreproducible or missing source: %v", child)
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(document)
	if count == 0 {
		t.Fatal("model has no explicit source references")
	}
	for _, preparation := range graph.BusinessContract.Preparation {
		if preparation.ID == "PREP_CONFIRM_PAYMENT" {
			if !strings.Contains(preparation.SourceRef, "tests/fixtures/ratio_providers.rs") || !strings.Contains(preparation.SourceRef, "tests/fixtures/browser_host.rs") || !reflect.DeepEqual(preparation.DependsOn, []string{"PREP_CREATE_PAYMENT"}) {
				t.Fatalf("controlled institution requirement lost its implementation source or dependency: %+v", preparation)
			}
		}
	}
	for _, branch := range graph.BusinessContract.Branches {
		usesConfirmation := containsBusinessID(businessPredicateRefs(branch.When), "provider.payment_confirmed")
		if usesConfirmation != (branch.ID == "BR_CREATE_REFUND_INDEX") {
			t.Fatalf("fixture confirmation spread to a gateway-wide precondition: %s", branch.ID)
		}
		if usesConfirmation && !strings.Contains(branch.SourceRef, "tests/fixtures/ratio_providers.rs") {
			t.Fatal("successful refund guard is not sourced as an institution fixture limitation")
		}
	}
}

func TestPayGatewayModelSelectsActualPrestateAndCapturedRequest(t *testing.T) {
	graph := payGatewayExampleModel(t)
	tests := []struct {
		name, state, action, status, branch string
		mutate                              func(*BusinessObservation)
	}{
		{"create", "EMPTY", "CREATE_PAYMENT", "READY", "BR_CREATE_PAYMENT_INDEX", nil},
		{"refund", "PAYMENT_INDEXED", "SUBMIT_REFUND", "READY", "BR_CREATE_REFUND_INDEX", nil},
		{"duplicate", "REFUND_INDEXED", "SUBMIT_REFUND", "READY", "BR_DUPLICATE_REFUND", nil},
		{"missing_parent", "EMPTY", "SUBMIT_REFUND", "READY", "BR_MISSING_PARENT", nil},
		{"unconfirmed_fixture_has_no_success_claim", "PAYMENT_INDEXED", "SUBMIT_REFUND", "SPECIFICATION_GAP", "", func(o *BusinessObservation) {
			payGatewayExampleSet(o, "provider.payment_confirmed", false, "pay-store")
		}},
		{"confirmation_unknown", "PAYMENT_INDEXED", "SUBMIT_REFUND", "BLOCKED", "", func(o *BusinessObservation) { delete(o.Values, "provider.payment_confirmed") }},
		{"duplicate_does_not_invent_paid_guard", "REFUND_INDEXED", "SUBMIT_REFUND", "READY", "BR_DUPLICATE_REFUND", func(o *BusinessObservation) {
			payGatewayExampleSet(o, "provider.payment_confirmed", false, "pay-store")
		}},
		{"different_actual_amount", "EMPTY", "CREATE_PAYMENT", "SPECIFICATION_GAP", "", func(o *BusinessObservation) { payGatewayExampleSet(o, "request.amount", 101, "request-capture") }},
		{"different_actual_currency", "EMPTY", "CREATE_PAYMENT", "SPECIFICATION_GAP", "", func(o *BusinessObservation) { payGatewayExampleSet(o, "request.currency", "USD", "request-capture") }},
		{"different_actual_account", "EMPTY", "CREATE_PAYMENT", "SPECIFICATION_GAP", "", func(o *BusinessObservation) {
			payGatewayExampleSet(o, "request.channel_account_id", "other", "request-capture")
		}},
		{"request_not_observed", "EMPTY", "CREATE_PAYMENT", "BLOCKED", "", func(o *BusinessObservation) { delete(o.Values, "request.amount") }},
		{"json_string_is_not_integer", "EMPTY", "CREATE_PAYMENT", "BLOCKED", "", func(o *BusinessObservation) { o.Values["request.amount"] = json.RawMessage(`"100"`) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := payGatewayExampleObservation(test.state)
			if test.mutate != nil {
				test.mutate(&before)
			}
			decision := FreezeBusinessExpectation(graph, test.action, before)
			if decision.Status != test.status || decision.Branch != test.branch {
				t.Fatalf("got %+v, want %s/%s", decision, test.status, test.branch)
			}
		})
	}
	decision := FreezeBusinessBranch(graph, "BR_CREATE_REFUND_INDEX", payGatewayExampleObservation("REFUND_INDEXED"))
	if decision.Status != "BLOCKED" || decision.Frozen != nil || decision.Branch != "BR_DUPLICATE_REFUND" {
		t.Fatalf("planned first refund displaced observed existing refund: %+v", decision)
	}
}

func TestPayGatewayModelResponseAndIndependentPersistenceMustBothMatch(t *testing.T) {
	graph := payGatewayExampleModel(t)
	for _, branch := range []string{"BR_CREATE_PAYMENT_INDEX", "BR_CREATE_REFUND_INDEX", "BR_DUPLICATE_REFUND", "BR_MISSING_PARENT"} {
		t.Run(branch, func(t *testing.T) {
			frozen := payGatewayExampleFreeze(t, graph, branch)
			after, denied := payGatewayExampleActual(branch)
			if comparison := CompareBusinessExpectation(graph, frozen, after, denied); comparison.Status != "PASS" {
				t.Fatalf("declared example result rejected: %+v", comparison)
			}
		})
	}
	tests := []struct {
		name, branch, key string
		value             any
	}{
		{"payment_wrong_amount", "BR_CREATE_PAYMENT_INDEX", "payment.amount", 99},
		{"payment_wrong_currency", "BR_CREATE_PAYMENT_INDEX", "payment.currency", "USD"},
		{"payment_wrong_account", "BR_CREATE_PAYMENT_INDEX", "payment.channel_account_id", "other"},
		{"payment_wrong_provider", "BR_CREATE_PAYMENT_INDEX", "payment.provider", "other"},
		{"refund_wrong_amount", "BR_CREATE_REFUND_INDEX", "refund.amount", 99},
		{"refund_wrong_currency", "BR_CREATE_REFUND_INDEX", "refund.currency", "USD"},
		{"refund_wrong_account", "BR_CREATE_REFUND_INDEX", "refund.channel_account_id", "other"},
		{"refund_wrong_provider", "BR_CREATE_REFUND_INDEX", "refund.provider", "other"},
		{"refund_unrelated_parent", "BR_CREATE_REFUND_INDEX", "relation.parent_valid", false},
		{"refund_cross_caller", "BR_CREATE_REFUND_INDEX", "relation.same_caller", false},
		{"refund_wrong_response_id", "BR_CREATE_REFUND_INDEX", "reply.order_id", "other-refund"},
		{"refund_not_processing", "BR_CREATE_REFUND_INDEX", "reply.status", "succeeded"},
		{"duplicate_wrong_resource_id", "BR_DUPLICATE_REFUND", "reply.resource_id", "other-refund"},
		{"duplicate_wrong_code", "BR_DUPLICATE_REFUND", "reply.code", "different_error"},
		{"duplicate_redispatched", "BR_DUPLICATE_REFUND", "provider.refund_dispatches", 2},
		{"duplicate_double_received", "BR_DUPLICATE_REFUND", "provider.refund_received", 2},
		{"duplicate_inserted_second_row", "BR_DUPLICATE_REFUND", "refund.rows", 2},
		{"missing_parent_dispatched", "BR_MISSING_PARENT", "provider.refund_dispatches", 1},
		{"missing_parent_inserted_row", "BR_MISSING_PARENT", "refund.rows", 1},
		{"create_wrong_institution_or_request", "BR_CREATE_PAYMENT_INDEX", "provider.unexpected_receipts", 1},
		{"refund_wrong_institution_or_request", "BR_CREATE_REFUND_INDEX", "provider.unexpected_receipts", 1},
		{"duplicate_unbound_creation_receipt", "BR_DUPLICATE_REFUND", "provider.unexpected_receipts", 1},
		{"missing_parent_unindexed_outbound", "BR_MISSING_PARENT", "provider.unexpected_receipts", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frozen := payGatewayExampleFreeze(t, graph, test.branch)
			after, denied := payGatewayExampleActual(test.branch)
			source := "pay-store"
			if strings.HasPrefix(test.key, "reply.") {
				source = "response-capture"
			}
			payGatewayExampleSet(&after, test.key, test.value, source)
			if comparison := CompareBusinessExpectation(graph, frozen, after, denied); comparison.Status != "FAIL" || comparison.Branch != frozen.Branch {
				t.Fatalf("business violation did not fail the original expectation: %+v", comparison)
			}
		})
	}
	frozen := payGatewayExampleFreeze(t, graph, "BR_CREATE_REFUND_INDEX")
	after, denied := payGatewayExampleActual("BR_CREATE_REFUND_INDEX")
	after.Sources["refund.amount"] = []string{"response-capture"}
	if comparison := CompareBusinessExpectation(graph, frozen, after, denied); comparison.Status != "BLOCKED" {
		t.Fatalf("API self-report substituted for independent persisted amount: %+v", comparison)
	}
	after, denied = payGatewayExampleActual("BR_CREATE_REFUND_INDEX")
	after.Sources["refund.id"] = []string{"response-capture"}
	if comparison := CompareBusinessExpectation(graph, frozen, after, denied); comparison.Status != "BLOCKED" {
		t.Fatalf("API self-report substituted for the persisted entity identity: %+v", comparison)
	}
}

func TestPayGatewayModelPreparationUsesObservedStoreWithoutAssumingRequestOrIDs(t *testing.T) {
	graph := payGatewayExampleModel(t)
	before := payGatewayExampleObservation("EMPTY")
	for key := range before.Values {
		if strings.HasPrefix(key, "request.") {
			delete(before.Values, key)
			delete(before.Sources, key)
		}
	}
	goal := BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		businessPredicate("eq", "payment", "exists", "boolean", "true"),
		businessPredicate("eq", "provider", "payment_confirmed", "boolean", "true"),
	}}
	request := PreparationRequest{Observed: before, Goal: goal, MaxSteps: 4, MaxStates: 16}
	for attempt := 0; attempt < 2; attempt++ {
		plan := PlanPreparation(graph, request)
		if plan.Status != "READY" || !plan.RequiresReadback || len(plan.Layers) != 2 || len(plan.Layers[0].Steps) != 1 || len(plan.Layers[1].Steps) != 1 || plan.Layers[0].Steps[0].ID != "PREP_CREATE_PAYMENT" || plan.Layers[1].Steps[0].ID != "PREP_CONFIRM_PAYMENT" {
			t.Fatalf("preparation did not find two real dependent actions from the unchanged observation: %+v", plan)
		}
		for _, layer := range plan.Layers {
			for _, step := range layer.Steps {
				for _, effect := range step.Effects {
					if effect.Attribute == "id" {
						t.Fatalf("planner fabricated a server-issued entity ID: %+v", effect)
					}
				}
			}
		}
	}
	if string(before.Values["payment.exists"]) != "false" || string(before.Values["provider.payment_confirmed"]) != "false" {
		t.Fatal("prediction mutated actual observation")
	}
	if decision := FreezeBusinessExpectation(graph, "SUBMIT_REFUND", before); decision.Status != "BLOCKED" {
		t.Fatalf("planning without an actual captured target request made the target executable: %+v", decision)
	}
}

func TestPayGatewayModelOffersContinuousSequenceAndExplicitCheckpointDuplicate(t *testing.T) {
	graph := payGatewayExampleModel(t)
	tests := map[string]TestDefinition{}
	for _, test := range graph.Tests {
		tests[test.ID] = test
	}
	chain := tests["TEST_REFUND_DUPLICATE_CHAIN"]
	if chain.Kind != "SEQUENCE" || !reflect.DeepEqual(chain.Steps, []string{"TR_CREATE_REFUND_INDEX", "TR_DUPLICATE_REFUND"}) || chain.Flow != "FLOW_REFUND_DUPLICATE" {
		t.Fatalf("continuous same-action/different-state sequence missing: %+v", chain)
	}
	duplicate := tests["TEST_DUPLICATE_REFUND"]
	if duplicate.Kind != "TRANSITION" || duplicate.Rule != "RULE_REFUND_INDEX" || !duplicate.Denied || duplicate.Expected != "UNCHANGED" || !reflect.DeepEqual(duplicate.Actors, []ActorRequirement{{Alias: "primary", Role: "CALLER"}}) {
		t.Fatalf("standalone observed checkpoint duplicate is not runnable: %+v", duplicate)
	}
	linked := false
	for _, edge := range graph.Edges {
		if edge.Kind == "verifies" && edge.From == duplicate.ID && edge.To == "TR_DUPLICATE_REFUND" {
			linked = true
		}
	}
	if !linked {
		t.Fatal("checkpoint case is not linked to its exact duplicate transition")
	}
	for _, branch := range graph.BusinessContract.Branches {
		if branch.ID == "BR_DUPLICATE_REFUND" && (!containsBusinessID(branch.Obligations, chain.ID) || !containsBusinessID(branch.Obligations, duplicate.ID)) {
			t.Fatal("duplicate branch lost one of its two explicitly scoped test obligations")
		}
	}
}
