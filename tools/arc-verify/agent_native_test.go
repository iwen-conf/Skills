package main
import (
 "context"
 "encoding/json"
 "os"
 "os/exec"
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

// Separate OS processes see the same durable Campaign. Model inference belongs
// to each host Agent, never to an in-process Go model client.
func TestAgentNativeIndependentProcessReadsCheckpoint(t *testing.T) {
 if os.Getenv("ARC_VERIFY_AGENT_NATIVE_HELPER")=="1" {
    file:=os.Getenv("ARC_VERIFY_AGENT_NATIVE_DB")
    campaign:=os.Getenv("ARC_VERIFY_AGENT_NATIVE_CAMPAIGN")
    d,err:=openDB(file,false)
    if err!=nil {t.Fatal(err)}
    defer d.close()
    handoff,err:=d.agentNativeContext(campaign)
    if err!=nil || handoff.Checkpoint=="" || handoff.Protocol!="arc-verify/agent-native/v1" {
        t.Fatalf("separate process could not recover Campaign: %+v %v",handoff,err)
    }
    return
 }
 _,server:=newSeatServer(t)
 f:=prepareSeatFixture(t,server.URL)
 f.openRound(t)
 adapter:=seatExploreAdapter(t,seatIsolationScope)
 report,err:=f.d.runExploration(context.Background(),ExploreRequest{
    Graph:"seat-v2",Flow:"flow.reservation",Environment:"independent",Round:f.round,
    Root:f.root,Config:f.config,Commit:f.commit,Account:f.account,Fixture:f.fixture,
    Adapter:adapter,Mode:"agent-native",Budget:ExploreRuntimeBudget{MaxStates:8,MaxPaths:8,MaxActions:2,Deadline:6*time.Second},
 })
 if err!=nil {t.Fatal(err)}
 child:=exec.Command(os.Args[0],"-test.run=^TestAgentNativeIndependentProcessReadsCheckpoint$","-test.count=1")
 child.Env=append(os.Environ(),
    "ARC_VERIFY_AGENT_NATIVE_HELPER=1",
    "ARC_VERIFY_AGENT_NATIVE_DB="+f.d.file,
    "ARC_VERIFY_AGENT_NATIVE_CAMPAIGN="+report.Campaign)
 output,err:=child.CombinedOutput()
 if err!=nil {t.Fatalf("second process handoff failed: %v\n%s",err,output)}
}
