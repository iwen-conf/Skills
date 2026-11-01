package main

import (
    "encoding/json"
    "os"
    "testing"
)

func TestGateRejectsMissingModernRAWIdentityEvenAfterRehash(t *testing.T) {
    for _, field := range []string{"campaign","task","fixture","actor","oracle"} {
        t.Run(field,func(t *testing.T){
            d:=isolatedLedger(t,sample())
            completeIsolatedGraph(t,d)
            ident:=testExploreIdentity(t,d,"academic-v1")
            campaign:=putIdentityCampaign(t,d,ident,func(report *ExploreReport,_ *RunBinding){
                rawBytes,err:=os.ReadFile(report.Steps[0].RAW)
                if err!=nil {t.Fatal(err)}
                var raw StepRAW
                if err:=json.Unmarshal(rawBytes,&raw);err!=nil {t.Fatal(err)}
                switch field {
                case "campaign": raw.Run=""
                case "task": raw.Identity.Task=""
                case "fixture": raw.Identity.Fixture=""
                case "actor": raw.Identity.Actor="";raw.Identity.Principal=""
                case "oracle": raw.Oracle=""
                }
                rewriteStepRAW(t,report,0,raw)
            })
            result:=d.verifyExploreEvidence(campaign)
            if result.EvidenceTrust==trustVerified || result.GateVerdict==string(gatePass) {
                t.Fatalf("missing %s must not be accepted even if SHA was updated: %+v",field,result)
            }
            if _,err:=cliOutput(t,"gate","--db",d.file,"--graph","academic-v1");err==nil {
                t.Fatalf("official gate accepted RAW without %s identity",field)
            }
        })
    }
}
