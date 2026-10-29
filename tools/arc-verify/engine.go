package main

import (
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
)

func (d db) importGraph(g Graph)error{
    rows,e:=d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(g.ID)+";",1)
    if e!=nil{return e};if len(rows)>0{return errors.New("graph version already exists; publish a new graph ID")}
    stmts:=[]string{fmt.Sprintf("INSERT INTO graphs(id,scope) VALUES(%s,%s);",quote(g.ID),quote(g.Scope))}
    for _,n:=range g.Nodes{stmts=append(stmts,fmt.Sprintf("INSERT INTO nodes VALUES(%s,%s,%s,%s);",quote(g.ID),quote(n.ID),quote(n.Kind),quote(n.Name)))}
    for _,e:=range g.Edges{stmts=append(stmts,fmt.Sprintf("INSERT INTO edges VALUES(%s,%s,%s,%s,%s);",quote(g.ID),quote(e.ID),quote(e.From),quote(e.To),quote(e.Kind)))}
    for _,r:=range g.Rules{
       cond,_:=json.Marshal(r.Conditions)
       stmts=append(stmts,fmt.Sprintf("INSERT INTO rules(graph,id,action,actor,entry,oracle,conditions,expected,rejected,source,description) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);",quote(g.ID),quote(r.ID),quote(r.Action),quote(r.Actor),quote(r.Entry),quote(r.Oracle),quote(string(cond)),quote(r.Expected),quote(r.Rejected),quote(r.Source),quote(r.Description)))
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
type TaskCard struct {
    ID string `json:"id"`
    Rule string `json:"rule"`
    Type string `json:"type"`
    Actor string `json:"actor"`
    Entry string `json:"entry"`
    Action string `json:"action"`
    Preconditions map[string]string `json:"preconditions"`
    Expected string `json:"expected"`
    Oracle string `json:"oracle"`
    Instruction string `json:"instruction"`
}
func (d db) next(graph string)([]TaskCard,error){
    rows,e:=d.query("SELECT "+fieldsSQL("t.id","t.rule","t.kind","t.condition","t.expected","r.action","r.actor","r.entry","r.oracle","r.conditions")+" FROM tasks t JOIN rules r ON t.graph=r.graph AND t.rule=r.id WHERE t.graph="+quote(graph)+" AND t.status='READY' AND r.approved=1 ORDER BY t.id LIMIT 8;",10)
    if e!=nil{return nil,e}
    cards:=make([]TaskCard,0,len(rows))
    for _,r:=range rows{
        var conditions []string
        if e=json.Unmarshal([]byte(r[9]),&conditions);e!=nil{return nil,e}
        conditionsMap:=map[string]string{}
        for _,c:=range conditions{
            conditionsMap[c]="SATISFIED"
            if (r[2]=="NEGATIVE"||r[2]=="FORCED")&&c==r[3]{conditionsMap[c]="UNSATISFIED"}
        }
        instruction:="From a real client, perform the action and verify independent readback."
        if r[2]=="NEGATIVE"{instruction="Perform the normal UI operation with the failed condition, verify rejection and no state change."}
        if r[2]=="FORCED"{instruction="In an authorized isolated environment, attempt the operation via the legitimate API with the failed condition; expect server-side rejection and no state change."}
        cards=append(cards,TaskCard{ID:r[0],Rule:r[1],Type:r[2],Actor:r[6],Entry:r[7],Action:r[5],Preconditions:conditionsMap,Expected:r[4],Oracle:r[8],Instruction:instruction})
    }
    return cards,nil
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
    detail,e:=d.step(run,"",observed,evidence)
    if e!=nil{return nil,e}
    if detail["verdict"]!="PASS"{
        return map[string]string{"run":run,"verdict":"FAIL","decision":fmt.Sprint(detail["decision"])},nil
    }
    finished,e:=d.finish(run)
    if e!=nil{return nil,e}
    return map[string]string{"run":run,"verdict":fmt.Sprint(finished["verdict"])},nil
}
func (d db) gate(graph string)(map[string]any,error){
    g,e:=d.query("SELECT "+fieldsSQL("approved")+" FROM graphs WHERE id="+quote(graph)+";",1)
    if e!=nil{return nil,e}
    if len(g)!=1{return nil,errors.New("graph not found")}
    rules,e:=d.query("SELECT "+fieldsSQL("id","conditions","approved")+" FROM rules WHERE graph="+quote(graph)+";",3)
    if e!=nil{return nil,e}
    expectedTasks:=map[string]bool{}
    unconfirmed:=0
    for _,r:=range rules{
        if r[2]!="1"{unconfirmed++;continue}
        id:=graph+"/"+r[0]
        expectedTasks[id+"/POSITIVE"]=true
        var conditions []string
        if e=json.Unmarshal([]byte(r[1]),&conditions);e!=nil{return nil,e}
        for _,c:=range conditions{
            expectedTasks[id+"/NEGATIVE/"+c]=true
            expectedTasks[id+"/FORCED/"+c]=true
        }
    }
    tasks,e:=d.query("SELECT "+fieldsSQL("id","status")+" FROM tasks WHERE graph="+quote(graph)+";",2)
    if e!=nil{return nil,e}
    present:=map[string]bool{};passed,broken:=0,0
    for _,t:=range tasks{
        present[t[0]]=true
        if !expectedTasks[t[0]]{broken++;continue}
        if t[1]!="PASS"{continue}
        runs,e:=d.query("SELECT "+fieldsSQL("id","status")+" FROM runs WHERE task="+quote(t[0])+" ORDER BY rowid DESC LIMIT 1;",2)
        if e!=nil{return nil,e}
        if len(runs)!=1||runs[0][1]!="PASS"{broken++;continue}
        steps,e:=d.query("SELECT "+fieldsSQL("CAST(idx AS TEXT)","expected","observed","verdict","evidence_path","evidence_sha256")+" FROM steps WHERE run="+quote(runs[0][0])+" ORDER BY idx;",6)
        if e!=nil{return nil,e}
        if len(steps)==0{broken++;continue}
        trustworthy:=true
        for i,s:=range steps{
            if s[0]!=fmt.Sprint(i+1)||s[3]!="PASS"||s[1]!=s[2]{
                trustworthy=false;break
            }
            if e=validateEvidenceHash(s[4],s[5]);e!=nil{trustworthy=false;break}
            _,raw,e:=loadEvidence(s[4]);if e!=nil{trustworthy=false;break}
            v,e:=parseStepRAW(raw,runs[0][0],s[1],s[2],i+1)
            if e!=nil||v.ExitCode!=0{trustworthy=false;break}
        }
        last:=steps[len(steps)-1]
        want,e:=d.query("SELECT "+fieldsSQL("expected")+" FROM tasks WHERE id="+quote(t[0])+";",1)
        if e!=nil{return nil,e}
        if len(want)!=1||last[1]!=want[0][0]{trustworthy=false}
        if trustworthy{passed++}else{broken++}
    }
    missing:=0
    for id:=range expectedTasks{if !present[id]{missing++}}
    risks,e:=d.query("SELECT "+fieldsSQL("id")+" FROM risks WHERE graph="+quote(graph)+";",1)
    if e!=nil{return nil,e}
    valid:=g[0][0]=="1"&&len(rules)>0&&unconfirmed==0&&len(tasks)==len(expectedTasks)&&missing==0&&broken==0&&passed==len(expectedTasks)
    verdict:="FAIL";if valid{verdict="PASS"}
    return map[string]any{
        "verdict":verdict,"catalog_approved":g[0][0]=="1",
        "confirmed_rules":len(rules)-unconfirmed,"unconfirmed_rules":unconfirmed,
        "required_tasks":len(expectedTasks),"actual_tasks":len(tasks),
        "passed_with_valid_raw":passed,"missing_tasks":missing,
        "invalid_tasks_or_evidence":broken,"open_risks":len(risks),
    },nil
}
func (d db) trace(run string)([][]string,error){
    return d.query("SELECT "+fieldsSQL("r.id","r.task","r.status","COALESCE(s.expected,'')","COALESCE(s.observed,'')","COALESCE(s.verdict,'')","COALESCE(s.decision,'')","COALESCE(s.evidence_path,'')","COALESCE(s.evidence_sha256,'')")+" FROM runs r LEFT JOIN steps s ON r.id=s.run WHERE r.id="+quote(run)+";",9)
}
