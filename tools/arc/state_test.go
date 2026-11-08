package main

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func testArcState(t *testing.T) (string, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".arc"), 0700); err != nil { t.Fatal(err) }
	db, err := openArcDB(statePath(root))
	if err != nil { t.Fatal(err) }
	if err := createArcSchema(db); err != nil { db.Close(); t.Fatal(err) }
	if err := writeManifest(root, arcManifest{Version:arcSchemaVersion, Active:true, DB:arcDBName}); err != nil {
		db.Close(); t.Fatal(err)
	}
	t.Cleanup(func(){ db.Close() })
	return root, db
}

func TestTaskStateAndEventAtomicity(t *testing.T) {
	_, db := testArcState(t)
	if err := executeTask(db,"create","","Payment recovery","",0);err!=nil{t.Fatal(err)}
	var id string
	if err:=db.QueryRow("SELECT id FROM arc_tasks LIMIT 1").Scan(&id);err!=nil{t.Fatal(err)}
	if err:=executeTask(db,"transition",id,"","ready",1);err!=nil{t.Fatal(err)}
	if err:=executeTask(db,"transition",id,"","running",1);err==nil{t.Fatal("stale version was accepted")}
	if err:=executeTask(db,"transition",id,"","running",2);err!=nil{t.Fatal(err)}
	if err:=executeTask(db,"transition",id,"","verifying",3);err!=nil{t.Fatal(err)}
	if err:=executeTask(db,"transition",id,"","done",4);err==nil{t.Fatal("done without independently bound proof was accepted")}
	var current string
	var version int
	if err:=db.QueryRow("SELECT status,version FROM arc_tasks WHERE id=?",id).Scan(&current,&version);err!=nil{t.Fatal(err)}
	if current!="verifying" || version!=4 {t.Fatalf("unexpected task state %s version %d",current,version)}
	var n, pending int
	if err:=db.QueryRow("SELECT count(*) FROM arc_events WHERE aggregate='task'").Scan(&n);err!=nil{t.Fatal(err)}
	if err:=db.QueryRow("SELECT count(*) FROM arc_outbox WHERE status='pending'").Scan(&pending);err!=nil{t.Fatal(err)}
	if n!=4 || pending!=4 {t.Fatalf("events=%d outbox=%d; should match committed transitions",n,pending)}
}

func TestFileHashReconciliationAndSymlinkBoundary(t *testing.T) {
	root,db:=testArcState(t)
	if err:=os.WriteFile(filepath.Join(root,"code.go"),[]byte("package test\n"),0600);err!=nil{t.Fatal(err)}
	if err:=executeFile(db,root,"code.go","");err!=nil{t.Fatal(err)}
	if err:=executeFile(db,root,"code.go","");err!=nil{t.Fatal(err)}
	var version int
	if err:=db.QueryRow("SELECT version FROM arc_files WHERE path='code.go'").Scan(&version);err!=nil{t.Fatal(err)}
	if version!=1{t.Fatalf("unchanged file has version %d",version)}
	if err:=os.WriteFile(filepath.Join(root,"code.go"),[]byte("package changed\n"),0600);err!=nil{t.Fatal(err)}
	if err:=executeFile(db,root,"code.go","");err!=nil{t.Fatal(err)}
	if err:=db.QueryRow("SELECT version FROM arc_files WHERE path='code.go'").Scan(&version);err!=nil{t.Fatal(err)}
	if version!=2{t.Fatalf("changed file version=%d",version)}
	if _,_,_,err:=observedFile(root,".arc/arc.db");err==nil{t.Fatal("internal arc files must not be indexed")}
	if runtime.GOOS!="windows" {
		if err:=os.Symlink(t.TempDir(),filepath.Join(root,"out"));err!=nil{t.Fatal(err)}
		if _,_,_,err:=observedFile(root,"out/absent.txt");err==nil{t.Fatal("path through external symlink accepted")}
	}
}

