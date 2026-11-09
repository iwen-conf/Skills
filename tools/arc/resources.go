package main

import (
	"context"
	"crypto/rand"
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

// This state model deliberately uses arc_-prefixed tables: the old verification
// graph already owns unprefixed tables such as tasks/runs/task_events.

func newID(prefix string) (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil { return "", err }
	return prefix + hex.EncodeToString(buf[:]), nil
}
func nowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func addEvent(ctx context.Context, tx *sql.Tx, resource, id, event string, detail any) error {
	payload, err := json.Marshal(detail)
	if err != nil { return err }
	eventID, err := newID("EV-")
	if err != nil { return err }
	if _, err = tx.ExecContext(ctx, "INSERT INTO arc_events(id,aggregate,aggregate_id,kind,payload,created) VALUES(?,?,?,?,?,?)",
		eventID, resource, id, event, string(payload), nowUTC()); err != nil { return err }
	_, err = tx.ExecContext(ctx, "INSERT INTO arc_outbox(event_id,status) VALUES(?,'pending')", eventID)
	return err
}

func emitJSON(obj any) error { return json.NewEncoder(os.Stdout).Encode(obj) }

var taskTransitions = map[string]map[string]bool{
	"draft": {"ready": true},
	"ready": {"running": true, "blocked": true},
	"running": {"verifying": true, "blocked": true},
	"blocked": {"ready": true, "running": true},
	"verifying": {"running": true, "done": true, "blocked": true},
	"done": {"reopened": true},
	"reopened": {"ready": true, "running": true},
}

func resourceCommand(kind string, args []string) error {
	if len(args) == 0 { return fmt.Errorf("usage: arc %s <action> [--repo DIR] [flags]", kind) }
	action := args[0]
	parseArgs := args[1:]
	if kind == "update" { action = "collect"; parseArgs = args }
	fs := flag.NewFlagSet("arc "+kind+" "+action, flag.ContinueOnError)
	repoArg := fs.String("repo", ".", "repository root")
	idFlag := fs.String("id", "", "resource id")
	title := fs.String("title", "", "task title")
	to := fs.String("to", "", "target task state")
	expected := fs.Int("expected-version", 0, "expected current version for CAS")
	path := fs.String("path", "", "repo-relative source path")
	task := fs.String("task", "", "optional owning task id")
	worktree := fs.String("worktree", ".", "Git worktree path")
	changedOnly := fs.Bool("changed-only", false, "collect only Git working tree changes")
	if err := fs.Parse(parseArgs); err != nil { return err }
	// IDs/paths can be positional, but flags must precede any positional args.
	if len(fs.Args()) > 1 { return fmt.Errorf("too many positional arguments: %v", fs.Args()) }
	if len(fs.Args()) == 1 {
		switch {
		case *idFlag == "" && (kind == "task" || kind == "change"): *idFlag = fs.Arg(0)
		case *path == "" && kind == "file": *path = fs.Arg(0)
		default: return errors.New("ambiguous resource identifier")
		}
	}
	root, err := repositoryRoot(*repoArg)
	if err != nil { return err }
	var completion *verifiedTaskProof
	if kind == "task" && (action == "complete" || (action == "transition" && *to == "done")) {
		if *idFlag == "" || *expected < 1 { return errors.New("completion requires --id and --expected-version") }
		proof, err := loadCompletionProof(root, *idFlag)
		if err != nil { return err }
		completion = &proof
	}
	return withArcLock(root, func() error {
		db, err := openActiveArcDB(root)
		if err != nil { return err }
		defer db.Close()
		switch kind {
		case "task":
			if completion != nil { return commitTaskCompletion(db,root,*idFlag,*expected,*completion) }
			return executeTask(db, action, *idFlag, *title, *to, *expected)
		case "file":
			switch action {
			case "update": return executeFile(db, root, *path, *task)
			case "show": return showFile(db, *path)
			default: return fmt.Errorf("unsupported arc file action %q (update|show)",action)
			}
		case "change":
			switch action {
			case "collect": return collectChanges(db, root, *task, *worktree)
			case "show": return showChange(db, *idFlag)
			default: return fmt.Errorf("unsupported arc change action %q", action)
			}
		case "update":
			if action != "collect" { return fmt.Errorf("unsupported arc update action %q", action) }
			if !*changedOnly { return errors.New("arc update requires --changed-only in v1; broad full reindexing is forbidden") }
			return collectChanges(db, root, *task, *worktree)
		default:
			return fmt.Errorf("unknown arc resource %q", kind)
		}
	})
}

func executeTask(db *sql.DB, action, id, title, to string, expected int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	switch action {
	case "create":
		if strings.TrimSpace(title) == "" || len(title) > 500 { return errors.New("--title required (1..500 bytes)") }
		newTaskID, err := newID("TASK-")
		if err != nil { return err }
		tx, err := db.BeginTx(ctx, nil)
		if err != nil { return err }
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx,"INSERT INTO arc_tasks(id,title,status,version,created,updated) VALUES(?,?,'draft',1,?,?)",
			newTaskID, strings.TrimSpace(title), nowUTC(), nowUTC()); err != nil { return err }
		if err := addEvent(ctx, tx, "task", newTaskID, "task.created", map[string]string{"title": title}); err != nil { return err }
		if err := tx.Commit(); err != nil { return err }
		return emitJSON(map[string]any{"id":newTaskID, "status":"draft", "version":1})
	case "ensure":
		// Bind the policy runtime's stable task ID to the resource graph.
		// This is an idempotent import; it MUST NOT promote task status.
		if id == "" || len(id) > 128 || strings.Trim(id, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
			return errors.New("task ensure requires a safe --id (1..128 ASCII characters)")
		}
		if strings.TrimSpace(title) == "" || len(title) > 500 {
			return errors.New("task ensure requires --title (1..500 bytes)")
		}
		name := strings.TrimSpace(title)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil { return err }
		defer tx.Rollback()
		var existingTitle, status string
		var version int
		err = tx.QueryRowContext(ctx, "SELECT title,status,version FROM arc_tasks WHERE id=?", id).Scan(&existingTitle,&status,&version)
		if err == nil {
			if existingTitle != name { return fmt.Errorf("task %s identity collision (title differs)",id) }
			return emitJSON(map[string]any{"id":id,"status":status,"version":version,"duplicate":true})
		}
		if !errors.Is(err,sql.ErrNoRows) { return err }
		if _,err=tx.ExecContext(ctx,"INSERT INTO arc_tasks(id,title,status,version,created,updated) VALUES(?,?,'draft',1,?,?)",id,name,nowUTC(),nowUTC());err!=nil{return err}
		if err=addEvent(ctx,tx,"task",id,"task.imported",map[string]string{"title":name,"source":"arc-policy"});err!=nil{return err}
		if err=tx.Commit();err!=nil{return err}
		return emitJSON(map[string]any{"id":id,"status":"draft","version":1,"duplicate":false})
	case "show":
		if id == "" { return errors.New("task ID required (--id or positional)") }
		var name, status, updated string
		var v int
		if err := db.QueryRowContext(ctx, "SELECT title,status,version,updated FROM arc_tasks WHERE id=?", id).Scan(&name,&status,&v,&updated); err != nil { return err }
		rows,err:=db.QueryContext(ctx,"SELECT path FROM arc_task_files WHERE task_id=? ORDER BY path",id)
		if err!=nil{return err}
		files:=[]string{}
		for rows.Next(){var path string;if e:=rows.Scan(&path);e!=nil{rows.Close();return e};files=append(files,path)}
		err=rows.Err();rows.Close();if err!=nil{return err}
		return emitJSON(map[string]any{"id":id,"title":name,"status":status,"version":v,"updated":updated,"files":files})
	case "transition":
		if id == "" || expected < 1 { return errors.New("task transition requires --id and --expected-version >=1") }
		tx, err := db.BeginTx(ctx, nil)
		if err != nil { return err }
		defer tx.Rollback()
		var old string
		var version int
		if err := tx.QueryRowContext(ctx,"SELECT status,version FROM arc_tasks WHERE id=?", id).Scan(&old,&version); err != nil { return err }
		if version != expected { return fmt.Errorf("task %s conflict: expected version %d, actual %d",id,expected,version) }
		if !taskTransitions[old][to] { return fmt.Errorf("invalid task transition %s -> %s",old,to) }
		if to == "done" {
			// Do not accept an application-level PASS without independent,
			// revision-scoped proof. The separate verifier still owns the gate.
			return errors.New("done requires accepted verification evidence; arc task verification is not implemented yet")
		}
		res, err := tx.ExecContext(ctx,"UPDATE arc_tasks SET status=?,version=version+1,updated=? WHERE id=? AND version=? AND status=?",
			to,nowUTC(),id,expected,old)
		if err != nil { return err }
		affected, err := res.RowsAffected()
		if err != nil || affected != 1 { return errors.New("task changed concurrently; transition aborted") }
		if err := addEvent(ctx,tx,"task",id,"task.transitioned",map[string]any{"from":old,"to":to,"from_version":version});err!=nil{return err}
		if err := tx.Commit();err!=nil{return err}
		return emitJSON(map[string]any{"id":id,"status":to,"version":version+1})
	default:
		return fmt.Errorf("unsupported task action %q (create|ensure|show|transition|complete)",action)
	}
}

