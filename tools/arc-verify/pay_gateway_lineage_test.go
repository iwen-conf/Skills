package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func payGatewayLineage(t *testing.T) (Graph, *BusinessLineage, map[string]BusinessBranchLineage) {
	t.Helper()
	graph := payGatewayExampleModel(t)
	lineage, err := buildBusinessLineage(graph)
	if err != nil {
		t.Fatal(err)
	}
	if lineage == nil || len(lineage.Branches) != 4 {
		t.Fatalf("the actual pay model must expose four distinct branch genealogies: %+v", lineage)
	}
	branches := map[string]BusinessBranchLineage{}
	for _, branch := range lineage.Branches {
		branches[branch.Branch] = branch
	}
	return graph, lineage, branches
}

func TestPayGatewayLineageExplainsOnlyDeclaredPreparationSources(t *testing.T) {
	graph, lineage, branches := payGatewayLineage(t)
	hash, err := businessJSONHash(graph)
	if err != nil || lineage.ContractSHA256 != hash {
		t.Fatalf("genealogy must remain bound to the exact reviewed contract: %s %v", lineage.ContractSHA256, err)
	}
	refund := branches["BR_CREATE_REFUND_INDEX"]
	nodes := map[string]BusinessLineageNode{}
	preparations := map[string]string{}
	for _, node := range refund.Nodes {
		nodes[node.ID] = node
		if node.Kind == "preparation" {
			preparations[node.Preparation] = node.ID
		}
		if !strings.Contains(node.SourceRef, "/blob/"+payGatewayExampleRevision+"/") {
			t.Fatalf("genealogy node lost the frozen source revision: %+v", node)
		}
	}
	if len(preparations) != 2 || preparations["PREP_CREATE_PAYMENT"] == "" || preparations["PREP_CONFIRM_PAYMENT"] == "" {
		t.Fatalf("refund needs the declared create/confirm preparation chain: %+v", preparations)
	}
	confirmed, createsParent, ordered := false, false, false
	for _, edge := range refund.Edges {
		if edge.Kind == "depends_on" && edge.From == preparations["PREP_CREATE_PAYMENT"] && edge.To == preparations["PREP_CONFIRM_PAYMENT"] {
			ordered = true
		}
		if edge.Kind != "candidate_provider" {
			continue
		}
		if !edge.Alternative {
			t.Fatalf("a candidate provider must not masquerade as a required or completed action: %+v", edge)
		}
		condition := nodes[edge.To]
		if condition.Predicate == nil {
			t.Fatalf("provider edge has no inspectable condition: %+v", edge)
		}
		for _, ref := range condition.References {
			key := ref.Subject + "." + ref.Attribute
			switch key {
			case "provider.payment_confirmed":
				if edge.From != preparations["PREP_CONFIRM_PAYMENT"] {
					t.Fatalf("the payment index alone cannot prove learned payment success: %+v", edge)
				}
				confirmed = true
			case "payment.exists":
				if edge.From != preparations["PREP_CREATE_PAYMENT"] {
					t.Fatalf("the original payment must come from its declared preparation: %+v", edge)
				}
				createsParent = true
			case "actor.enabled", "account.enabled", "environment.controlled", "request.amount", "request.currency", "request.channel_account_id":
				t.Fatalf("preparation fabricated an external observation or the actual request input: %s %+v", key, edge)
			}
		}
	}
	if !confirmed || !createsParent || !ordered {
		t.Fatalf("refund ancestry cannot explain create -> confirm -> required success: confirmed=%v parent=%v ordered=%v", confirmed, createsParent, ordered)
	}
	for _, id := range []string{"BR_CREATE_PAYMENT_INDEX", "BR_DUPLICATE_REFUND", "BR_MISSING_PARENT"} {
		for _, node := range branches[id].Nodes {
			if node.Kind == "preparation" {
				t.Fatalf("%s has no declared missing-state preparation; export invented %s", id, node.Preparation)
			}
		}
	}
}

