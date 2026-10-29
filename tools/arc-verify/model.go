package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "strings"
)

type Node struct { ID string `json:"id"`; Kind string `json:"kind"`; Name string `json:"name"` }
type Edge struct { ID string `json:"id"`; From string `json:"from"`; To string `json:"to"`; Kind string `json:"kind"` }
type Rule struct {
    ID string `json:"id"`
    Action string `json:"action"`
    Actor string `json:"actor"`
    Entry string `json:"entry"`
    Oracle string `json:"oracle"`
    Conditions []string `json:"conditions"`
    Expected string `json:"expected"`
    Rejected string `json:"rejected"`
    Source string `json:"source"`
    Description string `json:"description"`
}
type Risk struct { ID string `json:"id"`; Description string `json:"description"`; Action string `json:"action"` }
type Graph struct {
    ID string `json:"id"`
    Scope string `json:"scope"`
    Nodes []Node `json:"nodes"`
    Edges []Edge `json:"edges"`
    Rules []Rule `json:"rules"`
    Risks []Risk `json:"risks"`
}
func loadGraph(path string) (Graph,error) {
    var g Graph
    b,e:=os.ReadFile(path); if e!=nil{return g,e}
    if e=json.Unmarshal(b,&g);e!=nil{return g,e}
    return g,validateGraph(g)
}
func validateGraph(g Graph) error {
    if g.ID==""||g.Scope==""{return errors.New("graph id and scope required")}
    ids:=map[string]string{}
    for _,n:=range g.Nodes {
        if n.ID==""||n.Name==""{return errors.New("node requires id and name")}
        switch n.Kind {case "feature","action","condition","state":default:return fmt.Errorf("node %s has unsupported kind %s",n.ID,n.Kind)}
        if _,ok:=ids[n.ID];ok{return fmt.Errorf("duplicate node %s",n.ID)}
        ids[n.ID]=n.Kind
    }
    for _,e:=range g.Edges {
        if e.ID==""||ids[e.From]==""||ids[e.To]==""{return fmt.Errorf("edge %s has missing endpoint",e.ID)}
        switch e.Kind {case "contains","requires","transitions","recovers","forbids","next":default:return fmt.Errorf("invalid edge kind %q",e.Kind)}
        if _,ok:=ids[e.ID];ok{return fmt.Errorf("duplicate id %q",e.ID)}
        switch e.Kind {
        case "contains":
            if ids[e.From]!="feature"||ids[e.To]!="action"{return fmt.Errorf("contains %s must be feature -> action",e.ID)}
        case "requires","forbids":
            if ids[e.From]!="condition"||ids[e.To]!="action"{return fmt.Errorf("%s %s must be condition -> action",e.Kind,e.ID)}
        case "transitions":
            if ids[e.From]!="action"||ids[e.To]!="state"{return fmt.Errorf("transitions %s must be action -> state",e.ID)}
        case "recovers":
            if ids[e.From]!="action"||ids[e.To]!="condition"{return fmt.Errorf("recovers %s must be action -> condition",e.ID)}
        case "next":
            if ids[e.From]!="state"||ids[e.To]!="action"{return fmt.Errorf("next %s must be state -> action",e.ID)}
        }
        ids[e.ID]="edge"
    }
    for _,r:=range g.Rules {
        if r.ID==""||ids[r.Action]!="action"||r.Expected==""||r.Rejected==""||strings.TrimSpace(r.Description)==""||r.Actor==""||r.Entry==""||r.Oracle==""{return fmt.Errorf("invalid rule %s",r.ID)}
        if ids[r.Expected]!="state"{return fmt.Errorf("rule %s expected is not a state node",r.ID)}
        if r.Rejected!="UNCHANGED"&&ids[r.Rejected]!="state"{return fmt.Errorf("rule %s rejected is not a state node or UNCHANGED",r.ID)}
        if r.Source!="user"&&r.Source!="document"{return fmt.Errorf("rule %s must come from user or explicit document; inferred requirements belong in risks",r.ID)}
        if _,ok:=ids[r.ID];ok{return fmt.Errorf("duplicate id %s",r.ID)}
        ids[r.ID]="rule"
        if len(r.Conditions)>64{return fmt.Errorf("rule %s exceeds 64 preconditions",r.ID)}
        seenConditions:=map[string]bool{}
        for _,c:=range r.Conditions{
            if ids[c]!="condition"||seenConditions[c]{return fmt.Errorf("rule %s has invalid or duplicated condition %s",r.ID,c)}
            seenConditions[c]=true
        }
        connected:=map[string]bool{}
        for _,e:=range g.Edges{
            if e.Kind=="requires"&&e.To==r.Action{connected[e.From]=true}
        }
        for _,c:=range r.Conditions{if !connected[c]{return fmt.Errorf("rule %s condition %s missing requires edge",r.ID,c)}}
        hasTransition:=false
        for _,e:=range g.Edges{if e.Kind=="transitions"&&e.From==r.Action&&e.To==r.Expected{hasTransition=true}}
        if !hasTransition{return fmt.Errorf("rule %s missing expected transition",r.ID)}
    }
    for _,r:=range g.Risks {
        if r.ID==""||r.Description==""||ids[r.Action]!="action"{return fmt.Errorf("invalid risk %s",r.ID)}
        if _,ok:=ids[r.ID];ok{return fmt.Errorf("duplicate id %s",r.ID)}
    }
    return nil
}