func observedFile(root, relative string) (string, int64, string, error) {
	if relative == "" || filepath.IsAbs(relative) { return "",0,"",errors.New("file path must be repository relative") }
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean,".."+string(filepath.Separator)) || clean == ".arc" || strings.HasPrefix(clean,".arc"+string(filepath.Separator)) {
		return "",0,"",errors.New("unsafe or internal file path")
	}
	full := filepath.Join(root,clean)
	// A symlink in any ancestor must never allow reading outside the worktree.
	if parent, e := filepath.EvalSymlinks(filepath.Dir(full)); e == nil {
		rel,e := filepath.Rel(root,parent)
		if e != nil || rel==".." || strings.HasPrefix(rel,".."+string(filepath.Separator)){return "",0,"",errors.New("file path escapes repository through symlink")}
	} else { return "",0,"",e }
	st,err:=os.Lstat(full)
	if errors.Is(err,os.ErrNotExist){return "",0,"deleted",nil}
	if err!=nil{return "",0,"",err}
	if st.Mode()&os.ModeSymlink!=0 {
		link,err:=os.Readlink(full)
		if err!=nil{return "",0,"",err}
		hash:=sha256.Sum256([]byte(link))
		return hex.EncodeToString(hash[:]),int64(len(link)),"symlink",nil
	}
	if !st.Mode().IsRegular(){return "",0,"",errors.New("not a regular file")}
	sha,err:=shaFile(full)
	return sha,st.Size(),"modified",err
}

