package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"runtime"
	"strings"
)

const (
	verifyHost60IP   = "192.168.0.60"
	verifyHost60User = "luwen"
)

type HostIdentity struct {
	Hostname string `json:"hostname"`
	User     string `json:"user"`
	IP       string `json:"ip"`
	GOOS     string `json:"goos"`
	PID      int    `json:"pid"`
	On60     bool   `json:"on_60"`
}

func collectHostIdentity() HostIdentity {
	id := HostIdentity{Hostname: hostnameNow(), GOOS: runtime.GOOS, PID: os.Getpid()}
	if current, err := user.Current(); err == nil {
		id.User = current.Username
	}
	id.IP = firstIPv4()
	id.On60 = runningOnHost60()
	return id
}

func hostnameNow() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

func firstIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		if ipnet.IP.String() == verifyHost60IP {
			return verifyHost60IP
		}
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		return ipnet.IP.String()
	}
	return ""
}

func runningOnHost60() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && ipnet.IP.To4() != nil && ipnet.IP.String() == verifyHost60IP {
			return true
		}
	}
	return false
}

func host60Required() bool {
	return os.Getenv("ARC_VERIFY_REQUIRE_HOST60") == "1"
}

func refuseOffHost60(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("target must be an HTTP URL on host 60: %s", raw)
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "127.0.0.1", "localhost", "::1", verifyHost60IP:
		return nil
	default:
		return fmt.Errorf("host-lock 60 forbids %s", parsed.Host)
	}
}

func imISOWriteURL() string {
	if raw := strings.TrimSpace(os.Getenv("ARC_VERIFY_IM_ISO_BASE")); raw != "" {
		return strings.TrimRight(raw, "/")
	}
	if runningOnHost60() {
		return "http://127.0.0.1:28886"
	}
	return "http://127.0.0.1:28886"
}
