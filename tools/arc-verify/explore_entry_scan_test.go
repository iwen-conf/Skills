package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverIgnoresBareStrings(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "comment.go"), []byte(`package app
const decoy = "/not-a-route"
func notes() { _ = "/hidden-comment" }
`), 0600); err != nil {
		t.Fatal(err)
	}
	found, err := discoverSourceEntries(root, ExploreAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range found {
		if item.Path == "/not-a-route" || item.Path == "/hidden-comment" {
			t.Fatalf("bare strings must not become entries: %+v", found)
		}
	}
}
