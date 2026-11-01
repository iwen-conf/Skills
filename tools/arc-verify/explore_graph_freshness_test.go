package main

import (
    "fmt"
    "testing"
)

func TestVersionedEvidenceBecomesStaleWhenCurrentGraphChanges(t *testing.T) {
    d:=isolatedLedger(t,sample())
    completeIsolatedGraph(t,d)
    ident:=testExploreIdentity(t,d,"academic-v1")
    campaign:=putIdentityCampaign(t,d,ident,nil)
    before:=d.verifyExploreEvidence(campaign)
    if before.EvidenceTrust!=trustVerified {t.Fatalf("known good evidence not verified: %+v",before)}
    if err:=d.tx([]string{fmt.Sprintf(
        "UPDATE graphs SET scope=%s WHERE id=%s",
        quote("changed-scope"),quote("academic-v1"),
    )});err!=nil{t.Fatal(err)}
    after:=d.verifyExploreEvidence(campaign)
    if after.GateVerdict!=string(gateStale) || after.EvidenceTrust!=trustStale {
        t.Fatalf("old RAW must become STALE on graph change, not VERIFIED/TAMPERED: %+v",after)
    }
    if _,err:=cliOutput(t,"gate","--db",d.file,"--graph","academic-v1");err==nil {
        t.Fatal("official gate accepted evidence for previous graph")
    }
}
