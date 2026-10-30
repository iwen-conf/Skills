package main
import "testing"
func TestLifecycleReasonRequired(t *testing.T){
 if e:=validReason("");e==nil{t.Fatal("empty retry reason accepted")}
 if e:=validReason("reset test data");e!=nil{t.Fatal(e)}
}
