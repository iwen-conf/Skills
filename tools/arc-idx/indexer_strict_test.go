package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A successful index must never silently acknowledge a truncated source set.
func TestStrictIndexRejectsMaxFilesLimit(t *testing.T) {
	root:=t.TempDir()
	if err:=os.MkdirAll(filepath.Join(root,".ai-code-index"),0700);err!=nil{t.Fatal(err)}
	if err:=os.MkdirAll(filepath.Join(root,".git"),0700);err!=nil{t.Fatal(err)}
	config:=[]byte("{\"max_files\":1}\n")
	if err:=os.WriteFile(filepath.Join(root,".ai-code-index","config.json"),config,0600);err!=nil{t.Fatal(err)}
	for _,name:=range []string{"a.go","b.go"}{
		if err:=os.WriteFile(filepath.Join(root,name),[]byte("package main\n"),0600);err!=nil{t.Fatal(err)}
	}
	err:=cmdIndex([]string{"--repo",root,"--quiet","--require-complete"})
	if err==nil||!strings.Contains(err.Error(),"max_files"){t.Fatalf("truncated index must fail in strict mode: %v",err)}
}