func TestPayGatewayLineageRetainsRejectionStatesWithoutTaskCycles(t *testing.T) {
	graph, _, branches := payGatewayLineage(t)
	loops := map[string]string{}
	for _, edge := range graph.Edges {
		if edge.Kind == "transitions_to" && edge.From == edge.To {
			loops[edge.ID] = edge.From
		}
	}
	if !reflect.DeepEqual(loops, map[string]string{"TR_DUPLICATE_REFUND": "REFUND_INDEXED", "TR_MISSING_PARENT": "EMPTY"}) {
		t.Fatalf("the source model no longer contains its two legal rejection loops: %+v", loops)
	}
	for _, branch := range graph.BusinessContract.Branches {
		view := branches[branch.ID]
		states, target := 0, false
		degree, children := map[string]int{}, map[string][]string{}
		for _, node := range view.Nodes {
			degree[node.ID] = 0
			if node.Kind == "state" {
				states++
				if !reflect.DeepEqual(node.FromStates, branch.FromStates) {
					t.Fatalf("branch %s lost the actual required index state: %+v", branch.ID, node)
				}
			}
			target = target || node.ID == view.Target && node.Kind == "target" && node.Action == branch.Action
		}
		for _, edge := range view.Edges {
			if _, ok := degree[edge.From]; !ok {
				t.Fatalf("dependency has no source node: %+v", edge)
			}
			if _, ok := degree[edge.To]; !ok {
				t.Fatalf("dependency has no destination node: %+v", edge)
			}
			degree[edge.To]++
			children[edge.From] = append(children[edge.From], edge.To)
		}
		queue := []string{}
		for id, count := range degree {
			if count == 0 {
				queue = append(queue, id)
			}
		}
		seen := 0
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			seen++
			for _, child := range children[id] {
				degree[child]--
				if degree[child] == 0 {
					queue = append(queue, child)
				}
			}
		}
		if states != 1 || !target || seen != len(view.Nodes) {
			t.Fatalf("%s mixed its business state loop into dependency scheduling: states=%d target=%v visited=%d/%d", branch.ID, states, target, seen, len(view.Nodes))
		}
	}
}

func TestPayGatewayLineageCLIExportsUnapprovedModelWithoutExecution(t *testing.T) {
	dir := t.TempDir()
	dbfile, out := filepath.Join(dir, "review.db"), filepath.Join(dir, "workbench")
	cliJSON(t, "init", "--db", dbfile)
	cliJSON(t, "import", "--db", dbfile, "--file", "examples/pay-gateway/model.json")
	cliJSON(t, "snapshot", "--db", dbfile, "--graph", "pay-gateway-index-fixture-v2",
		"--config", filepath.Join(dir, "missing-config.jsonc"), "--assets", snapshotAssets(t), "--out", out)
	snapshot := readSnapshot(t, out)
	if snapshot.BusinessLineage == nil || len(snapshot.BusinessLineage.Branches) != 4 {
		t.Fatal("the actual CLI export lost the pay model genealogy")
	}
	if snapshot.Model.Approved || snapshot.Meta.Environment != "" || snapshot.Meta.Round != "" || len(snapshot.Runs) != 0 || snapshot.AvailableRuns != 0 {
		t.Fatalf("model review acquired execution or approval: model=%v meta=%+v runs=%d/%d", snapshot.Model.Approved, snapshot.Meta, len(snapshot.Runs), snapshot.AvailableRuns)
	}
	for rule, approved := range snapshot.Model.Rules {
		if approved {
			t.Fatalf("source review approved business execution for %s", rule)
		}
	}
	if len(snapshot.Catalog.Rounds)+len(snapshot.Catalog.Runners)+len(snapshot.Catalog.Fixtures)+len(snapshot.Catalog.Runs)+len(snapshot.Catalog.Environments) != 0 {
		t.Fatalf("model review fabricated an account or execution environment: %+v", snapshot.Catalog)
	}
	if snapshot.Catalog.EnvironmentError == "" {
		t.Fatal("the unavailable execution environment must remain visible")
	}
	if snapshot.ContractAnalysis == nil || snapshot.ContractAnalysis.Status != "PROVEN_IN_SCOPE" || snapshot.ContractExecution == nil || snapshot.ContractExecution.Total != 4 || snapshot.ContractExecution.NotRun != 4 || snapshot.ContractExecution.Passed != 0 {
		t.Fatalf("finite model proof was confused with pay_gateway E2E: proof=%+v execution=%+v", snapshot.ContractAnalysis, snapshot.ContractExecution)
	}
}