func storeFile(ctx context.Context, tx *sql.Tx, root,path string) (string,error) {
	sha,size,op,err:=observedFile(root,path)
	if err!=nil{return "",err}
	clean:=filepath.ToSlash(filepath.Clean(path))
	var old string
	var version int
	err=tx.QueryRowContext(ctx,"SELECT sha256,version FROM arc_files WHERE path=?",clean).Scan(&old,&version)
	if err!=nil && !errors.Is(err,sql.ErrNoRows){return "",err}
	if old==sha && version>0{return sha,nil}
	if version==0 {
		_,err=tx.ExecContext(ctx,"INSERT INTO arc_files(path,sha256,bytes,version,changed) VALUES(?,?,?,1,?)",clean,sha,size,nowUTC())
	} else {
		_,err=tx.ExecContext(ctx,"UPDATE arc_files SET sha256=?,bytes=?,version=version+1,changed=? WHERE path=? AND version=?",sha,size,nowUTC(),clean,version)
	}
	if err!=nil{return "",err}
	if err=addEvent(ctx,tx,"file",clean,"file."+op,map[string]any{"sha256":sha,"version":version+1});err!=nil{return "",err}
	return sha,nil
}
func executeFile(db *sql.DB,root,path,task string) error {
	if path==""{return errors.New("file update requires --path")}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	tx,err:=db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
	if task!="" {
		var present int
		if err:=tx.QueryRowContext(ctx,"SELECT 1 FROM arc_tasks WHERE id=?",task).Scan(&present);err!=nil{return fmt.Errorf("owning task: %w",err)}
	}
	sha,err:=storeFile(ctx,tx,root,path);if err!=nil{return err}
	if task!="" {
		_,err=tx.ExecContext(ctx,"INSERT OR REPLACE INTO arc_task_files(task_id,path,updated) VALUES(?,?,?)",task,filepath.ToSlash(filepath.Clean(path)),nowUTC())
		if err!=nil{return err}
	}
	if err:=tx.Commit();err!=nil{return err}
	return emitJSON(map[string]any{"path":filepath.ToSlash(filepath.Clean(path)),"sha256":sha,"task":task,"index":"pending"})
}

