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
        switch e.Kind {case "contains","requires","transitions","recovers","forbids":default:return fmt.Errorf("invalid edge kind %q",e.Kind)}
        if _,ok:=ids[e.ID];ok{return fmt.Errorf("duplicate id %q",e.ID)}
        ids[e.ID]="edge"
    }
    for _,r:=range g.Rules {
        if r.ID==""||ids[r.Action]!="action"||r.Expected==""||r.Rejected==""||strings.TrimSpace(r.Description)==""{return fmt.Errorf("invalid rule %s",r.ID)}
        if r.Source!="user"&&r.Source!="document"&&r.Source!="derived"{return fmt.Errorf("rule %s has invalid source",r.ID)}
        if _,ok:=ids[r.ID];ok{return fmt.Errorf("duplicate id %s",r.ID)}
        ids[r.ID]="rule"
        for _,c:=range r.Conditions{if ids[c]!="condition"{return fmt.Errorf("rule %s has unknown condition %s",r.ID,c)}}
    }
    for _,r:=range g.Risks {
        if r.ID==""||r.Description==""||ids[r.Action]!="action"{return fmt.Errorf("invalid risk %s",r.ID)}
        if _,ok:=ids[r.ID];ok{return fmt.Errorf("duplicate id %s",r.ID)}
    }
    return nil
}
