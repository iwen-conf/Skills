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
    // A brand-new task cannot be marked passed merely by inserting a fake PASS status.
    if err=other.tx([]string{fmt.Sprintf("UPDATE steps SET evidence_sha256='tampered' WHERE run=(SELECT id FROM runs LIMIT 1);")});err!=nil{t.Fatal(err)}
    result,err=other.gate(g.ID);if err!=nil{t.Fatal(err)}
    if result["verdict"]!="FAIL"{t.Fatalf("changed evidence hash must fail closed, got %+v",result)}
}
