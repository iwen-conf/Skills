package main

import "testing"

func TestAbortUnresolvedSideEffectIsFencedAndRemainsBlocked(t *testing.T) {
    fixture:=prepareSeatFixture(t,"http://127.0.0.1:9")
    campaign:="X-abort-unresolved"
    seedExploreCampaign(t,fixture.d,campaign)
    lease,err:=fixture.d.claimExploreCampaign(campaign,"worker-abort")
    if err!=nil {t.Fatal(err)}
    if err:=fixture.d.lockExploreFixtures(campaign,[]string{fixture.fixture});err!=nil{t.Fatal(err)}
    action:=CandidateAction{ID:"claim-1",Action:"claim",Role:"STUDENT",RequestID:"request-1"}
    state:=ExploreCheckpoint{
        Campaign:campaign, Graph:"seat-v2",Status:exploreRunning,
        Actions:[]ExploreActionRecord{{
            AttemptID:"AT-inflight",RequestID:"request-1",Candidate:action,
            Status:actionDispatching, SideEffecting:true, Before:IndependentFacts{State:"idle"},
        }},
    }
    if err:=fixture.d.saveExploreCheckpointOwned(state,lease);err!=nil {t.Fatal(err)}
    finished,err:=fixture.d.abortExploreCampaign(campaign,"user requested stop")
    if err!=nil {t.Fatal(err)}
    if finished.Status!=exploreAborted || !finished.UnsafeResume || !finished.Report.UnsafeResume {
        t.Fatalf("unsafe abort must remain visibly blocked: %+v",finished)
    }
    persisted,err:=fixture.d.loadExploreCheckpoint(campaign)
    if err!=nil || !persisted.UnsafeResume || persisted.Actions[0].AttemptID!="AT-inflight"{
        t.Fatalf("abort dropped side-effect trace: %+v %v",persisted,err)
    }
    rows,err:=fixture.d.query("SELECT COUNT(*) FROM fixture_locks WHERE fixture="+quote(fixture.fixture)+" AND run="+quote(campaign),1)
    if err!=nil || len(rows)!=1 || rows[0][0]!="1" {
        t.Fatalf("abort silently released an unverified fixture lock: %v %v",rows,err)
    }
    if _,err:=fixture.d.abortExploreCampaign(campaign,"again");err==nil {t.Fatal("terminal abort must reject repeat")}
}
