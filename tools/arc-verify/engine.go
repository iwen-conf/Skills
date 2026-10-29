package main

import (
 "crypto/rand"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
)

func (d db) importGraph(g Graph)error{
    rows,e:=d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(g.ID)+";",1)
    if e!=nil{return e};if len(rows)>0{return errors.New("graph version already exists; publish a new graph ID")}
    stmts:=[]string{fmt.Sprintf("INSERT INTO graphs(id,scope) VALUES(%s,%s);",quote(g.ID),quote(g.Scope))}
    for _,n:=range g.Nodes{stmts=append(stmts,fmt.Sprintf("INSERT INTO nodes VALUES(%s,%s,%s,%s);",quote(g.ID),quote(n.ID),quote(n.Kind),quote(n.Name)))}
    for _,e:=range g.Edges{stmts=append(stmts,fmt.Sprintf("INSERT INTO edges VALUES(%s,%s,%s,%s,%s);",quote(g.ID),quote(e.ID),quote(e.From),quote(e.To),quote(e.Kind)))}
    for _,r:=range g.Rules{
       cond,_:=json.Marshal(r.Conditions)
       stmts=append(stmts,fmt.Sprintf("INSERT INTO rules(graph,id,action,conditions,expected,rejected,source,description) VALUES(%s,%s,%s,%s,%s,%s,%s,%s);",quote(g.ID),quote(r.ID),quote(r.Action),quote(string(cond)),quote(r.Expected),quote(r.Rejected),quote(r.Source),quote(r.Description)))
    }
    for _,r:=range g.Risks{stmts=append(stmts,fmt.Sprintf("INSERT INTO risks(graph,id,action,description) VALUES(%s,%s,%s,%s);",quote(g.ID),quote(r.ID),quote(r.Action),quote(r.Description)))}
    return d.tx(stmts)
}
func (d db) approve(graph,rule string)error{
    if graph==""{return errors.New("graph ID required")}
    exists,e:=d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(graph)+";",1)
    if e!=nil{return e};if len(exists)!=1{return errors.New("graph not found")}
    if rule==""{return d.tx([]string{fmt.Sprintf("UPDATE graphs SET approved=1 WHERE id=%s;",quote(graph))})}
    rows,e:=d.query("SELECT "+fieldsSQL("source")+" FROM rules WHERE graph="+quote(graph)+" AND id="+quote(rule)+";",1)
    if e!=nil{return e};if len(rows)!=1{return errors.New("rule not found")}
    return d.tx([]string{fmt.Sprintf("UPDATE rules SET approved=1 WHERE graph=%s AND id=%s;",quote(graph),quote(rule))})
}
func (d db) plan(graph string)(int,error){
    rows,e:=d.query("SELECT "+fieldsSQL("approved")+" FROM graphs WHERE id="+quote(graph)+";",1)
    if e!=nil{return 0,e};if len(rows)!=1||rows[0][0]!="1"{return 0,errors.New("business catalog requires explicit user approval")}
    rows,e=d.query("SELECT "+fieldsSQL("id","conditions","expected","rejected")+" FROM rules WHERE graph="+quote(graph)+" AND approved=1;",4)
    if e!=nil{return 0,e}
    stmts:=[]string{}
    for _,r:=range rows{
        id:=r[0];var conditions []string
        if e=json.Unmarshal([]byte(r[1]),&conditions);e!=nil{return 0,e}
        add:=func(kind,condition,expected string){
            taskID:=graph+"/"+id+"/"+kind
            if condition!=""{taskID+="/"+condition}
            stmts=append(stmts,fmt.Sprintf("INSERT OR IGNORE INTO tasks(id,graph,rule,kind,condition,expected) VALUES(%s,%s,%s,%s,%s,%s);",quote(taskID),quote(graph),quote(id),quote(kind),quote(condition),quote(expected)))
        }
        add("POSITIVE","",r[2])
        for _,c:=range conditions{add("NEGATIVE",c,r[3]);add("FORCED",c,r[3])}
    }
    // Only explicit user-approved rules can create mandatory test tasks.
    // Risks remain recorded but are not FAIL or MISSING.
    return len(stmts),d.tx(stmts)
}
func (d db) next(graph string)([][]string,error){
    return d.query("SELECT "+fieldsSQL("id","rule","kind","condition","expected")+" FROM tasks WHERE graph="+quote(graph)+" AND status='READY' ORDER BY id LIMIT 8;",5)
}
func newID(prefix string)string{
    b:=make([]byte,16);if _,e:=rand.Read(b);e!=nil{panic(e)}
    return prefix+"-"+hex.EncodeToString(b)
}
func (d db) start(task,environment,commit string)(string,error){
    if environment==""||commit==""{return "",errors.New("environment and commit required")}
    rows,e:=d.query("SELECT "+fieldsSQL("status")+" FROM tasks WHERE id="+quote(task)+";",1)
    if e!=nil{return "",e};if len(rows)!=1||rows[0][0]!="READY"{return "",errors.New("task not READY")}
    id:=newID("R")
    err:=d.tx([]string{
       fmt.Sprintf("INSERT INTO runs(id,task,environment,commit_sha) SELECT %s,id,%s,%s FROM tasks WHERE id=%s AND status='READY';",quote(id),quote(environment),quote(commit),quote(task)),
       fmt.Sprintf("UPDATE tasks SET status='RUNNING' WHERE id=%s AND status='READY';",quote(task)),
    })
    if err!=nil{return "",err}
    check,err:=d.query("SELECT "+fieldsSQL("id")+" FROM runs WHERE id="+quote(id)+";",1)
    if err!=nil{return "",err};if len(check)!=1{return "",errors.New("task claimed by another process")}
    return id,nil
}
func (d db) record(run,observed,evidence string)(map[string]string,error){
    rows,e:=d.query("SELECT "+fieldsSQL("t.expected","r.status","t.graph")+" FROM runs r JOIN tasks t ON r.task=t.id WHERE r.id="+quote(run)+";",3)
    if e!=nil{return nil,e};if len(rows)!=1||rows[0][1]!="RUNNING"{return nil,errors.New("run not RUNNING")}
    expected:=rows[0][0]
    st,e:=os.Lstat(evidence);if e!=nil{return nil,e}
    if !st.Mode().IsRegular()||st.Size()==0||st.Size()>4<<20{return nil,errors.New("evidence must be regular nonempty file <=4MiB")}
    b,e:=os.ReadFile(evidence);if e!=nil{return nil,e}
    var raw struct {
        Run string `json:"run"`
        Observed string `json:"observed"`
        Oracle string `json:"oracle"`
        Runner string `json:"runner"`
        ExitCode int `json:"exit_code"`
    }
    if e=json.Unmarshal(b,&raw);e!=nil{return nil,fmt.Errorf("runner RAW must be JSON: %w",e)}
    if raw.Run!=run||raw.Observed!=observed||raw.Oracle==""||raw.Runner==""||raw.ExitCode!=0{
       return nil,errors.New("runner RAW identity, oracle or exit code invalid")
    }
    hash:=sha256.Sum256(b)
    verdict:="FAIL";decision:="HALT"
    if observed==expected {verdict="PASS";decision="CONTINUE"} else {
        known,e:=d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(rows[0][2])+" AND kind='state' AND id="+quote(observed)+";",1)
        if e!=nil{return nil,e};if len(known)>0{decision="REPLAN"}
    }
    err:=d.tx([]string{
       fmt.Sprintf("INSERT INTO steps VALUES(%s,1,%s,%s,%s,%s,%s,%s);",quote(run),quote(expected),quote(observed),quote(verdict),quote(decision),quote(filepath.Clean(evidence)),quote(hex.EncodeToString(hash[:]))),
       fmt.Sprintf("UPDATE runs SET status=%s WHERE id=%s;",quote(verdict),quote(run)),
       fmt.Sprintf("UPDATE tasks SET status=%s WHERE id=(SELECT task FROM runs WHERE id=%s);",quote(verdict),quote(run)),
    })
    if err!=nil{return nil,err}
    return map[string]string{"run":run,"verdict":verdict,"decision":decision,"expected":expected,"observed":observed,"sha256":hex.EncodeToString(hash[:])},nil
}
func (d db) gate(graph string)(map[string]any,error){
    g,e:=d.query("SELECT "+fieldsSQL("approved")+" FROM graphs WHERE id="+quote(graph)+";",1)
    if e!=nil{return nil,e};if len(g)!=1{return nil,errors.New("graph not found")}
    unconfirmed,e:=d.query("SELECT "+fieldsSQL("id")+" FROM rules WHERE graph="+quote(graph)+" AND approved<>1;",1);if e!=nil{return nil,e}
    tasks,e:=d.query("SELECT "+fieldsSQL("id","status")+" FROM tasks WHERE graph="+quote(graph)+";",2);if e!=nil{return nil,e}
    risks,e:=d.query("SELECT "+fieldsSQL("id")+" FROM risks WHERE graph="+quote(graph)+";",1);if e!=nil{return nil,e}
    passed:=0;for _,t:=range tasks{if t[1]=="PASS"{passed++}}
    valid:=g[0][0]=="1"&&len(unconfirmed)==0&&len(tasks)>0&&len(tasks)==passed
    verdict:="FAIL";if valid{verdict="PASS"}
    return map[string]any{"verdict":verdict,"catalog_approved":g[0][0]=="1","unconfirmed_rules":len(unconfirmed),"total_tasks":len(tasks),"passed":passed,"open_risks":len(risks)},nil
}
func (d db) trace(run string)([][]string,error){
    return d.query("SELECT "+fieldsSQL("r.id","r.task","r.status","s.expected","s.observed","s.verdict","s.decision","s.evidence_path","s.evidence_sha256")+" FROM runs r LEFT JOIN steps s ON r.id=s.run WHERE r.id="+quote(run)+";",9)
}
