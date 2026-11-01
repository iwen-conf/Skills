package main

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestResumeObservedWithTamperedRAWBlocksSideEffects(t *testing.T) {
    for _, tc := range []struct{name, body string; sha string; graph string}{
        {name:"digest mismatch",body:`{"run":"X-raw-corrupt","identity":{"graph":"seat-v2"}}`,sha:"known-invalid"},
        {name:"identity mismatch",body:`{"run":"X-raw-corrupt","identity":{"graph":"foreign-graph"}}`,graph:"seat-v2"},
        {name:"foreign campaign",body:`{"run":"X-foreign","identity":{"graph":"seat-v2"}}`,graph:"seat-v2"},
        {name:"missing campaign",body:`{"identity":{"graph":"seat-v2"}}`,graph:"seat-v2"},
    } {
        t.Run(tc.name,func(t *testing.T){
            fixture:=prepareSeatFixture(t,"http://127.0.0.1:9")
            campaign:="X-raw-corrupt"
            seedExploreCampaign(t,fixture.d,campaign)
            lease,err:=fixture.d.claimExploreCampaign(campaign,"worker-raw")
            if err!=nil {t.Fatal(err)}
            rawPath:=filepath.Join(t.TempDir(),"observed.json")
            body:=[]byte(tc.body)
            if err:=os.WriteFile(rawPath,body,0600);err!=nil{t.Fatal(err)}
            sha:=tc.sha
            if sha=="" {sha=digestBytes(body)}
            item:=CandidateAction{ID:"claim-1",Action:"claim",Role:"STUDENT",RequestID:"side-effect-1"}
            state:=ExploreCheckpoint{
                Campaign:campaign,Graph:"seat-v2",Status:exploreRunning,
                Report:ExploreReport{EvidenceContract: exploreIdentityContract},
                Frontier:[]CandidateAction{item},
                Actions:[]ExploreActionRecord{{
                    AttemptID:"AT-immutable",RequestID:item.RequestID,Candidate:item,
                    Status:actionObserved,SideEffecting:true,Before:IndependentFacts{State:"idle"},
                    Step:&ExploreStep{Candidate:item,RAW:rawPath,SHA256:sha},
                }},
            }
            if err:=fixture.d.saveExploreCheckpointOwned(state,lease);err!=nil{t.Fatal(err)}
            binding:=RunBinding{Identity:ExecutionIdentity{Graph:"seat-v2"}}
            if err:=fixture.d.reconcileUnknownActions(context.Background(),binding,ExploreAdapter{},nil,&state,lease);err!=nil{t.Fatal(err)}
            if !state.UnsafeResume || !state.Report.UnsafeResume || state.Actions[0].Status!=actionBlocked || !state.Actions[0].ReplayForbidden {
                t.Fatalf("invalid observed RAW must seal action and stop campaign: %+v",state)
            }
            for _,entry:=range state.Frontier{if entry.ID==item.ID{t.Fatalf("invalid attempt returned to frontier: %+v",state.Frontier)}}
            persisted,err:=fixture.d.loadExploreCheckpoint(campaign)
            if err!=nil || !persisted.UnsafeResume || persisted.Actions[0].AttemptID!="AT-immutable" {
                t.Fatalf("unsafe hold not durable: %+v %v",persisted,err)
            }
        })
    }
}

func TestSourceEntryMethodMismatchNeverConfirmed(t *testing.T) {
    adapter:=ExploreAdapter{Entries:[]ExploreEntry{{Action:"pay",Entry:"/payments",Method:"POST"}}}
    found:=discoverOpenAPIEntries("openapi.json",[]byte(`{"paths":{"/payments":{"get":{},"post":{}}}}`),map[string]bool{"/payments":true})
    if len(found)!=2 {t.Fatalf("expected two methods, got %+v",found)}
    for _,entry:=range found {
        result:=authorizeDiscoveredEntry(entry,adapter)
        if entry.Method=="GET" && (result.Class==entryConfirmed||result.Action!=""){
            t.Fatalf("GET must not use POST adapter approval: %+v",result)
        }
        if entry.Method=="POST" && (result.Class!=entryConfirmed||result.Action!="pay"){
            t.Fatalf("POST must match approved entry: %+v",result)
        }
    }
}
