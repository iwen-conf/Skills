package main

import (
    "errors"
    "fmt"
    "sort"
)

type RecoveryAction struct {
    Condition string `json:"condition"`
    Action string `json:"action"`
    Approved bool `json:"approved"`
    Actor string `json:"actor,omitempty"`
    Entry string `json:"entry,omitempty"`
    Oracle string `json:"oracle,omitempty"`
    Prerequisites []string `json:"prerequisites,omitempty"`
}
type RecoveryGraph struct {
    Root string `json:"root"`
    Actions []RecoveryAction `json:"actions"`
    Unresolved []string `json:"unresolved_conditions"`
    Cycles []string `json:"cycles,omitempty"`
    Truncated bool `json:"truncated"`
}
type RecoveryRule struct {Action,Actor,Entry,Oracle string;Approved bool}
func resolveRecovery(root string,edges []Edge,rules []RecoveryRule,maxDepth int)RecoveryGraph{
    result:=RecoveryGraph{Root:root,Actions:[]RecoveryAction{},Unresolved:[]string{}}
    ruleByAction:=map[string]RecoveryRule{}
    for _,r:=range rules{if r.Approved{ruleByAction[r.Action]=r}}
    providers:=map[string][]string{}
    needs:=map[string][]string{}
    for _,e:=range edges {
        if e.Kind=="recovers"{providers[e.To]=append(providers[e.To],e.From)}
        if e.Kind=="requires"{needs[e.To]=append(needs[e.To],e.From)}
    }
    for k:=range providers{sort.Strings(providers[k])}
    for k:=range needs{sort.Strings(needs[k])}
    expanded:=map[string]bool{}
    unresolved:=map[string]bool{}
    cycles:=map[string]bool{}
    var walk func(string,int,map[string]bool)
    walk=func(cond string,depth int,ancestors map[string]bool){
        if depth>maxDepth || len(result.Actions)>=128{result.Truncated=true;return}
        if ancestors[cond]{cycles[cond]=true;return}
        if expanded[cond]{return}
        expanded[cond]=true
        if len(providers[cond])==0{unresolved[cond]=true;return}
        nextAncestors:=map[string]bool{}
        for k,v:=range ancestors{nextAncestors[k]=v};nextAncestors[cond]=true
        for _,action:=range providers[cond]{
            r,ok:=ruleByAction[action]
            ar:=RecoveryAction{Condition:cond,Action:action,Approved:ok&&r.Approved,Prerequisites:needs[action]}
            if ar.Approved {ar.Actor=r.Actor;ar.Entry=r.Entry;ar.Oracle=r.Oracle}
            result.Actions=append(result.Actions,ar)
            if !ar.Approved{unresolved[cond]=true;continue}
            for _,dep:=range needs[action]{walk(dep,depth+1,nextAncestors)}
        }
    }
    walk(root,0,map[string]bool{})
    for k:=range unresolved{result.Unresolved=append(result.Unresolved,k)}
    for k:=range cycles{result.Cycles=append(result.Cycles,k)}
    sort.Strings(result.Unresolved);sort.Strings(result.Cycles)
    return result
}
// recover fetches only the reachable local graph slice. Even if the overall
// repository has millions of nodes, a command never loads the entire graph.
func (d db) recover(graph,condition string,maxDepth int)(RecoveryGraph,error){
    if maxDepth<1||maxDepth>8{return RecoveryGraph{},errors.New("max-depth must be 1..8")}
    exists,e:=d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(graph)+" AND id="+quote(condition)+" AND kind='condition';",1)
    if e!=nil{return RecoveryGraph{},e}
    if len(exists)!=1{return RecoveryGraph{},fmt.Errorf("unknown condition %s",condition)}
    result:=RecoveryGraph{Root:condition,Actions:[]RecoveryAction{},Unresolved:[]string{}}
    visited:=map[string]bool{}
    unresolved:=map[string]bool{}
    cycles:=map[string]bool{}
    var walk func(string,int,map[string]bool)error
    walk=func(cond string,depth int,ancestors map[string]bool)error{
        if depth>maxDepth||len(result.Actions)>=128{result.Truncated=true;return nil}
        if ancestors[cond]{cycles[cond]=true;return nil}
        if visited[cond]{return nil}
        visited[cond]=true
        // One bounded indexed query per explored prerequisite. This replaces
        // loading every edge and rule regardless of relevance.
        q:="SELECT "+fieldsSQL(
            "e.source","COALESCE(r.actor,'')","COALESCE(r.entry,'')",
            "COALESCE(r.oracle,'')","CASE WHEN r.id IS NULL THEN '0' ELSE '1' END",
            "COALESCE(dep.source,'')",
        )+" FROM edges e "+
          "LEFT JOIN rules r ON r.rowid=("+
            "SELECT rr.rowid FROM rules rr WHERE rr.graph=e.graph AND rr.action=e.source "+
            "AND rr.approved=1 ORDER BY rr.id LIMIT 1) "+
          "LEFT JOIN edges dep ON dep.graph=e.graph AND dep.kind='requires' AND dep.target=e.source "+
          "WHERE e.graph="+quote(graph)+" AND e.kind='recovers' AND e.target="+quote(cond)+
          " ORDER BY e.source,dep.source LIMIT 513;"
        rows,e:=d.query(q,6)
        if e!=nil{return e}
        if len(rows)==513{result.Truncated=true;rows=rows[:512]}
        if len(rows)==0{unresolved[cond]=true;return nil}
        nextAncestors:=make(map[string]bool,len(ancestors)+1)
        for k,v:=range ancestors{nextAncestors[k]=v}
        nextAncestors[cond]=true
        for i:=0;i<len(rows);{
            if len(result.Actions)>=128{result.Truncated=true;return nil}
            name:=rows[i][0]
            entry:=RecoveryAction{
                Condition:cond,Action:name,Approved:rows[i][4]=="1",
                Actor:rows[i][1],Entry:rows[i][2],Oracle:rows[i][3],
                Prerequisites:[]string{},
            }
            for i<len(rows)&&rows[i][0]==name{
                if rows[i][5]!=""{entry.Prerequisites=append(entry.Prerequisites,rows[i][5])}
                i++
            }
            result.Actions=append(result.Actions,entry)
            if !entry.Approved{unresolved[cond]=true;continue}
            for _,dependent:=range entry.Prerequisites{
                if err:=walk(dependent,depth+1,nextAncestors);err!=nil{return err}
            }
        }
        return nil
    }
    if e=walk(condition,0,map[string]bool{});e!=nil{return RecoveryGraph{},e}
    for k:=range unresolved{result.Unresolved=append(result.Unresolved,k)}
    for k:=range cycles{result.Cycles=append(result.Cycles,k)}
    sort.Strings(result.Unresolved);sort.Strings(result.Cycles)
    return result,nil
}
