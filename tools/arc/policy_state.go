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
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The Policy Runtime is implemented in Python, but after cutover this
// Turso-backed state is the only authoritative task/history/receipt store.
// All Policy callers must obey policy task.lock -> arc.lock order.
var policyStoreSchema = []string{
	`CREATE TABLE IF NOT EXISTS arc_policy_state (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL CHECK(version>=1), body TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_policy_receipts (sha256 TEXT PRIMARY KEY, task_id TEXT NOT NULL, verdict TEXT NOT NULL, body TEXT NOT NULL, captured_at TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS arc_policy_receipts_task ON arc_policy_receipts(task_id, captured_at)`,
}

type policyDocument struct {
	Mode    string          `json:"mode"`
	Version int64           `json:"version"`
	State   json.RawMessage `json:"state"`
}

func loadPolicyDocument(db *sql.DB) (policyDocument, error) {
	// Arc 0.3 ledgers predate policy tables. An absent table means legacy
	// state is still authoritative until explicit policy migration.
	var table string
	tableErr:=db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='arc_policy_state'").Scan(&table)
	if errors.Is(tableErr,sql.ErrNoRows) {
		return policyDocument{Mode:"legacy", Version:0, State:json.RawMessage("{}")},nil
	}
	if tableErr!=nil{return policyDocument{},tableErr}
	var body string
	var version int64
	err := db.QueryRow("SELECT version, body FROM arc_policy_state WHERE id=1").Scan(&version, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return policyDocument{Mode:"legacy", Version:0, State:json.RawMessage("{}")}, nil
	}
	if err != nil { return policyDocument{}, err }
	if _, err := decodePolicyState([]byte(body)); err != nil { return policyDocument{}, err }
	return policyDocument{Mode:"arc.db", Version:version, State:json.RawMessage(body)}, nil
}

func decodePolicyState(raw []byte) (map[string]any, error) {
	if len(raw) == 0 || len(raw)>16<<20 { return nil, errors.New("policy state size must be 1..16 MiB") }
	var value map[string]any
	if err := json.Unmarshal(raw,&value);err!=nil{return nil,fmt.Errorf("invalid policy JSON: %w",err)}
	if value==nil {return nil,errors.New("policy state must be a JSON object")}
	if v,ok:=value["history"];ok {
		if _,valid:=v.([]any);!valid {return nil,errors.New("policy state history must be an array")}
	}
	if v,ok:=value["active"];ok && v!=nil {
		item,valid:=v.(map[string]any)
		if !valid {return nil,errors.New("policy active task must be an object or null")}
		for _,k:=range []string{"id","slug","skill"}{
			x,yes:=item[k].(string)
			if !yes || strings.TrimSpace(x)=="" {return nil,fmt.Errorf("policy active task missing %s",k)}
		}
	}
	return value,nil
}

type policyReceipt struct {
	Hash string
	TaskID string
	Verdict string
	Body string
}

func collectPolicyReceipts(state map[string]any) ([]policyReceipt,error) {
	values:=[]any{}
	if active,ok:=state["active"].(map[string]any);ok {values=append(values,active)}
	if history,ok:=state["history"].([]any);ok {values=append(values,history...)}
	unique:=map[string]policyReceipt{}
	for _,v:=range values {
		task,ok:=v.(map[string]any)
		if !ok{return nil,errors.New("policy history entry must be an object")}
		raw,exists:=task["receipt"]
		if !exists || raw==nil {continue}
		receipt,ok:=raw.(map[string]any)
		if !ok{return nil,errors.New("policy receipt must be an object")}
		taskID,ok:=task["id"].(string)
		if !ok||taskID==""{return nil,errors.New("policy receipt task lacks ID")}
		if receiptID,_:=receipt["task_id"].(string);receiptID!=""&&receiptID!=taskID {
			return nil,errors.New("policy receipt and owner task IDs disagree")
		}
		verdict,_:=receipt["verdict"].(string)
		if verdict!="PASS"&&verdict!="AUTHORIZED" {
			return nil,fmt.Errorf("invalid archived policy receipt verdict %q",verdict)
		}
		canonical,err:=json.Marshal(receipt)
		if err!=nil{return nil,err}
		hash:=sha256.Sum256(canonical)
		id:=hex.EncodeToString(hash[:])
		unique[id]=policyReceipt{Hash:id,TaskID:taskID,Verdict:verdict,Body:string(canonical)}
	}
	out:=make([]policyReceipt,0,len(unique))
	for _,v:=range unique{out=append(out,v)}
	return out,nil
}

