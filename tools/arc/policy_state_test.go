package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testLegacyPolicyState(t *testing.T,root string,taskID string) []byte{
	t.Helper()
	dir:=filepath.Join(root,".arc","policy")
	if err:=os.MkdirAll(filepath.Join(dir,"receipts"),0700);err!=nil{t.Fatal(err)}
	status:=testPolicyReceipt(taskID,"PASS")
	body,err:=json.Marshal(map[string]any{
		"active":map[string]any{
			"id":taskID,"slug":"payment-fix","skill":"arc-code",
			"receipt":json.RawMessage(status.Task.Receipt),
		},
		"history":[]any{map[string]any{"id":"older","slug":"old-task","skill":"arc-code","receipt":nil}},
	})
	if err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(dir,"state.json"),body,0600);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(dir,"receipts","payment-fix.json"),status.Task.Receipt,0600);err!=nil{t.Fatal(err)}
	return body
}

func TestPolicyMigrationStoresActiveHistoryAndReceipts(t *testing.T){
	if runtime.GOOS=="windows"{t.Skip("Windows policy lock is fail-closed")}
	root,db:=testArcState(t)
	if err:=db.Close();err!=nil{t.Fatal(err)}
	const taskID="policy-migrate-task"
	raw:=testLegacyPolicyState(t,root,taskID)
	if err:=policyStateCommand([]string{"migrate","--repo",root});err==nil{t.Fatal("migration without offline confirmation was permitted")}
	if err:=policyStateCommand([]string{"migrate","--repo",root,"--confirm-offline"});err!=nil{t.Fatal(err)}
	db,err:=openActiveArcDB(root);if err!=nil{t.Fatal(err)};defer db.Close()
	doc,err:=loadPolicyDocument(db);if err!=nil{t.Fatal(err)}
	if doc.Mode!="arc.db"||doc.Version!=1{t.Fatalf("migration not activated: %+v",doc)}
	var current map[string]any
	if err:=json.Unmarshal(doc.State,&current);err!=nil{t.Fatal(err)}
	if current["active"].(map[string]any)["id"]!=taskID{t.Fatal("active task identity lost")}
	if len(current["history"].([]any))!=1{t.Fatal("historical tasks lost")}
	var count int
	if err:=db.QueryRow("SELECT COUNT(*) FROM arc_policy_receipts WHERE task_id=?",taskID).Scan(&count);err!=nil{t.Fatal(err)}
	if count!=1{t.Fatalf("receipt import duplicated/missing: %d",count)}
	var attested string
	if err:=db.QueryRow("SELECT value FROM arc_meta WHERE key='policy_migration_verified'").Scan(&attested);err!=nil{t.Fatal(err)}
	if len(attested)!=64{t.Fatal("migration parity has no durable attestation")}
	if err:=policyStateCommand([]string{"migrate","--repo",root,"--confirm-offline"});err!=nil{t.Fatal("idempotent migration finalization failed:",err)}
	legacyGuard:=filepath.Join(root,".arc","policy","state.json")
	st,err:=os.Lstat(legacyGuard);if err!=nil{t.Fatal(err)}
	if !st.IsDir(){t.Fatal("legacy state file was not replaced by a write-blocking tombstone")}
	if err:=os.WriteFile(legacyGuard,[]byte("{}"),0600);err==nil{t.Fatal("legacy state path remained writable after cutover")}
	if _,err:=os.Stat(filepath.Join(root,".arc","backups","policy-receipts-"));err==nil{t.Fatal("unexpected receipt archive name")}
	after,err:=loadPolicyDocument(db);if err!=nil{t.Fatal(err)}
	if string(after.State)!=string(doc.State){t.Fatal("DB state followed legacy JSON after cutover")}
	// Backup retained and byte-for-byte equal to original legacy source.
	entries,err:=filepath.Glob(filepath.Join(root,".arc","backups","policy-state-*.json"));if err!=nil{t.Fatal(err)}
	if len(entries)!=1{t.Fatalf("expected one policy JSON backup, got %d",len(entries))}
	backup,err:=os.ReadFile(entries[0]);if err!=nil{t.Fatal(err)}
	if string(backup)!=string(raw){t.Fatal("migration backup differs from original")}
	receiptArchives,err:=filepath.Glob(filepath.Join(root,".arc","backups","policy-receipts-*"));if err!=nil{t.Fatal(err)}
	if len(receiptArchives)!=1{t.Fatalf("expected legacy receipts archive, got %d",len(receiptArchives))}
	if _,err:=os.Stat(filepath.Join(receiptArchives[0],"payment-fix.json"));err!=nil{t.Fatal(err)}
}

