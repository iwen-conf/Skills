package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	changed := make([]observedFileEntry, 0, len(keys))
	for _, path := range keys {
		sha, _, op, obsErr := observedFile(root, path)
		if obsErr != nil {
			return obsErr
		}
		if op == "deleted" {
			paths[path] = "deleted"
		}
		changed = append(changed, observedFileEntry{Path: path, Operation: paths[path], SHA: sha})
	}
	var changeID string
	var duplicate bool
	err = dbTransaction(ctx, db, func(tx *sql.Tx) error {
		if task != "" {
			var present int
			if qErr := tx.QueryRowContext(ctx, "SELECT 1 FROM arc_tasks WHERE id=?", task).Scan(&present); qErr != nil {
				return fmt.Errorf("task %q: %w", task, qErr)
			}
		}
		if len(changed) > 0 {
			body, _ := json.Marshal(changeFingerprintInput{Task: task, Root: root, Revision: head, Files: changed})
			fingerprint := shaBytes(body)
			qErr := tx.QueryRowContext(ctx, "SELECT id FROM arc_changes WHERE fingerprint=?", fingerprint).Scan(&changeID)
			if qErr == nil {
				duplicate = true
			} else if !errors.Is(qErr, sql.ErrNoRows) {
				return qErr
			} else {
				id, idErr := newID("CH-")
				if idErr != nil {
					return idErr
				}
				for _, f := range changed {
					if _, sErr := storeFile(ctx, tx, root, f.Path); sErr != nil {
						return sErr
					}
					if task != "" {
						if _, sErr := tx.ExecContext(ctx,
							"INSERT OR REPLACE INTO arc_task_files(task_id,path,updated) VALUES(?,?,?)",
							task, filepath.ToSlash(f.Path), nowUTC()); sErr != nil {
							return sErr
						}
					}
				}
				if _, execErr := tx.ExecContext(ctx,
					"INSERT INTO arc_changes(id,task,fingerprint,status,worktree,revision,created) VALUES(?,?,?,'collected',?,?,?)",
					id, task, fingerprint, root, head, nowUTC()); execErr != nil {
					return execErr
				}
				for _, f := range changed {
					if _, execErr := tx.ExecContext(ctx,
						"INSERT INTO arc_change_files(change_id,path,sha256,operation) VALUES(?,?,?,?)",
						id, filepath.ToSlash(f.Path), f.SHA, f.Operation); execErr != nil {
						return execErr
					}
				}
				rawEv, mErr := json.Marshal(changeCollectedEventPayload{Task: task, Revision: head, Files: len(changed), Reconciled: true})
				if evErr := addEventPayload(ctx, tx, "change", id, "change.collected", rawEv, mErr); evErr != nil {
					return evErr
				}
				changeID = id
			}
		}
		_, execErr := tx.ExecContext(ctx, "INSERT OR REPLACE INTO arc_meta(key,value) VALUES('sync_head',?)", head)
		return execErr
	})
	if err != nil {
		return err
	}
	return emitRawJSON(json.Marshal(reconcileResponse{
		Source:            "git",
		PreviousHead:      previous,
		CurrentHead:       head,
		CommitsReconciled: previous != "" && previous != head,
		ChangedFiles:      len(changed),
		ChangeID:          changeID,
		Duplicate:         duplicate,
		Index:             "pending",
	}))
}