func persistPolicyReceipts(ctx context.Context,tx *sql.Tx,items []policyReceipt)error {
	for _,receipt:=range items {
		if _,err:=tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO arc_policy_receipts(sha256,task_id,verdict,body,captured_at) VALUES(?,?,?,?,?)",
			receipt.Hash,receipt.TaskID,receipt.Verdict,receipt.Body,nowUTC());err!=nil{return err}
	}
	return nil
}

func policyStateCommand(args []string)error{
	if len(args)==0{return errors.New("usage: arc policy state <get|put|status|migrate> [--repo DIR] [--expected-version N] [--confirm-offline]")}
	action:=args[0]
	fs:=flag.NewFlagSet("arc policy state "+action,flag.ContinueOnError)
	repoFlag:=fs.String("repo",".","repository root")
	expected:=fs.Int64("expected-version",0,"CAS expected store version")
	confirm:=fs.Bool("confirm-offline",false,"explicit offline migration of legacy policy JSON")
	if err:=fs.Parse(args[1:]);err!=nil{return err}
	if len(fs.Args())!=0{return fmt.Errorf("unexpected positional arguments: %v",fs.Args())}
	root,err:=repositoryRoot(*repoFlag)
	if err!=nil{return err}
	if _,err:=readManifest(root);err!=nil{return err}
	switch action{
	case "get","status":
		return withArcLock(root,func()error{
			db,err:=openActiveArcDB(root);if err!=nil{return err};defer db.Close()
			doc,err:=loadPolicyDocument(db);if err!=nil{return err}
			if action=="status"{return emitJSON(map[string]any{"mode":doc.Mode,"version":doc.Version})}
			return emitJSON(doc)
		})
	case "put":
		if *expected<1 {return errors.New("policy state put requires --expected-version >=1")}
		raw,err:=io.ReadAll(io.LimitReader(os.Stdin,(16<<20)+1));if err!=nil{return err}
		state,err:=decodePolicyState(raw);if err!=nil{return err}
		receipts,err:=collectPolicyReceipts(state);if err!=nil{return err}
		// Canonical JSON ensures stable hashes across Python/Go serializers.
		body,err:=json.Marshal(state);if err!=nil{return err}
		return withArcLock(root,func()error{
			db,err:=openActiveArcDB(root);if err!=nil{return err};defer db.Close()
			ctx,cancel:=context.WithTimeout(context.Background(),25*time.Second);defer cancel()
			var next int64
			err=dbTransaction(ctx,db,func(tx *sql.Tx)error{
				res,err:=tx.ExecContext(ctx,
					"UPDATE arc_policy_state SET body=?,version=version+1,updated_at=? WHERE id=1 AND version=?",
					string(body),nowUTC(),*expected)
				if err!=nil{return err}
				n,err:=res.RowsAffected();if err!=nil{return err}
				if n!=1{return fmt.Errorf("policy state version conflict (expected %d); reload before writing",*expected)}
				if err:=persistPolicyReceipts(ctx,tx,receipts);err!=nil{return err}
				sum:=sha256.Sum256(body)
				if err:=addEvent(ctx,tx,"policy","state","policy.state.updated",map[string]any{"version":*expected+1,"sha256":hex.EncodeToString(sum[:])});err!=nil{return err}
				next=*expected+1
				return nil
			})
			if err!=nil{return err}
			return emitJSON(map[string]any{"mode":"arc.db","version":next})
		})
	case "migrate":
		if !*confirm{return errors.New("policy migration requires --confirm-offline and no concurrent legacy policy writers")}
		return withPolicyTaskLock(root,func()error{
			return withArcLock(root,func()error{return migratePolicyState(root)})
		})
	default:
		return fmt.Errorf("unknown arc policy state action %q",action)
	}
}

