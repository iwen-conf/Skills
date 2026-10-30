package main

import "testing"

func TestRecoveryGraphShowsUnapprovedAction(t *testing.T){
 edges:=[]Edge{{"e1","login","logged_in","recovers"}}
 r:=resolveRecovery("logged_in",edges,nil,6)
 if len(r.Actions)!=1||r.Actions[0].Approved||len(r.Unresolved)!=1{t.Fatalf("must not execute unconfirmed recovery action: %+v",r)}
}
func TestRecoveryGraphFindsMultiStepDependenciesAndCycles(t *testing.T){
 edges:=[]Edge{
  {"e1","login","logged_in","recovers"},
  {"e2","password_ready","login","requires"},
  {"e3","reset","password_ready","recovers"},
  {"e4","logged_in","reset","requires"},
 }
 rules:=[]RecoveryRule{{Action:"login",Actor:"STUDENT",Entry:"/login",Oracle:"session",Approved:true},{Action:"reset",Actor:"STUDENT",Entry:"/reset",Oracle:"identity",Approved:true}}
 r:=resolveRecovery("logged_in",edges,rules,6)
 if len(r.Actions)!=2||len(r.Cycles)!=1||r.Cycles[0]!="logged_in"{t.Fatalf("cycle not bounded: %+v",r)}
}
