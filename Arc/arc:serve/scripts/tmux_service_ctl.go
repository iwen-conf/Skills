package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	schemaVersion = "1.0.0"
	managedBy     = "arc:serve"
)

type options struct {
	projectRoot      string
	serviceName      string
	workingDirectory string
	command          string
	portsCSV         string
	registryPath     string
	sessionName      string
}

type registry struct {
	SchemaVersion string         `json:"schema_version"`
	ProjectRoot   string         `json:"project_root"`
	UpdatedAt     string         `json:"updated_at"`
	Sessions      []sessionEntry `json:"sessions"`
}

type sessionEntry struct {
	ServiceName   string   `json:"service_name"`
	SessionName   string   `json:"session_name"`
	Cwd           string   `json:"cwd"`
	Command       string   `json:"command"`
	Ports         []string `json:"ports"`
	Status        string   `json:"status"`
	StartedAt     *string  `json:"started_at"`
	StoppedAt     *string  `json:"stopped_at"`
	LastAction    string   `json:"last_action"`
	LastActionAt  string   `json:"last_action_at"`
	LastCheckedAt string   `json:"last_checked_at"`
	ManagedBy     string   `json:"managed_by"`
}

type actionStatus struct {
	Action       string        `json:"action"`
	Result       string        `json:"result"`
	ProjectRoot  string        `json:"project_root"`
	RegistryPath string        `json:"registry_path"`
	ServiceName  string        `json:"service_name"`
	SessionName  string        `json:"session_name"`
	Live         bool          `json:"live"`
	Entry        *sessionEntry `json:"entry"`
}

type cleanupStatus struct {
	Action       string         `json:"action"`
	Result       string         `json:"result"`
	ProjectRoot  string         `json:"project_root"`
	RegistryPath string         `json:"registry_path"`
	Sessions     []sessionEntry `json:"sessions"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("action is required")
	}

	action := args[0]
	if action == "-h" || action == "--help" {
		printUsage()
		return nil
	}

	if err := requireCommand("tmux"); err != nil {
		return err
	}

	opts, err := parseArgs(action, args[1:])
	if err != nil {
		return err
	}

	if opts.projectRoot == "" {
		return errors.New("--project-root is required")
	}
	if action != "cleanup" && opts.serviceName == "" {
		return fmt.Errorf("--service is required for %s", action)
	}

	opts.projectRoot, err = filepath.Abs(opts.projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	if opts.registryPath == "" {
		opts.registryPath = filepath.Join(opts.projectRoot, ".arc", "serve", "tmux-sessions.json")
	}

	reg, err := loadOrInitRegistry(opts.registryPath, opts.projectRoot)
	if err != nil {
		return err
	}

	if opts.serviceName != "" && opts.sessionName == "" {
		opts.sessionName = resolveSessionName(reg, opts.projectRoot, opts.serviceName)
	}

	switch action {
	case "start":
		return startOrRestart(opts, &reg, "start")
	case "restart":
		return startOrRestart(opts, &reg, "restart")
	case "stop":
		return stopService(opts, &reg)
	case "status":
		return statusService(opts, &reg)
	case "cleanup":
		return cleanupRegistry(opts, &reg)
	default:
		printUsage()
		return fmt.Errorf("unsupported action: %s", action)
	}
}

func parseArgs(action string, args []string) (options, error) {
	var opts options

	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.projectRoot, "project-root", "", "project root path")
	fs.StringVar(&opts.serviceName, "service", "", "logical service name")
	fs.StringVar(&opts.workingDirectory, "cwd", "", "working directory")
	fs.StringVar(&opts.command, "command", "", "service command")
	fs.StringVar(&opts.portsCSV, "ports", "", "comma-separated ports")
	fs.StringVar(&opts.registryPath, "registry", "", "override registry path")
	fs.StringVar(&opts.sessionName, "session", "", "override session name")
	fs.Usage = printUsage

	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unknown argument: %s", fs.Arg(0))
	}

	return opts, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  Arc/arc:serve/scripts/tmux_service_ctl start   --project-root PATH --service NAME [--cwd PATH] [--ports CSV] [--registry PATH] [--session NAME] --command "CMD"
  Arc/arc:serve/scripts/tmux_service_ctl restart --project-root PATH --service NAME [--cwd PATH] [--ports CSV] [--registry PATH] [--session NAME] --command "CMD"
  Arc/arc:serve/scripts/tmux_service_ctl stop    --project-root PATH --service NAME [--registry PATH] [--session NAME]
  Arc/arc:serve/scripts/tmux_service_ctl status  --project-root PATH --service NAME [--registry PATH] [--session NAME]
  Arc/arc:serve/scripts/tmux_service_ctl cleanup --project-root PATH [--registry PATH]

Examples:
  Arc/arc:serve/scripts/tmux_service_ctl start --project-root /repo --service frontend --cwd /repo/web --ports 3000,5173 --command "pnpm dev"
  Arc/arc:serve/scripts/tmux_service_ctl restart --project-root /repo --service backend --cwd /repo/server --ports 8080 --command "go run ./cmd/server"
  Arc/arc:serve/scripts/tmux_service_ctl stop --project-root /repo --service frontend

The launcher caches a compiled Go binary under ${XDG_CACHE_HOME:-$HOME/.cache}/arc-serve/ and reuses it until the source changes.`)
}

func requireCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("missing required command: %s", name)
	}
	return nil
}

func loadOrInitRegistry(path string, projectRoot string) (registry, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return registry{}, fmt.Errorf("create registry dir: %w", err)
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		reg := registry{
			SchemaVersion: schemaVersion,
			ProjectRoot:   projectRoot,
			UpdatedAt:     isoNow(),
			Sessions:      []sessionEntry{},
		}
		if err := writeRegistry(path, reg); err != nil {
			return registry{}, err
		}
		return reg, nil
	} else if err != nil {
		return registry{}, fmt.Errorf("stat registry: %w", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return registry{}, fmt.Errorf("read registry: %w", err)
	}

	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return registry{}, fmt.Errorf("parse registry: %w", err)
	}
	if reg.SchemaVersion == "" {
		reg.SchemaVersion = schemaVersion
	}
	if reg.ProjectRoot == "" {
		reg.ProjectRoot = projectRoot
	}
	if reg.Sessions == nil {
		reg.Sessions = []sessionEntry{}
	}

	return reg, nil
}

func writeRegistry(path string, reg registry) error {
	reg.SchemaVersion = schemaVersion
	if reg.Sessions == nil {
		reg.Sessions = []sessionEntry{}
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "tmux-sessions-*.json")
	if err != nil {
		return fmt.Errorf("create temp registry: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	enc := json.NewEncoder(tmp)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(reg); err != nil {
		tmp.Close()
		return fmt.Errorf("encode registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp registry: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace registry: %w", err)
	}
	return nil
}

func isoNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(value) {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}

	result := strings.Trim(b.String(), "-")
	return result
}

func defaultSessionName(projectRoot string, serviceName string) string {
	projectSlug := slugify(filepath.Base(projectRoot))
	serviceSlug := slugify(serviceName)
	sum := sha256.Sum256([]byte(projectRoot + ":" + serviceName))
	hash := hex.EncodeToString(sum[:])[:8]
	return fmt.Sprintf("arc-%s-%s-%s", projectSlug, serviceSlug, hash)
}

func resolveSessionName(reg registry, projectRoot string, serviceName string) string {
	for _, entry := range reg.Sessions {
		if entry.ServiceName == serviceName && entry.SessionName != "" {
			return entry.SessionName
		}
	}
	return defaultSessionName(projectRoot, serviceName)
}

func parsePorts(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return []string{}
	}
	parts := strings.Split(csv, ",")
	ports := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			ports = append(ports, trimmed)
		}
	}
	return ports
}

func portsToCSV(ports []string) string {
	return strings.Join(ports, ",")
}