// retireLegacyPolicyFiles leaves a directory tombstone at state.json so
// older plugin builds cannot silently recreate an authoritative JSON file.
// The original bytes remain in a content-addressed backup. Re-running is safe
// after a crash between committing the DB row and retiring the files.
func retireLegacyPolicyFiles(root, expectedHash string)error{
	if len(expectedHash)!=64{return errors.New("missing original policy backup fingerprint")}
	base:=filepath.Join(root,".arc")
	backup:=filepath.Join(base,"backups","policy-state-"+expectedHash+".json")
	if err:=checkRegular(backup);err!=nil{return fmt.Errorf("policy backup unavailable: %w",err)}
	checksum,err:=shaFile(backup);if err!=nil||checksum!=expectedHash{return errors.New("policy backup checksum mismatch")}
	policyDir:=filepath.Join(base,"policy")
	legacy:=filepath.Join(policyDir,"state.json")
	st,err:=os.Lstat(legacy)
	if errors.Is(err,os.ErrNotExist){
		if err:=os.Mkdir(legacy,0700);err!=nil{return err}
	}else if err!=nil{return err}else if st.IsDir(){
		entries,err:=os.ReadDir(legacy);if err!=nil{return err}
		if len(entries)!=0{return errors.New("legacy state tombstone contains unexpected files")}
	}else if st.Mode().IsRegular(){
		liveHash,err:=shaFile(legacy);if err!=nil{return err}
		if liveHash!=expectedHash{return errors.New("legacy state changed after migration; manual recovery required")}
		// The backup is verified; only now may the duplicate live path be removed.
		if err:=os.Remove(legacy);err!=nil{return err}
		if err:=os.Mkdir(legacy,0700);err!=nil{return fmt.Errorf("cannot install old-runtime tombstone: %w",err)}
	}else{return errors.New("unsafe legacy policy state path")}
	receipts:=filepath.Join(policyDir,"receipts")
	if st,err:=os.Lstat(receipts);err==nil{
		if !st.IsDir()||st.Mode()&os.ModeSymlink!=0{return errors.New("unsafe legacy receipts path")}
		dest:=filepath.Join(base,"backups","policy-receipts-"+expectedHash[:16])
		if _,err:=os.Lstat(dest);err==nil{return errors.New("legacy receipt archive already exists; inspect before retrying")}
		if !errors.Is(err,os.ErrNotExist){return err}
		if err:=os.Rename(receipts,dest);err!=nil{return err}
	}else if !errors.Is(err,os.ErrNotExist){return err}
	return nil
}

func policyMigrationFingerprint(db *sql.DB)(string,error){
	var payload string
	err:=db.QueryRow("SELECT payload FROM arc_events WHERE aggregate='policy' AND kind='policy.state.migrated' ORDER BY created DESC,id DESC LIMIT 1").Scan(&payload)
	if err!=nil{return "",err}
	var v struct{ SHA string `json:"state_sha256"` }
	if err:=json.Unmarshal([]byte(payload),&v);err!=nil{return "",err}
	if len(v.SHA)!=64{return "",errors.New("migration event has no policy backup fingerprint")}
	return v.SHA,nil
}

