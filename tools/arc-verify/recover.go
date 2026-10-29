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
func (d db) recover(graph,condition string,maxDepth int)(RecoveryGraph,error){
    if maxDepth<1||maxDepth>8{return RecoveryGraph{},errors.New("max-depth must be 1..8")}
    exists,e:=d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(graph)+" AND id="+quote(condition)+" AND kind='condition';",1)
    if e!=nil{return RecoveryGraph{},e};if len(exists)!=1{return RecoveryGraph{},fmt.Errorf("unknown condition %s",condition)}
    edgesRows,e:=d.query("SELECT "+fieldsSQL("id","source","target","kind")+" FROM edges WHERE graph="+quote(graph)+" AND kind IN ('recovers','requires');",4)
    if e!=nil{return RecoveryGraph{},e}
    edges:=[]Edge{}
    for _,row:=range edgesRows{edges=append(edges,Edge{ID:row[0],From:row[1],To:row[2],Kind:row[3]})}
    rulesRows,e:=d.query("SELECT "+fieldsSQL("action","actor","entry","oracle","CAST(approved AS TEXT)")+" FROM rules WHERE graph="+quote(graph)+";",5)
    if e!=nil{return RecoveryGraph{},e}
    rules:=[]RecoveryRule{}
    for _,row:=range rulesRows{rules=append(rules,RecoveryRule{Action:row[0],Actor:row[1],Entry:row[2],Oracle:row[3],Approved:row[4]=="1"})}
    return resolveRecovery(condition,edges,rules,maxDepth),nil
}
