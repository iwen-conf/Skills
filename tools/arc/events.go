package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "os"
    "strings"
    "time"
)

// eventCommand reads bounded, durable events; it does not acknowledge outbox
// rows or falsely claim that code indexing has completed.
func eventCommand(args []string) error {
    if len(args)==0 || args[0]!="list" { return errors.New("usage: arc event list [--repo DIR] [--resource KIND] [--id RESOURCE_ID] [--limit 1..200]") }
    fs:=flag.NewFlagSet("arc event list",flag.ContinueOnError)
    repo:=fs.String("repo",".","repository root")
    resource:=fs.String("resource","","aggregate kind")
    id:=fs.String("id","","aggregate ID")
    limit:=fs.Int("limit",50,"maximum event rows (1..200)")
    if err:=fs.Parse(args[1:]);err!=nil{return err}
    if len(fs.Args())!=0 {return fmt.Errorf("unexpected arguments: %v",fs.Args())}
    if *limit<1||*limit>200{return errors.New("--limit must be between 1 and 200")}
    if *id!=""&&*resource==""{return errors.New("--id requires --resource")}
    root,err:=repositoryRoot(*repo);if err!=nil{return err}
    return withArcLock(root,func()error{
        db,err:=openActiveArcDB(root);if err!=nil{return err}
        defer db.Close()
        return printEvents(db,*resource,*id,*limit)
    })
}
func printEvents(db *sql.DB,aggregate,aggregateID string,limit int)error{
    ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
    query:="SELECT id,aggregate,aggregate_id,kind,payload,created FROM arc_events"
    params:=[]any{}
    conditions:=[]string{}
    if aggregate!=""{conditions=append(conditions,"aggregate=?");params=append(params,aggregate)}
    if aggregateID!=""{conditions=append(conditions,"aggregate_id=?");params=append(params,aggregateID)}
    if len(conditions)!=0 {query+=" WHERE "+strings.Join(conditions," AND ")}
    query+=" ORDER BY created DESC,id DESC LIMIT ?"
    params=append(params,limit)
    rows,err:=db.QueryContext(ctx,query,params...);if err!=nil{return err}
    defer rows.Close()
    type event struct {
        ID string `json:"id"`
        Aggregate string `json:"resource"`
        AggregateID string `json:"resource_id"`
        Kind string `json:"kind"`
        Payload json.RawMessage `json:"payload"`
        Created string `json:"created_at"`
    }
    items:=make([]event,0,limit)
    for rows.Next(){
        var e event;var payload string
        if err:=rows.Scan(&e.ID,&e.Aggregate,&e.AggregateID,&e.Kind,&payload,&e.Created);err!=nil{return err}
        if !json.Valid([]byte(payload)){return fmt.Errorf("event %s has invalid JSON payload",e.ID)}
        e.Payload=json.RawMessage(payload)
        items=append(items,e)
    }
    if err:=rows.Err();err!=nil{return err}
    return json.NewEncoder(os.Stdout).Encode(map[string]any{"events":items,"count":len(items)})
}