func findEntryIndex(reg registry, serviceName string, sessionName string) int {
	for idx, entry := range reg.Sessions {
		if (serviceName != "" && entry.ServiceName == serviceName) || (sessionName != "" && entry.SessionName == sessionName) {
			return idx
		}
	}
	return -1
}

func findEntry(reg registry, serviceName string, sessionName string) *sessionEntry {
	idx := findEntryIndex(reg, serviceName, sessionName)
	if idx == -1 {
		return nil
	}
	entry := reg.Sessions[idx]
	return &entry
}

func liveSessions() (map[string]struct{}, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}

	sessions := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions[line] = struct{}{}
		}
	}
	return sessions, nil
}

func tmuxSessionExists(sessionName string) (bool, error) {
	if strings.TrimSpace(sessionName) == "" {
		return false, nil
	}

	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, fmt.Errorf("check tmux session %s: %w", sessionName, err)
	}

	return true, nil
}

func reconcileRegistry(opts options, reg *registry) error {
	now := isoNow()
	live, err := liveSessions()
	if err != nil {
		return err
	}

	reg.ProjectRoot = reg.ProjectRoot
	if reg.ProjectRoot == "" {
		reg.ProjectRoot = opts.projectRoot
	}
	reg.UpdatedAt = now

	for idx := range reg.Sessions {
		entry := &reg.Sessions[idx]
		entry.LastCheckedAt = now
		if _, ok := live[entry.SessionName]; ok {
			entry.Status = "running"
			continue
		}
		if entry.Status == "running" {
			entry.Status = "missing"
		}
	}

	return nil
}

func hydrateDefaultsFromEntry(opts *options, reg registry) {
	entry := findEntry(reg, opts.serviceName, opts.sessionName)
	if entry == nil {
		return
	}
	if opts.workingDirectory == "" {
		opts.workingDirectory = entry.Cwd
	}
	if opts.command == "" {
		opts.command = entry.Command
	}
	if opts.portsCSV == "" {
		opts.portsCSV = portsToCSV(entry.Ports)
	}
}

func upsertEntry(opts options, reg *registry, status string, action string) {
	now := isoNow()
	ports := parsePorts(opts.portsCSV)

	existingIdx := findEntryIndex(*reg, opts.serviceName, opts.sessionName)
	var existing *sessionEntry
	if existingIdx >= 0 {
		entryCopy := reg.Sessions[existingIdx]
		existing = &entryCopy
	}

	filtered := make([]sessionEntry, 0, len(reg.Sessions))
	for _, entry := range reg.Sessions {
		if entry.ServiceName == opts.serviceName || entry.SessionName == opts.sessionName {
			continue
		}
		filtered = append(filtered, entry)
	}

	var startedAt *string
	switch {
	case status != "running":
		if existing != nil {
			startedAt = existing.StartedAt
		}
	case action == "reuse" && existing != nil && existing.StartedAt != nil:
		startedAt = existing.StartedAt
	default:
		startedAt = stringPtr(now)
	}

	var stoppedAt *string
	switch status {
	case "stopped":
		stoppedAt = stringPtr(now)
	case "running":
		stoppedAt = nil
	default:
		if existing != nil {
			stoppedAt = existing.StoppedAt
		}
	}

	filtered = append(filtered, sessionEntry{
		ServiceName:   opts.serviceName,
		SessionName:   opts.sessionName,
		Cwd:           opts.workingDirectory,
		Command:       opts.command,
		Ports:         ports,
		Status:        status,
		StartedAt:     startedAt,
		StoppedAt:     stoppedAt,
		LastAction:    action,
		LastActionAt:  now,
		LastCheckedAt: now,
		ManagedBy:     managedBy,
	})

	reg.ProjectRoot = opts.projectRoot
	reg.UpdatedAt = now
	reg.Sessions = filtered
}

func stringPtr(value string) *string {
	v := value
	return &v
}

func emitJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func emitActionStatus(opts options, reg registry, action string, result string, live bool) error {
	return emitJSON(actionStatus{
		Action:       action,
		Result:       result,
		ProjectRoot:  opts.projectRoot,
		RegistryPath: opts.registryPath,
		ServiceName:  opts.serviceName,
		SessionName:  opts.sessionName,
		Live:         live,
		Entry:        findEntry(reg, opts.serviceName, opts.sessionName),
	})
}