func TestGitWorktreeChangeIsIdempotent(t *testing.T) {
	root,db:=testArcState(t)
	runGit:=func(args ...string){t.Helper();cmd:=exec.Command("git",append([]string{"-C",root},args...)...);if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("git %v: %v %s",args,err,out)}}
	runGit("init")
	runGit("config","user.email","arc-test@example.invalid")
	runGit("config","user.name","Arc Test")
	if err:=os.WriteFile(filepath.Join(root,"tracked.txt"),[]byte("baseline"),0600);err!=nil{t.Fatal(err)}
	runGit("add","tracked.txt");runGit("commit","-m","baseline")
	if err:=os.WriteFile(filepath.Join(root,"tracked.txt"),[]byte("updated"),0600);err!=nil{t.Fatal(err)}
	if err:=collectChanges(db,root,"",".");err!=nil{t.Fatal(err)}
	if err:=collectChanges(db,root,"",".");err!=nil{t.Fatal(err)}
	var changeCount int
	if err:=db.QueryRow("SELECT count(*) FROM arc_changes").Scan(&changeCount);err!=nil{t.Fatal(err)}
	if changeCount!=1{t.Fatalf("duplicate ChangeSet created, count=%d",changeCount)}
	var op string
	if err:=db.QueryRow("SELECT operation FROM arc_change_files").Scan(&op);err!=nil{t.Fatal(err)}
	if op!="modified"{t.Fatalf("incorrect change operation %q",op)}
}

func TestCutoverManifestRejectsWrongVersion(t *testing.T) {
	root, db := testArcState(t)
	if _,err:=readManifest(root);err!=nil{t.Fatal(err)}
	db.Close()
	if err:=os.WriteFile(manifestPath(root),[]byte(`{"schema_version":999,"active":true,"db":".arc/arc.db"}`),0600);err!=nil{t.Fatal(err)}
	if _,err:=readManifest(root);err==nil{t.Fatal("invalid migration manifest accepted")}
}

func TestMigrationBlocksLiveWal(t *testing.T) {
	root:=t.TempDir()
	source:=filepath.Join(root,legacyVerifyDB)
	if err:=os.MkdirAll(filepath.Dir(source),0700);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(source,[]byte("legacy"),0600);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(source+"-wal",[]byte("pending"),0600);err!=nil{t.Fatal(err)}
	if err:=noLiveSidecars(source);err==nil{t.Fatal("unsafe live WAL was accepted")}
	if err:=dbCommand([]string{"migrate","--repo",root,"--confirm-offline"});err==nil{t.Fatal("migration with WAL was accepted")}
}

func TestVerifyBackendDBFlagRecognition(t *testing.T) {
	if !isArcDBArgument([]string{"gate","--db",".arc/arc.db"}){t.Fatal("separate db flag not detected")}
	if !isArcDBArgument([]string{"gate","--db=.arc/arc.db"}){t.Fatal("equals db flag not detected")}
	if isArcDBArgument([]string{"gate","--graph","g"}){t.Fatal("false positive for explicit DB")}
}

func TestMigrationCopiesGraphAndRequiresParity(t *testing.T) {
	if runtime.GOOS=="windows"{t.Skip("shell fake verifier is Unix-only")}
	root:=t.TempDir()
	source:=filepath.Join(root,legacyVerifyDB)
	if err:=os.MkdirAll(filepath.Dir(source),0700);err!=nil{t.Fatal(err)}
	d,err:=openArcDB(source);if err!=nil{t.Fatal(err)}
	if _,err:=d.Exec("CREATE TABLE graphs (id TEXT PRIMARY KEY, scope TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0)");err!=nil{t.Fatal(err)}
	if _,err:=d.Exec("INSERT INTO graphs(id,scope,approved) VALUES('graph-v1','unit',1)");err!=nil{t.Fatal(err)}
	// A checkpoint ensures the test follows the offline-migration precondition.
	if _,err:=d.Exec("PRAGMA wal_checkpoint(TRUNCATE)");err!=nil{t.Skipf("driver checkpoint unavailable: %v",err)}
	if err:=d.Close();err!=nil{t.Fatal(err)}
	if err:=noLiveSidecars(source);err!=nil{t.Skipf("database still has WAL sidecars: %v",err)}
	bin:=t.TempDir()
	sh:=filepath.Join(bin,"arc-verify")
	if err:=os.WriteFile(sh,[]byte(`#!/bin/sh
printf '%s\\n' '{"verdict":"BLOCKED"}'
exit 1
`),0700);err!=nil{t.Fatal(err)}
	t.Setenv("PATH",bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err:=dbCommand([]string{"migrate","--repo",root,"--confirm-offline"});err!=nil{t.Fatal(err)}
	m,err:=readManifest(root);if err!=nil{t.Fatal(err)}
	if m==nil || m.ParityGraphs!=1 || len(m.LegacySHA256)!=64{t.Fatal("migration was not activated after gate parity")}
	if _,err:=os.Stat(filepath.Join(root,".arc","backups","verify-"+m.LegacySHA256[:16]+".db"));err!=nil{t.Fatal(err)}
	if err:=dbCommand([]string{"migrate","--repo",root,"--confirm-offline"});err==nil{t.Fatal("second migration was accepted")}
}


func TestGitRenameAndNestedUntrackedFiles(t *testing.T) {
	root:=t.TempDir()
	git:=func(args ...string) {
		t.Helper()
		cmd:=exec.Command("git",append([]string{"-C",root},args...)...)
		if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("git %v: %v %s",args,err,out)}
	}
	git("init","-q")
	git("config","user.email","arc-test@example.invalid")
	git("config","user.name","Arc Test")
	if err:=os.MkdirAll(filepath.Join(root,"nested"),0700);err!=nil{t.Fatal(err)}
	oldPath:=filepath.Join(root,"nested","old.go")
	if err:=os.WriteFile(oldPath,[]byte("package old\n"),0600);err!=nil{t.Fatal(err)}
	git("add",".")
	git("commit","-qm","baseline")
	if err:=os.Rename(oldPath,filepath.Join(root,"nested","new.go"));err!=nil{t.Fatal(err)}
	git("add","-A")
	if err:=os.MkdirAll(filepath.Join(root,"fresh"),0700);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(root,"fresh","inside.go"),[]byte("package fresh\n"),0600);err!=nil{t.Fatal(err)}
	changes,err:=changedGitPaths(root);if err!=nil{t.Fatal(err)}
	got:=map[string]string{}
	for _,change:=range changes {got[change.Path]=change.Operation}
	for path,want:=range map[string]string{
		"nested/old.go":"deleted",
		"nested/new.go":"renamed",
		"fresh/inside.go":"untracked",
	} {
		if got[path]!=want{t.Fatalf("path %q: got %q, want %q; full changes=%v",path,got[path],want,got)}
	}
}


