package main

import (
 "crypto/rand"
 "crypto/sha256"
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
        return map[string]string{"run":run,"verdict":fmt.Sprint(detail["verdict"]),"decision":fmt.Sprint(detail["decision"])},nil
    }
    finished,e:=d.finish(run)
    if e!=nil{return nil,e}
    return map[string]string{"run":run,"verdict":fmt.Sprint(finished["verdict"])},nil
}
// gate scans the execution ledger once, in order, and re-reads each RAW
// exactly once. The previous implementation used 3+ SQL queries per
// PASS task (N+1), and loaded every step set repeatedly into memory.
func (d db) gate(graph string)(map[string]any,error){
    versions,e:=d.query("SELECT "+fieldsSQL("CAST(approved AS TEXT)")+" FROM graphs WHERE id="+quote(graph)+";",1)
    if e!=nil{return nil,e}
    if len(versions)!=1{return nil,errors.New("graph not found")}
    rules,e:=d.query("SELECT "+fieldsSQL("id","conditions","CAST(approved AS TEXT)")+" FROM rules WHERE graph="+quote(graph)+";",3)
    if e!=nil{return nil,e}
    required:=map[string]bool{}
    unconfirmed:=0
    for _,r:=range rules{
        if r[2]!="1"{unconfirmed++;continue}
        name:=graph+"/"+r[0]
        required[name+"/POSITIVE"]=true
        var conditions []string
        if e=json.Unmarshal([]byte(r[1]),&conditions);e!=nil{return nil,e}
        for _,c:=range conditions{
            required[name+"/NEGATIVE/"+c]=true
            required[name+"/FORCED/"+c]=true
        }
    }
    // The task, latest attempt, rule and every step are fetched in one
    // stable ordered query. Evidence bytes remain on disk until checked.
    statement:="SELECT "+fieldsSQL(
        "t.id","t.status","t.kind","t.condition","t.expected",
        "COALESCE(q.conditions,'[]')","COALESCE(r.id,'')","COALESCE(r.status,'')",
        "COALESCE(CAST(s.idx AS TEXT),'')","COALESCE(s.expected,'')",
        "COALESCE(s.observed,'')","COALESCE(s.verdict,'')",
        "COALESCE(s.evidence_path,'')","COALESCE(s.evidence_sha256,'')",
    )+" FROM tasks t LEFT JOIN rules q ON q.graph=t.graph AND q.id=t.rule "+
      "LEFT JOIN runs r ON r.rowid=(SELECT MAX(rr.rowid) FROM runs rr WHERE rr.task=t.id) "+
      "LEFT JOIN steps s ON s.run=r.id WHERE t.graph="+quote(graph)+
      " ORDER BY t.id,s.idx;"
    expectedCount:=len(required)
    total,passed,broken:=0,0,0
    current:=""
    currentPass:=false
    currentExpected:=""
    stepCount:=0
    lastExpected,lastObserved:="",""
    lastRAW:=StepRAW{}
    var lastRequired []string
    var lastKind,lastFailed string
    finishTask:=func(){
        if current==""{return}
        if stepCount==0||lastExpected!=currentExpected||lastObserved!=currentExpected{
            currentPass=false
        }
        if currentPass&&!conditionsMatch(lastRAW,lastKind,lastFailed,lastRequired){
            currentPass=false
        }
        if currentPass{passed++}else{broken++}
    }
    e=d.each(statement,14,func(row []string)error{
        id:=row[0]
        if id!=current{
            finishTask()
            current=id;total++;stepCount=0
            lastExpected="";lastObserved="";lastRAW=StepRAW{}
            currentPass=required[id]&&row[1]=="PASS"&&row[7]=="PASS"&&row[6]!=""
            lastKind,lastFailed,currentExpected=row[2],row[3],row[4]
            delete(required,id)
            if e=json.Unmarshal([]byte(row[5]),&lastRequired);e!=nil{return e}
        }
        if row[8]==""{return nil}
        stepCount++
        expectedIndex:=fmt.Sprint(stepCount)
        if row[8]!=expectedIndex||row[11]!="PASS"||row[9]!=row[10]{
            currentPass=false
        }
        lastExpected,lastObserved=row[9],row[10]
        if !currentPass{return nil}
        // A mutable RAW cannot be trusted from a stored verdict. Rehash it.
        _,raw,e:=loadEvidence(row[12])
        if e!=nil{currentPass=false;return nil}
        hash:=sha256.Sum256(raw)
        if hex.EncodeToString(hash[:])!=row[13]{currentPass=false;return nil}
        parsed,e:=parseStepRAW(raw,row[6],row[9],row[10],stepCount)
        if e!=nil||parsed.ExitCode!=0{currentPass=false;return nil}
        lastRAW=parsed
        return nil
    })
    if e!=nil{return nil,e}
    finishTask()
    risks,e:=d.query("SELECT "+fieldsSQL("CAST(COUNT(*) AS TEXT)")+" FROM risks WHERE graph="+quote(graph)+";",1)
    if e!=nil{return nil,e}
    openRisks:=0
    if len(risks)==1{if _,e=fmt.Sscan(risks[0][0],&openRisks);e!=nil{return nil,e}}
    missing:=len(required)
    ok:=versions[0][0]=="1"&&len(rules)>0&&unconfirmed==0&&
        total>0&&total==passed&&missing==0&&broken==0
    verdict:="FAIL";if ok{verdict="PASS"}
    return map[string]any{
        "verdict":verdict,"catalog_approved":versions[0][0]=="1",
        "confirmed_rules":len(rules)-unconfirmed,"unconfirmed_rules":unconfirmed,
        "required_tasks":expectedCount,"actual_tasks":total,
        "passed_with_valid_raw":passed,"missing_tasks":missing,
        "invalid_tasks_or_evidence":broken,"open_risks":openRisks,
    },nil
}
type StepTrace struct {
    Index string `json:"step"`
    Expected string `json:"expected"`
    Observed string `json:"observed"`
    Verdict string `json:"verdict"`
    Decision string `json:"decision"`
    Evidence string `json:"raw"`
    SHA256 string `json:"sha256"`
}
type TraceReport struct {
    Run string `json:"run"`
    Task string `json:"task"`
    Status string `json:"status"`
    FirstDivergence string `json:"first_divergence,omitempty"`
    LatestObserved string `json:"latest_observed,omitempty"`
    ContinueDecision string `json:"continue_decision"`
    Steps []StepTrace `json:"steps"`
}
func (d db) trace(run string)(TraceReport,error){
    rows,e:=d.query("SELECT "+fieldsSQL("r.id","r.task","r.status","COALESCE(CAST(s.idx AS TEXT),'')","COALESCE(s.expected,'')","COALESCE(s.observed,'')","COALESCE(s.verdict,'')","COALESCE(s.decision,'')","COALESCE(s.evidence_path,'')","COALESCE(s.evidence_sha256,'')")+" FROM runs r LEFT JOIN steps s ON r.id=s.run WHERE r.id="+quote(run)+" ORDER BY s.idx;",10)
    if e!=nil{return TraceReport{},e}
    if len(rows)==0{return TraceReport{},errors.New("run not found")}
    out:=TraceReport{Run:run,Task:rows[0][1],Status:rows[0][2],ContinueDecision:"NONE",Steps:[]StepTrace{}}
    for _,r:=range rows {
        if r[3]==""{continue}
        out.Steps=append(out.Steps,StepTrace{Index:r[3],Expected:r[4],Observed:r[5],Verdict:r[6],Decision:r[7],Evidence:r[8],SHA256:r[9]})
        out.LatestObserved=r[5]
        if r[6]=="FAIL" && out.FirstDivergence=="" {
            out.FirstDivergence=r[3]
            out.ContinueDecision=r[7]
        }
    }
    if out.Status=="RUNNING"{out.ContinueDecision="CONTINUE"}
    if out.Status=="BLOCKED"{out.ContinueDecision="RESET"}
    return out,nil
}