func TestMigrationRetryRejectsMissingParityAttestation(t *testing.T){
	if runtime.GOOS=="windows"{t.Skip("Windows Policy lock intentionally blocked")}
	root,db:=testArcState(t)
	if err:=db.Close();err!=nil{t.Fatal(err)}
	testLegacyPolicyState(t,root,"policy-attestation")
	if err:=policyStateCommand([]string{"migrate","--repo",root,"--confirm-offline"});err!=nil{t.Fatal(err)}
	db,err:=openActiveArcDB(root);if err!=nil{t.Fatal(err)}
	if _,err:=db.Exec("DELETE FROM arc_meta WHERE key='policy_migration_verified'");err!=nil{t.Fatal(err)}
	db.Close()
	if err:=policyStateCommand([]string{"migrate","--repo",root,"--confirm-offline"});err==nil{
		t.Fatal("migration retry without receipt parity attestation was accepted")
	}
}

func TestPolicyDBStateCASAndReceiptRejection(t *testing.T){
	if runtime.GOOS=="windows"{t.Skip("Windows policy lock is fail-closed")}
	root,db:=testArcState(t);if err:=db.Close();err!=nil{t.Fatal(err)}
	const id="policy-cas-task"
	testLegacyPolicyState(t,root,id)
	if err:=policyStateCommand([]string{"migrate","--repo",root,"--confirm-offline"});err!=nil{t.Fatal(err)}
	db,err:=openActiveArcDB(root);if err!=nil{t.Fatal(err)};defer db.Close()
	before,err:=loadPolicyDocument(db);if err!=nil{t.Fatal(err)}
	write:=func(expected string,raw []byte)error{
		t.Helper()
		path:=filepath.Join(t.TempDir(),"input.json")
		if err:=os.WriteFile(path,raw,0600);err!=nil{t.Fatal(err)}
		f,err:=os.Open(path);if err!=nil{t.Fatal(err)}
		old:=os.Stdin;os.Stdin=f
		defer func(){os.Stdin=old;f.Close()}()
		return policyStateCommand([]string{"put","--repo",root,"--expected-version",expected})
	}
	var state map[string]any
	if err:=json.Unmarshal(before.State,&state);err!=nil{t.Fatal(err)}
	task:=state["active"].(map[string]any)
	task["receipt"]=nil
	raw,err:=json.Marshal(state);if err!=nil{t.Fatal(err)}
	if err:=write("1",raw);err!=nil{t.Fatal(err)}
	if err:=write("1",raw);err==nil{t.Fatal("stale CAS version silently accepted")}
	if err:=write("2",[]byte("null"));err==nil{t.Fatal("invalid policy state accepted")}
	after,err:=loadPolicyDocument(db);if err!=nil{t.Fatal(err)}
	if after.Version!=2{t.Fatalf("CAS version=%d, want 2",after.Version)}
	proof,err:=requirePolicyProof(id,testPolicyReceipt(id,"PASS"));if err!=nil{t.Fatal(err)}
	if err:=validateLivePolicyReceipt(root,proof);err==nil{t.Fatal("revoked DB receipt accepted because legacy JSON still contains PASS")}
}

func TestPolicyStoreDoesNotInitializeImplicitly(t *testing.T){
	_,db:=testArcState(t);defer db.Close()
	doc,err:=loadPolicyDocument(db);if err!=nil{t.Fatal(err)}
	if doc.Mode!="legacy" {t.Fatal("Policy unexpectedly migrated without an explicit command")}
	var tableCount int
	if err:=db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='arc_policy_state'").Scan(&tableCount);err!=nil{t.Fatal(err)}
	if tableCount!=0{t.Fatal("policy table was created without an explicit migration")}
}
