package main

import (
    "errors"
    "fmt"
    "strings"
)

func validReason(reason string)error{
    if strings.TrimSpace(reason)==""||len(reason)>512{return errors.New("a nonempty reason <=512 bytes is required")}
    return nil
}
// abort closes only a RUNNING attempt and preserves all previously captured
// steps and raw evidence. It never turns a test into PASS.
func (d db) abort(run,reason string)(map[string]string,error){
    if e:=validReason(reason);e!=nil{return nil,e}
    rows,e:=d.query("SELECT "+fieldsSQL("task","status")+" FROM runs WHERE id="+quote(run)+";",2)
    if e!=nil{return nil,e}
    if len(rows)!=1||rows[0][1]!="RUNNING"{return nil,errors.New("run not RUNNING")}
    task:=rows[0][0]
    if e=d.tx([]string{
        fmt.Sprintf("UPDATE runs SET status='BLOCKED' WHERE id=%s AND status='RUNNING';",quote(run)),
        fmt.Sprintf("UPDATE tasks SET status='BLOCKED' WHERE id=%s AND status='RUNNING';",quote(task)),
        fmt.Sprintf("INSERT INTO task_events(task,run,event,reason) VALUES(%s,%s,'ABORT',%s);",quote(task),quote(run),quote(reason)),
    });e!=nil{return nil,e}
    return map[string]string{"run":run,"status":"BLOCKED","next":"retry after environment/data restoration"},nil
}
// retry prepares a new attempt after a documented reset. The old FAIL/BLOCKED
// run is immutable; a new start command obtains a fresh run ID.
func (d db) retry(task,reason string)(map[string]string,error){
    if e:=validReason(reason);e!=nil{return nil,e}
    rows,e:=d.query("SELECT "+fieldsSQL("status")+" FROM tasks WHERE id="+quote(task)+";",1)
    if e!=nil{return nil,e}
    if len(rows)!=1||rows[0][0]!="BLOCKED"&&rows[0][0]!="FAIL"{
        return nil,errors.New("only BLOCKED or FAIL tasks may be retried; PASS is immutable")
    }
    if e=d.tx([]string{
        fmt.Sprintf("UPDATE tasks SET status='READY' WHERE id=%s AND status IN ('FAIL','BLOCKED');",quote(task)),
        fmt.Sprintf("INSERT INTO task_events(task,run,event,reason) VALUES(%s,'','RETRY',%s);",quote(task),quote(reason)),
    });e!=nil{return nil,e}
    return map[string]string{"task":task,"status":"READY","next":"start a new run with verified preconditions"},nil
}
