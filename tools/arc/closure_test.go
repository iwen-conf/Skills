package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initClosureRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Arc Test"},
		{"config", "user.email", "arc-test@example.invalid"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("git %v: %v %s",args,err,out) }
	}
	if err := os.WriteFile(filepath.Join(root,"README.txt"), []byte("initial\n"),0600);err!=nil{t.Fatal(err)}
	for _, args := range [][]string{{"add","README.txt"},{"commit","-qm","initial"}} {
		cmd:=exec.Command("git",append([]string{"-C",root},args...)...)
		if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("git %v: %v %s",args,err,out)}
	}
}

func TestOutboxFailureRaceAndIdempotentRecovery(t *testing.T) {
	root, db := testArcState(t)
	initClosureRepo(t, root)
	if err:=executeTask(db,"create","","a pending task","",0);err!=nil{t.Fatal(err)}
	if err:=db.Close();err!=nil{t.Fatal(err)}
	countCalls:=0
	failBuild:=func(context.Context,string)error{countCalls++;return errors.New("index backend unavailable")}
	if err:=flushIndexOutboxWith(root,failBuild);err==nil{t.Fatal("failure incorrectly acknowledged events")}
	check:=func(wantStatus string,wantAttempts int){
		t.Helper()
		d,err:=openActiveArcDB(root);if err!=nil{t.Fatal(err)};defer d.Close()
		var status string;var attempts int
		if err:=d.QueryRow("SELECT status, attempts FROM arc_outbox LIMIT 1").Scan(&status,&attempts);err!=nil{t.Fatal(err)}
		if status!=wantStatus||attempts!=wantAttempts{t.Fatalf("outbox status=%s attempts=%d want=%s/%d",status,attempts,wantStatus,wantAttempts)}
	}
	check("pending",1)
	changeDuringIndex:=func(context.Context,string)error{
		countCalls++
		return os.WriteFile(filepath.Join(root,"README.txt"),[]byte("changed during indexing\n"),0600)
	}
	if err:=flushIndexOutboxWith(root,changeDuringIndex);err==nil{t.Fatal("source race incorrectly acknowledged events")}
	check("pending",2)
	okBuild:=func(context.Context,string)error{countCalls++;return nil}
	if err:=flushIndexOutboxWith(root,okBuild);err!=nil{t.Fatal(err)}
	check("delivered",3)
	if err:=flushIndexOutboxWith(root,okBuild);err!=nil{t.Fatal(err)}
	if countCalls!=3{t.Fatalf("idempotent no-pending flush ran backend: %d",countCalls)}
}

func testPolicyReceipt(taskID, verdict string) policyTaskStatus {
	status:=policyTaskStatus{ReceiptOK:true}
	status.Task.ID=taskID
	receipt:=map[string]any{
		"task_id":taskID,"verdict":verdict,"created_at":"2026-11-09T00:00:00Z",
		"head":"abc","digest":"source-digest","tree":"git-tree",
		"checks":[]map[string]any{
			{"id":"snapshot","verdict":"PASS"},
			{"id":"cli_receipt","verdict":"PASS"},
			{"id":"tests","verdict":"PASS"},
		},
	}
	status.Task.Receipt,_=json.Marshal(receipt)
	return status
}

func TestTaskProofRejectsAuthorizationAndStaleness(t *testing.T) {
	const id="policy-task"
	for _,tc:=range []struct{ name string; status policyTaskStatus; valid bool }{
		{"PASS",testPolicyReceipt(id,"PASS"),true},
		{"waiver-not-test",testPolicyReceipt(id,"AUTHORIZED"),false},
		{"failed-tests",testPolicyReceipt(id,"FAIL"),false},
		{"wrong-task",testPolicyReceipt("another-task","PASS"),false},
		{"stale-tree",func()policyTaskStatus{s:=testPolicyReceipt(id,"PASS");s.ReceiptOK=false;return s}(),false},
	}{
		t.Run(tc.name,func(t *testing.T){
			proof,err:=requirePolicyProof(id,tc.status)
			if (err==nil)!=tc.valid{t.Fatalf("valid=%v error=%v",tc.valid,err)}
			if tc.valid&&proof.ReceiptSHA256==""{t.Fatal("PASS not fingerprinted")}
		})
	}
}

func TestCompletionRequiresConfirmedOutboxAndBoundRevision(t *testing.T) {
	root,db:=testArcState(t)
	initClosureRepo(t,root)
	const id="policy-task-final"
	if err:=executeTask(db,"ensure",id,"final task (arc-code)","",0);err!=nil{t.Fatal(err)}
	for _,step:=range []struct{to string;version int}{{"ready",1},{"running",2},{"verifying",3}}{
		if err:=executeTask(db,"transition",id,"",step.to,step.version);err!=nil{t.Fatal(err)}
	}
	proof,err:=requirePolicyProof(id,testPolicyReceipt(id,"PASS"));if err!=nil{t.Fatal(err)}
	proof.SourceSnapshot,err=sourceSnapshot(root);if err!=nil{t.Fatal(err)}
	if err:=commitTaskCompletion(db,root,id,4,proof);err==nil{t.Fatal("completion with pending events succeeded")}
	if err:=db.Close();err!=nil{t.Fatal(err)}
	if err:=flushIndexOutboxWith(root,func(context.Context,string)error{return nil});err!=nil{t.Fatal(err)}
	db,err=openActiveArcDB(root);if err!=nil{t.Fatal(err)};defer db.Close()
	changedProof:=proof
	if err:=os.WriteFile(filepath.Join(root,"README.txt"),[]byte("new changes\n"),0600);err!=nil{t.Fatal(err)}
	if err:=commitTaskCompletion(db,root,id,4,changedProof);err==nil{t.Fatal("stale revision was accepted")}
	// The pending index batch was confirmed; restore the previously proven source.
	if err:=os.WriteFile(filepath.Join(root,"README.txt"),[]byte("initial\n"),0600);err!=nil{t.Fatal(err)}
	if err:=commitTaskCompletion(db,root,id,4,proof);err!=nil{t.Fatal(err)}
	var state string;var version int
	if err:=db.QueryRow("SELECT status,version FROM arc_tasks WHERE id=?",id).Scan(&state,&version);err!=nil{t.Fatal(err)}
	if state!="done"||version!=5{t.Fatalf("completion state=%s version=%d",state,version)}
	var completed int
	if err:=db.QueryRow("SELECT COUNT(*) FROM arc_events WHERE aggregate_id=? AND kind='task.completed'",id).Scan(&completed);err!=nil{t.Fatal(err)}
	if completed!=1{t.Fatalf("completion events=%d want one",completed)}
	if err:=commitTaskCompletion(db,root,id,4,proof);err==nil{t.Fatal("CAS allowed duplicate completion")}
}
