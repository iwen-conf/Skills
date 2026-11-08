package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCommandRouting(t *testing.T) {
	cases := []struct {
		args []string
		tool string
		forward []string
		wantError bool
	}{
		{[]string{"search", "hello", "--format", "json"}, "arc-idx", []string{"search", "hello", "--format", "json"}, false},
		{[]string{"verify", "gate", "--graph", "g1"}, "arc-verify", []string{"gate", "--graph", "g1"}, false},
		{[]string{"verify", "discover", "list", "--graph", "g1"}, "arc-verify", []string{"discover", "list", "--graph", "g1"}, false},
		{[]string{"verify"}, "", nil, true},
		{[]string{"task", "create"}, "", nil, true},
		{[]string{"update"}, "", nil, true},
		{[]string{"huh"}, "", nil, true},
	}
	for _, c := range cases {
		got, err := selectCommand(c.args)
		if (err != nil) != c.wantError { t.Fatalf("%v: error=%v", c.args, err) }
		if c.wantError { continue }
		if got.executable != c.tool || !reflect.DeepEqual(got.argv, c.forward) {
			t.Fatalf("%v: got %+v", c.args, got)
		}
	}
}

func TestCapabilitiesDoNotAdvertiseUnimplementedResources(t *testing.T) {
	got := capabilities()
	if got["unified_database"] != false || got["arc_db_resources"] != false {
		t.Fatal("must not claim implemented state engine")
	}
}

func TestHelpAndVersion(t *testing.T) {
	if run([]string{"--help"}) != 0 || run([]string{"version"}) != 0 {
		t.Fatal("help/version failed")
	}
	if !strings.Contains(usage, "arc verify") { t.Fatal("missing verification route") }
}
