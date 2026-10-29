package main

import (
    "bytes"
    "context"
    "encoding/hex"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

// Direct local-file access using the official tursodb CLI. No server, remote token or daemon.
type db struct { file string }
func openDB(path string, create bool)(db,error){
    if _,e:=exec.LookPath("tursodb");e!=nil{return db{},errors.New("tursodb missing: install official Turso Database SQL CLI")}
    abs,e:=filepath.Abs(path);if e!=nil{return db{},e}
    if create {if e=os.MkdirAll(filepath.Dir(abs),0700);e!=nil{return db{},e}} else {
        if st,e:=os.Stat(abs);e!=nil||st.IsDir(){return db{},fmt.Errorf("local database not initialized: %s",abs)}
    }
    return db{file:abs},nil
}
func (d db) sql(script string)(string,error){
    ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
    cmd:=exec.CommandContext(ctx,"tursodb","-q","-m","list",d.file)
    cmd.Stdin=strings.NewReader(script+"\n")
    var stdout,stderr bytes.Buffer;cmd.Stdout=&stdout;cmd.Stderr=&stderr
    if e:=cmd.Run();e!=nil{return "",fmt.Errorf("tursodb exited: %w: %s",e,strings.TrimSpace(stderr.String()))}
    if ctx.Err()!=nil{return "",ctx.Err()}
    if stderr.Len()!=0{return "",fmt.Errorf("tursodb SQL error: %s",strings.TrimSpace(stderr.String()))}
    return strings.TrimSpace(stdout.String()),nil
}
func quote(v string)string{return "'"+strings.ReplaceAll(v,"'","''")+"'"}
func fieldsSQL(columns ...string)string{
    a:=make([]string,len(columns));for i,c:=range columns{a[i]="hex("+c+")"};return strings.Join(a,"||'|'||")
}
func (d db) query(script string,columns int)([][]string,error){
    out,e:=d.sql(script);if e!=nil{return nil,e};if out==""{return [][]string{},nil}
    result:=[][]string{}
    for _,line:=range strings.Split(out,"\n"){
        parts:=strings.Split(line,"|");if len(parts)!=columns{return nil,fmt.Errorf("tursodb unexpected columns: %d != %d",len(parts),columns)}
        row:=make([]string,columns)
        for i,p:=range parts{b,e:=hex.DecodeString(p);if e!=nil{return nil,e};row[i]=string(b)}
        result=append(result,row)
    }
    return result,nil
}
func (d db) tx(stmts []string)error{
    if len(stmts)==0{return nil}
    _,e:=d.sql("BEGIN IMMEDIATE;\n"+strings.Join(stmts,"\n")+"\nCOMMIT;");return e
}
var schema=[]string{
    "CREATE TABLE IF NOT EXISTS graphs (id TEXT PRIMARY KEY, scope TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0);",
    "CREATE TABLE IF NOT EXISTS nodes (graph TEXT NOT NULL, id TEXT NOT NULL, kind TEXT NOT NULL, name TEXT NOT NULL, PRIMARY KEY(graph,id));",
    "CREATE TABLE IF NOT EXISTS edges (graph TEXT NOT NULL, id TEXT NOT NULL, source TEXT NOT NULL, target TEXT NOT NULL, kind TEXT NOT NULL, PRIMARY KEY(graph,id));",
    "CREATE TABLE IF NOT EXISTS rules (graph TEXT NOT NULL, id TEXT NOT NULL, action TEXT NOT NULL, actor TEXT NOT NULL, entry TEXT NOT NULL, oracle TEXT NOT NULL, conditions TEXT NOT NULL, expected TEXT NOT NULL, rejected TEXT NOT NULL, source TEXT NOT NULL, description TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(graph,id));",
    "CREATE TABLE IF NOT EXISTS risks (graph TEXT NOT NULL, id TEXT NOT NULL, action TEXT NOT NULL, description TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'OPEN', PRIMARY KEY(graph,id));",
    "CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, graph TEXT NOT NULL, rule TEXT NOT NULL, kind TEXT NOT NULL, condition TEXT NOT NULL, expected TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'READY');",
    "CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, task TEXT NOT NULL, environment TEXT NOT NULL, commit_sha TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'RUNNING', created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
    "CREATE TABLE IF NOT EXISTS steps (run TEXT NOT NULL, idx INTEGER NOT NULL, expected TEXT NOT NULL, observed TEXT NOT NULL, verdict TEXT NOT NULL, decision TEXT NOT NULL, evidence_path TEXT NOT NULL, evidence_sha256 TEXT NOT NULL, PRIMARY KEY(run,idx));",
    "CREATE TABLE IF NOT EXISTS fixtures (id TEXT PRIMARY KEY, body TEXT NOT NULL);",
"CREATE TABLE IF NOT EXISTS implementation_scans (id INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, action TEXT NOT NULL, status TEXT NOT NULL, evidence TEXT NOT NULL, sha256 TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
}