func emitCleanupStatus(opts options, reg registry) error {
	return emitJSON(cleanupStatus{
		Action:       "cleanup",
		Result:       "ok",
		ProjectRoot:  opts.projectRoot,
		RegistryPath: opts.registryPath,
		Sessions:     reg.Sessions,
	})
}

func killLiveSessionIfPresent(sessionName string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return fmt.Errorf("kill tmux session %s: %w", sessionName, err)
	}
	return nil
}

func waitUntilGone(sessionName string) error {
	for attempts := 0; attempts < 20; attempts++ {
		exists, err := tmuxSessionExists(sessionName)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("session still alive after kill: %s", sessionName)
}

func startOrRestart(opts options, reg *registry, action string) error {
	if err := reconcileRegistry(opts, reg); err != nil {
		return err
	}
	hydrateDefaultsFromEntry(&opts, *reg)

	exists, err := tmuxSessionExists(opts.sessionName)
	if err != nil {
		return err
	}
	if exists {
		if action == "start" {
			upsertEntry(opts, reg, "running", "reuse")
			if err := writeRegistry(opts.registryPath, *reg); err != nil {
				return err
			}
			return emitActionStatus(opts, *reg, "start", "already_running", true)
		}
		if err := killLiveSessionIfPresent(opts.sessionName); err != nil {
			return err
		}
		if err := waitUntilGone(opts.sessionName); err != nil {
			return err
		}
	}

	if opts.command == "" {
		return fmt.Errorf("--command is required for %s", action)
	}
	if opts.workingDirectory == "" {
		opts.workingDirectory = opts.projectRoot
	}

	cmd := exec.Command("tmux", "new-session", "-d", "-s", opts.sessionName, "-c", opts.workingDirectory, opts.command)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start tmux session %s: %w", opts.sessionName, err)
	}

	time.Sleep(time.Second)

	exists, err = tmuxSessionExists(opts.sessionName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("failed to keep tmux session alive after launch: %s", opts.sessionName)
	}

	upsertEntry(opts, reg, "running", action)
	if err := writeRegistry(opts.registryPath, *reg); err != nil {
		return err
	}
	return emitActionStatus(opts, *reg, action, "ok", true)
}

func stopService(opts options, reg *registry) error {
	if err := reconcileRegistry(opts, reg); err != nil {
		return err
	}
	hydrateDefaultsFromEntry(&opts, *reg)

	exists, err := tmuxSessionExists(opts.sessionName)
	if err != nil {
		return err
	}
	if exists {
		if err := killLiveSessionIfPresent(opts.sessionName); err != nil {
			return err
		}
		if err := waitUntilGone(opts.sessionName); err != nil {
			return err
		}
	}

	if opts.workingDirectory == "" {
		opts.workingDirectory = opts.projectRoot
	}

	upsertEntry(opts, reg, "stopped", "stop")
	if err := writeRegistry(opts.registryPath, *reg); err != nil {
		return err
	}
	return emitActionStatus(opts, *reg, "stop", "ok", false)
}

func statusService(opts options, reg *registry) error {
	if err := reconcileRegistry(opts, reg); err != nil {
		return err
	}
	hydrateDefaultsFromEntry(&opts, *reg)
	if opts.workingDirectory == "" {
		opts.workingDirectory = opts.projectRoot
	}

	if err := writeRegistry(opts.registryPath, *reg); err != nil {
		return err
	}

	live, err := tmuxSessionExists(opts.sessionName)
	if err != nil {
		return err
	}
	return emitActionStatus(opts, *reg, "status", "ok", live)
}

func cleanupRegistry(opts options, reg *registry) error {
	if err := reconcileRegistry(opts, reg); err != nil {
		return err
	}
	if err := writeRegistry(opts.registryPath, *reg); err != nil {
		return err
	}
	return emitCleanupStatus(opts, *reg)
}
