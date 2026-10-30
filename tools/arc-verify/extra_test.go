package main
import "testing"
func TestFixtureSecrets(t *testing.T){
 if e:=validateFixtures(Fixtures{[]Fixture{{ID:"student",Kind:"account",State:"ready",SecretRef:"env://STUDENT_PASSWORD"}}});e!=nil{t.Fatal(e)}
 if e:=validateFixtures(Fixtures{[]Fixture{{ID:"student",Kind:"account",State:"ready",SecretRef:"mypassword"}}});e==nil{t.Fatal("plaintext secret accepted")}
}
