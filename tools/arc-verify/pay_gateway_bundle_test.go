package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This checks the shipped Python generator against the actual Go protocol. It
// does not contact a gateway or create product execution evidence.
func TestPayGatewayBundleAcceptedByRunnerProtocol(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("pay example requires Python 3.11+")
	}
	graph := payGatewayExampleModel(t)
	dir := t.TempDir()
	session := map[string]any{
		"case_id": "bundle_contract", "scope": "pay-owned-bundle-contract", "caller_id": "caller-contract",
		"pay_repo": dir, "payment_no": "contract_payment", "refund_no": "contract_refund",
		"target": "http://127.0.0.1:23456", "notify_url": "https://merchant.example/notify", "client_ip": "203.0.113.1",
	}
	body, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(dir, "session.json")
	if err := os.WriteFile(sessionPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"core", "refund", "create", "missing-parent", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			out := filepath.Join(dir, scenario)
			command := exec.Command(python, "examples/pay-gateway/bundle.py", "--session", sessionPath, "--out", out, "--scenario", scenario)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("bundle generation failed: %v\n%s", err, output)
			}
			var manifest RunnerManifest
			readPayBundleJSON(t, filepath.Join(out, "runner.json"), &manifest)
			if _, err := validateManifest(graph, manifest); err != nil {
				t.Fatalf("shipped runner cannot execute against shipped model: %v", err)
			}
			var fixtures Fixtures
			readPayBundleJSON(t, filepath.Join(out, "fixtures.json"), &fixtures)
			if err := validateFixtures(fixtures); err != nil {
				t.Fatal(err)
			}
			if len(fixtures.Fixtures) != 2 || fixtures.Fixtures[0].SecretRef != "env://ARC_PAY_CALLER_KEY" || fixtures.Fixtures[1].State != "EMPTY" {
				t.Fatalf("fixture declaration inferred target state or embedded credentials: %+v", fixtures)
			}
			for _, step := range manifest.Steps {
				if step.BusinessBranch == "" || step.Call.Kind != adapterHTTP || step.InputMeasurements["request.amount"] != "/amount" {
					t.Fatalf("action lost its pre-dispatch business branch/request capture: %+v", step)
				}
				if step.Oracles[0].Call.Kind != adapterCommand || step.Oracles[0].Measurements["request.amount"] != "" {
					t.Fatal("request input cannot come from the SQL observer")
				}
			}
			if manifest.Recovery == nil || manifest.Recovery.Mode != recoveryCheckpoint || len(manifest.Recovery.Steps) != 0 {
				t.Fatal("payment history must be retained behind independently verified checkpoint")
			}
			if scenario == "core" || scenario == "refund" {
				if manifest.Preparation == nil || len(manifest.Preparation.Adapters) != 2 {
					t.Fatal("refund preparation lost its two declared real actions")
				}
			} else if manifest.Preparation != nil {
				t.Fatal("scenario must not silently repair a mismatched requested pre-state")
			}
			if scenario == "core" && (len(manifest.Steps) != 2 || manifest.Steps[0].BusinessBranch != "BR_CREATE_REFUND_INDEX" || manifest.Steps[1].BusinessBranch != "BR_DUPLICATE_REFUND") {
				t.Fatal("core sequence must retain the same refund across both frozen branches")
			}
			if scenario == "duplicate" && (len(manifest.Setup) != 1 || len(manifest.Setup[0].Call.Extract) != 2) {
				t.Fatal("continuation must independently rebind both actual entities")
			}
		})
	}
}

func readPayBundleJSON(t *testing.T, path string, out any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeUniqueJSON(body, out); err != nil {
		t.Fatal(err)
	}
}
