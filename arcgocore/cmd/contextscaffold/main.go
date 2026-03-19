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
	opts := scaffold.ContextOptions{Mode: "prime", ContextBudget: scaffold.DefaultContextBudget}
	fs := flag.NewFlagSet("contextscaffold", flag.ExitOnError)
	fs.StringVar(&opts.ProjectPath, "project-path", "", "Absolute path of target project")
	fs.StringVar(&opts.TaskName, "task-name", "", "Task name used for the context packet")
	fs.StringVar(&opts.Mode, "mode", "prime", "Context workflow mode")
	fs.StringVar(&opts.Objective, "objective", "", "Task objective or resume goal")
	fs.IntVar(&opts.ContextBudget, "context-budget", scaffold.DefaultContextBudget, "Approximate token budget for the compressed packet")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "Case output directory override")
	var entrypoints, dataSources stringList
	fs.Var(&entrypoints, "entrypoint", "Relevant file or artifact path")
	fs.Var(&dataSources, "data-source", "Large output source")
	fs.Parse(os.Args[1:])
	opts.Entrypoints = entrypoints
	opts.DataSources = dataSources
	if opts.ProjectPath == "" || opts.TaskName == "" {
		fmt.Fprintln(os.Stderr, "--project-path and --task-name are required")
		os.Exit(2)
	}
	caseDir, err := scaffold.RunContextScaffold(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Created context case: %s\n", caseDir)
}