func TestMigrationGateProofMustBeStructured(t *testing.T) {
	testCases:=[]struct{
		name string
		output string
		exitCode int
		allowed bool
	}{
		{"empty","",0,false},
		{"garbage","not json",1,false},
		{"false PASS",`{"verdict":"PASS"}`,0,false},
		{"contradiction",`{"verdict":"BLOCKED"}`,0,false},
		{"blocked",`{"verdict":"BLOCKED"}`,1,true},
		{"valid PASS",`{"verdict":"PASS","proof_scope":"planned_rule_tasks","inputs_unchanged":true,"context":{"graph":"graph-v1"}}`,0,true},
		{"wrong graph",`{"verdict":"PASS","proof_scope":"planned_rule_tasks","inputs_unchanged":true,"context":{"graph":"other"}}`,0,false},
	}
	for _,tc:=range testCases{
		t.Run(tc.name,func(t *testing.T){
			err:=validateGateProof([]byte(tc.output),tc.exitCode,"graph-v1")
			if (err==nil)!=tc.allowed {t.Fatalf("valid=%v, want=%v (err=%v)",err==nil,tc.allowed,err)}
		})
	}
}


func TestSyncReconcileReplaysCommittedChanges(t *testing.T) {
	root,db:=testArcState(t)
	git:=func(args ...string){
		t.Helper()
		cmd:=exec.Command("git",append([]string{"-C",root},args...)...)
		if output,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("git %v: %v: %s",args,err,output)}
	}
	git("init","-q")
	git("config","user.name","Arc Test")
	git("config","user.email","arc@example.invalid")
	file:=filepath.Join(root,"sample.go")
	if err:=os.WriteFile(file,[]byte("package a\n"),0600);err!=nil{t.Fatal(err)}
	git("add","sample.go");git("commit","-qm","baseline")
	if err:=reconcileGit(db,root,"");err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(file,[]byte("package b\n"),0600);err!=nil{t.Fatal(err)}
	git("add","sample.go");git("commit","-qm","changed without PostToolUse")
	if err:=reconcileGit(db,root,"");err!=nil{t.Fatal(err)}
	if err:=reconcileGit(db,root,"");err!=nil{t.Fatal(err)}
	var count int
	if err:=db.QueryRow("SELECT count(*) FROM arc_changes").Scan(&count);err!=nil{t.Fatal(err)}
	if count!=1{t.Fatalf("wanted one recovered change, got %d",count)}
	var cursor string
	if err:=db.QueryRow("SELECT value FROM arc_meta WHERE key='sync_head'").Scan(&cursor);err!=nil{t.Fatal(err)}
	head,err:=gitHead(root);if err!=nil{t.Fatal(err)}
	if cursor!=head{t.Fatalf("cursor=%s, head=%s",cursor,head)}
	var version int
	if err:=db.QueryRow("SELECT version FROM arc_files WHERE path='sample.go'").Scan(&version);err!=nil{t.Fatal(err)}
	if version!=1{t.Fatalf("file version=%d after replay",version)}
}
