package main

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func sample()Graph{
 return Graph{ID:"academic-v1",Scope:"course enrollment",
 Nodes:[]Node{{"enroll","action","Enroll"},{"login","condition","Logged in"},{"ready","state","Ready"}},
 Edges:[]Edge{{"e1","login","enroll","requires"},{"e2","enroll","ready","transitions"}},
 Rules:[]Rule{{ID:"R1",Action:"enroll",Actor:"STUDENT",Entry:"/courses",Oracle:"independent enrollment SQL readback",Conditions:[]string{"login"},Expected:"ready",Rejected:"UNCHANGED",Source:"user",Description:"Only authenticated students can enroll"}},
 Risks:[]Risk{{"K1","Possible schedule clash","enroll"}}}
}
func TestGraphAndRules(t *testing.T){
 g:=sample()
 if e:=validateGraph(g);e!=nil{t.Fatal(e)}
 g.Rules[0].Conditions=[]string{"nonexistent"}
 if e:=validateGraph(g);e==nil{t.Fatal("unknown condition accepted")}
}
func TestRiskCannotBeRule(t *testing.T){
 g:=sample();g.Rules=nil
 if e:=validateGraph(g);e!=nil{t.Fatal(e)}
 if len(g.Risks)!=1{t.Fatal("risk lost")}
}
func TestQuoteAndJSONC(t *testing.T){
 if quote("a'b")!="'a''b'"{t.Fatal("SQL quoting")}
 s,e:=cleanJSONC(`{"url":"https://example.test/x//y", /* note */ "approved":true // comment
}`)
 if e!=nil||!strings.Contains(s,"https://example.test/x//y"){t.Fatal(s,e)}
}
func TestConfigPreflight(t *testing.T){
 dir:=t.TempDir();path:=filepath.Join(dir,"config.jsonc")
 if e:=os.WriteFile(path,[]byte(`{"environments":{"test":{"approved":true,"url":"https://example.test"},"prod":{"approved":false,"url":"https://example.test"}}}`),0600);e!=nil{t.Fatal(e)}
 if e:=approvedEnvironment(path,"test");e!=nil{t.Fatal(e)}
 if e:=approvedEnvironment(path,"prod");e==nil{t.Fatal("unapproved environment accepted")}
}

func TestNativeSQLProjectionDoesNotHexEncode(t *testing.T){
 if got:=fieldsSQL("t.id","CAST(approved AS TEXT)");got!="t.id,CAST(approved AS TEXT)"{t.Fatalf("unexpected native projection %q",got)}
}
