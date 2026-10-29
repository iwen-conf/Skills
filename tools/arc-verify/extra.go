package main

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "strings"
)

type Implementation struct { Graph string `json:"graph"`; Entrances []Entrance `json:"entrances"` }
type Entrance struct { Action string `json:"action"`; Status string `json:"status"`; Evidence string `json:"evidence"` }
type Fixtures struct { Fixtures []Fixture `json:"fixtures"` }
type Fixture struct { ID string `json:"id"`; Kind string `json:"kind"`; State string `json:"state"`; SecretRef string `json:"secretRef,omitempty"`; Refs map[string]string `json:"refs,omitempty"` }

func validateFixtures(f Fixtures)error{
 if len(f.Fixtures)==0{return errors.New("fixture list empty")}
 seen:=map[string]bool{}
 for _,v:=range f.Fixtures {
  if v.ID==""||v.Kind==""||v.State==""||seen[v.ID]{return fmt.Errorf("invalid/duplicate fixture %q",v.ID)}
  seen[v.ID]=true
  if v.SecretRef!=""&&!strings.HasPrefix(v.SecretRef,"env://")&&!strings.HasPrefix(v.SecretRef,"secret://"){return fmt.Errorf("fixture %s must use a secret reference, never plaintext",v.ID)}
 }
 return nil
}
func (d db) importFixtures(f Fixtures)(int,error){
 if e:=validateFixtures(f);e!=nil{return 0,e}
 stmts:=[]string{}
 for _,v:=range f.Fixtures{
  b,_:=json.Marshal(v)
  stmts=append(stmts,fmt.Sprintf("INSERT OR IGNORE INTO fixtures(id,body) VALUES(%s,%s);",quote(v.ID),quote(string(b))))
 }
 return len(stmts),d.tx(stmts)
}
func (d db) fixture(id string)([][]string,error){return d.query("SELECT "+fieldsSQL("id","body")+" FROM fixtures WHERE id="+quote(id)+";",2)}
func (d db) scan(i Implementation)(int,error){
 if i.Graph==""||len(i.Entrances)==0{return 0,errors.New("implementation scan requires graph and entrances")}
 check,e:=d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(i.Graph)+";",1)
 if e!=nil{return 0,e};if len(check)!=1{return 0,errors.New("graph not found")}
 stmts:=[]string{}
 for _,entry:=range i.Entrances{
  switch entry.Status {case "PRESENT","NOT_FOUND","UNKNOWN":default:return 0,fmt.Errorf("invalid scan status %q",entry.Status)}
  found,e:=d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(i.Graph)+" AND id="+quote(entry.Action)+" AND kind='action';",1)
  if e!=nil{return 0,e};if len(found)!=1{return 0,fmt.Errorf("unknown action %s",entry.Action)}
  st,e:=os.Lstat(entry.Evidence);if e!=nil{return 0,e}
  if !st.Mode().IsRegular()||st.Size()==0||st.Size()>4<<20{return 0,errors.New("scan evidence must be regular nonempty file <=4MiB")}
  b,e:=os.ReadFile(entry.Evidence);if e!=nil{return 0,e};hash:=sha256.Sum256(b)
  stmts=append(stmts,fmt.Sprintf("INSERT INTO implementation_scans(graph,action,status,evidence,sha256) VALUES(%s,%s,%s,%s,%s);",quote(i.Graph),quote(entry.Action),quote(entry.Status),quote(entry.Evidence),quote(hex.EncodeToString(hash[:]))))
 }
 return len(stmts),d.tx(stmts)
}
func (d db) gaps(graph string)([][]string,error){
 // NOT_FOUND is a candidate discovery gap, never automatic confirmation of missing functionality.
 return d.query("SELECT "+fieldsSQL("r.id","r.action","COALESCE((SELECT s.status FROM implementation_scans s WHERE s.graph=r.graph AND s.action=r.action ORDER BY s.id DESC LIMIT 1),'UNOBSERVED')")+" FROM rules r WHERE r.graph="+quote(graph)+" AND r.approved=1 ORDER BY r.id;",3)
}
