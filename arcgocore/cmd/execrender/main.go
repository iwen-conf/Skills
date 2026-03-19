package main

import (
	"flag"
	"fmt"
	"os"

	"skills/arcgocore/internal/scaffold"
)

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	opts := scaffold.ExecRenderOptions{Route: "direct-dispatch", RouteReason: "待补充", Risk: "medium"}
	fs := flag.NewFlagSet("execrender", flag.ExitOnError)
	fs.StringVar(&opts.CaseDir, "case-dir", "", "Path to .arc/exec/<task-name>")
	fs.StringVar(&opts.TaskName, "task-name", "", "Task name")
	fs.StringVar(&opts.Route, "route", "direct-dispatch", "Routing result")
	fs.StringVar(&opts.RouteReason, "route-reason", "待补充", "Why this route was selected")
	fs.StringVar(&opts.Risk, "risk", "medium", "Risk level summary")
	var dispatches, nextSteps stringList
	fs.Var(&dispatches, "dispatch", "Dispatch row in format profile|capabilities|description|status|output")
	fs.Var(&nextSteps, "next-step", "Next action item")
	fs.Parse(os.Args[1:])
	opts.DispatchRows = dispatches
	opts.NextSteps = nextSteps
	if opts.CaseDir == "" || opts.TaskName == "" {
		fmt.Fprintln(os.Stderr, "--case-dir and --task-name are required")
		os.Exit(2)
	}
	caseDir, err := scaffold.RunExecRender(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Rendered dispatch artifacts in: %s\n", caseDir)
}
