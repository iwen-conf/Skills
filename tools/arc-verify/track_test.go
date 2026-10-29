package main
import (
    "testing"
)
func TestStepRAWMatchingIdentity(t *testing.T){
 raw:=[]byte(\`{"run":"R1","step":1,"expected":"APPROVED","observed":"APPROVED","oracle":"database readback","runner":"web-e2e","exit_code":0}\`)
 if _,e:=parseStepRAW(raw,"R1","APPROVED","APPROVED",1);e!=nil{t.Fatal(e)}
 if _,e:=parseStepRAW(raw,"R2","APPROVED","APPROVED",1);e==nil{t.Fatal("mismatched run accepted")}
 if _,e:=parseStepRAW(raw,"R1","APPROVED","APPROVED",2);e==nil{t.Fatal("mismatched step accepted")}
}
func TestRejectionRequiresBeforeAfter(t *testing.T){
 invalid:=[]byte(\`{"run":"R1","step":1,"expected":"UNCHANGED","observed":"UNCHANGED","oracle":"API readback","runner":"web-e2e","exit_code":0}\`)
 if _,e:=parseStepRAW(invalid,"R1","UNCHANGED","UNCHANGED",1);e==nil{t.Fatal("rejection without before/after accepted")}
 valid:=[]byte(\`{"run":"R1","step":1,"expected":"UNCHANGED","observed":"UNCHANGED","before":"PENDING","after":"PENDING","denied":true,"oracle":"readback","runner":"web-e2e","exit_code":0}\`)
 if _,e:=parseStepRAW(valid,"R1","UNCHANGED","UNCHANGED",1);e!=nil{t.Fatal(e)}
}
func TestRawRejectsEmbeddedCredentialFields(t *testing.T){
 raw:=[]byte(\`{"run":"R1","step":1,"expected":"APPROVED","observed":"APPROVED","oracle":"db","runner":"runner","exit_code":0,"request":{"headers":{"Authorization":"Bearer x"}}}\`)
 if _,e:=parseStepRAW(raw,"R1","APPROVED","APPROVED",1);e==nil{t.Fatal("credential captured in RAW")}
}
