package main
import "testing"

func TestHandoffTargetRejectsInventedSkill(t *testing.T){
 for _,name:=range []string{"arc-explore","arc-plan","arc-code","arc-verify","arc-audit","arc-vet"}{
  if e:=validateHandoffTarget(name);e!=nil{t.Fatal(e)}
 }
 if e:=validateHandoffTarget("arc-approve");e==nil{t.Fatal("invented skill accepted")}
}
func TestHandoffFirstFailureIsImmutable(t *testing.T){
 steps:=[]StepTrace{{Index:"1",Verdict:"PASS"},{Index:"2",Verdict:"FAIL",Expected:"APPROVED",Observed:"PENDING"},{Index:"3",Verdict:"PASS"}}
 divergence,ok:=firstDivergence(steps)
 if !ok||divergence.Index!="2"{t.Fatal("first divergence must be earliest failed step")}
}
func TestHandoffTruncatesContext(t *testing.T){
 rows:=make([][]string,17)
 limited,wasTruncated:=takeRows(rows,8)
 if !wasTruncated||len(limited)!=8{t.Fatal("unbounded handoff context")}
}
