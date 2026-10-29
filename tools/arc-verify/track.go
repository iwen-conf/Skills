package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// StepRAW is an independent runner's observation, never the expected result
// derived from the implementation being tested.
type StepRAW struct {
    Run string `json:"run"`
    Step int `json:"step"`
    Expected string `json:"expected"`
    Observed string `json:"observed"`
    Preconditions map[string]bool `json:"preconditions"`
    Before string `json:"before,omitempty"`
    After string `json:"after,omitempty"`
    Denied bool `json:"denied,omitempty"`
    SafeToContinue bool `json:"safe_to_continue,omitempty"`
    Oracle string `json:"oracle"`
    Runner string `json:"runner"`
    ExitCode int `json:"exit_code"`
}

func hasSensitiveKey(v any) bool {
    switch data := v.(type) {
    case map[string]any:
        for key, child := range data {
            k := strings.ToLower(key)
            for _,part := range []string{"password","secret","token","authorization","cookie","api_key","apikey"} {
                if strings.Contains(k, part) { return true }
            }
            if hasSensitiveKey(child) { return true }
        }
    case []any:
        for _,child := range data { if hasSensitiveKey(child) { return true } }
    }
    return false
}

func parseStepRAW(raw []byte, run, expected, observed string, index int) (StepRAW,error) {
    var value any
    if err:=json.Unmarshal(raw,&value);err!=nil{return StepRAW{},fmt.Errorf("invalid runner RAW: %w",err)}
    if hasSensitiveKey(value){return StepRAW{},errors.New("RAW contains a credential-like field; redact it before recording")}
    var v StepRAW
    if err:=json.Unmarshal(raw,&v);err!=nil{return StepRAW{},err}
    if v.Run!=run||v.Step!=index||v.Expected!=expected||v.Observed!=observed||v.Oracle==""||v.Runner==""{
        return StepRAW{},errors.New("RAW run, step, oracle, expected, observed or runner identity mismatch")
    }
    // For a rejected operation, a bare claim of UNCHANGED is insufficient.
    // An independent observer must capture both sides of the state transition.
    if expected=="UNCHANGED" {
        if observed!="UNCHANGED"||v.Before==""||v.After==""||v.Before!=v.After||!v.Denied{
            return StepRAW{},errors.New("UNCHANGED requires identical before/after readback and a rejected operation")
        }
    }
    return v,nil
}

// Validate actual preconditions independently before evaluating the action's
// result. A rejection for the wrong reason is never a PASS.
func conditionsMatch(v StepRAW, kind, failed string, required []string) bool {
    if len(v.Preconditions)<len(required){return false}
    for _,condition:=range required{
        actual,found:=v.Preconditions[condition]
        if !found{return false}
        shouldBeSatisfied:=kind=="POSITIVE"||condition!=failed
        if actual!=shouldBeSatisfied{return false}
    }
    return true
}

func loadEvidence(path string)(string,[]byte,error){
    abs,e:=filepath.Abs(path);if e!=nil{return "",nil,e}
    stat,e:=os.Lstat(abs);if e!=nil{return "",nil,e}
    if !stat.Mode().IsRegular()||stat.Size()==0||stat.Size()>4<<20{
        return "",nil,errors.New("RAW evidence must be a nonempty regular file <=4 MiB")
    }
    b,e:=os.ReadFile(abs);return abs,b,e
}