// showFile reads the last stored observation without scanning the source tree.
// A missing file in the worktree does not erase its historical DB record.
func showFile(db *sql.DB,path string)error{
	if path==""||filepath.IsAbs(path){return errors.New("file show requires a relative --path")}
	clean:=filepath.ToSlash(filepath.Clean(path))
	if clean=="."||clean==".."||strings.HasPrefix(clean,"../")||clean==".arc"||strings.HasPrefix(clean,".arc/"){
		return errors.New("unsafe file path")
	}
	ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
	var sha,changed string
	var size int64
	var version int
	if err:=db.QueryRowContext(ctx,"SELECT sha256,bytes,version,changed FROM arc_files WHERE path=?",clean).Scan(&sha,&size,&version,&changed);err!=nil{return err}
	return emitJSON(map[string]any{"path":clean,"sha256":sha,"bytes":size,"version":version,"changed_at":changed})
}

type gitChange struct{ Path, Operation string }
func changedGitPaths(root string) ([]gitChange,error) {
	cmd:=exec.Command("git","-C",root,"status","--porcelain=v1","-z","--untracked-files=all")
	out,err:=cmd.Output();if err!=nil{return nil,fmt.Errorf("git status: %w",err)}
	parts:=strings.Split(string(out),"\x00")
	ops:=map[string]string{}
	for i:=0;i<len(parts);i++{
		rec:=parts[i];if rec==""{continue}
		if len(rec)<4 || rec[2]!=' ' {return nil,errors.New("invalid git porcelain status")}
		code,path:=rec[:2],rec[3:]
		if strings.ContainsAny(code,"RC") {
			// Git porcelain -z lists destination first, then original path.
			// A rename invalidates both paths; a copy keeps its source.
			if i+1>=len(parts)||parts[i+1]=="" {return nil,errors.New("incomplete Git rename/copy record")}
			original:=parts[i+1];i++
			if strings.Contains(code,"R") && original!=path && original!=".arc" && !strings.HasPrefix(original,".arc/") && !isGeneratedIndexFile(original) {
				ops[original]="deleted"
			}
		}
		if path==".arc" || strings.HasPrefix(path,".arc/") || isGeneratedIndexFile(path){continue}
		op:="modified"
		if strings.Contains(code,"D"){op="deleted"}else if code=="??" {op="untracked"}else if strings.Contains(code,"A"){op="added"}else if strings.ContainsAny(code,"RC"){op="renamed"}
		ops[path]=op
		if len(ops)>1000{return nil,errors.New("too many changed files (>1000), narrow scope")}
	}
	paths:=make([]string,0,len(ops));for p:=range ops{paths=append(paths,p)}
	sort.Strings(paths)
	result:=make([]gitChange,0,len(paths))
	for _,p:=range paths{result=append(result,gitChange{p,ops[p]})}
	return result,nil
}