func migratePolicyState(root string)error{
	db,err:=openActiveArcDB(root);if err!=nil{return err};defer db.Close()
	for _,stmt:=range policyStoreSchema{
		if _,err:=db.Exec(stmt);err!=nil{return fmt.Errorf("policy schema: %w",err)}
	}
	doc,err:=loadPolicyDocument(db);if err!=nil{return err}
	if doc.Mode=="arc.db"{
		hash,err:=policyMigrationFingerprint(db);if err!=nil{return err}
		if err:=retireLegacyPolicyFiles(root,hash);err!=nil{return err}
		return emitJSON(map[string]any{"storage":"arc.db","already_migrated":true,"legacy_guard":"directory","active":true})
	}
	legacy:=filepath.Join(root,".arc","policy","state.json")
	body:=[]byte("{}")
	if st,err:=os.Lstat(legacy);err==nil {
		if !st.Mode().IsRegular(){return errors.New("unsafe legacy policy state path")}
		body,err=os.ReadFile(legacy);if err!=nil{return err}
	} else if !errors.Is(err,os.ErrNotExist){return err}
	state,err:=decodePolicyState(body);if err!=nil{return err}
	receipts,err:=collectPolicyReceipts(state);if err!=nil{return err}
	receiptsDir:=filepath.Join(root,".arc","policy","receipts")
	if info,err:=os.Lstat(receiptsDir);err==nil{
		if !info.IsDir()||info.Mode()&os.ModeSymlink!=0 {return errors.New("unsafe legacy policy receipts directory")}
		list,err:=filepath.Glob(filepath.Join(receiptsDir,"*.json"));if err!=nil{return err}
		for _,file:=range list {
			if err:=checkRegular(file);err!=nil{return err}
			raw,err:=os.ReadFile(file);if err!=nil{return err}
			var receipt map[string]any
			if err:=json.Unmarshal(raw,&receipt);err!=nil{return fmt.Errorf("legacy receipt %s: %w",file,err)}
			if receipt==nil{return fmt.Errorf("legacy receipt %s is null",file)}
			taskID,_:=receipt["task_id"].(string)
			if taskID==""{return fmt.Errorf("legacy receipt %s has no task ID",file)}
			verdict,_:=receipt["verdict"].(string)
			if verdict!="PASS"&&verdict!="AUTHORIZED"{return fmt.Errorf("legacy receipt %s has invalid verdict",file)}
			canonical,err:=json.Marshal(receipt);if err!=nil{return err}
			hash:=sha256.Sum256(canonical)
			receipts=append(receipts,policyReceipt{Hash:hex.EncodeToString(hash[:]),TaskID:taskID,Verdict:verdict,Body:string(canonical)})
		}
	}else if !errors.Is(err,os.ErrNotExist){return err}
	// Keep an untouched file backup; after cutover the runtime never reads
	// or writes this legacy JSON. User can inspect or restore it offline.
	rawHash:=sha256.Sum256(body)
	backups:=filepath.Join(root,".arc","backups")
	if err:=os.MkdirAll(backups,0700);err!=nil{return err}
	if st,err:=os.Lstat(backups);err!=nil||!st.IsDir()||st.Mode()&os.ModeSymlink!=0{
		return errors.New("unsafe policy backup directory")
	}
	backup:=filepath.Join(backups,"policy-state-"+hex.EncodeToString(rawHash[:])+".json")
	if _,err:=os.Lstat(backup);errors.Is(err,os.ErrNotExist){
		out,err:=os.OpenFile(backup,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600);if err!=nil{return err}
		if _,err=out.Write(body);err==nil{err=out.Sync()}
		if e:=out.Close();err==nil{err=e};if err!=nil{return err}
	}else if err!=nil{return err}
	backupBody,err:=os.ReadFile(backup);if err!=nil{return err}
	if sha256.Sum256(backupBody)!=rawHash{return errors.New("policy backup checksum mismatch")}
	canonical,err:=json.Marshal(state);if err!=nil{return err}
	ctx,cancel:=context.WithTimeout(context.Background(),35*time.Second);defer cancel()
	err=dbTransaction(ctx,db,func(tx *sql.Tx)error{
		if _,err:=tx.ExecContext(ctx,
			"INSERT INTO arc_policy_state(id,version,body,updated_at) VALUES(1,1,?,?)",
			string(canonical),nowUTC());err!=nil{return err}
		if err:=persistPolicyReceipts(ctx,tx,receipts);err!=nil{return err}
		return addEvent(ctx,tx,"policy","state","policy.state.migrated",map[string]any{
			"state_sha256":hex.EncodeToString(rawHash[:]),"receipts":len(receipts),"version":1})
	})
	if err!=nil{return err}
	live,err:=loadPolicyDocument(db)
	if err!=nil||live.Mode!="arc.db"{return errors.New("policy migration did not commit a readable state")}
	var restored map[string]any
	if err:=json.Unmarshal(live.State,&restored);err!=nil{return err}
	reencoded,err:=json.Marshal(restored);if err!=nil{return err}
	if string(reencoded)!=string(canonical){return errors.New("migrated policy state differs from original; fail closed")}
	if err:=retireLegacyPolicyFiles(root,hex.EncodeToString(rawHash[:]));err!=nil{
		return fmt.Errorf("policy DB migrated, but legacy guard setup failed; rerun migrate offline to finalize: %w",err)
	}
	return emitJSON(map[string]any{"storage":"arc.db","version":1,"backup":backup,"imported_receipts":len(receipts),"legacy_guard":"directory","active":true})
}