func (d db) step(run,expected,observed,evidence string)(map[string]any,error){
    rows,e:=d.query("SELECT "+fieldsSQL("t.expected","t.graph","r.status","t.kind","t.condition","q.conditions")+" FROM runs r JOIN tasks t ON t.id=r.task JOIN rules q ON q.graph=t.graph AND q.id=t.rule WHERE r.id="+quote(run)+";",6)
    if e!=nil{return nil,e}
    if len(rows)!=1||rows[0][2]!="RUNNING"{return nil,errors.New("run not RUNNING")}
    finalExpected,graph:=rows[0][0],rows[0][1]
    if expected==""{expected=finalExpected}
    if expected!="UNCHANGED"{
        known,e:=d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(graph)+" AND id="+quote(expected)+" AND kind='state';",1)
        if e!=nil{return nil,e}
        if len(known)==0{return nil,fmt.Errorf("expected state %q is not in the frozen business graph",expected)}
    }
    n,e:=d.query("SELECT "+fieldsSQL("CAST(COALESCE(MAX(idx),0)+1 AS TEXT)")+" FROM steps WHERE run="+quote(run)+";",1)
    if e!=nil{return nil,e}
    index:=1
    if len(n)==1 {if _,e=fmt.Sscan(n[0][0],&index);e!=nil{return nil,e}}
    path,raw,e:=loadEvidence(evidence);if e!=nil{return nil,e}
    observation,e:=parseStepRAW(raw,run,expected,observed,index);if e!=nil{return nil,e}
    h:=sha256.Sum256(raw);sha:=hex.EncodeToString(h[:])
    verdict,decision:="PASS","CONTINUE"
    if expected==finalExpected {
        var required []string
        if e=json.Unmarshal([]byte(rows[0][5]),&required);e!=nil{return nil,e}
        if !conditionsMatch(observation,rows[0][3],rows[0][4],required){
            verdict,decision="BLOCKED","RESET"
        }
    }
    if verdict=="PASS"&&(expected!=observed||observation.ExitCode!=0){
        verdict,decision="FAIL","HALT"
        // Replan only when a known state was independently observed and the
        // observer explicitly certifies the state is safe for a new branch.
        if observation.SafeToContinue&&observation.ExitCode==0 {
            known,e:=d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(graph)+" AND id="+quote(observed)+" AND kind='state';",1)
            if e!=nil{return nil,e}
            if len(known)>0 {decision="REPLAN"}
        }
    }
    stmts:=[]string{
        fmt.Sprintf("INSERT INTO steps(run,idx,expected,observed,verdict,decision,evidence_path,evidence_sha256) SELECT %s,%d,%s,%s,%s,%s,%s,%s FROM runs WHERE id=%s AND status='RUNNING';",
          quote(run),index,quote(expected),quote(observed),quote(verdict),quote(decision),quote(path),quote(sha),quote(run)),
    }
    if verdict!="PASS"{
        stmts=append(stmts,fmt.Sprintf("UPDATE runs SET status=%s WHERE id=%s AND status='RUNNING';",quote(verdict),quote(run)))
        stmts=append(stmts,fmt.Sprintf("UPDATE tasks SET status=%s WHERE id=(SELECT task FROM runs WHERE id=%s);",quote(verdict),quote(run)))
    }
    if e=d.tx(stmts);e!=nil{return nil,e}
    check,e:=d.query("SELECT "+fieldsSQL("verdict","evidence_sha256")+" FROM steps WHERE run="+quote(run)+" AND idx="+fmt.Sprint(index)+";",2)
    if e!=nil{return nil,e};if len(check)!=1||check[0][1]!=sha{return nil,errors.New("step not durably recorded")}
    return map[string]any{"run":run,"step":index,"expected":expected,"observed":observed,"verdict":verdict,"decision":decision,"sha256":sha},nil
}

func (d db) finish(run string)(map[string]any,error){
    rows,e:=d.query("SELECT "+fieldsSQL("r.status","t.expected")+" FROM runs r JOIN tasks t ON t.id=r.task WHERE r.id="+quote(run)+";",2)
    if e!=nil{return nil,e};if len(rows)!=1||rows[0][0]!="RUNNING"{return nil,errors.New("run not RUNNING")}
    steps,e:=d.query("SELECT "+fieldsSQL("expected","observed","verdict","evidence_path","evidence_sha256")+" FROM steps WHERE run="+quote(run)+" ORDER BY idx;",5)
    if e!=nil{return nil,e}
    if len(steps)==0{return nil,errors.New("cannot PASS an unexecuted run")}
    for _,s:=range steps {
        if s[2]!="PASS" {return nil,errors.New("a step failed")}
        if e=validateEvidenceHash(s[3],s[4]);e!=nil{return nil,e}
    }
    last:=steps[len(steps)-1]
    if last[0]!=rows[0][1]||last[1]!=rows[0][1]{
        return nil,errors.New("final observed step does not match frozen task expectation")
    }
    if e=d.tx([]string{
        fmt.Sprintf("UPDATE runs SET status='PASS' WHERE id=%s AND status='RUNNING';",quote(run)),
        fmt.Sprintf("UPDATE tasks SET status='PASS' WHERE id=(SELECT task FROM runs WHERE id=%s);",quote(run)),
    });e!=nil{return nil,e}
    return map[string]any{"run":run,"verdict":"PASS","steps":len(steps)},nil
}

func validateEvidenceHash(path,hash string)error{
    _,raw,e:=loadEvidence(path);if e!=nil{return e}
    sum:=sha256.Sum256(raw)
    if hex.EncodeToString(sum[:])!=hash{return errors.New("RAW changed after being recorded")}
    return nil
}
