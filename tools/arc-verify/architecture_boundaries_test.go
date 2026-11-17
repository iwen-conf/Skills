package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Keep business decisions reproducible. IO belongs in the runtime adapters,
// never in guard evaluation, frozen comparison or preparation planning.
func TestBusinessKernelHasNoAmbientIO(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		kernel := strings.HasPrefix(name, "business_contract") || name == "business_successor.go" || name == "preparation.go" || name == "preparation_runtime.go"
		if !kernel || strings.Contains(name, "_runtime") && name != "preparation_runtime.go" || strings.HasSuffix(name, "_test.go") || !strings.HasSuffix(name, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"os", "net", "database", "syscall", "unsafe", "runtime", "math/rand", "crypto/rand"} {
				if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
					t.Errorf("%s imports %s: deterministic business decisions must receive explicit observations or injected IO callbacks", name, path)
				}
			}
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil && function.Name.Name == "init" {
				t.Errorf("%s has ambient initialization: pass model and observation explicitly", name)
			}
		}
	}
}
