package main

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	imIsolationEnv   = "iso28886"
	imSharedHumanEnv = "lan50"
	imSharedTXEnv    = "tx"
	imIsolationPort  = "28886"
	imSharedPort     = "28885"
	imObservePort    = "28895"
)

func refuseSharedIMTarget(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("IM target must be an HTTP(S) URL: %s", raw)
	}
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if port == imSharedPort || strings.Contains(host, "tx.nulltech.cn") {
		return fmt.Errorf("shared IM stack %s is not an authorized write target", parsed.Host)
	}
	if port == imObservePort {
		return fmt.Errorf("observe stack %s is not an authorized write target", parsed.Host)
	}
	if strings.Contains(raw, "121.43.161.42") {
		return fmt.Errorf("production IM host is not an authorized write target")
	}
	if port != imIsolationPort {
		return fmt.Errorf("IM write target must be isolation port %s, got %s", imIsolationPort, parsed.Host)
	}
	return nil
}

func fixturesBindIsolationOnly(fixtures Fixtures, env string) error {
	if env != imIsolationEnv {
		return fmt.Errorf("IM write fixtures must use %s, not %s", imIsolationEnv, env)
	}
	if len(fixtures.Fixtures) == 0 {
		return fmt.Errorf("IM fixture list is empty")
	}
	var bad []string
	for _, item := range fixtures.Fixtures {
		bound := item.Refs["environment"]
		if bound != env || bound == imSharedHumanEnv || bound == imSharedTXEnv || strings.Contains(bound, imSharedPort) {
			bad = append(bad, item.ID+":"+bound)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("fixtures still bind a shared or unapproved environment: %s", strings.Join(bad, ", "))
	}
	return nil
}
