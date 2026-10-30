package main

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
)

// selftestLocalTurso proves that the actual embedded driver can create a
// fresh local database, apply the full schema, persist and read a row.
// It never contacts the tested application, cloud database or network.
func selftestLocalTurso() error {
    dir,e:=os.MkdirTemp("","arc-verify-selftest-")
    if e!=nil{return e}
    defer os.RemoveAll(dir)
    d,e:=openDB(filepath.Join(dir,"data.db"),true)
    if e!=nil{return e}
    defer d.close()
    if e=d.tx(schema);e!=nil{return fmt.Errorf("schema: %w",e)}
    _,e=d.sql("INSERT INTO graphs (id,scope,approved) VALUES ('selftest','local-only',1)")
    if e!=nil{return fmt.Errorf("insert: %w",e)}
    rows,e:=d.query("SELECT "+fieldsSQL("scope")+" FROM graphs WHERE id='selftest'",1)
    if e!=nil{return e}
    if len(rows)!=1||rows[0][0]!="local-only"{return errors.New("database readback mismatch")}
    indexes,e:=d.query("SELECT "+fieldsSQL("name")+" FROM sqlite_master WHERE type='index' AND name='idx_scans_action_history'",1)
    if e!=nil{return e}
    if len(indexes)!=1{return errors.New("implementation scan index missing")}
    return nil
}
