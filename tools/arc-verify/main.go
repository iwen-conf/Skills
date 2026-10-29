// arc-verify: short-lived Go CLI; local Turso file, no server or background worker.
package main

import (
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "os"
 "path/filepath"
 "strings"
)
func emit(v any){b,_:=json.MarshalIndent(v,"","  ");fmt.Println(string(b))}
func main(){if e:=run(os.Args[1:]);e!=nil{fmt.Fprintln(os.Stderr,"arc-verify:",e);os.Exit(1)}}
func run(args []string)error{
 if len(args)==0{return errors.New("usage: arc-verify <init|validate|import|approve|plan|next|start|record|trace|gate|risk> [options]")}
 name:=args[0]
 f:=flag.NewFlagSet(name,flag.ContinueOnError)
 dbfile:=f.String("db",".arc/verify/data.db","local Turso database path")
 file:=f.String("file","","business graph JSON file")
 graph:=f.String("graph","","graph id")
 rule:=f.String("rule","","rule id for approval")
 task:=f.String("task","","task id")
 runID:=f.String("run","","run id")
 env:=f.String("env","","approved environment alias")
 commit:=f.String("commit","","source commit")
 observed:=f.String("observed","","observed business state")
 evidence:=f.String("evidence","","local runner evidence path")
 config:=f.String("config",".arc/verify/config.jsonc","test data and environment config")
 if e:=f.Parse(args[1:]);e!=nil{return e}
 if len(f.Args())!=0{return fmt.Errorf("unexpected arguments %v",f.Args())}
 if name=="validate" {
    if *file==""{return errors.New("--file required")}
    g,e:=loadGraph(*file)
    if e==nil{emit(map[string]any{"valid":true,"graph":g.ID,"nodes":len(g.Nodes),"edges":len(g.Edges)})}
    return e
 }
 if name=="init" {
    d,e:=openDB(*dbfile,true);if e!=nil{return e}
    for _,s:=range schema{if _,e=d.sql(s);e!=nil{return e}}
    emit(map[string]string{"database":d.file,"status":"initialized"})
    return nil
 }
 if (name=="approve"||name=="plan"||name=="next"||name=="gate"||name=="risk")&&*graph==""{
     return errors.New("--graph required")
 }
 d,e:=openDB(*dbfile,false);if e!=nil{return e}
 switch name {
 case "import":
   if *file==""{return errors.New("--file required")}
   g,e:=loadGraph(*file);if e!=nil{return e}
   if e=d.importGraph(g);e==nil{emit(map[string]string{"graph":g.ID,"status":"candidate"})}
   return e
 case "approve":
   info,e:=os.Stdin.Stat();if e!=nil{return e}
   if info.Mode()&os.ModeCharDevice==0{return errors.New("human approval needs an interactive terminal; a pipe is not allowed")}
   approved:=*graph;if *rule!=""{approved=*rule}
   fmt.Fprintf(os.Stderr,"User approval for %s. Type APPROVE %s: ",approved,approved)
   var word,value string
   _,e=fmt.Fscanln(os.Stdin,&word,&value)
   if e!=nil||word!="APPROVE"||value!=approved{return errors.New("approval declined")}
   if e=d.approve(*graph,*rule);e==nil{emit(map[string]string{"approved":approved})}
   return e
 case "plan":
   n,e:=d.plan(*graph);if e==nil{emit(map[string]any{"graph":*graph,"task_candidates":n})};return e
 case "next":
   rows,e:=d.next(*graph);if e==nil{emit(rows)};return e
 case "start":
   if *task==""||*env==""||*commit==""{return errors.New("--task --env --commit required")}
   if e=approvedEnvironment(*config,*env);e!=nil{return e}
   id,e:=d.start(*task,*env,*commit);if e==nil{emit(map[string]string{"run":id,"status":"RUNNING"})};return e
 case "record":
   if *runID==""||*evidence==""||*observed==""{return errors.New("--run --observed --evidence required")}
   rawPath,e:=filepath.Abs(*evidence);if e!=nil{return e}
   v,e:=d.record(*runID,*observed,rawPath);if e==nil{emit(v)};return e
 case "trace":
   rows,e:=d.trace(*runID);if e==nil{emit(rows)};return e
 case "gate":
   v,e:=d.gate(*graph);if e!=nil{return e}
   emit(v);if v["verdict"]!="PASS"{return errors.New("graph gate not satisfied")};return nil
 case "risk":
   rows,e:=d.query("SELECT "+fieldsSQL("id","description","status")+" FROM risks WHERE graph="+quote(*graph)+";",3)
   if e==nil{emit(rows)};return e
 default:return fmt.Errorf("unknown command %q",name)
 }
}

// JSONC comments removed with a finite-state scanner; URLs and escapes inside strings survive.
func cleanJSONC(s string)(string,error){
 var b strings.Builder
 inString,escaped,line,block:=false,false,false,false
 for i:=0;i<len(s);i++{
  c:=s[i];next:=byte(0);if i+1<len(s){next=s[i+1]}
  if line{if c=='\n'{line=false;b.WriteByte('\n')};continue}
  if block{if c=='*'&&next=='/'{block=false;i++}else if c=='\n'{b.WriteByte('\n')};continue}
  if inString{b.WriteByte(c);if escaped{escaped=false}else if c=='\\'{escaped=true}else if c=='"'{inString=false};continue}
  if c=='"'{inString=true;b.WriteByte(c);continue}
  if c=='/'&&next=='/'{line=true;i++;continue}
  if c=='/'&&next=='*'{block=true;i++;continue}
  b.WriteByte(c)
 }
 if block||inString{return "",errors.New("unterminated JSONC string or comment")}
 return b.String(),nil
}
func approvedEnvironment(path,name string)error{
 raw,e:=os.ReadFile(path);if e!=nil{return e}
 s,e:=cleanJSONC(string(raw));if e!=nil{return e}
 var c struct {
    Environments map[string]struct{
       Approved bool `json:"approved"`
       URL string `json:"url"`
    } `json:"environments"`
 }
 if e=json.Unmarshal([]byte(s),&c);e!=nil{return e}
 target,ok:=c.Environments[name]
 if !ok||!target.Approved||target.URL==""{return fmt.Errorf("environment %s not explicitly approved in %s",name,path)}
 return nil
}
