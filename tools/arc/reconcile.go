package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sync reconcile repairs missed post-edit events, including edits committed
// without running the plugin. The head cursor commits in the same SQL
// transaction as its change set; an interrupted reconciliation is replayable.
func syncCommand(args []string) error {
	if len(args)==0 || args[0]!="reconcile" { return errors.New("usage: arc sync reconcile [--repo DIR] [--task TASK_ID]") }
	fs:=flag.NewFlagSet("arc sync reconcile",flag.ContinueOnError)
	repo:=fs.String("repo",".","repository/worktree root")
	task:=fs.String("task","","optional owning task")
	if err:=fs.Parse(args[1:]);err!=nil{return err}
	if len(fs.Args())!=0{return fmt.Errorf("unexpected arguments: %v",fs.Args())}
	root,err:=repositoryRoot(*repo);if err!=nil{return err}
	return withArcLock(root,func()error {
		db,err:=openActiveArcDB(root);if err!=nil{return err}
		defer db.Close()
		return reconcileGit(db,root,*task)
	})
}

func gitHead(root string)(string,error){
	cmd:=exec.Command("git","-C",root,"rev-parse","HEAD")
	out,err:=cmd.Output();if err!=nil{return "",fmt.Errorf("git HEAD: %w",err)}
	return strings.TrimSpace(string(out)),nil
}

func reconcileGit(db *sql.DB,root,task string)error{
	head,err:=gitHead(root);if err!=nil{return err}
	ctx,cancel:=context.WithTimeout(context.Background(),60*time.Second);defer cancel()
	var previous string
	err=db.QueryRowContext(ctx,"SELECT value FROM arc_meta WHERE key='sync_head'").Scan(&previous)
	if err!=nil&&!errors.Is(err,sql.ErrNoRows){return err}
	dirty,err:=changedGitPaths(root);if err!=nil{return err}
	paths:=map[string]string{}
	for _,change:=range dirty{paths[change.Path]=change.Operation}
	if previous!=""&&previous!=head{
		// --no-renames reports both sides of renames. NUL-delimited paths avoid
		// ambiguity with spaces, tabs and newlines in filenames.
		output,err:=exec.Command("git","-C",root,"diff","--name-only","--no-renames","-z",previous,head).Output()
		if err!=nil{return fmt.Errorf("cannot replay changed commits from %s to %s: %w",previous,head,err)}
		for _,path:=range strings.Split(string(output),"\x00"){
			if path==""||path==".arc"||strings.HasPrefix(path,".arc/") {continue}
			if _,ok:=paths[path];!ok{paths[path]="modified"}
		}
	}
	if len(paths)>1000{return fmt.Errorf("reconcile exceeded 1000 changed paths: %d",len(paths))}
	keys:=make([]string,0,len(paths));for p:=range paths{keys=append(keys,p)};sort.Strings(keys)
	type observed struct{Path,Operation,SHA string}
	changed:=make([]observed,0,len(keys))
	for _,path:=range keys{
		sha,_,op,err:=observedFile(root,path);if err!=nil{return err}
		if op=="deleted"{paths[path]="deleted"}
		changed=append(changed,observed{path,paths[path],sha})
	}
	var changeID string
	var duplicate bool
	err=dbTransaction(ctx,db,func(tx *sql.Tx)error{
		if task!=""{
			var present int
			if err:=tx.QueryRowContext(ctx,"SELECT 1 FROM arc_tasks WHERE id=?",task).Scan(&present);err!=nil{return fmt.Errorf("task %q: %w",task,err)}
		}
		if len(changed)>0{
			body,_:=json.Marshal(map[string]any{"task":task,"root":root,"revision":head,"files":changed})
			hash:=sha256.Sum256(body);fingerprint:=hex.EncodeToString(hash[:])
			err:=tx.QueryRowContext(ctx,"SELECT id FROM arc_changes WHERE fingerprint=?",fingerprint).Scan(&changeID)
			if err==nil{duplicate=true}else if !errors.Is(err,sql.ErrNoRows){return err}else{
				id,err:=newID("CH-");if err!=nil{return err}
				for _,f:=range changed{
					// Capture the current file in the same worktree, but do
					// not treat a successful hash as verification evidence.
					if _,err:=storeFile(ctx,tx,root,f.Path);err!=nil{return err}
				}
				if _,err:=tx.ExecContext(ctx,
					"INSERT INTO arc_changes(id,task,fingerprint,status,worktree,revision,created) VALUES(?,?,?,'collected',?,?,?)",
					id,task,fingerprint,root,head,nowUTC());err!=nil{return err}
				for _,f:=range changed{
					if _,err:=tx.ExecContext(ctx,
						"INSERT INTO arc_change_files(change_id,path,sha256,operation) VALUES(?,?,?,?)",
						id,filepath.ToSlash(f.Path),f.SHA,f.Operation);err!=nil{return err}
				}
				if err:=addEvent(ctx,tx,"change",id,"change.collected",map[string]any{"task":task,"revision":head,"files":len(changed),"reconciled":true});err!=nil{return err}
				changeID=id
			}
		}
		_,err:=tx.ExecContext(ctx,"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('sync_head',?)",head)
		return err
	})
	if err!=nil{return err}
	return emitJSON(map[string]any{
		"source":"git","previous_head":previous,"current_head":head,
		"commits_reconciled":previous!=""&&previous!=head,
		"changed_files":len(changed),"change_id":changeID,
		"duplicate":duplicate,"index":"pending",
	})
}

func dbTransaction(ctx context.Context,db *sql.DB,fn func(*sql.Tx)error)error{
	tx,err:=db.BeginTx(ctx,nil);if err!=nil{return err}
	defer tx.Rollback()
	if err:=fn(tx);err!=nil{return err}
	return tx.Commit()
}

var _=os.ErrNotExist
