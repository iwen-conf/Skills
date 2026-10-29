package main

import (
    "errors"
    "fmt"
)

// HandoffPacket is a bounded, read-only view of the existing graph and run.
// It never assigns approval, modifies tasks, changes a verdict or grants authority.
type HandoffPacket struct {
    To string `json:"to"`
    Graph string `json:"graph_version"`
    Rule string `json:"rule,omitempty"`
    Run string `json:"run,omitempty"`
    Status string `json:"status,omitempty"`
    SourceCommit string `json:"source_commit,omitempty"`
    Environment string `json:"environment,omitempty"`
    Actor string `json:"actor,omitempty"`
    Entry string `json:"entry,omitempty"`
    Action string `json:"action,omitempty"`
    Oracle string `json:"oracle,omitempty"`
    FirstDivergence string `json:"first_divergence,omitempty"`
    Expected string `json:"expected,omitempty"`
    Observed string `json:"observed,omitempty"`
    Evidence string `json:"evidence,omitempty"`
    NextDecision string `json:"next_decision,omitempty"`
    Candidates []TaskCard `json:"ready_tasks,omitempty"`
    PendingRules [][]string `json:"unconfirmed_rules,omitempty"`
    Risks [][]string `json:"risks,omitempty"`
    ImplementationGaps [][]string `json:"implementation_gaps,omitempty"`
    Actions []string `json:"recommended_next_actions"`
    Truncated bool `json:"truncated,omitempty"`
}

func validateHandoffTarget(target string)error{
    switch target{
    case "arc-explore","arc-plan","arc-code","arc-verify","arc-audit","arc-vet":return nil
    default:return fmt.Errorf("unsupported Arc handoff target %q",target)
    }
}
func firstDivergence(steps []StepTrace)(StepTrace,bool){
    for _,s:=range steps{if s.Verdict=="FAIL"{return s,true}}
    return StepTrace{},false
}
func takeRows(rows [][]string,max int)([][]string,bool){
    if len(rows)<=max{return rows,false}
    return rows[:max],true
}
func (d db) handoff(to,graph,runID string)(HandoffPacket,error){
    if e:=validateHandoffTarget(to);e!=nil{return HandoffPacket{},e}
    if graph==""&&runID==""{return HandoffPacket{},errors.New("--graph or --run required")}
    packet:=HandoffPacket{To:to,Graph:graph,Actions:[]string{}}
    if runID!=""{
        rows,e:=d.query("SELECT "+fieldsSQL("r.environment","r.commit_sha","r.status","t.graph","t.rule","q.actor","q.entry","q.action","q.oracle")+" FROM runs r JOIN tasks t ON r.task=t.id JOIN rules q ON q.graph=t.graph AND q.id=t.rule WHERE r.id="+quote(runID)+";",9)
        if e!=nil{return HandoffPacket{},e}
        if len(rows)!=1{return HandoffPacket{},errors.New("run not found")}
        row:=rows[0]
        if graph!=""&&row[3]!=graph{return HandoffPacket{},errors.New("run does not belong to supplied graph version")}
        packet.Graph=row[3];packet.Run=runID;packet.Environment=row[0];packet.SourceCommit=row[1]
        packet.Status=row[2];packet.Rule=row[4];packet.Actor=row[5]
        packet.Entry=row[6];packet.Action=row[7];packet.Oracle=row[8]
        trace,e:=d.trace(runID);if e!=nil{return HandoffPacket{},e}
        if failure,ok:=firstDivergence(trace.Steps);ok{
            packet.FirstDivergence=failure.Index
            packet.Expected=failure.Expected;packet.Observed=failure.Observed
            packet.Evidence=failure.Evidence;packet.NextDecision=failure.Decision
        }
        if to=="arc-code"&&packet.Status!="FAIL"{
            return HandoffPacket{},errors.New("arc-code accepts only confirmed failed run; BLOCKED/unknown or risk returns to verify/plan")
        }
        if to=="arc-code"&&packet.FirstDivergence==""{
            return HandoffPacket{},errors.New("failed run has no observed divergent step")
        }
    }
    if packet.Graph==""{return HandoffPacket{},errors.New("graph ID is missing")}
    exists,e:=d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(packet.Graph)+";",1)
    if e!=nil{return HandoffPacket{},e}
    if len(exists)!=1{return HandoffPacket{},errors.New("graph version not found")}
    switch to{
    case "arc-verify":
        cards,e:=d.next(packet.Graph);if e!=nil{return HandoffPacket{},e}
        packet.Candidates=cards
        packet.Actions=[]string{"select a READY task with a real authorized environment","record independent RAW with step observations","run graph gate after every required task"}
    case "arc-code":
        packet.Actions=[]string{"inspect independent RAW and code locations via arc-idx","confirm root cause before source edits","repair implementation without modifying frozen rule, RAW or earlier FAIL","return to arc-verify for new real E2E run"}
    case "arc-plan":
        unconfirmed,e:=d.query("SELECT "+fieldsSQL("id","description","source")+" FROM rules WHERE graph="+quote(packet.Graph)+" AND approved<>1 ORDER BY id LIMIT 9;",3)
        if e!=nil{return HandoffPacket{},e}
        packet.PendingRules,packet.Truncated=takeRows(unconfirmed,8)
        risks,e:=d.query("SELECT "+fieldsSQL("id","description","status")+" FROM risks WHERE graph="+quote(packet.Graph)+" ORDER BY id LIMIT 9;",3)
        if e!=nil{return HandoffPacket{},e}
        packet.Risks,more:=takeRows(risks,8);packet.Truncated=packet.Truncated||more
        packet.Actions=[]string{"review business intent in the existing PRD/closure model","obtain explicit user confirmation before changing any RULE","hand confirmed contract and unchanged IDs to arc-verify"}
    case "arc-explore":
        gaps,e:=d.gaps(packet.Graph);if e!=nil{return HandoffPacket{},e}
        packet.ImplementationGaps,packet.Truncated=takeRows(gaps,8)
        packet.Actions=[]string{"trace entry points via arc-idx and cite file:line","treat NOT_FOUND as an observation requiring more evidence","send implementation facts to arc-verify without changing local graph"}
    case "arc-audit":
        gaps,e:=d.gaps(packet.Graph);if e!=nil{return HandoffPacket{},e}
        packet.ImplementationGaps,packet.Truncated=takeRows(gaps,8)
        packet.Actions=[]string{"read-only review source claims and independent evidence","separate missing product rules from unconfirmed risks","send verified problems to arc-code or unresolved requirements to arc-plan"}
    case "arc-vet":
        packet.Actions=[]string{"only if a new external runner/library is required, review existing options and licenses","keep Go CLI, local Turso and no daemon constraints","return a versioned decision record to arc-code"}
    }
    return packet,nil
}
