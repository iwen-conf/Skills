package main

import (
	"reflect"
	"testing"
)

func TestUnifiedLedgerPreservesVerifierSubcommandPosition(t *testing.T) {
	for _, test := range []struct{ in, want []string }{
		{[]string{"contract", "match", "--action", "refund"}, []string{"contract", "match", "--db", "/project/.arc/arc.db", "--action", "refund"}},
		{[]string{"discover", "status", "--id", "E-1"}, []string{"discover", "status", "--db", "/project/.arc/arc.db", "--id", "E-1"}},
		{[]string{"risk", "refute", "--risk", "R-1"}, []string{"risk", "refute", "--db", "/project/.arc/arc.db", "--risk", "R-1"}},
		{[]string{"snapshot", "--graph", "payment"}, []string{"snapshot", "--db", "/project/.arc/arc.db", "--graph", "payment"}},
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
