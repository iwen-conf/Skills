package main

import (
	"strings"
	"testing"
)

func TestEntryScanDoesNotMistakeOutgoingClientCallForRoute(t *testing.T) {
	src:=[]byte("package demo\nfunc ping(){client.Get(\"/external\")}; func routes(){router.GET(\"/orders\",handler)}")
	found:=discoverGoHTTPEntries("routes.go",src,map[string]bool{})
	if len(found)!=1 || found[0].Path!="/orders" {
		t.Fatalf("client.Get must not become an endpoint: %+v",found)
	}
}

func TestEntryScannerHonorsMethodAuthorization(t *testing.T) {
	adapter:=ExploreAdapter{Entries: []ExploreEntry{{Action:"submit",Entry:"/orders",Method:"POST"}}}
	item:=authorizeDiscoveredEntry(DiscoveredEntry{Path:"/orders",Method:"GET",Class:entryConfirmed},adapter)
	if item.Class==entryConfirmed || item.Action!="" {
		t.Fatalf("GET must not inherit POST permission: %+v",item)
	}
	good:=authorizeDiscoveredEntry(DiscoveredEntry{Path:"/orders",Method:"POST",Class:entryUndeclared},adapter)
	if good.Class!=entryConfirmed || good.Action!="submit" {
		t.Fatalf("approved entry was not matched: %+v",good)
	}
}

func TestOpenAPIEntryScanKeepsRealMethodsAndIgnoresMetadata(t *testing.T) {
	src:=[]byte(`{"paths":{"/orders":{"parameters":[],"post":{},"get":{}},"/health":{"get":{}}}}`)
	found:=discoverOpenAPIEntries("openapi.json",src,map[string]bool{})
	if len(found)!=3 {t.Fatalf("only actual HTTP methods qualify: %+v",found)}
	got:=[]string{}
	for _,item:=range found {got=append(got,item.Method+" "+item.Path)}
	joined:=strings.Join(got,",")
	if joined!="GET /health,GET /orders,POST /orders" {t.Fatalf("nondeterministic or missing methods: %s",joined)}
}
