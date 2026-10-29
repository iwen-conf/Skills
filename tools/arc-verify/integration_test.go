package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "testing"
)

func TestTursoLocalBusinessChain(t *testing.T){
    path:=filepath.Join(t.TempDir(),"data.db")
    d,err:=openDB(path,true);if err!=nil{t.Fatal(err)}
    defer d.close()
    for _,sql:=range schema{if _,err=d.sql(sql);err!=nil{t.Fatalf("schema: %v",err)}}
    g:=sample()
    if err=validateGraph(g);err!=nil{t.Fatal(err)}
    if err=d.importGraph(g);err!=nil{t.Fatal(err)}
    if err=d.approve(g.ID,"");err!=nil{t.Fatal(err)}
    if err=d.approve(g.ID,"R1");err!=nil{t.Fatal(err)}
    if _,err=d.plan(g.ID);err!=nil{t.Fatal(err)}
    cards,err:=d.next(g.ID);if err!=nil{t.Fatal(err)}
    if len(cards)!=3{t.Fatalf("one condition must generate 3 obligations: %+v",cards)}
    for _,card:=range cards{
        if card.Actor!="STUDENT"||card.Entry!="/courses"||card.Oracle==""{t.Fatalf("incomplete task card %+v",card)}
        run,err:=d.start(card.ID,"isolated-test","0123456789012345678901234567890123456789")
        if err!=nil{t.Fatal(err)}
        raw:=StepRAW{
            Run:run,Step:1,Expected:card.Expected,Observed:card.Expected,
            Oracle:"independent database readback",Runner:"test-e2e-adapter",ExitCode:0,
            Preconditions:map[string]bool{"login":card.Type=="POSITIVE"},
        }
        if card.Expected=="UNCHANGED"{raw.Before="PENDING";raw.After="PENDING";raw.Denied=true}
        content,_:=json.Marshal(raw)
        proof:=filepath.Join(t.TempDir(),run+".json")
        if err=os.WriteFile(proof,content,0600);err!=nil{t.Fatal(err)}
        if _,err=d.step(run,"",card.Expected,proof);err!=nil{t.Fatal(err)}
        if _,err=d.finish(run);err!=nil{t.Fatal(err)}
    }
    // Separate DB object opens the same local file; no daemon or cloud endpoint.
    other,err:=openDB(path,false);if err!=nil{t.Fatal(err)}
    defer other.close()
    result,err:=other.gate(g.ID);if err!=nil{t.Fatal(err)}
    if result["verdict"]!="PASS"{t.Fatalf("expected full verified task set, got %+v",result)}
    // Orphan/extra tasks must not disappear from a gate via an inner join.
    if err=other.tx([]string{"INSERT INTO tasks(id,graph,rule,kind,condition,expected,status) VALUES('orphan', 'academic-v1','not-a-rule','POSITIVE','','ready','PASS');"});err!=nil{t.Fatal(err)}
    result,err=other.gate(g.ID);if err!=nil{t.Fatal(err)}
    if result["verdict"]!="FAIL"{t.Fatalf("orphan task must fail closed, got %+v",result)}
    if err=other.tx([]string{"DELETE FROM tasks WHERE id='orphan';"});err!=nil{t.Fatal(err)}
    // A recorded PASS cannot survive RAW tampering.
    if err=other.tx([]string{fmt.Sprintf("UPDATE steps SET evidence_sha256='tampered' WHERE run=(SELECT id FROM runs LIMIT 1);")});err!=nil{t.Fatal(err)}
    result,err=other.gate(g.ID);if err!=nil{t.Fatal(err)}
    if result["verdict"]!="FAIL"{t.Fatalf("changed evidence hash must fail closed, got %+v",result)}
}

func TestTursoLazyPlannerHonorsBoundedFrontier(t *testing.T){
 path:=filepath.Join(t.TempDir(),"lazy.db")
 d,err:=openDB(path,true);if err!=nil{t.Fatal(err)}
 defer d.close()
 for _,q:=range schema{if _,err=d.sql(q);err!=nil{t.Fatal(err)}}
 g:=sample()
 for i:=1;i<50;i++{rule:=g.Rules[0];rule.ID=fmt.Sprintf("RULE-%03d",i);g.Rules=append(g.Rules,rule)}
 if err=validateGraph(g);err!=nil{t.Fatal(err)}
 if err=d.importGraph(g);err!=nil{t.Fatal(err)}
 if err=d.approve(g.ID,"");err!=nil{t.Fatal(err)}
 var approvals []string
 for _,r:=range g.Rules{approvals=append(approvals,"UPDATE rules SET approved=1 WHERE graph="+quote(g.ID)+" AND id="+quote(r.ID)+";")}
 if err=d.tx(approvals);err!=nil{t.Fatal(err)}
 first,err:=d.plan(g.ID,32);if err!=nil||first!=32{t.Fatalf("first batch %d %v",first,err)}
 verdict,err:=d.gate(g.ID);if err!=nil{t.Fatal(err)}
 if verdict["verdict"]!="FAIL"||verdict["missing_tasks"]==0{t.Fatalf("partial frontier must fail gate %+v",verdict)}
 total:=first
 for i:=0;i<10;i++{
  n,err:=d.plan(g.ID,32);if err!=nil{t.Fatal(err)}
  total+=n
  if n==0{break}
 }
 if total!=150{t.Fatalf("expected 150 obligations, materialized %d",total)}
 if n,err:=d.plan(g.ID,32);err!=nil||n!=0{t.Fatalf("duplicate batch %d %v",n,err)}
}