func collectChanges(db *sql.DB,root,task,worktree string) error {
	if worktree!="." {return errors.New("v1 change collection requires the same repository worktree; --worktree switching is not yet supported")}
	files,err:=changedGitPaths(root);if err!=nil{return err}
	if len(files)==0{return emitJSON(map[string]any{"changed_files":0,"change_id":nil,"index":"unchanged"})}
	head:=exec.Command("git","-C",root,"rev-parse","HEAD")
	raw,err:=head.Output();if err!=nil{return fmt.Errorf("git HEAD: %w",err)}
	revision:=strings.TrimSpace(string(raw))
	ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second);defer cancel()
	tx,err:=db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
	if task!="" {
		var present int
		if err:=tx.QueryRowContext(ctx,"SELECT 1 FROM arc_tasks WHERE id=?",task).Scan(&present);err!=nil{return fmt.Errorf("owning task: %w",err)}
	}
	type fileState struct {Path,Operation,SHA string}
	var observed []fileState
	for _,f:=range files{
		sha,_,op,err:=observedFile(root,f.Path);if err!=nil{return err}
		if op=="deleted"{f.Operation="deleted"}
		observed=append(observed,fileState{f.Path,f.Operation,sha})
	}
	digestInput,_:=json.Marshal(map[string]any{"task":task,"root":root,"revision":revision,"files":observed})
	digest:=sha256.Sum256(digestInput)
	fingerprint:=hex.EncodeToString(digest[:])
	var existing string
	err=tx.QueryRowContext(ctx,"SELECT id FROM arc_changes WHERE fingerprint=?",fingerprint).Scan(&existing)
	if err==nil {return emitJSON(map[string]any{"change_id":existing,"changed_files":len(files),"duplicate":true,"index":"pending"})}
	if !errors.Is(err,sql.ErrNoRows){return err}
	id,err:=newID("CH-");if err!=nil{return err}
	for _,f:=range observed {
		if _,err:=storeFile(ctx,tx,root,f.Path);err!=nil{return err}
		if task!="" {
			_,err=tx.ExecContext(ctx,"INSERT OR REPLACE INTO arc_task_files(task_id,path,updated) VALUES(?,?,?)",task,filepath.ToSlash(f.Path),nowUTC())
			if err!=nil{return err}
		}
	}
	if _,err:=tx.ExecContext(ctx,"INSERT INTO arc_changes(id,task,fingerprint,status,worktree,revision,created) VALUES(?,?,?,'collected',?,?,?)",id,task,fingerprint,root,revision,nowUTC());err!=nil{return err}
	for _,f:=range observed {
		if _,err:=tx.ExecContext(ctx,"INSERT INTO arc_change_files(change_id,path,sha256,operation) VALUES(?,?,?,?)",id,filepath.ToSlash(f.Path),f.SHA,f.Operation);err!=nil{return err}
	}
	if err:=addEvent(ctx,tx,"change",id,"change.collected",map[string]any{"task":task,"revision":revision,"files":len(files)});err!=nil{return err}
	if err:=tx.Commit();err!=nil{return err}
	return emitJSON(map[string]any{"change_id":id,"changed_files":len(files),"duplicate":false,"index":"pending","status":"collected"})
}

func showChange(db *sql.DB,id string) error {
	if id==""{return errors.New("change ID required")}
	ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
	var task,status,rev,created string
	if err:=db.QueryRowContext(ctx,"SELECT task,status,revision,created FROM arc_changes WHERE id=?",id).Scan(&task,&status,&rev,&created);err!=nil{return err}
	rows,err:=db.QueryContext(ctx,"SELECT path,sha256,operation FROM arc_change_files WHERE change_id=? ORDER BY path",id)
	if err!=nil{return err}
	defer rows.Close()
	paths:=[]map[string]string{}
	for rows.Next(){var p,h,op string;if err:=rows.Scan(&p,&h,&op);err!=nil{return err};paths=append(paths,map[string]string{"path":p,"sha256":h,"operation":op})}
	if err:=rows.Err();err!=nil{return err}
	return emitJSON(map[string]any{"id":id,"task":task,"status":status,"revision":rev,"created":created,"files":paths})
}
