package main

import (
	"os"
	"path/filepath"
	"testing"
)

// `arc db migrate` refuses a legacy ledger with a non-empty WAL, so every
// short-lived command must leave its writes checkpointed into the main file.
func TestCommandsLeaveNoUncheckpointedWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	if err := run([]string{"init", "--db", path}); err != nil {
		t.Fatal(err)
	}
	example := filepath.Join("examples", "academic-v1.json")
	if err := run([]string{"import", "--db", path, "--file", example}); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"-wal", "-shm"} {
		st, err := os.Lstat(path + ext)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() > 0 {
			t.Fatalf("%s%s holds %d uncheckpointed bytes after the command exited", filepath.Base(path), ext, st.Size())
		}
	}
	d, err := openDB(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	rows, err := d.query("SELECT id FROM graphs", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("imported graph not durable in the main file: %v", rows)
	}
}
