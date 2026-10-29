package main

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    _ "turso.tech/database/tursogo"
)

// db is an in-process local Turso handle. Every arc-verify invocation is still
// short-lived; no server, cloud auth, Python worker or SQL-shell process exists.
type db struct {
    file string
    handle *sql.DB
}

func openDB(path string, create bool)(db,error){
    if strings.Contains(path,"://")||strings.TrimSpace(path)=="" {
        return db{},errors.New("only a local database file path is accepted")
    }
    abs,e:=filepath.Abs(path);if e!=nil{return db{},e}
    if create {
        if e=os.MkdirAll(filepath.Dir(abs),0700);e!=nil{return db{},e}
    }else{
        st,e:=os.Lstat(abs)
        if e!=nil||!st.Mode().IsRegular(){return db{},fmt.Errorf("local database not initialized or unsafe path: %s",abs)}
    }
    conn,e:=sql.Open("turso",abs)
    if e!=nil{return db{},e}
    // Single writer and only one connection per short command.
    // Each invocation may run concurrently with others; Turso's file locks
    // and transaction handling remain the source of truth.
    conn.SetMaxOpenConns(1)
    conn.SetMaxIdleConns(1)
    ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
    defer cancel()
    if e=conn.PingContext(ctx);e!=nil{conn.Close();return db{},e}
    return db{file:abs,handle:conn},nil
}
func (d db) close()error {
    if d.handle==nil{return nil}
    return d.handle.Close()
}
func (d db) sql(statement string)(string,error){
    ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
    defer cancel()
    _,e:=d.handle.ExecContext(ctx,statement)
    return "",e
}
func quote(v string)string{return "'"+strings.ReplaceAll(v,"'","''")+"'"}
func fieldsSQL(columns ...string)string{
    // Pass the projection as real SQL columns. No hex/string concatenation.
    return strings.Join(columns,",")
}
// each streams a typed SQL result. A single reusable scan-target allocation
// replaces hex encode/decode and splitting of one huge joined TEXT cell.
func (d db) each(statement string,columns int,visit func([]string)error)error{
    ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
    defer cancel()
    rows,e:=d.handle.QueryContext(ctx,statement)
    if e!=nil{return e}
    defer rows.Close()
    values:=make([]sql.NullString,columns)
    targets:=make([]any,columns)
    row:=make([]string,columns)
    for i:=range values{targets[i]=&values[i]}
    for rows.Next(){
        if e=rows.Scan(targets...);e!=nil{return e}
        for i,value:=range values{
            if value.Valid{row[i]=value.String}else{row[i]=""}
        }
        // A visitor must not retain the row; query() copies it for callers
        // that materialize results. Gate visitors consume it immediately.
        if e=visit(row);e!=nil{return e}
    }
    return rows.Err()
}
func (d db) query(statement string,columns int)([][]string,error){
    result:=make([][]string,0,16)
    e:=d.each(statement,columns,func(row []string)error{
        result=append(result,append([]string(nil),row...));return nil
    })
    return result,e
}
func (d db) tx(statements []string)error{
    if len(statements)==0{return nil}
    ctx,cancel:=context.WithTimeout(context.Background(),60*time.Second)
    defer cancel()
    tx,e:=d.handle.BeginTx(ctx,nil)
    if e!=nil{return e}
    defer tx.Rollback()
    for _,statement:=range statements{
        if _,e=tx.ExecContext(ctx,statement);e!=nil{return e}
    }
    return tx.Commit()
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
"CREATE INDEX IF NOT EXISTS idx_edges_reverse ON edges(graph,kind,target,source);",
"CREATE INDEX IF NOT EXISTS idx_rules_action ON rules(graph,action,approved);",
"CREATE INDEX IF NOT EXISTS idx_tasks_ready ON tasks(graph,status,id);",
"CREATE INDEX IF NOT EXISTS idx_rules_approved ON rules(graph,approved,id);",
"CREATE INDEX IF NOT EXISTS idx_runs_task_history ON runs(task,id);",
"CREATE INDEX IF NOT EXISTS idx_scans_action_history ON implementation_scans(graph,action,id DESC);",
"CREATE TABLE IF NOT EXISTS task_events (id INTEGER PRIMARY KEY AUTOINCREMENT, task TEXT NOT NULL, run TEXT NOT NULL, event TEXT NOT NULL, reason TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
"CREATE TABLE IF NOT EXISTS implementation_scans (id INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, action TEXT NOT NULL, status TEXT NOT NULL, evidence TEXT NOT NULL, sha256 TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
}
