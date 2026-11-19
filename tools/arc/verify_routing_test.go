package main

import (
	"reflect"
	"testing"
)

func TestUnifiedLedgerPreservesVerifierSubcommandPosition(t *testing.T) {
	for _, test := range []struct{ in, want []string }{
		{[]string{"checkpoint", "build", "--story", "cancel.story.yaml"}, []string{"checkpoint", "build", "--db", "/project/.arc/arc.db", "--story", "cancel.story.yaml"}},
		{[]string{"checkpoint", "clone", "--story", "cancel.story.yaml", "--save", "save_1"}, []string{"checkpoint", "clone", "--db", "/project/.arc/arc.db", "--story", "cancel.story.yaml", "--save", "save_1"}},
		{[]string{"play", "cancel.story.yaml", "--parallel"}, []string{"play", "--db", "/project/.arc/arc.db", "cancel.story.yaml", "--parallel"}},
		{[]string{"completion", "cancel.story.yaml"}, []string{"completion", "--db", "/project/.arc/arc.db", "cancel.story.yaml"}},
		{[]string{"snapshot", "--story", "ecommerce_account_cancellation", "--out", "out"}, []string{"snapshot", "--db", "/project/.arc/arc.db", "--story", "ecommerce_account_cancellation", "--out", "out"}},
	} {
		original := append([]string{}, test.in...)
		if actual := withVerifyDatabase(test.in, "/project/.arc/arc.db"); !reflect.DeepEqual(actual, test.want) {
			t.Fatalf("%v -> %v; want %v", test.in, actual, test.want)
		}
		if !reflect.DeepEqual(test.in, original) {
			t.Fatal("routing modified caller arguments")
		}
	}
}
