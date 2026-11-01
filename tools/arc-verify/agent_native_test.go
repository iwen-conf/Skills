package main
import (
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
 "time"
)
func TestAgentNativeNeedsNoModelConfiguration(t *testing.T) {
 t.Setenv("OPENAI_API_KEY","")
 t.Setenv("ARC_VERIFY_PLANNER_URL","")
 if decodePlannerMode("agent-native")!="agent-native" {t.Fatal("native mode rejected")}
}
func TestAgentNativeProposalPersistsAcrossTurns(t *testing.T) {
 _,server:=newSeatServer(t)
 f:=prepareSeatFixture(t,server.URL)
 f.openRound(t)
 adapter:=seatExploreAdapter(t,seatIsolationScope)
 adapterBytes,err:=json.Marshal(adapter)
 if err!=nil {t.Fatal(err)}
 adapterPath:=filepath.Join(t.TempDir(),"adapter.json")
 if err:=os.WriteFile(adapterPath,adapterBytes,0600);err!=nil{t.Fatal(err)}
 req:=ExploreRequest{Graph:"seat-v2",Flow:"flow.reservation",Environment:"independent",Round:f.round,
 Root:f.root,Config:f.config,Commit:f.commit,Account:f.account,Fixture:f.fixture,
 Adapter:adapter,AdapterPath:adapterPath,Mode:"agent-native",Budget:ExploreRuntimeBudget{MaxStates:8,MaxPaths:8,MaxActions:4,Deadline:8*time.Second}}
 report,err:=f.d.runExploration(context.Background(),req)
 if err!=nil {t.Fatal(err)}
 if report.Actions!=0 || report.StopReason!="awaiting_agent" {t.Fatalf("start executed work: %+v",report)}
 state,err:=f.d.agentNativeContext(report.Campaign)
 if err!=nil {t.Fatal(err)}
 proposal:=AgentNativeProposal{Campaign:report.Campaign,Checkpoint:state.Checkpoint,HypothesisID:"BH-001",
 Suggestion:PlannerSuggestion{Capability:"capability.reservation",Action:"claim",Entry:"/claim",Role:"STUDENT",Reason:"test ownership",Observe:"state",Risk:"medium"}}
 queued,err:=f.d.agentNativePropose(report.Campaign,adapter,proposal)
 if err!=nil {t.Fatal(err)}
 if len(queued.Frontier)!=1 || queued.Checkpoint==state.Checkpoint {t.Fatal("proposal not checkpointed")}
 if queued.AdapterPath=="" || queued.Environment!="independent" || len(queued.AllowedEntries)==0 {t.Fatalf("handoff has incomplete portable context: %+v",queued)}
 if _,err:=f.d.agentNativePropose(report.Campaign,adapter,proposal);err==nil {t.Fatal("stale agent overwrote the handoff")}
 req.Campaign=report.Campaign
 result,err:=f.d.runExploration(context.Background(),req)
 if err!=nil {t.Fatal(err)}
 if result.Actions!=1 || result.StopReason!="awaiting_agent" {t.Fatalf("expected one action only: %+v",result)}
 next,err:=f.d.agentNativeContext(report.Campaign)
 if err!=nil || len(next.Frontier)!=0 || len(next.Recent)!=1 {t.Fatalf("handoff lost: %+v %v",next,err)}
}

// The safe finish gate must reject an empty or pending campaign without releasing fixtures.
func TestAgentNativeCompleteRefusesUnexecutedCampaign(t *testing.T) {
 _,server:=newSeatServer(t)
 f:=prepareSeatFixture(t,server.URL)
 f.openRound(t)
 adapter:=seatExploreAdapter(t,seatIsolationScope)
 req:=ExploreRequest{Graph:"seat-v2",Flow:"flow.reservation",Environment:"independent",Round:f.round,
 Root:f.root,Config:f.config,Commit:f.commit,Account:f.account,Fixture:f.fixture,
 Adapter:adapter,Mode:"agent-native",Budget:ExploreRuntimeBudget{MaxStates:8,MaxPaths:8,MaxActions:4,Deadline:8*time.Second}}
 report,err:=f.d.runExploration(context.Background(),req)
 if err!=nil{t.Fatal(err)}
 if _,err:=f.d.agentNativeComplete(context.Background(),report.Campaign,adapter);err==nil{t.Fatal("empty campaign must not complete")}
}
